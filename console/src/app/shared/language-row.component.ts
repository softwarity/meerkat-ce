import { Component, computed, input } from '@angular/core';
import { LocaleView } from '../api.service';
import { languageName } from './language-name';

// One language, named the way a language has to be named here: its tag, its own
// name, and English.
//
// The endonym alone is a row nobody can pick - a list of twenty going from
// العربية to 日本語 is scrolled twice and abandoned - and English alone would
// name a French page in a language the page does not speak. English is dropped
// where it would only repeat the endonym, which is every language whose own
// name we already carry in English.
//
// One component because the two lists that show languages, the picker's menu
// and the "start from" of the add dialog, ARE the same list: one to read a
// language, one to copy it.
@Component({
  selector: 'app-language-row',
  template: `
    <b>{{ locale().code }}</b>
    <span class="own">{{ locale().name }}</span>
    @if (english(); as en) {
      <span class="en">{{ en }}</span>
    }
    @if (showHoles() && locale().holes) {
      <span class="holes">{{ locale().holes }}</span>
    }
  `,
  styles: [
    `
      :host {
        display: inline-flex;
        align-items: center;
        gap: 0.5rem;
        min-width: 0;
      }
      .own {
        color: var(--mat-sys-on-surface-variant);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      /* Fainter than the endonym: English is here to help find the language,
         not to name it. */
      .en {
        color: var(--mat-sys-outline);
        font-size: 0.8rem;
      }
      /* What is left to translate. Loud on purpose - it is the number that
         sends somebody to the editor, and the reason a copy of this language
         would arrive with holes. */
      .holes {
        padding: 0 0.4rem;
        border-radius: 999px;
        font-size: 0.7rem;
        line-height: 1.3rem;
        background: var(--mat-sys-error-container);
        color: var(--mat-sys-on-error-container);
      }
    `,
  ],
})
export class LanguageRowComponent {
  readonly locale = input.required<LocaleView>();
  // What is left to translate, where that is the question. It is not the
  // question when a source is being chosen to copy: every language but English
  // shows the same fourteen, so it separates nothing, and in red it reads as a
  // warning against a choice that is not a mistake.
  readonly showHoles = input(true);

  protected readonly english = computed(() => {
    const l = this.locale();
    const name = languageName(l.code, 'en');
    return name === l.name ? '' : name;
  });
}
