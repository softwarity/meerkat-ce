import { Range } from '@codemirror/state';
import { Decoration, DecorationSet, EditorView, ViewPlugin, ViewUpdate } from '@codemirror/view';

// Colouring a Go template, shallowly.
//
// Two editors of this console write the same dialect: the respond body and the
// role pipeline. They only differ by their verbs, so the colours live here
// rather than twice - a product whose two code editors highlight differently
// reads as two products.
//
// Deliberately regex, not a parser: this is highlighting, and the gateway owns
// the truth - it answers in the preview under each editor.

export interface TemplateSyntax {
  // The verbs to call out. A name starting with a dot is a method (.Keep), the
  // rest are functions (join, cutHead).
  verbs: readonly string[];
  // Colour "tag:NAME" as its own thing. Only the role pipeline has tags, and
  // that string is the one idea in it nobody would guess.
  tags?: boolean;
}

// The dialect a role expression is written in. Declared once, next to the
// colours, because everything that shows one of these - the editor, its
// documentation, the line under the Identity section - has to read it the same.
export const ROLE_SYNTAX: TemplateSyntax = {
  verbs: ['.Keep', '.Drop', 'cutHead', 'cutTail', 'addHead', 'addTail', 'lower', 'upper', 'join', 'json', 'count'],
  tags: true,
};

const ACTION = /\{\{[^}]*\}\}/g;

const action = Decoration.mark({ class: 'tpl-action' });
const field = Decoration.mark({ class: 'tpl-field' });
const variable = Decoration.mark({ class: 'tpl-var' });
const fn = Decoration.mark({ class: 'tpl-fn' });
const str = Decoration.mark({ class: 'tpl-str' });
const tag = Decoration.mark({ class: 'tpl-tag' });

// ONE palette, for the editor and for everything around it - the documentation
// under it, the rendered result. A verb that is orange while typing and grey in
// the explanation two lines below teaches nothing.
export const TPL_COLORS = {
  action: 'var(--mat-sys-tertiary)',
  field: 'var(--mk-signal)',
  var: 'var(--mat-sys-primary)',
  fn: '#f5a97f',
  str: '#a6da95',
  tag: '#eed49f',
} as const;

export type TplKind = keyof typeof TPL_COLORS;

// The palette as CodeMirror wants it, for EditorView.theme(). Declared through
// its own API rather than a global stylesheet: the editor builds its own DOM,
// and this is the door it provides for reaching it.
export const TEMPLATE_COLORS: Record<string, Record<string, string>> = {
  '.tpl-action': { color: TPL_COLORS.action },
  '.tpl-field': { color: TPL_COLORS.field, fontWeight: '600' },
  '.tpl-var': { color: TPL_COLORS.var },
  '.tpl-fn': { color: TPL_COLORS.fn },
  '.tpl-str': { color: TPL_COLORS.str },
  '.tpl-tag': { color: TPL_COLORS.tag, fontWeight: '600' },
};

// One rule per thing worth a colour, in the order they are tried: the first
// that matches at a position wins, so the specific ones come before "any
// string" and "any .Field".
function rulesFor(syntax: TemplateSyntax): { source: string; deco: Decoration; kind: TplKind }[] {
  const methods = syntax.verbs.filter((v) => v.startsWith('.')).map((v) => '\\' + v);
  const words = syntax.verbs.filter((v) => !v.startsWith('.'));
  const rules: { source: string; deco: Decoration; kind: TplKind }[] = [];
  if (methods.length) rules.push({ source: `(?:${methods.join('|')})\\b`, deco: fn, kind: 'fn' });
  if (words.length) rules.push({ source: `\\b(?:${words.join('|')})\\b`, deco: fn, kind: 'fn' });
  if (syntax.tags) rules.push({ source: '"tag:[^"]*"', deco: tag, kind: 'tag' });
  rules.push({ source: '"[^"]*"', deco: str, kind: 'str' });
  rules.push({ source: '\\.[A-Za-z][A-Za-z0-9]*', deco: field, kind: 'field' });
  rules.push({ source: '\\$[A-Za-z][A-Za-z0-9]*', deco: variable, kind: 'var' });
  return rules;
}

// The same reading, outside CodeMirror: a piece of pipeline cut into coloured
// spans. Used by the documentation, where every verb is quoted, and where the
// colours have to be the editor's or they teach the wrong thing. Text with no
// {{ }} around it is read as pipeline anyway - the doc quotes `join ","`, not
// a whole action.
export function tokenizeTemplate(text: string, syntax: TemplateSyntax): { text: string; color?: string }[] {
  const rules = rulesFor(syntax);
  const inner = new RegExp(rules.map((r) => `(${r.source})`).join('|'), 'g');
  const out: { text: string; color?: string }[] = [];
  let at = 0;
  for (const m of text.matchAll(inner)) {
    const from = m.index ?? 0;
    const hit = m.findIndex((g, i) => i > 0 && g !== undefined);
    if (hit < 1) continue;
    if (from > at) out.push({ text: text.slice(at, from) });
    out.push({ text: m[0], color: TPL_COLORS[rules[hit - 1].kind] });
    at = from + m[0].length;
  }
  if (at < text.length) out.push({ text: text.slice(at) });
  return out;
}

function decorate(view: EditorView, rules: { source: string; deco: Decoration }[], inner: RegExp): DecorationSet {
  const text = view.state.doc.toString();
  const ranges: Range<Decoration>[] = [];
  for (const m of text.matchAll(ACTION)) {
    const base = m.index ?? 0;
    ranges.push(action.range(base, base + m[0].length));
    for (const i of m[0].matchAll(inner)) {
      // Which alternative matched: its group is the only one that is set.
      const hit = i.findIndex((g, at) => at > 0 && g !== undefined);
      if (hit < 1) continue;
      const from = base + (i.index ?? 0);
      ranges.push(rules[hit - 1].deco.range(from, from + i[0].length));
    }
  }
  // Sorted by CodeMirror (the `true`): the inner marks nest inside the action
  // that contains them, and hand-sorting two builders was how this first went
  // wrong.
  return Decoration.set(ranges, true);
}

export function templateHighlight(syntax: TemplateSyntax) {
  const rules = rulesFor(syntax);
  // Every source is written with non-capturing groups, so alternative N is
  // capture group N and the mapping above holds.
  const inner = new RegExp(rules.map((r) => `(${r.source})`).join('|'), 'g');
  return ViewPlugin.fromClass(
    class {
      decorations: DecorationSet;
      constructor(view: EditorView) {
        this.decorations = decorate(view, rules, inner);
      }
      update(u: ViewUpdate) {
        if (u.docChanged || u.viewportChanged) this.decorations = decorate(u.view, rules, inner);
      }
    },
    { decorations: (v) => v.decorations },
  );
}
