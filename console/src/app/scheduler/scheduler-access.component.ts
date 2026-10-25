import { Component, computed, effect, inject, input, output, signal, untracked, viewChild } from "@angular/core";
import { HttpErrorResponse, httpResource } from "@angular/common/http";
import { COMMA, ENTER } from "@angular/cdk/keycodes";
import { FormField, disabled, form } from "@angular/forms/signals";
import {
  MatAutocompleteModule,
  MatAutocompleteSelectedEvent,
  MatAutocompleteTrigger,
} from "@angular/material/autocomplete";
import { MatButtonModule } from "@angular/material/button";
import { MatChipInputEvent, MatChipsModule } from "@angular/material/chips";
import { MatFormFieldModule } from "@angular/material/form-field";
import { MatIconModule } from "@angular/material/icon";
import { MatRadioModule } from "@angular/material/radio";
import { RouterLink } from "@angular/router";
import { MatSnackBar } from "@angular/material/snack-bar";
import {
  ApiService,
  InternalAccess,
  InternalAccessCandidates,
  InternalAccessMode,
} from "../api.service";

// Who may call the scheduler's API on the internal plane (SCHED-01), in the
// Scheduler's drawer: HOW a caller is recognised, then WHO gets in.
//
// The two answers are kept apart because they are two decisions. The mode
// says what a service has to present - nothing, a Meerkat token, its pod's
// ServiceAccount token - and the list is compared with the name that mode
// gives it. Each mode has its own list, kept while somebody switches between
// them: a token's name and a workload are not the same kind of name, and a
// list carried from one mode to the other would admit nobody.
//
// What neither decides is said under them, in one line, because it is the
// half people assume: the roles a task calls with are the ones it asks for,
// and the route it calls can be anybody's.
//
// An infrastructure administrator changes it; the Scheduler's other readers
// see it read-only. Read when the drawer opens: a form never reloads under
// somebody, and the save answers with what was stored.
@Component({
  selector: "app-scheduler-access",
  imports: [
    FormField,
    MatAutocompleteModule,
    MatButtonModule,
    MatChipsModule,
    MatFormFieldModule,
    MatIconModule,
    MatRadioModule,
    RouterLink,
  ],
  templateUrl: "./scheduler-access.component.html",
  styleUrl: "./scheduler-access.component.scss",
})
export class SchedulerAccessComponent {
  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);

  // Which API of the internal plane this drawer guards: the scheduler's, or
  // the notifications' (NOTIF-05), each with its own mode and list.
  readonly target = input<"schedules" | "notifications">("schedules");

  // Said when the drawer may close: saved, or put down.
  readonly closed = output<void>();

  // What the gateway holds, as it answered.
  protected readonly stored = signal<InternalAccess | null>(null);
  protected readonly editable = computed(() => this.stored()?.editable ?? false);
  protected readonly modes = computed<InternalAccessMode[]>(
    () => this.stored()?.modes ?? ["none", "token"],
  );

  // The two choices, as a form; the lists beside them, one per mode.
  protected readonly draft = signal<{ mode: InternalAccessMode; who: "everyone" | "only" }>({
    mode: "none",
    who: "everyone",
  });
  protected readonly f = form(this.draft, (p) => {
    disabled(p, () => !this.editable());
  });
  private readonly lists = signal<Record<InternalAccessMode, string[]>>({
    none: [],
    token: [],
    cluster: [],
  });
  protected readonly callers = computed(() => this.lists()[this.draft().mode]);

  protected readonly saving = signal(false);
  protected readonly problem = signal("");

  constructor() {
    effect(() => {
      const target = this.target();
      untracked(() =>
        (target === "notifications" ? this.api.notifyAccess() : this.api.internalAccess()).subscribe({
          next: (a) => this.take(a),
          error: (e: HttpErrorResponse) => this.problem.set(e.error?.error ?? e.message),
        }),
      );
    });
  }

  private take(a: InternalAccess): void {
    this.stored.set(a);
    this.draft.set({ mode: a.mode, who: a.callers.length ? "only" : "everyone" });
    this.lists.set({ none: [], token: [], cluster: [], [a.mode]: [...a.callers] });
  }

  // What the mode's list can offer, asked when the mode changes.
  private readonly candidatesRes = httpResource<InternalAccessCandidates>(() =>
    this.stored()
      ? `/api/settings/internal-access/candidates?mode=${this.draft().mode}` +
        (this.target() === "notifications" ? "&for=notifications" : "")
      : undefined,
  );
  protected readonly unavailable = computed(() => this.candidatesRes.value()?.unavailable ?? "");
  protected readonly query = signal("");
  protected readonly options = computed(() => {
    const q = this.query().trim().toLowerCase();
    const taken = new Set(this.callers());
    return (this.candidatesRes.value()?.candidates ?? []).filter(
      (c) => !taken.has(c) && (!q || c.toLowerCase().includes(q)),
    );
  });

  // The list's label and its one-line help, in the words of the mode.
  protected readonly listLabel = computed(() => {
    switch (this.draft().mode) {
      case "token":
        return $localize`:@@Access_list_tokens:Tokens`;
      case "cluster":
        return $localize`:@@Access_list_service_accounts:ServiceAccounts`;
      default:
        return $localize`:@@Access_list_services:Services`;
    }
  });
  protected readonly listHint = computed(() => {
    switch (this.draft().mode) {
      case "token":
        return $localize`:@@Access_list_tokens_hint:Internal tokens, by name`;
      case "cluster":
        return $localize`:@@Access_list_sa_hint:namespace/serviceaccount`;
      default:
        return $localize`:@@Access_list_services_hint:namespace/Kind/name, or a Docker service`;
    }
  });

  // None and everyone: the one combination whose consequence nobody guesses.
  protected readonly wideOpen = computed(
    () => this.draft().mode === "none" && this.draft().who === "everyone",
  );

  protected readonly dirty = computed(() => {
    const s = this.stored();
    if (!s) return false;
    const now = this.payload();
    return now.mode !== s.mode || now.callers.join("\n") !== s.callers.join("\n");
  });
  protected readonly canSave = computed(
    () =>
      this.editable() &&
      this.dirty() &&
      !this.saving() &&
      (this.draft().who === "everyone" || this.callers().length > 0),
  );

  private payload(): { mode: InternalAccessMode; callers: string[] } {
    const { mode, who } = this.draft();
    return { mode, callers: who === "only" ? this.callers() : [] };
  }

  protected readonly separators = [ENTER, COMMA];
  private readonly trigger = viewChild(MatAutocompleteTrigger);

  protected add(event: MatChipInputEvent): void {
    // Enter on a highlighted suggestion reaches both the autocomplete and the
    // chip input: that key is pick()'s, which adds the whole name.
    if (this.trigger()?.panelOpen && this.trigger()?.activeOption) return;
    this.push(event.value);
    event.chipInput.clear();
    this.query.set("");
  }

  protected pick(event: MatAutocompleteSelectedEvent, input: HTMLInputElement): void {
    this.push(event.option.value as string);
    input.value = "";
    this.query.set("");
    event.option.deselect();
  }

  private push(raw: string): void {
    const name = raw.trim();
    const mode = this.draft().mode;
    if (!name || this.lists()[mode].includes(name)) return;
    this.lists.update((l) => ({ ...l, [mode]: [...l[mode], name] }));
  }

  protected remove(name: string): void {
    const mode = this.draft().mode;
    this.lists.update((l) => ({ ...l, [mode]: l[mode].filter((c) => c !== name) }));
  }

  protected save(): void {
    if (!this.canSave()) return;
    const { mode, callers } = this.payload();
    this.saving.set(true);
    this.problem.set("");
    const notifications = this.target() === "notifications";
    (notifications ? this.api.setNotifyAccess(mode, callers) : this.api.setInternalAccess(mode, callers)).subscribe({
      next: (a) => {
        this.saving.set(false);
        this.take(a);
        this.snack.open(
          notifications
            ? $localize`:@@Access_notifications_saved:Who may publish notifications saved`
            : $localize`:@@Access_saved:Access to the scheduler saved`,
          undefined,
          { duration: 2500 },
        );
        this.closed.emit();
      },
      error: (e: HttpErrorResponse) => {
        this.saving.set(false);
        this.problem.set(e.error?.error ?? e.message);
      },
    });
  }
}
