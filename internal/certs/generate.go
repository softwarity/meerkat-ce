package certs

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"strings"
	"time"
)

// Key types Meerkat will generate. A closed set, and a short one: an operator
// choosing a curve is choosing between "modern" and "an old appliance in front
// of me refuses anything but RSA", and every further option is noise.
const (
	KeyECDSAP256 = "ecdsa-p256"
	KeyECDSAP384 = "ecdsa-p384"
	KeyRSA2048   = "rsa-2048"
	KeyRSA4096   = "rsa-4096"
)

// KeyTypes is the allowed set, in the order a form should offer them.
var KeyTypes = []string{KeyECDSAP256, KeyECDSAP384, KeyRSA2048, KeyRSA4096}

// DefaultDays is how long a generated self-signed certificate lasts.
//
// 397 and not "a few years": Apple refuses any server certificate whose
// validity exceeds 398 days, self-signed and manually trusted ones included.
// A two-year laboratory certificate is a certificate that works everywhere
// except on the Mac of whoever asks why it does not work.
const DefaultDays = 397

// Subject is the distinguished name, split the way a form asks for it.
type Subject struct {
	CommonName         string `json:"commonName"`
	Organization       string `json:"organization,omitempty"`
	OrganizationalUnit string `json:"organizationalUnit,omitempty"`
	Country            string `json:"country,omitempty"`
	Province           string `json:"province,omitempty"`
	Locality           string `json:"locality,omitempty"`
}

// Request is what to generate: for whom, under which key, and - when it is a
// self-signed certificate rather than a signing request - for how long.
type Request struct {
	Subject
	DNSNames    []string `json:"dnsNames"`
	IPAddresses []string `json:"ipAddresses,omitempty"`
	KeyType     string   `json:"keyType"`
	Days        int      `json:"days,omitempty"`
}

// normalize fills the defaults and refuses what cannot work, naming the
// allowed values rather than saying "invalid".
func (r *Request) normalize() ([]net.IP, error) {
	r.CommonName = strings.TrimSpace(r.CommonName)
	names := make([]string, 0, len(r.DNSNames))
	for _, n := range r.DNSNames {
		if n = strings.TrimSpace(n); n != "" {
			names = append(names, n)
		}
	}
	r.DNSNames = names
	ips := make([]net.IP, 0, len(r.IPAddresses))
	for _, raw := range r.IPAddresses {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		ip := net.ParseIP(raw)
		if ip == nil {
			return nil, fmt.Errorf("%q is not an IP address", raw)
		}
		ips = append(ips, ip)
	}
	if len(r.DNSNames) == 0 && len(ips) == 0 {
		return nil, fmt.Errorf("no host to certify: give at least one DNS name, or an IP address")
	}
	if r.CommonName == "" {
		// The common name no longer decides anything, but it is what every
		// tool prints as the certificate's name. Leaving it empty makes a list
		// of unnamed rows.
		if len(r.DNSNames) > 0 {
			r.CommonName = r.DNSNames[0]
		} else {
			r.CommonName = r.IPAddresses[0]
		}
	}
	if r.KeyType == "" {
		r.KeyType = KeyECDSAP256
	}
	if !validKeyType(r.KeyType) {
		return nil, fmt.Errorf("unknown key type %q (allowed: %s)", r.KeyType, strings.Join(KeyTypes, ", "))
	}
	if r.Days <= 0 {
		r.Days = DefaultDays
	}
	return ips, nil
}

func validKeyType(k string) bool {
	for _, t := range KeyTypes {
		if t == k {
			return true
		}
	}
	return false
}

func (r Request) pkixName() pkix.Name {
	n := pkix.Name{CommonName: r.CommonName}
	add := func(dst *[]string, v string) {
		if v = strings.TrimSpace(v); v != "" {
			*dst = append(*dst, v)
		}
	}
	add(&n.Organization, r.Organization)
	add(&n.OrganizationalUnit, r.OrganizationalUnit)
	add(&n.Country, r.Country)
	add(&n.Province, r.Province)
	add(&n.Locality, r.Locality)
	return n
}

// newKey generates the private key of the requested type.
func newKey(keyType string) (crypto.Signer, error) {
	switch keyType {
	case KeyECDSAP256:
		return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case KeyECDSAP384:
		return ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	case KeyRSA2048:
		return rsa.GenerateKey(rand.Reader, 2048)
	case KeyRSA4096:
		return rsa.GenerateKey(rand.Reader, 4096)
	default:
		return nil, fmt.Errorf("unknown key type %q (allowed: %s)", keyType, strings.Join(KeyTypes, ", "))
	}
}

// SelfSigned makes a certificate that vouches for itself, immediately usable.
//
// This is the one-click HTTPS of a laboratory or a first integration: the
// browser will warn, and it is right to - nobody vouched for anything. What it
// buys is a real TLS handshake, real secure cookies, and passkeys that work,
// today, without waiting for an authority.
func SelfSigned(req Request, now time.Time) (*Material, error) {
	ips, err := req.normalize()
	if err != nil {
		return nil, err
	}
	key, err := newKey(req.KeyType)
	if err != nil {
		return nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("drawing a serial number: %w", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      req.pkixName(),
		DNSNames:     req.DNSNames,
		IPAddresses:  ips,
		// An hour of slack before: a freshly generated certificate refused as
		// "not yet valid" by a client whose clock runs a minute slow is a very
		// long afternoon.
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(0, 0, req.Days),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, key.Public(), key)
	if err != nil {
		return nil, fmt.Errorf("signing the certificate: %w", err)
	}
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM, err := MarshalKey(key)
	if err != nil {
		return nil, err
	}
	return ParsePEM(certPEM, keyPEM)
}

// NewCSR generates a private key and the signing request that goes with it.
//
// This is the enterprise door, and the offline one: the request leaves, the
// KEY never does. Whoever runs the company authority signs the request and
// hands a certificate back, which Adopt then marries to the key that stayed
// here. No network, no authority Meerkat has to know how to talk to.
func NewCSR(req Request) (csrPEM, keyPEM string, err error) {
	ips, err := req.normalize()
	if err != nil {
		return "", "", err
	}
	key, err := newKey(req.KeyType)
	if err != nil {
		return "", "", err
	}
	tmpl := &x509.CertificateRequest{
		Subject:     req.pkixName(),
		DNSNames:    req.DNSNames,
		IPAddresses: ips,
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, tmpl, key)
	if err != nil {
		return "", "", fmt.Errorf("building the signing request: %w", err)
	}
	csrPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: der}))
	if keyPEM, err = MarshalKey(key); err != nil {
		return "", "", err
	}
	return csrPEM, keyPEM, nil
}

// CSRInfo is what a pending request asked for, so a console can show a row
// that has no certificate yet.
type CSRInfo struct {
	Subject     string   `json:"subject"`
	DNSNames    []string `json:"dnsNames"`
	IPAddresses []string `json:"ipAddresses"`
	KeyType     string   `json:"keyType"`
}

// ParseCSR reads back a request Meerkat produced.
func ParseCSR(csrPEM string) (CSRInfo, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(csrPEM)))
	if block == nil {
		return CSRInfo{}, fmt.Errorf("no PEM block: expected one starting with -----BEGIN CERTIFICATE REQUEST-----")
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		return CSRInfo{}, fmt.Errorf("unreadable signing request: %w", err)
	}
	ips := make([]string, 0, len(csr.IPAddresses))
	for _, ip := range csr.IPAddresses {
		ips = append(ips, ip.String())
	}
	return CSRInfo{
		Subject:     csr.Subject.String(),
		DNSNames:    append([]string{}, csr.DNSNames...),
		IPAddresses: ips,
		KeyType:     KeyTypeOf(csr.PublicKey),
	}, nil
}

// Adopt marries a certificate that came back from an authority to the private
// key that never left.
//
// The check that matters is the key: paste the certificate from the wrong
// order - the one signed last month, a colleague's, the right domain but the
// previous request - and TLS fails at handshake time with a message nobody
// connects to this screen. Refusing here, by name, is the whole point of
// having a signing-request flow rather than a paste box.
func Adopt(keyPEM, signedPEM string) (*Material, error) {
	m, err := ParsePEM(signedPEM, keyPEM)
	if err != nil {
		if strings.Contains(err.Error(), "two halves of two different pairs") {
			return nil, fmt.Errorf("this certificate was not signed from this request: its public key is not the one Meerkat generated, so the private key held here cannot serve it")
		}
		return nil, err
	}
	return m, nil
}
