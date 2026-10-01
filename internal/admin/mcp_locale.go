package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/softwarity/meerkat/internal/auth"
)

// A WHOLE LANGUAGE, written by an agent (I18N-05).
//
// Correcting one wording is a judgement about a sentence, made against the
// screen it appears on - which is what the console's editor is for, and the
// reason this section was out of an agent's reach at first. Producing a
// catalogue of two hundred and seventy strings in a language nobody on the
// team speaks is the opposite kind of work: nobody does it by hand, and the
// editor is then the place to REVIEW it rather than the place to write it.
//
// So the shape here is deliberately coarse. read_language hands over the whole
// catalogue with the English beside each key; write_language takes the whole
// answer back. What an agent must not be encouraged to do is a wording at a
// time on a live gateway, which is why there is no write_string.

// localeStringView is one string as an agent needs it to translate: the key it
// lives under, what English says, what this language says today, and whether
// that wording is its own or English standing in.
type localeStringView struct {
	Key string `json:"key"`
	// English is the reference. A key name is not a sentence, and a translator
	// - machine or not - works from the meaning.
	English string `json:"english"`
	// Current is what the gateway renders in this language today. Equal to
	// English when nothing has been written yet.
	Current string `json:"current"`
	// Missing says Current is English standing in, not a translation: these
	// are the ones to write.
	Missing bool `json:"missing"`
	// Screens is how many built-in screens render this string. More than one
	// means a wording that has to suit several places at once.
	Screens int `json:"screens"`
}

func (a *API) toolListLanguages(ctx context.Context, _ json.RawMessage) (any, error) {
	layer, err := a.st.LocaleOverrides(ctx)
	if err != nil {
		return nil, err
	}
	ref := auth.ReferenceStrings()
	type row struct {
		Code     string `json:"code"`
		Name     string `json:"name"`
		Shipped  bool   `json:"shipped"`
		Edited   bool   `json:"edited"`
		ToWrite  int    `json:"toWrite"`
		Complete bool   `json:"complete"`
	}
	out := make([]row, 0, len(auth.KnownLanguages()))
	for _, code := range auth.KnownLanguages() {
		left := 0
		for key := range ref {
			if auth.OwnString(code, key) == "" && layer[code][key] == "" {
				left++
			}
		}
		out = append(out, row{
			Code: code, Name: auth.LanguageName(code),
			Shipped: auth.IsEmbeddedLanguage(code), Edited: len(layer[code]) > 0,
			ToWrite: left, Complete: left == 0,
		})
	}
	return map[string]any{"strings": len(ref), "languages": out}, nil
}

func (a *API) toolReadLanguage(ctx context.Context, args json.RawMessage) (any, error) {
	var in struct {
		Code string `json:"code"`
		Only string `json:"only"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return nil, fmt.Errorf("this is not a language request: %w", err)
	}
	if in.Code == "" {
		return nil, fmt.Errorf("which language? pass a code from list_languages, or a new BCP 47 tag")
	}
	layer, err := a.st.LocaleOverrides(ctx)
	if err != nil {
		return nil, err
	}
	ref := auth.ReferenceStrings()
	keys := make([]string, 0, len(ref))
	for k := range ref {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]localeStringView, 0, len(keys))
	for _, key := range keys {
		own := auth.OwnString(in.Code, key)
		current := own
		if over := layer[in.Code][key]; over != "" {
			current = over
		}
		missing := current == ""
		if missing {
			// What the page really shows there: English, the fallback on the
			// data plane too. Handing back an empty string would read as "this
			// key has no text", which is not what a visitor sees.
			current = ref[key]
		}
		if in.Only == "missing" && !missing {
			continue
		}
		out = append(out, localeStringView{
			Key: key, English: ref[key], Current: current, Missing: missing,
			Screens: auth.ScreenCount(key),
		})
	}
	return map[string]any{
		"code": in.Code, "name": auth.LanguageName(in.Code),
		"shipped": auth.IsEmbeddedLanguage(in.Code),
		"strings": out,
	}, nil
}

func (a *API) toolWriteLanguage(ctx context.Context, args json.RawMessage) (any, error) {
	var in struct {
		Code    string            `json:"code"`
		Entries map[string]string `json:"entries"`
		Replace bool              `json:"replace"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return nil, fmt.Errorf("this is not a language: %w", err)
	}
	if !validLanguageTag(in.Code) {
		return nil, fmt.Errorf("%q is not a language tag: expected a BCP 47 code such as fr, pt-BR or zh-Hans", in.Code)
	}
	layer, err := a.st.LocaleOverrides(ctx)
	if err != nil {
		return nil, err
	}
	next := map[string]string{}
	if !in.Replace {
		for k, v := range layer[in.Code] {
			next[k] = v
		}
	}
	known := auth.ReferenceStrings()
	var unknown []string
	for k, v := range in.Entries {
		if _, ok := known[k]; !ok {
			unknown = append(unknown, k)
			continue
		}
		if v == "" || v == auth.OwnString(in.Code, k) {
			// Equal to what we ship is not a correction, and keeping it would
			// freeze that string at today's wording behind every later fix.
			delete(next, k)
			continue
		}
		next[k] = v
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("%d of these are not strings this product has (the catalogue is fixed, "+
			"only its wordings are yours): %v - call read_language for the keys", len(unknown), unknown)
	}
	if err := a.st.SetLocaleOverride(ctx, in.Code, next); err != nil {
		return nil, err
	}
	a.applyLocalesCtx(ctx)
	if actor := mcpActor(ctx); actor.ID != "" {
		a.auditEvent(ctx, actor, "locale.update", "locale", in.Code, in.Code, "",
			fmt.Sprintf("%d strings (agent)", len(next)))
	}

	left := 0
	for key := range known {
		if auth.OwnString(in.Code, key) == "" && next[key] == "" {
			left++
		}
	}
	return map[string]any{
		"code": in.Code, "written": len(next), "toWrite": left, "complete": left == 0,
	}, nil
}

// applyLocalesCtx pushes the whole layer into the data plane. The REST handler
// has the same three lines with a request in hand; an agent has only a context.
func (a *API) applyLocalesCtx(ctx context.Context) {
	layer, err := a.st.LocaleOverrides(ctx)
	if err != nil {
		return
	}
	auth.SetLocaleOverrides(layer)
}
