import { CoreKey } from './m3';

// Every colour token a theme carries, as the gateway emits them
// (store.ThemeCSSVar): --mk- and the role in kebab case.
export function cssVar(key: string): string {
  return '--mk-' + key.replace(/[A-Z]/g, (c) => '-' + c.toLowerCase());
}

// The glow of the flow pages: the one token the Material spec has no role for
// (store.GlowToken).
export const GLOW = 'night';

export interface CoreRow {
  key: CoreKey;
  label: string;
  // What the colour is FOR, said once under its name - the builder's own
  // wording where it has one.
  hint?: string;
}

export const CORE_ROWS: CoreRow[] = [
  { key: 'primary', label: $localize`:@@Theme_core_primary:Primary`, hint: $localize`:@@Theme_core_primary_hint:Acts as the source colour` },
  { key: 'secondary', label: $localize`:@@Theme_core_secondary:Secondary` },
  { key: 'tertiary', label: $localize`:@@Theme_core_tertiary:Tertiary` },
  { key: 'error', label: $localize`:@@Theme_core_error:Error` },
  { key: 'neutral', label: $localize`:@@Theme_core_neutral:Neutral`, hint: $localize`:@@Theme_core_neutral_hint:Backgrounds and surfaces` },
  {
    key: 'neutralVariant',
    label: $localize`:@@Theme_core_neutral_variant:Neutral variant`,
    hint: $localize`:@@Theme_core_neutral_variant_hint:Medium emphasis and outlines`,
  },
];

// What a source colour DRIVES on the page, so hovering it blinks those tokens
// in the preview. Backgrounds only for the neutral: blinking a surface and the
// text on it together would make the text vanish into it.
export const CORE_DRIVES: Record<CoreKey, string[]> = {
  primary: ['primary', 'primaryContainer', 'primaryFixed', 'primaryFixedDim', 'inversePrimary', 'surfaceTint', GLOW],
  secondary: ['secondary', 'secondaryContainer', 'secondaryFixed', 'secondaryFixedDim'],
  tertiary: ['tertiary', 'tertiaryContainer', 'tertiaryFixed', 'tertiaryFixedDim'],
  error: ['error', 'errorContainer'],
  neutral: [
    'background', 'surface', 'surfaceDim', 'surfaceBright', 'surfaceContainerLowest', 'surfaceContainerLow',
    'surfaceContainer', 'surfaceContainerHigh', 'surfaceContainerHighest', 'inverseSurface',
  ],
  neutralVariant: ['surfaceVariant', 'onSurfaceVariant', 'outline', 'outlineVariant'],
};

export interface RoleGroup {
  label: string;
  roles: string[];
}

// The generated roles, grouped the way the builder's scheme view groups them.
export const ROLE_GROUPS: RoleGroup[] = [
  {
    label: $localize`:@@Theme_roles_primary:Primary`,
    roles: ['primary', 'onPrimary', 'primaryContainer', 'onPrimaryContainer', 'inversePrimary', 'surfaceTint'],
  },
  {
    label: $localize`:@@Theme_roles_secondary:Secondary`,
    roles: ['secondary', 'onSecondary', 'secondaryContainer', 'onSecondaryContainer'],
  },
  {
    label: $localize`:@@Theme_roles_tertiary:Tertiary`,
    roles: ['tertiary', 'onTertiary', 'tertiaryContainer', 'onTertiaryContainer'],
  },
  { label: $localize`:@@Theme_roles_error:Error`, roles: ['error', 'onError', 'errorContainer', 'onErrorContainer'] },
  {
    label: $localize`:@@Theme_roles_surface:Surface`,
    roles: [
      'surface', 'onSurface', 'surfaceVariant', 'onSurfaceVariant', 'surfaceDim', 'surfaceBright',
      'surfaceContainerLowest', 'surfaceContainerLow', 'surfaceContainer', 'surfaceContainerHigh',
      'surfaceContainerHighest', 'inverseSurface', 'inverseOnSurface', 'background', 'onBackground',
    ],
  },
  { label: $localize`:@@Theme_roles_outline:Outline`, roles: ['outline', 'outlineVariant', 'shadow', 'scrim'] },
  {
    label: $localize`:@@Theme_roles_fixed:Fixed`,
    roles: [
      'primaryFixed', 'primaryFixedDim', 'onPrimaryFixed', 'onPrimaryFixedVariant',
      'secondaryFixed', 'secondaryFixedDim', 'onSecondaryFixed', 'onSecondaryFixedVariant',
      'tertiaryFixed', 'tertiaryFixedDim', 'onTertiaryFixed', 'onTertiaryFixedVariant',
    ],
  },
  { label: $localize`:@@Theme_roles_meerkat:Flow pages`, roles: [GLOW] },
];

// A role's name as a reader says it: "surfaceContainerHigh" -> "Surface
// container high". The glow is Meerkat's own and has its own name.
export function roleLabel(role: string): string {
  if (role === GLOW) return $localize`:@@Glow:Glow`;
  const words = role.replace(/([A-Z])/g, ' $1').toLowerCase();
  return words[0].toUpperCase() + words.slice(1);
}
