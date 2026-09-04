package store

import (
	"context"
	"fmt"
	"slices"
	"time"
)

// SettingMaintenance is the global switch that closes an installation to its
// visitors (LIFE-05).
//
// There is already a maintenance FILTER, which a route carries when its own
// service is down. This is the other half, and the reason it exists is in one
// sentence: on the day everything is taken down, nobody should have to edit
// routes - and, worse, edit them back afterwards from memory. One switch, no
// route touched, nothing to undo.
const SettingMaintenance = "maintenance"

// The reasons an installation can give. A CLOSED list, and that is the whole
// point: whatever is said here is read by visitors in twenty languages, and a
// sentence typed in the console is a sentence in one of them. An operator
// picks; the product translates.
//
// ReasonNone is the default and stays available. Sometimes the honest answer
// is that the reason is nobody's business.
const (
	ReasonNone        = ""
	ReasonMaintenance = "maintenance"
	ReasonUpgrade     = "upgrade"
	ReasonIncident    = "incident"
)

// Reasons is the catalogue, in the order a console offers it.
var Reasons = []string{ReasonNone, ReasonMaintenance, ReasonUpgrade, ReasonIncident}

// Maintenance is that switch.
type Maintenance struct {
	Enabled bool `json:"enabled"`
	// Reason is one of the catalogue above, translated where it is shown.
	Reason string `json:"reason,omitempty"`
	// For is how long the operator said it would take, as an ISO-8601 duration
	// chosen from a list: PT1H, PT4H, P1D... A CHOICE, like the reason, and for
	// the same reason - what it becomes on the page is a phrase, and a phrase
	// has to come from somewhere the product can translate.
	//
	// It is kept beside the instant rather than instead of it, because the two
	// answer different questions: this one is what was decided, and the
	// console shows it back unchanged; Until is when that lands, which is the
	// only thing that stays meaningful an hour later.
	For string `json:"for,omitempty"`
	// Until is when service is expected back, in Unix seconds, computed from
	// For. Zero means "we are not saying when", which the page renders as soon
	// rather than as a blank: an outage with no horizon at all reads as an
	// abandoned service.
	Until int64 `json:"until,omitempty"`
	// Since is when it was turned on, in Unix seconds, stamped by the server.
	//
	// It exists for the failure mode this feature actually has, which is not
	// "the switch does not work": it is a Tuesday morning closure still on on
	// Thursday because the person who flipped it went home. A console showing
	// "since 3 days ago" is what asks the question.
	Since int64 `json:"since,omitempty"`
}

// SanitizeMaintenance refuses a reason nobody can translate and stamps the
// moment.
//
// previous is what is currently stored: the timestamp is set when the switch
// GOES ON and kept while it stays on, so changing the reason halfway through
// does not make it look like it just started.
func SanitizeMaintenance(m *Maintenance, previous Maintenance, now time.Time) error {
	if !slices.Contains(Reasons, m.Reason) {
		return fmt.Errorf("reason %q is not one this product can say: allowed are "+
			"%q (say nothing), %q, %q and %q - the page is read in twenty languages, so the "+
			"wording is the product's and the choice is yours",
			m.Reason, ReasonNone, ReasonMaintenance, ReasonUpgrade, ReasonIncident)
	}
	// The instant is COMPUTED from the duration, never sent: a client that
	// could post a moment could post one in the past, and there would be a
	// rule to write about it. Chosen from a list, in the future by
	// construction, and re-anchored from now every time it is saved - which is
	// what somebody pushing a window back means by it.
	m.Until = 0
	if m.For != "" {
		d, err := ParseISODuration(m.For)
		if err != nil {
			return fmt.Errorf("how long: %w", err)
		}
		m.Until = now.Add(d).Unix()
	}
	switch {
	case !m.Enabled:
		m.Since, m.Until, m.Reason, m.For = 0, 0, ReasonNone, ""
	case previous.Enabled && previous.Since > 0:
		m.Since = previous.Since
	default:
		m.Since = now.Unix()
	}
	return nil
}

// GetMaintenance reads the switch. Anything unreadable answers OFF: a gateway
// that refuses every request because a setting could not be parsed is a worse
// outage than the one it was closed for.
func (s *Store) GetMaintenance(ctx context.Context) Maintenance {
	var m Maintenance
	if err := s.GetSetting(ctx, SettingMaintenance, &m); err != nil {
		return Maintenance{}
	}
	if !slices.Contains(Reasons, m.Reason) {
		m.Reason = ReasonNone
	}
	return m
}
