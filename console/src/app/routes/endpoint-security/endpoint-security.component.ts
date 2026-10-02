import { HttpErrorResponse } from '@angular/common/http';
import { Component, computed, inject, signal, viewChild } from '@angular/core';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';
import { MatButtonModule } from '@angular/material/button';
import { MatExpansionModule } from '@angular/material/expansion';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatSelectModule } from '@angular/material/select';
import { MatSidenavModule } from '@angular/material/sidenav';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatSort, MatSortModule, Sort } from '@angular/material/sort';
import { MatTable, MatTableModule } from '@angular/material/table';
import { MatTooltipModule } from '@angular/material/tooltip';
import { toSignal } from '@angular/core/rxjs-interop';
import { ActivatedRoute, RouterLink } from '@angular/router';
import { sessionStored } from '@softwarity/store';
import { Subject, catchError, debounceTime, firstValueFrom, map, of } from 'rxjs';
import {
  AUDIT_FILE_KIND,
  Access,
  ApiService,
  AuditField,
  AuditFile,
  EndpointAudit,
  EndpointPolicy,
  OpenAPIOperation,
  RateLimit,
  Role,
  Route,
  RouteOperations,
  RouteSecurity,
  Tenant,
  User,
} from '../../api.service';
import { AccessBadgesComponent } from './access-badges.component';
import { AccessEditorComponent, AccessState, emptyAccess, isEmpty } from './access-editor.component';
import { RateLimitsComponent, limitLabel, limitScope, limitScopeTip } from '../rate-limits.component';
import { MatInputModule } from '@angular/material/input';

// The three questions one screen answers, one per menu entry: who may call an
// operation, how much it may carry, and whether its calls are audited.
type Intent = 'security' | 'limits' | 'audit';

// One operation's editable state: whether it overrides the route-wide default,
// and (when it does) its own access rule.
interface OpState {
  override: boolean;
  access: AccessState;
  // What this one operation may carry (QUOTA-05). A SEPARATE axis from the
  // access override: the expensive report of an otherwise open API needs its
  // own ceiling without any rule about who may call it, and forcing the two
  // together would make somebody pose a security rule to write a bound.
  limits: RateLimit[];
}

function opKey(method: string, path: string): string {
  return `${method.toUpperCase()} ${path}`;
}

// Conventional verb order for the method sort.
const METHOD_ORDER = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS'];
function methodRank(m: string): number {
  const i = METHOD_ORDER.indexOf(m.toUpperCase());
  return i < 0 ? METHOD_ORDER.length : i;
}

// Turn the editor's non-optional access into the wire shape (a delegated level
// and empty lists are simply omitted).
function toPolicy(method: string, path: string, s: OpState): EndpointPolicy {
  const ep: EndpointPolicy = { method: method.toUpperCase(), path };
  // The access half is only written when the operation actually overrides:
  // an operation posed for its bound alone must not silently acquire a rule.
  if (s.override) {
    const a = s.access;
    if (a.level) ep.level = a.level;
    if (a.tenants.length) ep.tenants = a.tenants;
    if (a.roles.length) ep.roles = a.roles;
    if (a.users.length) ep.users = a.users;
  } else {
    // Said, not implied: an entry with no access fields would READ as the
    // deliberate reopening to the upstream, and a bound alone would open the
    // operation to anyone.
    ep.inherit = true;
  }
  if (s.limits.length) ep.limits = s.limits;
  return ep;
}

function fromWire(a: Access | undefined): AccessState {
  return {
    level: a?.level ?? '',
    tenants: a?.tenants ?? [],
    roles: a?.roles ?? [],
    users: a?.users ?? [],
  };
}

// The Endpoints screen (RBAC-07, QUOTA-05): a dedicated Gateway page. Pick a route that
// exposes an OpenAPI spec; its operations load in a table (sticky header,
// scrolling rows, global Save in the footer). One access rule (authenticated /
// users / roles) is set for the WHOLE route in the header, and any operation
// can override it by expanding its row. The spec is fetched and parsed
// SERVER-SIDE, so this screen only ever sees a flat operation list. Saving PUTs
// the assembled security to the admin API, which validates by compiling and
// reloads the data plane (saving IS applying).
@Component({
  selector: 'app-endpoint-security',
  imports: [
    RouterLink,
    MatButtonModule,
    MatExpansionModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatProgressBarModule,
    MatSelectModule,
    MatSidenavModule,
    MatSlideToggleModule,
    MatSortModule,
    MatTableModule,
    MatTooltipModule,
    AccessEditorComponent,
    AccessBadgesComponent,
    RateLimitsComponent,
  ],
  templateUrl: './endpoint-security.component.html',
  styleUrl: './endpoint-security.component.scss',
})
export class EndpointSecurityComponent {
  private readonly api = inject(ApiService);
  private readonly table = viewChild(MatTable);

  protected readonly loadingRoutes = signal(true);
  protected readonly loadingOps = signal(false);
  protected readonly error = signal('');
  // Auto-save: every change persists on its own (debounced); the footer shows
  // the state rather than a Save button.
  protected readonly saveState = signal<'idle' | 'saving' | 'saved' | 'error'>('idle');
  protected readonly saveError = signal('');
  private readonly saveTrigger = new Subject<void>();
  protected readonly routes = signal<Route[]>([]);
  protected readonly roles = signal<Role[]>([]);
  protected readonly users = signal<User[]>([]);
  protected readonly tenants = signal<Tenant[]>([]);
  protected readonly selectedId = signal('');
  protected readonly data = signal<RouteOperations | null>(null);

  // The route-wide default rule (applies to every operation with no override).
  protected readonly routeAccess = signal<AccessState>(emptyAccess());
  // Only what a rule lists is reachable (RBAC-07): an operation with no rule
  // of its own is then closed to everyone rather than gated by the route.
  protected readonly denyUnlisted = signal(false);

  // Per-operation edits, keyed by opKey.
  private readonly state = signal<Record<string, OpState>>({});
  // Saved overrides that match no listed operation: kept so a save never
  // silently drops policy.
  private readonly extras = signal<EndpointPolicy[]>([]);
  // Which operation the drawer is showing, and which of its two sections was
  // aimed at. One at a time: a drawer is a place, not a stack.
  // Which of the two questions this visit is about. It decides what the title
  // says, what the table leads with, and which half of the drawer opens - not
  // what is reachable: both halves are always there, or an entry would be a
  // dead end for the other question.
  protected readonly intent = toSignal(
    inject(ActivatedRoute).data.pipe(
      map((d) => (d['intent'] === 'limits' || d['intent'] === 'audit' ? d['intent'] : 'security') as Intent),
    ),
    { initialValue: 'security' as Intent },
  );

  // Endpoint audit (AUD-04), per operation, keyed like the rest. Kept apart
  // from the access state on purpose: auditing observes, it decides nothing.
  private readonly audits = signal<Record<string, EndpointAudit>>({});
  // Audited operations the spec no longer declares: kept on save.
  private readonly auditExtras = signal<EndpointAudit[]>([]);
  protected readonly auditedCount = computed(() => Object.keys(this.audits()).length);
  // Whether audit events leave at all: the OpenTelemetry Audit switch governs
  // every audit, these operations' included. Read once, for the warning.
  protected readonly auditSent = toSignal(
    this.api.telemetrySetting().pipe(
      map((t) => !!(t.enabled && t.audit)),
      catchError(() => of(true)),
    ),
    { initialValue: true },
  );
  protected readonly auditSources: { value: AuditField['from']; label: string }[] = [
    { value: 'path', label: $localize`:@@Audit_from_path:Path variable` },
    { value: 'query', label: $localize`:@@Audit_from_query:Query parameter` },
    { value: 'header', label: $localize`:@@Audit_from_header:Header` },
    { value: 'body', label: $localize`:@@Audit_from_body:Body (JSON pointer)` },
  ];

  protected auditOf(o: OpenAPIOperation): EndpointAudit | undefined {
    return this.audits()[opKey(o.method, o.path)];
  }

  // On: pre-filled with the operation's summary, which is what the event says.
  protected setAudited(o: OpenAPIOperation, on: boolean): void {
    const k = opKey(o.method, o.path);
    this.audits.update((a) => {
      const next = { ...a };
      if (on) next[k] = { method: o.method.toUpperCase(), path: o.path, description: o.summary ?? '' };
      else delete next[k];
      return next;
    });
    this.scheduleSave();
  }

  protected patchAudit(o: OpenAPIOperation, patch: Partial<EndpointAudit>): void {
    const k = opKey(o.method, o.path);
    const cur = this.audits()[k];
    if (!cur) return;
    this.audits.update((a) => ({ ...a, [k]: { ...cur, ...patch } }));
    this.scheduleSave();
  }

  protected addAuditField(o: OpenAPIOperation): void {
    const fields = [...(this.auditOf(o)?.fields ?? []), { name: '', from: 'path' as const, key: '' }];
    this.patchAudit(o, { fields });
  }

  protected setAuditField(o: OpenAPIOperation, i: number, patch: Partial<AuditField>): void {
    const fields = (this.auditOf(o)?.fields ?? []).map((f, j) => (j === i ? { ...f, ...patch } : f));
    this.patchAudit(o, { fields });
  }

  protected removeAuditField(o: OpenAPIOperation, i: number): void {
    this.patchAudit(o, { fields: (this.auditOf(o)?.fields ?? []).filter((_, j) => j !== i) });
  }

  protected setAuditMask(o: OpenAPIOperation, text: string): void {
    const mask = text
      .split(',')
      .map((m) => m.trim())
      .filter((m) => m);
    this.patchAudit(o, { mask });
  }

  protected readonly defaultMask = 'password, passwd, secret, token, apiKey, api_key, authorization, cookie';
  // Which operation the drawer is showing. Which SECTION is no longer a
  // question: the page decides, and the drawer carries the one the page is
  // about.
  protected readonly openKey = signal<string>('');

  protected readonly apiRoutes = computed(() => this.routes().filter((r) => !!r.api?.spec));
  protected readonly operations = computed(() => this.data()?.operations ?? []);
  protected readonly columns = ['status', 'method', 'path', 'tags', 'summary', 'expand'];

  protected readonly openOp = computed(() =>
    this.operations().find((o) => opKey(o.method, o.path) === this.openKey()) ?? null,
  );

  // Distinct tags across the spec, for the column-header filter.
  protected readonly allTags = computed(() => {
    const set = new Set<string>();
    for (const o of this.operations()) for (const t of o.tags ?? []) set.add(t);
    return [...set].sort((a, b) => a.localeCompare(b));
  });
  // Sort + method/tag filters persisted in sessionStorage: they survive a page
  // refresh (not the session). $prop() signals drive the template and computeds.
  protected readonly view = sessionStored(
    {
      sortActive: '',
      sortDir: '' as '' | 'asc' | 'desc',
      methods: [] as string[],
      tags: [] as string[],
      routeOpen: true, // the "whole route" panel is expanded by default
    },
    { storageKey: 'endpoint-security-view.v3' },
  );

  // Distinct methods present, in conventional verb order, for the header filter.
  protected readonly allMethods = computed(() => {
    const set = new Set<string>();
    for (const o of this.operations()) set.add(o.method.toUpperCase());
    return [...set].sort((a, b) => methodRank(a) - methodRank(b));
  });

  protected setTagFilter(tags: string[]): void {
    this.view.tags = tags;
    this.table()?.renderRows();
  }
  protected setMethodFilter(methods: string[]): void {
    this.view.methods = methods;
    this.table()?.renderRows();
  }
  protected onSort(s: Sort): void {
    this.view.sortActive = s.active;
    this.view.sortDir = s.direction;
    this.table()?.renderRows();
  }
  // The rows the table shows: filtered by method and tags, then sorted (path only).
  protected readonly sortedOps = computed(() => {
    const methods = this.view.$methods();
    const tags = this.view.$tags();
    let ops = this.operations();
    if (methods.length) ops = ops.filter((o) => methods.includes(o.method.toUpperCase()));
    if (tags.length) ops = ops.filter((o) => (o.tags ?? []).some((t) => tags.includes(t)));
    ops = [...ops];
    const active = this.view.$sortActive();
    const dir = this.view.$sortDir();
    if (!dir) return ops;
    const mul = dir === 'asc' ? 1 : -1;
    ops.sort((a, b) => {
      const byMethod = methodRank(a.method) - methodRank(b.method) || a.path.localeCompare(b.path);
      const byPath = a.path.localeCompare(b.path) || methodRank(a.method) - methodRank(b.method);
      return (active === 'method' ? byMethod : byPath) * mul;
    });
    return ops;
  });

  // Operations whose EFFECTIVE access gates something. The preserved extras -
  // saved rules matching no operation of the spec - used to be ADDED to this,
  // which is how a route with 14 operations read "24 secured": two different
  // sets, one total. They are reported on their own line instead, because a
  // rule nothing matches is not a secured operation, it is a rule to look at.
  protected readonly securedCount = computed(() => {
    let n = 0;
    for (const o of this.operations()) if (!isEmpty(this.effective(o))) n++;
    return n;
  });

  // Of those, the ones gated by a rule of THEIR OWN. The table shows the route's
  // rule repeated on every line - which is the truth about what gates each
  // operation, and reads as if somebody had set fifteen rules. So the footer
  // tells the two apart: what the route gates, and what was decided per
  // endpoint.
  //
  // Not overrideCount: that one adds the stray rules, because the badges need
  // "gated somewhere" - and a stray rule is already a line of its own down
  // here. Counting it twice is how "14 operations" once read "24 secured".
  protected readonly overriddenCount = computed(
    () => this.operations().filter((o) => this.stateOf(o).override).length,
  );

  // And what the ROUTE's own rule gates: every operation that did not take one
  // of its own, when the route has a rule at all.
  protected readonly byRouteCount = computed(() =>
    isEmpty(this.routeAccess()) ? 0 : this.operations().length - this.overriddenCount(),
  );

  // Saved rules that match no operation this spec declares. Kept on save (never
  // silently dropped) and now SAID: they are invisible on this screen otherwise,
  // and what they usually mean is that the spec moved under them.
  protected readonly strayCount = computed(() => this.extras().length);

  constructor() {
    const preselect = inject(ActivatedRoute).snapshot.queryParamMap.get('route') ?? '';
    // Coalesce rapid edits (e.g. picking several roles) into one PUT.
    this.saveTrigger.pipe(debounceTime(500), takeUntilDestroyed()).subscribe(() => void this.persist());
    void this.init(preselect);
  }

  // A user edit happened: reflect it immediately, then persist after the debounce.
  private scheduleSave(): void {
    this.saveState.set('saving');
    this.saveTrigger.next();
  }

  private async init(preselect: string): Promise<void> {
    this.loadingRoutes.set(true);
    this.error.set('');
    try {
      // Roles, users and organisations are app-scoped: tolerate a 403 for a
      // pure gateway admin (the rule can still be set to a level that names
      // nothing).
      const [routes, roles, users, tenants] = await Promise.all([
        firstValueFrom(this.api.listRoutes()),
        firstValueFrom(this.api.listRoles().pipe(catchError(() => of<Role[]>([])))),
        firstValueFrom(this.api.listUsers().pipe(catchError(() => of<User[]>([])))),
        firstValueFrom(this.api.listTenants().pipe(catchError(() => of<Tenant[]>([])))),
      ]);
      this.roles.set(roles);
      this.users.set(users);
      this.tenants.set(tenants);
      this.routes.set(routes);
      const exposing = routes.filter((r) => !!r.api?.spec);
      const pick = exposing.find((r) => r.id === preselect)?.id ?? exposing[0]?.id ?? '';
      if (pick) await this.selectRoute(pick);
    } catch (e) {
      this.error.set(this.message(e));
    } finally {
      this.loadingRoutes.set(false);
    }
  }

  protected async selectRoute(id: string): Promise<void> {
    this.selectedId.set(id);
    this.data.set(null);
    this.close();
    this.error.set('');
    if (!id) return;
    this.loadingOps.set(true);
    try {
      const ops = await firstValueFrom(this.api.getRouteOperations(id));
      this.seed(ops);
      this.data.set(ops);
      // Keep only the persisted filters that this route actually has.
      const knownTags = new Set<string>();
      const knownMethods = new Set<string>();
      for (const o of ops.operations) {
        knownMethods.add(o.method.toUpperCase());
        for (const t of o.tags ?? []) knownTags.add(t);
      }
      this.view.tags = this.view.tags.filter((t) => knownTags.has(t));
      this.view.methods = this.view.methods.filter((m) => knownMethods.has(m));
    } catch (e) {
      this.error.set(this.message(e));
    } finally {
      this.loadingOps.set(false);
    }
  }

  private seed(ops: RouteOperations): void {
    const known = new Set(ops.operations.map((o) => opKey(o.method, o.path)));
    const audits: Record<string, EndpointAudit> = {};
    const strayAudits: EndpointAudit[] = [];
    for (const a of ops.audit ?? []) {
      const k = opKey(a.method, a.path);
      if (known.has(k)) audits[k] = a;
      else strayAudits.push(a);
    }
    this.audits.set(audits);
    this.auditExtras.set(strayAudits);
    // The "whole route" default is the route's own Access now; overrides come
    // from the endpoint-security block.
    this.routeAccess.set(fromWire(ops.access));
    this.denyUnlisted.set(!!ops.security?.denyUnlisted);
    const saved = new Map<string, EndpointPolicy>();
    for (const e of ops.security?.endpoints ?? []) saved.set(opKey(e.method, e.path), e);
    // A rule may name "*" - every verb on that path, which is how one writes
    // "nobody but an admin touches /admin/loggers" without listing four verbs.
    // It matches each operation of that path, and matching is what decides
    // whether the screen can show it: read as a method of its own, such a rule
    // matched nothing and was filed among the strays, invisible.
    const anyVerb = new Map<string, EndpointPolicy>();
    for (const e of ops.security?.endpoints ?? []) {
      if (e.method === '*') anyVerb.set(e.path, e);
    }

    const st: Record<string, OpState> = {};
    const matched = new Set<string>();
    for (const o of ops.operations) {
      const k = opKey(o.method, o.path);
      const p = saved.get(k) ?? anyVerb.get(o.path);
      if (p) {
        // An entry saved for its BOUND alone carries no access fields, and
        // must not read back as an override of nothing.
        st[k] = { override: !isEmpty(fromWire(p)), access: fromWire(p), limits: p.limits ?? [] };
        matched.add(opKey(p.method, p.path));
      } else {
        st[k] = { override: false, access: emptyAccess(), limits: [] };
      }
    }
    this.state.set(st);
    this.extras.set((ops.security?.endpoints ?? []).filter((e) => !matched.has(opKey(e.method, e.path))));
  }

  // ── The drawer ─────────────────────────────────────────────────────────────
  //
  // It replaced an inline fold, and the reason is arithmetic: the detail now
  // carries two editors, so a seventy-row table pushed sixty of them several
  // screens down to show one. A drawer has the room the fold never had, and
  // the table stays a table.
  // Opening is setting which operation the drawer is on, and nothing else.
  //
  // It used to also aim a scroll at one of the two sections, on a timer. Both
  // sections fit in the panel, so the scroll moved nothing that needed moving
  // and reached into the DOM from a component to do it - and the drawer opened,
  // shut and opened again in front of whoever clicked. A panel that flickers is
  // not paying for a nicety.
  protected open(o: OpenAPIOperation): void {
    this.openKey.set(opKey(o.method, o.path));
  }

  protected close(): void {
    this.openKey.set('');
  }

  protected isOpen(o: OpenAPIOperation): boolean {
    return this.openKey() === opKey(o.method, o.path);
  }

  protected stateOf(o: OpenAPIOperation): OpState {
    return this.state()[opKey(o.method, o.path)] ?? { override: false, access: emptyAccess(), limits: [] };
  }

  // One bound in a few characters, for the list on the limits page.
  // The chips are on operations, so they name the operation and not the route.
  protected readonly label = (l: RateLimit) => limitLabel(l, 'operation');
  protected readonly scope = limitScope;
  protected readonly scopeTip = limitScopeTip;
  protected readonly inheritsTip = $localize`:@@Bounded_by_the_route:Only the route's limits apply.`;

  protected setOpLimits(o: OpenAPIOperation, limits: RateLimit[]): void {
    const k = opKey(o.method, o.path);
    this.state.update((s) => ({ ...s, [k]: { ...this.stateOf(o), limits } }));
    // Every other edit on this screen persists itself; this one did not, so a
    // bound written on an operation lived until the page was left.
    this.scheduleSave();
  }

  // How many operations carry a bound, for the header - the same reading as
  // the override count beside it: what is posed here rather than inherited.
  protected readonly boundCount = computed(
    () => Object.values(this.state()).filter((s) => s.limits.length > 0).length,
  );

  // How many operations carry their own rule, plus the saved overrides that
  // match no listed operation: what the badges need to tell "delegated on the
  // route but gated per endpoint" from "gated nowhere at all".
  protected readonly overrideCount = computed(
    () => Object.values(this.state()).filter((s) => s.override).length + this.extras().length,
  );

  protected readonly isEmptyRule = isEmpty;

  // The rule actually in force for an operation: its override, or the route default.
  // What the route puts in front of every operation, shown once beside the
  // spec's name rather than repeated down the column.
  protected readonly prefix = computed(() => this.data()?.prefix ?? '');

  // A path as the column shows it: without the prefix every line shares. What
  // is STORED and what the gateway compares is always the whole path - this is
  // the display and nothing else.
  protected shortPath(path: string): string {
    const p = this.prefix();
    return p && path.startsWith(p) ? path.slice(p.length) || '/' : path;
  }

  protected effective(o: OpenAPIOperation): AccessState {
    const s = this.stateOf(o);
    if (s.override) return s.access;
    return this.denyUnlisted() ? { ...emptyAccess(), level: 'deny' } : this.routeAccess();
  }

  protected setOpAccess(o: OpenAPIOperation, access: AccessState): void {
    const k = opKey(o.method, o.path);
    this.state.update((s) => ({ ...s, [k]: { ...this.stateOf(o), override: true, access } }));
    this.scheduleSave();
  }

  protected setDenyUnlisted(on: boolean): void {
    this.denyUnlisted.set(on);
    this.scheduleSave();
  }

  // Toggle whether an operation overrides the route default. Turning it on seeds
  // from the route default so the admin edits a concrete starting point.
  protected setOverride(o: OpenAPIOperation, on: boolean): void {
    const k = opKey(o.method, o.path);
    this.state.update((s) => {
      const cur = s[k] ?? { override: false, access: emptyAccess(), limits: [] };
      // Turning the override off drops the RULE and keeps the bound: they are
      // two axes, and losing a limit because somebody stopped narrowing who
      // may call the operation would be a deletion nobody asked for.
      if (!on) return { ...s, [k]: { ...cur, override: false, access: emptyAccess() } };
      const seed = isEmpty(cur.access) ? { ...this.routeAccess() } : cur.access;
      return { ...s, [k]: { ...cur, override: true, access: seed } };
    });
    this.scheduleSave();
  }

  // The audited operations as a file: the route editor uploads it back, onto
  // this route or another (AUD-04).
  protected exportAudit(): void {
    const route = this.apiRoutes().find((r) => r.id === this.selectedId());
    const file: AuditFile = {
      kind: AUDIT_FILE_KIND,
      route: route?.name ?? this.selectedId(),
      audit: this.auditEndpoints(),
    };
    const url = URL.createObjectURL(new Blob([JSON.stringify(file, null, 2) + '\n'], { type: 'application/json' }));
    const a = document.createElement('a');
    a.href = url;
    a.download = `${(route?.name ?? 'route').toLowerCase().replace(/[^a-z0-9]+/g, '-')}-audit.json`;
    a.click();
    URL.revokeObjectURL(url);
  }

  // Only what can be valid leaves: a field being typed is kept on screen until
  // it has a name and a key.
  private auditEndpoints(): EndpointAudit[] {
    return [...Object.values(this.audits()), ...this.auditExtras()].map((a) => ({
      ...a,
      fields: (a.fields ?? []).filter((f) => f.name.trim() && f.key.trim()),
    }));
  }

  // ── Save ───────────────────────────────────────────────────────────────────
  private async persist(): Promise<void> {
    const id = this.selectedId();
    if (!id) return;
    if (this.intent() === 'audit') {
      const endpoints = this.auditEndpoints();
      this.saveState.set('saving');
      try {
        await firstValueFrom(this.api.saveRouteAudit(id, endpoints));
        this.saveState.set('saved');
      } catch (e) {
        this.saveError.set(this.message(e));
        this.saveState.set('error');
      }
      return;
    }
    const endpoints: EndpointPolicy[] = [];
    for (const o of this.operations()) {
      const s = this.state()[opKey(o.method, o.path)];
      // Posed for either reason: an access override, a bound, or both.
      if (!s || (!s.override && s.limits.length === 0)) continue;
      endpoints.push(toPolicy(o.method, o.path, s));
    }
    endpoints.push(...this.extras());
    const ra = this.routeAccess();
    const security: RouteSecurity = {
      access: {
        ...(ra.level ? { level: ra.level } : {}),
        ...(ra.tenants.length ? { tenants: ra.tenants } : {}),
        ...(ra.roles.length ? { roles: ra.roles } : {}),
        ...(ra.users.length ? { users: ra.users } : {}),
      },
      endpoints,
      ...(this.denyUnlisted() ? { denyUnlisted: true } : {}),
    };

    this.saveState.set('saving');
    try {
      await firstValueFrom(this.api.saveRouteSecurity(id, security));
      this.saveState.set('saved');
    } catch (e) {
      this.saveError.set(this.message(e));
      this.saveState.set('error');
    }
  }

  private message(e: unknown): string {
    const err = e as HttpErrorResponse;
    const body = err?.error as { error?: string } | undefined;
    if (typeof body?.error === 'string') return body.error;
    if (typeof err?.message === 'string') return err.message;
    return $localize`:@@Request_failed:Request failed`;
  }
}
