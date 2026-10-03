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
    what: $localize`:@@EE_AUTH_03_what:Sign people in against your directory, and map its groups.`,
    where: '/infra/auth-providers',
    whereLabel: $localize`:@@Authentication:Authentication`,
  },
  'AUTH-19': {
    label: $localize`:@@EE_AUTH_19:SAML`,
    what: $localize`:@@EE_AUTH_19_what:Federate with a SAML identity provider.`,
  },
  'AUTH-20': {
    label: $localize`:@@EE_AUTH_20:Kerberos`,
    what: $localize`:@@EE_AUTH_20_what:Sign in without typing anything, from a machine in the domain.`,
  },
  'RBAC-10': {
    label: $localize`:@@EE_RBAC_10:Group rules`,
    what: $localize`:@@EE_RBAC_10_what:Put people in groups from what their directory says about them.`,
    where: '/application/group-rules',
    whereLabel: $localize`:@@Group_rules:Group rules`,
  },
  'TENANT-01': {
    label: $localize`:@@EE_TENANT_01:Several organisations`,
    what: $localize`:@@EE_TENANT_01_what:Isolate customers, each with its own groups, members and settings.`,
    where: '/tenants',
    whereLabel: $localize`:@@Tenants:Tenants`,
  },
  'TENANT-04': {
    label: $localize`:@@EE_TENANT_04:Working hours`,
    what: $localize`:@@EE_TENANT_04_what:Refuse access outside declared hours, gateway-wide, per organisation or per person.`,
    where: '/application/general',
    whereLabel: $localize`:@@General:General`,
  },
  'TENANT-08': {
    label: $localize`:@@EE_TENANT_08:One organisation or several`,
    what: $localize`:@@EE_TENANT_08_what:Switch the installation between the two, live.`,
    where: '/application/general',
    whereLabel: $localize`:@@General:General`,
  },
  'SVC-02': {
    label: $localize`:@@EE_SVC_02:Cluster discovery`,
    what: $localize`:@@EE_SVC_02_what:Find the services of a Kubernetes or Swarm cluster when creating a route.`,
    where: '/infra/routes',
    whereLabel: $localize`:@@Routes:Routes`,
  },
  'THEME-02': {
    label: $localize`:@@EE_THEME_02:Your brand only`,
    what: $localize`:@@EE_THEME_02_what:Remove the Meerkat mark from the pages your users see.`,
    where: '/application/built-in-pages/branding',
    whereLabel: $localize`:@@Branding:Branding`,
  },
  'CONSOLE-14': {
    label: $localize`:@@EE_CONSOLE_14:This list`,
    what: $localize`:@@EE_CONSOLE_14_what:What the Enterprise edition carries, read from the product's own contract.`,
    where: '/license',
    whereLabel: $localize`:@@License:License`,
  },
  'DEV-03': {
    label: $localize`:@@EE_DEV_03:Developer tunnel, laptop to cluster`,
    what: $localize`:@@EE_DEV_03_what:A developer's code calls the services of the cluster as if it ran there.`,
    where: '/infra/plug',
    whereLabel: $localize`:@@Plug:Plug`,
  },
  'DEV-04': {
    label: $localize`:@@EE_DEV_04:Developer tunnel, cluster to laptop`,
    what: $localize`:@@EE_DEV_04_what:The cluster's traffic for one service reaches a developer's machine.`,
    where: '/infra/plug',
    whereLabel: $localize`:@@Plug:Plug`,
  },
  'DEV-07': {
    label: $localize`:@@EE_DEV_07:Tunnel lifecycle`,
    what: $localize`:@@EE_DEV_07_what:A substitution ends on its own when its developer goes.`,
    where: '/infra/plug',
    whereLabel: $localize`:@@Plug:Plug`,
  },
  'DEV-08': {
    label: $localize`:@@EE_DEV_08:Several substitutions at once`,
    what: $localize`:@@EE_DEV_08_what:Several developers on several services, without stepping on each other.`,
    where: '/infra/plug',
    whereLabel: $localize`:@@Plug:Plug`,
  },
  'DEV-11': {
    label: $localize`:@@EE_DEV_11:Tunnel security`,
    what: $localize`:@@EE_DEV_11_what:Who may open a tunnel, to what, and the record of it.`,
    where: '/infra/plug',
    whereLabel: $localize`:@@Plug:Plug`,
  },
  'CFG-07': {
    label: $localize`:@@EE_CFG_07:Configurations in git`,
    what: $localize`:@@EE_CFG_07_what:Keep configurations in a git repository, exported under your own name.`,
    where: '/infra/configuration',
    whereLabel: $localize`:@@Configuration:Configuration`,
  },
  'PAGE-02': {
    label: $localize`:@@EE_PAGE_02:Page layouts`,
    what: $localize`:@@EE_PAGE_02_what:Arrange the sign-in pages beyond the default layout.`,
    where: '/application/built-in-pages/layout',
    whereLabel: $localize`:@@Layout:Layout`,
  },
  'AUD-03': {
    label: $localize`:@@EE_AUD_03:Audit to OpenTelemetry`,
    what: $localize`:@@EE_AUD_03_what:Send the audit trail to your collector, kept apart from the logs.`,
    where: '/infra/opentelemetry',
    whereLabel: $localize`:@@OpenTelemetry:OpenTelemetry`,
  },
  'SSL-05': {
    label: $localize`:@@EE_SSL_05:ACME certificates`,
    what: $localize`:@@EE_SSL_05_what:Have Let's Encrypt, ZeroSSL or your own authority issue and renew certificates on their own.`,
    where: '/infra/tls',
    whereLabel: $localize`:@@TLS:TLS`,
  },
  'AUD-04': {
    label: $localize`:@@EE_AUD_04:Endpoint audit`,
    what: $localize`:@@EE_AUD_04_what:Record chosen operations of a route in the audit trail.`,
    where: '/infra/endpoint-audit',
    whereLabel: $localize`:@@Endpoint_audit:Endpoint audit`,
  },
  'PERF-03': {
    label: $localize`:@@EE_PERF_03:Active/active cluster`,
    what: $localize`:@@EE_PERF_03_what:Several gateways behind one load balancer, none of them primary.`,
  },
  'OBS-04': {
    label: $localize`:@@EE_OBS_04:Tracing`,
    what: $localize`:@@EE_OBS_04_what:The gateway's own spans and the browser's, sent to your collector.`,
    where: '/infra/opentelemetry',
    whereLabel: $localize`:@@OpenTelemetry:OpenTelemetry`,
  },
  'OBS-06': {
    label: $localize`:@@EE_OBS_06:Metrics and logs to a collector`,
    what: $localize`:@@EE_OBS_06_what:Push the counters and the logs to an OpenTelemetry Collector.`,
    where: '/infra/opentelemetry',
    whereLabel: $localize`:@@OpenTelemetry:OpenTelemetry`,
  },
  'DEPLOY-07': {
    label: $localize`:@@EE_DEPLOY_07:Deployment files`,
    what: $localize`:@@EE_DEPLOY_07_what:The Helm chart and compose files for a clustered installation.`,
  },
  'STORE-03': {
    label: $localize`:@@EE_STORE_03:Shared state`,
    what: $localize`:@@EE_STORE_03_what:The gateways share one PostgreSQL and tell each other what changed.`,
  },
  'STORE-06': {
    label: $localize`:@@EE_STORE_06:Audit export`,
    what: $localize`:@@EE_STORE_06_what:Export the filtered trail as a file, for an auditor or a SIEM.`,
    where: '/audit',
    whereLabel: $localize`:@@Audit:Audit`,
  },
};
