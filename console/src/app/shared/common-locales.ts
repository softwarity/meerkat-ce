// The autocomplete's seed where a language is declared: common languages plus
// the usual regional variants.
//
// A seed, not a list of what is allowed - any BCP 47 tag the browser can
// canonicalise and name goes in. It lives here rather than beside one screen
// because the declaration moved: a route says what it speaks now, and the
// gateway's offer is the union of those.
export const COMMON_LOCALES = [
  'ar', 'bg', 'cs', 'da', 'de', 'de-AT', 'de-CH', 'el', 'en', 'en-GB', 'en-US',
  'es', 'es-MX', 'et', 'fi', 'fr', 'fr-BE', 'fr-CA', 'fr-CH', 'he', 'hi', 'hr',
  'hu', 'id', 'it', 'ja', 'ko', 'lt', 'lv', 'nb', 'nl', 'nl-BE', 'pl', 'pt',
  'pt-BR', 'ro', 'ru', 'sk', 'sl', 'sr', 'sv', 'th', 'tr', 'uk', 'vi', 'zh',
  'zh-CN', 'zh-TW',
];
