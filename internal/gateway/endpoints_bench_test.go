package gateway

import (
	"net/http"
	"os"
	"testing"

	"github.com/softwarity/meerkat/internal/metrics"
	"github.com/softwarity/meerkat/internal/openapi"
)

// What naming an operation costs, per request and per refresh.
//
// Two very different budgets, and keeping them apart is the whole arrangement:
// parsing a spec happens once per route every ten minutes on a control-plane
// goroutine, matching one happens on every single request. Only the second has
// to be free.
//
// Needs a real spec: MEERKAT_BENCH_SPEC=/path/to/spec.json
func loadBenchSpec(b *testing.B) []openapi.Operation {
	b.Helper()
	path := os.Getenv("MEERKAT_BENCH_SPEC")
	if path == "" {
		b.Skip("set MEERKAT_BENCH_SPEC to a real spec")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		b.Fatal(err)
	}
	spec, err := openapi.Parse(raw)
	if err != nil {
		b.Fatal(err)
	}
	return spec.Operations
}

// Parsing the spec: the refresh path, once per route per ten minutes.
func BenchmarkParseSpec(b *testing.B) {
	path := os.Getenv("MEERKAT_BENCH_SPEC")
	if path == "" {
		b.Skip("set MEERKAT_BENCH_SPEC to a real spec")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := openapi.Parse(raw); err != nil {
			b.Fatal(err)
		}
	}
}

// Building the index from what was parsed: also the refresh path.
func BenchmarkBuildIndex(b *testing.B) {
	ops := loadBenchSpec(b)
	reg := metrics.NewRegistry()
	b.ReportAllocs()
	for b.Loop() {
		idx := newEndpointIndex(0)
		for _, op := range ops {
			idx.add(reg, "r1", op.Method, op.Path)
		}
	}
}

// Naming ONE request: the only one on the hot path, and it runs after the
// answer has been written.
func BenchmarkNameOperation(b *testing.B) {
	ops := loadBenchSpec(b)
	reg := metrics.NewRegistry()
	idx := newEndpointIndex(0)
	for _, op := range ops {
		idx.add(reg, "r1", op.Method, op.Path)
	}
	b.Logf("%d operations in the index", len(ops))

	req, _ := http.NewRequest("GET", "http://x/anything/whatever", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if idx.of(req) == nil {
			b.Fatal("no match")
		}
	}
}

// And the miss, which is the worst case: every candidate compared, none taken.
func BenchmarkNameOperationMiss(b *testing.B) {
	ops := loadBenchSpec(b)
	reg := metrics.NewRegistry()
	idx := newEndpointIndex(0)
	for _, op := range ops {
		idx.add(reg, "r1", op.Method, op.Path)
	}
	req, _ := http.NewRequest("GET", "http://x/nothing/declares/this/one", nil)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if idx.of(req) != nil {
			b.Fatal("unexpected match")
		}
	}
}

// And the deduced path, which is what a route with no spec pays: fold the
// shape, then find the template it folded to.
func BenchmarkDeduceKnownShape(b *testing.B) {
	slot := &opsSlot{reg: metrics.NewRegistry(), routeID: "r1"}
	req, _ := http.NewRequest("GET", "http://x/orders/1042/items", nil)
	slot.deduce(req, 0) // the template exists after the first request
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if slot.deduce(req, 0) == nil {
			b.Fatal("no counters")
		}
	}
}

// The same, on a path where nothing folds - no builder, no allocation.
func BenchmarkDeduceNothingFolds(b *testing.B) {
	slot := &opsSlot{reg: metrics.NewRegistry(), routeID: "r1"}
	req, _ := http.NewRequest("GET", "http://x/anything/whatever", nil)
	slot.deduce(req, 0)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if slot.deduce(req, 0) == nil {
			b.Fatal("no counters")
		}
	}
}
