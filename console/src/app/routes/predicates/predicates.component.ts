import { Component, computed, input, model } from '@angular/core';
import { type FormValueControl, type ValidationError } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatSidenavModule } from '@angular/material/sidenav';
import { CatalogEntry, Param, Spec } from '../../api.service';
import { PLANNED_PREDICATES, brickDoc } from '../brick-docs';
import { humanize } from './args';
import { PredicateItemComponent } from './predicate-item.component';

// Catalog order: the everyday matchers first, the specialized ones after.
const PREFERRED_ORDER = [
  'path',
  'host',
  'method',
  'header',
  'cookie',
  'query',
  'remote-addr',
  'x-forwarded-remote-addr',
  'version',
  'time-window',
  'weight',
];

// Predicates are ANDed, so a second path/host/... can never widen a route (OR
// lives INSIDE the predicate: several patterns in one path). Only the named
// matchers make sense several times (two different headers, cookies, params).
const MULTI_INSTANCE = ['header', 'cookie', 'query'];

// Predicates section - an addable list; "Add" opens a right-hand drawer with
// the whole catalog, a real explanation per entry. Predicates are ANDed
// (order carries no meaning, no reorder); single-instance types gray out
// once present. A FormValueControl bound with [formField].
@Component({
  selector: 'app-predicates',
  imports: [MatButtonModule, MatIconModule, MatSidenavModule, PredicateItemComponent],
  styleUrl: './predicates.component.scss',
  templateUrl: './predicates.component.html',
})
export class PredicatesComponent implements FormValueControl<Spec[]> {
  readonly value = model<Spec[]>([]);
  readonly entries = input.required<CatalogEntry[]>();
  readonly errors = input<readonly ValidationError.WithOptionalFieldTree[]>([]);

  protected readonly ordered = computed(() => {
    const rank = (t: string) => {
      const i = PREFERRED_ORDER.indexOf(t);
      return i < 0 ? PREFERRED_ORDER.length : i;
    };
    return [...this.entries()].sort((a, b) => rank(a.type) - rank(b.type) || a.type.localeCompare(b.type));
  });

  protected label(value: string): string {
    return humanize(value);
  }

  // Promises minus what the gateway already serves, so a matcher that ships
  // stops announcing itself without anyone editing this file.
  protected readonly planned = computed(() => {
    const shipped = new Set(this.entries().map((e) => e.type));
    return PLANNED_PREDICATES.filter((b) => !shipped.has(b.type));
  });

  // What the catalog says this brick takes, for the generic editor.
  protected paramsOf(type: string): Param[] {
    return this.entries().find((e) => e.type === type)?.params ?? [];
  }

  protected refOf(type: string) {
    return this.entries().find((e) => e.type === type)?.ref;
  }

  protected doc(type: string): string {
    return brickDoc(type) || this.entries().find((e) => e.type === type)?.doc || '';
  }

  // The long form for a POSED predicate; the palette keeps the short line.
  protected details(type: string): string {
    return this.entries().find((e) => e.type === type)?.details || this.doc(type);
  }

  // Single-instance types gray out once present (ANDing a second one could
  // only narrow the match to nothing).
  protected taken(type: string): boolean {
    return !MULTI_INSTANCE.includes(type) && this.value().some((s) => s.type === type);
  }

  // What a brick starts with: the catalog's `initial` values, written into the
  // spec so they are SEEN and editable. Not defaults - the server applies
  // none of them, so a required argument is still required.
  private initialArgs(type: string): Record<string, unknown> {
    const args: Record<string, unknown> = {};
    for (const p of this.entries().find((e) => e.type === type)?.params ?? []) {
      if (p.initial !== undefined) args[p.name] = p.initial;
    }
    return args;
  }

  protected add(type: string): void {
    this.value.update((list) => [...list, { type, ...(Object.keys(this.initialArgs(type)).length ? { args: this.initialArgs(type) } : {}) }]);
  }

  protected updateAt(index: number, spec: Spec): void {
    this.value.update((list) => list.map((s, i) => (i === index ? spec : s)));
  }

  protected removeAt(index: number): void {
    this.value.update((list) => list.filter((_, i) => i !== index));
  }
}
