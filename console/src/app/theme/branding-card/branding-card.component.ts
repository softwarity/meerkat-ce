import { Component, computed, inject, model, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatSliderModule } from '@angular/material/slider';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';

type BackgroundFit = 'cover' | 'contain' | 'tile';

const ACCEPTED = ['image/png', 'image/jpeg', 'image/webp', 'image/svg+xml'];

// An icon may also be an .ico, which is what most people already have when
// they think "favicon". Browsers report it under either of these two types.
const ACCEPTED_ICON = [
  ...ACCEPTED,
  'image/x-icon',
  'image/vnd.microsoft.icon',
];

// Global application identity (THEME-02): name, tagline, and the logo as a
// drop zone whose empty state IS the flow pages' generic placeholder mark.
// Two-way model signals - the page owns persistence.
@Component({
  selector: 'app-branding-card',
  imports: [
    MatButtonModule,
    MatCardModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatSelectModule,
    MatSliderModule,
    MatTooltipModule,
  ],
  templateUrl: './branding-card.component.html',
  styleUrl: './branding-card.component.scss',
})
export class BrandingCardComponent {
  readonly appName = model.required<string>();
  readonly tagline = model.required<string>();
  readonly logo = model.required<string>();
  readonly favicon = model.required<string>();
  // The background of the built-in pages (THEME-06): the picture, how it meets
  // a screen it was not cut for, and how much surface colour is laid over it.
  readonly background = model.required<string>();
  readonly backgroundFit = model.required<BackgroundFit>();
  readonly backgroundDim = model.required<number>();
  readonly changed = output<void>();

  protected readonly dragging = signal(false);
  protected readonly draggingIcon = signal(false);
  protected readonly draggingBg = signal(false);
  protected readonly fits: { value: BackgroundFit; label: string }[] = [
    { value: 'cover', label: $localize`:@@Fit_cover:Cover` },
    { value: 'contain', label: $localize`:@@Fit_contain:Contain` },
    { value: 'tile', label: $localize`:@@Fit_tile:Tile` },
  ];
  // What the tab will actually show, which is the cascade the gateway applies
  // when it serves /meerkat/favicon: the icon, else the logo, else nothing -
  // and "nothing" is where Meerkat's own mark takes over.
  protected readonly tabIcon = computed(() => this.favicon() || this.logo());

  private readonly snack = inject(MatSnackBar);

  protected onLogoFile(ev: Event): void {
    this.readFile((ev.target as HTMLInputElement).files?.[0]);
  }

  protected onDrop(ev: DragEvent): void {
    ev.preventDefault();
    this.dragging.set(false);
    this.readFile(ev.dataTransfer?.files?.[0]);
  }

  protected onIconDrop(ev: DragEvent): void {
    ev.preventDefault();
    this.draggingIcon.set(false);
    this.read(ev.dataTransfer?.files?.[0], ACCEPTED_ICON, 40_000, 'icon', this.favicon);
  }

  // A full-screen picture, so a wider budget than a logo - but it is fetched
  // once from /meerkat/background and cached, never inlined in a page.
  protected onBackgroundFile(ev: Event): void {
    const input = ev.target as HTMLInputElement;
    this.read(input.files?.[0], ACCEPTED, 1_000_000, 'background', this.bgTarget);
    input.value = '';
  }

  protected onBackgroundDrop(ev: DragEvent): void {
    ev.preventDefault();
    this.draggingBg.set(false);
    this.read(ev.dataTransfer?.files?.[0], ACCEPTED, 1_000_000, 'background', this.bgTarget);
  }

  // The first picture arrives with a veil already on it. At zero, a photograph
  // with a bright corner leaves the wordmark and the card unreadable in one
  // scheme or the other, and the obvious next move would be to edit the image
  // rather than move a slider nobody noticed. A value the admin has chosen -
  // even zero, once they have touched it - is never overridden.
  private readonly bgTarget = {
    set: (value: string) => {
      const first = !this.background();
      this.background.set(value);
      if (value && first && this.backgroundDim() === 0) this.backgroundDim.set(35);
    },
  };

  protected onFaviconFile(ev: Event): void {
    const input = ev.target as HTMLInputElement;
    // A 32-pixel square: past 40 KiB it is a photo someone picked by mistake,
    // and it would be fetched by every sign-in page.
    this.read(input.files?.[0], ACCEPTED_ICON, 40_000, 'icon', this.favicon);
    input.value = ''; // re-picking the same file must fire change again
  }

  private readFile(file: File | undefined): void {
    this.read(file, ACCEPTED, 200_000, 'logo', this.logo);
  }

  private read(
    file: File | undefined,
    accepted: string[],
    maxBytes: number,
    what: string,
    target: { set(value: string): void },
  ): void {
    if (!file) return;
    if (!accepted.includes(file.type)) {
      const types = accepted.includes('image/x-icon') ? 'png, svg, ico or webp' : 'png, jpeg, webp or svg';
      this.snack.open(`Use a ${types} image`, undefined, { duration: 3000 });
      return;
    }
    if (file.size > maxBytes) {
      this.snack.open(`Keep the ${what} under ${Math.round(maxBytes / 1024)} KiB`, undefined, { duration: 3000 });
      return;
    }
    const reader = new FileReader();
    reader.onload = () => {
      target.set(String(reader.result));
      this.changed.emit();
    };
    reader.readAsDataURL(file);
  }
}
