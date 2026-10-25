import { Component, computed, inject, signal } from "@angular/core";
import { HttpErrorResponse } from "@angular/common/http";
import { FormField, form, min, readonly } from "@angular/forms/signals";
import { MatButtonModule } from "@angular/material/button";
import { MatCheckboxChange, MatCheckboxModule } from "@angular/material/checkbox";
import { MatFormFieldModule } from "@angular/material/form-field";
import { MatIconModule } from "@angular/material/icon";
import { MatInputModule } from "@angular/material/input";
import { MatSidenavModule } from "@angular/material/sidenav";
import { MatSlideToggleModule } from "@angular/material/slide-toggle";
import { MatSnackBar } from "@angular/material/snack-bar";
import { MatTableModule } from "@angular/material/table";
import { MatTooltipModule } from "@angular/material/tooltip";
import {
  ApiService,
  NotificationReader,
  NotifySettings,
  NotifySettingsWrite,
  NotifyTenant,
  SentNotification,
} from "../api.service";
import { MeService } from "../me.service";
import { ZonedDatePipe } from "../shared/zoned-date.pipe";
import { SchedulerAccessComponent } from "../scheduler/scheduler-access.component";
import { DialogsService } from "../shared/dialogs.service";
import { NotificationsApiComponent } from "./notifications-api.component";
import { offByText } from "./notify-levels";

// One level's decision on a tenant, while the drawer is open: turned off,
// allowed again, or forced on (what the levels below decided is cleared).
type Pick = "off" | "on" | "force";

// A tenant's checkbox: [x] allowed and received, [-] allowed but turned off
// below, [ ] turned off here, disabled when turned off above.
interface Box {
  checked: boolean;
  indeterminate: boolean;
  disabled: boolean;
  tip: string;
}

// What the services behind the gateway told their users (NOTIF-05).
//
// The services publish on the internal plane; this screen sends nothing. It
// turns the feature on and off - three levels, root, the application admin and
// the tenant admin, each one able only to take away; this drawer holds the
// first two, the tenant's page the third -
// explains how a service publishes (the scheduler's API drawer, for this
// API), says who may publish (the scheduler's Access drawer, aimed at this API),
// and answers "who was told, and who read it": the list of what was sent, and
// a notification's recipients in its drawer, where it can be withdrawn.
@Component({
  selector: "app-notifications-page",
  imports: [
    ZonedDatePipe,
    FormField,
    MatButtonModule,
    MatCheckboxModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatSidenavModule,
    MatSlideToggleModule,
    MatTableModule,
    MatTooltipModule,
    SchedulerAccessComponent,
    NotificationsApiComponent,
  ],
  templateUrl: "./notifications-page.component.html",
  styleUrl: "./notifications-page.component.scss",
})
export class NotificationsPageComponent {
  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);
  private readonly dialogs = inject(DialogsService);
  protected readonly me = inject(MeService);

  protected readonly settings = signal<NotifySettings | null>(null);
  // Why the screen cannot show anything: the community image, mostly.
  protected readonly unavailable = signal("");
  protected readonly list = signal<SentNotification[]>([]);

  protected readonly columns = computed(() =>
    this.settings()?.single
      ? ["title", "sender", "reads", "sent"]
      : ["title", "sender", "tenant", "reads", "sent"],
  );
  private readonly tenantNames = computed(
    () => new Map((this.settings()?.tenants ?? []).map((t) => [t.id, t.name])),
  );
  protected readonly anyOn = computed(() => (this.settings()?.tenants ?? []).some((t) => t.effective));
  // On a single tenant root's one checkbox is its switch for the gateway.
  protected readonly rootSingle = computed(() => !!this.settings()?.single && this.settings()?.level === "root");

  // The drawer holds one thing: the settings, how a service publishes, the
  // access, or a notification.
  protected readonly panel = signal<"" | "settings" | "howto" | "access">("");
  protected readonly selected = signal<SentNotification | null>(null);
  protected readonly readers = signal<NotificationReader[]>([]);

  // The settings as edited: root's switch, the cap, and this level's picks.
  protected readonly draft = signal<{ enabled: boolean; cap: number; picks: Record<string, Pick> }>({
    enabled: false,
    cap: 0,
    picks: {},
  });
  protected readonly f = form(this.draft, (p) => {
    min(p.cap, 0);
    readonly(p.cap, () => !this.settings()?.rootEditable);
  });
  protected readonly saving = signal(false);
  protected readonly problem = signal("");

  constructor() {
    this.load();
  }

  protected load(): void {
    this.api.notifySettings().subscribe({
      next: (s) => {
        this.settings.set(s);
        this.api.sentNotifications().subscribe((l) => this.list.set(l));
      },
      error: (e: HttpErrorResponse) => this.unavailable.set(e.error?.error ?? e.message),
    });
  }

  protected title(n: SentNotification): string {
    return n.title["en"] ?? n.title[""] ?? Object.values(n.title)[0] ?? "";
  }

  protected body(n: SentNotification): string {
    return n.body["en"] ?? n.body[""] ?? Object.values(n.body ?? {})[0] ?? "";
  }

  protected label(l: Record<string, string>): string {
    return l["en"] ?? l[""] ?? Object.values(l)[0] ?? "";
  }

  protected tenantName(id: string): string {
    return this.tenantNames().get(id) ?? id;
  }

  // Live, expired or withdrawn: a word, not a chip.
  protected state(n: SentNotification): "live" | "expired" | "withdrawn" {
    if (n.withdrawnAt) return "withdrawn";
    return n.expiresAt * 1000 < Date.now() ? "expired" : "live";
  }

  protected metaOf(n: SentNotification): string[] {
    return Object.entries(n.meta ?? {}).map(([k, v]) => k + "=" + v);
  }

  protected open(n: SentNotification): void {
    this.panel.set("");
    this.selected.set(n);
    this.readers.set([]);
    this.api.notificationReaders(n.id).subscribe((r) => this.readers.set(r));
  }

  protected openSettings(): void {
    const s = this.settings();
    if (!s) return;
    this.selected.set(null);
    this.problem.set("");
    this.draft.set({ enabled: s.enabled, cap: s.cap || s.defaultCap, picks: {} });
    this.panel.set("settings");
  }

  protected openHowto(): void {
    this.selected.set(null);
    this.panel.set("howto");
  }

  protected openAccess(): void {
    this.selected.set(null);
    this.panel.set("access");
  }

  protected closePanel(): void {
    this.panel.set("");
    this.selected.set(null);
  }

  protected box(t: NotifyTenant): Box {
    const editable = !!this.settings()?.editable;
    if (t.state === "offAbove") {
      return { checked: false, indeterminate: false, disabled: true, tip: offByText(t.offBy) };
    }
    const d = this.draft();
    const pick = d.picks[t.id];
    let mine = pick ? pick !== "off" : t.state !== "off";
    if (this.rootSingle()) mine = mine && d.enabled;
    if (!mine) return { checked: false, indeterminate: false, disabled: !editable, tip: "" };
    // Forcing clears what the levels below decided.
    const below = pick === "force" ? undefined : t.belowBy;
    if (below) return { checked: false, indeterminate: true, disabled: !editable, tip: offByText(below) };
    return { checked: true, indeterminate: false, disabled: !editable, tip: "" };
  }

  // [x] turns off, [ ] allows again, [-] forces on. The checkbox's own toggle
  // is put back to what the decision shows: three states, not two.
  protected flip(t: NotifyTenant, e: MatCheckboxChange): void {
    const before = this.box(t);
    if (!before.disabled) {
      const next: Pick = before.indeterminate ? "force" : before.checked ? "off" : "on";
      this.draft.update((d) => {
        if (this.rootSingle() && next === "off") return { ...d, enabled: false };
        const enabled = this.rootSingle() ? true : d.enabled;
        return { ...d, enabled, picks: { ...d.picks, [t.id]: next } };
      });
    }
    const after = this.box(t);
    e.source.checked = after.checked;
    e.source.indeterminate = after.indeterminate;
  }

  protected setEnabled(on: boolean): void {
    this.draft.update((d) => ({ ...d, enabled: on }));
  }

  protected save(): void {
    const s = this.settings();
    if (!s || this.f().invalid()) return;
    const { enabled, cap, picks } = this.draft();
    const named = (p: Pick) =>
      Object.entries(picks)
        .filter(([, v]) => v === p)
        .map(([id]) => id);
    const write: NotifySettingsWrite = { off: named("off"), on: named("on"), force: named("force") };
    if (s.rootEditable) {
      write.enabled = enabled;
      write.cap = cap === s.defaultCap ? 0 : cap;
    }
    this.saving.set(true);
    this.problem.set("");
    this.api.setNotifySettings(write).subscribe({
      next: (next) => {
        this.saving.set(false);
        this.settings.set(next);
        this.snack.open($localize`:@@Notifications_settings_saved:Notification settings saved`, undefined, {
          duration: 2500,
        });
        this.closePanel();
      },
      error: (e: HttpErrorResponse) => {
        this.saving.set(false);
        this.problem.set(e.error?.error ?? e.message);
      },
    });
  }

  // Confirmed: it leaves every recipient's page and badge at once.
  protected async withdraw(n: SentNotification): Promise<void> {
    const ok = await this.dialogs.confirm({
      title: $localize`:@@Withdraw_notification_TITLE:Withdraw "${this.title(n)}:TITLE:"?`,
      message: $localize`:@@Withdraw_notification_message:It disappears at once for all its recipients. The list of who read it is kept.`,
      confirmLabel: $localize`:@@Withdraw:Withdraw`,
      danger: true,
    });
    if (!ok) return;
    this.api.withdrawNotification(n.id).subscribe(() => {
      this.closePanel();
      this.load();
    });
  }
}
