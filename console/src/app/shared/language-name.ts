// What a BCP 47 tag names, in whichever language one is asked to read it.
//
// fallback: 'none' is the whole point, and it is why this is a function rather
// than three lines written twice. Left to itself, DisplayNames invents an
// answer for a tag it does not know: fr-TY came back "francais (TY)" in one
// browser and "fr-TY" in another, so the same typo was accepted in one and
// refused in the other. Asked this way it answers nothing at all, which is
// both the right label and the right verdict.
export function languageName(code: string, inLang: string): string {
  if (!code) return '';
  try {
    return new Intl.DisplayNames([inLang], { type: 'language', fallback: 'none' }).of(code) ?? '';
  } catch {
    return '';
  }
}

// The tag in its canonical spelling, or empty when it is not one. The browser
// carries the BCP 47 grammar; a regexp of ours would be a second opinion that
// disagrees in the cases nobody tests.
export function canonicalTag(raw: string): string {
  const v = raw.trim();
  if (!v) return '';
  try {
    return Intl.getCanonicalLocales(v)[0] ?? '';
  } catch {
    return '';
  }
}
