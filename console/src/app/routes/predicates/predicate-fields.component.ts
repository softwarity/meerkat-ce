import { Component, computed, inject, linkedSignal, model, signal, untracked } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatRadioModule } from '@angular/material/radio';
import { MatIconModule } from '@angular/material/icon';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { ApiService, Spec, VersionPreview } from '../../api.service';
import { argBool, argList, argStr, patchSpec } from './args';
import { StringListComponent } from './string-list.component';

// Dedicated editors, one per predicate shape. Each takes the predicate Spec
// through a model() signal and edits its args in place - the type is fixed at
// add-time (chosen from the catalog menu), same pattern as the filters.

const FIELDS_STYLE = `
  .fields {
    display: grid;
    gap: 4px 14px;
    grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
    align-items: start;
  }
  mat-form-field {
    width: 100%;
  }
  mat-checkbox {
    align-self: center;
  }
`;

const METHODS = ['GET', 'HEAD', 'POST', 'PUT', 'PATCH', 'DELETE', 'OPTIONS'];

// One string-list arg: path (patterns) and host (hosts).
@Component({
  selector: 'app-list-predicate',
  imports: [StringListComponent],
  templateUrl: './predicate-fields.component.html',
})
export class ListPredicateComponent {
  readonly spec = model.required<Spec>();
  protected readonly key = computed(() => (this.spec().type === 'host' ? 'hosts' : 'patterns'));
  protected readonly items = computed(() => argList(this.spec(), this.key()));
  protected readonly placeholder = computed(() =>
    this.spec().type === 'host' ? '*.example.com' : '/api/**',
  );
  protected set(values: string[]): void {
    this.spec.update((s) => patchSpec(s, this.key(), values));
  }
}

// The HTTP methods, as a multiselect over the standard verbs.
@Component({
  selector: 'app-method-predicate',
  imports: [MatFormFieldModule, MatSelectModule],
  styles: [FIELDS_STYLE],
  template: `
    <div class="fields">
      <mat-form-field>
        <mat-label i18n="@@Methods">Methods</mat-label>
        <mat-select multiple [value]="methods()" (selectionChange)="set($event.value)">
          @for (m of allMethods; track m) {
            <mat-option [value]="m">{{ m }}</mat-option>
          }
        </mat-select>
      </mat-form-field>
    </div>
  `,
})
export class MethodPredicateComponent {
  readonly spec = model.required<Spec>();
  protected readonly allMethods = METHODS;
  protected readonly methods = computed(() => argList(this.spec(), 'methods'));
  protected set(values: string[]): void {
    this.spec.update((s) => patchSpec(s, 'methods', values));
  }
}

// name + optional full-match regexp: header, cookie and query share the shape.
@Component({
  selector: 'app-matcher-predicate',
  imports: [MatFormFieldModule, MatInputModule, StringListComponent],
  styles: [
    FIELDS_STYLE,
    `
      .values {
        margin-top: 4px;
        max-width: 340px;
      }
      .values-label {
        margin: 0 0 2px;
        font-size: 0.75rem;
        color: var(--mat-sys-on-surface-variant);
      }
    `,
  ],
  template: `
    <div class="fields">
      <mat-form-field>
        <mat-label i18n="@@Name">Name</mat-label>
        <input matInput required [value]="name()" (input)="set('name', $any($event.target).value)" />
      </mat-form-field>
      <!-- Disabled rather than hidden when a list is posed: the two are
           exclusive on the server, and a field that vanishes leaves the reader
           looking for it. -->
      <mat-form-field>
        <mat-label i18n="@@Value_regexp">Value (regexp)</mat-label>
        <input
          matInput
          [disabled]="values().length > 0"
          i18n-placeholder="@@empty_presence_only"
          placeholder="empty = presence only"
          [value]="regexp()"
          (input)="set('regexp', $any($event.target).value)"
        />
      </mat-form-field>
    </div>
    <div class="values">
      <p class="values-label" i18n="@@Or_one_of_these_values">Or one of these values</p>
      <app-string-list [values]="values()" (valuesChange)="setValues($event)" i18n-placeholder="@@Value" placeholder="Value" />
    </div>
  `,
})
export class MatcherPredicateComponent {
  readonly spec = model.required<Spec>();
  protected readonly name = computed(() => argStr(this.spec(), 'name'));
  protected readonly regexp = computed(() => argStr(this.spec(), 'regexp'));
  protected readonly values = computed(() => argList(this.spec(), 'values'));
  protected set(key: string, v: string): void {
    this.spec.update((s) => patchSpec(s, key, v));
  }

  // The list is kept AS TYPED, blanks included: Add creates an empty row, and
  // filtering them here deleted it the instant it appeared - the button looked
  // dead. Blank rows are dropped at save, like the path patterns' are.
  //
  // A list and a regexp are exclusive, so a real value clears the pattern;
  // rows that are still empty decide nothing and leave it alone.
  protected setValues(values: string[]): void {
    const posed = values.some((v) => v !== '');
    this.spec.update((s) => patchSpec(posed ? patchSpec(s, 'regexp', '') : s, 'values', values));
  }
}

// CIDR sources; remote-addr additionally offers the trusted-proxy toggle
// (x-forwarded-remote-addr always reads the rightmost X-Forwarded-For entry).
@Component({
  selector: 'app-addr-predicate',
  imports: [MatCheckboxModule, StringListComponent],
  template: `
    <app-string-list [values]="cidrs()" (valuesChange)="set($event)" placeholder="10.0.0.0/8" />
    @if (spec().type === 'remote-addr') {
      <mat-checkbox [checked]="useForwarded()" (change)="setForwarded($event.checked)">
        <ng-container i18n="@@Trust_first_X_Forwarded_For">
          Trust the first X-Forwarded-For entry (only behind a trusted proxy)
        </ng-container>
      </mat-checkbox>
    }
  `,
})
export class AddrPredicateComponent {
  readonly spec = model.required<Spec>();
  protected readonly cidrs = computed(() => argList(this.spec(), 'cidrs'));
  protected readonly useForwarded = computed(() => argBool(this.spec(), 'useForwarded'));
  protected set(values: string[]): void {
    this.spec.update((s) => patchSpec(s, 'cidrs', values));
  }
  protected setForwarded(v: boolean): void {
    this.spec.update((s) => patchSpec(s, 'useForwarded', v ? true : ''));
  }
}

// RFC 3339 bounds: after/before take one datetime, between takes two.
// One window, open at either end or both. Empty is not "no bound" by accident:
// leaving one side blank IS how a route starts matching at a moment and never
// stops, or stops at a moment having always matched - which is what the three
// bricks this replaced used to say separately.
@Component({
  selector: 'app-window-predicate',
  imports: [MatFormFieldModule, MatInputModule],
  styles: [FIELDS_STYLE],
  template: `
    <div class="fields">
      <mat-form-field>
        <mat-label i18n="@@From">From</mat-label>
        <input
          matInput
          placeholder="2026-01-20T17:42:47+01:00"
          [value]="from()"
          (input)="set('from', $any($event.target).value)"
        />
        <mat-hint i18n="@@Empty_matches_from_the_start">empty: matches from the start</mat-hint>
      </mat-form-field>
      <mat-form-field>
        <mat-label i18n="@@To">To</mat-label>
        <input
          matInput
          placeholder="2026-01-21T17:42:47+01:00"
          [value]="to()"
          (input)="set('to', $any($event.target).value)"
        />
        <mat-hint i18n="@@Empty_never_closes">empty: never closes</mat-hint>
      </mat-form-field>
    </div>
  `,
})
export class WindowPredicateComponent {
  readonly spec = model.required<Spec>();
  protected readonly from = computed(() => argStr(this.spec(), 'from'));
  protected readonly to = computed(() => argStr(this.spec(), 'to'));
  protected set(key: string, v: string): void {
    this.spec.update((s) => patchSpec(s, key, v));
  }
}

// Canary split: group + weight.
@Component({
  selector: 'app-weight-predicate',
  imports: [MatFormFieldModule, MatInputModule],
  styles: [FIELDS_STYLE],
  template: `
    <div class="fields">
      <mat-form-field>
        <mat-label i18n="@@Group">Group</mat-label>
        <input matInput required [value]="group()" (input)="set('group', $any($event.target).value)" />
      </mat-form-field>
      <mat-form-field>
        <mat-label i18n="@@Weight">Weight</mat-label>
        <input matInput type="number" min="1" required [value]="weight()" (input)="setWeight($any($event.target).value)" />
      </mat-form-field>
    </div>
  `,
})
export class WeightPredicateComponent {
  readonly spec = model.required<Spec>();
  protected readonly group = computed(() => argStr(this.spec(), 'group'));
  protected readonly weight = computed(() => argStr(this.spec(), 'weight'));
  protected set(key: string, v: string): void {
    this.spec.update((s) => patchSpec(s, key, v));
  }
  protected setWeight(v: string): void {
    const n = parseInt(v, 10);
    this.spec.update((s) => patchSpec(s, 'weight', Number.isFinite(n) ? n : ''));
  }
}

// The pattern each source starts on. It is always WRITTEN OUT, never applied
// behind the scenes, because the permissive one it is - any dotted number -
// reads an order id as a version on /api/orders/42. Offered, it can be
// anchored; assumed, it is a route that matches the wrong thing in silence.
//
// One capturing group and no name: the engine takes the first group when none
// is named, and naming the only group there is says nothing.
function defaultPattern(source: string): string {
  return source === 'path' ? String.raw`/v?(\d+(?:\.\d+)*)` : String.raw`v?(\d+(?:\.\d+)*)`;
}

// Version: a half-open range, written as one. The bracket in the prefix and the
// one in the suffix ARE the explanation - [ 2.0 .. 3.0 [ says inclusive on the
// left and exclusive on the right without a sentence saying so.
//
// Three lines, in the order the brick works: where the version is read, how it
// is pulled out of what was read, then what range is accepted. The try-it box
// closes it, because a regexp typed blind is a route that silently matches
// nothing - and the answer comes from the ENGINE (RE2 refuses lookarounds
// JavaScript allows, so guessing here would promise what the gateway refuses).
@Component({
  selector: 'app-version-predicate',
  imports: [MatButtonModule, MatFormFieldModule, MatIconModule, MatInputModule, MatRadioModule],
  styles: [
    FIELDS_STYLE,
    `
      .line {
        display: flex;
        align-items: baseline;
        gap: 16px;
        flex-wrap: wrap;
      }
      /* The range has to sit on ONE line with the pattern that feeds it: read
         left to right, it says extract this, accept that. Wrapped, the two
         halves stop being one sentence. */
      .line mat-form-field {
        width: 150px;
      }
      .line .wide {
        width: 260px;
      }
      mat-radio-group {
        display: flex;
        gap: 8px;
      }
      .bound {
        color: var(--mat-sys-primary);
        font-weight: 600;
      }
      .verdict {
        display: flex;
        align-items: center;
        gap: 6px;
        font-size: 0.82rem;
        color: var(--mat-sys-on-surface-variant);
      }
      .verdict mat-icon {
        font-size: 18px;
        width: 18px;
        height: 18px;
      }
      .verdict.in {
        color: var(--mat-sys-primary);
      }
      .verdict.out {
        color: var(--mat-sys-error);
      }
      .extracted {
        font-family: monospace;
      }
    `,
  ],
  template: `
    <div class="line">
      <mat-radio-group [value]="source()" (change)="setSource($event.value)">
        <mat-radio-button value="header" i18n="@@Header">Header</mat-radio-button>
        <mat-radio-button value="query" i18n="@@Parameter">Parameter</mat-radio-button>
        <mat-radio-button value="path" i18n="@@Path">Path</mat-radio-button>
      </mat-radio-group>
      <!-- Shown and DISABLED on path rather than removed: a field that vanishes
           leaves the reader wondering where it went. -->
      <mat-form-field class="wide" floatLabel="always">
        <mat-label i18n="@@Name">Name</mat-label>
        <input
          matInput
          [value]="name()"
          [disabled]="source() === 'path'"
          placeholder="X-API-Version"
          (input)="patch('name', $any($event.target).value)"
        />
      </mat-form-field>
    </div>
    <div class="line">
      <mat-form-field class="wide">
        <mat-label i18n="@@Pattern">Pattern</mat-label>
        <input
          matInput
          [value]="pattern()"
          
          (input)="patch('pattern', $any($event.target).value)"
        />
      </mat-form-field>
      <mat-form-field>
        <mat-label i18n="@@From_included">From (included)</mat-label>
        <span class="bound" matTextPrefix>[&nbsp;</span>
        <input matInput [value]="from()" placeholder="2.0" (input)="patch('from', $any($event.target).value)" />
      </mat-form-field>
      <mat-form-field>
        <mat-label i18n="@@To_excluded">To (excluded)</mat-label>
        <input matInput [value]="to()" placeholder="3.0" (input)="patch('to', $any($event.target).value)" />
        <span class="bound" matTextSuffix>&nbsp;[</span>
      </mat-form-field>
    </div>
    <div class="line">
      <mat-form-field class="wide">
        <mat-label i18n="@@Try_it">Try it</mat-label>
        <input matInput [value]="sample()" (keydown.enter)="test()" (input)="sample.set($any($event.target).value)" />
      </mat-form-field>
      <button matButton="tonal" [disabled]="!sample() || testing()" (click)="test()" i18n="@@Test">Test</button>
      @if (verdict(); as v) {
        <span class="verdict" [class.in]="v.state === 'in'" [class.out]="v.state === 'out'">
          <mat-icon>{{ v.icon }}</mat-icon>
          <span>{{ v.text }}</span>
        </span>
      }
    </div>
  `,
})
export class VersionPredicateComponent {
  readonly spec = model.required<Spec>();
  private readonly api = inject(ApiService);

  protected readonly from = computed(() => argStr(this.spec(), 'from'));
  protected readonly to = computed(() => argStr(this.spec(), 'to'));
  protected readonly name = computed(() => argStr(this.spec(), 'name'));
  protected readonly pattern = computed(() => argStr(this.spec(), 'pattern'));
  // Absent means the default the server applies, and the radio has to show it:
  // a pair of empty radios reads as "not decided yet" for something that is.
  protected readonly source = computed(() => argStr(this.spec(), 'source') || 'header');
  // Filled, not suggested: a placeholder asks someone to invent an example
  // before they can see anything, and the verdict is the whole point of the
  // box. It follows the source - a path sample says nothing about a header -
  // and typing over it wins until the source changes.
  //
  // linkedSignal over the source COMPUTED, not over an expression reading the
  // spec: a linkedSignal is reseeded when a producer it read changes version,
  // and the spec changes on every keystroke in the other fields, which would
  // wipe what was typed here mid-sentence.
  protected readonly sample = linkedSignal({
    source: () => this.source(),
    computation: (source: string) => (source === 'path' ? '/api/v2.1/orders' : '2.1'),
  });

  // On demand, not on every keystroke. And the answer is CLEARED the moment
  // anything that would change it moves - the sample or the brick's own
  // arguments - because a verdict left standing after the range was edited is
  // a verdict about a rule that no longer exists.
  private readonly inputs = computed(() => JSON.stringify({ args: this.spec().args ?? {}, sample: this.sample() }));
  private readonly answer = linkedSignal<string, VersionPreview | null>({
    source: () => this.inputs(),
    computation: () => null,
  });
  protected readonly testing = signal(false);

  protected test(): void {
    const sample = this.sample();
    if (!sample) return;
    this.testing.set(true);
    this.api.versionPreview(this.spec().args ?? {}, sample).subscribe({
      next: (res) => {
        this.answer.set(res);
        this.testing.set(false);
      },
      error: () => this.testing.set(false),
    });
  }

  protected readonly verdict = computed(() => {
    const a = this.answer();
    if (!this.sample() || !a) return null;
    if (a.error) return { state: 'err', icon: 'error_outline', text: a.error };
    if (!a.extracted) {
      return {
        state: 'out',
        icon: 'block',
        text: $localize`:@@No_version_found:nothing extracted - this route would not match`,
      };
    }
    return a.matches
      ? { state: 'in', icon: 'check_circle', text: $localize`:@@Extracted_matches:${a.extracted}:VERSION: - inside the range` }
      : { state: 'out', icon: 'cancel', text: $localize`:@@Extracted_outside:${a.extracted}:VERSION: - outside the range` };
  });

  protected patch(key: string, value: string): void {
    this.spec.update((s) => patchSpec(s, key, value));
  }

  // Choosing the path fills the pattern in, rather than applying one in
  // silence: a default that reads /api/orders/42 as version 42 is right often
  // enough to be offered and wrong often enough that it has to be SEEN - and
  // anchored, when a route carries ids in its urls. An existing pattern is
  // never overwritten.
  protected setSource(source: string): void {
    this.spec.update((s) => {
      const next = patchSpec(s, 'source', source);
      // The pattern follows the source while it is still one of the offered
      // ones - a path pattern starts with a slash and a header one does not -
      // but an edited pattern is somebody's work and is left alone.
      const current = argStr(next, 'pattern');
      const offered = !current || current === defaultPattern('path') || current === defaultPattern('header');
      return offered ? patchSpec(next, 'pattern', defaultPattern(source)) : next;
    });
  }
}
