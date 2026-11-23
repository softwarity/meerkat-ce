package store

import "testing"

func TestAccessCapabilities(t *testing.T) {
	admin := Access{Level: AccessCapable, Capabilities: []string{CapTenantAdmin}}
	adminOfAcme := Access{Level: AccessCapable, Capabilities: []string{CapTenantAdmin}, AdminTenants: []string{"acme"}}
	infra := Access{Level: AccessCapable, Capabilities: []string{CapInfraAdmin}}

	member := Caller{Authenticated: true, TenantID: "acme", Roles: []string{"ROLE_ADMIN"}}
	acmeAdmin := Caller{Authenticated: true, TenantID: "acme", Capabilities: []string{CapTenantAdmin}}
	globexAdmin := Caller{Authenticated: true, TenantID: "globex", Capabilities: []string{CapTenantAdmin}}
	root := Caller{Authenticated: true, Capabilities: []string{CapRoot, CapInfraAdmin, CapAppAdmin}}
	noTenant := Caller{Authenticated: true, Capabilities: []string{CapTenantAdmin}}

	cases := []struct {
		name string
		a    Access
		c    Caller
		want bool
	}{
		// The point of the axis: a role a tenant administrator can compose,
		// ROLE_ADMIN included, does not open a screen asking for a capability.
		{"role is not a capability", admin, member, false},
		{"tenant admin, any tenant", admin, acmeAdmin, true},
		{"tenant admin of a named tenant", adminOfAcme, acmeAdmin, true},
		{"tenant admin of another tenant", adminOfAcme, globexAdmin, false},
		{"root passes wherever one is asked", adminOfAcme, root, true},
		{"the administrator of no active tenant", admin, noTenant, false},
		{"ROLE_ADMIN is not infra admin", infra, member, false},
		{"infra admin", infra, root, true},
		{"anonymous", admin, Caller{}, false},
	}
	for _, c := range cases {
		if got := c.a.Grants(c.c); got != c.want {
			t.Errorf("%s: Grants = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestSanitizeAccessCapabilities(t *testing.T) {
	a := Access{Level: AccessCapable, Capabilities: []string{"owner"}}
	if err := SanitizeAccess(&a); err == nil {
		t.Fatal("an unknown capability must be refused")
	}
	a = Access{Level: AccessCapable}
	if err := SanitizeAccess(&a); err == nil {
		t.Fatal("the capabilities level names at least one")
	}
	a = Access{Level: AccessCapable, Capabilities: []string{CapAppAdmin}, AdminTenants: []string{"acme"}, Roles: []string{"r"}}
	if err := SanitizeAccess(&a); err != nil {
		t.Fatal(err)
	}
	if a.AdminTenants != nil || a.Roles != nil {
		t.Fatalf("tenants narrow tenantAdmin only, and the level poses no role: kept %v %v", a.AdminTenants, a.Roles)
	}
	a = Access{Level: AccessAuth, Capabilities: []string{CapDev}}
	if err := SanitizeAccess(&a); err != nil || a.Capabilities != nil {
		t.Fatalf("capabilities belong to their level: %v %v", err, a.Capabilities)
	}
	if err := NoCapabilities(Access{Level: AccessCapable, Capabilities: []string{CapDev}}); err == nil {
		t.Fatal("an endpoint or a limit must refuse capabilities")
	}
}
