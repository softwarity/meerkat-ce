package certs

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

// Several authorities, one per provider an installation deals with.
//
// One account per installation was the first shape, and it stopped at the
// first real question: softwarity.io at Let's Encrypt, a customer's domain at
// ZeroSSL, internal names at the company's step-ca. An authority is now a
// named entry - set up once, in the authorities drawer - and every order to
// an authority says which one it asks. There are never fifty: a list is
// enough, and a list is what the drawer shows.

// Authority is one ACME account: where it is, and what it needs.
type Authority struct {
	ID string `json:"id"`
	// Name is what the pool and the Add certificate menu call it.
	Name string `json:"name"`
	// Provider is a known one (ProviderLetsEncrypt...) or ProviderCustom. A
	// known provider fixes the directory; only a custom one carries its own.
	Provider     string `json:"provider"`
	DirectoryURL string `json:"directoryUrl"`
	// Email is the account's contact, which some authorities require. Expiry
	// is watched by the daily digest, not by this address.
	Email string `json:"email,omitempty"`
	// RootCA is the root that signs the authority's OWN https certificate, for
	// a private authority this gateway does not already trust.
	RootCA string `json:"rootCa,omitempty"`
	// The external account binding: which customer account this gateway
	// registers under, for the authorities that refuse anonymous ones. The
	// HMAC key is a secret and follows the vault rule (VAULT-05).
	EABKeyID   string `json:"eabKeyId,omitempty"`
	EABHMACKey string `json:"eabHmacKey,omitempty"`
	AcceptTOS  bool   `json:"acceptTos"`
	// CachePrefix keeps this account's key and what it issued apart from the
	// others' in the one shared cache. The account set up before authorities
	// existed keeps none, so what it already issued is still found.
	CachePrefix string `json:"cachePrefix,omitempty"`
	CreatedAt   int64  `json:"createdAt,omitempty"`
}

// The providers the drawer offers by name. A provider is a directory, the
// page of its terms, and whether it insists on an account binding - which is
// the whole of what an operator would otherwise have to look up.
const (
	ProviderLetsEncrypt        = "letsencrypt"
	ProviderLetsEncryptStaging = "letsencrypt-staging"
	ProviderZeroSSL            = "zerossl"
	ProviderGoogle             = "google"
	ProviderCustom             = "custom"
)

// Provider describes one known authority.
type Provider struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Directory string `json:"directory"`
	Terms     string `json:"terms"`
	// NeedsEAB: it refuses an account without a binding, and Where says
	// where its console hands the pair out.
	NeedsEAB bool   `json:"needsEab"`
	Where    string `json:"where,omitempty"`
	// Note is the one thing worth knowing before using it.
	Note string `json:"note,omitempty"`
}

// Providers is the catalogue, in the order the drawer lists it.
var Providers = []Provider{
	{
		ID: ProviderLetsEncryptStaging, Label: "Let's Encrypt (staging)",
		Directory: LetsEncryptStagingURL, Terms: "https://letsencrypt.org/repository/",
		Note: "Certificates browsers do not trust, outside the weekly limits: the place to check that a name reaches this gateway on port 443.",
	},
	{
		ID: ProviderLetsEncrypt, Label: "Let's Encrypt",
		Directory: LetsEncryptURL, Terms: "https://letsencrypt.org/repository/",
		Note: "Free, no account details. Fifty certificates a week per registered domain: test with staging first.",
	},
	{
		ID: ProviderZeroSSL, Label: "ZeroSSL",
		Directory: "https://acme.zerossl.com/v2/DV90", Terms: "https://zerossl.com/terms/",
		NeedsEAB: true, Where: "ZeroSSL dashboard, Developer, EAB Credentials for ACME Clients: Generate.",
	},
	{
		ID: ProviderGoogle, Label: "Google Trust Services",
		Directory: "https://dv.acme-v02.api.pki.goog/directory", Terms: "https://pki.goog/repository/",
		NeedsEAB: true, Where: "gcloud publicca external-account-keys create, in a Google Cloud project with the Public CA API enabled.",
	},
	{
		ID: ProviderCustom, Label: "Another authority",
		Note: "An internal authority such as step-ca, or any other that speaks ACME: every value comes from whoever runs it.",
	},
}

// ProviderByID finds a provider of the catalogue.
func ProviderByID(id string) (Provider, bool) {
	for _, p := range Providers {
		if p.ID == id {
			return p, true
		}
	}
	return Provider{}, false
}

// Directory is where this authority answers: the provider's, or its own.
func (a Authority) Directory() string {
	if p, ok := ProviderByID(a.Provider); ok && p.Directory != "" {
		return p.Directory
	}
	return strings.TrimSpace(a.DirectoryURL)
}

// Normalize fixes what the provider decides, and drops what it does not use:
// a private root or a binding left from a custom authority would otherwise
// ride along unseen once a public one is chosen - and a private root in place
// of the public ones makes every call to that authority fail.
func (a Authority) Normalize() Authority {
	a.Name = strings.TrimSpace(a.Name)
	a.Email = strings.TrimSpace(a.Email)
	p, known := ProviderByID(a.Provider)
	if !known {
		a.Provider = ProviderCustom
		p, _ = ProviderByID(ProviderCustom)
	}
	if a.Name == "" {
		a.Name = p.Label
	}
	if a.Provider != ProviderCustom {
		a.DirectoryURL = p.Directory
		a.RootCA = ""
		if !p.NeedsEAB {
			a.EABKeyID, a.EABHMACKey = "", ""
		}
	}
	return a
}

// Check refuses an authority that cannot work, naming what is missing: at
// save time, not at renewal time three months later. eabKey is the HMAC key
// resolved from the vault.
func (a Authority) Check(eabKey string) error {
	if a.Directory() == "" {
		return fmt.Errorf("no directory URL: the address of the authority's ACME service, given by whoever runs it")
	}
	if p, _ := ProviderByID(a.Provider); p.NeedsEAB && (a.EABKeyID == "" || a.EABHMACKey == "") {
		return fmt.Errorf("%s refuses accounts without a binding: its key ID and HMAC key are both needed (%s)", p.Label, p.Where)
	}
	_, err := NewACME(ACMEOptions{
		DirectoryURL: a.Directory(), Email: a.Email, Hosts: []string{"account.check.invalid"},
		RootCA: a.RootCA, EABKeyID: a.EABKeyID, EABHMACKey: eabKey, AcceptTOS: a.AcceptTOS,
	})
	return err
}

// Discover asks the authority's directory whether it answers, so a wrong URL
// or an unreachable authority is said when it is saved. No account is
// created: that waits for the first order.
func (a Authority) Discover(ctx context.Context) error {
	client := &acme.Client{DirectoryURL: a.Directory(), UserAgent: "meerkat"}
	if a.RootCA != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(a.RootCA)) {
			return fmt.Errorf("the authority's root certificate is not readable: expected PEM blocks starting with -----BEGIN CERTIFICATE-----")
		}
		client.HTTPClient = &http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
		}}
	} else {
		client.HTTPClient = &http.Client{Timeout: 15 * time.Second}
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if _, err := client.Discover(ctx); err != nil {
		return fmt.Errorf("the authority does not answer at %s: %w", a.Directory(), err)
	}
	return nil
}

// NewAuthorityID names a new authority, and its cache prefix with it.
func NewAuthorityID() string {
	b := make([]byte, 5)
	_, _ = rand.Read(b)
	return "ca-" + hex.EncodeToString(b)
}

// PrefixedCache keeps one authority's entries apart in the shared cache.
type PrefixedCache struct {
	C      autocert.Cache
	Prefix string
}

// Get reads under the prefixed key.
func (p PrefixedCache) Get(ctx context.Context, key string) ([]byte, error) {
	return p.C.Get(ctx, p.Prefix+key)
}

// Put stores under the prefixed key.
func (p PrefixedCache) Put(ctx context.Context, key string, data []byte) error {
	return p.C.Put(ctx, p.Prefix+key, data)
}

// Delete removes the prefixed key.
func (p PrefixedCache) Delete(ctx context.Context, key string) error {
	return p.C.Delete(ctx, p.Prefix+key)
}

// AuthorityByID finds an authority of these settings.
func (a ACMESettings) AuthorityByID(id string) (Authority, bool) {
	for _, x := range a.Authorities {
		if x.ID == id {
			return x, true
		}
	}
	return Authority{}, false
}
