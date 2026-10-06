package admin

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/softwarity/meerkat/internal/edition"
	"github.com/softwarity/meerkat/internal/store"
)

// Snapshots (STORE-05) - root only.
//
// What the gateway owes an operator is a COHERENT copy taken while it runs;
// everything else about backups belongs to tools that already do it well. What
// it deliberately does NOT offer is a restore button: a database cannot be
// swapped underneath the process holding it open, the sessions and users a
// restore brings back are the ones the request doing it depends on, and
// accepting an arbitrary database as trusted state would turn one borrowed
// admin session into permanent control of the gateway. Restoring happens with
// the service stopped, and the console prints the exact commands.

func (a *API) registerBackup(mux Mux) {
	mux.Handle("GET /api/backup", a.rootOnly(a.downloadSnapshot))
	mux.Handle("GET /api/backup/info", a.rootOnly(a.backupInfo))
	mux.Handle("POST /api/backup/copy", a.rootOnly(a.copyDatabase))
	mux.Handle("POST /api/backup/check", a.rootOnly(a.checkDatabase))
}

// backupInfo tells the console where this installation keeps its state, so the
// restore procedure it prints carries the REAL paths and can be pasted as is.
func (a *API) backupInfo(w http.ResponseWriter, _ *http.Request, _ store.User) {
	layout := a.st.Where()
	out := map[string]any{
		"dataDir":    layout.DataDir,
		"dbFile":     layout.DBFile,
		"keyFile":    layout.KeyFile,
		"keyFromEnv": layout.KeyFromEnv,
		// What this gateway runs on, so the screen can tell a restore from a
		// migration.
		"dialect": a.st.Dialect(),
	}
	if info, err := os.Stat(layout.DBFile); err == nil {
		out["size"] = info.Size()
	}
	writeJSON(w, http.StatusOK, out)
}

// downloadSnapshot serves a coherent copy of the database.
//
// Written to a temporary file first rather than streamed: VACUUM INTO needs a
// destination on disk, and finishing the copy BEFORE answering means a failure
// is an error the admin reads, not a truncated download they discover months
// later when they try to use it.
//
// ?format= names the KIND of the copy: sqlite for a database file, postgres
// for a dump in pg_dump's plain format. Absent, the same kind as this gateway
// runs on - a backup. The other kind is a migration (store/transfer.go).
func (a *API) downloadSnapshot(w http.ResponseWriter, r *http.Request, actor store.User) {
	format := r.URL.Query().Get("format")
	if format == "" {
		format = a.st.Dialect()
	}
	// A copy of the OTHER kind is a migration, and a migration is taken
	// paused: what is written after it would be lost at the switch. A copy of
	// the same kind is a backup, taken hot - which is what it is for.
	if format != a.st.Dialect() && !a.st.Paused() {
		writeErr(w, http.StatusConflict, "pause the gateway first: a copy to the other kind is a migration, and what is written after it would be lost at the switch")
		return
	}
	switch format {
	case "postgres":
		if err := edition.Require("a PostgreSQL copy of the database"); err != nil {
			writeErr(w, http.StatusForbidden, err.Error())
			return
		}
		a.downloadDump(w, r, actor)
		return
	case "sqlite":
		if a.st.Dialect() != "sqlite" {
			a.downloadAsSQLite(w, r, actor)
			return
		}
	default:
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("format %q: expected sqlite (a database file) or postgres (a dump)", format))
		return
	}
	dir, err := os.MkdirTemp("", "meerkat-snapshot-")
	if err != nil {
		a.internal(w, fmt.Errorf("snapshot: %w", err))
		return
	}
	defer func() { _ = os.RemoveAll(dir) }()

	path := filepath.Join(dir, store.DBFileName)
	size, err := a.st.Snapshot(r.Context(), path)
	if err != nil {
		a.internal(w, err)
		return
	}
	f, err := os.Open(path) //nolint:gosec // a path this function just built
	if err != nil {
		a.internal(w, fmt.Errorf("snapshot: %w", err))
		return
	}
	defer func() { _ = f.Close() }()

	// The date is in the name because a snapshot without one is unusable in a
	// directory of snapshots.
	name := "meerkat-" + time.Now().UTC().Format("2006-01-02-1504") + ".db"
	a.auditEvent(r.Context(), actor, "backup.snapshot", "backup", "", name, "",
		fmt.Sprintf("%d bytes", size))
	w.Header().Set("Content-Type", "application/vnd.sqlite3")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	if _, err := io.Copy(w, f); err != nil {
		// The headers are already on the wire: there is no error left to send,
		// only one to record.
		slog.Error("snapshot download interrupted", "err", err)
	}
}

// downloadDump streams this database as a PostgreSQL dump (pg_dump's plain
// format), whichever database it runs on.
//
// Streamed, unlike the file: a dump is text written row by row, and holding a
// year of audit trail in a temporary file first would only double the disk it
// takes. A failure half way leaves a file with no COMMIT, which psql refuses
// whole - the transaction is what makes a cut dump harmless.
func (a *API) downloadDump(w http.ResponseWriter, r *http.Request, actor store.User) {
	name := "meerkat-" + time.Now().UTC().Format("2006-01-02-1504") + ".sql"
	a.auditEvent(r.Context(), actor, "backup.dump", "backup", "", name, "", "from "+a.st.Dialect())
	w.Header().Set("Content-Type", "application/sql; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	if err := a.st.WritePostgresDump(r.Context(), w); err != nil {
		slog.Error("dump download interrupted", "err", err)
	}
}

// downloadAsSQLite builds a database file from an external database: a fresh
// embedded store in a temporary directory, the whole database copied into it.
// The way back from a cluster to one gateway, or production on a laptop.
func (a *API) downloadAsSQLite(w http.ResponseWriter, r *http.Request, actor store.User) {
	dir, err := os.MkdirTemp("", "meerkat-snapshot-")
	if err != nil {
		a.internal(w, fmt.Errorf("snapshot: %w", err))
		return
	}
	defer func() { _ = os.RemoveAll(dir) }()
	dst, err := store.Open(dir)
	if err != nil {
		a.internal(w, err)
		return
	}
	if _, err := a.st.CopyTo(r.Context(), dst, nil); err != nil {
		_ = dst.Close()
		a.internal(w, err)
		return
	}
	if err := dst.Close(); err != nil {
		a.internal(w, err)
		return
	}
	path := filepath.Join(dir, store.DBFileName)
	f, err := os.Open(path) //nolint:gosec // a path this function just built
	if err != nil {
		a.internal(w, fmt.Errorf("snapshot: %w", err))
		return
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		a.internal(w, fmt.Errorf("snapshot: %w", err))
		return
	}
	name := "meerkat-" + time.Now().UTC().Format("2006-01-02-1504") + ".db"
	a.auditEvent(r.Context(), actor, "backup.snapshot", "backup", "", name, "", fmt.Sprintf("from %s, %d bytes", a.st.Dialect(), info.Size()))
	w.Header().Set("Content-Type", "application/vnd.sqlite3")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	if _, err := io.Copy(w, f); err != nil {
		slog.Error("snapshot download interrupted", "err", err)
	}
}

// copyRequest names the target: its parts, one control each, or a whole URL
// for a script that has one. The parts are assembled here, where a password
// holding an @ or a / is escaped by the URL package rather than by hand.
type copyRequest struct {
	URL      string `json:"url,omitempty"`
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	Database string `json:"database,omitempty"`
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
	SSLMode  string `json:"sslmode,omitempty"`
}

// target is the URL the request names.
func (p copyRequest) target() (string, error) {
	if strings.TrimSpace(p.URL) != "" {
		return strings.TrimSpace(p.URL), nil
	}
	if p.Host == "" || p.Database == "" || p.User == "" {
		return "", errors.New("target: expected host, database and user (and the password), or a url")
	}
	port := p.Port
	if port == 0 {
		port = 5432
	}
	switch p.SSLMode {
	case "", "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
	default:
		return "", fmt.Errorf("sslmode %q: expected disable, prefer, require, verify-ca or verify-full", p.SSLMode)
	}
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(p.User, p.Password),
		Host:   net.JoinHostPort(p.Host, strconv.Itoa(port)),
		Path:   "/" + p.Database,
	}
	if p.SSLMode != "" {
		u.RawQuery = "sslmode=" + p.SSLMode
	}
	return u.String(), nil
}

type copyAnswer struct {
	Target string             `json:"target"`
	Tables []store.TableCount `json:"tables"`
	Rows   int64              `json:"rows"`
}

// copyDatabase copies this whole database into a PostgreSQL server: the move
// to a cluster.
//
// Paused first, always: a copy taken while writes go on loses whatever is
// written between the copy and the switch, and for the audit trail that is
// not acceptable. The URL is used for the copy and nowhere else - never stored,
// never logged, never written to the trail but as its host and database.
//
// The trail of the move is written into the TARGET, at the end: the source is
// paused and is being left, and the new database starts its history by saying
// where it came from.
func (a *API) copyDatabase(w http.ResponseWriter, r *http.Request, actor store.User) {
	if err := edition.Require("copying the database to PostgreSQL"); err != nil {
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}
	if !a.st.Paused() {
		writeErr(w, http.StatusConflict, "pause the gateway first: a copy taken while it writes would lose what is written before the switch")
		return
	}
	var p copyRequest
	if err := decodeStrict(r, &p); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed copy: "+err.Error())
		return
	}
	raw, err := p.target()
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	p.URL = raw
	target, err := describeDatabase(p.URL)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	dst, err := store.OpenAt(a.st.Where().DataDir, p.URL)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "could not open "+target+": "+redactURL(err.Error(), p.URL))
		return
	}
	defer func() { _ = dst.Close() }()
	tables, err := a.st.CopyTo(r.Context(), dst, nil)
	if errors.Is(err, store.ErrTargetInUse) {
		writeErr(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		writeErr(w, http.StatusBadGateway, "the copy to "+target+" failed, and nothing was kept there: "+redactURL(err.Error(), p.URL))
		return
	}
	var rows int64
	for _, t := range tables {
		rows += t.Source
	}
	_ = dst.AddAuditEvent(r.Context(), store.AuditEvent{
		ActorID: actor.ID, Action: "backup.migrate", Target: "backup", TargetName: target,
		Detail: fmt.Sprintf("copied from %s: %d tables, %d rows", a.st.Dialect(), len(tables), rows),
	})
	writeJSON(w, http.StatusOK, copyAnswer{Target: target, Tables: tables, Rows: rows})
}

// checkDatabase tells the console what a target server says before anything
// is copied: reachable, which version, which SSL mode works, empty or not, and
// whether the user may create tables there. It reads and writes nothing - no
// schema is created - so it needs no pause and may be run as often as wanted.
func (a *API) checkDatabase(w http.ResponseWriter, r *http.Request, _ store.User) {
	if err := edition.Require("checking a PostgreSQL server"); err != nil {
		writeErr(w, http.StatusForbidden, err.Error())
		return
	}
	var p copyRequest
	if err := decodeStrict(r, &p); err != nil {
		writeErr(w, http.StatusBadRequest, "malformed check: "+err.Error())
		return
	}
	raw, err := p.target()
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	target, err := describeDatabase(raw)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	probe, err := store.ProbePostgres(r.Context(), raw)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "could not reach "+target+": "+redactURL(err.Error(), raw))
		return
	}
	writeJSON(w, http.StatusOK, probe)
}

// describeDatabase names a PostgreSQL URL for a person: host and database,
// never the credentials. It also refuses what is not one.
func describeDatabase(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
		return "", errors.New("url: expected postgres://user:password@host:5432/database")
	}
	return u.Host + strings.TrimSuffix(u.Path, "/"), nil
}

// redactURL keeps a driver's message from repeating the password it was given,
// and says what to do about the one failure whose own words do not: a
// certificate this gateway cannot verify, under verify-ca or verify-full.
func redactURL(msg, raw string) string {
	if strings.Contains(msg, "unknown authority") {
		msg += " - the server's certificate is signed by an authority this gateway does not hold (an operator's own CA, typically): " +
			"choose sslmode require, which encrypts without verifying, or mount that CA and name it with sslrootcert"
	}
	if u, err := url.Parse(raw); err == nil && u.User != nil {
		if pw, ok := u.User.Password(); ok && pw != "" {
			msg = strings.ReplaceAll(msg, pw, "***")
		}
	}
	return msg
}
