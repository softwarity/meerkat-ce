import {
  argbFromHex,
  DynamicColor,
  DynamicScheme,
  hexFromArgb,
  Hct,
  MaterialDynamicColors as MDC,
  SchemeContent,
  SchemeTonalSpot,
  TonalPalette,
} from '@material/material-color-utilities';

// Material 3 schemes the way Material Theme Builder makes them, so a theme
// exported from it and one made here from the same six colours are the same
// theme, role for role. The gateway generates with a Go port of the same
// library release (internal/m3color), and a golden file written FROM THIS FILE
// holds the two to one output: the live preview draws what the save stores.

export interface CoreColors {
  primary: string;
  secondary?: string;
  tertiary?: string;
  error?: string;
  neutral?: string;
  neutralVariant?: string;
}

export const CORE_KEYS = ['primary', 'secondary', 'tertiary', 'error', 'neutral', 'neutralVariant'] as const;
export type CoreKey = (typeof CORE_KEYS)[number];

export type Contrast = 'standard' | 'medium' | 'high';
export const CONTRAST_LEVEL: Record<Contrast, number> = { standard: 0, medium: 0.5, high: 1 };

// The 49 roles, in the order the builder's JSON export lists them.
export const ROLES = [
  'primary', 'surfaceTint', 'onPrimary', 'primaryContainer', 'onPrimaryContainer',
  'secondary', 'onSecondary', 'secondaryContainer', 'onSecondaryContainer',
  'tertiary', 'onTertiary', 'tertiaryContainer', 'onTertiaryContainer',
  'error', 'onError', 'errorContainer', 'onErrorContainer',
  'background', 'onBackground', 'surface', 'onSurface', 'surfaceVariant', 'onSurfaceVariant',
  'outline', 'outlineVariant', 'shadow', 'scrim', 'inverseSurface', 'inverseOnSurface', 'inversePrimary',
  'primaryFixed', 'onPrimaryFixed', 'primaryFixedDim', 'onPrimaryFixedVariant',
  'secondaryFixed', 'onSecondaryFixed', 'secondaryFixedDim', 'onSecondaryFixedVariant',
  'tertiaryFixed', 'onTertiaryFixed', 'tertiaryFixedDim', 'onTertiaryFixedVariant',
  'surfaceDim', 'surfaceBright', 'surfaceContainerLowest', 'surfaceContainerLow',
  'surfaceContainer', 'surfaceContainerHigh', 'surfaceContainerHighest',
] as const;

export const PALETTE_TONES = [0, 5, 10, 15, 20, 25, 30, 35, 40, 50, 60, 70, 80, 90, 95, 98, 99, 100];

const HEX = /^#([0-9a-f]{3}|[0-9a-f]{6})$/i;

export function isHex(v: unknown): v is string {
  return typeof v === 'string' && HEX.test(v.trim());
}

const hex = (argb: number) => hexFromArgb(argb).toUpperCase();
const role = (name: string) => (MDC as unknown as Record<string, DynamicColor>)[name];
const cap = (s: string) => s[0].toUpperCase() + s.slice(1);

function seeded(color: string, dark: boolean, contrast: number, colorMatch: boolean): DynamicScheme {
  const source = Hct.fromInt(argbFromHex(color));
  return colorMatch ? new SchemeContent(source, dark, contrast) : new SchemeTonalSpot(source, dark, contrast);
}

// One scheme. A scheme seeded by the primary gives every role, then each
// colour that was SET replaces its group with the matching roles of a scheme
// seeded by it - the builder's own composition, quirks included: background
// and onBackground stay on the primary's scheme when a neutral is set.
export function generateScheme(core: CoreColors, dark: boolean, contrast: number, colorMatch: boolean): Record<string, string> {
  const base = seeded(core.primary, dark, contrast, colorMatch);
  const out: Record<string, string> = {};
  for (const r of ROLES) out[r] = hex(role(r).getArgb(base));
  const take = (color: string | undefined, pairs: [string, string][]) => {
    if (!color) return;
    const s = seeded(color, dark, contrast, colorMatch);
    for (const [to, from] of pairs) out[to] = hex(role(from).getArgb(s));
  };
  const group = (name: string, withFixed: boolean): [string, string][] => {
    const up = cap(name);
    const pairs: [string, string][] = [
      [name, 'primary'], ['on' + up, 'onPrimary'],
      [name + 'Container', 'primaryContainer'], ['on' + up + 'Container', 'onPrimaryContainer'],
    ];
    if (withFixed) {
      pairs.push(
        [name + 'Fixed', 'primaryFixed'], ['on' + up + 'Fixed', 'onPrimaryFixed'],
        [name + 'FixedDim', 'primaryFixedDim'], ['on' + up + 'FixedVariant', 'onPrimaryFixedVariant'],
      );
    }
    return pairs;
  };
  const same = (...roles: string[]): [string, string][] => roles.map((r) => [r, r]);
  take(core.secondary, group('secondary', true));
  take(core.tertiary, group('tertiary', true));
  take(core.error, group('error', false));
  take(core.neutral, same('surface', 'onSurface', 'shadow', 'scrim', 'inverseSurface', 'inverseOnSurface',
    'surfaceDim', 'surfaceBright', 'surfaceContainerLowest', 'surfaceContainerLow', 'surfaceContainer',
    'surfaceContainerHigh', 'surfaceContainerHighest'));
  take(core.neutralVariant, same('surfaceVariant', 'onSurfaceVariant', 'outline', 'outlineVariant'));
  return out;
}

// The builder's export palettes: the "content" core palette, whatever the
// variant the schemes were made with.
function contentPalettes(core: CoreColors): Record<string, TonalPalette> {
  const hc = (c: string) => {
    const h = Hct.fromInt(argbFromHex(c));
    return [h.hue, h.chroma] as const;
  };
  const [h, c] = hc(core.primary);
  const p: Record<string, TonalPalette> = {
    primary: TonalPalette.fromHueAndChroma(h, c),
    secondary: TonalPalette.fromHueAndChroma(h, c / 3),
    tertiary: TonalPalette.fromHueAndChroma(h + 60, c / 2),
    error: TonalPalette.fromHueAndChroma(25, 84),
    neutral: TonalPalette.fromHueAndChroma(h, Math.min(c / 12, 4)),
    'neutral-variant': TonalPalette.fromHueAndChroma(h, Math.min(c / 6, 8)),
  };
  if (core.secondary) p['secondary'] = TonalPalette.fromHueAndChroma(...hc(core.secondary));
  if (core.tertiary) p['tertiary'] = TonalPalette.fromHueAndChroma(...hc(core.tertiary));
  if (core.error) p['error'] = TonalPalette.fromHueAndChroma(...hc(core.error));
  if (core.neutral) {
    const [nh, nc] = hc(core.neutral);
    p['neutral'] = TonalPalette.fromHueAndChroma(nh, Math.min(nc / 12, 4));
  }
  if (core.neutralVariant) {
    const [vh, vc] = hc(core.neutralVariant);
    p['neutral-variant'] = TonalPalette.fromHueAndChroma(vh, Math.min(vc / 6, 8));
  }
  return p;
}

export function generatePalettes(core: CoreColors): Record<string, Record<string, string>> {
  const p = contentPalettes(core);
  const out: Record<string, Record<string, string>> = {};
  for (const k of ['primary', 'secondary', 'tertiary', 'neutral', 'neutral-variant']) {
    out[k] = {};
    for (const t of PALETTE_TONES) out[k][String(t)] = hex(p[k].tone(t));
  }
  return out;
}

// What the builder SHOWS for a colour nobody set: tone 60 of the palette it
// would be derived from. A colour the operator has not chosen still has a
// value, and showing it is what lets them decide whether to change it.
export function derivedCore(core: CoreColors): Record<CoreKey, string> {
  const p = contentPalettes({ primary: core.primary });
  const pick = (k: CoreKey, palette: string) => core[k] || hex(p[palette].tone(60));
  return {
    primary: core.primary,
    secondary: pick('secondary', 'secondary'),
    tertiary: pick('tertiary', 'tertiary'),
    error: pick('error', 'error'),
    neutral: pick('neutral', 'neutral'),
    neutralVariant: pick('neutralVariant', 'neutral-variant'),
  };
}

// The glow of the flow pages, a colour the spec has no role for: a deep tint
// of the primary in the dark scheme, a pale one in the light. Same recipe as
// the gateway's (store.glowTone).
export function glow(primary: string, dark: boolean): string {
  const h = Hct.fromInt(argbFromHex(primary));
  return hex(TonalPalette.fromHueAndChroma(h.hue, 16).tone(dark ? 20 : 80));
}

// The two schemes a theme stores, at its contrast.
export function themeSchemes(core: CoreColors, contrast: Contrast, colorMatch: boolean) {
  const level = CONTRAST_LEVEL[contrast];
  // Lower case, as the gateway stores them: the builder writes upper case,
  // and a theme read back must compare equal to what the preview showed.
  const lower = (m: Record<string, string>) => Object.fromEntries(Object.entries(m).map(([k, v]) => [k, v.toLowerCase()]));
  const light = lower({ ...generateScheme(core, false, level, colorMatch), night: glow(core.primary, false) });
  const dark = lower({ ...generateScheme(core, true, level, colorMatch), night: glow(core.primary, true) });
  return { light, dark };
}

// The builder's JSON export, so a theme made here opens wherever one made
// there does. Meerkat's own settings ride along under a key of their own,
// which the builder's readers ignore.
export function toBuilderJson(core: CoreColors, meerkat: Record<string, unknown>): string {
  const coreColors: Record<string, string> = { primary: core.primary.toUpperCase() };
  for (const k of CORE_KEYS) if (k !== 'primary' && core[k]) coreColors[k] = core[k]!.toUpperCase();
  const schemes: Record<string, Record<string, string>> = {};
  for (const [name, dark, level] of SCHEME_NAMES) {
    schemes[name] = generateScheme(core, dark, level, !!meerkat['colorMatch']);
  }
  const stamp = new Date().toISOString().replace('T', ' ').slice(0, 19);
  return JSON.stringify(
    {
      description: 'TYPE: CUSTOM\nMeerkat theme export ' + stamp,
      seed: coreColors['primary'],
      coreColors,
      extendedColors: [],
      schemes,
      palettes: generatePalettes(core),
      meerkat,
    },
    null,
    4,
  );
}

export const SCHEME_NAMES: [string, boolean, number][] = [
  ['light', false, 0],
  ['light-medium-contrast', false, 0.5],
  ['light-high-contrast', false, 1],
  ['dark', true, 0],
  ['dark-medium-contrast', true, 0.5],
  ['dark-high-contrast', true, 1],
];

// Reads a builder export. The file does not say whether "Color match" was on,
// so the schemes it carries are asked: regenerate both ways and keep the one
// that reproduces them. An older export (the builder's contrast rules moved in
// 2025) matches neither - it still imports, and the reader is told the
// colours were regenerated.
export function fromBuilderJson(data: unknown): { core: CoreColors; colorMatch: boolean; exact: boolean; meerkat: Record<string, unknown> } | null {
  if (!data || typeof data !== 'object') return null;
  const d = data as Record<string, unknown>;
  const cc = d['coreColors'] as Record<string, unknown> | undefined;
  if (!cc || !isHex(cc['primary'])) return null;
  const core: CoreColors = { primary: (cc['primary'] as string).toUpperCase() };
  for (const k of CORE_KEYS) {
    if (k !== 'primary' && isHex(cc[k])) core[k] = (cc[k] as string).toUpperCase();
  }
  const meerkat = (d['meerkat'] && typeof d['meerkat'] === 'object' ? d['meerkat'] : {}) as Record<string, unknown>;
  const light = (d['schemes'] as Record<string, Record<string, string>> | undefined)?.['light'];
  const matches = (cm: boolean) => {
    if (!light) return false;
    const got = generateScheme(core, false, 0, cm);
    return ROLES.every((r) => (light[r] ?? '').toUpperCase() === got[r]);
  };
  if (typeof meerkat['colorMatch'] === 'boolean') {
    const cm = meerkat['colorMatch'] as boolean;
    return { core, colorMatch: cm, exact: true, meerkat };
  }
  if (matches(false)) return { core, colorMatch: false, exact: true, meerkat };
  if (matches(true)) return { core, colorMatch: true, exact: true, meerkat };
  return { core, colorMatch: false, exact: !light, meerkat };
}
