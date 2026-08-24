import { Component, computed, input, model, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { BrickRef, Param, Spec } from '../../api.service';
import { humanize } from '../predicates/args';
import {
  HeaderFilterComponent,
  MaintenanceFilterComponent,
  RespondFilterComponent,
  PrefixPathFilterComponent,
  RedirectFilterComponent,
  RemoveHeaderFilterComponent,
  RemoveQueryParamFilterComponent,
  RewritePathFilterComponent,
  SetQueryParamFilterComponent,
  SetStatusFilterComponent,
  StripPrefixFilterComponent,
} from './filter-fields.component';
import { BrickFieldsComponent } from '../brick-fields.component';

// type -> which dedicated editor renders it. Several types share an editor
// (a header value is a header value, whichever verb/phase), so the map collapses
// the 14 filter types onto the dedicated components.
const KIND: Record<string, string> = {
  'add-request-header': 'header',
  'set-request-header': 'header',
  'add-response-header': 'header',
  'set-response-header': 'header',
  'remove-request-header': 'remove-header',
  'remove-response-header': 'remove-header',
  'set-query-param': 'set-query',
  'remove-query-param': 'remove-query',
  'strip-prefix': 'strip-prefix',
  'prefix-path': 'prefix-path',
  'rewrite-path': 'rewrite-path',
  'set-status': 'set-status',
  redirect: 'redirect',
  maintenance: 'maintenance',
  respond: 'respond',
};

// One filter in the ordered list: a header (the type, its phase, reorder and
// remove controls) over the dedicated editor for its type.
@Component({
  selector: 'app-filter-item',
  imports: [
    MatButtonModule,
    MatIconModule,
    HeaderFilterComponent,
    RemoveHeaderFilterComponent,
    SetQueryParamFilterComponent,
    RemoveQueryParamFilterComponent,
    StripPrefixFilterComponent,
    PrefixPathFilterComponent,
    RewritePathFilterComponent,
      SetStatusFilterComponent,
    RedirectFilterComponent,
    MaintenanceFilterComponent,
    RespondFilterComponent,
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
      .phase {
        font-size: 0.62rem;
        letter-spacing: 0.08em;
        text-transform: uppercase;
        padding: 2px 7px;
        border-radius: 999px;
        color: var(--mat-sys-on-secondary-container);
        background: color-mix(in srgb, var(--mat-sys-secondary-container) 60%, transparent);
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
      @if (phase()) {
        <span class="phase">{{ phaseLabel() }}</span>
      }
      <span class="spacer"></span>
      <button matIconButton [disabled]="first()" (click)="moveUp.emit()" i18n-aria-label="@@Move_up" aria-label="Move up">
        <mat-icon>arrow_upward</mat-icon>
      </button>
      <button matIconButton [disabled]="last()" (click)="moveDown.emit()" i18n-aria-label="@@Move_down" aria-label="Move down">
        <mat-icon>arrow_downward</mat-icon>
      </button>
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
      @case ('header') {
        <app-header-filter [spec]="spec()" (specChange)="spec.set($event)" />
      }
      @case ('remove-header') {
        <app-remove-header-filter [spec]="spec()" (specChange)="spec.set($event)" />
      }
      @case ('set-query') {
        <app-set-query-param-filter [spec]="spec()" (specChange)="spec.set($event)" />
      }
      @case ('remove-query') {
        <app-remove-query-param-filter [spec]="spec()" (specChange)="spec.set($event)" />
      }
      @case ('strip-prefix') {
        <app-strip-prefix-filter [spec]="spec()" (specChange)="spec.set($event)" />
      }
      @case ('prefix-path') {
        <app-prefix-path-filter [spec]="spec()" (specChange)="spec.set($event)" />
      }
      @case ('rewrite-path') {
        <app-rewrite-path-filter [spec]="spec()" (specChange)="spec.set($event)" />
      }
      @case ('set-status') {
        <app-set-status-filter [spec]="spec()" (specChange)="spec.set($event)" />
      }
      @case ('redirect') {
        <app-redirect-filter [spec]="spec()" (specChange)="spec.set($event)" />
      }
      @case ('maintenance') {
        <app-maintenance-filter [spec]="spec()" (specChange)="spec.set($event)" />
      }
      @case ('respond') {
        <app-respond-filter [spec]="spec()" (specChange)="spec.set($event)" />
      }
      @default {
        <!-- Everything without a dedicated editor, rendered from the catalog's
             own description of the brick. Without this case the card showed a
             title and no field at all. -->
        <app-brick-fields [spec]="spec()" (specChange)="spec.set($event)" [params]="params()" />
      }
    }
  `,
})
export class FilterItemComponent {
  readonly spec = model.required<Spec>();
  // What this brick does, in one line, from the catalog (see brick-docs).
  readonly doc = input('');
  // The norm this brick's explanation points at, when it has one.
  readonly ref = input<BrickRef | undefined>(undefined);
  // The catalog's description of THIS brick's arguments, for the generic editor.
  readonly params = input<Param[]>([]);
  readonly phase = input('');
  readonly first = input(false);
  readonly last = input(false);
  readonly removed = output<void>();
  readonly moveUp = output<void>();
  readonly moveDown = output<void>();

  protected readonly kind = computed(() => KIND[this.spec().type] ?? 'generic');
  protected readonly label = computed(() => humanize(this.spec().type));
  protected readonly phaseLabel = computed(() => {
    switch (this.phase()) {
      case 'request':
        return $localize`:@@incoming:incoming`;
      case 'response':
        return $localize`:@@outgoing:outgoing`;
      case 'terminal':
        return $localize`:@@terminal:terminal`;
      default:
        return humanize(this.phase());
    }
  });
}
