import { HttpClient } from "@angular/common/http";
import { Injectable, inject } from "@angular/core";
import { LiveTopic, LivewireClient } from "@softwarity/livewire";
import type { LiveRow } from "@softwarity/livewire";

// One scheduled call, as the live channel sends it (SCHED-01).
//
// The id is the schedule's; the VERSION is its updated_at, so a row already on
// screen is only sent again when something moved it. A screen watching fifty
// schedules therefore receives the one that just started a run, not fifty.
export interface ScheduleRow extends LiveRow {
  id: string;
  name: string;
  // The roles the CALL carries: a scheduled call is made as "meerkat", and
  // these are what the route's rule and the forwarded identity read.
  roles?: string[];
  tenantId: string;
  routeId: string;
  method: string;
  path: string;
  // WHEN, one of two ways: a cadence (ISO duration) or a calendar (five-field
  // cron, read in its own zone). Never both.
  every: string;
  cron: string;
  timezone: string;
  // The third way to say when: ONE date, RFC 3339. The call goes out then,
  // once, and the schedule is finished rather than armed again.
  at?: string;
  // The service's own filing system - station=42, kind=poll - which Meerkat
  // stores and matches on without knowing what any of it means.
  metadata?: Record<string, string>;
  // Read from the drawer's own call, never from the live row.
  // JSON as it was written: an object or an array goes out as it stands, a JSON
  // string goes out as its content - so the shape here is whatever was sent.
  body?: unknown;
  contentType?: string;
  // Values may be vault references, resolved at the moment of the call: what
  // this API hands back is the reference, never the secret behind it.
  headers?: Record<string, string>;
  overlap?: string;
  catchUp?: number;
  timeout?: string;
  paused: boolean;
  nextAt: number;
  // The run in flight, when there is one: its identifier, when it started,
  // which node claimed it, and how far the service says it has got.
  runId: string;
  runStarted: number;
  claimedBy: string;
  progress: number;
  // Where that run stands: "calling" until the service answers, "accepted"
  // once it took the work (a 202). And how many times it has been sent: 2 or
  // more means a gateway stopped before the answer and another sent it again.
  runState: string;
  attempts: number;
  // How the LAST FINISHED run ended: done, failed, or lost. A run in flight
  // does not touch it, so the two show side by side.
  lastState: string;
  lastDetail: string;
  lastAt: number;
  createdBy: string;
}

// One ENDED turn, as the history keeps it (SCHED-03). The row above keeps
// only the last result; this is what answers "since when is it failing" and
// "did last night's close run".
export interface ScheduleRun {
  id: string;
  scheduleId: string;
  // What the service was told to dedupe on. Empty for a turn that never went
  // out - dropped for coming round too late.
  runId?: string;
  // Which attempt ENDED the turn: 2 means a gateway stopped before the answer
  // and another sent the call again.
  attempt?: number;
  node?: string;
  // Why it went out, and which run of this schedule it continues: that pair
  // is what makes a chain readable.
  cause?: string;
  ofRun?: string;
  startedAt?: number;
  endedAt: number;
  state: string;
  status?: number;
  detail?: string;
}

// What the screen narrows by. Sent with the subscription rather than filtered
// on the client: one reader watching one organisation should not receive
// everybody else's rows to hide them.
export interface ScheduleFilter {
  tenant?: string;
  route?: string;
  // Conditions on the service's metadata, as written: `station=42`, or
  // `station=~^st-\d+$`. Sent verbatim - the gateway compiles them, refuses
  // what does not compile, and matches.
  meta?: string[];
}

// The scheduled calls, live.
//
// A service and not a component field, for the reason the traffic one gives:
// the topic is the screen's one subscription, and the component is rebuilt
// whenever somebody navigates away and back.
@Injectable({ providedIn: "root" })
export class SchedulesService {
  private readonly topic = new LiveTopic<ScheduleRow>(
    inject(LivewireClient),
    "schedules",
  );
  private readonly http = inject(HttpClient);

  window = (filter: ScheduleFilter, offset: number, limit: number) =>
    this.topic.window(filter, offset, limit);
  resync = () => this.topic.resync();

  // The whole schedule, as the gateway stores it. The live row carries what
  // the LIST draws; the drawer asks for the rest when somebody opens it -
  // including the body, which has no business travelling to fifty screens
  // that are not showing it.
  get = (id: string) =>
    this.http.get<ScheduleRow>(`/api/schedules/${encodeURIComponent(id)}`);

  // The three things an operator does at two in the morning. Creating one is
  // not here on purpose: a schedule runs as an ACCOUNT, and the console's
  // administrators are not the accounts it should run as - a service writes
  // its own with its own token, and the API refuses an operator who tries.
  // What each turn did, newest first. A plain read rather than a live topic:
  // a history moves when a run ends, which the row beside it already
  // announces, and nobody watches a list of closed runs by the second.
  runs = (id: string) =>
    this.http.get<ScheduleRun[]>(
      `/api/schedules/${encodeURIComponent(id)}/runs`,
    );

  // Ask for a past turn again. It goes out as a NEW run carrying today's
  // payload, and the history says which one it replays.
  replay = (id: string, runId: string) =>
    this.http.post(`/api/schedules/${id}/run`, { replayOf: runId });

  pause = (id: string) => this.http.post(`/api/schedules/${id}/pause`, {});
  resume = (id: string) => this.http.post(`/api/schedules/${id}/resume`, {});
  runNow = (id: string) => this.http.post(`/api/schedules/${id}/run`, {});
  remove = (id: string) => this.http.delete(`/api/schedules/${id}`);
}
