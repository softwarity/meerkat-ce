import { Component, computed, effect, inject, signal, untracked } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { toSignal } from '@angular/core/rxjs-interop';
import { MatIconModule } from '@angular/material/icon';
import { MatSidenavModule } from '@angular/material/sidenav';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTableModule } from '@angular/material/table';
import { MatTooltipModule } from '@angular/material/tooltip';
import { ActivatedRoute, Router } from '@angular/router';
import { LoadingIndicatorComponent } from '@softwarity/loading-indicator';
import { RowActionsDirective } from '@softwarity/row-actions';
import { ApiService, AuthProvider, Group, GroupRule } from '../../api.service';
import { DialogsService } from '../../shared/dialogs.service';
import { TenantScope } from '../tenant-scope';
import { RuleEditorComponent } from './rule-editor.component';

// The rules that turn what an authority says into membership of THIS
// organisation and groups within it (RBAC-10).
//
// The screen belongs to the organisation on purpose. An authority is
// infrastructure and only proves who someone is; who that lets into which
// organisation, with which groups, is the organisation's own call. So these
// rules are written here, by its administrators, and cannot reach anywhere
// else.
//
// A rule opens in the right drawer, and the URL says which (.../rules/<id>,
// .../rules/new): a refresh or a pasted link comes back to it, like routes,
// roles and authorities. The same screen serves two addresses - an
// organisation's rules, and Application's in single-organisation mode - so
// the drawer's address is built from the one it was reached by.
@Component({
  selector: 'app-tenant-rules',
  imports: [
    MatButtonModule,
    MatIconModule,
    MatSidenavModule,
    MatTableModule,
    MatTooltipModule,
    LoadingIndicatorComponent,
    RowActionsDirective,
    RuleEditorComponent,
  ],
  templateUrl: './tenant-rules.component.html',
  styleUrl: './tenant-rules.component.scss',
})
export class TenantRulesComponent {
  protected readonly scope = inject(TenantScope);
  private readonly api = inject(ApiService);
  private readonly router = inject(Router);
  private readonly ar = inject(ActivatedRoute);
  private readonly dialogs = inject(DialogsService);
  private readonly snack = inject(MatSnackBar);

  protected readonly loading = signal(true);
  protected readonly rules = signal<GroupRule[]>([]);
  protected readonly groups = signal<Group[]>([]);
  protected readonly authorities = signal<AuthProvider[]>([]);
  protected readonly reported = signal<string[]>([]);
  protected readonly columns = ['when', 'grants'];

  // The URL drives the drawer: the segment after rules, when there is one.
  private readonly params = toSignal(this.ar.paramMap);
  protected readonly editing = computed<GroupRule | 'new' | null>(() => {
    const id = this.params()?.get('rule');
    if (!id) return null;
    if (id === 'new') return 'new';
    return this.rules().find((r) => r.id === id) ?? null;
  });
  protected readonly editingRule = computed(() => {
    const e = this.editing();
    return e === null || e === 'new' ? null : e;
  });
  protected readonly enabledAuthorities = computed(() => this.authorities().filter((a) => a.enabled));

  // Nothing to map before an authority exists, and nothing to grant before the
  // organisation has a group: say which one is missing rather than showing an
  // empty table with a button that leads nowhere.
  protected readonly blocker = computed(() => {
    if (!this.authorities().some((a) => a.enabled)) return 'authority';
    return this.groups().length === 0 ? 'group' : '';
  });

  constructor() {
    effect(() => {
      const t = this.scope.tenant();
      if (t) untracked(() => this.load(t.id));
    });
  }

  private load(tenantId: string): void {
    this.loading.set(true);
    this.api.listGroupRules(tenantId).subscribe({
      next: (list) => {
        this.rules.set(list);
        this.loading.set(false);
      },
      error: () => this.loading.set(false),
    });
    this.api.listGroups(tenantId).subscribe({ next: (g) => this.groups.set(g) });
    this.api.reportedGroups(tenantId).subscribe({ next: (n) => this.reported.set(n) });
    // An app admin may not list the authorities; the screen then simply offers
    // no authority filter rather than failing.
    this.api.authProviders().subscribe({
      next: (list) => this.authorities.set(list),
      error: () => this.authorities.set([]),
    });
  }

  // The rules screen's own address, whichever of the two it was reached by.
  private base(): string {
    const path = this.router.url.split(/[?#]/)[0];
    const m = /^(.*\/(?:group-)?rules)(?:\/[^/]+)?$/.exec(path);
    return m ? m[1] : path;
  }

  protected open(rule: GroupRule): void {
    void this.router.navigateByUrl(`${this.base()}/${encodeURIComponent(rule.id)}`);
  }

  protected openNew(): void {
    void this.router.navigateByUrl(`${this.base()}/new`);
  }

  protected closeEditor(): void {
    if (this.editing() !== null) void this.router.navigateByUrl(this.base());
  }

  protected save(result: Partial<GroupRule>): void {
    const t = this.scope.tenant();
    if (!t) return;
    const editing = this.editingRule();
    this.api.saveGroupRule(t.id, { ...result, id: editing?.id }).subscribe({
      next: () => {
        this.load(t.id);
        this.closeEditor();
      },
      error: (err: unknown) => this.fail(err),
    });
  }

  protected async remove(rule: GroupRule): Promise<void> {
    const t = this.scope.tenant();
    if (!t) return;
    const ok = await this.dialogs.confirm({
      title: $localize`:@@Delete_rule:Delete this rule?`,
      // Saying what actually happens: nothing changes until people sign in
      // again, which is the single most surprising part of the mechanism.
      message: $localize`:@@Delete_rule_hint:What it granted is taken back at each person's next sign-in. Manual placements stay.`,
      confirmLabel: $localize`:@@Delete:Delete`,
      danger: true,
    });
    if (!ok) return;
    this.api.deleteGroupRule(t.id, rule.id).subscribe({
      next: () => {
        if (this.editingRule()?.id === rule.id) this.closeEditor();
        this.load(t.id);
      },
      error: (err: unknown) => this.fail(err),
    });
  }

  // The left half of a row: what has to be true for the rule to apply.
  protected when(rule: GroupRule): string {
    if (!rule.external) {
      return $localize`:@@Anyone_from_AUTHORITY:Anyone from ${rule.providerName || rule.providerId}:AUTHORITY:`;
    }
    return rule.external;
  }

  private fail(err: unknown): void {
    const e = err as { error?: { error?: string } };
    this.snack.open(
      typeof e?.error?.error === 'string' ? e.error.error : $localize`:@@Save_failed:Save failed`,
      undefined,
      { duration: 6000 },
    );
  }
}
