package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/softwarity/meerkat/internal/certs"
)

// TLS material (SSL-01/02/03). The four ways a certificate reaches Meerkat,
// and none of them is an edition feature: a gateway that cannot serve HTTPS is
// not a gateway, and the community image runs in front of production too.

// Certificate sources - how this row's material got here.
const (
	// CertSourceImport is material that already existed: a PEM from a CA, a
	// .p12 from a Windows or Java world.
	CertSourceImport = "import"
	// CertSourceSelfSigned vouches for itself. Honest, immediate, warned about.
	CertSourceSelfSigned = "self-signed"
	// CertSourceCSR was generated here as a request. It serves nothing until an
	// authority signs it and the answer is adopted - the private key never left.
	CertSourceCSR = "csr"
	// CertSourceFile is read from the file system (SSL-11): a certificate and
	// its key that a runtime mounted. Only the paths are stored; the key never
	// enters the database.
	CertSourceFile = "file"
	// CertSourceCertManager is issued by cert-manager at the gateway's request
	// (SSL-13): a Certificate in its own namespace, whose Secret is read
	// through the API and watched. Only the Secret's name is stored; the key
	// stays in the Secret, and in each node's memory.
	CertSourceCertManager = "cert-manager"
	// CertSourceACME is an order placed with the automatic authority. It never
	// is a row of this table - the order lives in the TLS settings, the
	// material in the ACME cache - but it sits in the same pool on screen, so
	// it is named the same way. ValidCertSource refuses it on purpose.
	CertSourceACME = "acme"
)

// ValidCertSource reports whether s is one of the rows' sources.
func ValidCertSource(s string) bool {
	return s == CertSourceImport || s == CertSourceSelfSigned || s == CertSourceCSR || s == CertSourceFile ||
		s == CertSourceCertManager
}

// KeyElsewhere reports whether the row's key is kept outside the database -
// in a mounted file, or in a Secret - so nothing of it is sealed or stored.
func (c Certificate) KeyElsewhere() bool {
	return c.Source == CertSourceFile || c.Source == CertSourceCertManager
}

// OnIssuedPair names where a certificate cert-manager issued is read on this
// node (discovery.CertManager.Pair).
func (s *Store) OnIssuedPair(f func(secret string) (certPEM, keyPEM string, ok bool)) {
	s.issuedPair.Store(&f)
}

func (s *Store) issued(secret string) (string, string, bool) {
	if f := s.issuedPair.Load(); f != nil {
		return (*f)(secret)
	}
	return "", "", false
}

// Planes a certificate can be placed on.
const (
	PlaneConsole  = "console"
	PlaneApp      = "app"
	PlaneInternal = "internal"
)

// CertPlanes are the three, in the order the screen draws them.
var CertPlanes = []string{PlaneConsole, PlaneApp, PlaneInternal}

// ValidCertPlane reports whether p is one of the three.
func ValidCertPlane(p string) bool {
	return p == PlaneConsole || p == PlaneApp || p == PlaneInternal
}

// Certificate is one piece of material in the pool (v73).
//
// The names are the certificate's own - what x509 says it answers for, read
// from the material - and not a label filed beside it. A certificate is made
// or imported once, then PLACED on the console, the application, or both: a
// wildcard or a certificate carrying several names serves every one of them,
// and material both planes use is one entry, not two copies to keep in step.
type Certificate struct {
	ID string `json:"id"`
	// Console and App say where the material is served. Neither is a
	// certificate in reserve: kept, shown, counted by the expiry digest, and
	// presented to nobody.
	Console bool `json:"console"`
	App     bool `json:"app"`
	// Internal is the internal plane's HTTPS door (SSL-12).
	Internal bool   `json:"internal"`
	Source   string `json:"source"`
	// CertPath and KeyPath are the files a CertSourceFile row is read from.
	CertPath string `json:"certPath,omitempty"`
	KeyPath  string `json:"keyPath,omitempty"`
	// SecretName is the Certificate, and its Secret, a CertSourceCertManager
	// row is issued into.
	SecretName string     `json:"secretName,omitempty"`
	Info       certs.Info `json:"info"`
	// Authority is the ACME authority an order (CertSourceACME) is asked of.
	// Never set on a row of the table.
	Authority string `json:"authority,omitempty"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`

	// CertPEM is public material by construction: it is what the gateway hands
	// to every visitor. It is excluded from the list payload only because a
	// table of twenty rows has no use for twenty PEM blocks.
	CertPEM string `json:"-"`
	// KeyPEM NEVER leaves the gateway. It is sealed at rest and it is not in
	// any payload, any export, or any log.
	KeyPEM string `json:"-"`
	// CSRPEM is the pending signing request, downloadable: that is the whole
	// point of it.
	CSRPEM string `json:"-"`
}

// Pending reports whether this row is a signing request still waiting for its
// answer. It holds a key and a request, and serves nothing.
func (c Certificate) Pending() bool { return c.CertPEM == "" && c.CSRPEM != "" }

// Names are what the material answers for, addresses included.
func (c Certificate) Names() []string {
	return append(append([]string{}, c.Info.DNSNames...), c.Info.IPAddresses...)
}

// Name is what a row is called in a sentence, a log line or a file name: its
// first name. Every certificate this gateway makes or accepts carries one.
func (c Certificate) Name() string {
	if n := c.Names(); len(n) > 0 {
		return n[0]
	}
	return c.ID
}

// On reports whether the material is placed on plane.
func (c Certificate) On(plane string) bool {
	switch plane {
	case PlaneConsole:
		return c.Console
	case PlaneInternal:
		return c.Internal
	}
	return c.App
}

// Placed reports whether it is served anywhere.
func (c Certificate) Placed() bool { return c.Console || c.App || c.Internal }

// SaveCertificate inserts or updates one row. The key is sealed here, once,
// so no caller has to remember to.
func (s *Store) SaveCertificate(ctx context.Context, c Certificate) error {
	if c.ID == "" {
		return fmt.Errorf("store: a certificate needs an id")
	}
	if !ValidCertSource(c.Source) {
		return fmt.Errorf("store: certificate %q: unknown source %q (allowed: %s, %s, %s, %s, %s)",
			c.Name(), c.Source, CertSourceImport, CertSourceSelfSigned, CertSourceCSR, CertSourceFile, CertSourceCertManager)
	}
	file := c.KeyElsewhere()
	if c.Source == CertSourceFile && (c.CertPath == "" || c.KeyPath == "") {
		return fmt.Errorf("store: certificate %q: read from the file system, it needs the path of its certificate and of its key", c.Name())
	}
	if c.Source == CertSourceCertManager && c.SecretName == "" {
		return fmt.Errorf("store: certificate %q: issued by cert-manager, it needs the name of its Secret", c.Name())
	}
	if !file && c.KeyPEM == "" {
		return fmt.Errorf("store: certificate %q: no private key", c.Name())
	}
	if c.Pending() && c.Placed() {
		return fmt.Errorf("store: certificate %q: a signing request serves nothing until its answer is adopted", c.Name())
	}
	// The key of a file certificate stays in its file, the one cert-manager
	// issued in its Secret: nothing is sealed, and nothing of it is stored.
	sealed := ""
	if !file {
		var err error
		if sealed, err = s.vaultCipher.Seal(c.KeyPEM); err != nil {
			return fmt.Errorf("store: certificate %q: sealing the private key: %w", c.Name(), err)
		}
	}
	dns, err := json.Marshal(nonNil(c.Info.DNSNames))
	if err != nil {
		return fmt.Errorf("store: certificate %q: %w", c.Name(), err)
	}
	ips, err := json.Marshal(nonNil(c.Info.IPAddresses))
	if err != nil {
		return fmt.Errorf("store: certificate %q: %w", c.Name(), err)
	}
	now := time.Now().Unix()
	if c.CreatedAt == 0 {
		c.CreatedAt = now
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO certificates (id, on_console, on_app, on_internal, source, cert_path, key_path, secret_name,
		   cert_pem, key_sealed, csr_pem,
		   subject, issuer, serial, algo, key_type, dns_names, ip_addresses, chain, self_signed,
		   not_before, not_after, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   on_console = excluded.on_console, on_app = excluded.on_app,
		   on_internal = excluded.on_internal, cert_path = excluded.cert_path,
		   key_path = excluded.key_path, secret_name = excluded.secret_name,
		   source = excluded.source, cert_pem = excluded.cert_pem,
		   key_sealed = excluded.key_sealed, csr_pem = excluded.csr_pem, subject = excluded.subject,
		   issuer = excluded.issuer, serial = excluded.serial, algo = excluded.algo,
		   key_type = excluded.key_type, dns_names = excluded.dns_names,
		   ip_addresses = excluded.ip_addresses, chain = excluded.chain,
		   self_signed = excluded.self_signed, not_before = excluded.not_before,
		   not_after = excluded.not_after, updated_at = excluded.updated_at`,
		c.ID, c.Console, c.App, c.Internal, c.Source, c.CertPath, c.KeyPath, c.SecretName, c.CertPEM, sealed, c.CSRPEM, c.Info.Subject,
		c.Info.Issuer, c.Info.Serial, c.Info.Algo, c.Info.KeyType, string(dns), string(ips),
		c.Info.Chain, c.Info.SelfSigned, c.Info.NotBefore, c.Info.NotAfter, c.CreatedAt, now)
	if err != nil {
		return fmt.Errorf("store: save certificate %q: %w", c.Name(), err)
	}
	return nil
}

// PlaceCertificate sets where one row is served, and nothing else: the
// material does not move, so the key is not resealed for a click.
func (s *Store) PlaceCertificate(ctx context.Context, id string, console, app, internal bool) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE certificates SET on_console = ?, on_app = ?, on_internal = ?, updated_at = ? WHERE id = ?`,
		console, app, internal, time.Now().Unix(), id)
	if err != nil {
		return fmt.Errorf("store: place certificate: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

const certCols = `id, on_console, on_app, on_internal, source, cert_path, key_path, secret_name, cert_pem, key_sealed, csr_pem, subject, issuer,
	serial, algo, key_type, dns_names, ip_addresses, chain, self_signed, not_before, not_after,
	created_at, updated_at`

func (s *Store) scanCertificate(sc scanner) (Certificate, error) {
	var c Certificate
	var sealed, dns, ips string
	if err := sc.Scan(&c.ID, &c.Console, &c.App, &c.Internal, &c.Source, &c.CertPath, &c.KeyPath, &c.SecretName, &c.CertPEM, &sealed, &c.CSRPEM,
		&c.Info.Subject, &c.Info.Issuer, &c.Info.Serial, &c.Info.Algo, &c.Info.KeyType, &dns, &ips,
		&c.Info.Chain, &c.Info.SelfSigned, &c.Info.NotBefore, &c.Info.NotAfter,
		&c.CreatedAt, &c.UpdatedAt); err != nil {
		return c, err
	}
	if err := json.Unmarshal([]byte(dns), &c.Info.DNSNames); err != nil {
		return c, fmt.Errorf("store: certificate %q: bad dns names: %w", c.ID, err)
	}
	if err := json.Unmarshal([]byte(ips), &c.Info.IPAddresses); err != nil {
		return c, fmt.Errorf("store: certificate %q: bad ip addresses: %w", c.ID, err)
	}
	if c.KeyElsewhere() {
		return c, nil
	}
	plain, err := s.vaultCipher.Open(sealed)
	if err != nil {
		return c, fmt.Errorf("store: certificate %q: the private key cannot be unsealed - is this the vault key it was written with? %w", c.ID, err)
	}
	c.KeyPEM = plain
	return c, nil
}

// ListCertificates returns the pool in the order it was filled. The order is
// not cosmetic: the first certificate placed on a plane is that plane's
// fallback (see CertificateMaterials).
func (s *Store) ListCertificates(ctx context.Context) ([]Certificate, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+certCols+` FROM certificates ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("store: list certificates: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []Certificate{}
	for rows.Next() {
		c, err := s.scanCertificate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// FindCertificate reads one row.
func (s *Store) FindCertificate(ctx context.Context, id string) (Certificate, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+certCols+` FROM certificates WHERE id = ?`, id)
	c, err := s.scanCertificate(row)
	if err != nil {
		return Certificate{}, err
	}
	return c, nil
}

// DeleteCertificate removes one row.
func (s *Store) DeleteCertificate(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM certificates WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("store: delete certificate: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// CertificateMaterials parses what is placed on one plane into what its TLS
// manager holds. A row that cannot be parsed does not sink the others: it is
// named in the returned problems and left out, because one corrupt
// certificate must not take a whole HTTPS door down with it.
//
// The fallback - what answers a handshake carrying no server name, from a
// client that came by IP address - is the oldest certificate placed there.
// Not a star to tick: the oldest is the one that has been answering, and a
// certificate added next to it must not change what those clients receive.
func (s *Store) CertificateMaterials(ctx context.Context, plane string) (list []*certs.Material, fallback *certs.Material, problems []string, err error) {
	rows, err := s.ListCertificates(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, c := range rows {
		if !c.On(plane) || c.Pending() {
			continue
		}
		certPEM, keyPEM := c.CertPEM, c.KeyPEM
		if c.Source == CertSourceFile {
			// Read now, on this node: the runtime mounts the same files in
			// every replica, and one that has not got them says so here.
			cb, kerr := os.ReadFile(c.CertPath)
			kb, rerr := os.ReadFile(c.KeyPath)
			if kerr != nil || rerr != nil {
				problems = append(problems, fmt.Sprintf("%s: the files %s and %s cannot both be read on this node - is the volume mounted here?",
					c.Name(), c.CertPath, c.KeyPath))
				continue
			}
			certPEM, keyPEM = string(cb), string(kb)
		}
		if c.Source == CertSourceCertManager {
			// The Secret as this node's watch last saw it.
			var ok bool
			if certPEM, keyPEM, ok = s.issued(c.SecretName); !ok {
				problems = append(problems, fmt.Sprintf("%s: cert-manager has not written the Secret %s yet, or this node cannot read it",
					c.Name(), c.SecretName))
				continue
			}
		}
		m, perr := certs.ParsePEM(certPEM, keyPEM)
		if perr != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", c.Name(), perr))
			continue
		}
		list = append(list, m)
		if fallback == nil {
			fallback = m
		}
	}
	return list, fallback, problems, nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// The ACME cache, as the three raw operations autocert needs. The values are
// sealed: this table holds the account key that signs on the gateway's behalf,
// and the private keys of every certificate an authority issued.
//
// The autocert vocabulary (a cache miss is a sentinel error, not an empty
// value) stays in the certs package; the store only reports absence.

// ACMECacheGet reads one entry. Absence is sql.ErrNoRows, like everywhere else.
func (s *Store) ACMECacheGet(ctx context.Context, key string) (string, error) {
	var sealed string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM acme_cache WHERE key = ?`, key).Scan(&sealed)
	if err != nil {
		return "", err
	}
	plain, err := s.vaultCipher.Open(sealed)
	if err != nil {
		return "", fmt.Errorf("store: acme cache %q: %w", key, err)
	}
	return plain, nil
}

// ACMECachePut writes one entry.
func (s *Store) ACMECachePut(ctx context.Context, key, value string) error {
	sealed, err := s.vaultCipher.Seal(value)
	if err != nil {
		return fmt.Errorf("store: acme cache %q: %w", key, err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO acme_cache (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, sealed, time.Now().Unix())
	if err != nil {
		return fmt.Errorf("store: acme cache %q: %w", key, err)
	}
	return nil
}

// ACMECacheDelete removes one entry.
func (s *Store) ACMECacheDelete(ctx context.Context, key string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM acme_cache WHERE key = ?`, key)
	if err != nil {
		return fmt.Errorf("store: acme cache %q: %w", key, err)
	}
	return nil
}

// ForgetACME empties the whole ACME state - the account key included. It is
// what "start again with another authority" means: keeping an account key
// registered at one directory while pointing at another is how a renewal fails
// months later with an error about an unknown account.
func (s *Store) ForgetACME(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM acme_cache`)
	if err != nil {
		return fmt.Errorf("store: forget acme state: %w", err)
	}
	return nil
}

// RawTLS reads the TLS settings AS STORED, references unexpanded: an admin
// form showing a resolved secret would send that secret back on the next save
// and quietly replace the reference with its value (VAULT-05).
func (s *Store) RawTLS(ctx context.Context) certs.Settings {
	var cfg certs.Settings
	_ = s.GetSetting(ctx, SettingTLS, &cfg)
	return cfg.Normalized()
}

// TLSSettings reads them RESOLVED, which is what the supervisor runs on. The
// external-account HMAC key of each authority is the one secret here, and it follows the rule
// every secret follows: stored as a $name, expanded only when used.
func (s *Store) TLSSettings(ctx context.Context) certs.Settings {
	cfg := s.RawTLS(ctx)
	for i := range cfg.ACME.Authorities {
		cfg.ACME.Authorities[i].EABHMACKey = s.ExpandInfra(ctx, cfg.ACME.Authorities[i].EABHMACKey)
	}
	return cfg
}

// RefreshFileCertificates reads the files of every certificate the file
// system provides (SSL-11) and, for each whose certificate changed since the
// last read - a renewal by cert-manager, a rotated Swarm secret - stores what
// the screen and the expiry digest show. It says whether anything moved, so
// the caller reloads the doors only then.
func (s *Store) RefreshFileCertificates(ctx context.Context) (bool, error) {
	rows, err := s.ListCertificates(ctx)
	if err != nil {
		return false, err
	}
	moved := false
	for _, c := range rows {
		if c.Source != CertSourceFile {
			continue
		}
		cb, err := os.ReadFile(c.CertPath)
		if err != nil || string(cb) == c.CertPEM {
			continue
		}
		// The key read beside it, only to check the pair still holds - a
		// renewal half-written is not stored as if it served.
		kb, err := os.ReadFile(c.KeyPath)
		if err != nil {
			continue
		}
		m, err := certs.ParsePEM(string(cb), string(kb))
		if err != nil {
			continue
		}
		if err := s.storeMaterialInfo(ctx, c, string(cb), m.Info); err != nil {
			return moved, err
		}
		moved = true
	}
	return moved, nil
}

// RefreshIssuedCertificate stores what the Secret of a certificate
// cert-manager issued now holds (SSL-13), for the screen and the expiry
// digest: a first issuance, or a renewal. Every node's watch calls it; the
// first one writes, the others find the same certificate and write nothing.
// It says whether anything moved.
func (s *Store) RefreshIssuedCertificate(ctx context.Context, secret, certPEM, keyPEM string) (bool, error) {
	rows, err := s.ListCertificates(ctx)
	if err != nil {
		return false, err
	}
	moved := false
	for _, c := range rows {
		if c.Source != CertSourceCertManager || c.SecretName != secret || c.CertPEM == certPEM {
			continue
		}
		m, err := certs.ParsePEM(certPEM, keyPEM)
		if err != nil {
			continue // half-written: the next event carries the whole pair
		}
		if err := s.storeMaterialInfo(ctx, c, certPEM, m.Info); err != nil {
			return moved, err
		}
		moved = true
	}
	return moved, nil
}

// storeMaterialInfo writes a new certificate, and what is read from it, onto
// a row whose key lives elsewhere.
func (s *Store) storeMaterialInfo(ctx context.Context, c Certificate, certPEM string, info certs.Info) error {
	if _, err := s.db.ExecContext(ctx,
		`UPDATE certificates SET cert_pem = ?, subject = ?, issuer = ?, serial = ?, algo = ?,
		   key_type = ?, dns_names = ?, ip_addresses = ?, chain = ?, self_signed = ?,
		   not_before = ?, not_after = ?, updated_at = ? WHERE id = ?`,
		certPEM, info.Subject, info.Issuer, info.Serial, info.Algo, info.KeyType,
		jsonList(info.DNSNames), jsonList(info.IPAddresses), info.Chain, info.SelfSigned,
		info.NotBefore, info.NotAfter, time.Now().Unix(), c.ID); err != nil {
		return fmt.Errorf("store: refresh certificate %q: %w", c.Name(), err)
	}
	return nil
}

func jsonList(v []string) string {
	b, _ := json.Marshal(nonNil(v))
	return string(b)
}

// FileCertificatesSum is a fingerprint of what the file system holds for the
// certificates it provides, as THIS node reads it: each node watches its own
// mounts, since one node writing the renewal to the database tells the others
// nothing about what their own doors present.
func (s *Store) FileCertificatesSum(ctx context.Context) string {
	rows, err := s.ListCertificates(ctx)
	if err != nil {
		return ""
	}
	h := sha256.New()
	for _, c := range rows {
		if c.Source != CertSourceFile {
			continue
		}
		_, _ = h.Write([]byte(c.ID + c.CertPath + c.KeyPath))
		for _, p := range []string{c.CertPath, c.KeyPath} {
			if b, err := os.ReadFile(p); err == nil {
				_, _ = h.Write(b)
			}
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}
