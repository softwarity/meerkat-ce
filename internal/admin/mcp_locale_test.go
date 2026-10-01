package admin

import (
	"encoding/json"
	"strings"
	"testing"
)

// A whole language, added by an agent: read it empty, write it, read it back.
func TestAnAgentWritesAWholeLanguage(t *testing.T) {
	f := setupBare(t)
	ctx := mcpCtx(rootUser(t, f))

	// Nobody has heard of Occitan: it reads as an empty language, every string
	// English standing in, which is what a translator starts from.
	raw, err := f.api.toolReadLanguage(ctx, json.RawMessage(`{"code":"oc","only":"missing"}`))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(raw)
	var read struct {
		Code    string `json:"code"`
		Strings []struct {
			Key, English string
			Missing      bool
		} `json:"strings"`
	}
	_ = json.Unmarshal(b, &read)
	if len(read.Strings) < 200 {
		t.Fatalf("an unknown language reports %d strings to write", len(read.Strings))
	}

	// Translate it, the way an agent would: every key it was handed.
	entries := map[string]string{}
	for _, s := range read.Strings {
		entries[s.Key] = "oc:" + s.English
	}
	payload, _ := json.Marshal(map[string]any{"code": "oc", "entries": entries})
	out, err := f.api.toolWriteLanguage(ctx, payload)
	if err != nil {
		t.Fatal(err)
	}
	wrote, _ := json.Marshal(out)
	var res struct {
		Written  int  `json:"written"`
		ToWrite  int  `json:"toWrite"`
		Complete bool `json:"complete"`
	}
	_ = json.Unmarshal(wrote, &res)
	if !res.Complete || res.ToWrite != 0 {
		t.Errorf("after writing %d strings there are %d left", res.Written, res.ToWrite)
	}

	// And the gateway speaks it: the page renders what was written.
	list, _ := f.api.toolListLanguages(ctx, nil)
	lb, _ := json.Marshal(list)
	if !strings.Contains(string(lb), `"code":"oc"`) {
		t.Errorf("the language is not listed: %s", lb)
	}
	back, _ := f.api.toolReadLanguage(ctx, json.RawMessage(`{"code":"oc","only":"missing"}`))
	bb, _ := json.Marshal(back)
	var left struct {
		Strings []struct{ Key string } `json:"strings"`
	}
	_ = json.Unmarshal(bb, &left)
	if len(left.Strings) != 0 {
		t.Errorf("%d strings still to write", len(left.Strings))
	}

	// A key the catalogue does not have is refused BY NAME.
	bad, _ := json.Marshal(map[string]any{"code": "oc", "entries": map[string]string{"notAKey": "x"}})
	if _, err := f.api.toolWriteLanguage(ctx, bad); err == nil || !strings.Contains(err.Error(), "notAKey") {
		t.Errorf("an invented key was accepted or refused without naming it: %v", err)
	}
}
