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
// on each: the AGENT, one per node (the address producers send to, and the
// reader of the containers' stdout), and the GATEWAY, which receives the traces
// alone from every agent and keeps them or not once each is complete. The
// Grafana files are their companions: a dashboard for the metrics written into
// Prometheus, and the datasources that dashboard reads.
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
      { file: 'collector/agent.yaml', name: 'agent.yaml' },
      { file: 'collector/gateway.yaml', name: 'gateway.yaml' },
      { file: 'collector/grafana-dashboard.json', name: 'grafana-dashboard.json' },
      { file: 'collector/grafana-datasources.yaml', name: 'grafana-datasources.yaml' },
    ],
  },
  {
    key: 'k8s',
    label: 'Kubernetes',
    files: [
      { file: 'kubernetes/agent-values.yaml', name: 'agent-values.yaml' },
      { file: 'kubernetes/gateway-values.yaml', name: 'gateway-values.yaml' },
      { file: 'collector/grafana-dashboard.json', name: 'grafana-dashboard.json' },
      { file: 'collector/grafana-datasources.yaml', name: 'grafana-datasources.yaml' },
    ],
  },
];

// Served at the root of the admin port, beside the console itself.
export const tracingUrl = (name: string) => `/tracing/${name}`;
