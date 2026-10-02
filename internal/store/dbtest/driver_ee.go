//go:build ee

package dbtest

import (
	// The driver, for the admin connection this helper opens to create and
	// drop a schema. Blank, and pgx's own: what the PRODUCT does with it is the
	// storage layer's business, and importing that here would be a cycle for
	// its tests. Grouped so the comment belongs to the import and the linter
	// reads it as its justification.
	_ "github.com/jackc/pgx/v5/stdlib"
)

const haveDriver = true
