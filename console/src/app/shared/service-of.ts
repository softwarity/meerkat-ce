// The service behind an upstream, as the gateway names it for the stream
// templates ({servicename}, internal/tracing/streams.go ServiceOf): the first
// label of its host - orders-api.shop.svc.cluster.local is
// orders-api, a Swarm or Compose name is itself -, an address whole,
// and fallback when the upstream names no host (empty, or a vault reference).
export function serviceOf(upstream: string, fallback: string): string {
  const raw = (upstream ?? '').trim();
  if (!raw || raw.includes('$')) return fallback;
  let host: string;
  try {
    host = new URL(raw).hostname;
  } catch {
    return fallback;
  }
  if (!host) return fallback;
  if (/^\d+(\.\d+){3}$/.test(host) || host.startsWith('[')) return host;
  return host.split('.')[0];
}
