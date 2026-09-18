import { Component, computed, inject, input, model, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';
import { CSS_VARS, TOKEN_GROUPS } from '../theme-tokens';

// A palette file is the editor's own state, portable between installs: the two
// colour maps plus the two switches that live in this card. A small version
// marker so a stray JSON is not mistaken for one.
interface PaletteFile {
  meerkatThemePalette: 1;
  pagesScheme: '' | 'light' | 'dark';
  flat: boolean;
  dark: Record<string, string>;
  light: Record<string, string>;
}

const HEX = /^#([0-9a-f]{3}|[0-9a-f]{6}|[0-9a-f]{8})$/i;

// The two palettes of a theme, dark and light side by side, one row per token.
// Hovering a token name tells the page, which highlights the matching elements
// in the preview.
@Component({
  selector: 'app-palette-editor',
  imports: [
    MatButtonModule,
    MatCardModule,
    MatCheckboxModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatTooltipModule,
  ],
  templateUrl: './palette-editor.component.html',
  styleUrl: './palette-editor.component.scss',
})
export class PaletteEditorComponent {
  // Which schemes the built-in pages OFFER (THEME-05). It belongs in this
  // header rather than on the preview: the question "is this scheme offered?"
  // sits right above the colours that answer it, and a column header needs no
  // paragraph to explain what unticking it means.
  readonly pagesScheme = model<'' | 'light' | 'dark'>('');
  protected readonly darkOffered = computed(() => this.pagesScheme() !== 'light');
  protected readonly lightOffered = computed(() => this.pagesScheme() !== 'dark');

  protected offerScheme(scheme: 'dark' | 'light', on: boolean): void {
    this.pagesScheme.set(on ? '' : scheme === 'dark' ? 'light' : 'dark');
  }

  readonly dark = model.required<Record<string, string>>();
  readonly light = model.required<Record<string, string>>();
  // Flat design: dropping every decorative flow-page effect (glows + app-name
  // gradient) at once. Surfaced as a "Glow" checkbox (checked = effects on), so
  // stored inverted. Two-way - the page persists it with the theme.
  readonly flat = model<boolean>(false);
  readonly hoverToken = output<string>();
  readonly save = output<void>();
  readonly saving = input(false);

  protected readonly tokenGroups = TOKEN_GROUPS;

  private readonly snack = inject(MatSnackBar);

  protected setColor(mode: 'dark' | 'light', key: string, value: string): void {
    const target = mode === 'dark' ? this.dark : this.light;
    target.update((m) => ({ ...m, [key]: value.trim().toLowerCase() }));
  }

  // Export the palette as it stands in the editor (unsaved edits included), so a
  // theme can be lifted from one install and dropped into another.
  protected exportPalette(): void {
    const file: PaletteFile = {
      meerkatThemePalette: 1,
      pagesScheme: this.pagesScheme(),
      flat: this.flat(),
      dark: this.dark(),
      light: this.light(),
    };
    const blob = new Blob([JSON.stringify(file, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'theme-palette.json';
    a.click();
    URL.revokeObjectURL(url);
  }

  // Import fills the editor (it does NOT save): the operator reviews the colours
  // in the preview, then saves. Only known tokens with a hex value are kept, so a
  // hand-edited or foreign file cannot smuggle anything in.
  protected importPalette(event: Event): void {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    input.value = ''; // let the same file be picked again
    if (!file) return;
    file
      .text()
      .then((text) => {
        let data: unknown;
        try {
          data = JSON.parse(text);
        } catch {
          this.fail($localize`:@@Theme_import_bad:That file is not valid JSON`);
          return;
        }
        const o = data as Partial<PaletteFile>;
        const dark = this.cleanColors(o.dark);
        const light = this.cleanColors(o.light);
        if (!dark && !light) {
          this.fail($localize`:@@Theme_import_empty:No palette colours found in that file`);
          return;
        }
        if (dark) this.dark.set(dark);
        if (light) this.light.set(light);
        if (typeof o.flat === 'boolean') this.flat.set(o.flat);
        if (o.pagesScheme === '' || o.pagesScheme === 'light' || o.pagesScheme === 'dark') {
          this.pagesScheme.set(o.pagesScheme);
        }
        this.snack.open($localize`:@@Theme_imported:Palette imported - review it, then save`, undefined, {
          duration: 3000,
        });
      })
      .catch(() => this.fail($localize`:@@Theme_import_read:Could not read that file`));
  }

  // Keep only the tokens this editor knows, with a plausible hex value; drop
  // everything else silently. Returns null when nothing survives.
  private cleanColors(value: unknown): Record<string, string> | null {
    if (!value || typeof value !== 'object') return null;
    const out: Record<string, string> = {};
    for (const key of Object.keys(CSS_VARS)) {
      const v = (value as Record<string, unknown>)[key];
      if (typeof v === 'string' && HEX.test(v.trim())) out[key] = v.trim().toLowerCase();
    }
    return Object.keys(out).length ? out : null;
  }

  private fail(message: string): void {
    this.snack.open(message, undefined, { duration: 4000 });
  }
}
