package store

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"unicode"
)

// SettingJWTIssuer is the "iss" of the identity JWT the gateway signs for its
// services (AUTH-06), and the issuer its discovery document names (SAUTH-04).
// It travels with a configuration: it is part of the contract the services
// check, like a route's algorithm.
const SettingJWTIssuer = "jwtIssuer"

// DefaultJWTIssuer is what every token said before the issuer could be set,
// and what it still says until somebody sets one.
const DefaultJWTIssuer = "meerkat"

// JWTIssuer reads it. Unset, or a stored value this build would refuse (an
// imported document is not validated key by key), is the default: a token
// must always carry an issuer a service can be told about.
func (s *Store) JWTIssuer(ctx context.Context) string {
	var v string
	if err := s.GetSetting(ctx, SettingJWTIssuer, &v); err != nil || CheckJWTIssuer(v) != nil {
		return DefaultJWTIssuer
	}
	return v
}

// SetJWTIssuer stores it, refusing what a service could not compare.
func (s *Store) SetJWTIssuer(ctx context.Context, v string) error {
	if err := CheckJWTIssuer(v); err != nil {
		return err
	}
	return s.SetSetting(ctx, SettingJWTIssuer, v)
}

// CheckJWTIssuer says whether v can be an issuer. A service compares it byte
// for byte with the "iss" it receives, so it is taken exactly as given: no
// space anywhere, and nothing trimmed on its behalf. A URL is welcome - it is
// what a library that starts from the issuer needs, since it reads
// {issuer}/.well-known/openid-configuration - and then it is an http(s) URL
// with a host, no query, no fragment and no trailing slash.
func CheckJWTIssuer(v string) error {
	if v == "" {
		return fmt.Errorf("the issuer is empty: give a name such as %q, or the internal plane's URL such as http://meerkat:7070", DefaultJWTIssuer)
	}
	if len(v) > 256 {
		return fmt.Errorf("the issuer is %d characters long, 256 at most", len(v))
	}
	for _, r := range v {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return fmt.Errorf("the issuer %q holds a space or a control character: a service compares it byte for byte", v)
		}
	}
	if !strings.Contains(v, "://") {
		return nil
	}
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("the issuer %q looks like a URL and is not an http or https one with a host", v)
	}
	if u.RawQuery != "" || u.Fragment != "" || strings.HasSuffix(v, "/") {
		return fmt.Errorf("the issuer %q carries a query, a fragment or a trailing slash: a library appends /.well-known/openid-configuration to it as it stands", v)
	}
	return nil
}
