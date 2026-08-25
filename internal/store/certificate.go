package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
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
)

// ValidCertSource reports whether s is one of the three.
func ValidCertSource(s string) bool {
	return s == CertSourceImport || s == CertSourceSelfSigned || s == CertSourceCSR
}

// Planes a certificate can belong to.
const (
	PlaneConsole = "console"
	PlaneApp     = "app"
)

// ValidCertPlane reports whether p is one of the two.
func ValidCertPlane(p string) bool { return p == PlaneConsole || p == PlaneApp }

// Certificate is ONE host's certificate.
//
// It is not floating material to be matched against a list of names later: the
// host is part of the row. The console has one, the application has one per
// host it serves, and material used by both is added twice. Two entries and
// two keys are cheaper to understand than one shared object plus the rule that
// works out which name it covers.
type Certificate struct {
	ID string `json:"id"`
	// Plane and Host are what this certificate is FOR. There is no separate
	// label: the host IS the name of the thing, and a second one to type would
	// only be a second one to keep in step.
	Plane     string     `json:"plane"`
	Host      string     `json:"host"`
	Source    string     `json:"source"`
	Info      certs.Info `json:"info"`
	CreatedAt int64      `json:"createdAt"`
	UpdatedAt int64      `json:"updatedAt"`

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

// SaveCertificate inserts or updates one row. The key is sealed here, once,
// so no caller has to remember to.
func (s *Store) SaveCertificate(ctx context.Context, c Certificate) error {
	c.Host = strings.ToLower(strings.TrimSpace(c.Host))
	if c.ID == "" {
		return fmt.Errorf("store: a certificate needs an id")
	}
	if c.Host == "" {
		return fmt.Errorf("store: a certificate needs a host - it belongs to the name it answers for")
	}
	if !ValidCertPlane(c.Plane) {
		return fmt.Errorf("store: certificate %q: unknown plane %q (allowed: %s, %s)",
			c.Host, c.Plane, PlaneConsole, PlaneApp)
	}
	if !ValidCertSource(c.Source) {
		return fmt.Errorf("store: certificate %q: unknown source %q (allowed: %s, %s, %s)",
			c.Host, c.Source, CertSourceImport, CertSourceSelfSigned, CertSourceCSR)
	}
	if c.KeyPEM == "" {
		return fmt.Errorf("store: certificate %q: no private key", c.Host)
	}
	sealed, err := s.vaultCipher.Seal(c.KeyPEM)
	if err != nil {
		return fmt.Errorf("store: certificate %q: sealing the private key: %w", c.Host, err)
	}
	dns, err := json.Marshal(nonNil(c.Info.DNSNames))
	if err != nil {
		return fmt.Errorf("store: certificate %q: %w", c.Host, err)
	}
	ips, err := json.Marshal(nonNil(c.Info.IPAddresses))
	if err != nil {
		return fmt.Errorf("store: certificate %q: %w", c.Host, err)
	}
	now := time.Now().Unix()
	if c.CreatedAt == 0 {
		c.CreatedAt = now
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO certificates (id, plane, host, source, cert_pem, key_sealed, csr_pem,
		   subject, issuer, serial, algo, key_type, dns_names, ip_addresses, chain, self_signed,
		   not_before, not_after, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   plane = excluded.plane, host = excluded.host,
		   source = excluded.source, cert_pem = excluded.cert_pem,
		   key_sealed = excluded.key_sealed, csr_pem = excluded.csr_pem, subject = excluded.subject,
		   issuer = excluded.issuer, serial = excluded.serial, algo = excluded.algo,
		   key_type = excluded.key_type, dns_names = excluded.dns_names,
		   ip_addresses = excluded.ip_addresses, chain = excluded.chain,
		   self_signed = excluded.self_signed, not_before = excluded.not_before,
		   not_after = excluded.not_after, updated_at = excluded.updated_at`,
		c.ID, c.Plane, c.Host, c.Source, c.CertPEM, sealed, c.CSRPEM, c.Info.Subject,
		c.Info.Issuer, c.Info.Serial, c.Info.Algo, c.Info.KeyType, string(dns), string(ips),
		c.Info.Chain, c.Info.SelfSigned, c.Info.NotBefore, c.Info.NotAfter, c.CreatedAt, now)
	if err != nil {
		return fmt.Errorf("store: save certificate %q: %w", c.Host, err)
	}
	return nil
}

const certCols = `id, plane, host, source, cert_pem, key_sealed, csr_pem, subject, issuer,
	serial, algo, key_type, dns_names, ip_addresses, chain, self_signed, not_before, not_after,
	created_at, updated_at`

func (s *Store) scanCertificate(sc scanner) (Certificate, error) {
	var c Certificate
	var sealed, dns, ips string
	if err := sc.Scan(&c.ID, &c.Plane, &c.Host, &c.Source, &c.CertPEM, &sealed, &c.CSRPEM,
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
	plain, err := s.vaultCipher.Open(sealed)
	if err != nil {
		return c, fmt.Errorf("store: certificate %q: the private key cannot be unsealed - is this the vault key it was written with? %w", c.ID, err)
	}
	c.KeyPEM = plain
	return c, nil
}

// ListCertificates returns every row, newest first among equals: an operator
// looking at this screen has just added something.
func (s *Store) ListCertificates(ctx context.Context) ([]Certificate, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+certCols+` FROM certificates ORDER BY plane, host COLLATE NOCASE`)
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

// CertificateMaterials parses one plane's rows into what its TLS manager
// holds. A row that cannot be parsed does not sink the others: it is named in
// the returned problems and left out, because one corrupt certificate must not
// take a whole HTTPS door down with it.
//
// The fallback - what answers a handshake carrying no server name, from a
// client that came by IP address - is simply the FIRST entry of the plane.
// There is no star to tick: with one certificate per host, asking an operator
// to also nominate a default would be asking about a mechanism they were never
// shown.
func (s *Store) CertificateMaterials(ctx context.Context, plane string) (list []*certs.Material, fallback *certs.Material, problems []string, err error) {
	rows, err := s.ListCertificates(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	for _, c := range rows {
		if c.Plane != plane || c.Pending() {
			continue
		}
		m, perr := certs.ParsePEM(c.CertPEM, c.KeyPEM)
		if perr != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", c.Host, perr))
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
	return cfg
}

// TLSSettings reads them RESOLVED, which is what the supervisor runs on. The
// external-account HMAC key is the one secret here, and it follows the rule
// every secret follows: stored as a $name, expanded only when used.
//
// The console defaults to localhost: a first start has to have a name to offer,
// and localhost is the one every installation is reachable by before anyone has
// decided anything.
func (s *Store) TLSSettings(ctx context.Context) certs.Settings {
	cfg := s.RawTLS(ctx)
	cfg.ACME.EABHMACKey = s.ExpandInfra(ctx, cfg.ACME.EABHMACKey)
	if cfg.ConsoleName == "" {
		cfg.ConsoleName = DefaultConsoleName
	}
	return cfg
}

// DefaultConsoleName is what the console answers to before anyone says
// otherwise. Every installation is reachable by it on the first day.
const DefaultConsoleName = "localhost"
