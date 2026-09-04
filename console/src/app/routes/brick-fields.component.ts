import { Component, computed, input, model } from '@angular/core';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatSelectModule } from '@angular/material/select';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { Param, Spec } from '../api.service';
import { argBool, argList, argStr, humanize, patchSpec } from './predicates/args';
import { StringListComponent } from './predicates/string-list.component';

// The editor for any brick - predicate or filter - that has no dedicated one,
// which is to say for every brick added since the dedicated ones were written.
//
// The catalog already describes each parameter: its name, its kind, whether it
// is required, its default and a line of documentation. That description was
// serving the palette and nothing else, so a brick outside the hand-kept map of
// fifteen types rendered as a card with a title, two reorder arrows and NO WAY
// to set anything - it could be added and never configured. Sixteen bricks were
// in that state.
//
// Driving the fields from the catalog instead means the next brick is editable
// the day the gateway registers it, with no console change at all.
@Component({
  selector: 'app-brick-fields',
  imports: [MatCheckboxModule, MatFormFieldModule, MatInputModule, MatSelectModule, StringListComponent],
  styles: [
    `
      .fields {
        display: grid;
        gap: 4px 14px;
        /* Wide enough for a documented field: the doc rides in the hint, and
           at 200px a sentence of it wrapped over four lines and made the card
           taller than the brick it describes. */
        grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
        align-items: start;
      }
      mat-form-field {
        width: 100%;
      }
      mat-checkbox {
        align-self: center;
      }
      .list-field {
        grid-column: 1 / -1;
      }
      .list-label {
        margin: 6px 0 2px;
        font-size: 0.75rem;
        color: var(--mat-sys-on-surface-variant);
      }
    `,
  ],
  template: `
    <!-- The hints are NOT wrapped in @if: Material projects mat-hint by
         selector, and projection does not look through a control-flow block -
         inside one, the hint lands in the field's own content, under the label
         and inside the outline. Empty, it simply renders nothing. -->
    <div class="fields">
      @for (p of params(); track p.name) {
        @if (p.options?.length) {
          <!-- A closed set, declared by the catalog: a list rather than a text
               field whose spelling has to be guessed - and the same list the
               server checks against, so the two cannot drift. -->
          <mat-form-field>
            <mat-label>{{ label(p) }}</mat-label>
            <mat-select [required]="!!p.required" [value]="str(p.name)" (selectionChange)="set(p.name, $event.value)">
              @for (o of p.options; track o) {
                <mat-option [value]="o">{{ o || none }}</mat-option>
              }
            </mat-select>
            <mat-hint>{{ p.doc }}</mat-hint>
          </mat-form-field>
        } @else {
        @switch (p.kind) {
          @case ('bool') {
            <mat-checkbox [checked]="bool(p.name)" (change)="set(p.name, $event.checked)">
              {{ label(p) }}
            </mat-checkbox>
          }
          @case ('stringList') {
            <div class="list-field">
              <p class="list-label">{{ label(p) }}@if (p.doc) { - {{ p.doc }} }</p>
              <app-string-list [values]="list(p.name)" (valuesChange)="set(p.name, $event)" />
            </div>
          }
          @case ('int') {
            <mat-form-field>
              <mat-label>{{ label(p) }}</mat-label>
              <input matInput type="number" [required]="!!p.required" [value]="str(p.name)" (input)="setNumber(p.name, $any($event.target).value)" />
              <mat-hint>{{ p.doc }}</mat-hint>
            </mat-form-field>
          }
          @default {
            <mat-form-field>
              <mat-label>{{ label(p) }}</mat-label>
              <input matInput [required]="!!p.required" [value]="str(p.name)" [placeholder]="placeholder(p)" (input)="set(p.name, $any($event.target).value)" />
              <mat-hint>{{ p.doc }}</mat-hint>
            </mat-form-field>
          }
        }
        }
      } @empty {
        <p class="list-label" i18n="@@Brick_takes_no_setting">This brick takes no setting.</p>
      }
    </div>
  `,
})
export class BrickFieldsComponent {
  readonly spec = model.required<Spec>();
  readonly params = input<Param[]>([]);

  // What an empty option is called in the list: "" reads as a typo.
  protected readonly none = $localize`:@@Leave_as_is:leave as is`;

  protected str(name: string): string {
    return argStr(this.spec(), name);
  }

  protected bool(name: string): boolean {
    return argBool(this.spec(), name);
  }

  // A list arg starts as ONE empty row rather than none: an empty list editor
  // shows only its Add button, and the first thing to do is always to type a
  // value into it.
  protected list(name: string): string[] {
    const values = argList(this.spec(), name);
    return values.length ? values : [''];
  }

  protected set(name: string, value: unknown): void {
    this.spec.update((s) => patchSpec(s, name, value));
  }

  // Numbers travel as numbers: the server decodes an int arg strictly, and a
  // quoted "3" from an input's string value is refused at save.
  protected setNumber(name: string, value: string): void {
    const n = Number(value);
    this.spec.update((s) => patchSpec(s, name, value.trim() === '' || !Number.isFinite(n) ? '' : n));
  }

  // The star is Material's now, drawn from the control's own required flag -
  // which is also what paints the field when it is required and empty, and
  // what a screen reader announces. Two stars would be one too many.
  protected label(p: Param): string {
    return humanize(p.name);
  }

  // The default is shown as the placeholder, so "what happens if I leave this
  // alone" is answered in the field itself rather than in a paragraph above it.
  protected placeholder(p: Param): string {
    return p.default === undefined || p.default === null || p.default === '' ? '' : String(p.default);
  }
}
