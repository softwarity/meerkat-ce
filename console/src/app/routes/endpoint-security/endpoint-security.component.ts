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
  Access,
  ApiService,
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
    inject(ActivatedRoute).data.pipe(map((d) => (d['intent'] === 'limits' ? 'limits' : 'security'))),
    { initialValue: 'security' as 'security' | 'limits' },
  );
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

  // Operations whose EFFECTIVE access gates something, plus the preserved extras.
  protected readonly securedCount = computed(() => {
    let n = this.extras().length;
    for (const o of this.operations()) if (!isEmpty(this.effective(o))) n++;
    return n;
  });

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
    // The "whole route" default is the route's own Access now; overrides come
    // from the endpoint-security block.
    this.routeAccess.set(fromWire(ops.access));
    const saved = new Map<string, EndpointPolicy>();
    for (const e of ops.security?.endpoints ?? []) saved.set(opKey(e.method, e.path), e);

    const st: Record<string, OpState> = {};
    const matched = new Set<string>();
    for (const o of ops.operations) {
      const k = opKey(o.method, o.path);
      const p = saved.get(k);
      if (p) {
        // An entry saved for its BOUND alone carries no access fields, and
        // must not read back as an override of nothing.
        st[k] = { override: !isEmpty(fromWire(p)), access: fromWire(p), limits: p.limits ?? [] };
        matched.add(k);
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
  protected readonly inheritsTip = $localize`:@@Bounded_by_the_route:Bounded by whatever the route carries, and by nothing of its own.`;

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
  protected effective(o: OpenAPIOperation): AccessState {
    const s = this.stateOf(o);
    return s.override ? s.access : this.routeAccess();
  }

  protected setOpAccess(o: OpenAPIOperation, access: AccessState): void {
    const k = opKey(o.method, o.path);
    this.state.update((s) => ({ ...s, [k]: { ...this.stateOf(o), override: true, access } }));
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

  // ── Save ───────────────────────────────────────────────────────────────────
  private async persist(): Promise<void> {
    const id = this.selectedId();
    if (!id) return;
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
