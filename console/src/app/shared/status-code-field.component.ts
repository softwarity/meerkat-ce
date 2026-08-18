import { Component, computed, input, model } from '@angular/core';
import { MatAutocompleteModule } from '@angular/material/autocomplete';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';

// The HTTP status codes one actually poses on a route, with what each says.
// Not a closed list: an integrator answering a proprietary client may need 418
// or 599, and a picker that refused them would send them back to typing a bare
// number with no help at all.
export const STATUS_CODES: readonly { value: string; what: string }[] = [
  { value: '200', what: $localize`:@@Status_200:OK - here is the answer` },
  { value: '201', what: $localize`:@@Status_201:Created` },
  { value: '202', what: $localize`:@@Status_202:Accepted - handled later` },
  { value: '204', what: $localize`:@@Status_204:No Content - nothing to send back` },
  { value: '301', what: $localize`:@@Status_301:Moved Permanently` },
  { value: '302', what: $localize`:@@Status_302:Found - temporary redirect` },
  { value: '304', what: $localize`:@@Status_304:Not Modified - the cached copy stands` },
  { value: '400', what: $localize`:@@Status_400:Bad Request` },
  { value: '401', what: $localize`:@@Status_401:Unauthorized - sign in first` },
  { value: '403', what: $localize`:@@Status_403:Forbidden - signed in, not allowed` },
  { value: '404', what: $localize`:@@Status_404:Not Found` },
  { value: '410', what: $localize`:@@Status_410:Gone - it existed, it will not come back` },
  { value: '418', what: $localize`:@@Status_418:I'm a teapot` },
  { value: '429', what: $localize`:@@Status_429:Too Many Requests` },
  { value: '500', what: $localize`:@@Status_500:Internal Server Error` },
  { value: '502', what: $localize`:@@Status_502:Bad Gateway` },
  { value: '503', what: $localize`:@@Status_503:Service Unavailable` },
  { value: '504', what: $localize`:@@Status_504:Gateway Timeout` },
];

// One status-code field, wherever a status is posed - the respond terminal, the
// set-status filter, the redirect. It was written twice: a picker with the
// meanings on one screen, a bare number input on the other, and the second is
// where someone types 203 meaning to close a door.
//
// The code and its meaning are NOT drawn alike. In the list the code leads and
// the meaning is a hint beneath it; once chosen, the meaning moves under the
// field, where a hint belongs - the field then shows a number, and the sentence
// that says what the number does sits where nothing else competes with it.
@Component({
  selector: 'app-status-code-field',
  imports: [MatAutocompleteModule, MatFormFieldModule, MatInputModule],
  styles: [
    `
      :host {
        display: block;
        min-width: 260px;
      }
      mat-form-field {
        width: 100%;
      }
      /* One line under the field. Wrapped, the meaning grew the box by a row
         and the field looked like a paragraph with a number in it. */
      mat-hint {
        display: block;
        max-width: 100%;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .code {
        font-weight: 600;
      }
      .what {
        display: block;
        font-size: 0.75rem;
        line-height: 1.25;
        color: var(--mat-sys-on-surface-variant);
      }
      mat-option .what {
        margin-top: -2px;
      }
    `,
  ],
  template: `
    <mat-form-field>
      <mat-label>{{ label() }}</mat-label>
      <input
        matInput
        inputmode="numeric"
        [matAutocomplete]="sc"
        [placeholder]="placeholder()"
        [value]="value()"
        (input)="set($any($event.target).value)"
      />
      <mat-autocomplete #sc="matAutocomplete" panelWidth="320px" (optionSelected)="set($event.option.value)">
        @for (o of filtered(); track o.value) {
          <mat-option [value]="o.value">
            <span class="code">{{ o.value }}</span>
            <small class="what">{{ o.what }}</small>
          </mat-option>
        }
      </mat-autocomplete>
      <!-- What the chosen code says, where a hint goes. Empty for a code the
           list does not know, which is a code someone typed on purpose.
           ALWAYS rendered, never behind an @if: the form field finds its hint
           through a content query at init, and one created later is left where
           it stands - inside the box, which is what it did. -->
      <mat-hint [title]="meaning()">{{ meaning() }}</mat-hint>
    </mat-form-field>
  `,
})
export class StatusCodeFieldComponent {
  readonly value = model<string>('');
  readonly label = input($localize`:@@Status_code:Status code`);
  readonly placeholder = input('200');

  protected readonly meaning = computed(
    () => STATUS_CODES.find((c) => c.value === this.value())?.what ?? '',
  );

  // Typing narrows the list: three digits with a picker of eighteen entries is
  // faster than three digits and a scroll.
  protected readonly filtered = computed(() => {
    const v = (this.value() ?? '').trim();
    if (!v) return STATUS_CODES;
    const hit = STATUS_CODES.filter((c) => c.value.startsWith(v));
    return hit.length ? hit : STATUS_CODES;
  });

  protected set(v: string): void {
    this.value.set(v.replace(/[^0-9]/g, '').slice(0, 3));
  }
}
