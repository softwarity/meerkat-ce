import { Component, computed, input, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatTooltipModule } from '@angular/material/tooltip';
import { Theme } from '../../api.service';

// Distance between two adjacent pills, as a % of the rail width - enough to
// spread across the width without cropping, but kept tight.
const STEP = 8;

// Fixed px from the centre to the FIRST pill on each side, carving a notch that
// clears the nav block (so the adjacent pills aren't cropped) without widening
// every other gap. Further pills add STEP on top.
const NAV_CLEAR = 112;

// Prefix that keeps a built-in palette's pill distinct from a copy made of it:
// a duplicate inherits the source's id, and an id is how a pill is selected.
export const PRESET_PREFIX = 'preset:';

// The theme picker. An INFINITE carousel: each pill is placed by its shortest
// circular distance from the selected theme (as a % of the width), so stepping
// past the end wraps around. Between the two arrows rides the selected
// palette, and clicking IT is what makes the theme live.
//
// The ring carries the stored themes AND the presets, the latter read-only:
// a preset is code, it cannot be deleted, so the "+" menu whose only job was
// to put a deleted one back has nothing left to do. Duplicating a preset is
// how a palette of your own begins.
//
// Navigating SELECTS, never activates - an arrow that pushed a half-set
// palette to every visitor would be a poor arrow. The cost of that split is
// that "what I am editing" and "what is served" can differ silently, which is
// what the centre pill's warning state exists to say.
@Component({
  selector: 'app-theme-carousel',
  imports: [MatButtonModule, MatIconModule, MatTooltipModule],
  templateUrl: './theme-carousel.component.html',
  styleUrl: './theme-carousel.component.scss',
})
export class ThemeCarouselComponent {
  readonly themes = input.required<Theme[]>();
  readonly presets = input<Theme[]>([]);
  readonly selectedId = input.required<string>();

  readonly pick = output<Theme>();
  readonly activateTheme = output<Theme>();

  protected readonly tipActivate = $localize`:@@Set_active:Not live - click to activate`;
  protected readonly tipActive = $localize`:@@Active_theme:Active theme`;

  // Stored themes first, then EVERY preset, under a namespaced id.
  //
  // They are not deduplicated against the stored ones, and that is deliberate.
  // A copy carries its source's id, so hiding a preset whose id is already
  // stored hid the original the moment anybody duplicated it - rename your
  // copy "Lavender copy" and the built-in Lavender was still nowhere, because
  // renaming does not change an id. The shelf of built-ins is permanent: the
  // original stays on it whatever you have made from it, which is the whole
  // point of having it to duplicate and to compare against.
  //
  // The namespace is what lets the two coexist: an id is the selection key,
  // and a stored copy and its source would otherwise be the same pill.
  protected readonly ring = computed(() => [
    ...this.themes(),
    ...this.presets().map((p) => ({ ...p, id: PRESET_PREFIX + p.id, active: false })),
  ]);

  protected isPreset(t: Theme): boolean {
    return t.id.startsWith(PRESET_PREFIX);
  }

  protected label(t: Theme): string {
    return this.isPreset(t) ? t.name + ' - ' + this.tipPreset : t.name;
  }

  private readonly tipPreset = $localize`:@@Theme_preset:built-in`;

  protected readonly selected = computed(
    () => this.ring().find((t) => t.id === this.selectedId()) ?? null,
  );

  private readonly index = computed(() => {
    const i = this.ring().findIndex((t) => t.id === this.selectedId());
    return i < 0 ? 0 : i;
  });

  // Shortest signed distance of pill i from the selected one, wrapping the ring.
  protected offset(i: number): number {
    const n = this.ring().length;
    if (!n) return 0;
    const raw = (((i - this.index()) % n) + n) % n;
    return raw > n / 2 ? raw - n : raw;
  }

  protected leftFor(o: number): string {
    if (o === 0) return '50%';
    const s = o > 0 ? 1 : -1;
    // First pill sits at a fixed distance (clears the nav); the rest step in %.
    return `calc(50% + ${s * NAV_CLEAR}px + ${(o - s) * STEP}%)`;
  }

  protected prev(): void {
    const n = this.ring().length;
    if (n) this.pick.emit(this.ring()[(this.index() - 1 + n) % n]);
  }

  protected next(): void {
    const n = this.ring().length;
    if (n) this.pick.emit(this.ring()[(this.index() + 1) % n]);
  }
}
