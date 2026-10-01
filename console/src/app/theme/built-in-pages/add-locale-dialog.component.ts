import { Component, computed, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MAT_DIALOG_DATA, MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { LocaleView } from '../../api.service';
import { canonicalTag, languageName } from '../../shared/language-name';
import { LanguageRowComponent } from '../../shared/language-row.component';

export interface AddLocaleData {
  // The languages this gateway already renders: what a new code may not be, and
  // what a new one can be copied from.
  existing: LocaleView[];
}

export interface AddLocaleResult {
  // The canonical tag - fr-ca typed becomes fr-CA created, or the two spellings
  // would be two languages saying the same thing.
  code: string;
  // Which language to start from, empty to start from nothing. A language that
  // starts from nothing renders English until it is filled, so this is a head
  // start rather than a requirement.
  from: string;
}

// Add a language the binary does not ship (I18N-05).
//
// The code carries the whole decision, so the code is checked before anything
// else is offered: a tag the browser cannot name is a typo, and naming the
// language back is how somebody sees that fr-CA is the Canadian French they
// meant. Until it is valid there is nothing to duplicate FROM, because there is
// nothing to duplicate INTO.
//
// Duplicating exists for variants. Starting fr-CA from fr means correcting a
// hundred wordings instead of writing three hundred, which is the difference
// between a variant somebody maintains and one they abandon.
@Component({
  selector: 'app-add-locale-dialog',
  imports: [
    MatButtonModule,
    MatDialogModule,
    MatFormFieldModule,
    MatInputModule,
    MatSelectModule,
    LanguageRowComponent,
  ],
  styles: [
    `
      mat-form-field {
        width: 100%;
      }
      /* Material keeps a field's hint inside the field's own box, so two of
         them stacked put a sentence about the first against the label of the
         second. They are two questions, and they need to look it. */
      mat-form-field + mat-form-field {
        margin-top: 1rem;
      }
      /* The resolved language, in the field beside the code that resolved to
         it. It is the answer to "is this the tag I think it is", so it sits
         where the question was typed. */
      .named {
        max-width: 14rem;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
        color: var(--mat-sys-on-surface-variant);
      }
      .gone {
        visibility: hidden;
      }
      .bad {
        color: var(--mat-sys-error);
      }
    `,
  ],
  template: `
    <h2 mat-dialog-title i18n="@@Add_a_language">Add a language</h2>
    <mat-dialog-content>
      <mat-form-field>
        <mat-label i18n="@@Language_tag2">BCP 47 language tag</mat-label>
        <input
          matInput
          [value]="raw()"
          (input)="raw.set($any($event.target).value)"
          (keydown.enter)="confirm()"
          placeholder="fr-CA"
          cdkFocusInitial
        />
        <!-- The language the code names, IN the field: a tag is five characters
             and the rest of the line is empty, so the answer sits where the
             question was asked. Always rendered and hidden when there is none -
             inside an @if it would land in the default slot instead of the
             suffix, the control-flow block having a static selector. -->
        <span matTextSuffix class="named" [class.gone]="!label()">{{ label() }}</span>
        <!-- The same language twice: in itself beside the code, in English
             underneath. A translator reads the endonym, and whoever is adding
             the language may not - the pair is what makes fr-CA unambiguous to
             both. Right, under the endonym it doubles rather than under the
             label of the field below. The refusal takes the same line, because
             it answers the same question. -->
        <mat-hint align="end" [class.bad]="!!error()">{{ error() || english() }}</mat-hint>
      </mat-form-field>

      <mat-form-field>
        <mat-label i18n="@@Start_from">Start from</mat-label>
        <mat-select [value]="from()" (valueChange)="pickFrom($event)" [disabled]="!!error() || !label()">
          <!-- The closed control says it in two words; the list is the picker's
               own menu, minus what is left to translate: that number is the
               same on every language but English, so it separates nothing
               here. -->
          <mat-select-trigger>{{ triggerLabel() }}</mat-select-trigger>
          <mat-option value="" i18n="@@Nothing_English">Nothing - the pages show English</mat-option>
          @for (l of data.existing; track l.code) {
            <mat-option [value]="l.code"><app-language-row [locale]="l" [showHoles]="false" /></mat-option>
          }
        </mat-select>
        <!-- And the source named the same way: its own language in the control,
             English underneath. -->
        <mat-hint align="end">{{ sourceEnglish() }}</mat-hint>
      </mat-form-field>
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button matButton mat-dialog-close i18n="@@Cancel">Cancel</button>
      <button matButton="filled" [disabled]="!!error() || !label()" (click)="confirm()" i18n="@@Add">
        Add
      </button>
    </mat-dialog-actions>
  `,
})
export class AddLocaleDialogComponent {
  protected readonly data = inject<AddLocaleData>(MAT_DIALOG_DATA);
  private readonly ref = inject(MatDialogRef<AddLocaleDialogComponent, AddLocaleResult>);

  protected readonly raw = signal('');

  protected readonly canonical = computed(() => canonicalTag(this.raw()));

  // What that tag means, in its own language - the proof the code says what one
  // thinks. Empty when CLDR names no language for it, and such a tag is
  // REFUSED: a code is valid or it is not. See languageName for why the browser
  // has to be asked with fallback: 'none' for that to hold.
  protected readonly label = computed(() => languageName(this.canonical(), this.canonical()));

  // The same language in English, for whoever is adding a language they do not
  // read. Empty when it would only repeat the endonym, which is the case for
  // English itself and for anyone whose language names itself the same way.
  protected readonly english = computed(() => {
    const code = this.canonical();
    if (!code) return '';
    const name = languageName(code, 'en');
    return name === this.label() ? '' : name;
  });

  // What the closed control shows: the code and the language's own name. The
  // rows carry English and the holes; a trigger that repeated all of it would
  // be a paragraph in a field.
  protected readonly triggerLabel = computed(() => {
    const code = this.from();
    if (!code) return $localize`:@@Nothing_English:Nothing - the pages show English`;
    const own = this.data.existing.find((l) => l.code === code)?.name ?? '';
    return own ? code + ' - ' + own : code;
  });

  protected readonly sourceEnglish = computed(() => {
    const code = this.from();
    if (!code) return '';
    const own = this.data.existing.find((l) => l.code === code)?.name ?? '';
    const name = languageName(code, 'en');
    return name === own ? '' : name;
  });

  protected readonly error = computed(() => {
    const v = this.raw().trim();
    if (!v) return '';
    const code = this.canonical();
    if (!code) return $localize`:@@Language_tag_invalid:Not a language tag.`;
    if (this.data.existing.some((l) => l.code === code)) {
      return $localize`:@@Language_already_there:${code}:code: is already here.`;
    }
    // Well formed and naming nothing is the same answer: a tag whose language
    // no browser knows is a typo far more often than it is Occitan.
    if (!this.label()) return $localize`:@@Language_tag_invalid:Not a language tag.`;
    return '';
  });

  // The parent language, preselected: fr-CA is being added because fr exists,
  // and copying fr is what somebody came to do. A choice of their own wins over
  // the suggestion for good - chosen holds null until they make one.
  private readonly chosen = signal<string | null>(null);
  protected readonly suggested = computed(() => {
    const code = this.canonical();
    const i = code.indexOf('-');
    if (i < 1) return '';
    const parent = code.slice(0, i);
    return this.data.existing.some((l) => l.code === parent) ? parent : '';
  });
  protected readonly from = computed(() => this.chosen() ?? this.suggested());

  protected pickFrom(code: string): void {
    this.chosen.set(code);
  }

  protected confirm(): void {
    const code = this.canonical();
    if (!code || this.error()) return;
    this.ref.close({ code, from: this.from() });
  }
}
