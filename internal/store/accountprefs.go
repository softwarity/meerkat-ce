package store

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/text/currency"
	"golang.org/x/text/language"
)

// Account preferences: the facts about a person that the PERSON chooses, as
// opposed to the custom fields (userfields.go), which an administrator fills
// and an upstream trusts.
//
// Two of them, each a switch in Infra > Model: the time zone dates are shown
// in, and the currency amounts are shown in. A switch because neither means
// anything to every installation - a gateway in front of tools that show no
// money has no use for a currency, and a box on everyone's profile that no
// application reads is a question nobody should have to answer.
//
// On, the person chooses it on their profile, and it is one more fact about
// the caller, selected and forwarded on a route like username or email. Off,
// nothing offers it and nothing carries it.

// SettingAccountPreferences holds which preferences are on, gateway-wide.
const SettingAccountPreferences = "account_preferences"

// The preferences, by the name they travel under.
const (
	PrefTimezone = "timezone"
	PrefCurrency = "currency"
)

// AccountPreferences is the setting: one switch per preference. Off, a value
// stored before stays in the row and travels again the day it is switched on.
type AccountPreferences struct {
	Timezone bool `json:"timezone"`
	Currency bool `json:"currency"`
}

// DefaultAccountPreferences is what an installation that never chose has: the
// time zone on, which is what every account carried before it became a
// choice; the currency off.
var DefaultAccountPreferences = AccountPreferences{Timezone: true}

// Names lists the preferences switched on, by the name they travel under.
func (p AccountPreferences) Names() []string {
	var out []string
	if p.Timezone {
		out = append(out, PrefTimezone)
	}
	if p.Currency {
		out = append(out, PrefCurrency)
	}
	return out
}

// Off says whether name is a preference this installation switched off: a
// fact a route may still name, and that carries nothing.
func (p AccountPreferences) Off(name string) bool {
	switch name {
	case PrefTimezone:
		return !p.Timezone
	case PrefCurrency:
		return !p.Currency
	}
	return false
}

// AccountPreferences reads the setting, the defaults when nobody wrote it. An
// unreadable value answers the defaults too: a preference is not worth
// refusing a request over.
func (s *Store) AccountPreferences(ctx context.Context) AccountPreferences {
	p := DefaultAccountPreferences
	_ = s.GetSetting(ctx, SettingAccountPreferences, &p)
	return p
}

// ValidCurrency checks an ISO 4217 code ("EUR"); empty is "no choice" and is
// valid. Upper case only: the code travels to applications as it is stored,
// and "eur" is not what one expects to parse.
func ValidCurrency(code string) error {
	if code == "" {
		return nil
	}
	if code != strings.ToUpper(code) || len(code) != 3 {
		return fmt.Errorf("currency %q is not allowed: a three-letter ISO 4217 code in upper case, such as EUR or USD", code)
	}
	if _, err := currency.ParseISO(code); err != nil {
		return fmt.Errorf("currency %q is not an ISO 4217 code this gateway knows: three letters such as EUR, USD, GBP or JPY", code)
	}
	return nil
}

// CurrencyOfLanguages is the currency a browser's Accept-Language points to,
// for a person who chose none: the country of the first language that names
// one ("fr-CH" is CHF), else the country the first language is most likely
// spoken in ("fr" is France, so EUR). A guess, and said to be one on the
// profile, where the person corrects it. "" when nothing points anywhere (no
// header, "*", a region that is not a country such as "es-419").
func CurrencyOfLanguages(acceptLanguage string) string {
	tags, _, err := language.ParseAcceptLanguage(acceptLanguage)
	if err != nil || len(tags) == 0 {
		return ""
	}
	for _, t := range tags {
		if r, conf := t.Region(); conf == language.Exact {
			if u, ok := currency.FromRegion(r); ok {
				return u.String()
			}
		}
	}
	r, _ := tags[0].Region()
	if u, ok := currency.FromRegion(r); ok {
		return u.String()
	}
	return ""
}
