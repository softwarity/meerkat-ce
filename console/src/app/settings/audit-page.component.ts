import { Component, computed, inject, LOCALE_ID, signal } from '@angular/core';
import { ActivatedRoute } from '@angular/router';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatCardModule } from '@angular/material/card';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatTooltipModule } from '@angular/material/tooltip';
import { LoadingIndicatorComponent } from '@softwarity/loading-indicator';
import { DateTime } from 'luxon';
import { ApiService, AuditChange, AuditEvent, AuditKind, AuditQuery } from '../api.service';
import { EeLockComponent } from '../shared/ee-lock.component';
import { MeService } from '../me.service';
import { LiveChangesService } from '../shared/live-changes.service';

// The audit trail (phase 2): who changed what, with the exact field-level diff.
// Its own transverse section (not under Application). Read-only and scoped
// server-side by capability (RBAC-05): root sees all, infra-admin the routing
// plane, app-admin the identity, a tenant admin their tenants. Target + period
// filter on the server; a free-text box narrows the loaded page.
@Component({
  selector: 'app-audit-page',
  imports: [
    MatButtonModule,
    MatButtonToggleModule,
    MatCardModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatSelectModule,
    MatTooltipModule,
    LoadingIndicatorComponent,
    EeLockComponent,
  ],
  styleUrl: './audit-page.component.scss',
  templateUrl: './audit-page.component.html',
})
export class AuditPageComponent {
  private readonly api = inject(ApiService);
  private readonly locale = inject(LOCALE_ID);

  // The known target kinds, per half (mirrors store.auditTargets). The list
  // follows the half that is shown: offering "route" on the security half is
  // offering a filter that can only answer nothing.
  private readonly adminTargets = [
    'route', 'authprovider', 'certificate', 'theme', 'locale', 'user', 'role', 'schedule', 'settings',
    'issue', 'tenant', 'membership', 'group', 'grouprule', 'vault', 'token', 'configuration', 'config', 'backup',
  ];
  // The target filter is for the changes only: the two sign-in views ARE a
  // target each, chosen by the toggle, and a select offering "route" under
  // "Console" could only answer nothing.
  protected readonly targets = this.adminTargets;

  // Which part of the trail: '' = all of it, 'admin' = the changes, and the
  // security half split by plane - the sign-ins to the applications and the
  // sign-ins to this console are read by different people for different
  // reasons, so they are two views rather than one.
  //
  // Which trail, from the route: under Data plane the sign-ins to the
  // applications and nothing else; under Meerkat everything else - the changes
  // and the sign-ins to this console. The applications' sign-ins are read by
  // the people who run the applications, and they would drown the rest.
  protected readonly trail = (inject(ActivatedRoute).snapshot.data['trail'] as 'data' | 'system') ?? 'system';
  // Under Data plane, two views: the sign-ins, and the calls of the audited
  // operations (AUD-04).
  protected readonly view = signal<'' | 'admin' | 'account' | 'console' | 'endpoint'>(
    this.trail === 'data' ? 'account' : '',
  );

  protected readonly me = inject(MeService);
  protected readonly anonymous = $localize`:@@Anonymous:anonymous`;
  // How long the trail keeps an event (AUD-02) - root's alone, so only root
  // is shown the choice.
  protected readonly retention = signal(0);
  protected readonly retentionChoices = signal<number[]>([]);
  // The audited calls' own lifetime, and what a full queue had to drop.
  protected readonly callRetention = signal(0);
  protected readonly callChoices = signal<number[]>([]);
  protected readonly callsDropped = signal(0);
  protected readonly loading = signal(true);
  protected readonly events = signal<AuditEvent[]>([]);
  protected readonly target = signal('');
  protected readonly period = signal(7); // days; 0 = all time
  protected readonly search = signal('');

  protected readonly filtered = computed(() => {
    const q = this.search().trim().toLowerCase();
    if (!q) return this.events();
    return this.events().filter((e) =>
      [e.action, e.actorName, e.actorId, e.actorToken, e.target, e.targetName, e.detail, e.ip,
        e.data?.path, e.data?.route, ...Object.values(e.data?.fields ?? {})]
        .some((v) => (v ?? '').toLowerCase().includes(q)),
    );
  });

  constructor() {
    this.reload();
    if (this.me.isRoot()) {
      this.api.auditSettings().subscribe({
        next: (s) => {
          this.retention.set(s.retentionDays);
          this.retentionChoices.set(s.choices);
          this.callRetention.set(s.endpointRetentionDays);
          this.callChoices.set(s.endpointChoices);
          this.callsDropped.set(s.endpointDropped);
        },
      });
    }
    // EVERY write, not one kind (CONSOLE-13): what this screen shows IS the
    // writes, so anything anybody does anywhere is its news. Quietly - the
    // trail grows while somebody is reading it, and a spinner in its place
    // would take away the line they were on.
    inject(LiveChangesService).onAny(() => this.reload(true));
  }

  protected reload(quiet = false): void {
    if (!quiet) this.loading.set(true);
    this.api.listAudit({ ...this.query(), limit: 500 }).subscribe({
      next: (events) => {
        this.events.set(events);
        this.loading.set(false);
      },
      error: () => {
        this.events.set([]);
        this.loading.set(false);
      },
    });
  }

  protected setRetention(days: number): void {
    const before = this.retention();
    this.retention.set(days);
    this.api.setAuditRetention({ retentionDays: days }).subscribe({ error: () => this.retention.set(before) });
  }

  protected setCallRetention(days: number): void {
    const before = this.callRetention();
    this.callRetention.set(days);
    this.api.setAuditRetention({ endpointRetentionDays: days }).subscribe({ error: () => this.callRetention.set(before) });
  }

  protected retentionLabel(days: number): string {
    if (days < 30) return $localize`:@@N_days:${days}:n: days`;
    if (days % 365 === 0) {
      const years = days / 365;
      return years === 1 ? $localize`:@@One_year:1 year` : $localize`:@@N_years:${years}:n: years`;
    }
    const months = Math.round(days / 30);
    return months === 1 ? $localize`:@@One_month:1 month` : $localize`:@@N_months:${months}:n: months`;
  }

  // The file: the filters on screen, the perimeter of the caller.
  protected exportCsv(): void {
    const a = document.createElement('a');
    a.href = this.api.auditExportUrl(this.query());
    a.download = 'meerkat-audit.csv';
    a.click();
  }

  private query(): AuditQuery {
    const days = this.period();
    const since = days > 0 ? Math.floor(Date.now() / 1000) - days * 86400 : undefined;
    const view = this.view();
    const kind: AuditKind | undefined =
      view === '' || view === 'endpoint' ? undefined : view === 'admin' ? 'admin' : 'security';
    const target =
      view === 'account' || view === 'console' || view === 'endpoint'
        ? view
        : view === 'admin'
          ? this.target() || undefined
          : undefined;
    // Meerkat's "All" is every kind but the data plane's: its sign-ins and
    // its audited calls.
    const notTarget = view === '' ? 'account,endpoint' : undefined;
    return { kind, target, notTarget, since };
  }

  protected setView(view: '' | 'admin' | 'account' | 'console' | 'endpoint'): void {
    this.view.set(view);
    this.target.set('');
    this.reload();
  }

  // An event of the security half: an account's own sign-in or a way into it,
  // rather than somebody changing the configuration.
  protected security(e: AuditEvent): boolean {
    return e.target === 'account' || e.target === 'console';
  }

  // A door that stayed shut: a refused sign-in, the lock-out after them, or
  // an audited call that was not served.
  protected refused(e: AuditEvent): boolean {
    return e.action === 'signin.refused' || e.action === 'signin.locked' || (e.data?.status ?? 0) >= 400;
  }

  // The fields an audited call carries, in a stable order.
  protected fieldsOf(e: AuditEvent): [string, string][] {
    return Object.entries(e.data?.fields ?? {}).sort(([a], [b]) => a.localeCompare(b));
  }

  // The body as it was sent, laid out to be read.
  protected bodyOf(e: AuditEvent): string {
    const b = e.data?.body ?? '';
    try {
      return JSON.stringify(JSON.parse(b), null, 2);
    } catch {
      return b;
    }
  }

  protected relWhen(at: number): string {
    return DateTime.fromSeconds(at).reconfigure({ locale: this.locale }).toRelative() ?? '';
  }

  protected fullWhen(at: number): string {
    return DateTime.fromSeconds(at).reconfigure({ locale: this.locale }).toLocaleString(DateTime.DATETIME_MED);
  }

  // A change value as a short readable string: empty/absent shows a placeholder,
  // objects/arrays as compact JSON, everything else as-is.
  protected fmt(v: AuditChange['from']): string {
    if (v === undefined || v === null || v === '') return '∅';
    if (typeof v === 'object') return JSON.stringify(v);
    return String(v);
  }
}
