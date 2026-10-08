import { Routes, UrlMatchResult, UrlSegment } from '@angular/router';
import { SectionShellComponent } from './shell/section-shell.component';
import {
  apiDocsAccess,
  appOnly,
  auditAccess,
  dataAuditAccess,
  dataPlaneLanding,
  systemLanding,
  logsAccess,
  sessionsAccess,
  schedulerAccess,
  issuesAccess,
  multiTenantOnly,
  singleTenantOnly,
  vaultAccess,
  metricsAccess,
  firstTenantRedirect,
  infraOnly,
  landingRedirect,
  rootOnly,
} from './access.guards';

// The routes page owns routes, routes/new and routes/:id/:section through ONE
// route config: the SAME component instance survives every drawer open/close
// (no re-create, no re-fetch, one clean drawer animation), only the params
// change. A plain routes/:id counts as :id + the default section.
//
// As a CHILD of /infra the matcher sees the segments left after "infra", so it
// is written against "routes" either way.
function routesMatcher(segments: UrlSegment[]): UrlMatchResult | null {
  if (segments.length === 0 || segments[0].path !== 'routes' || segments.length > 3) return null;
  const posParams: Record<string, UrlSegment> = {};
  if (segments.length >= 2 && segments[1].path !== 'new') posParams['id'] = segments[1];
  if (segments.length === 3) posParams['section'] = segments[2];
  return { consumed: segments, posParams };
}

// users and users/:id, one config (see routesMatcher above).
function usersMatcher(segments: UrlSegment[]): UrlMatchResult | null {
  if (segments.length === 0 || segments[0].path !== 'users' || segments.length > 2) return null;
  const posParams: Record<string, UrlSegment> = {};
  if (segments.length === 2) posParams['id'] = segments[1];
  return { consumed: segments, posParams };
}

// issues and issues/:id, one config (see routesMatcher above).
function issuesMatcher(segments: UrlSegment[]): UrlMatchResult | null {
  if (segments.length === 0 || segments[0].path !== 'issues' || segments.length > 2) return null;
  const posParams: Record<string, UrlSegment> = {};
  if (segments.length === 2) posParams['id'] = segments[1];
  return { consumed: segments, posParams };
}

// rules and rules/:rule (rules/new included), for an organisation's group
// rules; group-rules and group-rules/:rule for Application's. The rule opens
// in the screen's right drawer.
function rulesMatcher(name: string) {
  return (segments: UrlSegment[]): UrlMatchResult | null => {
    if (segments.length === 0 || segments[0].path !== name || segments.length > 2) return null;
    const posParams: Record<string, UrlSegment> = {};
    if (segments.length === 2) posParams['rule'] = segments[1];
    return { consumed: segments, posParams };
  };
}

// auth-providers, auth-providers/new and auth-providers/:id.
function authProvidersMatcher(segments: UrlSegment[]): UrlMatchResult | null {
  if (segments.length === 0 || segments[0].path !== 'auth-providers' || segments.length > 2) return null;
  const posParams: Record<string, UrlSegment> = {};
  if (segments.length === 2 && segments[1].path !== 'new') posParams['id'] = segments[1];
  return { consumed: segments, posParams };
}

// roles, roles/new and roles/:id - same one-config trick.
function rolesMatcher(segments: UrlSegment[]): UrlMatchResult | null {
  if (segments.length === 0 || segments[0].path !== 'roles' || segments.length > 2) return null;
  const posParams: Record<string, UrlSegment> = {};
  if (segments.length === 2 && segments[1].path !== 'new') posParams['id'] = segments[1];
  return { consumed: segments, posParams };
}

// management, and management/<id> when a configuration's file is open in the
// drawer - "current" being the id of what the gateway serves.
function managementMatcher(segments: UrlSegment[]): UrlMatchResult | null {
  if (!segments.length || segments[0].path !== 'management') return null;
  if (segments.length > 2) return null;
  const posParams: Record<string, UrlSegment> = {};
  if (segments.length === 2) posParams['id'] = segments[1];
  return { consumed: segments, posParams };
}

// history, and history/<id> when a point's document is open in the drawer.
function historyMatcher(segments: UrlSegment[]): UrlMatchResult | null {
  if (!segments.length || segments[0].path !== 'history') return null;
  if (segments.length > 2) return null;
  const posParams: Record<string, UrlSegment> = {};
  if (segments.length === 2) posParams['id'] = segments[1];
  return { consumed: segments, posParams };
}

export const routes: Routes = [
  // "/" resolves to the first section the user may use (infra admin ->
  // routes, app admin -> general, others -> tenants).
  { path: '', pathMatch: 'full', canActivate: [landingRedirect], children: [] },

  // ── the infra plane ────────────────────────────────────────────────────────
  // Its sections are CHILDREN, so the shell hosting the left nav stays mounted
  // across them and the URL says which plane one is in.
  {
    path: 'infra',
    component: SectionShellComponent,
    data: { plane: 'infra' },
    children: [
      { path: '', pathMatch: 'full', redirectTo: 'routes' },
      {
        // The editor drawer is URL-driven (F5-proof): routes/new opens a blank
        // one, routes/:id/:section opens that route on that section.
        matcher: routesMatcher,
        canActivate: [infraOnly],
        loadComponent: () =>
          import('./routes/routes-page/routes-page.component').then((m) => m.RoutesPageComponent),
      },
      {
        // Where this gateway's traces go (OBS-04). Infra, with the other
        // systems this installation is wired to - and NOT on the Metrics
        // screen, which is where it used to hide: which routes are traced is
        // decided on the routes themselves.
        path: 'opentelemetry',
        canActivate: [infraOnly],
        loadComponent: () =>
          import('./otel/otel-page.component').then((m) => m.OtelPageComponent),
      },
      {
        // The developer tunnel (DEV-11): a port into the cluster, so Infra.
        // Developer mode stays in Application, General - a configuration of
        // what the installation offers its developers - and is the other
        // condition the tunnel needs.
        path: 'plug',
        canActivate: [infraOnly],
        loadComponent: () => import('./plug/plug-page.component').then((m) => m.PlugPageComponent),
      },
      {
        // The SHAPE of what this installation keeps, starting with the
        // account. Infra and not beside the accounts: defining a field and
        // filling it are two acts by two people.
        path: 'model',
        canActivate: [infraOnly],
        loadComponent: () =>
          import('./model/user-model.component').then((m) => m.UserModelComponent),
      },
      {
        // The operation inventory (RBAC-07, QUOTA-05): a dedicated page with a
        // route selector; picking a route that exposes an OpenAPI spec loads
        // its operations in a swagger-like editor. Optional ?route=<id>
        // preselects one.
        //
        // TWO ENTRIES on one component, and that is not a screen listed twice:
        // it is one inventory read on two axes, the way the traffic ranking is
        // read on three. Security asks who may call an operation, Rate limits
        // asks how much - two questions somebody arrives with, and a menu that
        // names neither is a menu where neither is found. The intent decides
        // what the table leads with and which half of the drawer opens; the
        // other half is always one click away, so neither entry is a dead end.
        path: 'endpoint-security',
        canActivate: [infraOnly],
        data: { intent: 'security' },
        loadComponent: () =>
          import('./routes/endpoint-security/endpoint-security.component').then(
            (m) => m.EndpointSecurityComponent,
          ),
      },
      {
        path: 'endpoint-limits',
        canActivate: [infraOnly],
        data: { intent: 'limits' },
        loadComponent: () =>
          import('./routes/endpoint-security/endpoint-security.component').then(
            (m) => m.EndpointSecurityComponent,
          ),
      },
      {
        path: 'endpoint-audit',
        canActivate: [infraOnly],
        data: { intent: 'audit' },
        loadComponent: () =>
          import('./routes/endpoint-security/endpoint-security.component').then(
            (m) => m.EndpointSecurityComponent,
          ),
      },
      {
        // External authentication (AUTH-19): a directory or an identity
        // provider is a third-party service, like an upstream or the relay.
        matcher: authProvidersMatcher,
        canActivate: [infraOnly],
        loadComponent: () =>
          import('./gateway/auth-providers/auth-providers-page.component').then(
            (m) => m.AuthProvidersPageComponent,
          ),
      },
      {
        path: 'mail-relay',
        canActivate: [infraOnly],
        loadComponent: () =>
          import('./gateway/mail-relay-page.component').then((m) => m.MailRelayPageComponent),
      },
      {
        // TLS (SSL-01/02/03/05): the certificates and the two HTTPS doors. It
        // sits in the infra plane for the same reason the relay does - it is a
        // property of the installation, not of the application it serves.
        path: 'tls',
        canActivate: [infraOnly],
        loadComponent: () => import('./gateway/tls/tls-page.component').then((m) => m.TlsPageComponent),
      },
      {
        // Anybody who administers this plane, for their own tokens.
        path: 'access-tokens',
        canActivate: [infraOnly],
        loadComponent: () =>
          import('./gateway/access-tokens-page.component').then((m) => m.AccessTokensPageComponent),
      },
      {
        // Connecting an agent (MCP-01/07). Its own section, below the
        // configuration: "let my assistant work on this gateway" is a
        // different question from "give me a key for the REST API", and the
        // OAuth flow answers the first one without producing a key at all.
        path: 'mcp',
        canActivate: [infraOnly],
        loadComponent: () => import('./gateway/mcp-page.component').then((m) => m.McpPageComponent),
      },
      // The configuration moved under Meerkat: a document crosses both planes,
      // so it was never the routing plane's. Its addresses follow it, tab and
      // open item included.
      { path: 'configuration', redirectTo: '/system/configuration' },
      { path: 'configuration/:tab', redirectTo: '/system/configuration/:tab' },
      { path: 'configuration/:tab/:id', redirectTo: '/system/configuration/:tab/:id' },
    ],
  },

  // ── the application plane ──────────────────────────────────────────────────
  {
    path: 'application',
    component: SectionShellComponent,
    data: { plane: 'application' },
    children: [
      { path: '', pathMatch: 'full', redirectTo: 'general' },
      {
        path: 'general',
        canActivate: [appOnly],
        loadComponent: () => import('./settings/general-page.component').then((m) => m.GeneralPageComponent),
      },
      {
        // Groups, Members and the rules administer the SERVED organisation -
        // the one single mode never names. They are app-scoped screens, not
        // organisation ones.
        // These three are about THE organisation: with several, Tenants is
        // where one is chosen, so the guard sends a bookmark there.
        path: 'groups',
        canActivate: [appOnly, singleTenantOnly],
        loadComponent: () =>
          import('./identity/app-scoped/app-groups.component').then((m) => m.AppGroupsComponent),
      },
      {
        path: 'members',
        canActivate: [appOnly, singleTenantOnly],
        loadComponent: () =>
          import('./identity/app-scoped/app-members.component').then((m) => m.AppMembersComponent),
      },
      {
        matcher: rulesMatcher('group-rules'),
        canActivate: [appOnly, singleTenantOnly],
        loadComponent: () =>
          import('./identity/app-scoped/app-rules.component').then((m) => m.AppRulesComponent),
      },
      {
        // The role drawer is URL-driven too: roles/new opens a blank one,
        // roles/:id opens that role.
        matcher: rolesMatcher,
        canActivate: [appOnly],
        loadComponent: () =>
          import('./identity/roles-page/roles-page.component').then((m) => m.RolesPageComponent),
      },
      {
        // Same one-config trick as routes: users and users/:id share ONE
        // component instance, so opening the drawer never re-creates (nor
        // re-fetches) the page - only the params change.
        matcher: usersMatcher,
        canActivate: [appOnly],
        loadComponent: () =>
          import('./identity/users-page/users-page.component').then((m) => m.UsersPageComponent),
      },
      // The pages this gateway serves: colours, arrangement and identity are
      // one subject with three tabs, and the tabs are child ROUTES so a
      // bookmark on the gallery comes back to the gallery.
      {
        path: 'built-in-pages',
        canActivate: [appOnly],
        loadComponent: () =>
          import('./theme/built-in-pages/built-in-pages.component').then((m) => m.BuiltInPagesComponent),
        children: [
          { path: '', pathMatch: 'full', redirectTo: 'theme' },
          {
            path: 'theme',
            loadComponent: () =>
              import('./theme/built-in-pages/theme-tab.component').then((m) => m.ThemeTabComponent),
          },
          {
            path: 'layout',
            loadComponent: () =>
              import('./theme/built-in-pages/layout-tab.component').then((m) => m.LayoutTabComponent),
          },
          {
            path: 'branding',
            loadComponent: () =>
              import('./theme/built-in-pages/branding-tab.component').then((m) => m.BrandingTabComponent),
          },
          {
            path: 'locale',
            loadComponent: () =>
              import('./theme/built-in-pages/locale-tab.component').then((m) => m.LocaleTabComponent),
          },
        ],
      },
      // They were two screens of their own until they became two tabs: the
      // bookmarks people already have must land on the tab, not on a 404.
      { path: 'theme', redirectTo: 'built-in-pages/theme' },
      { path: 'branding', redirectTo: 'built-in-pages/branding' },
      {
        // The navigation portal (PORTAL-01): the bar the proxied applications
        // wear, edited with a live preview beside it.
        path: 'portal',
        canActivate: [appOnly],
        loadComponent: () =>
          import('./portal/portal-page.component').then((m) => m.PortalPageComponent),
      },
      // Moved to the rail: sessions are not configuration (see /sessions).
      { path: 'sessions', redirectTo: '/data-plane/sessions' },
      {
        path: 'security',
        canActivate: [appOnly],
        loadComponent: () =>
          import('./settings/security-page.component').then((m) => m.SecurityPageComponent),
      },
      {
        // The same screen the infra plane has: tokens are personal, and an app
        // admin's agent needs one as much as an infra admin's does.
        path: 'access-tokens',
        canActivate: [appOnly],
        loadComponent: () =>
          import('./gateway/access-tokens-page.component').then((m) => m.AccessTokensPageComponent),
      },
    ],
  },

  // ── transverse screens ─────────────────────────────────────────────────────
  // They belong to no single plane (the vault and the audit trail scope
  // themselves per caller), and a tenant brings its own left nav.
  {
    path: 'tenants',
    canActivate: [multiTenantOnly, firstTenantRedirect],
    loadComponent: () => import('./identity/no-tenant.component').then((m) => m.NoTenantComponent),
  },
  // Every tenant section is a child ROUTE of the tenant layout: deep links
  // work, and the left nav's active state is plain routerLinkActive.
  {
    path: 'tenants/:id',
    canActivate: [multiTenantOnly],
    loadComponent: () =>
      import('./identity/tenant-page/tenant-page.component').then((m) => m.TenantPageComponent),
    children: [
      { path: '', pathMatch: 'full', redirectTo: 'general' },
      {
        path: 'general',
        loadComponent: () =>
          import('./identity/tenant-sections/tenant-general.component').then((m) => m.TenantGeneralComponent),
      },
      {
        path: 'groups',
        loadComponent: () =>
          import('./identity/tenant-sections/tenant-groups.component').then((m) => m.TenantGroupsComponent),
      },
      {
        path: 'members',
        loadComponent: () =>
          import('./identity/tenant-sections/tenant-members.component').then((m) => m.TenantMembersComponent),
      },
      {
        matcher: rulesMatcher('rules'),
        loadComponent: () =>
          import('./identity/tenant-sections/tenant-rules.component').then((m) => m.TenantRulesComponent),
      },
      {
        path: 'danger',
        loadComponent: () =>
          import('./identity/tenant-sections/tenant-danger.component').then((m) => m.TenantDangerComponent),
      },
    ],
  },
  { path: 'license', redirectTo: '/system/license' },

  {
    path: 'vault',
    canActivate: [vaultAccess],
    loadComponent: () => import('./gateway/vault-page.component').then((m) => m.VaultPageComponent),
  },
  // ── the data plane ─────────────────────────────────────────────────────────
  // What the applications this gateway serves are doing: their sign-ins, their
  // sessions, the calls made to them on a schedule, their traffic and what
  // their users reported. Sections, like Infra, each guarded on its own - the
  // entry shows for anyone who may open one of them, and the bare /data-plane
  // forwards to the first one they may.
  {
    path: 'data-plane',
    component: SectionShellComponent,
    data: { plane: 'data-plane' },
    children: [
      { path: '', pathMatch: 'full', canActivate: [dataPlaneLanding], children: [] },
      {
        path: 'audit',
        canActivate: [dataAuditAccess],
        data: { trail: 'data' },
        loadComponent: () => import('./settings/audit-page.component').then((m) => m.AuditPageComponent),
      },
      {
        // Read within the caller's perimeter - root every application session,
        // an application administrator the same, an organisation's
        // administrator theirs.
        path: 'sessions',
        canActivate: [sessionsAccess],
        data: { sessions: 'data' },
        loadComponent: () =>
          import('./identity/sessions-page/sessions-page.component').then((m) => m.SessionsPageComponent),
      },
      {
        // The scheduled calls (SCHED-01): what the thing this gateway serves
        // is doing, not what the gateway holds.
        path: 'scheduler',
        canActivate: [schedulerAccess],
        loadComponent: () =>
          import('./scheduler/scheduler-page.component').then((m) => m.SchedulerPageComponent),
      },
      {
        // Not at /metrics: a child path, so nothing the control plane serves
        // at its root can shadow it on a reload.
        path: 'metrics',
        canActivate: [metricsAccess],
        loadComponent: () =>
          import('./metrics/metrics-page.component').then((m) => m.MetricsPageComponent),
      },
      {
        matcher: issuesMatcher,
        canActivate: [issuesAccess],
        loadComponent: () => import('./issues/issues-page.component').then((m) => m.IssuesPageComponent),
      },
    ],
  },

  // ── Meerkat itself ─────────────────────────────────────────────────────────
  // What was done to the gateway and what it says about itself: the changes
  // and the console's sign-ins, its own log, who holds the console. At
  // /system, because /meerkat/ is the gateway's own prefix (its scripts, its
  // events) and a reload there would never reach the console.
  {
    path: 'system',
    component: SectionShellComponent,
    data: { plane: 'system' },
    children: [
      { path: '', pathMatch: 'full', canActivate: [systemLanding], children: [] },
      {
        path: 'audit',
        canActivate: [auditAccess],
        data: { trail: 'system' },
        loadComponent: () => import('./settings/audit-page.component').then((m) => m.AuditPageComponent),
      },
      {
        // The gateway's own log (OBS-03), live.
        path: 'logs',
        canActivate: [logsAccess],
        loadComponent: () => import('./logs/logs-page.component').then((m) => m.LogsPageComponent),
      },
      {
        // Who holds the console: root's alone.
        path: 'sessions',
        canActivate: [rootOnly],
        data: { sessions: 'admin' },
        loadComponent: () =>
          import('./identity/sessions-page/sessions-page.component').then((m) => m.SessionsPageComponent),
      },
      {
        // The configuration screen (CFG-02/03/05). Root only: a document
        // crosses both planes at once, which is why it is Meerkat's and not
        // Infra's. Three tabs, and they are child ROUTES so
        // a bookmark on one comes back to it.
        path: 'configuration',
        canActivate: [rootOnly],
        loadComponent: () =>
          import('./gateway/configuration/configuration-page.component').then(
            (m) => m.ConfigurationPageComponent,
          ),
        children: [
          { path: '', pathMatch: 'full', redirectTo: 'management' },
          // The Import/export tab is gone: its export report, its plan and its
          // vault entries live in the dialogs of Management now. The path stays
          // as a redirect - the bookmarks people already have must land
          // somewhere that makes sense, not on a 404.
          { path: 'import-export', redirectTo: 'management' },
          {
            path: 'snapshot',
            loadComponent: () =>
              import('./gateway/configuration/snapshot-tab.component').then(
                (m) => m.ConfigurationSnapshotComponent,
              ),
          },
          {
            // The tape, and one point open in its drawer: same shape as
            // management, same reason.
            matcher: historyMatcher,
            loadComponent: () =>
              import('./gateway/configuration/history-tab.component').then(
                (m) => m.ConfigurationHistoryComponent,
              ),
          },
          {
            // management and management/:id share ONE component instance (the
            // same trick as routes and roles): opening the drawer changes a
            // param, it does not re-create the screen nor re-fetch the list.
            matcher: managementMatcher,
            loadComponent: () =>
              import('./gateway/configuration/management-tab.component').then(
                (m) => m.ConfigurationManagementComponent,
              ),
          },
        ],
      },
      {
        // The swagger-ui screen: an iframe over the gateway-served /apidocs/
        // page. The control plane's reference, for whoever scripts it.
        path: 'api',
        canActivate: [apiDocsAccess],
        loadComponent: () => import('./gateway/api-docs-page.component').then((m) => m.ApiDocsPageComponent),
      },
      {
        // What this version brought (CONSOLE-15). Anyone holding the console.
        path: 'release-notes',
        loadComponent: () =>
          import('./shared/release-notes.component').then((m) => m.ReleaseNotesPageComponent),
      },
      {
        // The edition's own screen (CONSOLE-14). No guard: it is the one page
        // anyone holding the console may open, which makes it the landing of
        // whoever administers nothing.
        path: 'license',
        loadComponent: () =>
          import('./settings/license-page.component').then((m) => m.LicensePageComponent),
      },
    ],
  },

  // Where these screens used to live, for the bookmarks and the links already
  // pasted somewhere.
  { path: 'traffic', redirectTo: '/data-plane/metrics' },
  { path: 'scheduler', redirectTo: '/data-plane/scheduler' },
  { path: 'sessions', redirectTo: '/data-plane/sessions' },
  { path: 'issues', redirectTo: '/data-plane/issues' },
  { path: 'issues/:id', redirectTo: '/data-plane/issues/:id' },
  { path: 'audit', redirectTo: '/system/audit' },
  { path: 'logs', redirectTo: '/system/logs' },
  { path: 'api', redirectTo: '/system/api' },

  { path: '**', redirectTo: '' },
];
