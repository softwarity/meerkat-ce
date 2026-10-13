package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The last copy of this database into a PostgreSQL server (Configuration >
// Database).
//
// The copy is the middle of a migration, not its end: the deployment still
// has to be pointed at the new database, and the console prints how. That
// procedure used to live in the browser's memory, so a reload in the middle of
// a migration - the worst moment to lose it - left an operator with a paused
// gateway and no idea what came next. So the gateway remembers the copy: where
// it went and how much it carried, never the password, and the screen rebuilds
// the procedure from it on any node.
//
// Written PAST the pause, deliberately. The pause stops the writes a switch
// would lose; this row is about the switch itself, it is written into the
// database being LEFT, after the copy, and the new database never needs it.
//
// It stops being shown once it no longer describes this database's own move:
// on the database it was copied INTO (the switch is done), and on a database
// it merely travelled into with a later copy or snapshot (it is another
// database's story). Either way it is cleared on read.

// lastCopyKey is the setting the record lives under.
const lastCopyKey = "database_last_copy"

// LastCopy is what the gateway remembers of a copy. No password, by
// construction: there is no field to put one in.
type LastCopy struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	User     string `json:"user"`
	SSLMode  string `json:"sslmode,omitempty"`
	// Schema is the search_path the URL named, when it named one: two
	// gateways may live in two schemas of one database.
	Schema string `json:"schema,omitempty"`
	// At is the Unix second the copy committed.
	At     int64 `json:"at"`
	Tables int   `json:"tables"`
	Rows   int64 `json:"rows"`
	// By is the username of whoever ran it.
	By string `json:"by,omitempty"`
	// From is the Location of the database the copy was taken from: a record
	// read anywhere else travelled there inside a copy, and is not this
	// database's.
	From string `json:"from"`
}

// Location names the target the way Store.Location names a database, so the
// two compare.
func (c LastCopy) Location() string { return pgLocation(c.Host, c.Port, c.Database, c.Schema) }

// standing says whether the record still describes this database's move, for
// a database of that dialect at that location.
func (c LastCopy) standing(dialect, location string) bool {
	if dialect == dialectPostgres && c.Location() == location {
		return false // running on the copy: the switch is done
	}
	return c.From == location
}

// LastCopyOf reads a PostgreSQL URL into a record: host, port, database, user
// and SSL mode - the password is dropped here, before anything is kept.
func LastCopyOf(raw string) (LastCopy, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
		return LastCopy{}, errors.New("store: last copy: expected postgres://user:password@host:5432/database")
	}
	c := LastCopy{
		Host:     u.Hostname(),
		Port:     5432,
		Database: strings.TrimPrefix(u.Path, "/"),
		SSLMode:  u.Query().Get("sslmode"),
		Schema:   u.Query().Get("search_path"),
	}
	if p, err := strconv.Atoi(u.Port()); err == nil && p > 0 {
		c.Port = p
	}
	if u.User != nil {
		c.User = u.User.Username()
	}
	return c, nil
}

// Location names the database this store runs on: the embedded file, or the
// PostgreSQL server and database. What a remembered copy is compared with.
func (s *Store) Location() string {
	if s.db.dialect == dialectPostgres {
		return s.location
	}
	return "sqlite:" + filepath.Join(s.dataDir, DBFileName)
}

// RememberCopy records a copy of this database, stamped with where it was
// taken from and when.
func (s *Store) RememberCopy(ctx context.Context, c LastCopy) error {
	c.From = s.Location()
	if c.At == 0 {
		c.At = time.Now().Unix()
	}
	if err := s.setPastPause(ctx, lastCopyKey, c); err != nil {
		return fmt.Errorf("store: remember the copy to %s: %w", c.Location(), err)
	}
	return nil
}

// LastCopy returns the copy this database remembers, nil when it remembers
// none - or one that is no longer its story, which is cleared on the way.
func (s *Store) LastCopy(ctx context.Context) (*LastCopy, error) {
	var c LastCopy
	if err := s.GetSetting(ctx, lastCopyKey, &c); err != nil {
		if errors.Is(err, ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if !c.standing(s.db.dialect, s.Location()) {
		_ = s.ForgetCopy(ctx)
		return nil, nil
	}
	return &c, nil
}

// ForgetCopy drops the remembered copy: an operator dismissing it, or a
// record that no longer applies.
func (s *Store) ForgetCopy(ctx context.Context) error {
	if _, err := s.db.DB.ExecContext(ctx, rebind(s.db.dialect, `DELETE FROM settings WHERE key = ?`), lastCopyKey); err != nil {
		return fmt.Errorf("store: forget the last copy: %w", err)
	}
	return nil
}

// setPastPause is SetSetting straight on the connection, for the one record
// the pause is not about.
func (s *Store) setPastPause(ctx context.Context, key string, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.db.DB.ExecContext(ctx, rebind(s.db.dialect,
		`INSERT INTO settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`), key, string(encoded))
	return err
}

// pgLocation is the one spelling of a PostgreSQL database's place: host in
// lower case, the port always, the database, and the schema when one is named.
func pgLocation(host string, port int, database, schema string) string {
	if port == 0 {
		port = 5432
	}
	l := "postgres:" + net.JoinHostPort(strings.ToLower(host), strconv.Itoa(port)) + "/" + database
	if schema != "" {
		l += "?search_path=" + schema
	}
	return l
}

// postgresLocation reads a database URL - postgres://... or the key=value
// form - into its location. Empty when it cannot tell: a remembered copy is
// then simply never taken for this database.
func postgresLocation(raw string) string {
	raw = strings.TrimSpace(raw)
	if u, err := url.Parse(raw); err == nil && (u.Scheme == "postgres" || u.Scheme == "postgresql") {
		port, _ := strconv.Atoi(u.Port())
		return pgLocation(u.Hostname(), port, strings.TrimPrefix(u.Path, "/"), u.Query().Get("search_path"))
	}
	var host, db, schema string
	var port int
	for _, f := range strings.Fields(raw) {
		k, v, ok := strings.Cut(f, "=")
		if !ok {
			continue
		}
		v = strings.Trim(v, "'")
		switch k {
		case "host":
			host = v
		case "port":
			port, _ = strconv.Atoi(v)
		case "dbname":
			db = v
		case "search_path":
			schema = v
		}
	}
	if host == "" || db == "" {
		return ""
	}
	return pgLocation(host, port, db, schema)
}
