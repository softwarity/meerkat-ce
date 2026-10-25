import { Service, computed, inject, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { ApiService, Edition, Me, User } from './api.service';

// Session identity. The gateway STAMPS it on <body> when it serves the
// console (capability roles as classes, data-meerkat-* attributes): reading
// the stamp replaces the /api/me boot call, and the role-CSS visibility
// (`any-role` in styles/_roles.scss) applies from the first paint. /api/me
// stays as the fallback when the stamp is absent (bare `ng serve`). Guards
// await the load either way; the API enforces the same scopes server-side.
@Service()
export class MeService {
  private readonly api = inject(ApiService);

  readonly me = signal<Me | null>(null);
  readonly user = computed(() => this.me()?.user ?? null);
  readonly isRoot = computed(() => this.user()?.root ?? false);
  // The zone this operator reads dates in: the one chosen on their profile,
  // else their browser's. Changing it on the profile reloads the page.
  readonly timezone = computed(
    () => this.user()?.timezone || Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
  );
  readonly isTenantCreator = computed(() => this.user()?.tenantCreator ?? false);
  // Split administration (RBAC-05): root implies both scopes.
  readonly isInfraAdmin = computed(() => this.isRoot() || (this.user()?.infraAdmin ?? false));
  readonly isAppAdmin = computed(() => this.isRoot() || (this.user()?.appAdmin ?? false));
  // Administers at least one tenant (owner or ADMIN), computed server-side and
  // carried on /api/me (a non-member owner would be missed by the memberships).
  readonly isTenantAdmin = computed(() => this.isRoot() || (this.me()?.tenantAdmin ?? false));
  // Administers something, so the live channel is open to them: it refuses
  // whoever administers nothing (a plain account reading the licence), and a
  // console that asked anyway logged a refused socket on every page. Unknown
  // until loaded, which counts as yes - the old behaviour, never a lost feed.
  readonly mayWatch = computed(
    () =>
      this.me() === null ||
      this.isRoot() ||
      this.isInfraAdmin() ||
      this.isAppAdmin() ||
      this.isTenantAdmin(),
  );

  // What this installation IS. Stamped on <body> by the gateway, like the
  // roles: read here, never written. The CSS in styles/_modes.scss already
  // applied it before this class existed - these signals are for the handful
  // of places that need the value rather than the styling (route guards, and
  // the screens that target the served organisation without naming it).
  readonly multiTenant = signal(document.body.classList.contains('multi-tenant'));
  readonly primaryTenant = signal(document.body.getAttribute('data-meerkat-primary-tenant') ?? '');
  // Which IMAGE is running, read from the same stamp the CSS reads. One class,
  // one signal: there is no feature registry any more, so a control is either
  // in this binary or it is not.
  readonly enterprise = signal(document.body.classList.contains('ee'));
  // The organisation the tenant-scoped endpoints are called with: whichever is
  // being administered in multi, the served one in single - where no screen
  // ever names it.
  readonly scopeTenant = computed(() => this.primaryTenant());

  // The fallback path: no stamp means the console was not served by the
  // gateway, so what it is has to be asked for and put on <body> by hand -
  // the CSS reads classes, not signals, and it would otherwise draw the
  // community shape over an Enterprise installation.
  private applyEdition(e: Edition): void {
    this.multiTenant.set(e.tenancy === 'multi');
    this.primaryTenant.set(e.primaryTenant);
    this.enterprise.set(e.enterprise);
    document.body.classList.toggle('multi-tenant', e.tenancy === 'multi');
    document.body.classList.toggle('ee', e.enterprise);
  }

  // Called after switching the mode: the stamp is a page-load thing, and the
  // switch is not worth a reload.
  refreshEdition(): Promise<void> {
    return firstValueFrom(this.api.edition())
      .then((e) => this.applyEdition(e))
      .catch(() => undefined);
  }

  private loading?: Promise<Me | null>;

  // Resolves the identity once (cached); safe to call from guards and the
  // app shell.
  ensureLoaded(): Promise<Me | null> {
    if (!this.loading) {
      const stamped = this.fromStamp();
      if (stamped) {
        this.me.set(stamped);
        this.loading = Promise.resolve(stamped);
      } else {
        this.loading = firstValueFrom(this.api.me())
          .then((me) => {
            this.apply(me);
            void this.refreshEdition();
            return me;
          })
          .catch(() => {
            this.me.set(null);
            return null;
          });
      }
    }
    return this.loading;
  }

  // The server stamp: the username attribute marks a signed-in session; the
  // role classes are already on <body>, nothing to mirror.
  private fromStamp(): Me | null {
    const b = document.body;
    const username = b.getAttribute('data-meerkat-username');
    if (!username) return null;
    const has = (c: string) => b.classList.contains(c);
    const user = {
      id: b.getAttribute('data-meerkat-user-id') ?? '',
      username,
      fullname: b.getAttribute('data-meerkat-fullname') ?? '',
      email: b.getAttribute('data-meerkat-email') ?? '',
      // The zone this operator's own screens write dates in (their profile
      // sets it, on either plane). Stamped rather than asked for: a screen
      // that shows an hour must show the right one on the first paint.
      timezone: b.getAttribute('data-meerkat-timezone') ?? '',
      root: has('root'),
      dev: has('dev'),
      tenantCreator: has('tenant-creator'),
      infraAdmin: has('infra-admin'),
      appAdmin: has('app-admin'),
    } as User;
    // The tenant-admin class is the server's answer to the same question
    // /api/me answers (owner or ADMIN of an organisation). Left out, every
    // guard asking isTenantAdmin said no on the console that ships - only the
    // unstamped dev server asked /api/me - and an organisation's administrator
    // was bounced off the Audit, Sessions and Issues screens the rail offered.
    return { user, tenants: [], tenantAdmin: has('tenant-admin') };
  }

  private apply(me: Me): void {
    this.me.set(me);
    const roles: string[] = [];
    if (me.user.root) roles.push('root');
    if (me.user.dev) roles.push('dev');
    if (me.user.tenantCreator) roles.push('tenant-creator');
    if (me.user.infraAdmin) roles.push('infra-admin');
    if (me.user.appAdmin) roles.push('app-admin');
    // Ownership is decoupled from membership, so the server computes this
    // (owner-or-admin of any tenant) - the memberships list alone can miss a
    // non-member owner.
    if (me.tenantAdmin) roles.push('tenant-admin');
    document.body.classList.remove(
      'root',
      'dev',
      'tenant-creator',
      'infra-admin',
      'app-admin',
      'tenant-admin',
    );
    document.body.classList.add(...roles);
  }
}
