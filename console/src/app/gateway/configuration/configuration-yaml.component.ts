import {
  Component,
  ElementRef,
  afterNextRender,
  effect,
  input,
  output,
  signal,
  untracked,
  viewChild,
} from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatTooltipModule } from '@angular/material/tooltip';
import { EditorState } from '@codemirror/state';
import { openSearchPanel, search, searchPanelOpen } from '@codemirror/search';
import { unifiedMergeView } from '@codemirror/merge';
import { yaml } from '@codemirror/lang-yaml';
import { oneDark } from '@codemirror/theme-one-dark';
import { basicSetup, EditorView } from 'codemirror';

// A configuration, as the file it is (CFG-01) - read, then edited if asked.
//
// The YAML is the object, not a rendering of it: it is what an export
// downloads and what an import applies, byte for byte, so showing anything
// else here would be showing a second truth. Hence a real editor rather than a
// tree of fields - a configuration is written by hand, in tickets and in git,
// and the console should meet it in the form it travels in.
//
// Read-only until Edit is pressed. Opening a drawer straight into an editable
// document invites a stray keystroke into something that is about to be
// applied to a gateway.
@Component({
  selector: 'app-configuration-yaml',
  imports: [MatButtonModule, MatIconModule, MatTooltipModule],
  styles: [
    `
      :host {
        display: flex;
        flex-direction: column;
        height: 100%;
      }
      header {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 12px 16px;
        border-bottom: 1px solid var(--mat-sys-outline-variant);
      }
      header .grow {
        flex: 1;
        min-width: 0;
      }
      h2 {
        margin: 0;
        font-size: 1.05rem;
        font-weight: 500;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .sub {
        margin: 2px 0 0;
        font-size: 0.8rem;
        color: var(--mat-sys-on-surface-variant);
      }
      .note {
        display: flex;
        align-items: flex-start;
        gap: 10px;
        margin: 0;
        padding: 10px 16px;
        font-size: 0.8rem;
        color: var(--mat-sys-on-surface-variant);
        background: var(--mat-sys-surface-container);
      }
      .note mat-icon {
        flex-shrink: 0;
        font-size: 18px;
        width: 18px;
        height: 18px;
      }
      .editor {
        flex: 1;
        min-height: 0;
        overflow: hidden;
      }
      footer {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 12px 16px;
        border-top: 1px solid var(--mat-sys-outline-variant);
      }
      footer .grow {
        flex: 1;
      }
    `,
  ],
  template: `
    <header>
      <div class="grow">
        <h2>{{ title() }}</h2>
        <p class="sub">
          @if (editing()) {
            <ng-container i18n="@@Editing_the_document">Editing the document</ng-container>
          } @else {
            {{ subtitle() }}
          }
        </p>
      </div>
      <!-- The editor has always had a search (Cmd-F): a keystroke nobody
           announces is a feature nobody has. This is the same panel, opened by
           something visible - and it works without the editor being focused
           first, which is where the keystroke fails. -->
      <button
        matIconButton
        (click)="find()"
        i18n-matTooltip="@@Search_in_the_document"
        matTooltip="Search in the document"
        i18n-aria-label="@@Search_in_the_document"
        aria-label="Search in the document"
      >
        <mat-icon>search</mat-icon>
      </button>
      <button matIconButton (click)="closed.emit()" i18n-aria-label="@@Close" aria-label="Close">
        <mat-icon>close</mat-icon>
      </button>
    </header>

    @if (note(); as text) {
      <p class="note"><mat-icon>image_not_supported</mat-icon>{{ text }}</p>
    }

    <div class="editor" #editor></div>

    <footer>
      <div class="grow"></div>
      @if (editing()) {
        <button matButton (click)="cancel()" i18n="@@Cancel">Cancel</button>
        <button matButton="filled" [disabled]="busy()" (click)="emitSave()">
          <mat-icon>check</mat-icon>
          {{ saveLabel() }}
        </button>
      } @else {
        <button matButton (click)="download.emit()">
          <mat-icon>download</mat-icon>
          <ng-container i18n="@@Export">Export</ng-container>
        </button>
        @if (canEdit()) {
          <button matButton="filled" (click)="editing.set(true)">
            <mat-icon>edit</mat-icon>
            <ng-container i18n="@@Edit">Edit</ng-container>
          </button>
        }
      }
    </footer>
  `,
})
export class ConfigurationYamlComponent {
  readonly title = input('');
  readonly subtitle = input('');
  readonly text = input('');
  readonly canEdit = input(true);
  // What the save button says, because it does not always do the same thing:
  // on a saved configuration it writes a copy, on the current one it changes
  // what the gateway serves.
  readonly saveLabel = input('');
  // What this text does NOT carry, said above it rather than discovered: the
  // pictures are out, and saving does not remove them.
  readonly note = input('');
  // When set, the editor shows what this document CHANGES against that text
  // rather than the document itself. A YAML diff is the honest view of a
  // configuration: it is the form the thing actually travels in, and the one a
  // reviewer already knows how to read.
  readonly against = input('');
  readonly busy = input(false);
  readonly saved = output<string>();
  readonly closed = output<void>();
  readonly download = output<void>();

  protected readonly editing = signal(false);
  private readonly host = viewChild<ElementRef<HTMLDivElement>>('editor');
  private view?: EditorView;

  constructor() {
    afterNextRender(() => this.mount());
    // The document and the mode both rebuild the editor: CodeMirror's
    // read-only state is a compartment-level thing, and swapping the view is
    // both simpler and safe here - the drawer holds one document at a time.
    effect(() => {
      this.text();
      this.editing();
      this.against();
      untracked(() => this.mount());
    });
  }

  private mount(): void {
    const el = this.host()?.nativeElement;
    if (!el) return;
    this.view?.destroy();
    const editing = this.editing();
    const against = this.against();
    this.view = new EditorView({
      doc: this.text(),
      extensions: [
        basicSetup,
        yaml(),
        oneDark,
        // At the TOP: a panel at the foot of a full-height drawer is a panel
        // under the buttons, and one nobody sees open.
        search({ top: true }),
        // A diff is never editable: what would Save mean on a view of two
        // documents at once?
        ...(against && !editing
          ? [unifiedMergeView({ original: against, mergeControls: false, highlightChanges: true })]
          : []),
        // The height is set through CodeMirror's own theme rather than a
        // stylesheet: Angular's view encapsulation does not reach into the
        // editor's DOM, so a `.cm-editor { height }` rule written here would
        // be silently scoped away and the editor would collapse to one line.
        // The search panel is themed here for the same reason - left alone it
        // wears the browser's own inputs, white on a dark drawer.
        EditorView.theme({
          '&': { height: '100%' },
          '.cm-scroller': { overflow: 'auto' },
          '.cm-panels': {
            backgroundColor: 'var(--mat-sys-surface-container-high)',
            color: 'var(--mat-sys-on-surface)',
            borderBottom: '1px solid var(--mat-sys-outline-variant)',
          },
          // One flex row, so the field, the buttons and the checkboxes sit on
          // the same line rather than on three baselines.
          '.cm-panel.cm-search': {
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
            flexWrap: 'wrap',
            padding: '8px 12px',
            fontFamily: 'inherit',
            fontSize: '0.85rem',
          },
          '.cm-panel.cm-search .cm-textfield': {
            flex: '0 1 240px',
            backgroundColor: 'var(--mat-sys-surface-container-highest)',
            color: 'var(--mat-sys-on-surface)',
            border: '1px solid var(--mat-sys-outline-variant)',
            borderRadius: '8px',
            padding: '5px 10px',
            fontFamily: 'inherit',
            margin: '0',
          },
          '.cm-panel.cm-search .cm-textfield:focus-visible': {
            outline: '2px solid var(--mat-sys-primary)',
            outlineOffset: '-1px',
          },
          // CodeMirror's buttons come with a gradient and a system border:
          // next to Material's pills they read as another application.
          '.cm-panel.cm-search .cm-button': {
            backgroundImage: 'none',
            backgroundColor: 'transparent',
            color: 'var(--mat-sys-primary)',
            border: '1px solid var(--mat-sys-outline-variant)',
            borderRadius: '999px',
            padding: '4px 14px',
            fontFamily: 'inherit',
            fontSize: '0.8rem',
            fontWeight: '500',
            cursor: 'pointer',
          },
          '.cm-panel.cm-search .cm-button:hover': {
            backgroundColor: 'color-mix(in srgb, var(--mat-sys-primary) 12%, transparent)',
          },
          '.cm-panel.cm-search label': {
            display: 'inline-flex',
            alignItems: 'center',
            gap: '5px',
            margin: '0',
            color: 'var(--mat-sys-on-surface-variant)',
          },
          '.cm-panel.cm-search label input': { margin: '0', accentColor: 'var(--mat-sys-primary)' },
          // Two controls dropped: "all" selects every match at once, which is
          // an editor's gesture and not a reader's, and "by word" is a
          // refinement nobody reaches for in a configuration file. What stays
          // is find, step through, and the two switches that change what a
          // query MEANS.
          '.cm-panel.cm-search [name=select]': { display: 'none' },
          '.cm-panel.cm-search label:has(input[name=word])': { display: 'none' },
          // The match one is ON stands out by HUE, not by brightness: with a
          // dozen highlights on screen, "the same colour but a bit stronger"
          // is exactly what the eye cannot pick out - which is what stepping
          // through with next/previous is for. So the others are a faint tint
          // of the primary, and the current one is the tertiary, ringed.
          // The class is repeated on purpose. One Dark styles these same
          // elements, both rules weigh the same, and which one wins is then
          // decided by the order two stylesheets happen to be mounted in -
          // which is why the outline below took and the background did not.
          // Repeating a class costs nothing and settles it.
          '.cm-searchMatch.cm-searchMatch': {
            backgroundColor: 'color-mix(in srgb, var(--mat-sys-primary) 22%, transparent)',
            borderRadius: '3px',
          },
          '.cm-searchMatch.cm-searchMatch-selected.cm-searchMatch-selected': {
            backgroundColor: 'color-mix(in srgb, var(--mat-sys-tertiary) 28%, transparent)',
            outline: '2px solid var(--mat-sys-tertiary)',
            // A halo, because the syntax colours underneath cannot be
            // overridden from here - a decoration wraps the token, it does not
            // recolour it - so the mark has to go AROUND the word rather than
            // through it.
            boxShadow: '0 0 0 3px color-mix(in srgb, var(--mat-sys-tertiary) 28%, transparent)',
            borderRadius: '3px',
          },
          // And the LINE it sits on is tinted the same hue. A ring around six
          // characters is hard to find in eighty lines; a band across the line
          // is found without looking for it, and the two together say "here"
          // twice. Free, too: next moves the cursor to the match, so the
          // editor's active line already IS the current match's line.
          '.cm-activeLine.cm-activeLine': {
            backgroundColor: 'color-mix(in srgb, var(--mat-sys-tertiary) 13%, transparent)',
          },
          // The editor's own selection sits on the current match as well (that
          // is how next moves), and left alone it washes the ring out.
          '&.cm-focused .cm-selectionBackground, .cm-selectionBackground': {
            backgroundColor: 'color-mix(in srgb, var(--mat-sys-on-surface) 14%, transparent)',
          },
          // The close cross joins the row instead of floating over its corner.
          '.cm-panel.cm-search [name=close]': {
            position: 'static',
            marginLeft: 'auto',
            padding: '0 4px',
            fontSize: '1.1rem',
            color: 'var(--mat-sys-on-surface-variant)',
            cursor: 'pointer',
          },
        }),
        EditorState.readOnly.of(!editing),
        EditorView.editable.of(editing),
      ],
      parent: el,
    });
  }

  // Opening it twice would toggle nothing and steal the focus back from the
  // field someone is typing in.
  protected find(): void {
    const view = this.view;
    if (!view) return;
    if (!searchPanelOpen(view.state)) openSearchPanel(view);
    view.focus();
  }

  protected cancel(): void {
    this.editing.set(false);
  }

  protected emitSave(): void {
    const text = this.view?.state.doc.toString() ?? '';
    this.editing.set(false);
    this.saved.emit(text);
  }
}
