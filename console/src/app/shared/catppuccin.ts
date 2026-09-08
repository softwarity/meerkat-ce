import { catppuccinMocha } from '@catppuccin/codemirror';
import { Prec } from '@codemirror/state';
import { EditorView } from '@codemirror/view';

// The colours every editor in this console writes with: Catppuccin Mocha, from
// the flavour's OWN package (codemirror.catppuccin.com).
//
// It replaced a hand-written mapping of the same palette, and the package won
// on coverage rather than on size: it themes the search panel, the tooltips,
// the autocomplete and the search matches, which this console actually uses -
// the configuration screen opens a search panel, the respond editor completes
// as you type - and every one of those was left wearing the browser's own
// colours by the hand-rolled version.
//
// ONE source for all of them, which is the point. The product already learned
// this in a smaller way: template-highlight.ts says it about its own six
// colours, and was itself written in Macchiato back when nothing else had a
// flavour at all. A verb that is peach in the role editor and something else
// in a YAML two screens away reads as two products.
export const catppuccin = catppuccinMocha;

// The BACKGROUND is the console's, not the theme's, and every editor adds this
// after the theme above.
//
// The editors sit inside cards and drawers that follow --mat-sys-*, and an
// installation chooses its own palette (THEME-01). A base colour imported from
// a syntax theme is the one thing that would show the seam - so Catppuccin owns
// the syntax and the console owns the surface it is written on.
// Prec.highest, and it is not a precaution: added plainly it LOST to the
// theme's own rule and the editors came up wearing #1e1e2e, which is exactly
// the imported slab this override exists to prevent. Measured, not assumed.
export const editorSurface = Prec.highest(
  EditorView.theme({
    '&': { backgroundColor: 'transparent' },
    '.cm-gutters': { backgroundColor: 'transparent' },
    // The class the hand-rolled comment pass paints with (snippet.component):
    // shell and PromQL have no grammar here, and the flavour's own theme
    // cannot know about a class this console invented.
    '.cm-comment': { color: '#6c7086', fontStyle: 'italic' },
  }),
);

// The three colours template-highlight.ts paints its verbs with, named rather
// than copied: peach, green and yellow of the same flavour.
export const MOCHA = {
  peach: '#fab387',
  green: '#a6e3a1',
  yellow: '#f9e2af',
} as const;
