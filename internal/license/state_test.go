package license

import (
	"strings"
	"testing"
	"time"

	"github.com/softwarity/meerkat/internal/evalmark"
)

const mark = "evaluation"

func marked() bool { return evalmark.Shown(mark) != "" }

func team() *License {
	return &License{ID: "LIC-1", Licensee: "ACME", Edition: EditionTeam, Tenants: 1, Instances: 1}
}

func TestALicenseTakesTheMarksAwayAndBringsThemBack(t *testing.T) {
	t.Cleanup(func() { Apply(nil) })
	Apply(nil)
	if !marked() {
		t.Fatal("no license: the marks must show")
	}
	Apply(team())
	if marked() {
		t.Fatal("a license must take the marks away")
	}
	Apply(nil)
	if !marked() {
		t.Fatal("the license gone: the marks must be back")
	}
}

// A rolling update runs one instance past the ceiling for a few minutes, and
// nothing must happen: the overrun only counts once it lasts.
func TestAnOverrunCountsOnlyOnceItLasts(t *testing.T) {
	t.Cleanup(func() { Apply(nil) })
	Apply(team())
	start := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

	Observe(2, start)
	Observe(2, start.Add(InstanceGrace-time.Minute))
	if marked() || Overrun() != "" {
		t.Fatal("within the grace, a second instance is a rolling update")
	}
	Observe(1, start.Add(InstanceGrace-time.Second))
	Observe(2, start.Add(InstanceGrace))
	if marked() {
		t.Fatal("the count must start again after going back under the ceiling")
	}
	Observe(2, start.Add(2*InstanceGrace))
	if !marked() {
		t.Fatal("an overrun that lasts brings the marks back")
	}
	if over := Overrun(); !strings.Contains(over, "2 instances") || !strings.Contains(over, "Team license covers 1") {
		t.Fatalf("the overrun is not said: %q", over)
	}
	Observe(1, start.Add(2*InstanceGrace+time.Minute))
	if marked() || Overrun() != "" {
		t.Fatal("back under the ceiling, the marks go at once")
	}
}

func TestEnterpriseHasNoCeiling(t *testing.T) {
	t.Cleanup(func() { Apply(nil) })
	Apply(&License{Edition: EditionEnterprise})
	start := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	Observe(9, start)
	Observe(9, start.Add(2*InstanceGrace))
	if marked() || Overrun() != "" {
		t.Fatal("Enterprise has no instance ceiling")
	}
	if err := TenantRefusal(50); err != nil {
		t.Fatal(err)
	}
}

func TestTeamRefusesASecondTenant(t *testing.T) {
	t.Cleanup(func() { Apply(nil) })
	if err := TenantRefusal(5); err != nil {
		t.Fatalf("no license, no ceiling: %v", err)
	}
	Apply(team())
	if err := TenantRefusal(0); err != nil {
		t.Fatalf("the first tenant: %v", err)
	}
	err := TenantRefusal(1)
	if err == nil || !strings.Contains(err.Error(), "Team license covers 1") {
		t.Fatalf("a second tenant: %v", err)
	}
}

// A license for one tenant does not cover the ones an evaluation created: the
// notice stays for as long as they are served, with no grace, and goes as soon
// as they no longer are.
func TestTenantsPastTheCeilingKeepTheMarks(t *testing.T) {
	t.Cleanup(func() { ObserveTenants(0); Apply(nil) })
	ObserveTenants(3)
	Apply(team())
	if !marked() {
		t.Fatal("three tenants on a license for one: the marks must stay")
	}
	over := TenantOverrun()
	if !strings.Contains(over, "3 tenants") || !strings.Contains(over, "Team license covers 1") ||
		!strings.Contains(over, "Several tenants") {
		t.Fatalf("the overrun is not said, or not what ends it: %q", over)
	}
	ObserveTenants(1)
	if marked() || TenantOverrun() != "" {
		t.Fatal("one tenant served: the marks must go")
	}
	ObserveTenants(3)
	Apply(&License{ID: "LIC-2", Licensee: "ACME", Edition: EditionEnterprise})
	if marked() || TenantOverrun() != "" {
		t.Fatal("no ceiling on Enterprise")
	}
}
