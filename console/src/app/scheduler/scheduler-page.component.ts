import { Component, computed, inject, signal } from "@angular/core";
import { toSignal } from "@angular/core/rxjs-interop";
import { httpResource } from "@angular/common/http";
import { MatButtonModule } from "@angular/material/button";
import { MatSlideToggleModule } from "@angular/material/slide-toggle";
import { MatChipsModule } from "@angular/material/chips";
import { KeyValuePipe } from "@angular/common";
import { MatExpansionModule } from "@angular/material/expansion";
import { MatFormFieldModule } from "@angular/material/form-field";
import { MatInputModule } from "@angular/material/input";
import { MatIconModule } from "@angular/material/icon";
import { MatSidenavModule } from "@angular/material/sidenav";
import { MatProgressBarModule } from "@angular/material/progress-bar";
import { MatSelectModule } from "@angular/material/select";
import { MatSnackBar } from "@angular/material/snack-bar";
import { MatTableModule } from "@angular/material/table";
import { MatTooltipModule } from "@angular/material/tooltip";
import { LiveWindowDataSource } from "@softwarity/livewire";
import { RowActionsDirective } from "@softwarity/row-actions";
import { ApiService } from "../api.service";
import { FormFieldComponent } from "../shared/form-field.component";
import { SnippetComponent } from "../shared/snippet.component";
import { MeService } from "../me.service";
import { ZonedDatePipe } from "../shared/zoned-date.pipe";
import { SchedulerApiComponent } from "./scheduler-api.component";
import {
  ScheduleRow,
  ScheduleRun,
  SchedulesService,
} from "./schedules.service";

// The scheduled calls (SCHED-01): what this gateway calls on a timer, for
// whom, and how the last run ended.
//
// It watches rather than polls, and that is the reason it is on the socket at
// all: the interesting moment lasts as long as the job does, so a run starting,
// advancing and finishing has to appear without anybody pressing anything.
//
// Nothing here CREATES a schedule. A schedule runs as an account, and the
// administrators reading this screen are not the accounts it should run as - a
// service asks for its own on the data plane, as itself, which is what keeps
// this from being a way to mint a standing call under somebody's identity.
@Component({
  selector: "app-scheduler-page",
  imports: [
    FormFieldComponent,
    KeyValuePipe,
    MatButtonModule,
    MatExpansionModule,
    MatSlideToggleModule,
    MatChipsModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatProgressBarModule,
    MatSelectModule,
    MatSidenavModule,
    MatTableModule,
    MatTooltipModule,
    RowActionsDirective,
    SchedulerApiComponent,
    SnippetComponent,
    ZonedDatePipe,
  ],
  templateUrl: "./scheduler-page.component.html",
  styleUrl: "./scheduler-page.component.scss",
})
export class SchedulerPageComponent {
  private readonly schedules = inject(SchedulesService);
  private readonly api = inject(ApiService);

  // WHICH HOURS these are. A scheduler is read by people who are not where the
  // gateway is: "2:24 AM" means nothing until it says whose 2:24. The times are
  // written in the zone this operator chose on their own profile, and the
  // switch offers the one alternative worth naming - UTC, which then says so
  // with a Z. The other state needs no label: it is the hours of whoever is
  // reading.
  private readonly mine = inject(MeService).timezone;
  protected readonly utc = signal(false);
  protected readonly zone = computed(() => (this.utc() ? "UTC" : this.mine()));
  // An account that never chose a zone reads UTC anyway, so the switch would
  // flip between two identical answers: it says so instead, and points at the
  // one place that fixes it.
  protected readonly zoneIsUtc = computed(() => this.mine() === "UTC");
  private readonly snack = inject(MatSnackBar);

  // The toggle says UTC; the snackbar says where you landed. Which matters in
  // the direction the toggle cannot label: coming BACK is coming back to a
  // zone that has a name, and naming it once beats printing it on the header
  // for ever.
  protected switchZone(utc: boolean): void {
    this.utc.set(utc);
    this.snack.open(
      $localize`:@@Times_shown_in:Times shown in ${this.zone()}:zone:`,
      undefined,
      { duration: 2500 },
    );
  }

  protected readonly zoneHint = computed(() =>
    this.zoneIsUtc()
      ? $localize`:@@Zone_none:Your profile has no timezone, so these already are the gateway's own hours`
      : $localize`:@@Zone_to_utc:Show the times in UTC rather than ${this.mine()}:zone:`,
  );

  // The three filters, and they travel WITH the subscription: a reader
  // watching one service does not receive the others to hide them.
  protected readonly tenant = signal("");
  protected readonly route = signal("");
  // The fourth filter, and the one that scales: a fleet's schedules are told
  // apart by what the SERVICE filed them under, not by anything this console
  // could have known to offer. `station=42`, or `station=~^st-\d+$`; several
  // conditions separated by spaces, all of them required.
  protected readonly meta = signal("");
  protected readonly metaError = signal("");
  protected readonly metaHint = computed(
    () =>
      this.metaError() ||
      $localize`:@@Metadata_filter_hint:key=value, or key=~expression`,
  );

  // Built here rather than in the service: the source repaints the view it was
  // created in, so one built outside a component has nobody to answer.
  protected readonly source = new LiveWindowDataSource<ScheduleRow>(
    () => this.schedules.resync(),
    200,
  );
  private readonly rows = toSignal(this.source.changes, { initialValue: [] });
  protected readonly all = computed(() =>
    this.rows().filter((r): r is ScheduleRow => !!r),
  );
  // A delayed action stays on screen after it has gone, until the sweep: it
  // is how somebody sees that it DID go out. But a service that posts a
  // hundred a day would bury the rest, so they hide on a switch - client
  // side, because the rows are already here.
  protected readonly hideFinished = signal(false);
  protected readonly list = computed(() =>
    this.hideFinished()
      ? this.all().filter((r) => !this.finished(r))
      : this.all(),
  );
  protected readonly finishedCount = computed(
    () => this.all().filter((r) => this.finished(r)).length,
  );

  // How long a finished delayed action is kept, root's alone (SCHED-02).
  protected readonly retention = signal(0);
  protected readonly retentionChoices = signal<number[]>([]);

  // The lists the filters offer. Read once: an account, an organisation and a
  // route are not what changes while somebody watches a scheduler.
  protected readonly users = toSignal(this.api.listUsers(), {
    initialValue: [],
  });
  protected readonly tenants = toSignal(this.api.listTenants(), {
    initialValue: [],
  });
  protected readonly routes = toSignal(this.api.listRoutes(), {
    initialValue: [],
  });

  protected readonly columns = [
    "name",
    "calls",
    "every",
    "next",
    "state",
    "last",
  ];

  // The metadata is READ in the drawer, never in the row: a row would carry a
  // clipped third of it, which is a tooltip pretending to be a column. On the
  // list, what it is for is the filter above.
  protected hasMeta(row: ScheduleRow): boolean {
    return Object.keys(row.metadata ?? {}).length > 0;
  }

  // The row opens a detail panel: the body sent, the three settings that
  // explain a skipped turn, and the full text of a failure - which the row can
  // only carry in a tooltip, and a tooltip is not where somebody reads an
  // error at two in the morning.
  protected readonly selected = signal<ScheduleRow | null>(null);

  // The drawer holds one of two things: a schedule, or the explanation of how
  // a service gets one. Two panels, one surface - and opening either closes
  // the other, because a drawer showing two things is a drawer showing one of
  // them badly.
  protected readonly howto = signal(false);

  protected open(row: ScheduleRow): void {
    this.howto.set(false);
    this.selected.set(row);
  }

  // What this schedule was WRITTEN as: the payload that recreates it, fetched
  // when the drawer opens. Two things at once - the fields a row cannot show
  // (the body, the three settings), and a worked example for whoever has to
  // write the next one.
  private readonly detail = httpResource<ScheduleRow>(() => {
    const one = this.selected();
    return one ? `/api/schedules/${encodeURIComponent(one.id)}` : undefined;
  });
  protected readonly payload = computed(() => {
    const s = this.detail.value();
    if (!s) return "";
    // Only what a caller SENDS: the run, the times and the identifiers are the
    // gateway's own answer, and copying them back would be noise.
    const out: Record<string, unknown> = { name: s.name };
    if (s.roles?.length) out["roles"] = s.roles;
    if (s.tenantId) out["tenantId"] = s.tenantId;
    out["routeId"] = s.routeId;
    if (s.method && s.method !== "POST") out["method"] = s.method;
    out["path"] = s.path;
    if (s.headers && Object.keys(s.headers).length) out["headers"] = s.headers;
    // JSON rather than text: a body of 0, false or "" is still a body, so only
    // its absence drops the field.
    if (s.body !== undefined && s.body !== null) out["body"] = s.body;
    if (s.contentType) out["contentType"] = s.contentType;
    if (s.at) {
      out["at"] = s.at;
    } else if (s.cron) {
      out["cron"] = s.cron;
      if (s.timezone) out["timezone"] = s.timezone;
    } else {
      out["every"] = s.every;
    }
    if (s.overlap && s.overlap !== "skip") out["overlap"] = s.overlap;
    if (s.catchUp) out["catchUp"] = s.catchUp;
    if (s.timeout) out["timeout"] = s.timeout;
    if (s.metadata && Object.keys(s.metadata).length)
      out["metadata"] = s.metadata;
    return JSON.stringify(out, null, 2);
  });

  // What each turn did (SCHED-03), read when the drawer opens. The row beside
  // it says how the LAST one ended; this is where "since when" lives - and
  // where a turn that never went out can be asked for again.
  private readonly reload = signal(0);
  private readonly runsRes = httpResource<ScheduleRun[]>(() => {
    const one = this.selected();
    this.reload();
    return one
      ? `/api/schedules/${encodeURIComponent(one.id)}/runs?limit=50`
      : undefined;
  });
  protected readonly runs = computed(() => this.runsRes.value() ?? []);

  // The chain, as a sentence: what this run continues, when it continues
  // something. An ordinary turn says nothing, which is the common case.
  protected chainOf(run: ScheduleRun): string {
    switch (run.cause) {
      case "replay":
        return $localize`:@@Replay_of:replay of an earlier run`;
      case "retry":
        return $localize`:@@Retry_of:another attempt at the same turn`;
      case "asked":
        return $localize`:@@Asked_by_service:the service asked to be called back`;
      default:
        return "";
    }
  }

  // How long it took, when that is worth a word: the seconds are what the
  // history stores, so anything under one is "it answered" and says nothing.
  protected took(run: ScheduleRun): string {
    if (!run.startedAt || run.endedAt <= run.startedAt) return "";
    const s = run.endedAt - run.startedAt;
    return s < 60
      ? $localize`:@@N_seconds:${s}:n: s`
      : $localize`:@@N_minutes:${Math.round(s / 60)}:n: min`;
  }

  // The sentence a run leaves. On a success it only repeats the status, which
  // is already there; on anything else it is the whole point of the line.
  protected detailOf(run: ScheduleRun): string {
    return run.state === "done" ? "" : (run.detail ?? "");
  }

  // A turn that did not go out, or went out badly, is worth asking for again.
  // A done one is not: replaying a success is how a job runs twice.
  protected replayable(run: ScheduleRun): boolean {
    return run.state !== "done";
  }

  protected replay(row: ScheduleRow, run: ScheduleRun): void {
    this.schedules.replay(row.id, run.id).subscribe(() => {
      this.subscribe();
      this.reload.update((n) => n + 1);
    });
  }

  protected openHowto(): void {
    this.selected.set(null);
    this.howto.set(true);
  }

  protected closePanel(): void {
    this.selected.set(null);
    this.howto.set(false);
  }

  // Whether this reader is root, for the one control that is root's alone.
  protected readonly me = inject(MeService);

  constructor() {
    this.subscribe();
    // Root's alone, so it is asked for only by root: a 403 in the console of
    // an application administrator is noise about a control they cannot see.
    if (this.me.isRoot()) {
      this.api.scheduleSettings().subscribe((s) => {
        this.retention.set(s.retentionDays);
        this.retentionChoices.set(s.choices);
      });
    }
  }

  // One subscription at a time: changing a filter asks a different question,
  // and the old answer is not a subset of the new one.
  protected subscribe(): void {
    const meta = this.conditions();
    // A condition that is not one would be sent, refused, and come back as an
    // empty list - which reads as "there are no schedules" rather than "that
    // is not a filter". So it is said here, and nothing is asked for.
    if (meta === null) return;
    this.source.reset((offset, limit) =>
      this.schedules.window(
        {
          tenant: this.tenant(),
          route: this.route(),
          meta,
        },
        offset,
        limit,
      ),
    );
  }

  // The conditions as written, or null when one of them is not a condition.
  // The gateway checks them again - it is the one that matches - but the
  // mistakes worth catching here are the two a person makes at the keyboard:
  // forgetting the `=`, and an expression that does not compile.
  private conditions(): string[] | null {
    const out: string[] = [];
    for (const one of this.meta().split(/\s+/).filter(Boolean)) {
      const at = one.indexOf("=");
      if (at <= 0) {
        this.metaError.set(
          $localize`:@@Meta_filter_shape:Write key=value, or key=~expression`,
        );
        return null;
      }
      const value = one.slice(at + 1);
      if (value.startsWith("~")) {
        try {
          new RegExp(value.slice(1));
        } catch {
          this.metaError.set(
            $localize`:@@Meta_filter_expr:That expression does not compile`,
          );
          return null;
        }
      }
      out.push(one);
    }
    this.metaError.set("");
    return out;
  }

  protected named(row: ScheduleRow): string {
    return this.routes().find((r) => r.id === row.routeId)?.name ?? row.routeId;
  }

  // What the call will carry, for the row: the roles it asks for, or nothing -
  // which is worth seeing, because a route with a rule will turn that away.
  protected rolesOf(row: ScheduleRow): string {
    return (row.roles ?? []).join(", ");
  }

  // A single date that has been and gone: it owes nothing, and nothing is in
  // flight. A schedule that repeats is never finished.
  // The instant a single date names, as the seconds every date on this
  // screen is drawn from.
  protected momentOf(row: ScheduleRow): number {
    return row.at ? Math.floor(Date.parse(row.at) / 1000) : 0;
  }

  protected finished(row: ScheduleRow): boolean {
    return !!row.at && !row.nextAt && !row.runId;
  }

  protected setRetention(days: number): void {
    const before = this.retention();
    this.retention.set(days);
    this.api
      .setScheduleRetention(days)
      .subscribe({ error: () => this.retention.set(before) });
  }

  // A week, a month, three months, a year - said the way a person says them,
  // singular included: "1 months" is the kind of label that makes a screen
  // look unfinished.
  protected retentionLabel(days: number): string {
    switch (days) {
      case 7:
        return $localize`:@@One_week:1 week`;
      case 30:
        return $localize`:@@One_month:1 month`;
      case 365:
        return $localize`:@@One_year:1 year`;
      default:
        return days % 30 === 0
          ? $localize`:@@N_months:${Math.round(days / 30)}:n: months`
          : $localize`:@@N_days:${days}:n: days`;
    }
  }

  // A run in flight, however long it has been one. The lease settles an
  // abandoned one within minutes - sent again, or lost - so a row here for
  // hours is a long job the service took and keeps reporting on.
  protected running(row: ScheduleRow): boolean {
    return !!row.runId;
  }

  protected pause(row: ScheduleRow): void {
    this.schedules.pause(row.id).subscribe(() => this.subscribe());
  }

  protected resume(row: ScheduleRow): void {
    this.schedules.resume(row.id).subscribe(() => this.subscribe());
  }

  protected runNow(row: ScheduleRow): void {
    this.schedules.runNow(row.id).subscribe(() => {
      this.subscribe();
      this.reload.update((n) => n + 1);
    });
  }

  protected remove(row: ScheduleRow): void {
    this.schedules.remove(row.id).subscribe(() => this.subscribe());
  }
}
