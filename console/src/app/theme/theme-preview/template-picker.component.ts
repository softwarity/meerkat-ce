import { Component, computed, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatDividerModule } from '@angular/material/divider';
import { MatIconModule } from '@angular/material/icon';
import { MatMenuModule } from '@angular/material/menu';
import { MatTooltipModule } from '@angular/material/tooltip';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { PreviewCategory, PreviewTemplate } from '../../api.service';

// What the two panes are showing, above them: "< Account confirmation >".
//
// Arrows rather than a list of categories. There are a handful of templates
// today and there will be thirty, but a reader does not go looking for one -
// they walk the set once and stop where something looks wrong. A menu makes
// that three clicks per template; an arrow makes it one, and the name under
// the thumb says where you are.
@Component({
  selector: 'app-template-picker',
  imports: [
    MatButtonModule,
    MatButtonToggleModule,
    MatDividerModule,
    MatIconModule,
    MatMenuModule,
    MatTooltipModule,
  ],
  template: `
    <div class="picker">
      <span class="mk-float-bar bar">
      <button matIconButton (click)="step(-1)" [disabled]="templates().length < 2"
        i18n-aria-label="@@Previous_template" aria-label="Previous template">
        <mat-icon>chevron_left</mat-icon>
      </button>
      <!-- The name IS the jump list. Walking the set with the arrows is how one
           finds what looks wrong; going back to the one that did is a jump, and
           thirty-six arrow presses is not a way to make one. -->
      <button matButton class="what" [matMenuTriggerFor]="menu" [disabled]="!templates().length">
        <!-- One row of our own rather than three children of the button:
             Material decides where a button's icons go, and with two of them it
             put the caret in front of the name. Here the order is ours. -->
        <span class="inner">
          <!-- The kind, in a glyph: it is what tells a page from a message at
               a glance, here as in the menu below. -->
          <mat-icon class="kind">{{ iconOf(current()) }}</mat-icon>
          <span class="label">{{ current()?.label }}</span>
          @if (current()?.kind === 'mail') {
            <span class="note" matTooltip="E-mails use the light colours only"
              i18n-matTooltip="@@Preview_mail_light_hint" i18n="@@Light_only">light only</span>
          }
          <mat-icon class="caret">arrow_drop_down</mat-icon>
        </span>
      </button>
      <button matIconButton (click)="step(1)" [disabled]="templates().length < 2"
        i18n-aria-label="@@Next_template" aria-label="Next template">
        <mat-icon>chevron_right</mat-icon>
      </button>

      <span class="sep"></span>

      <!-- The filter, in the SAME bar rather than beside it. A filter one
           cannot see is a filter that makes the arrows skip things for no
           visible reason - which is a bug report, not a feature.
           NOTHING LIT IS EVERYTHING SHOWN: one selects what one wants rather
           than switching off what one does not, so the list starts whole, a
           single click narrows it to one subject, and clicking that same
           toggle back off returns the whole list. Selecting every category
           comes to the same as selecting none, which is why no toggle ever has
           to be disabled to stop somebody emptying the list. -->
      <mat-button-toggle-group class="cats" multiple hideSingleSelectionIndicator
        [value]="onKeys()" (change)="filter.emit($event.value)">
        @for (c of categories(); track c.key) {
          <mat-button-toggle [value]="c.key"
            [matTooltip]="c.label" matTooltipPosition="above"
            [attr.aria-label]="c.label">
            <mat-icon>{{ c.icon }}</mat-icon>
          </mat-button-toggle>
        }
      </mat-button-toggle-group>
      </span>
    </div>

    <mat-menu #menu="matMenu" class="tpl-menu">
      @for (t of templates(); track t.key; let i = $index) {
        <!-- One divider where the catalogue changes kind: pages then messages.
             Not a submenu - that would be a click to open and a click to pick,
             for a list one reads in a glance. -->
        @if (i > 0 && t.kind !== templates()[i - 1].kind) {
          <mat-divider />
        }
        <button mat-menu-item (click)="pick.emit(t)" [class.on]="t.key === selected()">
          <mat-icon>{{ t.key === selected() ? 'check' : iconOf(t) }}</mat-icon>
          <span>{{ t.label }}</span>
        </button>
      }
    </mat-menu>
  `,
  styles: [
    `
      /* The row floats ON the preview's top edge - straddling it, the way the
         theme ring straddles the seam between its two panes. A command that
         belongs to what is behind it sits on it, not above it. */
      .picker {
        display: flex;
        justify-content: center;
        padding: 0 24px;
        position: relative;
        z-index: 5;
        margin-bottom: -20px;
        pointer-events: none;
      }
      .picker > * {
        pointer-events: auto;
      }
      /* Full height, flush with the bar's own edges: the filter is a set of
         segments cut out of the bar, not a row of buttons sitting on it. */
      .sep {
        width: 1px;
        align-self: stretch;
        margin: -5px 0 -5px 6px;
        background: var(--mat-sys-outline-variant);
      }
      .cats {
        align-self: stretch;
        /* Cancels the bar's padding on three sides so the segments reach its
           edges; the last one then takes the pill's own corner. */
        margin: -5px -6px -5px 0;
      }
      .cats ::ng-deep .mat-button-toggle:last-child,
      .cats ::ng-deep .mat-button-toggle:last-child .mat-button-toggle-button {
        border-top-right-radius: 999px;
        border-bottom-right-radius: 999px;
      }
      /* The toggles lose their own frame: inside a pill, a bordered group reads
         as a box in a box. Not standard M3, and deliberately - the surface here
         is the bar, and a control that brings its second surface fights it. */
      .cats ::ng-deep .mat-button-toggle,
      .cats {
        border: 0;
        border-radius: 0;
        background: transparent;
      }
      .cats ::ng-deep .mat-button-toggle + .mat-button-toggle {
        border-left: 0;
      }
      .cats ::ng-deep .mat-button-toggle,
      .cats ::ng-deep .mat-button-toggle-button {
        height: 100%;
      }
      .cats ::ng-deep .mat-button-toggle-button {
        padding: 0;
        width: 40px;
      }
      .cats ::ng-deep .mat-button-toggle-label-content {
        display: flex;
        align-items: center;
        justify-content: center;
        height: 100%;
        line-height: 1;
        padding: 0;
      }
      /* A segment says it is live when the pointer is on it: without a frame
         of its own there is nothing else left to say so. */
      .cats ::ng-deep .mat-button-toggle:not(.mat-button-toggle-disabled):hover {
        background: color-mix(in srgb, var(--mat-sys-on-surface) 8%, transparent);
      }
      /* Off is dim, on is the surface colour: the state reads at a glance
         without a second background behind each glyph. */
      .cats ::ng-deep .mat-button-toggle:not(.mat-button-toggle-checked) mat-icon {
        opacity: 0.35;
      }
      .cats ::ng-deep .mat-button-toggle-checked {
        background: transparent;
        color: var(--mat-sys-primary);
      }
      .cats mat-icon {
        font-size: 18px;
        width: 18px;
        height: 18px;
      }
      /* Wide enough that a short name and a long one do not shuffle the arrows
         around: the two chevrons are the thing the hand goes back to.

         And un-cased, through the same token the console uses to case every
         other button: that rule is right for an ACTION label and wrong here,
         where the text is a VALUE - "SIGN IN, REFUSED" is not the name of
         anything. Same reason it wears the surface colour rather than the
         primary: it is what is being shown, not something to click towards. */
      .what {
        min-width: 16rem;
        display: inline-flex;
        align-items: center;
        --mat-button-text-label-text-transform: none;
        --mat-button-text-label-text-color: var(--mat-sys-on-surface);
      }
      .inner {
        display: inline-flex;
        align-items: center;
        gap: 0.4rem;
        min-width: 0;
      }
      .kind,
      .caret {
        flex: 0 0 auto;
        color: var(--mat-sys-on-surface-variant);
      }
      .what .label {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .note {
        padding: 0 0.4rem;
        border-radius: 999px;
        font-size: 0.7rem;
        line-height: 1.3rem;
        background: var(--mat-sys-surface-container-high);
        color: var(--mat-sys-on-surface-variant);
      }
    `,
  ],
})
export class TemplatePickerComponent {
  readonly templates = input.required<PreviewTemplate[]>();
  readonly categories = input<PreviewCategory[]>([]);
  // The categories asked for. Empty is every one of them, not none.
  readonly only = input<Set<string>>(new Set());
  readonly selected = input('');
  readonly pick = output<PreviewTemplate>();
  // The whole selection, not the toggle that moved: the state IS the set, and
  // reconstructing it from a sequence of single changes is how the control and
  // the list drift apart.
  readonly filter = output<string[]>();

  protected readonly onKeys = computed(() => [...this.only()]);

  // A template wears its CATEGORY's glyph, here and in the toggles above. The
  // filter is a row of icons, and an icon that appears nowhere in the list is
  // one nobody can match to anything - you would be filtering by a symbol you
  // had never been shown next to a name.
  protected iconOf(t: PreviewTemplate | null | undefined): string {
    if (!t) return 'web_asset';
    return this.categories().find((c) => c.key === t.category)?.icon ?? 'web_asset';
  }

  protected readonly current = computed(
    // .at(0) rather than [0]: indexing an array is typed as if it always
    // hit, so the compiler read this as never-null and flagged every ?. in
    // the template as pointless - while an empty list really does give
    // undefined here. at() says so, and the guards stay.
    () => this.templates().find((t) => t.key === this.selected()) ?? this.templates().at(0) ?? null,
  );

  protected step(dir: -1 | 1): void {
    const all = this.templates();
    if (all.length < 2) return;
    const i = all.findIndex((t) => t.key === this.selected());
    this.pick.emit(all[(((i < 0 ? 0 : i) + dir + all.length) % all.length)]);
  }
}
