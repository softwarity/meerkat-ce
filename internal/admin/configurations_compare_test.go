package admin

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/store"
)

// Two configurations that carry nothing differ in nothing, and the answer
// says so with an empty list: "changes": null reads as "no answer" to a
// client that iterates it, which is why no list of this API is ever null.
func TestComparingTwoEmptyConfigurationsAnswersAnEmptyList(t *testing.T) {
	f := setupBare(t)
	ctx := context.Background()
	for _, id := range []string{"c1", "c2"} {
		c := store.Configuration{ID: id, Name: id, Document: "version: 1\n"}
		if err := f.api.st.SaveConfiguration(ctx, &c); err != nil {
			t.Fatal(err)
		}
	}
	code, body := f.call(t, "GET", "/api/configurations/c1/compare/c2", "", f.rootC)
	if code != http.StatusOK {
		t.Fatalf("compare: %d %s", code, body)
	}
	if !strings.Contains(body, `"changes":[]`) {
		t.Fatalf("no difference must read as an empty list: %s", body)
	}
}
