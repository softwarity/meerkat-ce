import { afterNextRender, Component, computed, ElementRef, inject, signal, viewChild } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MAT_DIALOG_DATA, MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatMenuModule } from '@angular/material/menu';
import { EditorState } from '@codemirror/state';
import { oneDark } from '@codemirror/theme-one-dark';
import { EditorView } from '@codemirror/view';
import { basicSetup } from 'codemirror';
import { Role } from '../api.service';
import { ROLE_SYNTAX, TEMPLATE_COLORS, templateHighlight, TPL_COLORS } from './template-highlight';
import { TplCodeComponent } from './tpl-code.component';

export interface RoleExprData {
  expr: string;
  roles: Role[];
  // What the OTHER routes already send. Most estates want the same shape on
  // most of their services, and writing it 150 times is not a design: take the
  // one the service next door uses, change the tag, done.
  others?: { name: string; expr: string }[];
}

// Shapes to start from. Each one narrows AND rewrites, because that is what a
// real one does - an example that only filters leaves the reader to invent the
// half that saves the bytes. TAGNAME is a hole on purpose: it lands SELECTED in
// the editor, so the next keystroke replaces it.
const SHAPES: { label: string; pipe: string }[] = [
  { label: 'One tag', pipe: '.Keep "tag:TAGNAME" | cutHead "ROLE_"' },
  { label: 'Two tags', pipe: '.Keep "tag:TAGNAME" "tag:OTHERTAG" | cutHead "ROLE_"' },
  { label: 'A tag plus named roles', pipe: '.Keep "tag:TAGNAME" "ROLE_USER" "ROLE_ADMIN" | cutHead "ROLE_"' },
  { label: 'Everything except one tag', pipe: '.Drop "tag:TAGNAME" | cutHead "ROLE_"' },
  { label: "Under this service's own head", pipe: '.Keep "tag:TAGNAME" | cutHead "ROLE_" | addHead "app:"' },
  { label: 'Everything, untouched', pipe: '' },
];

// The same shapes, twice: what changes between an upstream reading a header and
// one parsing a body is the last verb, and nothing else.
function shapes(tail: string): { label: string; expr: string }[] {
  return SHAPES.map((s) => ({ label: s.label, expr: `{{.Roles | ${s.pipe ? s.pipe + ' | ' : ''}${tail}}}` }));
}

const CSV_SHAPES = shapes('join ","');
const JSON_SHAPES = shapes('json');

const HOLE = 'TAGNAME';

const highlight = templateHighlight(ROLE_SYNTAX);

// What a route sends as roles, written as a pipeline.
//
// It replaced three switches (a JSON toggle, a prefix to trim, a tag filter)
// because every estate wants a different shape and no set of switches was ever
// going to cover them. The weight is the point: 320 roles is 9 KB of header on
// every request for a service that uses three, and some servers refuse a header
// that size outright.
//
// The dialog carries its own documentation: the syntax is not guessable, and
// `tag:` in particular is ours - nothing outside Meerkat would suggest it.
@Component({
  selector: 'app-role-expr-dialog',
  imports: [MatButtonModule, MatDialogModule, MatIconModule, MatMenuModule, TplCodeComponent],
  styles: [
    `
      :host {
        display: block;
      }
      /* The expression on the left, what the verbs mean on the right. The
         documentation is what made this dialog tall - beside the editor it
         costs nothing, and it is READ while typing rather than scrolled to. */
      .cols {
        display: grid;
        grid-template-columns: minmax(0, 1.1fr) minmax(0, 1fr);
        gap: 22px;
      }
      @media (max-width: 900px) {
        .cols {
          grid-template-columns: minmax(0, 1fr);
        }
      }
      .bar {
        display: flex;
        align-items: center;
        justify-content: flex-end;
        margin-bottom: 6px;
      }
      .menu-head {
        padding: 8px 16px 2px;
        font-size: 0.72rem;
        letter-spacing: 0.04em;
        text-transform: uppercase;
        color: var(--mat-sys-on-surface-variant);
      }
      .opt-label {
        display: block;
        line-height: 1.35;
      }
      .opt-expr {
        display: block;
        font-family: var(--mk-mono);
        font-size: 0.72rem;
        line-height: 1.35;
        color: var(--mat-sys-on-surface-variant);
      }
      .tags {
        margin-top: 4px;
      }
      .editor {
        border: 1px solid var(--mat-sys-outline-variant);
        border-radius: 8px;
        overflow: hidden;
      }
      .editor:focus-within {
        border-color: var(--mat-sys-primary);
      }
      .out {
        margin-top: 10px;
        border-radius: 8px;
        padding: 8px 10px;
        background: var(--mat-sys-surface-container);
      }
      .out.err {
        background: color-mix(in srgb, var(--mat-sys-error) 12%, transparent);
        color: var(--mat-sys-error);
      }
      /* Over the ceiling: the expression still saves, the requests will not. */
      .out.heavy {
        background: color-mix(in srgb, var(--mat-sys-error) 12%, transparent);
      }
      .limit {
        display: flex;
        align-items: flex-start;
        gap: 6px;
        margin-top: 8px;
        font-size: 0.74rem;
        line-height: 1.45;
        color: var(--mat-sys-on-surface-variant);
      }
      .out.heavy .limit {
        color: var(--mat-sys-error);
      }
      .out-head {
        display: flex;
        align-items: center;
        gap: 6px;
        font-size: 0.74rem;
        color: var(--mat-sys-on-surface-variant);
        margin-bottom: 5px;
      }
      .out.err .out-head {
        color: var(--mat-sys-error);
      }
      .out-body {
        margin: 0;
        font-family: var(--mk-mono);
        font-size: 0.78rem;
        line-height: 1.5;
        white-space: pre-wrap;
        word-break: break-word;
      }
      .weight {
        font-weight: 600;
      }
      dl {
        margin: 0;
        font-size: 0.8rem;
        line-height: 1.55;
        color: var(--mat-sys-on-surface-variant);
      }
      dt {
        margin-top: 7px;
      }
      dd {
        margin: 0 0 0 14px;
      }
      code {
        font-family: var(--mk-mono);
        background: var(--mat-sys-surface-container-high);
        border-radius: 4px;
        padding: 1px 5px;
      }
      mat-icon {
        font-size: 15px;
        width: 15px;
        height: 15px;
      }
    `,
  ],
  template: `
    <h2 mat-dialog-title>Roles sent to this service</h2>
    <mat-dialog-content>
      <div class="cols">
        <div>
          <!-- Nobody writes this from scratch on 150 routes: start from a
               common shape, or from what the service next door already sends. -->
          <div class="bar">
            <button matButton [matMenuTriggerFor]="starters">
              <mat-icon>content_copy</mat-icon>
              Start from
            </button>
            <mat-menu #starters="matMenu">
              <button mat-menu-item [matMenuTriggerFor]="csv">Comma-separated</button>
              <button mat-menu-item [matMenuTriggerFor]="jsonArray">JSON array</button>
              @if (reuse().length) {
                <div class="menu-head">Already sent by</div>
                @for (r of reuse(); track r.expr) {
                  <button mat-menu-item (click)="use(r.expr)">
                    <span class="opt-label">{{ r.label }}</span>
                    <span class="opt-expr">{{ r.expr }}</span>
                  </button>
                }
              }
            </mat-menu>
            <mat-menu #csv="matMenu">
              @for (p of CSV_SHAPES; track p.expr) {
                <button mat-menu-item (click)="use(p.expr)">
                  <span class="opt-label">{{ p.label }}</span>
                  <span class="opt-expr">{{ p.expr }}</span>
                </button>
              }
            </mat-menu>
            <mat-menu #jsonArray="matMenu">
              @for (p of JSON_SHAPES; track p.expr) {
                <button mat-menu-item (click)="use(p.expr)">
                  <span class="opt-label">{{ p.label }}</span>
                  <span class="opt-expr">{{ p.expr }}</span>
                </button>
              }
            </mat-menu>
          </div>

          <div class="editor" #host></div>

          @if (error(); as e) {
            <div class="out err">
              <div class="out-head"><mat-icon>error_outline</mat-icon><span>This expression cannot be saved</span></div>
              <pre class="out-body">{{ e }}</pre>
            </div>
          } @else {
            <div class="out" [class.heavy]="bytes() >= LIMIT">
              <div class="out-head">
                <mat-icon>play_arrow</mat-icon>
                <span>
                  What this route sends every caller of this service, on EVERY request:
                  <span class="weight">{{ count() }} roles, {{ bytes() }} bytes</span> of header
                </span>
              </div>
              <!-- The answer, in the editor's own colours: what came from the
                   catalogue reads as data, what the expression wrote around it
                   reads as the literal it was typed as. -->
              <pre class="out-body">@for (t of sampleParts(); track $index) {<span [style.color]="t.color">{{ t.text }}</span>}</pre>
              <!-- The number above means nothing without what it is measured
                   against: 8 KB is where the usual servers stop, and a request
                   that crosses it is refused before the application sees it. -->
              <div class="limit">
                @if (bytes() >= LIMIT) {
                  <mat-icon>warning</mat-icon>
                  <span>
                    Past <strong>8 KB</strong>, which is the default ceiling for one header on Apache
                    (LimitRequestFieldSize), Tomcat (maxHttpHeaderSize) and nginx
                    (large_client_header_buffers). Requests will be refused with a 431 or a 400 before the
                    application sees them. Narrow it down.
                  </span>
                } @else {
                  <span>
                    8 KB is the default ceiling for one header on Apache, Tomcat and nginx, and this
                    travels on every request - not only the first.
                  </span>
                }
              </div>
            </div>
          }
        </div>

        <dl>
          <dt><code tpl=".Roles" [syntax]="SYNTAX"></code></dt>
          <dd>the roles this caller holds, before anything is done to them.</dd>

          <dt><code [tpl]="KEEP" [syntax]="SYNTAX"></code> and <code [tpl]="DROP" [syntax]="SYNTAX"></code></dt>
          <dd>
            keep (or remove) what a selector names. <code tpl="&quot;tag:BILLING&quot;" [syntax]="SYNTAX"></code> is a
            <strong>catalogue tag</strong> - Meerkat's own idea, and how a whole family of roles travels as one
            word; anything else is a role name. <strong>Every selector listed counts</strong>, so two tags, or a
            tag and a few named roles, is one call.
            @if (tags().length) {
              <div class="tags">In this catalogue: <code [style.color]="TAG_COLOR">{{ tags().join(', ') }}</code></div>
            }
          </dd>

          <dt>
            <code tpl="cutHead &quot;ROLE_&quot;" [syntax]="SYNTAX"></code>,
            <code tpl="cutTail &quot;_V2&quot;" [syntax]="SYNTAX"></code>
          </dt>
          <dd>remove that head or tail from each name. A name that does not carry it is left alone.</dd>

          <dt>
            <code tpl="addHead &quot;app:&quot;" [syntax]="SYNTAX"></code>,
            <code tpl="addTail &quot;:ro&quot;" [syntax]="SYNTAX"></code>
          </dt>
          <dd>the opposite, for a service whose namespace differs from the catalogue's.</dd>

          <dt><code tpl="lower" [syntax]="SYNTAX"></code>, <code tpl="upper" [syntax]="SYNTAX"></code></dt>
          <dd>case, for a service that expects one.</dd>

          <dt><code tpl="join &quot;,&quot;" [syntax]="SYNTAX"></code>, <code tpl="json" [syntax]="SYNTAX"></code></dt>
          <dd>
            how the list is written: separated by something, or a JSON array. One of the two ends the
            pipeline. Leaving it out sends Go's own rendering, which no service expects.
          </dd>
        </dl>
      </div>
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button matButton (click)="ref.close()">Cancel</button>
      <button matButton="tonal" [disabled]="!!error()" (click)="apply()">Apply</button>
    </mat-dialog-actions>
  `,
})
export class RoleExprDialogComponent {
  protected readonly data = inject<RoleExprData>(MAT_DIALOG_DATA);
  protected readonly ref = inject(MatDialogRef<RoleExprDialogComponent, string | undefined>);
  private readonly host = viewChild.required<ElementRef<HTMLDivElement>>('host');

  protected readonly error = signal('');
  protected readonly sample = signal('');
  protected readonly count = signal(0);
  protected readonly bytes = signal(0);
  // Written here rather than inline: Angular reads { and } as control flow.
  protected readonly KEEP = '.Keep "tag:BILLING" "ROLE_USER"';
  protected readonly DROP = '.Drop "tag:OTHER"';
  protected readonly CSV_SHAPES = CSV_SHAPES;
  protected readonly JSON_SHAPES = JSON_SHAPES;
  protected readonly SYNTAX = ROLE_SYNTAX;
  protected readonly TAG_COLOR = TPL_COLORS.tag;
  // Where the usual servers stop for ONE header: Apache LimitRequestFieldSize,
  // Tomcat maxHttpHeaderSize and nginx large_client_header_buffers all default
  // to 8192 bytes. Past it the request dies at the door, not in the code.
  protected readonly LIMIT = 8192;

  // The rendered answer, cut the way the editor cuts an expression: the names
  // that came from the catalogue in the colour of .Roles, the punctuation the
  // expression wrote around them in the colour of the literal it was typed as.
  protected readonly sampleParts = computed(() => splitResult(this.sample()));

  // The tags this catalogue actually carries. Without them `tag:` is a syntax
  // with nothing to put in it: nobody remembers thirty tag names.
  protected readonly tags = signal<string[]>([]);

  // What the other routes already send, one entry per DISTINCT expression: on
  // an estate that shapes its roles the same way everywhere, the same line
  // repeated forty times is one choice, not forty.
  protected readonly reuse = signal<{ expr: string; label: string }[]>([]);

  private view?: EditorView;

  constructor() {
    this.tags.set([...new Set(this.data.roles.flatMap((r) => r.tags ?? []))].sort());
    this.reuse.set(groupByExpr(this.data.others ?? [], this.data.expr));

    afterNextRender(() => {
      this.view = new EditorView({
        state: EditorState.create({
          doc: this.data.expr || '{{join "," .Roles}}',
          extensions: [
            basicSetup,
            oneDark,
            highlight,
            EditorView.lineWrapping,
            EditorView.theme({ '&': { maxHeight: '160px' }, ...TEMPLATE_COLORS, '.cm-content': { minHeight: '60px' } }),
            EditorView.updateListener.of((u) => {
              if (u.docChanged) this.evaluate(u.state.doc.toString());
            }),
          ],
        }),
        parent: this.host().nativeElement,
      });
      this.evaluate(this.view.state.doc.toString());
    });
  }

  // Take a shape as the new expression, with its hole selected so the tag name
  // is the first thing typed.
  protected use(expr: string): void {
    const v = this.view;
    if (!v) return;
    const hole = expr.indexOf(HOLE);
    v.dispatch({
      changes: { from: 0, to: v.state.doc.length, insert: expr },
      selection: hole < 0 ? { anchor: expr.length } : { anchor: hole, head: hole + HOLE.length },
    });
    v.focus();
  }

  // Evaluated in the browser, on the real catalogue. It mirrors the gateway's
  // verbs rather than calling it: this is a preview of a shape, not of a
  // decision, and the round trip on every keystroke would be worse than the
  // duplication. What CANNOT be judged here - a broken expression - is refused
  // by the server on save anyway.
  private evaluate(expr: string): void {
    try {
      const names = this.data.roles.map((r) => r.name);
      const tagsOf = new Map(this.data.roles.map((r) => [r.name, r.tags ?? []]));
      const out = runRoleExpr(expr, names, tagsOf);
      this.error.set('');
      this.sample.set(out.text.length > 400 ? out.text.slice(0, 400) + '...' : out.text);
      this.count.set(out.count);
      this.bytes.set(out.text.length);
    } catch (e) {
      this.error.set(e instanceof Error ? e.message : String(e));
    }
  }

  protected apply(): void {
    this.ref.close(this.view?.state.doc.toString() ?? '');
  }
}

// A role name as it comes out: letters, digits, and the punctuation a head or
// tail may carry. Everything BETWEEN two of them was written by the expression
// - the separator, the brackets, the quotes of a JSON array - which is exactly
// the split worth colouring: what the catalogue holds, and what costs bytes
// around it.
const RESULT_ITEM = /[A-Za-z0-9_.:-]+/g;

function splitResult(text: string): { text: string; color: string }[] {
  const out: { text: string; color: string }[] = [];
  let at = 0;
  for (const m of text.matchAll(RESULT_ITEM)) {
    const from = m.index ?? 0;
    if (from > at) out.push({ text: text.slice(at, from), color: TPL_COLORS.str });
    out.push({ text: m[0], color: TPL_COLORS.field });
    at = from + m[0].length;
  }
  if (at < text.length) out.push({ text: text.slice(at), color: TPL_COLORS.str });
  return out;
}

// The other routes' expressions, one entry per distinct one, labelled by who
// sends it. Past three names the list becomes noise, so it counts instead. The
// current expression is left out: offering someone what they already have is a
// menu entry that does nothing.
function groupByExpr(others: { name: string; expr: string }[], current: string): { expr: string; label: string }[] {
  const byExpr = new Map<string, string[]>();
  for (const o of others) {
    const expr = o.expr.trim();
    if (!expr || expr === current.trim()) continue;
    byExpr.set(expr, [...(byExpr.get(expr) ?? []), o.name]);
  }
  return [...byExpr]
    .sort((a, b) => b[1].length - a[1].length)
    .map(([expr, names]) => ({
      expr,
      label: names.length > 3 ? `${names.slice(0, 3).join(', ')} +${names.length - 3}` : names.join(', '),
    }));
}

// A reader for the same pipeline the gateway runs. Deliberately narrow: it
// understands the verbs and nothing else, so anything it cannot read is left
// for the server to judge on save.
export function runRoleExpr(
  expr: string,
  roles: string[],
  tagsOf: Map<string, string[]>,
): { text: string; count: number } {
  const body = expr.trim().replace(/^\{\{/, '').replace(/\}\}$/, '').trim();
  if (!body) return { text: roles.join(','), count: roles.length };

  let list = [...roles];
  let text: string | null = null;

  for (const rawStage of splitPipeline(body)) {
    const stage = rawStage.trim();
    if (!stage || stage === '.Roles') continue;
    const [verb, ...rest] = tokenize(stage);
    const args = rest.map(unquote);
    switch (verb) {
      case '.Keep':
      case '.Drop': {
        const wanted = new Set(args.filter((a) => !a.startsWith('tag:')));
        const tags = new Set(args.filter((a) => a.startsWith('tag:')).map((a) => a.slice(4)));
        const hit = (r: string) => wanted.has(r) || (tagsOf.get(r) ?? []).some((t) => tags.has(t));
        list = list.filter((r) => (verb === '.Keep' ? hit(r) : !hit(r)));
        break;
      }
      case 'cutHead':
        list = list.map((r) => (r.startsWith(args[0]) ? r.slice(args[0].length) : r));
        break;
      case 'cutTail':
        list = list.map((r) => (r.endsWith(args[0]) ? r.slice(0, -args[0].length) : r));
        break;
      case 'addHead':
        list = list.map((r) => args[0] + r);
        break;
      case 'addTail':
        list = list.map((r) => r + args[0]);
        break;
      case 'lower':
        list = list.map((r) => r.toLowerCase());
        break;
      case 'upper':
        list = list.map((r) => r.toUpperCase());
        break;
      case 'join':
        text = list.join(args[0] ?? ',');
        break;
      case 'json':
        text = JSON.stringify(list);
        break;
      default:
        throw new Error(`unknown verb ${verb}`);
    }
  }
  return { text: text ?? list.join(','), count: list.length };
}

// The pipeline's stages, splitting on | outside quotes.
function splitPipeline(body: string): string[] {
  const out: string[] = [];
  let current = '';
  let quoted = false;
  for (const ch of body) {
    if (ch === '"') quoted = !quoted;
    if (ch === '|' && !quoted) {
      out.push(current);
      current = '';
      continue;
    }
    current += ch;
  }
  out.push(current);
  return out;
}

function tokenize(stage: string): string[] {
  return stage.match(/"[^"]*"|\S+/g) ?? [];
}

function unquote(token: string): string {
  return token.startsWith('"') && token.endsWith('"') ? token.slice(1, -1) : token;
}
