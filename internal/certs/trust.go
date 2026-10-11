package certs

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
)

// The authorities the gateway trusts when IT calls (SSL-09). Not the pool of
// certificates it presents: those carry a private key and are served, these
// are public certificates and only ever verify somebody else.

// ParseAuthorities reads a PEM bundle of one or more certificates. A block
// that is a private key is refused rather than skipped: somebody pasted the
// wrong file, and what they meant was probably the certificate pool.
func ParseAuthorities(data []byte) ([]*x509.Certificate, error) {
	var out []*x509.Certificate
	rest := data
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if strings.Contains(block.Type, "PRIVATE KEY") {
			return nil, errors.New("this holds a private key: a trusted authority is a public certificate only, " +
				"and a certificate the gateway presents belongs in the certificate pool")
		}
		if block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("a %q block is not a certificate: expected blocks starting with -----BEGIN CERTIFICATE-----", block.Type)
		}
		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("certificate %d does not read: %w", len(out)+1, err)
		}
		out = append(out, c)
	}
	if len(out) == 0 {
		return nil, errors.New("no certificate found: expected PEM blocks starting with -----BEGIN CERTIFICATE-----")
	}
	return out, nil
}

// AuthorityCert is one certificate of a bundle, as the console shows it.
type AuthorityCert struct {
	Subject     string `json:"subject"`
	Issuer      string `json:"issuer"`
	NotAfter    int64  `json:"notAfter"`
	IsCA        bool   `json:"isCa"`
	Fingerprint string `json:"fingerprint"`
}

// DescribeAuthorities says what each certificate of a bundle is.
func DescribeAuthorities(certs []*x509.Certificate) []AuthorityCert {
	out := make([]AuthorityCert, 0, len(certs))
	for _, c := range certs {
		sum := sha256.Sum256(c.Raw)
		out = append(out, AuthorityCert{
			Subject: c.Subject.String(), Issuer: c.Issuer.String(), NotAfter: c.NotAfter.Unix(),
			IsCA: c.IsCA, Fingerprint: strings.ToUpper(hex.EncodeToString(sum[:])),
		})
	}
	return out
}
