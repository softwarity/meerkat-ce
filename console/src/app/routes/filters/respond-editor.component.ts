import { afterNextRender, Component, ElementRef, inject, input, model, output, signal, viewChild } from '@angular/core';
import { MatIconModule } from '@angular/material/icon';
import { MatTooltipModule } from '@angular/material/tooltip';
import { EditorView } from '@codemirror/view';
import { EditorState } from '@codemirror/state';
import { oneDark } from '@codemirror/theme-one-dark';
import { basicSetup } from 'codemirror';
import { ApiService } from '../../api.service';
import { TEMPLATE_COLORS, templateHighlight } from '../template-highlight';

// The template editor: colours, and an answer under the cursor.
//
// A plain <textarea> was not enough, and the reason is not decoration. Go
// template syntax is not guessable - {{if $i}},{{end}} to place a comma reads
// like line noise until someone explains it - so the editor has to do the
// explaining: actions stand out from the literal text, and the panel below
// shows what an application would actually receive, rendered by the gateway
// itself for a witness caller. Someone who cannot remember the syntax can
// still tell whether they got it right, which is the part that matters.

// The verbs a respond body may call. The colouring itself is shared with the
// role pipeline (see template-highlight): same dialect, same colours, because
// a product whose two code editors highlight differently reads as two.
const highlight = templateHighlight({ verbs: ['json', 'join', 'wrap', 'range', 'if', 'else', 'end', 'with'] });

@Component({
  selector: 'app-respond-editor',
  imports: [MatIconModule, MatTooltipModule],
  styles: [
    `
      :host {
        display: block;
      }
      .label {
        font-size: 0.75rem;
        color: var(--mat-sys-on-surface-variant);
        margin-bottom: 4px;
      }
      .editor {
        border: 1px solid var(--mat-sys-outline-variant);
        border-radius: 8px;
        overflow: hidden;
      }
      .editor:focus-within {
        border-color: var(--mat-sys-primary);
      }
      .cm-editor {
        background: var(--mat-sys-surface-container-lowest);
        font-family: var(--mk-mono);
        font-size: 0.82rem;
      }
      .cm-editor .cm-content {
        caret-color: var(--mat-sys-primary);
      }
      .cm-editor .cm-gutters {
        background: transparent;
        border: 0;
        color: var(--mat-sys-outline);
      }
      .cm-editor .cm-activeLine {
        background: color-mix(in srgb, var(--mat-sys-primary) 6%, transparent);
      }
      .out {
        margin-top: 8px;
        border-radius: 8px;
        padding: 8px 10px;
        background: var(--mat-sys-surface-container);
      }
      /* The rendered answer, in a <pre> of its own: it used to sit directly in
         a pre-wrap block, which printed the template's own indentation. */
      .out-body {
        margin: 0;
        font-family: var(--mk-mono);
        font-size: 0.78rem;
        line-height: 1.5;
        white-space: pre-wrap;
        word-break: break-word;
      }
      .out.err {
        background: color-mix(in srgb, var(--mat-sys-error) 12%, transparent);
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
      mat-icon {
        font-size: 15px;
        width: 15px;
        height: 15px;
      }
    `,
  ],
  template: `
    <div class="label">Template</div>
    <div class="editor" #host></div>
    @if (error(); as e) {
      <div class="out err">
        <div class="out-head"><mat-icon>error_outline</mat-icon><span>This template cannot be saved</span></div>
        <pre class="out-body">{{ e }}</pre>
      </div>
    } @else if (output(); as o) {
      <div class="out">
        <div class="out-head">
          <mat-icon>play_arrow</mat-icon><span>What an application receives, for a caller named {{ callerName }}</span>
        </div>
        <pre class="out-body">{{ o }}</pre>
      </div>
    }
  `,
})
export class RespondEditorComponent {
  readonly value = model<string>('');
  readonly changed = output<string>();
  private readonly api = inject(ApiService);
  private readonly host = viewChild.required<ElementRef<HTMLDivElement>>('host');

  protected readonly output = signal('');
  protected readonly error = signal('');
  // Kept in step with routing.SampleIdentity.
  protected readonly callerName = 'john';

  private view?: EditorView;
  private timer?: ReturnType<typeof setTimeout>;

  constructor() {
    afterNextRender(() => {
      this.view = new EditorView({
        state: EditorState.create({
          doc: this.value(),
          extensions: [
            // The same pair as the Add CSS / Add JavaScript dialog: one library
            // is not enough, it has to be one SETUP too, or the two editors of
            // the same product show different gutters.
            basicSetup,
            oneDark,
            highlight,
            EditorView.lineWrapping,
            EditorView.theme({
              '&': { maxHeight: '280px' },
              ...TEMPLATE_COLORS,
              '.cm-scroller': { overflow: 'auto' },
              // A floor, plus the blank lines a new template opens with: an
              // editor showing one line inside an empty box reads as broken.
              // Those lines are trimmed when the route is saved, so they never
              // reach the answer.
              '.cm-content': { minHeight: '120px' },
            }),
            EditorView.updateListener.of((u) => {
              if (!u.docChanged) return;
              const text = u.state.doc.toString();
              this.value.set(text);
              this.changed.emit(text);
              this.schedulePreview(text);
            }),
          ],
        }),
        parent: this.host().nativeElement,
      });
      this.schedulePreview(this.value());
    });
  }

  // Debounced: the preview follows typing, it does not race it.
  private schedulePreview(body: string): void {
    clearTimeout(this.timer);
    if (!body.trim()) {
      this.output.set('');
      this.error.set('');
      return;
    }
    this.timer = setTimeout(() => {
      this.api.respondPreview(body).subscribe({
        next: (r) => {
          this.error.set(r.error ?? '');
          this.output.set(r.output ?? '');
        },
        error: () => undefined, // a preview that cannot be fetched says nothing
      });
    }, 400);
  }
}
