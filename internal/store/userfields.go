package store

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Custom identity fields: what an installation knows about a person that this
// product could not have guessed.
//
// An employee number, a cost centre, a contract reference, a legacy account id
// - every estate has two or three, none of them are the same two or three, and
// the applications behind the gateway expect them. Until now the answer was to
// write them into the identity provider or to patch the applications, because
// the fields a route may stamp or forward were a fixed list in this file.
//
// So the LIST becomes configuration and the values become part of an account.
// Nothing else changes: a custom field is selected, renamed and forwarded
// exactly like username or email, because to everything downstream it is one
// more fact about the caller.
//
// It is deliberately NOT a free-form map. A field that is only a name and a
// string cannot stop three spellings of one cost centre, and a console cannot
// draw a form for it beyond a text box - so a kind, and for a choice its list.
// The kind is what makes the value worth forwarding.

// SettingUserFields holds the definitions, gateway-wide.
const SettingUserFields = "user_fields"

// The kinds a custom field may take. Small on purpose: each one is a widget in
// the console, a validation here, and a promise about what an upstream
// receives. A kind nobody asked for is three of those forever.
const (
	// FieldText is a string, taken as it is typed.
	FieldText = "text"
	// FieldNumber is an integer or decimal, stored as it was typed.
	FieldNumber = "number"
	// FieldDate is a calendar date, YYYY-MM-DD. Not a timestamp: a hiring date
	// has no hour, and pretending it does invents a timezone question.
	FieldDate = "date"
	// FieldChoice is one of a list. This is the kind that earns the feature -
	// it is what turns a cost centre into data instead of prose.
	FieldChoice = "choice"
	// FieldBool is true or false, and travels as "true"/"false".
	FieldBool = "bool"
)

// UserFieldKinds is the closed list, named in every refusal.
var UserFieldKinds = []string{FieldText, FieldNumber, FieldDate, FieldChoice, FieldBool}

// UserField is one custom field's definition, held in the settings.
//
// There is no "required": no custom field is ever mandatory, and that is a
// decision rather than an omission. A field is defined the day somebody needs
// it, on an installation that already has its accounts - none of which carry
// it. Making it required would refuse to save every one of them until a human
// had filled a box, on a screen they opened to change something else. An empty
// custom field means "not known here", which is the truth about most accounts
// most of the time.
type UserField struct {
	// Name is the key on an account AND the default name it travels under, so
	// it is bounded like a header name rather than like a label.
	Name string `json:"name"`
	// Label is what the form shows. Empty falls back to the name.
	Label string `json:"label,omitempty"`
	Kind  string `json:"kind"`
	// Choices is the list for FieldChoice, ignored otherwise.
	Choices []string `json:"choices,omitempty"`
}

// fieldNameOK bounds a name the way a header name is bounded: it becomes one.
var fieldNameOK = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

// SanitizeUserFields checks a whole definition list and normalises what it
// can. Refusals name what is allowed, because this is read by someone typing
// into a form and the error is the only teacher they get.
func SanitizeUserFields(fields []UserField) ([]UserField, error) {
	out := make([]UserField, 0, len(fields))
	seen := map[string]bool{}
	for i := range fields {
		f := fields[i]
		f.Name = strings.TrimSpace(f.Name)
		f.Label = strings.TrimSpace(f.Label)
		if !fieldNameOK.MatchString(f.Name) {
			return nil, fmt.Errorf("field name %q is not allowed: a letter, then letters, digits, - and _", f.Name)
		}
		// A custom field that shadows a built-in would be ambiguous everywhere
		// it is named - in a forwarding attribute, in a template, on a page -
		// and the ambiguity would be silent. Refused once, here.
		if slices.Contains(PageUserFields, f.Name) || slices.Contains(IdentityFields, f.Name) {
			return nil, fmt.Errorf("field name %q is already one this gateway carries: %s",
				f.Name, strings.Join(builtInFieldNames(), ", "))
		}
		if seen[strings.ToLower(f.Name)] {
			// Lower-cased: these become header names, and a header name is
			// case-insensitive. Two fields differing only in case would be one
			// header with two meanings.
			return nil, fmt.Errorf("field name %q is defined twice", f.Name)
		}
		seen[strings.ToLower(f.Name)] = true
		if !slices.Contains(UserFieldKinds, f.Kind) {
			return nil, fmt.Errorf("field %q: kind %q is not allowed: %s",
				f.Name, f.Kind, strings.Join(UserFieldKinds, ", "))
		}
		if f.Kind == FieldChoice {
			f.Choices = trimmedChoices(f.Choices)
			if len(f.Choices) == 0 {
				return nil, fmt.Errorf("field %q is a choice with nothing to choose from", f.Name)
			}
		} else {
			f.Choices = nil
		}
		out = append(out, f)
	}
	return out, nil
}

func trimmedChoices(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]bool{}
	for _, c := range in {
		c = strings.TrimSpace(c)
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	return out
}

func builtInFieldNames() []string {
	all := slices.Clone(PageUserFields)
	for _, f := range IdentityFields {
		if !slices.Contains(all, f) {
			all = append(all, f)
		}
	}
	slices.Sort(all)
	return all
}

// ValidateUserValues checks one account's values against the definitions, and
// DROPS what no definition claims: a field removed from the settings must stop
// travelling, and leaving orphans in the row would have them reappear the day
// somebody redefines the name for something else.
func ValidateUserValues(defs []UserField, values map[string]string) (map[string]string, error) {
	out := make(map[string]string, len(defs))
	for _, f := range defs {
		v := strings.TrimSpace(values[f.Name])
		if v == "" {
			continue
		}
		switch f.Kind {
		case FieldNumber:
			if _, err := strconv.ParseFloat(v, 64); err != nil {
				return nil, fmt.Errorf("%s is not a number: %q", label(f), v)
			}
		case FieldDate:
			if _, err := time.Parse(time.DateOnly, v); err != nil {
				return nil, fmt.Errorf("%s is not a date (YYYY-MM-DD): %q", label(f), v)
			}
		case FieldChoice:
			if !slices.Contains(f.Choices, v) {
				return nil, fmt.Errorf("%s must be one of: %s", label(f), strings.Join(f.Choices, ", "))
			}
		case FieldBool:
			if v != "true" && v != "false" {
				return nil, fmt.Errorf("%s is true or false, not %q", label(f), v)
			}
		}
		out[f.Name] = v
	}
	return out, nil
}

func label(f UserField) string {
	if f.Label != "" {
		return f.Label
	}
	return f.Name
}

// encodeFields and decodeFields are the column's two directions. A map that
// cannot be encoded is stored empty rather than failing the save: the values
// are strings, so it cannot happen - and if it ever does, an account that
// saves without a custom field beats an account that cannot be saved.
func encodeFields(m map[string]string) string {
	if len(m) == 0 {
		return "{}"
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func decodeFields(s string) map[string]string {
	if s == "" || s == "{}" {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil
	}
	return m
}

// ValidFieldName reports whether a name COULD be a custom field's.
//
// It is what route validation asks, and it deliberately does not ask whether
// the field is defined: Validate is a pure function on a route, used by the
// tests and by the agent, and giving it the settings would make every caller
// carry a store to check a spelling.
//
// So a route may name a field that no longer exists, and what happens then is
// what happens to an empty value - nothing is forwarded. The console offers
// only the defined ones and flags a route naming something else, which is
// where a typo is actually seen.
func ValidFieldName(name string) bool { return fieldNameOK.MatchString(name) }
