package store

import "errors"

// ErrInvalid marks a refusal that is the CALLER's to fix, not a fault.
//
// The sanitisers live in the store on purpose - six writers save a route, and
// a rule enforced in five of them is a rule nobody can trust - but that put
// them behind a boundary the API reads as "something broke". An operator
// asking for a three-hour timeout, or an access level that does not exist, got
// "internal error" and a line in the log, while the sentence naming what IS
// allowed sat one return statement away.
//
// So: a refusal says it is one. The handlers turn it into 422 with its own
// words; anything else stays a 500, which is what a 500 is for.
var ErrInvalid = errors.New("invalid")

// refusal carries the sentinel WITHOUT wearing it: errors.Is finds ErrInvalid
// through the multi-unwrap, and Error() is the sentence the caller wrote and
// nothing else. errors.Join would have prefixed every refusal with the word
// "invalid" and a newline, on a screen where the sentence is the whole point.
type refusal struct{ err error }

func (r refusal) Error() string   { return r.err.Error() }
func (r refusal) Unwrap() []error { return []error{ErrInvalid, r.err} }

// invalidf marks a refusal, keeping the wording the caller wrote.
func invalidf(err error) error {
	if err == nil {
		return nil
	}
	return refusal{err}
}
