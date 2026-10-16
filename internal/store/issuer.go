package store

import (
	"context"
	"fmt"
	"net"
	"slices"
	"strings"
)

// SettingJWTIssuer is the CHOICE of issuer among the application plane's
// public addresses: the "iss" of every token the gateway signs (AUTH-06), the
// issuer both discovery documents name (SAUTH-04) and the address the OpenID
// provider lives at (SAUTH-05). One issuer for all of them: an application
// signing its people in and a service verifying a route's token are told the
// same address.
//
// It is not free text any more. An issuer is where a library looks for
// {issuer}/.well-known/openid-configuration, so it has to be an address the
// gateway answers on - and the names it answers on are the ones the operator
// declared in Domains & TLS (SSL-14). The setting only says WHICH of them,
// when there are several. It travels with a configuration like the routes,
// and lands as a choice the public names of the target confirm or not.
const SettingJWTIssuer = "jwtIssuer"

// DefaultJWTIssuer is what every token says while the application plane has
// no public name: a JWT needs an iss, and a service can be told this one. The
// OpenID provider, which has to live at its issuer, stays off meanwhile.
const DefaultJWTIssuer = "meerkat"

// LoopbackName says whether host is this machine: a developer's gateway, or
// the test suite's. Such a name is reached over plain http on the data
// plane's port, since no certificate vouches for it to anybody else.
func LoopbackName(host string) bool {
	h := strings.Trim(host, "[]")
	if h == "localhost" {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// IssuerAddress is the address one public name gives the issuer:
// https://<name>, whoever serves it - this gateway with its certificate, or a
// proxy in front, which serves the name over HTTPS by definition. A loopback
// name is http://<name>:<port>, the port of the data plane's plain listen
// address (dataAddr), left out when it is not known.
func IssuerAddress(name, dataAddr string) string {
	if !LoopbackName(name) {
		return "https://" + name
	}
	host := name
	if ip := net.ParseIP(strings.Trim(name, "[]")); ip != nil && ip.To4() == nil {
		host = "[" + ip.String() + "]"
	}
	port := dataAddr
	if i := strings.LastIndexByte(dataAddr, ':'); i >= 0 {
		port = dataAddr[i+1:]
	}
	if port == "" {
		return "http://" + host
	}
	return "http://" + host + ":" + port
}

// IssuerAddresses are the addresses the issuer may be: one per public name of
// the application plane, in the order the operator declared them.
func IssuerAddresses(names []string, dataAddr string) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		if a := IssuerAddress(n, dataAddr); !slices.Contains(out, a) {
			out = append(out, a)
		}
	}
	return out
}

// IssuerChoices are the issuer's possible addresses on this installation, none
// while the application plane has no public name.
func (s *Store) IssuerChoices(ctx context.Context, dataAddr string) []string {
	return IssuerAddresses(s.RawTLS(ctx).AppDoor.Names, dataAddr)
}

// JWTIssuer is the issuer in effect. One public address: that one. Several:
// the one chosen, else the first. A stored choice that is none of them - the
// free-text issuer of before, a name since removed, a document imported from
// elsewhere - is simply not a choice, and the first address stands in for it:
// the issuer follows the public names rather than holding on to a name the
// gateway no longer answers on. None: DefaultJWTIssuer.
func (s *Store) JWTIssuer(ctx context.Context, dataAddr string) string {
	choices := s.IssuerChoices(ctx, dataAddr)
	if len(choices) == 0 {
		return DefaultJWTIssuer
	}
	var v string
	if err := s.GetSetting(ctx, SettingJWTIssuer, &v); err == nil && slices.Contains(choices, v) {
		return v
	}
	return choices[0]
}

// SetJWTIssuer stores the choice, refusing anything but one of the public
// addresses.
func (s *Store) SetJWTIssuer(ctx context.Context, dataAddr, v string) error {
	if err := CheckJWTIssuer(v, s.IssuerChoices(ctx, dataAddr)); err != nil {
		return err
	}
	return s.SetSetting(ctx, SettingJWTIssuer, v)
}

// CheckJWTIssuer says whether v is one of the public addresses, and when it
// is not, which ones are.
func CheckJWTIssuer(v string, choices []string) error {
	if len(choices) == 0 {
		return fmt.Errorf("the issuer %q: the application plane has no public name yet - "+
			"declare one in Domains & TLS, Application tab, and the issuer is its address", v)
	}
	if !slices.Contains(choices, v) {
		return fmt.Errorf("the issuer %q is not one of the application plane's public addresses: expected %s",
			v, strings.Join(choices, ", "))
	}
	return nil
}
