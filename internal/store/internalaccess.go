package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// Who may call the internal plane's scheduled calls API (SCHED-01, DEPLOY-09).
//
// Two answers, in this order: HOW a caller is recognised, then WHO is let in.
//
// The mode is the first. None asks nothing of the caller and works out who it
// is - a token if it sends one, its ServiceAccount if it sends that, and
// otherwise the workload or the container holding its address. A Meerkat
// token is today's credential: minted on the console, named, revocable. A
// cluster token is the pod's own ServiceAccount token, checked by the API
// server (TokenReview): nothing to mint, nothing to hand out, Kubernetes only.
//
// The list is the second, and it is compared with the identity the mode gives.
// Empty is everyone; a caller nobody could identify is refused as soon as
// there is a list, because a list nobody can be matched against is a list that
// means nothing.
//
// What neither answer bounds, said where it is decided: the ROLES a schedule
// asks for (the task names them, as before), and the route it calls - a task
// may call another service's route. In None mode with everyone let in, any pod
// of the cluster can schedule a call with any role on any route, and the
// console says so beside the switch.

// SettingInternalAccess holds the mode and the list. Not exported with a
// configuration: the identities it lists are this platform's own names (a
// namespace, a stack), and they mean nothing where the document lands.
const SettingInternalAccess = "internal_access"

// The three modes.
const (
	// InternalAccessNone asks for nothing: the caller is identified as well as
	// it can be, and that identity is what the list is compared with.
	InternalAccessNone = "none"
	// InternalAccessToken asks for a Meerkat token of perimeter schedules.
	InternalAccessToken = "token"
	// InternalAccessCluster asks for a Kubernetes ServiceAccount token, which
	// the API server vouches for.
	InternalAccessCluster = "cluster"
)

// InternalAccessModes are the modes, in the order a screen offers them.
var InternalAccessModes = []string{InternalAccessNone, InternalAccessToken, InternalAccessCluster}

// maxInternalCallers bounds the list: it is read on every request, and a list
// of hundreds is a list nobody reviews.
const maxInternalCallers = 200

// InternalAccess is the setting.
type InternalAccess struct {
	Mode string `json:"mode"`
	// Callers are the identities let in, in the mode's own form; empty is
	// everyone.
	Callers []string `json:"callers"`
}

// How a schedule's creator was recognised (Schedule.CallerVia).
const (
	// CallerToken: a Meerkat token, named by its name.
	CallerToken = "token"
	// CallerServiceAccount: a ServiceAccount token the API server vouched for,
	// named namespace/serviceaccount.
	CallerServiceAccount = "serviceaccount"
	// CallerWorkload: deduced from the source address on Kubernetes, named
	// namespace/Kind/name - the workload owning the pod at that address.
	CallerWorkload = "workload"
	// CallerContainer: deduced from the source address on Docker, named by its
	// Swarm service, its compose service or the container itself.
	CallerContainer = "container"
	// CallerUnidentified: nothing said who it was; only the address is known.
	CallerUnidentified = "unidentified"
)

// SanitizeInternalAccess normalises the setting and refuses what cannot be
// enforced, naming what is allowed.
func SanitizeInternalAccess(a *InternalAccess) error {
	a.Mode = strings.ToLower(strings.TrimSpace(a.Mode))
	if a.Mode == "" {
		a.Mode = InternalAccessNone
	}
	if !contains(InternalAccessModes, a.Mode) {
		return fmt.Errorf("internal access mode %q: allowed are %s", a.Mode, strings.Join(InternalAccessModes, ", "))
	}
	seen := map[string]bool{}
	out := []string{}
	for _, c := range a.Callers {
		c = strings.TrimSpace(c)
		if c == "" || seen[strings.ToLower(c)] {
			continue
		}
		if len(c) > 253 {
			return fmt.Errorf("caller %q: an identity is at most 253 characters (a token's name, namespace/serviceaccount, namespace/Kind/name or a service name)", c[:40]+"...")
		}
		seen[strings.ToLower(c)] = true
		out = append(out, c)
	}
	if len(out) > maxInternalCallers {
		return fmt.Errorf("%d callers listed: at most %d, or none for everyone", len(out), maxInternalCallers)
	}
	a.Callers = out
	return nil
}

// Admits reports whether one of identities is let in by the list. An empty
// list admits everyone, identified or not; a list admits only an identity it
// names, compared without regard to case.
func (a InternalAccess) Admits(identities ...string) bool {
	if len(a.Callers) == 0 {
		return true
	}
	for _, id := range identities {
		if id == "" {
			continue
		}
		for _, c := range a.Callers {
			if strings.EqualFold(c, id) {
				return true
			}
		}
	}
	return false
}

// InternalAccess reads the setting. Absent is None for everyone: a fresh
// installation asks nothing of its services. An installation that already
// handed out schedules tokens was given the token mode when it upgraded (the
// v90 step), so an upgrade opens nothing.
func (s *Store) InternalAccess(ctx context.Context) InternalAccess {
	var a InternalAccess
	if err := s.GetSetting(ctx, SettingInternalAccess, &a); err != nil {
		return InternalAccess{Mode: InternalAccessNone, Callers: []string{}}
	}
	if SanitizeInternalAccess(&a) != nil {
		// A value nobody can read is not a reason to open the plane: the
		// strictest mode that works everywhere.
		return InternalAccess{Mode: InternalAccessToken, Callers: []string{}}
	}
	return a
}

// SetInternalAccess stores it, sanitised.
func (s *Store) SetInternalAccess(ctx context.Context, a InternalAccess) error {
	if err := SanitizeInternalAccess(&a); err != nil {
		return err
	}
	return s.SetSetting(ctx, SettingInternalAccess, a)
}

// ScheduleTokenNames are the names of the tokens a service schedules with:
// every account's, enabled, of perimeter schedules. What the Access drawer
// offers in token mode, where a token's name is the caller's identity.
func (s *Store) ScheduleTokenNames(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT name FROM api_tokens WHERE scope = ? AND enabled = ? ORDER BY name`,
		ScopeSchedules, true)
	if err != nil {
		return nil, fmt.Errorf("store: list the schedules tokens: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []string{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, fmt.Errorf("store: scan a schedules token: %w", err)
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// internalAccessKept is the v90 step: an installation that already holds a
// schedules token keeps asking for one. Its services hold that token and send
// it; opening the plane under them on an upgrade would be a decision nobody
// took. One without such a token had nobody calling the plane, and gets what
// a fresh installation gets: None, written nowhere.
func internalAccessKept(ctx context.Context, tx *transaction) error {
	var value string
	err := tx.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, SettingInternalAccess).Scan(&value)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("setting %s: %w", SettingInternalAccess, err)
	}
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM api_tokens WHERE scope = ?`, ScopeSchedules).Scan(&n); err != nil {
		return fmt.Errorf("counting the schedules tokens: %w", err)
	}
	if n == 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO settings (key, value) VALUES (?, ?)`,
		SettingInternalAccess, `{"mode":"`+InternalAccessToken+`","callers":[]}`); err != nil {
		return fmt.Errorf("setting %s: %w", SettingInternalAccess, err)
	}
	return nil
}
