import { Component, computed, inject, LOCALE_ID, signal } from '@angular/core';
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
  protected readonly view = signal<'' | 'admin' | 'account' | 'console'>('');

  protected readonly me = inject(MeService);
  // How long the trail keeps an event (AUD-02) - root's alone, so only root
  // is shown the choice.
  protected readonly retention = signal(0);
  protected readonly retentionChoices = signal<number[]>([]);
  protected readonly loading = signal(true);
  protected readonly events = signal<AuditEvent[]>([]);
  protected readonly target = signal('');
  protected readonly period = signal(7); // days; 0 = all time
  protected readonly search = signal('');

  protected readonly filtered = computed(() => {
    const q = this.search().trim().toLowerCase();
    if (!q) return this.events();
    return this.events().filter((e) =>
      [e.action, e.actorName, e.actorId, e.actorToken, e.target, e.targetName, e.detail, e.ip]
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
    const days = this.period();
    const since = days > 0 ? Math.floor(Date.now() / 1000) - days * 86400 : undefined;
    const view = this.view();
    const kind: AuditKind | undefined = view === '' ? undefined : view === 'admin' ? 'admin' : 'security';
    const target = view === 'account' || view === 'console' ? view : view === 'admin' ? this.target() || undefined : undefined;
    this.api.listAudit({ kind, target, since, limit: 500 }).subscribe({
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
    this.api.setAuditRetention(days).subscribe({ error: () => this.retention.set(before) });
  }

  protected retentionLabel(days: number): string {
    if (days % 365 === 0) {
      const years = days / 365;
      return years === 1 ? $localize`:@@One_year:1 year` : $localize`:@@N_years:${years}:n: years`;
    }
    return $localize`:@@N_months:${Math.round(days / 30)}:n: months`;
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
    const kind: AuditKind | undefined = view === '' ? undefined : view === 'admin' ? 'admin' : 'security';
    const target = view === 'account' || view === 'console' ? view : view === 'admin' ? this.target() || undefined : undefined;
    return { kind, target, since };
  }

  protected setView(view: '' | 'admin' | 'account' | 'console'): void {
    this.view.set(view);
    this.target.set('');
    this.reload();
  }

  // What a security line's kind means to the person reading it.
  protected plane(e: AuditEvent): string {
    return e.target === 'console' ? $localize`:@@Audit_plane_console:console` : $localize`:@@Audit_plane_data:data plane`;
  }

  // An event of the security half: an account's own sign-in or a way into it,
  // rather than somebody changing the configuration.
  protected security(e: AuditEvent): boolean {
    return e.target === 'account' || e.target === 'console';
  }

  // A door that stayed shut: a refused sign-in, or the lock-out after them.
  protected refused(e: AuditEvent): boolean {
    return e.action === 'signin.refused' || e.action === 'signin.locked';
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
