// The files somebody has to write to scrape this gateway - real files under
// console/public/monitoring, not strings built in TypeScript.
//
// Three things follow from being files, and they are why they moved there:
//   - they are EDITED as what they are. A YAML held in a template literal is
//     one no editor validates and no linter reads, where a backtick or a
//     ${ } ends the string instead of appearing in the file.
//   - they are FETCHED as what they are: `curl <gateway>/monitoring/swarm/
//     prometheus.yml` gets the file, with nothing in the way. The console is
//     one reader of them, not the only one.
//   - each one carries its own DEFAULTS and therefore works as it stands.
//
// SHORT ON PURPOSE. Prometheus has a manual and these are not it: a comment is
// spent only where the thing cannot be guessed or fails in silence - our path,
// our token, the label a ServiceMonitor is ignored without.

// The five, named once. The console reads these paths and a Go test checks
// that each one is on disk: they are strings, and a renamed file would
// otherwise turn into an empty panel that nothing failed to produce.
//
// The scrape sits at the ROOT because it is the one file no platform owns.
export const MONITORING = {
  scrape: 'prometheus.yml',
  swarmStack: 'swarm/docker-compose.yml',
  k8sMonitor: 'kubernetes/servicemonitor.yaml',
  grafanaSource: 'grafana/datasource.yaml',
  grafanaDashboards: 'grafana/dashboards.yaml',
  grafanaDashboard: 'grafana/dashboard.json',
  grafanaQueries: 'grafana/queries.promql',
} as const;

// The platforms the scrape carries, and the marker each of its lines is
// prefixed with.
//
// ONE scrape file rather than one per platform, because there was never more
// than one scrape: the target's discovery is all that changes, and two files
// alike but for six lines are two files a reader compares by hand to find out
// which difference was meant. So the file carries BOTH discoveries, each
// commented out, and the drawer hands back the one you asked for.
export const PLATFORMS = [
  { key: 'swarm', label: 'Docker Swarm' },
  { key: 'k8s', label: 'Kubernetes' },
] as const;

// The scrape as ONE platform needs it: that platform's lines uncommented, the
// other platform's gone.
//
// A line-start marker (`#swarm `) rather than a fenced region, and it is the
// marker that carries the indentation: strip it and the YAML underneath is
// already aligned, so nothing has to be reindented and a block can be read in
// place. A comment inside a marked block is marked too - it belongs to its
// platform, and a Swarm file explaining Kubernetes is a file that answers a
// question nobody asked.
export function variant(text: string, key: string): string {
  const kept = text
    .split('\n')
    .filter((line) => {
      const marker = markerOf(line);
      return marker === undefined || marker === key;
    })
    .map((line) => (markerOf(line) === key ? line.slice(key.length + 2) : line));
  // Removing a block leaves the blank line that separated it from the next.
  return kept
    .filter((line, i) => line.trim() !== '' || kept[i - 1]?.trim() !== '')
    .join('\n')
    .trimEnd()
    .concat('\n');
}

const keys: readonly string[] = PLATFORMS.map((p) => p.key);

// A marker is `#` + a declared platform + a space, at the start of the line.
// An ordinary comment starts `# `, so the two never meet.
function markerOf(line: string): string | undefined {
  const m = /^#([a-z0-9]+) /.exec(line);
  return m && keys.includes(m[1]) ? m[1] : undefined;
}

// Served at the root of the admin port, beside the console itself.
export const monitoringUrl = (name: string) => `/monitoring/${name}`;

// What this installation knows and a file on disk cannot.
export interface Installation {
  // The exposition's path, sent by the gateway rather than assembled here.
  path: string;
  // The port the gateway LISTENS on - not the one the console was reached at.
  // These scrapes address the container on a shared network, so a published
  // mapping (9092 onto 9090) and an ingress answering on 443 are both beside
  // the point; the browser's own port was right in development and nowhere
  // else.
  port: string;
  // The network Prometheus has to join to reach this gateway, as the runtime
  // named it (GET /api/services, `reach`). Empty when nothing answered.
  network: string;
  // Where the DATA plane answers. Behind a route, Prometheus and Grafana still
  // have to know the public URL they are served under - the gateway strips the
  // prefix, it does not rewrite the links they print.
  dataOrigin: string;
}

// Which of the gateway's networks Prometheus should join. The first that is
// not one of the runtime's own: ingress carries the routing mesh and the
// default bridges register no service names, so a stack joining either
// reaches nothing by name.
const infrastructure = ['ingress', 'bridge', 'host', 'none', 'docker_gwbridge'];

export function gatewayNetwork(reach: readonly string[]): string {
  return reach.find((n) => !infrastructure.includes(n)) ?? '';
}

// The installation's own values, written INTO the defaults rather than into
// markers.
//
// A file full of __PLACEHOLDER__ is a file nobody can run, which would undo
// the point of having files at all - so what ships is a working example for a
// default install, and this replaces the two values we happen to know better.
// On a default install both are no-ops.
//
// Literals rather than regexes: each of the three means exactly one thing in
// these files, so the substitution survives them being reindented,
// recommented or reordered.
export function stamp(text: string, i: Installation): string {
  const stamped = text
    .replaceAll('/metrics', i.path)
    .replaceAll('port: 9090', `port: ${i.port}`);
  const withNetwork = i.network ? stamped.replaceAll('meerkat_default', i.network) : stamped;
  return i.dataOrigin ? withNetwork.replaceAll('http://localhost:8080', i.dataOrigin) : withNetwork;
}
