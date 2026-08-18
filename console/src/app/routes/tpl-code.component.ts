import { Component, computed, input } from '@angular/core';
import { TemplateSyntax, tokenizeTemplate } from './template-highlight';

// A piece of template, coloured like the editor colours it.
//
// The documentation under an editor quotes the syntax it explains. Quoting it
// in flat grey while the editor paints it in five colours makes the reader
// translate between two renderings of the same thing; painting it the same way
// makes the explanation and the box above it one lesson.
//
// Spans, not innerHTML: what is coloured includes catalogue data (tag names),
// and a template that never builds markup cannot be talked into building any.
@Component({
  selector: 'code[tpl]',
  template: `@for (t of parts(); track $index) {<span [style.color]="t.color">{{ t.text }}</span>}`,
  styles: [
    `
      :host {
        font-family: var(--mk-mono);
        background: var(--mat-sys-surface-container-high);
        border-radius: 4px;
        padding: 1px 5px;
      }
    `,
  ],
})
export class TplCodeComponent {
  readonly tpl = input.required<string>();
  readonly syntax = input.required<TemplateSyntax>();
  protected readonly parts = computed(() => tokenizeTemplate(this.tpl(), this.syntax()));
}
