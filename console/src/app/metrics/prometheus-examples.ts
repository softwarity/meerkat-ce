// The files somebody actually has to write to scrape this gateway.
//
// Written out in full rather than described, and carrying THIS installation's
// own address: an example with <your-gateway-here> in it is an example the
// reader has to translate, and the translation is where it goes wrong. The
// token is left as a placeholder on purpose - it is the one value that must
// not travel through a page, a screenshot or a paste buffer by accident.

export interface ExampleContext {
  // Where the control plane answers, as the browser reached it.
  origin: string;
  // The exposition's path, sent by the gateway rather than assembled here.
  path: string;
}

const host = (c: ExampleContext) => {
  try {
    const u = new URL(c.origin);
    return { hostname: u.hostname, port: u.port || (u.protocol === 'https:' ? '443' : '80'), scheme: u.protocol.replace(':', '') };
  } catch {
    return { hostname: 'meerkat', port: '9090', scheme: 'http' };
  }
};

// ── Docker Swarm ───────────────────────────────────────────────────────────

export function swarmScrapeConfig(c: ExampleContext): string {
  const h = host(c);
  return `global:
  scrape_interval: 15s

scrape_configs:
  - job_name: meerkat
    metrics_path: ${c.path}
    scheme: ${h.scheme}
    # The token is read from a file rather than written here: a scrape config
    # lives in a git repository, and a credential in one is a credential that
    # outlives everyone who remembers putting it there.
    authorization:
      type: Bearer
      credentials_file: /run/secrets/meerkat_metrics_token
    static_configs:
      # The service name on the shared network, not the published port: this
      # traffic never has to leave the cluster.
      - targets: ['meerkat:${h.port}']
`;
}

export function swarmSecret(): string {
  return `# Mint the token in the console first: Infra, Access tokens, perimeter
# "metrics". It is shown once. Then hand it to the cluster, never to a file
# in a repository.

printf '%s' 'mk_paste_the_token_here' | docker secret create meerkat_metrics_token -
`;
}

export function swarmStack(c: ExampleContext): string {
  const h = host(c);
  return `# docker stack deploy -c monitoring.yml monitoring
#
# Both services join the network Meerkat is already on, so the scrape is an
# internal call. Only the two consoles are published.

services:
  prometheus:
    image: prom/prometheus:latest
    command:
      - --config.file=/etc/prometheus/prometheus.yml
      # How much history to keep. This is the whole reason for scraping at
      # all: the gateway's own screen holds an hour, in memory.
      - --storage.tsdb.retention.time=30d
    configs:
      - source: meerkat_prometheus_yml
        target: /etc/prometheus/prometheus.yml
    secrets:
      - meerkat_metrics_token
    volumes:
      - prometheus_data:/prometheus
    networks:
      - meerkat
    ports:
      - "9091:9090"

  grafana:
    image: grafana/grafana:latest
    environment:
      GF_SECURITY_ADMIN_PASSWORD__FILE: /run/secrets/grafana_admin
    secrets:
      - grafana_admin
    volumes:
      - grafana_data:/var/lib/grafana
    networks:
      - meerkat
    ports:
      - "3000:3000"

configs:
  meerkat_prometheus_yml:
    file: ./prometheus.yml

secrets:
  meerkat_metrics_token:
    external: true
  grafana_admin:
    external: true

volumes:
  prometheus_data:
  grafana_data:

networks:
  meerkat:
    # The network Meerkat is on. External, because this stack joins it rather
    # than creating a second one that cannot reach ${h.hostname}.
    external: true
`;
}

// ── Kubernetes ─────────────────────────────────────────────────────────────

export function k8sSecret(): string {
  return `# Mint the token in the console first: Infra, Access tokens, perimeter
# "metrics". It is shown once.
#
# Created with kubectl rather than committed as a manifest: a Secret manifest
# is base64, which is not encryption, and it ends up in the repository.

kubectl create secret generic meerkat-metrics \\
  --namespace monitoring \\
  --from-literal=token='mk_paste_the_token_here'
`;
}

export function k8sServiceMonitor(c: ExampleContext): string {
  return `# For the Prometheus Operator (kube-prometheus-stack). The Secret must live
# in the SAME namespace as the ServiceMonitor, which is why it is created in
# monitoring above and not next to the gateway.

apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: meerkat
  namespace: monitoring
  labels:
    # Must match your Prometheus' serviceMonitorSelector, or it is ignored in
    # silence - the one failure mode of this object.
    release: kube-prometheus-stack
spec:
  namespaceSelector:
    matchNames:
      - default
  selector:
    matchLabels:
      app.kubernetes.io/name: meerkat
  endpoints:
    # The NAME of the control-plane port on the Service, not its number.
    - port: admin
      path: ${c.path}
      interval: 15s
      authorization:
        type: Bearer
        credentials:
          name: meerkat-metrics
          key: token
`;
}

export function k8sPlainScrape(c: ExampleContext): string {
  return `# Without the Operator: the same thing as a scrape config, discovering the
# gateway's pods through the API rather than by name.

scrape_configs:
  - job_name: meerkat
    metrics_path: ${c.path}
    authorization:
      type: Bearer
      credentials_file: /etc/prometheus/secrets/meerkat-metrics/token
    kubernetes_sd_configs:
      - role: pod
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_name]
        action: keep
        regex: meerkat
      # The control-plane port. Every gateway pod answers, and each counts
      # what IT served - which is what makes the sum the cluster's.
      - source_labels: [__meta_kubernetes_pod_container_port_name]
        action: keep
        regex: admin
`;
}

// ── Grafana ────────────────────────────────────────────────────────────────

export function grafanaDatasource(): string {
  return `# Drop this in Grafana's provisioning directory
# (/etc/grafana/provisioning/datasources/) so the data source exists before
# anybody logs in.

apiVersion: 1
datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus:9090
    isDefault: true
`;
}

export function grafanaQueries(): string {
  return `# The queries a gateway is actually read on. Paste one per panel.
#
# All of them group by \`name\` - the route's NAME, which a person recognises -
# while the series identity is \`route\`, its id. A route renamed keeps its
# history and changes its label.

# Requests per second, by route
sum by (name) (rate(meerkat_requests_total[5m]))

# What fraction is going wrong, by route
sum by (name) (rate(meerkat_requests_total{status=~"4xx|5xx"}[5m]))
  / sum by (name) (rate(meerkat_requests_total[5m]))

# The 95th percentile of the answer time, by route. Read the distribution
# rather than the mean: a mean hides the tail, and the tail is the complaint.
histogram_quantile(0.95,
  sum by (name, le) (rate(meerkat_request_duration_seconds_bucket[5m])))

# The ten slowest ENDPOINTS, by mean answer. source="declared" keeps this to
# the templates somebody wrote, leaving out the shapes the gateway deduced.
topk(10,
  sum by (name, endpoint) (rate(meerkat_endpoint_duration_seconds_sum{source="declared"}[5m]))
    / sum by (name, endpoint) (rate(meerkat_endpoint_requests_total{source="declared"}[5m])))

# The ten endpoints costing the most time overall - which is neither the
# slowest nor the busiest, and is usually what to fix first.
topk(10, sum by (name, endpoint) (rate(meerkat_endpoint_duration_seconds_sum[5m])))

# Services not answering, by kind of failure
sum by (name, kind) (rate(meerkat_upstream_failures_total[5m]))

# Saturation: requests in flight, summed over the gateways
sum(meerkat_requests_in_flight)

# Refused sign-ins - a rising line here is what a brute-force attempt looks
# like from the gateway
rate(meerkat_logins_total{outcome="refused"}[5m])

# Requests matching no route at all: a misconfiguration nobody has noticed
rate(meerkat_unmatched_total[5m])
`;
}
