// Package certs is Meerkat's TLS material: how a certificate ENTERS the
// gateway, and how the right one reaches each handshake.
//
// There are four doors, because the rooms Meerkat runs in are not alike. A
// public site renews itself from an ACME authority and nobody ever touches a
// file. An air-gapped room has an internal authority, a paper procedure and no
// route to the internet. A laboratory wants HTTPS in one click and does not
// care who vouches for it. A company already holds the certificate it bought
// and only wants to hand it over. Close any one door and one of those rooms
// has no TLS at all, which is why none of them is an edition feature.
package certs

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"

	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

// Info is what a certificate says about ITSELF. It is extracted once, when the
// material enters, and stored beside it: a list of twenty certificates should
// not re-parse twenty DER blobs to draw a table.
type Info struct {
	Subject     string   `json:"subject"`
	Issuer      string   `json:"issuer"`
	Serial      string   `json:"serial"`
	Algo        string   `json:"algo"`
	KeyType     string   `json:"keyType"`
	DNSNames    []string `json:"dnsNames"`
	IPAddresses []string `json:"ipAddresses"`
	NotBefore   int64    `json:"notBefore"`
	NotAfter    int64    `json:"notAfter"`
	// SelfSigned says the certificate vouches for itself. It is not a fault -
	// it is the honest state of a laboratory - but a console that does not show
	// it lets someone believe an authority was involved.
	SelfSigned bool `json:"selfSigned"`
	// Chain is how many certificates came with the leaf. A missing chain is the
	// classic import mistake: it works in curl, and fails in a browser that has
	// never met the intermediate.
	Chain int `json:"chain"`
}

// Material is a keypair that is ready to serve: the PEM as it will be stored,
// and the parsed pair the TLS stack hands to a handshake.
type Material struct {
	CertPEM string
	KeyPEM  string
	Info    Info

	pair tls.Certificate
}

// Names is every host this material can answer for, in one list, the way an
// operator reads it.
func (m *Material) Names() []string {
	out := make([]string, 0, len(m.Info.DNSNames)+len(m.Info.IPAddresses))
	out = append(out, m.Info.DNSNames...)
	out = append(out, m.Info.IPAddresses...)
	return out
}

// TLS is the parsed pair, for the manager.
func (m *Material) TLS() *tls.Certificate { return &m.pair }

// Leaf is the served certificate itself.
func (m *Material) Leaf() *x509.Certificate { return m.pair.Leaf }

// ParsePEM reads a certificate (leaf first, then any intermediates) and its
// private key. This is the format everyone actually has: what a CA e-mails
// back, what certbot writes, what a Kubernetes secret holds.
func ParsePEM(certPEM, keyPEM string) (*Material, error) {
	certPEM, keyPEM = strings.TrimSpace(certPEM), strings.TrimSpace(keyPEM)
	if certPEM == "" {
		return nil, fmt.Errorf("no certificate: expected a PEM block starting with -----BEGIN CERTIFICATE-----")
	}
	if keyPEM == "" {
		return nil, fmt.Errorf("no private key: expected a PEM block starting with -----BEGIN (RSA|EC|)PRIVATE KEY-----")
	}
	pair, err := tls.X509KeyPair([]byte(certPEM+"\n"), []byte(keyPEM+"\n"))
	if err != nil {
		// The stdlib's own wording for the mismatch case is exact but terse;
		// say what it MEANS, because this is the error an operator hits when
		// they paste the wrong half of two orders. There are TWO wordings -
		// "does not match public key" when the pair is the same kind, "type
		// does not match" when an RSA key meets an ECDSA certificate - and
		// matching only the first is how the commoner mistake slips through
		// with the raw stdlib sentence.
		if strings.Contains(err.Error(), "does not match") {
			return nil, fmt.Errorf("this private key does not belong to this certificate: they are two halves of two different pairs")
		}
		return nil, fmt.Errorf("unreadable PEM: %w", err)
	}
	return fromPair(pair)
}

// ParsePKCS12 reads a .p12/.pfx keystore - the format the Java and Windows
// worlds hand out, and the one Archway accepted exclusively.
//
// A keystore holding no key at all is a TRUST STORE (a bundle of authorities),
// and it is the mistake worth naming: it looks like a certificate file, it
// imports nowhere, and the generic "expected exactly one key" says nothing to
// whoever exported it.
func ParsePKCS12(raw []byte, password string) (*Material, error) {
	key, leaf, chain, err := pkcs12.DecodeChain(raw, password)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "decryption password incorrect"):
			return nil, fmt.Errorf("wrong keystore password")
		case strings.Contains(err.Error(), "expected exactly one key"),
			strings.Contains(err.Error(), "private key missing"):
			return nil, fmt.Errorf("this keystore holds no usable private key: it looks like a trust store (a bundle of authorities), not a server certificate")
		}
		return nil, fmt.Errorf("unreadable keystore: %w", err)
	}
	var buf strings.Builder
	if err := pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: leaf.Raw}); err != nil {
		return nil, fmt.Errorf("re-encoding the certificate: %w", err)
	}
	for _, ca := range chain {
		if err := pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: ca.Raw}); err != nil {
			return nil, fmt.Errorf("re-encoding the chain: %w", err)
		}
	}
	keyPEM, err := MarshalKey(key)
	if err != nil {
		return nil, err
	}
	return ParsePEM(buf.String(), keyPEM)
}

// MarshalKey writes a private key as PKCS#8 PEM - one encoding for every key
// type, so the store never has to remember which flavour it holds.
func MarshalKey(key any) (string, error) {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return "", fmt.Errorf("unsupported private key type %T: %w", key, err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), nil
}

// fromPair completes a parsed pair: it keeps the leaf resolved (the TLS stack
// would otherwise re-parse it on every handshake) and extracts the metadata.
func fromPair(pair tls.Certificate) (*Material, error) {
	if len(pair.Certificate) == 0 {
		return nil, fmt.Errorf("no certificate in this PEM")
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("unreadable certificate: %w", err)
	}
	pair.Leaf = leaf
	if len(leaf.DNSNames) == 0 && len(leaf.IPAddresses) == 0 {
		// Go and every current browser stopped reading the common name years
		// ago. Refusing here is kinder than accepting material that would fail
		// every handshake with a name mismatch nobody can explain.
		return nil, fmt.Errorf(
			"this certificate carries no subject alternative name (SAN), only the common name %q: browsers stopped reading the common name, so it can serve no host",
			leaf.Subject.CommonName)
	}
	m := &Material{
		CertPEM: encodeChain(pair.Certificate),
		Info: Info{
			Subject:     leaf.Subject.String(),
			Issuer:      leaf.Issuer.String(),
			Serial:      leaf.SerialNumber.String(),
			Algo:        leaf.SignatureAlgorithm.String(),
			KeyType:     KeyTypeOf(leaf.PublicKey),
			DNSNames:    append([]string{}, leaf.DNSNames...),
			IPAddresses: ipStrings(leaf),
			NotBefore:   leaf.NotBefore.Unix(),
			NotAfter:    leaf.NotAfter.Unix(),
			SelfSigned:  leaf.Subject.String() == leaf.Issuer.String(),
			Chain:       len(pair.Certificate) - 1,
		},
		pair: pair,
	}
	keyPEM, err := MarshalKey(pair.PrivateKey)
	if err != nil {
		return nil, err
	}
	m.KeyPEM = keyPEM
	return m, nil
}

func encodeChain(der [][]byte) string {
	var buf strings.Builder
	for _, b := range der {
		_ = pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: b})
	}
	return buf.String()
}

func ipStrings(leaf *x509.Certificate) []string {
	out := make([]string, 0, len(leaf.IPAddresses))
	for _, ip := range leaf.IPAddresses {
		out = append(out, ip.String())
	}
	return out
}

// KeyTypeOf names a public key the way the generation form names it, so the
// list and the form speak one vocabulary.
func KeyTypeOf(pub any) string {
	switch k := pub.(type) {
	case *ecdsa.PublicKey:
		// "P-256" becomes "ecdsa-p256": one spelling for the list and
		// for the generation form, or the two would disagree on screen.
		return "ecdsa-" + strings.ToLower(strings.ReplaceAll(k.Curve.Params().Name, "-", ""))
	case *rsa.PublicKey:
		return fmt.Sprintf("rsa-%d", k.N.BitLen())
	case ed25519.PublicKey:
		return "ed25519"
	default:
		return "unknown"
	}
}
