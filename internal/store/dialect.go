package store

import "strings"

// What differs between the databases Meerkat can sit on.
//
// The embedded one is the default and always will be: a `docker run` with
// nothing else is the first impression, and it must not pay for the cluster
// case. PostgreSQL exists for the day several gateways serve one installation
// - and it was chosen for a reason that is not storage. LISTEN/NOTIFY carries
// the "this changed, go re-read it" between processes, and advisory locks give
// the one exclusion the product needs (issuing a certificate twice is a
// quota problem). No Redis, no message bus, nothing else to run.
//
// Deliberately NOT an open choice of database. Every dialect added is every
// query that has to work on it, forever, and a CI that runs them all. A second
// one is a commercial conversation, not a default to carry.
const (
	dialectSQLite   = "sqlite"
	dialectPostgres = "postgres"
)

// rebind turns the `?` placeholders every query in this package is written
// with into what the driver expects.
//
// The alternative was writing `$1, $2, …` by hand in a hundred and sixty-nine
// places, or a query builder. Both cost more than this: the queries stay
// readable in one form, and the translation is six lines with a test.
func rebind(dialect, query string) string {
	if dialect != dialectPostgres {
		return query
	}
	// A `?` inside a string literal is not a placeholder. There is none today
	// and a test says so, but skipping quoted runs costs one boolean and
	// removes a class of corruption that would only show up on the one query
	// that has one.
	var b strings.Builder
	b.Grow(len(query) + 8)
	n, quoted := 0, false
	for i := range len(query) {
		c := query[i]
		switch {
		case c == '\'':
			quoted = !quoted
			b.WriteByte(c)
		case c == '?' && !quoted:
			n++
			b.WriteByte('$')
			b.WriteString(itoa(n))
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// itoa is strconv.Itoa for small positive numbers, kept local so the hot path
// of every query does not reach for a package to format one integer.
func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	var buf [8]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
