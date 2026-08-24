import { Component, computed, input, model, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { BrickRef, Param, Spec } from '../../api.service';
import { humanize } from './args';
import {
  AddrPredicateComponent,
  WindowPredicateComponent,
  ListPredicateComponent,
  MatcherPredicateComponent,
  MethodPredicateComponent,
  VersionPredicateComponent,
  WeightPredicateComponent,
} from './predicate-fields.component';
import { BrickFieldsComponent } from '../brick-fields.component';

// type -> which dedicated editor renders it: the 12 predicate types collapse
// onto 6 shapes (a matcher is a matcher, whatever it reads).
const KIND: Record<string, string> = {
  path: 'list',
  host: 'list',
  method: 'method',
  header: 'matcher',
  cookie: 'matcher',
  query: 'matcher',
  'remote-addr': 'addr',
  'x-forwarded-remote-addr': 'addr',
  'time-window': 'window',
  weight: 'weight',
  version: 'version',
};

// One predicate in the list: a header (the type + remove; predicates are
// ANDed, so no reorder controls) over the dedicated editor for its type.
@Component({
  selector: 'app-predicate-item',
  imports: [
    MatButtonModule,
    MatIconModule,
    ListPredicateComponent,
    MethodPredicateComponent,
    MatcherPredicateComponent,
    AddrPredicateComponent,
    WindowPredicateComponent,
    WeightPredicateComponent,
    VersionPredicateComponent,
    BrickFieldsComponent,
  ],
  styles: [
    `
      :host {
        display: block;
        border: 1px solid var(--mat-sys-outline-variant);
        border-radius: 10px;
        padding: 10px 12px 4px;
        margin-bottom: 10px;
        background: var(--mat-sys-surface-container-low);
      }
      .head {
        display: flex;
        align-items: center;
        gap: 8px;
        margin-bottom: 4px;
      }
      .doc a {
        color: var(--mat-sys-primary);
        text-decoration: none;
      }
      .doc a:hover {
        text-decoration: underline;
      }
      .doc {
        margin: 0 0 8px;
        font-size: 0.78rem;
        line-height: 1.35;
        color: var(--mat-sys-on-surface-variant);
      }
      .type {
        font-size: 0.9rem;
        font-weight: 500;
      }
      .spacer {
        flex: 1;
      }
      .head button {
        --mat-icon-button-state-layer-size: 32px;
      }
    `,
  ],
  template: `
    <div class="head">
      <span class="type">{{ label() }}</span>
      <span class="spacer"></span>
      <button matIconButton (click)="removed.emit()" i18n-aria-label="@@Remove" aria-label="Remove">
        <mat-icon>close</mat-icon>
      </button>
    </div>
    <!-- The same line the palette showed when this brick was picked. It
         disappeared the moment it was posed, exactly when a reader has the
         thing in front of them and wants to know what its fields do. -->
    @if (doc()) {
      <p class="doc">
        {{ doc() }}
        <!-- The reference closes the explanation, inline, because that is where
             someone finishes reading and wonders about the edge cases. -->
        @if (ref(); as r) {
          <a [href]="r.url" target="_blank" rel="noopener noreferrer">{{ r.label }}</a>
        }
      </p>
    }

    @switch (kind()) {
      @case ('list') {
        <app-list-predicate [spec]="spec()" (specChange)="spec.set($event)" />
      }
      @case ('method') {
        <app-method-predicate [spec]="spec()" (specChange)="spec.set($event)" />
      }
      @case ('matcher') {
        <app-matcher-predicate [spec]="spec()" (specChange)="spec.set($event)" />
      }
      @case ('addr') {
        <app-addr-predicate [spec]="spec()" (specChange)="spec.set($event)" />
      }
      @case ('window') {
        <app-window-predicate [spec]="spec()" (specChange)="spec.set($event)" />
      }
      @case ('weight') {
        <app-weight-predicate [spec]="spec()" (specChange)="spec.set($event)" />
      }
      @case ('version') {
        <app-version-predicate [spec]="spec()" (specChange)="spec.set($event)" />
      }
      @default {
        <!-- Anything without a dedicated editor, rendered from the catalog's
             own description. -->
        <app-brick-fields [spec]="spec()" (specChange)="spec.set($event)" [params]="params()" />
      }
    }
  `,
})
export class PredicateItemComponent {
  readonly spec = model.required<Spec>();
  // What this brick does, in one line, from the catalog (see brick-docs).
  readonly doc = input('');
  // The norm this brick's explanation points at, when it has one.
  readonly ref = input<BrickRef | undefined>(undefined);
  // The catalog's description of THIS brick's arguments, for the generic editor.
  readonly params = input<Param[]>([]);
  readonly removed = output<void>();

  // 'generic' and not 'matcher': falling back on the matcher editor showed a
  // name and a regexp for a brick that has neither - version arrived with two
  // fields it does not own and none of the four it does.
  protected readonly kind = computed(() => KIND[this.spec().type] ?? 'generic');
  protected readonly label = computed(() => humanize(this.spec().type));
}
