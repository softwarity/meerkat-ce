package certs

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"
)

func trustTestCA(t *testing.T, name string) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: name},
		NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

// TestABundleReadsAndAKeyIsRefused: two authorities in one file (the old one
// and the new one, through a rotation) read as two; a private key is refused
// with the place it belongs; nothing readable is said as such.
func TestABundleReadsAndAKeyIsRefused(t *testing.T) {
	bundle := append(trustTestCA(t, "old"), trustTestCA(t, "new")...)
	got, err := ParseAuthorities(bundle)
	if err != nil || len(got) != 2 {
		t.Fatalf("a bundle of two: %d %v", len(got), err)
	}
	if d := DescribeAuthorities(got); d[1].Subject != "CN=new" || !d[1].IsCA || len(d[1].Fingerprint) != 64 {
		t.Fatalf("described: %+v", d[1])
	}
	key := []byte("-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n")
	if _, err := ParseAuthorities(append(trustTestCA(t, "x"), key...)); err == nil || !strings.Contains(err.Error(), "certificate pool") {
		t.Fatalf("a private key: %v", err)
	}
	if _, err := ParseAuthorities([]byte("not a pem")); err == nil {
		t.Fatal("garbage was accepted")
	}
}
