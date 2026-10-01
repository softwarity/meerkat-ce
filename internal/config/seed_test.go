package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

const seedFile = `version: 1
roles:
  - id: from-file
    name: From the file
routes:
  - id: api
    name: api
    enabled: true
    upstream: http://up.invalid
    predicates:
      - type: path
        args:
          patterns:
            - /api/**
`

func write(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "meerkat.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestSeedOnlyOnFirstStart is the whole rule of CFG-03: the file is a door in,
// not a master. It fills an empty gateway, and a gateway that has been
// configured since is never rewritten behind the admin's back.
func TestSeedOnlyOnFirstStart(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)
	path := write(t, seedFile)

	seeded, err := Seed(ctx, s, path, 1000)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if !seeded {
		t.Fatal("an empty gateway must take the file")
	}
	routes, err := s.ListRoutes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatalf("routes = %v", routes)
	}

	// The admin deletes the route in the console. A restart must NOT bring it
	// back: that is the failure mode this guard exists for.
	if _, err := s.DeleteRoute(ctx, "api"); err != nil {
		t.Fatal(err)
	}
	seeded, err = Seed(ctx, s, path, 2000)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if seeded {
		t.Fatal("the file was replayed and resurrected a deleted route")
	}
	routes, err = s.ListRoutes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 0 {
		t.Fatalf("the deleted route came back: %v", routes)
	}
}

// TestSeedIgnoresAConfiguredGateway: a file dropped next to a gateway that has
// been running for months is somebody's leftover bootstrap, not an instruction.
func TestSeedIgnoresAConfiguredGateway(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)
	seed(t, s) // this gateway already has routes, and no seed mark

	seeded, err := Seed(ctx, s, write(t, seedFile), 1000)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if seeded {
		t.Fatal("a configured gateway must not be seeded")
	}
	if _, err := s.GetRole(ctx, "from-file"); err == nil {
		t.Fatal("the file was applied to a configured gateway")
	}
}

// TestSeedRefusesAnUnreadableFile: the operator asked for this file by name.
// Starting a gateway that is silently not the one they described is worse than
// not starting.
func TestSeedRefusesAnUnreadableFile(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)
	if _, err := Seed(ctx, s, filepath.Join(t.TempDir(), "nope.yaml"), 1000); err == nil {
		t.Fatal("a missing seed file must be fatal")
	}
	if _, err := Seed(ctx, s, write(t, "version: 1\nrouts: []\n"), 1000); err == nil {
		t.Fatal("a malformed seed file must be fatal")
	}
	// And it wrote nothing on the way out.
	var mark SeedMark
	if err := s.GetSetting(ctx, store.SettingConfigSeed, &mark); err == nil {
		t.Fatal("a failed seed must leave no mark")
	}
}

// TestSeedWithoutFileDoesNothing: no file named, nothing happens, no error.
func TestSeedWithoutFileDoesNothing(t *testing.T) {
	s := openTemp(t)
	seeded, err := Seed(context.Background(), s, "", 1000)
	if err != nil || seeded {
		t.Fatalf("Seed with no file = %v, %v", seeded, err)
	}
}

// TestSeedReservesTheVaultEntries: a seeded gateway starts complete or says
// what it is missing. The entries are created empty so they are findable.
func TestSeedReservesTheVaultEntries(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)
	file := strings.Replace(seedFile, "http://up.invalid", "http://${api-host}:8080", 1)
	if _, err := Seed(ctx, s, write(t, file), 1000); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if _, err := s.GetVaultEntry(ctx, "infra", "api-host"); err != nil {
		t.Fatalf("the reference should have been reserved: %v", err)
	}
}

// A file the gateway does not apply is not dropped either (CFG-03, LIFE-02):
// it lands on the shelf as a saved configuration, once per content, where the
// console can compare it and someone can decide to set it as current. What
// runs is untouched.
func TestADifferingFileIsShelvedNotApplied(t *testing.T) {
	ctx := context.Background()
	s := openTemp(t)
	path := write(t, seedFile)
	if _, err := Seed(ctx, s, path, 1000); err != nil {
		t.Fatal(err)
	}
	if shelf, _ := s.ListConfigurations(ctx); len(shelf) != 0 {
		t.Fatalf("the file that seeded was shelved too: %+v", shelf)
	}

	// Somebody edits the file and restarts.
	changed := strings.Replace(seedFile, "From the file", "From the edited file", 1)
	if err := os.WriteFile(path, []byte(changed), 0o600); err != nil {
		t.Fatal(err)
	}
	if seeded, err := Seed(ctx, s, path, 2000); err != nil || seeded {
		t.Fatalf("a changed file was applied: %v %v", seeded, err)
	}
	if r, _ := s.GetRole(ctx, "from-file"); r.Name != "From the file" {
		t.Fatalf("what runs was changed: %q", r.Name)
	}
	shelf, _ := s.ListConfigurations(ctx)
	if len(shelf) != 1 || !strings.HasPrefix(shelf[0].Name, "meerkat.yaml (") {
		t.Fatalf("the changed file is not on the shelf: %+v", shelf)
	}
	full, _ := s.GetConfiguration(ctx, shelf[0].ID)
	if !strings.Contains(full.Document, "From the edited file") {
		t.Fatalf("the shelved document is not the file:\n%s", full.Document)
	}

	// A second restart with the same file offers nothing new.
	if _, err := Seed(ctx, s, path, 3000); err != nil {
		t.Fatal(err)
	}
	if shelf, _ := s.ListConfigurations(ctx); len(shelf) != 1 {
		t.Fatalf("the same file was shelved twice: %d", len(shelf))
	}
}
