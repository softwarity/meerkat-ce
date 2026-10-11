package certs

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

// TestTheMountTableIsReadForWhatTheDeploymentMounted: the root, the pseudo
// file systems, the files every container gets, the service account and the
// data directory are left out; what a manifest mounted stays, unescaped.
func TestTheMountTableIsReadForWhatTheDeploymentMounted(t *testing.T) {
	info := `2210 2155 0:345 / / rw,relatime master:766 - overlay overlay rw
2211 2210 0:347 / /proc rw,nosuid - proc proc rw
2213 2210 0:348 / /dev rw,nosuid - tmpfs tmpfs rw
2222 2210 254:1 /docker/volumes/x /data rw,relatime - ext4 /dev/vda1 rw
2223 2210 254:1 /pods/a/etc-hosts /etc/hosts rw - ext4 /dev/vda1 rw
2224 2210 0:330 / /etc/tls/internal ro,relatime - tmpfs tmpfs rw
2225 2210 0:331 / /run/secrets/kubernetes.io/serviceaccount ro - tmpfs tmpfs rw
2226 2210 254:1 /secrets/app /run/secrets/app\040cert ro - ext4 /dev/vda1 rw
`
	got := ParseMountInfo(info, "/data")
	want := []string{"/etc/tls/internal", "/run/secrets/app cert"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

// TestASecretVolumeIsScannedAndItsKeyPaired: a Kubernetes TLS Secret as the
// kubelet mounts it - files that are symlinks into a ..data directory - gives
// one certificate, paired with its key; the CA beside it serves nothing and is
// not offered.
func TestASecretVolumeIsScannedAndItsKeyPaired(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "..2026_10_10_01_00_00.123")
	if err := os.Mkdir(data, 0o700); err != nil {
		t.Fatal(err)
	}
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tpl := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "meerkat-internal"},
		DNSNames: []string{"meerkat-internal.shop.svc.cluster.local"}, NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour)}
	der, _ := x509.CreateCertificate(rand.Reader, tpl, tpl, &key.PublicKey, key)
	kder, _ := x509.MarshalPKCS8PrivateKey(key)
	write := func(name string, b []byte) {
		if err := os.WriteFile(filepath.Join(data, name), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("tls.crt", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	write("tls.key", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: kder}))
	write("ca.crt", trustTestCA(t, "cluster CA"))
	if err := os.Symlink(filepath.Base(data), filepath.Join(root, "..data")); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"tls.crt", "tls.key", "ca.crt"} {
		if err := os.Symlink(filepath.Join("..data", n), filepath.Join(root, n)); err != nil {
			t.Fatal(err)
		}
	}
	got := ScanPKI(root)
	if len(got) != 1 {
		t.Fatalf("found %+v", got)
	}
	c := got[0]
	if c.Path != filepath.Join(root, "tls.crt") || c.KeyPath != filepath.Join(root, "tls.key") ||
		c.Names[0] != "meerkat-internal.shop.svc.cluster.local" {
		t.Fatalf("found %+v", c)
	}
}
