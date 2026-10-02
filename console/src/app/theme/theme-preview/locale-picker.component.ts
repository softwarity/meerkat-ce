import { Component, computed, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatDividerModule } from '@angular/material/divider';
import { MatIconModule } from '@angular/material/icon';
import { MatMenuModule } from '@angular/material/menu';
import { MatTooltipModule } from '@angular/material/tooltip';
import { LocaleView } from '../../api.service';
import { LanguageRowComponent } from '../../shared/language-row.component';

// Which language the two panes speak, in the ring's place on the Locale tab
// (I18N-05).
//
// The same shape as the theme ring it replaces - arrows either side, the value
// between them, a jump list under it - because it is the same gesture: walk
// the set, stop where something looks wrong. What changes is what is being
// chosen, and on this tab the palette is not it.
//
// Each language shows the count of strings it has no wording for. That number
// is the reason somebody opens this tab, so it belongs where the choice is
// made rather than two clicks further in.
@Component({
  selector: 'app-locale-picker',
  imports: [
    MatButtonModule,
    MatDividerModule,
    MatIconModule,
    MatMenuModule,
    MatTooltipModule,
    LanguageRowComponent,
  ],
  template: `
    <div class="nav mk-float-bar">
      <button matIconButton (click)="step(-1)" [disabled]="locales().length < 2"
        i18n-aria-label="@@Previous_language" aria-label="Previous language">
        <mat-icon>chevron_left</mat-icon>
      </button>
      <button matButton class="what" [matMenuTriggerFor]="menu" [disabled]="!locales().length">
        <span class="inner">
          <span class="code">{{ selected() }}</span>
          <span class="name">{{ current()?.name }}</span>
          @if (current()?.holes) {
            <span class="holes"
              matTooltip="Untranslated strings, shown in English"
              i18n-matTooltip="@@Locale_holes_hint3">{{ current()?.holes }}</span>
          }
          @if (current()?.edited) {
            <mat-icon class="edited" matTooltip="This installation changed wordings here"
              i18n-matTooltip="@@Locale_edited_hint">edit</mat-icon>
          }
          <mat-icon class="caret">arrow_drop_down</mat-icon>
        </span>
      </button>
      <button matIconButton (click)="step(1)" [disabled]="locales().length < 2"
        i18n-aria-label="@@Next_language" aria-label="Next language">
        <mat-icon>chevron_right</mat-icon>
      </button>
      <!-- A language the binary does not ship. Here rather than in a menu
           because this bar IS the list of languages: the place one looks to
           find one is the place to add one. -->
      <span class="sep"></span>
      <button matIconButton (click)="add.emit()"
        matTooltip="Add a language" i18n-matTooltip="@@Add_a_language"
        i18n-aria-label="@@Add_a_language" aria-label="Add a language">
        <mat-icon>add</mat-icon>
      </button>
    </div>

    <mat-menu #menu="matMenu">
      @for (l of locales(); track l.code; let i = $index) {
        <!-- One divider where the product's own languages end and the ones
             added here begin: they are not the same kind of thing, and only
             the second can be deleted. -->
        @if (i > 0 && l.embedded !== locales()[i - 1].embedded) {
          <mat-divider />
        }
        <button mat-menu-item (click)="pick.emit(l.code)" [class.on]="l.code === selected()">
          <app-language-row [locale]="l" />
        </button>
      }
    </mat-menu>
  `,
  styles: [
    `
      /* Centred and no wider than what it holds. The theme ring below spreads
         across the width because it IS a ring one walks; this is a single
         control, and a control stretched to the width of a preview reads as a
         toolbar for the page behind it. */
      :host {
        display: flex;
        justify-content: center;
        padding: 0 24px;
        /* The band is as wide as the preview, so its empty sides must not eat
           the clicks meant for what is underneath. */
        pointer-events: none;
      }
      :host > * {
        pointer-events: auto;
      }
      /* Full height and flush with the bar's edge, like the template filter's:
         a divider inside a pill is a seam, not a border. */
      .sep {
        width: 1px;
        align-self: stretch;
        margin: -5px 2px -5px 4px;
        background: var(--mat-sys-outline-variant);
      }
      /* The surface itself is .mk-float-bar, shared with the template picker
         and the theme ring: they are the same kind of control. */
      /* Un-cased and in the surface colour: the text is a VALUE, not an action
         label, which is the same reason the template picker overrides it. */
      .what {
        min-width: 13rem;
        --mat-button-text-label-text-transform: none;
        --mat-button-text-label-text-color: var(--mat-sys-on-surface);
      }
      .inner {
        display: inline-flex;
        align-items: center;
        gap: 0.4rem;
        min-width: 0;
      }
      .code {
        font-weight: 600;
      }
      .name {
        color: var(--mat-sys-on-surface-variant);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .holes {
        padding: 0 0.4rem;
        border-radius: 999px;
        font-size: 0.7rem;
        line-height: 1.3rem;
        background: var(--mat-sys-error-container);
        color: var(--mat-sys-on-error-container);
      }
      .edited,
      .caret {
        flex: 0 0 auto;
        color: var(--mat-sys-on-surface-variant);
      }
      .edited {
        font-size: 16px;
        width: 16px;
        height: 16px;
      }
    `,
  ],
})
export class LocalePickerComponent {
  readonly locales = input.required<LocaleView[]>();
  readonly selected = input('en');
  readonly pick = output<string>();
  // Add a language. The dialog belongs to whoever owns the list, not to the
  // bar that shows it.
  readonly add = output<void>();

  protected readonly current = computed(
    // .at(0) rather than [0] - see the same note in the template picker: an
    // index is typed as if it always hit, which made every ?. below look
    // useless to the compiler and none of them useless at runtime.
    () => this.locales().find((l) => l.code === this.selected()) ?? this.locales().at(0) ?? null,
  );

  protected step(dir: -1 | 1): void {
    const all = this.locales();
    if (all.length < 2) return;
    const i = all.findIndex((l) => l.code === this.selected());
    this.pick.emit(all[((i < 0 ? 0 : i) + dir + all.length) % all.length].code);
  }
}
