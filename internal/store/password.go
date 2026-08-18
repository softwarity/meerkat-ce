package store

import (
	"context"
	"unicode"
)

// SettingPasswordPolicy is the gateway-wide password policy (AUTH-10): the
// minimum length and how many characters of each kind a password must carry.
// One policy for the whole installation - a password is checked where it is
// TYPED, and the places it is typed (sign-up, the forced change at first
// login, the profile, a reset link) know nothing about organisations.
const SettingPasswordPolicy = "password_policy"

// PasswordPolicy is what a new password must satisfy. Zero means "do not
// care": a policy of all zeros accepts anything, which is why the default
// carries a length.
type PasswordPolicy struct {
	MinLength  int `json:"minLength"`
	MinLower   int `json:"minLower"`
	MinUpper   int `json:"minUpper"`
	MinDigits  int `json:"minDigits"`
	MinSpecial int `json:"minSpecial"`
	// History is how many previous passwords may not be reused (0 = reuse is
	// allowed). Kept as hashes, never as passwords: comparing a candidate
	// means bcrypt against each one, which is why the count is bounded.
	History int `json:"history"`
	// ExpiryDays forces a change after that many days (0 = never). The check
	// is at SIGN-IN, not by a clock: a password expiring at three in the
	// morning would sign nobody out, it would only refuse the next login.
	ExpiryDays int `json:"expiryDays"`
}

// DefaultPasswordPolicy is the eight characters the code asked for before
// there was a policy at all. Deliberately not stricter: raising the bar for
// everyone at once, on an upgrade, locks people out of a password change they
// were in the middle of. The administrator raises it, and knows they did.
func DefaultPasswordPolicy() PasswordPolicy {
	return PasswordPolicy{MinLength: 8}
}

// PasswordRuleKind names a rule for the catalogue and for the browser: the
// page renders the checklist from these, and its script ticks the same names
// as someone types.
const (
	PasswordRuleLength  = "length"
	PasswordRuleLower   = "lower"
	PasswordRuleUpper   = "upper"
	PasswordRuleDigit   = "digit"
	PasswordRuleSpecial = "special"
)

// PasswordRule is one line of the checklist: what is required, and whether the
// candidate satisfies it.
type PasswordRule struct {
	Kind string `json:"kind"`
	Need int    `json:"need"`
	OK   bool   `json:"ok"`
}

// Rules returns the policy as the checklist a page shows, in a fixed order,
// with each line answered for pwd. Pass "" to get the requirements alone.
//
// Same function for the page and for the refusal, on purpose: a checklist that
// says one thing while the server enforces another is worse than no checklist,
// because it is believed.
func (p PasswordPolicy) Rules(pwd string) []PasswordRule {
	var lower, upper, digits, special int
	for _, r := range pwd {
		switch {
		case unicode.IsLower(r):
			lower++
		case unicode.IsUpper(r):
			upper++
		case unicode.IsDigit(r):
			digits++
		case unicode.IsLetter(r):
			// A letter in a script that HAS no case - kana, Chinese, Arabic,
			// Hebrew. It is not lowercase, not uppercase, and above all not a
			// special character: counting it as one would tell someone writing
			// a Japanese passphrase that they had satisfied a rule they never
			// meant to, and refuse them the lowercase they cannot type.
		case !unicode.IsSpace(r):
			// Punctuation and symbols. Deliberately not a list of "allowed"
			// specials: such a list is how policies end up rejecting a
			// perfectly good passphrase typed on somebody else's keyboard.
			special++
		}
	}
	// Counted in RUNES, not bytes: a passphrase in Greek or Japanese is not
	// three times longer than it looks, and len() would say it is.
	count := map[string]int{
		PasswordRuleLength:  len([]rune(pwd)),
		PasswordRuleLower:   lower,
		PasswordRuleUpper:   upper,
		PasswordRuleDigit:   digits,
		PasswordRuleSpecial: special,
	}
	need := map[string]int{
		PasswordRuleLength:  p.MinLength,
		PasswordRuleLower:   p.MinLower,
		PasswordRuleUpper:   p.MinUpper,
		PasswordRuleDigit:   p.MinDigits,
		PasswordRuleSpecial: p.MinSpecial,
	}
	out := make([]PasswordRule, 0, 5)
	for _, kind := range []string{PasswordRuleLength, PasswordRuleLower,
		PasswordRuleUpper, PasswordRuleDigit, PasswordRuleSpecial} {
		if need[kind] <= 0 {
			continue // not required: not shown, not checked
		}
		out = append(out, PasswordRule{Kind: kind, Need: need[kind], OK: count[kind] >= need[kind]})
	}
	return out
}

// Accepts says whether pwd satisfies every rule.
func (p PasswordPolicy) Accepts(pwd string) bool {
	for _, r := range p.Rules(pwd) {
		if !r.OK {
			return false
		}
	}
	return true
}

// Sanitize bounds the policy to what a person can actually type. The ceiling
// is not paranoia: a length nobody can satisfy locks every account out of its
// own password change, and the screen that set it is behind that same login.
func (p PasswordPolicy) Sanitize() PasswordPolicy {
	clamp := func(v, ceiling int) int {
		if v < 0 {
			return 0
		}
		if v > ceiling {
			return ceiling
		}
		return v
	}
	p.MinLength = clamp(p.MinLength, 128)
	p.MinLower = clamp(p.MinLower, 16)
	p.MinUpper = clamp(p.MinUpper, 16)
	p.MinDigits = clamp(p.MinDigits, 16)
	p.MinSpecial = clamp(p.MinSpecial, 16)
	// Bounded because every remembered hash is a bcrypt comparison on the way
	// to a new password: twenty is already a slow save, a hundred is a denial
	// of service someone typed into a settings box.
	p.History = clamp(p.History, 24)
	p.ExpiryDays = clamp(p.ExpiryDays, 3650)
	// A length below the sum of the parts is not a policy, it is a promise the
	// checklist could never keep: four kinds at 2 each need eight characters.
	if sum := p.MinLower + p.MinUpper + p.MinDigits + p.MinSpecial; p.MinLength < sum {
		p.MinLength = sum
	}
	return p
}

// GetPasswordPolicy reads the policy, falling back to the default on an
// unset or unreadable setting - a password is never left unchecked.
func (s *Store) GetPasswordPolicy(ctx context.Context) PasswordPolicy {
	var p PasswordPolicy
	if err := s.GetSetting(ctx, SettingPasswordPolicy, &p); err != nil {
		return DefaultPasswordPolicy()
	}
	if p.MinLength <= 0 && p.MinLower <= 0 && p.MinUpper <= 0 && p.MinDigits <= 0 &&
		p.MinSpecial <= 0 && p.History <= 0 && p.ExpiryDays <= 0 {
		return DefaultPasswordPolicy()
	}
	return p.Sanitize()
}
