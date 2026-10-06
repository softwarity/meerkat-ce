package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/softwarity/meerkat/internal/store"
)

// meerkat db: the database moves of Configuration > Snapshot, for a script or
// a Kubernetes Job run before a helm upgrade. The same engine as the console
// (store/transfer.go); run it with the gateway stopped, or paused from the
// console, so nothing is written while it copies.
//
//	meerkat db dump -format postgres -out meerkat.sql   a dump, for psql
//	meerkat db dump -format sqlite -out meerkat.db      a database file
//	meerkat db copy -to postgres://...                  straight into a server
const dbUsage = `usage:
  meerkat db dump -out FILE [-format sqlite|postgres]   (default: the kind this gateway runs on)
  meerkat db copy -to postgres://user:password@host:5432/database

  -data and -database-url (or MEERKAT_DATA, MEERKAT_DATABASE_URL) name the source,
  as for the gateway itself. Stop the gateway, or pause it, first.`

func runDB(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, dbUsage)
		return 2
	}
	verb, rest := args[0], args[1:]
	fs := flag.NewFlagSet("meerkat db "+verb, flag.ContinueOnError)
	dataDir := fs.String("data", envOr("MEERKAT_DATA", "data"), "data directory of the source")
	sourceURL := fs.String("database-url", envOr("MEERKAT_DATABASE_URL", ""), "the source database, when it is PostgreSQL")
	out := fs.String("out", "", "dump: the file to write")
	format := fs.String("format", "", "dump: sqlite or postgres")
	to := fs.String("to", "", "copy: the target PostgreSQL URL")
	if err := fs.Parse(rest); err != nil {
		return 2
	}
	ctx := context.Background()
	src, err := store.OpenAt(*dataDir, *sourceURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, "meerkat db: open the source:", err)
		return 1
	}
	defer func() { _ = src.Close() }()
	switch verb {
	case "dump":
		err = dumpDB(ctx, src, *format, *out)
	case "copy":
		err = copyDB(ctx, src, *dataDir, *to)
	default:
		fmt.Fprintf(os.Stderr, "meerkat db %s: expected dump or copy\n%s\n", verb, dbUsage)
		return 2
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "meerkat db "+verb+":", err)
		return 1
	}
	return 0
}

func dumpDB(ctx context.Context, src *store.Store, format, out string) error {
	if out == "" {
		return errors.New("-out: name the file to write")
	}
	if _, err := os.Stat(out); err == nil {
		return fmt.Errorf("%s already exists: a dump never overwrites", out)
	}
	if format == "" {
		format = src.Dialect()
	}
	switch format {
	case "postgres":
		f, err := os.Create(out) //nolint:gosec // the operator's own path
		if err != nil {
			return err
		}
		if err := src.WritePostgresDump(ctx, f); err != nil {
			_ = f.Close()
			return err
		}
		fmt.Printf("wrote %s: load it into an empty database with psql -v ON_ERROR_STOP=1 -f %s, and carry the vault key\n", out, out)
		return f.Close()
	case "sqlite":
		if src.Dialect() == "sqlite" {
			if _, err := src.Snapshot(ctx, out); err != nil {
				return err
			}
		} else if err := sqliteFrom(ctx, src, out); err != nil {
			return err
		}
		fmt.Printf("wrote %s: put it in the data directory as %s, and keep the vault key\n", out, store.DBFileName)
		return nil
	default:
		return fmt.Errorf("-format %q: expected sqlite or postgres", format)
	}
}

// sqliteFrom builds a database file from an external database.
func sqliteFrom(ctx context.Context, src *store.Store, out string) error {
	dir, err := os.MkdirTemp("", "meerkat-db-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	dst, err := store.Open(dir)
	if err != nil {
		return err
	}
	if _, err := src.CopyTo(ctx, dst, nil); err != nil {
		_ = dst.Close()
		return err
	}
	if err := dst.Close(); err != nil {
		return err
	}
	in, err := os.Open(filepath.Join(dir, store.DBFileName)) //nolint:gosec // a path this function built
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	f, err := os.Create(out) //nolint:gosec // the operator's own path
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, in); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func copyDB(ctx context.Context, src *store.Store, dataDir, to string) error {
	if !strings.HasPrefix(to, "postgres://") && !strings.HasPrefix(to, "postgresql://") {
		return errors.New("-to: expected postgres://user:password@host:5432/database")
	}
	dst, err := store.OpenAt(dataDir, to)
	if err != nil {
		return fmt.Errorf("open the target: %w", err)
	}
	defer func() { _ = dst.Close() }()
	var rows int64
	counts, err := src.CopyTo(ctx, dst, func(c store.TableCount) {
		rows += c.Source
		fmt.Printf("  %-24s %8d rows\n", c.Table, c.Source)
	})
	if err != nil {
		return err
	}
	_ = dst.AddAuditEvent(ctx, store.AuditEvent{
		Action: "backup.migrate", Target: "backup",
		Detail: fmt.Sprintf("copied from %s by the command line: %d tables, %d rows", src.Dialect(), len(counts), rows),
	})
	fmt.Printf("copied %d rows in %d tables, every table counted on both sides; start the gateway on the new database with the same vault key\n", rows, len(counts))
	return nil
}
