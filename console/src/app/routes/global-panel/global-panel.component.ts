import { Component, computed, inject, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';
import { ApiService, Maintenance, UnavailableReason } from '../../api.service';
import { DialogsService } from '../../shared/dialogs.service';
import { withCurrent } from '../../shared/iso-duration';

// What is true of EVERY route at once.
//
// The routes page is a list of per-route configuration, so the axis is clean:
// this drawer holds what applies across all of them and belongs to none: the
// maintenance switch (LIFE-05), whose whole point is that no route is edited on
// the day everything comes down; how long any route waits for a service before
// it says otherwise; and the ceiling on what a body-rewriting filter may hold.
//
// They used to be two places and a nowhere: a button on this banner, a screen
// in the infra menu, and nothing at all. One drawer, because an operator
// looking for "the things that are not about one route" was looking in three.
//
// The signing keys were here too, and left for a drawer of their own: these
// three are settings, one line each, and keys are a subject.
@Component({
  selector: 'app-global-panel',
  imports: [
    MatButtonModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatSelectModule,
    MatSlideToggleModule,
    MatTooltipModule,
  ],
  styleUrl: './global-panel.component.scss',
  templateUrl: './global-panel.component.html',
})
export class GlobalPanelComponent {
  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);
  private readonly dialogs = inject(DialogsService);

  readonly closed = output<void>();
  // The page's banner carries the badge, so it has to hear about a change made
  // in here.
  readonly maintenanceChanged = output<Maintenance>();

  protected readonly maintenance = signal<Maintenance>({ enabled: false });
  protected readonly switching = signal(false);
  // The reason, and the moment service is expected back. Both are what the
  // page says to visitors, so both are edited here and neither is free text:
  // the wording is the product's, in the visitor's language.
  protected readonly reason = signal<UnavailableReason>('');
  // How long it should take, chosen rather than typed - the same argument as
  // the reason beside it: what a visitor reads is a phrase, and a phrase has
  // to come from a list the product can translate. ISO-8601, because that is
  // what this product already speaks for a duration.
  protected readonly howLong = signal('');
  protected readonly durations: { key: string; label: string }[] = [
    { key: '', label: $localize`:@@Do_not_say_when:Do not say when` },
    { key: 'PT1H', label: $localize`:@@One_hour:1 hour` },
    { key: 'PT2H', label: $localize`:@@Two_hours:2 hours` },
    { key: 'PT4H', label: $localize`:@@Four_hours:4 hours` },
    { key: 'PT8H', label: $localize`:@@Eight_hours:8 hours` },
    { key: 'P1D', label: $localize`:@@One_day:1 day` },
    { key: 'P2D', label: $localize`:@@Two_days:2 days` },
    { key: 'P3D', label: $localize`:@@Three_days:3 days` },
  ];
  protected readonly reasons: { key: UnavailableReason; label: string }[] = [
    { key: '', label: $localize`:@@Say_nothing:Do not say why` },
    { key: 'maintenance', label: $localize`:@@Reason_maintenance:Planned maintenance` },
    { key: 'upgrade', label: $localize`:@@Reason_upgrade:Update being rolled out` },
    { key: 'incident', label: $localize`:@@Reason_incident:Technical incident` },
  ];

  protected readonly bodyRewriteMiB = signal(20);
  // What every route waits unless it says otherwise (ROUTE-07). The same two
  // lists the route editor offers, plus an entry naming the product's own -
  // an installation that never decided still has to see what it is running.
  protected readonly connect = signal('');
  protected readonly response = signal('');
  // The offered list, plus whatever is actually in force when it came from
  // somewhere that does not use this list - the API, a seeded configuration.
  protected readonly connectOffered = computed(() => withCurrent(this.connectChoices, this.connect()));
  protected readonly responseOffered = computed(() => withCurrent(this.responseChoices, this.response()));
  private readonly connectChoices = [
    { key: '', label: $localize`:@@Built_in_5s:Built-in (5 s)` },
    { key: 'PT2S', label: '2 s' },
    { key: 'PT5S', label: '5 s' },
    { key: 'PT10S', label: '10 s' },
    { key: 'PT30S', label: '30 s' },
  ];
  private readonly responseChoices = [
    { key: '', label: $localize`:@@Built_in_15s:Built-in (15 s)` },
    { key: 'PT5S', label: '5 s' },
    { key: 'PT15S', label: '15 s' },
    { key: 'PT30S', label: '30 s' },
    { key: 'PT1M', label: '1 min' },
    { key: 'PT5M', label: '5 min' },
    { key: 'PT10M', label: '10 min' },
  ];
  protected readonly savingLimits = signal(false);
  // The bounds the server enforces anyway, repeated so the field refuses
  // before the round trip. The server stays the one that decides.
  protected readonly minMiB = 1;
  protected readonly maxMiB = 256;

  constructor() {
    this.api.maintenance().subscribe((m) => {
      this.maintenance.set(m);
      this.reason.set(m.reason ?? '');
      this.howLong.set(m.for ?? '');
    });
    this.api.proxyLimits().subscribe((l) => {
      this.bodyRewriteMiB.set(l.bodyRewriteMiB);
      this.connect.set(l.timeouts?.connect ?? '');
      this.response.set(l.timeouts?.response ?? '');
    });
  }

  // How long it has been on, in the words somebody would use out loud. It is
  // the whole reason the server stamps a moment: a maintenance still on two
  // days later is the failure this feature has, and a date nobody reads does
  // not ask about it.
  protected readonly since = () => {
    const s = this.maintenance().since;
    if (!s) return '';
    const mins = Math.max(0, Math.round((Date.now() / 1000 - s) / 60));
    if (mins < 1) return $localize`:@@Just_now:just now`;
    if (mins < 60) return $localize`:@@Since_minutes:since ${mins}:MINUTES: min`;
    const hours = Math.round(mins / 60);
    if (hours < 48) return $localize`:@@Since_hours:since ${hours}:HOURS: h`;
    return $localize`:@@Since_days:since ${Math.round(hours / 24)}:DAYS: days`;
  };

  // Turning it ON asks, and the confirmation says what actually happens rather
  // than "are you sure": every visitor gets an unavailable page at once, on
  // every route. Turning it OFF does not ask - reopening a service is never
  // the dangerous direction.
  protected async setEnabled(on: boolean): Promise<void> {
    if (on) {
      const ok = await this.dialogs.confirm({
        title: $localize`:@@Close_to_visitors_q:Close this installation to visitors?`,
        message: $localize`:@@Close_to_visitors_message:Every route stops serving at once and visitors get the unavailable page. The sign-in pages keep working, and anyone who administers or develops here can still go through - so the fix can be checked before reopening.`,
        confirmLabel: $localize`:@@Close:Close`,
        danger: true,
      });
      if (!ok) return;
    }
    this.write({ enabled: on });
  }

  // Written on the change, like the switch above it: the reason and the moment
  // are what a visitor is reading RIGHT NOW, and a Save button between the
  // choice and the page is a page saying something nobody meant any more.
  protected saveDetails(): void {
    this.write({ enabled: this.maintenance().enabled });
  }

  private write(m: Maintenance): void {
    this.switching.set(true);
    this.api.saveMaintenance({ ...m, reason: this.reason(), for: this.howLong() }).subscribe({
      next: (saved) => {
        this.maintenance.set(saved);
        this.reason.set(saved.reason ?? '');
        this.howLong.set(saved.for ?? '');
        this.switching.set(false);
        this.maintenanceChanged.emit(saved);
        this.snack.open($localize`:@@Saved:Saved`, undefined, { duration: 2000 });
      },
      error: (err: unknown) => {
        this.switching.set(false);
        this.fail(err);
      },
    });
  }

  protected saveLimits(): void {
    this.savingLimits.set(true);
    const timeouts = { connect: this.connect() || undefined, response: this.response() || undefined };
    this.api.saveProxyLimits({ bodyRewriteMiB: Number(this.bodyRewriteMiB()), timeouts }).subscribe({
      next: (saved) => {
        this.bodyRewriteMiB.set(saved.bodyRewriteMiB);
        this.connect.set(saved.timeouts?.connect ?? '');
        this.response.set(saved.timeouts?.response ?? '');
        this.savingLimits.set(false);
        this.snack.open($localize`:@@Saved:Saved`, undefined, { duration: 2000 });
      },
      error: (err: unknown) => {
        this.savingLimits.set(false);
        this.fail(err);
      },
    });
  }

  private fail(err: unknown): void {
    const e = err as { error?: { error?: string } };
    this.snack.open(
      typeof e?.error?.error === 'string' ? e.error.error : $localize`:@@Save_failed:Save failed`,
      undefined,
      { duration: 4000 },
    );
  }
}
