package store

import (
	"context"
	"errors"
	"sync/atomic"
)

// production says this instance is a PRODUCTION gateway: the whole developer
// surface is off here, whatever the database says.
//
// An environment decision rather than a stored one, and that is the point. The
// stored switch travels: a configuration export, a restored backup, a database
// copied from staging - each can carry devMode ON into production, and nobody
// notices until a tunnel port is open in front of customers. The environment
// does not travel. It is a property of WHERE this process runs, so it is where
// this belongs.
//
// A process-wide value rather than a field, because it is read from every dev
// gate on the request path and there is exactly one process.
var production atomic.Bool

// SetProduction records the environment's answer. Called once, at startup.
func SetProduction(on bool) { production.Store(on) }

// Production reports whether the developer surface is closed by the
// environment. What it buys a screen is the ability to say WHY a switch is
// off instead of showing one that does nothing.
func Production() bool { return production.Load() }

// DevMode reports whether this installation offers a developer surface at all
// (DEV-01, LIFE-03).
//
// It is read on the request path, so it stays a plain settings lookup rather
// than a cached one: the read is a single indexed row, and a switch an operator
// flips to close a demo has to be closed on the NEXT request, not in five
// seconds.
//
// Missing means ON. The seed writes the row on every start, so a missing value
// only happens on a database that predates the setting - and there, the dev
// capability was the whole gate, which is exactly what ON reproduces.
func (s *Store) DevMode(ctx context.Context) bool {
	// The environment has the last word, and only in one direction: it can
	// close the developer surface, never open one an operator turned off.
	if production.Load() {
		return false
	}
	on := true
	_ = s.GetSetting(ctx, SettingDevMode, &on)
	return on
}

// SetDevMode records the switch.
//
// Refused outright on a production gateway rather than accepted and ignored: a
// switch that stores ON while every gate reads OFF is how someone spends an
// afternoon wondering why the tooling never appears.
func (s *Store) SetDevMode(ctx context.Context, on bool) error {
	if on && production.Load() {
		return errors.New("store: this gateway is declared production (MEERKAT_PRODUCTION), so the developer surface stays closed here; open it on a gateway that is not")
	}
	return s.SetSetting(ctx, SettingDevMode, on)
}

// DevAllowed answers the question every developer gate asks: may THIS user use
// the developer tooling here? Both halves have to be true - the installation
// offers it, and the account holds the capability - and having one place to ask
// is what stops the next dev entry point from checking only the account, the
// way each of them did before the switch existed.
func (s *Store) DevAllowed(ctx context.Context, u User) bool {
	return u.Dev && u.Enabled && s.DevMode(ctx)
}
