import { Component, computed, input, linkedSignal, model } from '@angular/core';
import { type FormValueControl, type ValidationError } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatSidenavModule } from '@angular/material/sidenav';
import { CatalogEntry, Param, Spec } from '../../api.service';
import { PLANNED_MODIFIERS, brickDoc } from '../brick-docs';
import { humanize } from '../predicates/args';
import { FilterItemComponent } from './filter-item.component';

// ONE PHASE of the route's modifiers (incoming request / outgoing response /
// terminal): the section shows and edits only its phase's specs, while the
// bound value stays the route's WHOLE modifier list (indices are global, the
// engine splits by phase at compile time). "Add" opens a right-hand drawer:
// the phase's catalog with a real explanation per entry, plus the PLANNED
// bricks grayed out - what exists and what is coming, in one place.
// A FormValueControl: bound with [formField], schema errors render at the top.
@Component({
  selector: 'app-filters',
  imports: [MatButtonModule, MatIconModule, MatSidenavModule, FilterItemComponent],
  styleUrl: './filters.component.scss',
  templateUrl: './filters.component.html',
})
export class FiltersComponent implements FormValueControl<Spec[]> {
  readonly value = model<Spec[]>([]);
  readonly entries = input.required<CatalogEntry[]>();
  // 'request' | 'response' | 'terminal' - the ONLY phase this section touches.
  readonly phase = input.required<string>();
  readonly errors = input<readonly ValidationError.WithOptionalFieldTree[]>([]);

  protected readonly phaseEntries = computed(() =>
    this.entries().filter((e) => (e.phase ?? 'request') === this.phase()),
  );
  // Promises minus what the server actually serves. The catalog is the truth
  // about what exists; a brick that shipped under the SAME name stops being
  // announced as coming the moment the gateway lists it, without anyone having
  // to remember this file.
  protected readonly planned = computed(() => {
    const shipped = new Set(this.entries().map((e) => e.type));
    return (PLANNED_MODIFIERS[this.phase()] ?? []).filter((b) => !shipped.has(b.type));
  });

  // This phase's specs, each keeping its GLOBAL index in the bound list.
  protected readonly indexed = computed(() =>
    this.value()
      .map((s, i) => ({ s, i }))
      .filter(({ s }) => this.typePhase(s.type) === this.phase()),
  );

  // A route has at most one terminal (the engine refuses more).
  protected readonly full = computed(() => this.phase() === 'terminal' && this.indexed().length > 0);

  // Whether this phase holds nothing - as a COMPUTED, which is the whole point
  // and not a style choice. A linkedSignal recomputes when a producer it read
  // changed VERSION, never when the expression's result changed: reading
  // indexed() straight from the computation below made the filter list a
  // producer, and the form re-emits that list as a new array on every cycle.
  // The palette was opening on the click and being thrown shut in the same
  // tick, so Add looked dead on any section that already had a modifier.
  // Behind a computed, the boolean only bumps its version when it really
  // moves, and a palette opened by hand stays open.
  private readonly phaseEmpty = computed(() => this.indexed().length === 0);

  // Nothing posed in this phase? Open the palette straight away - an empty
  // section has nothing to read, and the next move is always "add one". It
  // stays closable (openedChange writes back), and reseeds when the phase
  // empties or gets its first modifier.
  protected readonly palOpen = linkedSignal(() => this.phaseEmpty());

  // What the palette's title says it adds, and what the section calls what it
  // holds. A gate is not a modifier - it refuses instead of transforming - and
  // a screen that calls it one teaches the wrong thing on the one word the
  // reader will remember.
  protected readonly addTitle = computed(() =>
    this.phase() === 'gate'
      ? $localize`:@@Add_gate:Add gate`
      : $localize`:@@Add_modifier:Add modifier`,
  );

  protected readonly intro = computed(() => {
    switch (this.phase()) {
      case 'gate':
        return $localize`:@@Gate_intro:What this route refuses to carry, checked in order before anything else it does. Unlike a predicate or an access rule - which let the next route try - a gate that refuses answers the caller and stops there. Remove a gate to lift its limit.`;
      case 'request':
        return $localize`:@@Incoming_modifiers_intro:Applied in order to the request before it reaches the service.`;
      case 'response':
        return $localize`:@@Outgoing_modifiers_intro:Applied in order to the response before it reaches the client.`;
      default:
        return $localize`:@@Terminal_modifiers_intro:Answers instead of proxying (redirect, maintenance): at most one, not combined with other modifiers.`;
    }
  });

  protected readonly empty = computed(() => {
    switch (this.phase()) {
      case 'terminal':
        return $localize`:@@No_terminal_the_route_proxies:None: the route proxies to its upstream.`;
      case 'gate':
        return $localize`:@@No_gates_yet:No gates yet: this route carries whatever it is sent.`;
      default:
        return $localize`:@@No_modifiers_yet:No modifiers yet.`;
    }
  });

  // What the catalog says this brick takes, for the generic editor.
  protected paramsOf(type: string): Param[] {
    return this.entries().find((e) => e.type === type)?.params ?? [];
  }

  protected refOf(type: string) {
    return this.entries().find((e) => e.type === type)?.ref;
  }

  private typePhase(type: string): string {
    return this.entries().find((e) => e.type === type)?.phase ?? 'request';
  }

  protected label(value: string): string {
    return humanize(value);
  }

  protected doc(type: string): string {
    return brickDoc(type) || this.entries().find((e) => e.type === type)?.doc || '';
  }

  // What a POSED brick says about itself: the long form when the catalog has
  // one, the palette's line otherwise. Read with the thing in front of you, so
  // it carries what the short line has no room for - what it breaks, what it
  // leaves alone, what to reach for instead.
  protected details(type: string): string {
    return this.entries().find((e) => e.type === type)?.details || this.doc(type);
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

  // Reorder WITHIN the phase: swap the two global slots.
  protected move(displayIdx: number, dir: -1 | 1): void {
    const rows = this.indexed();
    const a = rows[displayIdx];
    const b = rows[displayIdx + dir];
    if (!a || !b) return;
    this.value.update((list) => {
      const out = [...list];
      [out[a.i], out[b.i]] = [out[b.i], out[a.i]];
      return out;
    });
  }
}
