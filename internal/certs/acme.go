package certs

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

// Well-known directories, offered as a starting point. They are NOT the
// feature: the field is a URL, and the reason it is a URL is that half the
// installations Meerkat is meant for have no route to the internet at all.
// An internal step-ca, an EJBCA, a Windows authority with the ACME role - any
// of them answers here, and then nothing about the automatic path depends on
// Let's Encrypt existing.
const (
	LetsEncryptURL        = acme.LetsEncryptURL
	LetsEncryptStagingURL = "https://acme-staging-v02.api.letsencrypt.org/directory"
)

// ACMEOptions is an automatic authority as an operator configures it.
type ACMEOptions struct {
	// DirectoryURL is the ACME directory. Empty means Let's Encrypt.
	DirectoryURL string
	// Email is the contact the authority warns about expiries. Optional for
	// most, required by some private ones.
	Email string
	// Hosts is the CLOSED list of names that may be requested. Never empty:
	// an open policy lets anyone reach the gateway by IP with a made-up SNI
	// and burn the authority's rate limit on names nobody owns.
	Hosts []string
	// RootCA is the authority that signs the ACME SERVER's own HTTPS
	// certificate, in PEM. A private ACME server is usually behind a private
	// certificate, and without this the very first call fails on a name nobody
	// connects to their own internal PKI.
	RootCA string
	// EABKeyID and EABHMACKey are the External Account Binding (RFC 8555
	// section 7.3.4): the credentials a corporate authority hands out to say
	// WHICH account this gateway registers under. Public authorities ignore it;
	// most private ones refuse without it.
	EABKeyID   string
	EABHMACKey string
	// AcceptTOS records that a human ticked the box. It is a legal act, so it
	// is never assumed: without it, NewACME refuses and says so.
	AcceptTOS bool
	// Cache persists the account key and the issued certificates. Without one,
	// every restart registers again and re-requests everything, which is the
	// fastest way to meet a rate limit.
	Cache autocert.Cache
}

// NewACME builds the automatic authority, or refuses with a sentence naming
// what is missing.
func NewACME(o ACMEOptions) (*autocert.Manager, error) {
	hosts := make([]string, 0, len(o.Hosts))
	for _, h := range o.Hosts {
		if h = strings.ToLower(strings.TrimSpace(h)); h != "" {
			hosts = append(hosts, h)
		}
	}
	if len(hosts) == 0 {
		return nil, fmt.Errorf("no domain listed: an automatic authority needs the closed list of names it may be asked for")
	}
	if !o.AcceptTOS {
		return nil, fmt.Errorf("the authority's terms of service have not been accepted: an account cannot be registered without that")
	}
	client := &acme.Client{
		DirectoryURL: strings.TrimSpace(o.DirectoryURL),
		UserAgent:    "meerkat",
	}
	if o.RootCA != "" {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM([]byte(o.RootCA)) {
			return nil, fmt.Errorf("the authority's root certificate is not readable: expected one or more PEM blocks starting with -----BEGIN CERTIFICATE-----")
		}
		client.HTTPClient = &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
			},
		}
	}
	m := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Cache:      o.Cache,
		HostPolicy: autocert.HostWhitelist(hosts...),
		Email:      strings.TrimSpace(o.Email),
		Client:     client,
	}
	if o.EABKeyID != "" || o.EABHMACKey != "" {
		if o.EABKeyID == "" || o.EABHMACKey == "" {
			return nil, fmt.Errorf("an external account binding needs BOTH its key identifier and its HMAC key: the authority issues them as a pair")
		}
		key, err := decodeEABKey(o.EABHMACKey)
		if err != nil {
			return nil, err
		}
		m.ExternalAccountBinding = &acme.ExternalAccountBinding{KID: o.EABKeyID, Key: key}
	}
	return m, nil
}

// decodeEABKey reads the HMAC key. RFC 8555 says base64url, and that is what
// most authorities print - but enough of them paste standard base64, with or
// without padding, that refusing those would be refusing a correct secret over
// a punctuation mark.
func decodeEABKey(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	for _, enc := range []*base64.Encoding{
		base64.RawURLEncoding, base64.URLEncoding,
		base64.RawStdEncoding, base64.StdEncoding,
	} {
		if b, err := enc.DecodeString(s); err == nil && len(b) > 0 {
			return b, nil
		}
	}
	return nil, fmt.Errorf("the external account HMAC key is not base64: copy it exactly as the authority issued it")
}

// CacheStore is the three operations autocert needs, in the store's own
// vocabulary: absence is an error, not an empty value.
type CacheStore interface {
	ACMECacheGet(ctx context.Context, key string) (string, error)
	ACMECachePut(ctx context.Context, key, value string) error
	ACMECacheDelete(ctx context.Context, key string) error
}

// StoreCache adapts the store to autocert.Cache.
//
// The state lives in the database rather than in a directory because it MUST
// be shared the day a second instance exists: two caches mean two accounts,
// two orders for the same name, and a rate limit met by one instance on behalf
// of the other.
type StoreCache struct {
	S CacheStore
	// Missing recognises the store's "no such row" so this adapter can
	// translate it into autocert's sentinel.
	Missing func(error) bool
}

// Get returns autocert.ErrCacheMiss when the entry is absent.
func (c StoreCache) Get(ctx context.Context, key string) ([]byte, error) {
	v, err := c.S.ACMECacheGet(ctx, key)
	if err != nil {
		if c.Missing != nil && c.Missing(err) {
			return nil, autocert.ErrCacheMiss
		}
		return nil, err
	}
	return []byte(v), nil
}

// Put stores one entry.
func (c StoreCache) Put(ctx context.Context, key string, data []byte) error {
	return c.S.ACMECachePut(ctx, key, string(data))
}

// Delete removes one entry.
func (c StoreCache) Delete(ctx context.Context, key string) error {
	return c.S.ACMECacheDelete(ctx, key)
}

// CachedInfo reads back what an authority issued for one host, so the console
// can show a real expiry instead of a form that says nothing about whether it
// ever worked. Absence is not an error here: it is the honest "nothing yet".
func CachedInfo(ctx context.Context, cache autocert.Cache, host string) (Info, bool) {
	raw, err := cache.Get(ctx, host)
	if err != nil {
		return Info{}, false
	}
	rest := raw
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			return Info{}, false
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		leaf, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return Info{}, false
		}
		return Info{
			Subject:     leaf.Subject.String(),
			Issuer:      leaf.Issuer.String(),
			Serial:      leaf.SerialNumber.String(),
			Algo:        leaf.SignatureAlgorithm.String(),
			KeyType:     KeyTypeOf(leaf.PublicKey),
			DNSNames:    append([]string{}, leaf.DNSNames...),
			IPAddresses: ipStrings(leaf),
			NotBefore:   leaf.NotBefore.Unix(),
			NotAfter:    leaf.NotAfter.Unix(),
			SelfSigned:  leaf.Subject.String() == leaf.Issuer.String(),
		}, true
	}
}
