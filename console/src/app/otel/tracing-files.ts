// The files somebody deploys to have a collector, as FILES - under
// console/public/tracing, not strings built in TypeScript.
//
// A YAML in a template literal is one no editor validates and no linter reads, and `curl
// <gateway>/tracing/swarm/collector-compose.yml` should get the file with
// nothing in the way. The console is one reader of them, not the only one.
//
// ONE FILE PER PLATFORM rather than one with markers: a compose stack and a
// pair of Kubernetes objects are not one thing with a variation, they are two
// different things.

export interface TracingFile {
  // Where it is served, under /tracing/.
  file: string;
  // What it is called once downloaded.
  name: string;
}

// The platforms, in the order a reader meets them, and the Collector's files
// on each. The Grafana files are its companions: a dashboard for the metrics it
// writes into Prometheus, and the datasources that dashboard reads.
export const TRACING_PLATFORMS: readonly {
  key: string;
  label: string;
  files: readonly TracingFile[];
}[] = [
  {
    key: 'swarm',
    label: 'Docker Swarm',
    files: [
      { file: 'swarm/collector-compose.yml', name: 'collector-compose.yml' },
      { file: 'collector/collector.yaml', name: 'collector.yaml' },
      { file: 'collector/logs-agent.yaml', name: 'logs-agent.yaml' },
      { file: 'collector/grafana-dashboard.json', name: 'grafana-dashboard.json' },
      { file: 'collector/grafana-datasources.yaml', name: 'grafana-datasources.yaml' },
    ],
  },
  {
    key: 'k8s',
    label: 'Kubernetes',
    files: [
      { file: 'kubernetes/collector-values.yaml', name: 'collector-values.yaml' },
      { file: 'kubernetes/collector-logs-values.yaml', name: 'collector-logs-values.yaml' },
      { file: 'kubernetes/opentelemetry-service.yaml', name: 'opentelemetry-service.yaml' },
      { file: 'collector/grafana-dashboard.json', name: 'grafana-dashboard.json' },
      { file: 'collector/grafana-datasources.yaml', name: 'grafana-datasources.yaml' },
    ],
  },
];

// Served at the root of the admin port, beside the console itself.
export const tracingUrl = (name: string) => `/tracing/${name}`;
