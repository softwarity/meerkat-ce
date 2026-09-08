//go:build ee

package dbtest

// The driver, for the admin connection this helper opens to create and drop a
// schema. Blank, and pgx's own: what the PRODUCT does with it is the storage
// layer's business, and importing that here would be a cycle for its tests.
import _ "github.com/jackc/pgx/v5/stdlib"

const haveDriver = true
