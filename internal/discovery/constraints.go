package discovery

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"slices"
	"strings"
)

// What the authority behind a Certificate lets it name.
//
// A cluster CA may carry X.509 name constraints: the DNS names it may sign
// for, the ones it may not. cert-manager's CA issuer does not read them and
// signs whatever the Certificate asks, and a leaf carrying one name outside
// them is refused WHOLE by every verifier (crypto/x509, OpenSSL), whatever
// name was called. So the names the gateway asks for follow what the chain
// cert-manager wrote beside the leaf permits. The rules are crypto/x509's,
// which exports no helper for them (constraints.go: dnsHasSuffix and the
// query of newDNSConstraints).

// nameLimits are the issuing certificates above a leaf that constrain DNS
// names; none is the ordinary authority, which permits anything.
type nameLimits struct {
	cas []*x509.Certificate
}

// limitsOf reads the chain of a Secret: the leaf first in tls.crt, the
// intermediates after it, the authority in ca.crt. Only the certificates that
// signed the leaf, one above the other, count: a bundle may carry others.
func limitsOf(tlsCrt, caCrt []byte) nameLimits {
	certs := parseCerts(tlsCrt)
	if len(certs) == 0 {
		return nameLimits{}
	}
	pool := slices.Concat(certs[1:], parseCerts(caCrt))
	var l nameLimits
	used := make([]bool, len(pool))
	for cur, depth := certs[0], 0; depth < 8; depth++ {
		parent := -1
		for i, c := range pool {
			if !used[i] && cur.CheckSignatureFrom(c) == nil {
				parent = i
				break
			}
		}
		if parent < 0 {
			break
		}
		used[parent] = true
		cur = pool[parent]
		if len(cur.PermittedDNSDomains) > 0 || len(cur.ExcludedDNSDomains) > 0 {
			l.cas = append(l.cas, cur)
		}
		if bytes.Equal(cur.RawIssuer, cur.RawSubject) {
			break
		}
	}
	return l
}

func parseCerts(b []byte) []*x509.Certificate {
	var out []*x509.Certificate
	for {
		var blk *pem.Block
		blk, b = pem.Decode(b)
		if blk == nil {
			return out
		}
		if blk.Type != "CERTIFICATE" {
			continue
		}
		if c, err := x509.ParseCertificate(blk.Bytes); err == nil {
			out = append(out, c)
		}
	}
}

func (l nameLimits) none() bool { return len(l.cas) == 0 }

// split keeps the names every constraining authority permits, in order, and
// says which it refuses.
func (l nameLimits) split(names []string) (kept, refused []string) {
	for _, n := range names {
		ok := true
		for _, ca := range l.cas {
			if !dnsPermitted(n, ca.PermittedDNSDomains, ca.ExcludedDNSDomains) {
				ok = false
				break
			}
		}
		if ok {
			kept = append(kept, n)
		} else {
			refused = append(refused, n)
		}
	}
	return kept, refused
}

// String names each constraining authority and its DNS constraints, for a
// log or a screen.
func (l nameLimits) String() string {
	parts := make([]string, 0, len(l.cas))
	for _, ca := range l.cas {
		var c []string
		if len(ca.PermittedDNSDomains) > 0 {
			c = append(c, "permitted "+strings.Join(ca.PermittedDNSDomains, ", "))
		}
		if len(ca.ExcludedDNSDomains) > 0 {
			c = append(c, "excluded "+strings.Join(ca.ExcludedDNSDomains, ", "))
		}
		parts = append(parts, ca.Subject.String()+" ("+strings.Join(c, "; ")+")")
	}
	return strings.Join(parts, ", ")
}

// dnsPermitted says whether one authority's DNS constraints let it sign for
// name, as crypto/x509 checks a chain: an exclusion wins; with permitted
// subtrees the name must fall in one, without any it is permitted; an empty
// constraint matches everything.
func dnsPermitted(name string, permitted, excluded []string) bool {
	for _, c := range excluded {
		if dnsUnder(name, c) {
			return false
		}
		// An excluded name excludes the wildcard one level up.
		if strings.HasPrefix(name, "*") && trimFirstLabel(strings.ToLower(c)) != "" &&
			trimFirstLabel(strings.ToLower(name)) == trimFirstLabel(strings.ToLower(c)) {
			return false
		}
	}
	if len(permitted) == 0 {
		return true
	}
	for _, c := range permitted {
		if dnsUnder(name, c) {
			return true
		}
	}
	return false
}

// dnsUnder is crypto/x509's dnsHasSuffix: the constraint "svc" covers "svc"
// and any "x.svc", never "xsvc"; ".svc" covers its subdomains only; case
// does not count.
func dnsUnder(name, constraint string) bool {
	if constraint == "" {
		return true
	}
	if len(constraint) > len(name) || !strings.EqualFold(name[len(name)-len(constraint):], constraint) {
		return false
	}
	return constraint[0] == '.' || len(name) == len(constraint) || name[len(name)-len(constraint)-1] == '.'
}

func trimFirstLabel(name string) string {
	i := strings.IndexByte(name, '.')
	if i < 0 {
		return ""
	}
	return name[i:]
}
