package idp

import (
	"context"
	"errors"
	"sync/atomic"
	"time"
)

// The assertions a redirect authority hands back once and only once (SAML,
// AUTH-19). A SAML assertion is a bearer credential for its validity window:
// what was posted once must not open a second session if it is posted again.
// The ledger is the store's - a table, so a replay is refused on every node -
// and main hands it in; a driver never reaches the store itself.

// AssertionLedger records an assertion and says whether it was the first time.
type AssertionLedger interface {
	SpendAssertion(ctx context.Context, id string, until int64) (bool, error)
}

var ledger atomic.Pointer[AssertionLedger]

// SetAssertionLedger wires the ledger. Called once, from main.
func SetAssertionLedger(l AssertionLedger) { ledger.Store(&l) }

// ErrReplayed is an assertion already used.
var ErrReplayed = errors.New("idp: this assertion was already used - a SAML response is accepted once")

// Spend records the assertion id, refusing one already spent. With no ledger
// wired it refuses everything: an authority whose assertions could be replayed
// is not one to sign anybody in through.
func Spend(ctx context.Context, id string, until time.Time) error {
	l := ledger.Load()
	if l == nil {
		return errors.New("idp: no assertion ledger is wired, so a replay could not be refused - refusing the sign-in")
	}
	first, err := (*l).SpendAssertion(ctx, id, until.Unix())
	if err != nil {
		return err
	}
	if !first {
		return ErrReplayed
	}
	return nil
}
