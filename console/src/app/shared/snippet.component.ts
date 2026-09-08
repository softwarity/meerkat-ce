import { AfterViewInit, Component, ElementRef, OnDestroy, effect, inject, input, viewChild } from '@angular/core';
import { yaml } from '@codemirror/lang-yaml';
import { EditorState, RangeSetBuilder } from '@codemirror/state';
import { Decoration, DecorationSet, EditorView, ViewPlugin, ViewUpdate } from '@codemirror/view';
import { catppuccin, editorSurface } from './catppuccin';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';

// A configuration file, shown with the two things somebody does with one.
//
// Copy AND download, because they are not the same act: copying is for the
// three lines you paste into a file you already have, downloading is for the
// file you do not - and telling somebody to select twenty lines of YAML in a
// scrolling box with their mouse is how a good example goes unused.
//
// The filename is not decoration either: `prometheus.yml` and `servicemonitor.yaml`
// have to land under those names, and a browser's "download.txt" makes the
// reader do the one step the example was meant to save.
@Component({
  selector: 'app-snippet',
  imports: [MatButtonModule, MatIconModule, MatTooltipModule],
  template: `
    <div class="head">
      <code class="file">{{ filename() }}</code>
      <span class="grow"></span>
      <button
        matIconButton
        type="button"
        (click)="copy()"
        i18n-matTooltip="@@Copy"
        matTooltip="Copy"
        i18n-aria-label="@@Copy"
        aria-label="Copy"
      >
        <mat-icon>content_copy</mat-icon>
      </button>
      @if (variants().length === 0) {
        <button
          matIconButton
          type="button"
          (click)="save(content(), downloadAs() || filename())"
          i18n-matTooltip="@@Download"
          matTooltip="Download"
          i18n-aria-label="@@Download"
          aria-label="Download"
        >
          <mat-icon>download</mat-icon>
        </button>
      } @else {
        @for (v of variants(); track v.label) {
          <button matButton type="button" (click)="save(v.content, v.filename)">
            <mat-icon>download</mat-icon>
            {{ v.label }}
          </button>
        }
      }
    </div>
    <div #host class="code"></div>
  `,
  styles: `
    :host {
      display: block;
      border: 1px solid var(--mat-sys-outline-variant);
      border-radius: 10px;
      overflow: hidden;
      margin-bottom: 12px;
    }
    .head {
      display: flex;
      align-items: center;
      gap: 4px;
      padding: 2px 4px 2px 12px;
      background: var(--mat-sys-surface-container);
    }
    .file {
      font-size: 0.75rem;
      color: var(--mat-sys-on-surface-variant);
    }
    .grow {
      flex: 1;
    }
    /* Long lines WRAP rather than scroll sideways: a drawer is narrow, and a
       panel with its own horizontal scrollbar is a panel whose right-hand half
       nobody reads. It costs nothing where it would matter - the copy button
       hands over the source, not what the box drew. */
    .code {
      font-size: 0.74rem;
    }
  `,
})
export class SnippetComponent implements AfterViewInit, OnDestroy {
  readonly filename = input.required<string>();
  // What the SAVED file is called, when that differs from what the box is
  // labelled. A compose file is read as docker-compose.yml - that is the name
  // that says what it is - and saved as something that will not collide with
  // the one already in the directory it lands in.
  readonly downloadAs = input('');
  readonly content = input.required<string>();
  // The file is one thing to read and several to save.
  //
  // A configuration that differs between two platforms by six lines is ONE
  // file with both of them in it, commented: the reader sees the whole thing
  // and what the choice actually costs, instead of comparing two panels. What
  // they carry away is a file for their own platform, which is what these
  // buttons are - a download each, rather than a download of what is drawn.
  readonly variants = input<{ label: string; filename: string; content: string }[]>([]);

  private readonly snack = inject(MatSnackBar);
  private readonly host = viewChild.required<ElementRef<HTMLElement>>('host');
  private view?: EditorView;

  constructor() {
    // The text can change under the panel - the examples carry this
    // installation's own address, resolved after the setting arrives - so the
    // document is replaced rather than the editor rebuilt.
    effect(() => {
      const text = this.content();
      this.view?.dispatch({ changes: { from: 0, to: this.view.state.doc.length, insert: text } });
    });
  }

  ngAfterViewInit(): void {
    this.view = new EditorView({
      doc: this.content(),
      parent: this.host().nativeElement,
      extensions: [
        // The SAME colours as the configuration editor two screens away: a
        // product whose YAML is one palette here and another there reads as
        // two products, which is the argument template-highlight.ts already
        // makes for its own.
        catppuccin,
        editorSurface,
        ...this.language(),
        // Read-only, and no basicSetup: this is a thing to copy, not a thing
        // to edit. Line numbers, a gutter and an active-line highlight would
        // be an editor's furniture around a paragraph.
        EditorState.readOnly.of(true),
        EditorView.editable.of(false),
        EditorView.lineWrapping,
        // Themed here rather than in the stylesheet above: Angular's view
        // encapsulation does not reach into the editor's DOM, so a
        // `.cm-editor` rule written there is silently scoped away.
        EditorView.theme({
          '.cm-content': { padding: '10px 12px', fontFamily: 'var(--mk-mono, ui-monospace, monospace)' },
          '.cm-line': { padding: '0' },
          '&.cm-focused': { outline: 'none' },
          '.cm-scroller': { lineHeight: '1.5' },
        }),
      ],
    });
  }

  ngOnDestroy(): void {
    this.view?.destroy();
  }

  // By extension. Only YAML has a grammar installed: the rest - shell, PromQL,
  // the dashboard's JSON - is read rather than edited, and a parser apiece
  // would be three dependencies for colour nothing depends on.
  private language() {
    return /\.ya?ml$/.test(this.filename()) ? [yaml()] : [hashComments];
  }

  protected copy(): void {
    void navigator.clipboard?.writeText(this.content());
    this.snack.open($localize`:@@Copied:Copied`, undefined, { duration: 1500 });
  }

  protected save(content: string, name: string): void {
    const url = URL.createObjectURL(new Blob([content], { type: 'text/plain' }));
    const a = document.createElement('a');
    a.href = url;
    a.download = name;
    a.click();
    // Released on the next turn: revoking it before the click has been acted
    // on cancels the download in some browsers.
    setTimeout(() => URL.revokeObjectURL(url), 0);
  }
}

// Shell and PromQL both comment with #, and neither has a language package
// installed. One regex covers the two, which is what this file needs: the
// snippets that are not YAML are mostly explanation, and a comment that reads
// as code is the one thing worth fixing about them.
//
// Regex and not a parser, deliberately - the same call template-highlight.ts
// makes, and for the same reason: this is colour, and nothing depends on it.
const comment = Decoration.mark({ class: 'cm-comment' });

const hashComments = ViewPlugin.fromClass(
  class {
    decorations: DecorationSet;
    constructor(view: EditorView) {
      this.decorations = build(view);
    }
    update(u: ViewUpdate) {
      if (u.docChanged || u.viewportChanged) this.decorations = build(u.view);
    }
  },
  { decorations: (v) => v.decorations },
);

function build(view: EditorView): DecorationSet {
  const b = new RangeSetBuilder<Decoration>();
  for (let n = 1; n <= view.state.doc.lines; n++) {
    const line = view.state.doc.line(n);
    const at = line.text.indexOf('#');
    if (at >= 0) b.add(line.from + at, line.to, comment);
  }
  return b.finish();
}
