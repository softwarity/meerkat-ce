// The files somebody deploys to have a collector, as FILES - under
// console/public/tracing, not strings built in TypeScript.
//
// Same reasoning as the monitoring files next door: a YAML in a template
// literal is one no editor validates and no linter reads, and `curl
// <gateway>/tracing/swarm/docker-compose.yml` should get the file with
// nothing in the way. The console is one reader of them, not the only one.
//
// TWO FILES RATHER THAN ONE with markers, unlike the Prometheus scrape. There
// the platforms differed by six lines of discovery inside one scrape, so one
// file with both commented out read better than two files somebody compares by
// hand. Here they are a compose stack and a pair of Kubernetes objects: not
// one thing with a variation, two different things.

export const TRACING = {
  swarmStack: 'swarm/docker-compose.yml',
  k8s: 'kubernetes/jaeger.yaml',
} as const;

// The platforms, in the order a reader meets them.
export const TRACING_PLATFORMS = [
  { key: 'swarm', label: 'Docker Swarm', file: TRACING.swarmStack, name: 'tracing-compose.yml' },
  { key: 'k8s', label: 'Kubernetes', file: TRACING.k8s, name: 'jaeger.yaml' },
] as const;

// Served at the root of the admin port, beside the console itself.
export const tracingUrl = (name: string) => `/tracing/${name}`;
