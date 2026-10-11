// What a certificate answers for, and whether that covers a host: the checks
// the TLS screen makes before a certificate is placed, without asking the
// gateway.

// The rule x509 applies: an exact match, or a wildcard standing for exactly
// one label - *.example.com covers app.example.com, not example.com.
export function covers(names: string[], host: string): boolean {
  const h = host.toLowerCase();
  return names.some((raw) => {
    const n = raw.toLowerCase();
    if (n === h) return true;
    if (!n.startsWith('*.')) return false;
    const rest = h.slice(0, h.length - (n.length - 1));
    return h.endsWith(n.slice(1)) && rest !== '' && !rest.includes('.');
  });
}

// Whether names answer for at least one of hosts.
export function coversOne(names: string[], hosts: string[]): boolean {
  return hosts.some((h) => covers(names, h));
}

// Whether every name a certificate carries is one of hosts: the internal
// plane's names, which reach only its own ports.
export function onlyAmong(names: string[], hosts: string[]): boolean {
  const set = new Set(hosts.map((h) => h.toLowerCase()));
  return names.length > 0 && names.every((n) => set.has(n.toLowerCase()));
}

// The names the first certificate of a PEM answers for: the DNS names and
// addresses of its subjectAltName, read from the DER. The subject's common
// name is not one - no TLS client has matched it for years. [] when the text
// holds no certificate this can read.
export function pemNames(pem: string): string[] {
  const m = pem.match(/-----BEGIN CERTIFICATE-----([\s\S]*?)-----END CERTIFICATE-----/);
  if (!m) return [];
  let der: Uint8Array;
  try {
    der = Uint8Array.from(atob(m[1].replace(/\s+/g, '')), (c) => c.charCodeAt(0));
  } catch {
    return [];
  }
  try {
    const cert = children(der, read(der, 0));
    const tbs = children(der, cert[0]);
    const ext = tbs.find((e) => e.tag === 0xa3);
    if (!ext) return [];
    for (const e of children(der, children(der, ext)[0])) {
      const parts = children(der, e);
      if (hex(der, parts[0]) !== '551d11') continue; // 2.5.29.17, subjectAltName
      const octets = parts[parts.length - 1];
      const names: string[] = [];
      for (const g of children(der, read(der, octets.start))) {
        const v = der.subarray(g.start, g.end);
        if (g.tag === 0x82) names.push(String.fromCharCode(...v));
        else if (g.tag === 0x87 && v.length === 4) names.push(Array.from(v).join('.'));
        else if (g.tag === 0x87 && v.length === 16) names.push(ipv6(v));
      }
      return names;
    }
  } catch {
    // Not DER this can walk: nothing to say about it.
  }
  return [];
}

// One DER element: its tag, and where its content starts and ends.
interface Tlv {
  tag: number;
  start: number;
  end: number;
}

function read(b: Uint8Array, at: number): Tlv {
  const tag = b[at];
  let len = b[at + 1];
  let start = at + 2;
  if (len & 0x80) {
    const n = len & 0x7f;
    len = 0;
    for (let i = 0; i < n; i++) len = len * 256 + b[start + i];
    start += n;
  }
  if (start + len > b.length) throw new Error('truncated');
  return { tag, start, end: start + len };
}

function children(b: Uint8Array, parent: Tlv): Tlv[] {
  const out: Tlv[] = [];
  for (let at = parent.start; at < parent.end; ) {
    const t = read(b, at);
    out.push(t);
    at = t.end;
  }
  return out;
}

function hex(b: Uint8Array, t: Tlv): string {
  return Array.from(b.subarray(t.start, t.end), (x) => x.toString(16).padStart(2, '0')).join('');
}

function ipv6(v: Uint8Array): string {
  const groups: string[] = [];
  for (let i = 0; i < 16; i += 2) groups.push(((v[i] << 8) | v[i + 1]).toString(16));
  return groups.join(':').replace(/(^|:)0(:0)+(:|$)/, '::');
}
