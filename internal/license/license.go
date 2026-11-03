// Package license reads the license files Softwarity issues, offline.
//
// A license is a file Keygen signs: Team or Enterprise, who holds it, how far
// its updates go. Validation never makes a network call: the signing public
// keys ship with the binary. It is what turns the evaluation image into Team
// or Enterprise: a valid one takes the evaluation notice away, and a Team one
// sets the ceilings (see state.go). The community image carries no notion of
// any of this: what it lacks is absent, not locked.
package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// Errors returned by Parse.
var (
	ErrSignature = errors.New("license: invalid signature")
	ErrNotYet    = errors.New("license: not valid yet")
	ErrProduct   = errors.New("license: issued for another product")
)

// The editions a license can name.
const (
	EditionTeam       = "team"
	EditionEnterprise = "enterprise"
)

// ExpiresHard in a policy's metadata makes a license's date a real end. It is
// what a test license has and a sold one does not: a file cannot be revoked, so
// one that is not worth anything after its date is one that can leak.
const ExpiresHard = "hard"

// ProductID is Meerkat's product at the issuer. One signing key serves every
// product of an account, so a signature alone would let a license for another
// product through.
const ProductID = "04225a54-5083-45ce-aa39-b9dfe606563a"

// The file as the issuer writes it. The algorithm is checked before anything
// else: what a signature means depends on it entirely.
const (
	fileHeader    = "-----BEGIN LICENSE FILE-----"
	fileFooter    = "-----END LICENSE FILE-----"
	algorithm     = "base64+ed25519"
	signingPrefix = "license/"
)

// clockSlack is how far ahead of a gateway's clock a license may be dated. A
// license is checked again on every read, so a machine a few hours behind must
// not lose the one it was given.
const clockSlack = 24 * time.Hour

// productionKeys holds the signing public keys embedded in released binaries;
// keeping several entries allows key rotation without invalidating older
// licenses.
var productionKeys = []ed25519.PublicKey{
	mustKey("9c37e80cfbd48e2b1cd7dad946b64a1c0336d41e8262c4f7bb9aff3df635846c"),
}

func mustKey(text string) ed25519.PublicKey {
	key, err := hex.DecodeString(text)
	if err != nil || len(key) != ed25519.PublicKeySize {
		panic("license: a signing key in this build is not a valid ed25519 key")
	}
	return key
}

// ProductionKeys are the signing keys this build verifies a license against.
func ProductionKeys() []ed25519.PublicKey { return productionKeys }

// License is what a verified file says.
type License struct {
	// ID names this licence: the console shows it beside the licensee, and it
	// is what a support request quotes.
	ID       string
	Licensee string
	// Edition is EditionTeam or EditionEnterprise.
	Edition string
	// Tenants and Instances are the ceilings, 0 for none.
	Tenants   int
	Instances int
	IssuedAt  time.Time
	// ExpiresAt is how far updates and support go: to the end of that day,
	// in UTC. See Covered and Ended for what it does.
	ExpiresAt time.Time
	// Expires is ExpiresHard when the date ends the license itself.
	Expires string
}

// certificate is the file once its armour is off.
type certificate struct {
	Enc string `json:"enc"`
	Sig string `json:"sig"`
	Alg string `json:"alg"`
}

// dataset is what the signature covers: the license as the issuer holds it,
// and its policy, whose metadata is the template of an edition. The issuer's
// meta.expiry is left out on purpose: it dates this copy of the file, a month
// out by default, and a license read offline does not expire with its copy.
type dataset struct {
	Data struct {
		ID         string `json:"id"`
		Attributes struct {
			Name   string     `json:"name"`
			Expiry *time.Time `json:"expiry"`
		} `json:"attributes"`
		Relationships struct {
			Product struct {
				Data struct {
					ID string `json:"id"`
				} `json:"data"`
			} `json:"product"`
		} `json:"relationships"`
	} `json:"data"`
	Included []included `json:"included"`
	Meta     struct {
		Issued time.Time `json:"issued"`
	} `json:"meta"`
}

type included struct {
	Type       string `json:"type"`
	Attributes struct {
		Metadata template `json:"metadata"`
	} `json:"attributes"`
}

// template is a policy's metadata: what one edition is.
type template struct {
	Edition   string `json:"edition"`
	Tenants   int    `json:"tenants,omitempty"`
	Instances int    `json:"instances,omitempty"`
	Expires   string `json:"expires,omitempty"`
}

// Load reads and validates a license file against the embedded production
// keys, using the current time.
func Load(path string) (*License, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("license: %w", err)
	}
	return Parse(data, productionKeys, time.Now())
}

// Parse validates a license file against the given public keys at the given
// instant. It is the testable core of Load.
//
// A license past its date still verifies: this is a perpetual fallback, and
// whoever paid keeps the versions released while they paid. What the date does
// is Covered's and Ended's to say, and the worst of it is the evaluation notice:
// nobody is ever locked out of their own gateway. Cutting authentication over a
// late invoice would make this unusable in front of a production, and nobody
// serious would deploy it.
func Parse(data []byte, keys []ed25519.PublicKey, now time.Time) (*License, error) {
	cert, err := unarmour(data)
	if err != nil {
		return nil, fmt.Errorf("license: malformed file: %w", err)
	}
	if cert.Alg != algorithm {
		return nil, fmt.Errorf("license: malformed file: signed with %q, expected %q", cert.Alg, algorithm)
	}
	if !verify(cert, keys) {
		return nil, ErrSignature
	}
	raw, err := base64.StdEncoding.DecodeString(cert.Enc)
	if err != nil {
		return nil, fmt.Errorf("license: malformed payload: %w", err)
	}
	var ds dataset
	if err := json.Unmarshal(raw, &ds); err != nil {
		return nil, fmt.Errorf("license: malformed payload: %w", err)
	}
	if ds.Data.Relationships.Product.Data.ID != ProductID {
		return nil, ErrProduct
	}
	lic, err := ds.license()
	if err != nil {
		return nil, fmt.Errorf("license: malformed payload: %w", err)
	}
	// Dated ahead of the clock is a minting mistake, not a customer.
	if now.Add(clockSlack).Before(lic.IssuedAt) {
		return nil, ErrNotYet
	}
	return lic, nil
}

func unarmour(data []byte) (certificate, error) {
	var cert certificate
	text := strings.TrimSpace(string(data))
	body, ok := strings.CutPrefix(text, fileHeader)
	if !ok {
		return cert, fmt.Errorf("it does not start with %s", fileHeader)
	}
	if body, ok = strings.CutSuffix(body, fileFooter); !ok {
		return cert, fmt.Errorf("it does not end with %s", fileFooter)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(body), ""))
	if err != nil {
		return cert, err
	}
	return cert, json.Unmarshal(raw, &cert)
}

func (ds dataset) license() (*License, error) {
	a := ds.Data.Attributes
	if a.Name == "" {
		return nil, errors.New("it names no licensee")
	}
	if a.Expiry == nil {
		return nil, errors.New("it carries no end date")
	}
	var t *template
	for i := range ds.Included {
		if ds.Included[i].Type == "policies" {
			t = &ds.Included[i].Attributes.Metadata
		}
	}
	if t == nil {
		return nil, errors.New("it does not include its policy")
	}
	switch t.Edition {
	case EditionEnterprise:
	case EditionTeam:
		if t.Tenants < 1 || t.Instances < 1 {
			return nil, errors.New("its Team policy sets no tenants or instances")
		}
	default:
		return nil, fmt.Errorf("its policy names the edition %q, and only %q and %q exist",
			t.Edition, EditionTeam, EditionEnterprise)
	}
	if t.Expires != "" && t.Expires != ExpiresHard {
		return nil, fmt.Errorf("its policy says it expires %q, and only %q exists", t.Expires, ExpiresHard)
	}
	return &License{ID: ds.Data.ID, Licensee: a.Name, Edition: t.Edition, Tenants: t.Tenants,
		Instances: t.Instances, IssuedAt: ds.Meta.Issued, ExpiresAt: *a.Expiry, Expires: t.Expires}, nil
}

// Covered reports whether a build dated buildDate falls within what this
// license paid for. A build that does not is an evaluation again, for as long as
// it runs: nothing is written anywhere, so going back to a covered build, or
// renewing, puts the license back in force.
func (l *License) Covered(buildDate time.Time) bool {
	return buildDate.Before(l.end())
}

// Hard says the date ends the license itself: a test license.
func (l *License) Hard() bool { return l.Expires == ExpiresHard }

// Ended reports whether a test license is past its date, whatever the build. A
// license that is sold never ends.
func (l *License) Ended(now time.Time) bool { return l.Hard() && !now.Before(l.end()) }

// end is when the license stops counting: the end of the day it names, in UTC.
// The issuer puts the time at noon so that the date reads the same everywhere;
// the whole day is still paid for.
func (l *License) end() time.Time {
	d := l.ExpiresAt.UTC()
	return time.Date(d.Year(), d.Month(), d.Day()+1, 0, 0, 0, 0, time.UTC)
}

// EditionName is the edition as a sentence names it.
func (l *License) EditionName() string {
	if l.Edition == EditionTeam {
		return "Team"
	}
	return "Enterprise"
}

func verify(cert certificate, keys []ed25519.PublicKey) bool {
	sig, err := base64.StdEncoding.DecodeString(cert.Sig)
	if err != nil {
		return false
	}
	message := []byte(signingPrefix + cert.Enc)
	for _, key := range keys {
		if ed25519.Verify(key, message, sig) {
			return true
		}
	}
	return false
}

// Sign writes a license the way the issuer does, so that the tests exercise
// the real format. Licenses are issued by Keygen, never by this: it signs with
// a key the caller holds, and only a key in ProductionKeys counts.
func Sign(lic License, key ed25519.PrivateKey) ([]byte, error) {
	return signFor(lic, key, ProductID)
}

func signFor(lic License, key ed25519.PrivateKey, product string) ([]byte, error) {
	var ds dataset
	ds.Data.ID = lic.ID
	ds.Data.Attributes.Name = lic.Licensee
	ds.Data.Attributes.Expiry = &lic.ExpiresAt
	ds.Data.Relationships.Product.Data.ID = product
	policy := included{Type: "policies"}
	policy.Attributes.Metadata = template{Edition: lic.Edition, Tenants: lic.Tenants, Instances: lic.Instances,
		Expires: lic.Expires}
	ds.Included = []included{policy}
	ds.Meta.Issued = lic.IssuedAt
	raw, err := json.Marshal(ds)
	if err != nil {
		return nil, fmt.Errorf("license: %w", err)
	}
	enc := base64.StdEncoding.EncodeToString(raw)
	cert, err := json.Marshal(certificate{Enc: enc, Alg: algorithm,
		Sig: base64.StdEncoding.EncodeToString(ed25519.Sign(key, []byte(signingPrefix+enc)))})
	if err != nil {
		return nil, fmt.Errorf("license: %w", err)
	}
	return []byte(fileHeader + "\n" + base64.StdEncoding.EncodeToString(cert) + "\n" + fileFooter + "\n"), nil
}
