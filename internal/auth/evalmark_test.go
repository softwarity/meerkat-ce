package auth

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/softwarity/meerkat/internal/evalmark"
	"github.com/softwarity/meerkat/internal/store"
)

// The evaluation marks are wired through DATA, not through the template's
// source, and this is what proves it - in every image, including the two that
// never carry a mark.
//
// The wiring is the fragile half. The pages are built from package-level
// variables, so a mark folded into their source would be read during this
// package's init(), and nothing orders that against ee/eval's init(): the two
// packages do not depend on each other, so the order is whatever the linker
// happened to choose. It worked, alphabetically, until someone renamed a
// package. Read at RENDER time, as here, it cannot come out empty.
//
// The test fills the marks itself rather than asking what image it runs on:
// that way the community build - the one with no evaluation code in it at all
// - still fails the day somebody drops {{.EvalBanner}} from the chrome.
func TestEvaluationMarksAreReadWhenThePageIsRendered(t *testing.T) {
	const banner = `<p class="mk-eval">EVALUATION NOTICE FOR THIS TEST</p>`
	const css = `.mk-eval { position: fixed; }`

	keepBanner, keepCSS := evalmark.FlowBanner, evalmark.FlowCSS
	t.Cleanup(func() { evalmark.FlowBanner, evalmark.FlowCSS = keepBanner, keepCSS })

	mux, _ := setupWithStore(t)
	page := func() string {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", "/login", nil))
		b, _ := io.ReadAll(rec.Result().Body)
		return string(b)
	}

	// Empty is the shipped case: nothing anywhere, and in particular no stray
	// markup where the banner would go.
	evalmark.FlowBanner, evalmark.FlowCSS = "", ""
	if got := page(); strings.Contains(got, "mk-eval") {
		t.Error("a page carried an evaluation mark with none registered")
	}

	// Filled, the same page carries both halves - the banner and the rule that
	// places it. A banner without its CSS is a line of text in the middle of
	// the form.
	evalmark.FlowBanner, evalmark.FlowCSS = banner, css
	got := page()
	if !strings.Contains(got, "EVALUATION NOTICE FOR THIS TEST") {
		t.Error("the banner was registered and the page did not carry it")
	}
	if !strings.Contains(got, "position: fixed") {
		t.Error("the banner's CSS was registered and the page did not carry it")
	}
}

// Nothing an operator can reach may remove the notice. Hiding the Meerkat mark
// is a setting the Enterprise image honours (see mark_test.go), and the whole
// point of the evaluation banner is that it is not that: it answers to the
// build and to nothing else.
func TestHidingTheMarkDoesNotHideTheEvaluationNotice(t *testing.T) {
	const banner = `<p class="mk-eval">EVALUATION NOTICE FOR THIS TEST</p>`
	keep := evalmark.FlowBanner
	t.Cleanup(func() { evalmark.FlowBanner = keep })
	evalmark.FlowBanner = banner

	mux, st := setupWithStore(t)
	var b store.Branding
	_ = st.GetSetting(context.Background(), store.SettingBranding, &b)
	b.HideMark = true
	if err := st.SetSetting(context.Background(), store.SettingBranding, b); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/login", nil))
	body, _ := io.ReadAll(rec.Result().Body)
	if !strings.Contains(string(body), "EVALUATION NOTICE FOR THIS TEST") {
		t.Error("HideMark removed the evaluation notice: a white-label checkbox must not buy that")
	}
}
