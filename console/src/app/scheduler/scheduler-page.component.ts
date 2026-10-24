import { Component, computed, inject, linkedSignal, signal } from "@angular/core";
import { FormField, form, maxLength } from "@angular/forms/signals";
import { TextFieldModule } from "@angular/cdk/text-field";
import { toSignal } from "@angular/core/rxjs-interop";
import { catchError, of } from "rxjs";
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
import { MatProgressBarModule, ProgressBarMode } from "@angular/material/progress-bar";
import { DialogsService } from "../shared/dialogs.service";
import { MatSelectModule } from "@angular/material/select";
import { MatTableModule } from "@angular/material/table";
import { MatTooltipModule } from "@angular/material/tooltip";
import { LiveWindowDataSource } from "@softwarity/livewire";
import { RowActionsDirective } from "@softwarity/row-actions";
import { ApiService } from "../api.service";
import { FormFieldComponent } from "../shared/form-field.component";
import { SnippetComponent } from "../shared/snippet.component";
import { MeService } from "../me.service";
import { ZonedDatePipe } from "../shared/zoned-date.pipe";
import { SchedulerAccessComponent } from "./scheduler-access.component";
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
    FormField,
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
    SchedulerAccessComponent,
    SchedulerApiComponent,
    SnippetComponent,
    TextFieldModule,
    ZonedDatePipe,
  ],
  templateUrl: "./scheduler-page.component.html",
  styleUrl: "./scheduler-page.component.scss",
})
export class SchedulerPageComponent {
  private readonly schedules = inject(SchedulesService);
  private readonly dialogs = inject(DialogsService);
  private readonly api = inject(ApiService);

  // WHICH HOURS these are: the zone this operator chose on their own profile,
  // UTC when they chose none. The profile is the answer; a second switch here
  // would only ask the same question again.
  protected readonly zone = inject(MeService).timezone;

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
  // route are not what changes while somebody watches a scheduler. A list the
  // viewer may not read (an application admin is refused the routes) is an
  // empty one: toSignal rethrows an error on every read, and one refused list
  // took the whole screen down with it.
  protected readonly users = toSignal(this.api.listUsers().pipe(catchError(() => of([]))), {
    initialValue: [],
  });
  protected readonly tenants = toSignal(this.api.listTenants().pipe(catchError(() => of([]))), {
    initialValue: [],
  });
  protected readonly routes = toSignal(this.api.listRoutes().pipe(catchError(() => of([]))), {
    initialValue: [],
  });

  protected readonly columns = [
    "name",
    "every",
    "next",
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

  // The drawer holds one of three things: a schedule, the explanation of how
  // a service gets one, or who may call that API. Three panels, one surface -
  // and opening one closes the others, because a drawer showing two things is
  // a drawer showing one of them badly.
  protected readonly howto = signal(false);
  protected readonly access = signal(false);

  protected open(row: ScheduleRow): void {
    this.howto.set(false);
    this.access.set(false);
    this.selected.set(row);
  }

  // The schedule the drawer shows, as the live channel has it NOW: a run
  // advancing, its detail, its end - the row clicked is only where it
  // started. Kept as clicked when the list no longer holds it (filtered out).
  protected readonly shown = computed(() => {
    const one = this.selected();
    if (!one) return null;
    return this.all().find((r) => r.id === one.id) ?? one;
  });
  // When the last run of the schedule shown ended: the history is read again
  // when it moves, and not at every progress report.
  private readonly lastEnd = computed(() => this.shown()?.lastAt ?? 0);

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
    if (s.description) out["description"] = s.description;
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
    if (s.retryLost) out["retryLost"] = s.retryLost;
    if (s.metadata && Object.keys(s.metadata).length)
      out["metadata"] = s.metadata;
    return JSON.stringify(out, null, 2);
  });

  // What the schedule is for: the one field the console writes, for the
  // schedules whose service never said. Reset whenever another one opens.
  protected readonly described = linkedSignal(() => ({
    description: this.selected()?.description ?? "",
  }));
  protected readonly describeForm = form(this.described, (p) => {
    maxLength(p.description, 500);
  });
  protected readonly describeDirty = computed(
    () =>
      this.described().description.trim() !==
      (this.selected()?.description ?? ""),
  );

  protected saveDescription(row: ScheduleRow): void {
    if (this.describeForm().invalid()) return;
    this.schedules
      .describe(row.id, this.described().description.trim())
      .subscribe((saved) => {
        const now = this.selected();
        if (now?.id === row.id) {
          this.selected.set({ ...now, description: saved.description });
        }
      });
  }

  // What each turn did (SCHED-03), read when the drawer opens. The row beside
  // it says how the LAST one ended; this is where "since when" lives - and
  // where a turn that never went out can be asked for again.
  private readonly reload = signal(0);
  private readonly runsRes = httpResource<ScheduleRun[]>(() => {
    const one = this.selected();
    this.reload();
    this.lastEnd();
    // A new object each time, so a run that ended reads the history again.
    return one
      ? { url: `/api/schedules/${encodeURIComponent(one.id)}/runs`, params: { limit: 50 } }
      : undefined;
  });
  protected readonly runs = computed(() => this.runsRes.value() ?? []);

  // What this run continues, in a word, when it continues something. A retry
  // says nothing more: its attempt number already does.
  protected chainOf(run: ScheduleRun): string {
    switch (run.cause) {
      case "replay":
        return $localize`:@@Replay_short:replay`;
      case "asked":
        return $localize`:@@Asked_short:called back`;
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

  // What qualifies a run's answer, on one line: how long, which attempt, what
  // it continues. Empty for an ordinary turn that answered at once.
  protected runFacts(run: ScheduleRun): string {
    const facts = [this.took(run), this.chainOf(run)];
    if (run.attempt && run.attempt > 1) {
      facts.splice(1, 0, $localize`:@@Attempt_number:attempt ${run.attempt}:n:`);
    }
    return facts.filter(Boolean).join(" - ");
  }

  protected openHowto(): void {
    this.selected.set(null);
    this.access.set(false);
    this.howto.set(true);
  }

  protected openAccess(): void {
    this.selected.set(null);
    this.howto.set(false);
    this.access.set(true);
  }

  protected closePanel(): void {
    this.selected.set(null);
    this.howto.set(false);
    this.access.set(false);
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

  // What the bar along a row says. Idle and paused are a quiet track; a call
  // waiting for its answer, or work accepted with no progress said yet, moves
  // without a value; accepted work with a progress fills; a lost run waiting
  // to be sent again runs backwards, in the error colour.
  protected barState(row: ScheduleRow): string {
    if (!this.running(row)) return row.paused ? "paused" : "idle";
    return row.runState || "calling";
  }

  protected barMode(row: ScheduleRow): ProgressBarMode {
    switch (this.barState(row)) {
      case "idle":
      case "paused":
        return "determinate";
      case "waiting":
        return "query";
      case "accepted":
        return row.progress > 0 ? "determinate" : "indeterminate";
      default:
        return "indeterminate";
    }
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

  // Confirmed: the service's work stops at its next report, and what it did
  // so far is its own business to keep or undo.
  protected async cancelRun(row: ScheduleRow): Promise<void> {
    const ok = await this.dialogs.confirm({
      title: $localize`:@@Cancel_the_run_of_NAME:Cancel the run of "${row.name}:NAME:"?`,
      message: $localize`:@@Cancel_run_message:The run is closed now. The service is told to stop at its next report; the next turn comes at the schedule's own time.`,
      confirmLabel: $localize`:@@Cancel_run:Cancel run`,
      danger: true,
    });
    if (!ok) return;
    this.schedules.cancelRun(row.id).subscribe(() => this.subscribe());
  }

  // Type-to-confirm: a schedule belongs to the service that wrote it, and the
  // service does not learn it is gone - it is simply never called again.
  // Says whether it was confirmed, so the drawer closes only then.
  protected async remove(row: ScheduleRow): Promise<boolean> {
    const typed = await this.dialogs.prompt({
      title: $localize`:@@Delete_schedule_NAME:Delete schedule "${row.name}:NAME:"?`,
      label: $localize`:@@Type_the_schedule_name_to_confirm:Type the schedule name to confirm`,
      confirmLabel: $localize`:@@Delete:Delete`,
      requireMatch: row.name,
      danger: true,
    });
    if (typed !== row.name) return false;
    this.schedules.remove(row.id).subscribe(() => this.subscribe());
    return true;
  }

  protected async removeAndClose(row: ScheduleRow): Promise<void> {
    if (await this.remove(row)) this.closePanel();
  }
}
