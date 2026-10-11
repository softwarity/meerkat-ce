package certs

import (
	"crypto"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The certificates a runtime MOUNTED in this container (SSL-11): a Kubernetes
// Secret, a Swarm secret, a compose volume. Not guessed from directories that
// may or may not exist - read from the mount table, which lists what the
// deployment actually handed this process, wherever its manifest put it.

// MountInfoPath is the kernel's own list of this process's mounts. A variable
// so a test reads a fixture.
var MountInfoPath = "/proc/self/mountinfo"

// MountedCert is one certificate found in a mount, as the console shows it:
// what it is, never what it holds.
type MountedCert struct {
	Path     string   `json:"path"`
	KeyPath  string   `json:"keyPath,omitempty"`
	Names    []string `json:"names"`
	Subject  string   `json:"subject"`
	Issuer   string   `json:"issuer"`
	NotAfter int64    `json:"notAfter"`
}

// MountedPKI is one mount that holds at least one certificate.
type MountedPKI struct {
	Mount        string        `json:"mount"`
	Certificates []MountedCert `json:"certificates"`
}

// neverPKI are the mounts no deployment puts a certificate in, and some it
// must never be looked into: the process's own pseudo-files, what the runtime
// writes for every container, and the service account's token.
var neverPKI = []string{
	"/proc", "/sys", "/dev",
	"/etc/hosts", "/etc/hostname", "/etc/resolv.conf",
	"/run/secrets/kubernetes.io", "/var/run/secrets/kubernetes.io",
}

// ParseMountInfo lists the mount points of a mountinfo text, unescaped, minus
// the root and the ones that are never a PKI, minus the given excluded paths
// (the data directory: the database and the vault key are not to be browsed).
func ParseMountInfo(text string, exclude ...string) []string {
	var out []string
	seen := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		f := strings.Fields(line)
		if len(f) < 5 {
			continue
		}
		mp := unescapeMount(f[4])
		if mp == "/" || seen[mp] || under(mp, neverPKI) || under(mp, exclude) {
			continue
		}
		seen[mp] = true
		out = append(out, mp)
	}
	sort.Strings(out)
	return out
}

// under says whether p is one of roots or inside one.
func under(p string, roots []string) bool {
	for _, r := range roots {
		r = filepath.Clean(r)
		if r == "" || r == "." {
			continue
		}
		if p == r || strings.HasPrefix(p, r+"/") {
			return true
		}
	}
	return false
}

// unescapeMount undoes the octal escapes mountinfo writes for a space, a tab,
// a newline and a backslash.
func unescapeMount(s string) string {
	r := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`)
	return r.Replace(s)
}

// MountedPKIs reads the mount table and looks into each mount for
// certificates, each paired with the private key beside it when there is one.
// An empty list with ok false means there is no mount table at all: this is
// not a Linux container.
func MountedPKIs(exclude ...string) ([]MountedPKI, bool) {
	raw, err := os.ReadFile(MountInfoPath)
	if err != nil {
		return nil, false
	}
	out := []MountedPKI{}
	for _, mp := range ParseMountInfo(string(raw), exclude...) {
		if found := ScanPKI(mp); len(found) > 0 {
			out = append(out, MountedPKI{Mount: mp, Certificates: found})
		}
	}
	return out, true
}

// The bounds of a look into one mount: shallow, small files, few of them. A
// mount is a Secret or a volume, not a disk to crawl.
const (
	scanDepth = 3
	scanFiles = 200
	scanSize  = 256 << 10
)

// ScanPKI looks into one mount - a file or a directory - for certificates that
// can serve (a leaf naming at least one host), and pairs each with the key in
// the same mount whose public half is its own. The ..data and ..timestamp
// entries a Kubernetes Secret volume carries are the same files twice, and
// are skipped.
func ScanPKI(root string) []MountedCert {
	var certsFound []MountedCert
	pubOf := map[string]string{}  // cert path -> public key fingerprint
	keyFor := map[string]string{} // public key fingerprint -> key path
	files := 0
	visit := func(path string) {
		if files >= scanFiles {
			return
		}
		st, err := os.Stat(path) // follows the symlinks a Secret volume is made of
		if err != nil || !st.Mode().IsRegular() || st.Size() > scanSize {
			return
		}
		files++
		raw, err := os.ReadFile(path)
		if err != nil {
			return
		}
		leaf, keyPub := inspectPEM(raw)
		if leaf != nil && (len(leaf.DNSNames) > 0 || len(leaf.IPAddresses) > 0) {
			names := append([]string{}, leaf.DNSNames...)
			for _, ip := range leaf.IPAddresses {
				names = append(names, ip.String())
			}
			certsFound = append(certsFound, MountedCert{
				Path: path, Names: names, Subject: leaf.Subject.String(),
				Issuer: leaf.Issuer.String(), NotAfter: leaf.NotAfter.Unix(),
			})
			pubOf[path] = publicFingerprint(leaf.PublicKey)
		}
		if keyPub != "" {
			if _, taken := keyFor[keyPub]; !taken {
				keyFor[keyPub] = path
			}
		}
	}
	st, err := os.Stat(root)
	if err != nil {
		return nil
	}
	if !st.IsDir() {
		visit(root)
	} else {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil //nolint:nilerr // an unreadable corner is skipped, not fatal
			}
			if path != root && strings.HasPrefix(d.Name(), "..") {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			if d.IsDir() {
				if path != root && strings.Count(rel, string(filepath.Separator))+1 >= scanDepth {
					return filepath.SkipDir
				}
				return nil
			}
			visit(path)
			return nil
		})
	}
	for i := range certsFound {
		certsFound[i].KeyPath = keyFor[pubOf[certsFound[i].Path]]
	}
	sort.Slice(certsFound, func(i, j int) bool { return certsFound[i].Path < certsFound[j].Path })
	return certsFound
}

// inspectPEM reads a file's PEM blocks: the first certificate, and the public
// half of a private key if the file holds one.
func inspectPEM(raw []byte) (*x509.Certificate, string) {
	var leaf *x509.Certificate
	keyPub := ""
	rest := raw
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		switch {
		case block.Type == "CERTIFICATE" && leaf == nil:
			if c, err := x509.ParseCertificate(block.Bytes); err == nil {
				leaf = c
			}
		case strings.Contains(block.Type, "PRIVATE KEY") && keyPub == "":
			if k := parsePrivate(block.Bytes); k != nil {
				keyPub = publicFingerprint(k.Public())
			}
		}
	}
	return leaf, keyPub
}

func parsePrivate(der []byte) crypto.Signer {
	if k, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		if s, ok := k.(crypto.Signer); ok {
			return s
		}
	}
	if k, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return k
	}
	if k, err := x509.ParseECPrivateKey(der); err == nil {
		return k
	}
	return nil
}

func publicFingerprint(pub any) string {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:])
}
