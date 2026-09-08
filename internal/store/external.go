package store

import (
	"context"
	"database/sql"
	"errors"
)

// The door to an EXTERNAL database.
//
// Meerkat ships as two images built from one commit (internal/edition). This
// one runs on the embedded database: one gateway, its own file, nothing to
// install. An external database - which is what several gateways serving one
// installation share - comes with the Enterprise image, driver included, and
// the seam below is where the two meet.
//
// The storage layer itself is written once for both. The placeholder
// rebinding and the introspection queries know more than one spelling because
// that is what keeps the schema a single schema; what varies is only who can
// open a connection.
var (
	dialExternal   func(url string) (*sql.DB, error)
	listenExternal func(ctx context.Context, conn *sql.Conn, channel string, on func(string)) error
)

// ErrExternalDatabase answers a URL this image cannot open. It names the way
// forward rather than the wall: a driver error tells a reader nothing, and the
// next thing they need to know is that the embedded database is right there.
var ErrExternalDatabase = errors.New(
	"store: this image runs on the embedded database - one gateway, its own file, nothing to install. " +
		"Connecting to an external database is an Enterprise feature. " +
		"Leave MEERKAT_DATABASE_URL unset to use the embedded one")

// RegisterExternalDatabase is called by the package that implements it: dial
// opens a connection pool for a URL, listen holds one connection open for the
// notifications that carry a change between gateways.
func RegisterExternalDatabase(
	dial func(url string) (*sql.DB, error),
	listen func(ctx context.Context, conn *sql.Conn, channel string, on func(string)) error,
) {
	dialExternal, listenExternal = dial, listen
}

// ExternalAvailable reports whether this build can open one at all.
func ExternalAvailable() bool { return dialExternal != nil }
