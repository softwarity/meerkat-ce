package store

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Probe is what a PostgreSQL server says about itself to a gateway about to
// copy into it (Configuration > Snapshot, Test).
type Probe struct {
	// Version is the server's own version string, short.
	Version string `json:"version"`
	// SSLMode is the mode that connected: the one asked for, or, when none was
	// or prefer was, require if the server takes TLS and disable otherwise.
	SSLMode string `json:"sslmode"`
	// SSL says the connection that answered is encrypted.
	SSL bool `json:"ssl"`
	// Empty is a database with no gateway in it - no Meerkat schema, or one
	// with no account and no route. What a copy needs.
	Empty bool `json:"empty"`
	// Schema is the version of a Meerkat schema already there, 0 when none.
	Schema int `json:"schema"`
	// CanCreate says the user may create tables where the copy will: since
	// PostgreSQL 15 only the owner of the public schema may, by default.
	CanCreate bool `json:"canCreate"`
}

// ProbePostgres connects to url and reads, never writes: no schema is created
// and nothing is left behind, which is what lets it run before a pause and as
// often as one likes.
func ProbePostgres(ctx context.Context, raw string) (Probe, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	u, err := url.Parse(raw)
	if err != nil {
		return Probe{}, fmt.Errorf("url: %w", err)
	}
	asked := u.Query().Get("sslmode")
	// Asked for nothing, or for prefer: try encrypted first, and say so. A
	// server that takes TLS should be spoken to over it; one that does not
	// answers that it does not, and plain is tried next.
	modes := []string{asked}
	if asked == "" || asked == "prefer" {
		modes = []string{"require", "disable"}
	}
	var last error
	for _, mode := range modes {
		p, err := probeWith(ctx, u, mode)
		if err == nil {
			return p, nil
		}
		last = err
		if !strings.Contains(strings.ToLower(err.Error()), "ssl") && !strings.Contains(strings.ToLower(err.Error()), "tls") {
			break // not a question of TLS: trying plain would not answer differently
		}
	}
	return Probe{}, last
}

func probeWith(ctx context.Context, u *url.URL, mode string) (Probe, error) {
	q := u.Query()
	q.Set("sslmode", mode)
	v := *u
	v.RawQuery = q.Encode()
	db, err := sql.Open("pgx", v.String())
	if err != nil {
		return Probe{}, err
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)
	p := Probe{SSLMode: mode}
	if err := db.QueryRowContext(ctx, `SELECT split_part(version(), ' ', 2)`).Scan(&p.Version); err != nil {
		return Probe{}, err
	}
	_ = db.QueryRowContext(ctx, `SELECT COALESCE((SELECT ssl FROM pg_stat_ssl WHERE pid = pg_backend_pid()), false)`).Scan(&p.SSL)
	_ = db.QueryRowContext(ctx,
		`SELECT has_database_privilege(current_database(), 'CREATE') AND has_schema_privilege(current_schema(), 'CREATE')`).Scan(&p.CanCreate)
	var users sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT to_regclass('users')::text`).Scan(&users); err != nil {
		return Probe{}, err
	}
	if !users.Valid {
		p.Empty = true
		return p, nil
	}
	_ = db.QueryRowContext(ctx, `SELECT COALESCE((SELECT version FROM schema_version LIMIT 1), 0)`).Scan(&p.Schema)
	var n int64
	if err := db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM users) + (SELECT COUNT(*) FROM routes)`).Scan(&n); err != nil {
		return Probe{}, err
	}
	p.Empty = n == 0
	return p, nil
}
