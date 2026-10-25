import { afterNextRender, Component, computed, DestroyRef, effect, ElementRef, inject, input, model, signal, viewChild } from '@angular/core';
import { type FormValueControl, type ValidationError } from '@angular/forms/signals';
import { json } from '@codemirror/lang-json';
import { EditorState } from '@codemirror/state';
import { EditorView } from '@codemirror/view';
import { basicSetup } from 'codemirror';
import { catppuccin, editorSurface } from './catppuccin';

// A JSON value to type, coloured: the body a notification's action sends.
//
// The same setup as the console's other editors (basicSetup, the shared
// flavour, the console's surface) so a JSON typed here reads like the one the
// JSON tab shows. Bound with [formField]: the form's own check says whether
// the text parses, and its message shows under the box.
@Component({
  selector: 'app-json-editor',
  styles: `
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
      font-size: 0.82rem;
    }
    .editor:focus-within {
      border-color: var(--mat-sys-primary);
    }
    .editor.bad {
      border-color: var(--mat-sys-error);
    }
    .sub {
      min-height: 20px;
      padding: 4px 16px 0;
      font-size: 0.75rem;
      color: var(--mat-sys-on-surface-variant);
    }
    .sub.bad {
      color: var(--mat-sys-error);
    }
  `,
  template: `
    @if (label()) {
      <div class="label">{{ label() }}</div>
    }
    <div class="editor" [class.bad]="message()" #host></div>
    <div class="sub" [class.bad]="message()">{{ message() || hint() }}</div>
  `,
})
export class JsonEditorComponent implements FormValueControl<string> {
  readonly value = model('');
  readonly errors = input<readonly ValidationError.WithOptionalFieldTree[]>([]);
  readonly label = input('');
  readonly hint = input('');
  // A refusal from the server about this value.
  readonly error = input('');

  protected readonly message = computed(() => this.error() || this.errors()[0]?.message || '');

  private readonly host = viewChild.required<ElementRef<HTMLDivElement>>('host');
  private readonly view = signal<EditorView | undefined>(undefined);

  constructor() {
    inject(DestroyRef).onDestroy(() => this.view()?.destroy());
    afterNextRender(() => {
      this.view.set(new EditorView({
        state: EditorState.create({
          doc: this.value(),
          extensions: [
            basicSetup,
            catppuccin,
            editorSurface,
            json(),
            EditorView.lineWrapping,
            EditorView.theme({
              '&': { maxHeight: '240px' },
              '.cm-scroller': { overflow: 'auto' },
              '.cm-content': { minHeight: '88px' },
            }),
            EditorView.updateListener.of((u) => {
              if (u.docChanged) this.value.set(u.state.doc.toString());
            }),
          ],
        }),
        parent: this.host().nativeElement,
      }));
    });
    // A value set from outside - another action's, when one above it was
    // removed - replaces the text.
    effect(() => {
      const v = this.value();
      const view = this.view();
      if (view && view.state.doc.toString() !== v) {
        view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: v } });
      }
    });
  }
}
