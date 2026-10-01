package auth

import (
	"sort"
	"strings"
	"sync"
)

// The strings an integrator corrected or added, over the ones we ship (I18N).
//
// Twenty catalogues are embedded in the binary, and nineteen of them are
// missing fourteen keys - so "the translation is wrong" and "the translation
// is absent" are both states an installation can be in today, with nothing to
// do about either but wait for a release. An override layer is the answer: a
// map per language holding ONLY what differs.
//
// ONLY WHAT DIFFERS, and that is the whole design. Storing a full copy would
// freeze a language at the day it was copied: every later fix we ship would be
// invisible behind it, and the installation would drift further from the
// product with each release. A thin layer picks up everything it does not
// override, and a reset is a delete.
//
// The layer is held here rather than read from the store on each lookup: these
// maps are consulted once per rendered string. Whoever owns the store pushes
// the layer in - at startup and after every write - the way the exporter is
// pointed at a collector.

var (
	overrideMu sync.RWMutex
	// overrides maps a language code to the entries that replace the embedded
	// ones. Nil until somebody pushes a layer, which is the state of every
	// installation that has never edited a string.
	overrides map[string]map[string]string
	// merged caches the result per language, invalidated whole on a push.
	merged map[string]map[string]string
)

// SetLocaleOverrides replaces the whole layer. Whole rather than per-language
// because a push follows a read of the table, and a partial push would leave a
// language that was just reset still overridden.
func SetLocaleOverrides(layer map[string]map[string]string) {
	overrideMu.Lock()
	defer overrideMu.Unlock()
	overrides = layer
	merged = nil
}

// catalogue is the strings for one language: what we embed, with the
// installation's own entries over the top. Unknown language falls back to
// English rather than returning an empty map, which would blank every label on
// the page rather than one.
func catalogue(lang string) map[string]string {
	overrideMu.RLock()
	if m, ok := merged[lang]; ok {
		overrideMu.RUnlock()
		return m
	}
	overrideMu.RUnlock()

	overrideMu.Lock()
	defer overrideMu.Unlock()
	if m, ok := merged[lang]; ok {
		return m
	}
	base, ok := messages[lang]
	if !ok {
		// A language the binary does not carry can still be overridden into
		// existence: an integrator adds the code, then fills it. Until they
		// have, the pages stand on English - a label in another language is
		// readable, and a blank one is a broken page. Their own wordings go
		// over the top as they arrive, and the editor counts what is left.
		base = messages["en"]
		if _, added := overrides[lang]; !added {
			// Not added either: an unknown tag, which is simply English.
			lang = "en"
		}
	}
	out := make(map[string]string, len(base)+len(overrides[lang]))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overrides[lang] {
		if v != "" {
			out[k] = v
		}
	}
	if merged == nil {
		merged = map[string]map[string]string{}
	}
	merged[lang] = out
	return out
}

// KnownLanguages is every language this installation can render: the ones we
// embed, plus any an integrator has added by overriding. Sorted, so a list is
// the same list twice.
func KnownLanguages() []string {
	overrideMu.RLock()
	defer overrideMu.RUnlock()
	seen := make(map[string]bool, len(messages)+len(overrides))
	for l := range messages {
		seen[l] = true
	}
	for l := range overrides {
		seen[l] = true
	}
	out := make([]string, 0, len(seen))
	for l := range seen {
		out = append(out, l)
	}
	sort.Strings(out)
	return out
}

// IsKnownLanguage says this installation can render a language: one we embed, or
// one somebody added here. Asked before a language is honoured, because an
// unknown tag has to become English somewhere.
func IsKnownLanguage(code string) bool {
	if _, ok := messages[code]; ok {
		return true
	}
	overrideMu.RLock()
	defer overrideMu.RUnlock()
	_, added := overrides[code]
	return added
}

// IsOverridden reports whether one key in one language was changed here. The
// editor marks those: a string one can reset is worth telling apart from one
// that is still ours.
func IsOverridden(lang, key string) bool {
	overrideMu.RLock()
	defer overrideMu.RUnlock()
	return overrides[lang][key] != ""
}

// EmbeddedString is what the product ships for a key, whatever the layer says.
// The editor shows it beside an edited value, so "reset this one" can be seen
// before it is done.
func EmbeddedString(lang, key string) string {
	return messages[lang][key]
}

// OwnString is the wording this language ITSELF ships for a key - empty when it
// has none and the loader stood English in its place. That difference is what
// the editor counts: with the backfill in front of it, every catalogue looks
// complete, and "fourteen strings left to translate" reads as zero.
func OwnString(lang, key string) string {
	if borrowed[lang][key] {
		return ""
	}
	return messages[lang][key]
}

// ReferenceStrings is the English catalogue: what a translator reads while
// filling another language, and what the editor uses as a placeholder.
func ReferenceStrings() map[string]string { return messages["en"] }

// IsEmbeddedLanguage says the product ships this language. False means somebody
// added it here, which changes what "reset" means: there is nothing to go back
// to, so resetting removes it.
func IsEmbeddedLanguage(code string) bool {
	_, ok := messages[code]
	return ok
}

// LanguageName is a language's own name, never translated - a person looking
// for their language looks for the word they would write, not for its English
// label. Falls back to the code for one we have no endonym for.
func LanguageName(code string) string {
	if n, ok := langNames[code]; ok {
		return n
	}
	// A variant of a language we do know: name the language and keep the rest
	// of the tag beside it, so fr-CA reads as French and not as four letters.
	// Nothing here resolves a region to its name - that would be a CLDR table
	// the binary does not carry, and "Francais (CA)" says enough.
	if i := strings.Index(code, "-"); i > 0 {
		if n, ok := langNames[code[:i]]; ok {
			return n + " (" + code[i+1:] + ")"
		}
	}
	return code
}
