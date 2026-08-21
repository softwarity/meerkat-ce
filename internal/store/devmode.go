package store

import "context"

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
	on := true
	_ = s.GetSetting(ctx, SettingDevMode, &on)
	return on
}

// SetDevMode records the switch.
func (s *Store) SetDevMode(ctx context.Context, on bool) error {
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
