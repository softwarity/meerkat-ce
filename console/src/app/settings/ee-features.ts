// What each Enterprise feature is, in the console's words (CONSOLE-14).
//
// WHICH features are Enterprise, and how far each is built, is not written
// here: the gateway reads it from FEATURES.md, the product's contract, and
// sends it with the edition. This only names them and says where they live.
// A Go test (features_test.go) holds the two together - an Enterprise row
// with no entry here, or an entry for a row that is not Enterprise, fails the
// build - so this list cannot drift from the contract.
//
// `where` is a console path when the feature has a screen, and absent when it
// has none: those are the ones a badge beside a control could never show.

export interface EeFeatureCopy {
  label: string;
  what: string;
  where?: string;
  whereLabel?: string;
}

export const EE_FEATURES: Record<string, EeFeatureCopy> = {
  'AUTH-03': {
    label: $localize`:@@EE_AUTH_03:LDAP and Active Directory`,
    what: $localize`:@@EE_AUTH_03_what:Sign users in with your LDAP or Active Directory, and use its groups.`,
    where: '/infra/auth-providers',
    whereLabel: $localize`:@@Authentication:Authentication`,
  },
  'AUTH-19': {
    label: $localize`:@@EE_AUTH_19:SAML`,
    what: $localize`:@@EE_AUTH_19_what:Sign users in through a SAML identity provider.`,
  },
  'AUTH-20': {
    label: $localize`:@@EE_AUTH_20:Kerberos`,
    what: $localize`:@@EE_AUTH_20_what:Sign users in automatically from a computer joined to the domain.`,
  },
  'SAUTH-05': {
    label: $localize`:@@EE_SAUTH_05:OpenID provider`,
    what: $localize`:@@EE_SAUTH_05_what:Meerkat acts as an OpenID Connect provider for the applications behind your routes.`,
    where: '/infra/routes',
    whereLabel: $localize`:@@Routes:Routes`,
  },
  'RBAC-10': {
    label: $localize`:@@EE_RBAC_10:Group rules`,
    what: $localize`:@@EE_RBAC_10_what:Place users in groups automatically, from what their directory says about them.`,
    where: '/application/group-rules',
    whereLabel: $localize`:@@Group_rules:Group rules`,
  },
  'TENANT-01': {
    label: $localize`:@@EE_TENANT_01:Several tenants`,
    what: $localize`:@@EE_TENANT_01_what:Serve several customers from one gateway, each with its own groups, members and settings.`,
    where: '/tenants',
    whereLabel: $localize`:@@Tenants:Tenants`,
  },
  'MODEL-03': {
    label: $localize`:@@EE_MODEL_03:Tenant fields`,
    what: $localize`:@@EE_MODEL_03_what:Your own fields on each tenant, such as plan or region, sent to applications by your routes.`,
    where: '/infra/model',
    whereLabel: $localize`:@@Model:Model`,
  },
  'TENANT-04': {
    label: $localize`:@@EE_TENANT_04:Working hours`,
    what: $localize`:@@EE_TENANT_04_what:Refuse access outside set hours, for the whole gateway, a tenant, a user or a group. Sessions end at closing time.`,
    where: '/application/general',
    whereLabel: $localize`:@@General:General`,
  },
  'TENANT-08': {
    label: $localize`:@@EE_TENANT_08:One tenant or several`,
    what: $localize`:@@EE_TENANT_08_what:Switch between one tenant and several at any time.`,
    where: '/application/general',
    whereLabel: $localize`:@@General:General`,
  },
  'SVC-02': {
    label: $localize`:@@EE_SVC_02:Cluster discovery`,
    what: $localize`:@@EE_SVC_02_what:Pick a service of your Kubernetes or Swarm cluster when you create a route.`,
    where: '/infra/routes',
    whereLabel: $localize`:@@Routes:Routes`,
  },
  'PORTAL-01': {
    label: $localize`:@@EE_PORTAL_01:Navigation bar`,
    what: $localize`:@@EE_PORTAL_01_what:A bar on every application page to move between your applications.`,
    where: '/application/portal',
    whereLabel: $localize`:@@Portal_menu:Portal`,
  },
  'NOTIF-05': {
    label: $localize`:@@EE_NOTIF_05:Notifications`,
    what: $localize`:@@EE_NOTIF_05_what:Your services send notifications, and users see them in whichever application they have open.`,
    where: '/data-plane/notifications',
    whereLabel: $localize`:@@Notifications:Notifications`,
  },
  'THEME-02': {
    label: $localize`:@@EE_THEME_02:Your brand only`,
    what: $localize`:@@EE_THEME_02_what:Remove the Meerkat mark from the pages your users see.`,
    where: '/application/built-in-pages/branding',
    whereLabel: $localize`:@@Branding:Branding`,
  },
  'CONSOLE-14': {
    label: $localize`:@@EE_CONSOLE_14:Enterprise feature list`,
    what: $localize`:@@EE_CONSOLE_14_what:The Enterprise features, and how far each one is built.`,
    where: '/system/license',
    whereLabel: $localize`:@@License:License`,
  },
  'DEV-02': {
    label: $localize`:@@EE_DEV_02:Developer keys`,
    what: $localize`:@@EE_DEV_02_what:Developers register the SSH key of each of their machines.`,
    where: '/application/plug',
    whereLabel: $localize`:@@Plug:Plug`,
  },
  'DEV-03': {
    label: $localize`:@@EE_DEV_03:Developer tunnel, laptop to cluster`,
    what: $localize`:@@EE_DEV_03_what:Code running on a developer's machine calls the cluster's services as if it ran there.`,
    where: '/infra/plug',
    whereLabel: $localize`:@@Plug:Plug`,
  },
  'DEV-04': {
    label: $localize`:@@EE_DEV_04:Developer tunnel, cluster to laptop`,
    what: $localize`:@@EE_DEV_04_what:Send the cluster's traffic for one service to a developer's machine.`,
    where: '/infra/plug',
    whereLabel: $localize`:@@Plug:Plug`,
  },
  'DEV-06': {
    label: $localize`:@@EE_DEV_06:Substitution notice`,
    what: $localize`:@@EE_DEV_06_what:Users are told when a developer's machine answers for a service they use.`,
  },
  'DEV-07': {
    label: $localize`:@@EE_DEV_07:Tunnel lifecycle`,
    what: $localize`:@@EE_DEV_07_what:A substitution ends by itself when its developer disconnects.`,
    where: '/infra/plug',
    whereLabel: $localize`:@@Plug:Plug`,
  },
  'DEV-08': {
    label: $localize`:@@EE_DEV_08:Several substitutions at once`,
    what: $localize`:@@EE_DEV_08_what:Several developers at once, each on a different service.`,
    where: '/infra/plug',
    whereLabel: $localize`:@@Plug:Plug`,
  },
  'DEV-11': {
    label: $localize`:@@EE_DEV_11:Tunnel security`,
    what: $localize`:@@EE_DEV_11_what:Choose which developers may open a tunnel, and see how to install and run plug.`,
    where: '/infra/plug',
    whereLabel: $localize`:@@Plug:Plug`,
  },
  'DEV-13': {
    label: $localize`:@@EE_DEV_13:Plug sessions`,
    what: $localize`:@@EE_DEV_13_what:See who is connected through plug, and end a session in one click.`,
    where: '/application/plug',
    whereLabel: $localize`:@@Plug:Plug`,
  },
  'DEV-14': {
    label: $localize`:@@EE_DEV_14:Tunnel audit`,
    what: $localize`:@@EE_DEV_14_what:Every plug session, and every service it answered for, in the audit trail.`,
    where: '/system/audit',
    whereLabel: $localize`:@@Audit:Audit`,
  },
  'EVAL-06': {
    label: $localize`:@@EE_EVAL_06:License`,
    what: $localize`:@@EE_EVAL_06_what:Paste the license Softwarity sent you, or mount it from a secret.`,
  },
  'CFG-07': {
    label: $localize`:@@EE_CFG_07:Configurations in git`,
    what: $localize`:@@EE_CFG_07_what:Keep configurations in a git repository, committed under your own name.`,
    where: '/system/configuration',
    whereLabel: $localize`:@@Configuration:Configuration`,
  },
  'PAGE-02': {
    label: $localize`:@@EE_PAGE_02:Page layouts`,
    what: $localize`:@@EE_PAGE_02_what:More layouts for the sign-in pages than the default one.`,
    where: '/application/built-in-pages/layout',
    whereLabel: $localize`:@@Layout:Layout`,
  },
  'AUD-03': {
    label: $localize`:@@EE_AUD_03:Audit to OpenTelemetry`,
    what: $localize`:@@EE_AUD_03_what:Send the audit trail to your OpenTelemetry collector, apart from the logs.`,
    where: '/infra/opentelemetry',
    whereLabel: $localize`:@@OpenTelemetry:OpenTelemetry`,
  },
  'SSL-05': {
    label: $localize`:@@EE_SSL_05:ACME certificates`,
    what: $localize`:@@EE_SSL_05_what:Let's Encrypt, ZeroSSL or your own authority issues and renews certificates automatically.`,
    where: '/infra/tls',
    whereLabel: $localize`:@@Domains_and_TLS:Domains & TLS`,
  },
  'AUD-04': {
    label: $localize`:@@EE_AUD_04:Endpoint audit`,
    what: $localize`:@@EE_AUD_04_what:Record chosen operations of a route in the audit trail.`,
    where: '/infra/endpoint-audit',
    whereLabel: $localize`:@@Endpoint_audit:Endpoint audit`,
  },
  'PERF-03': {
    label: $localize`:@@EE_PERF_03:Active/active cluster`,
    what: $localize`:@@EE_PERF_03_what:Several gateways behind one load balancer, with no primary.`,
  },
  'OBS-04': {
    label: $localize`:@@EE_OBS_04:Tracing`,
    what: $localize`:@@EE_OBS_04_what:Traces from the gateway and from browsers, sent to your collector.`,
    where: '/infra/opentelemetry',
    whereLabel: $localize`:@@OpenTelemetry:OpenTelemetry`,
  },
  'OBS-06': {
    label: $localize`:@@EE_OBS_06:Metrics and logs to a collector`,
    what: $localize`:@@EE_OBS_06_what:Send metrics and logs to an OpenTelemetry Collector.`,
    where: '/infra/opentelemetry',
    whereLabel: $localize`:@@OpenTelemetry:OpenTelemetry`,
  },
  'DEPLOY-07': {
    label: $localize`:@@EE_DEPLOY_07:Deployment files`,
    what: $localize`:@@EE_DEPLOY_07_what:Helm chart and compose files for a clustered installation.`,
  },
  'STORE-03': {
    label: $localize`:@@EE_STORE_03:Shared state`,
    what: $localize`:@@EE_STORE_03_what:Several gateways share one PostgreSQL database and stay in sync.`,
  },
  'STORE-06': {
    label: $localize`:@@EE_STORE_06:Audit export`,
    what: $localize`:@@EE_STORE_06_what:Download the filtered audit trail as a file, for an auditor or a SIEM.`,
    where: '/system/audit',
    whereLabel: $localize`:@@Audit:Audit`,
  },
};
