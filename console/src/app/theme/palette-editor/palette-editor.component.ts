import { Component, computed, inject, input, model, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatCardModule } from '@angular/material/card';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatExpansionModule } from '@angular/material/expansion';
import { MatIconModule } from '@angular/material/icon';
import { MatMenuModule } from '@angular/material/menu';
import { MatSelectModule } from '@angular/material/select';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';
import { FileButtonComponent } from '../../shared/file-pick';
import { FontCatalogue, ThemeColors, ThemeFonts } from '../../api.service';
import { Contrast, CoreKey, derivedCore, fromBuilderJson, isHex, toBuilderJson } from '../m3';
import { CORE_ROWS, ROLE_GROUPS, roleLabel } from '../theme-tokens';

// A recipe read from a file, handed to the page: it replaces what is on
// screen and is saved only when the operator says so.
export interface ImportedRecipe {
  colors: ThemeColors;
  colorMatch: boolean;
  contrast?: Contrast;
  fonts?: ThemeFonts;
  flat?: boolean;
  pagesScheme?: '' | 'light' | 'dark';
}

const CONTRASTS: Contrast[] = ['standard', 'medium', 'high'];

// A theme is made the way Material Theme Builder makes one: six source colours
// (the primary required, the others derived until somebody sets them), a
// contrast level, and its "color match". Every role of both schemes follows
// from them - shown below, read-only, dark and light side by side. Hovering a
// colour or a role tells the page, which blinks what it drives in the preview.
@Component({
  selector: 'app-palette-editor',
  imports: [
    MatButtonModule,
    MatButtonToggleModule,
    MatCardModule,
    MatCheckboxModule,
    MatExpansionModule,
    MatIconModule,
    MatMenuModule,
    MatSelectModule,
    MatTooltipModule,
    FileButtonComponent,
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

  readonly colors = model.required<ThemeColors>();
  readonly contrast = model<Contrast>('standard');
  readonly colorMatch = model(false);
  // The typefaces, and the families the gateway ships to choose them from.
  readonly fonts = model<ThemeFonts>({});
  readonly fontCatalogue = input<FontCatalogue['families']>([]);
  protected readonly fontSlots: { key: keyof ThemeFonts; label: string; hint: string; kinds: string[] }[] = [
    { key: 'display', label: $localize`:@@Theme_font_display:Display`, hint: $localize`:@@Theme_font_display_hint:Titles and the application's name`, kinds: ['sans', 'serif'] },
    { key: 'body', label: $localize`:@@Theme_font_body:Body`, hint: $localize`:@@Theme_font_body_hint:Text and buttons`, kinds: ['sans', 'serif'] },
    { key: 'code', label: $localize`:@@Theme_font_code:Code`, hint: $localize`:@@Theme_font_code_hint:Codes, keys, fields and labels`, kinds: ['mono'] },
  ];
  protected readonly systemLabel = $localize`:@@Theme_font_system:System`;
  protected readonly kindLabel: Record<string, string> = {
    sans: $localize`:@@Theme_font_sans:Sans serif`,
    serif: $localize`:@@Theme_font_serif:Serif`,
    mono: $localize`:@@Theme_font_mono:Monospace`,
  };

  protected familiesOf(kind: string) {
    return this.fontCatalogue().filter((f) => f.kind === kind);
  }

  // How a family is shown in a picker: in itself, the system's behind it.
  protected stackOf(family: string | undefined, kind: string): string {
    const system = kind === 'mono' ? 'ui-monospace, monospace' : 'system-ui, sans-serif';
    return family ? `'${family}', ${system}` : system;
  }

  protected setFont(key: keyof ThemeFonts, family: string): void {
    this.fonts.update((f) => {
      const next = { ...f };
      if (family) next[key] = family;
      else delete next[key];
      return next;
    });
  }
  // The schemes the colours make, for the read-only list of roles.
  readonly dark = input.required<Record<string, string>>();
  readonly light = input.required<Record<string, string>>();
  // Flat design: dropping every decorative flow-page effect (glows + app-name
  // gradient) at once. Surfaced as a "Glow" checkbox (checked = effects on), so
  // stored inverted. Two-way - the page persists it with the theme.
  readonly flat = model<boolean>(false);
  // The theme this palette belongs to, named where a column header would read
  // "Token". Two-way, because the pencil renames in place.
  readonly name = model('');
  // A built-in palette is shown and duplicated, never written to: it is code,
  // and there is no row behind it.
  readonly readOnly = input(false);
  // The live theme. The store already refuses to delete it - "activate another
  // theme first" - but a menu item that silently does nothing is worse than
  // one that is plainly out of reach.
  readonly active = input(false);
  protected readonly canDelete = computed(() => !this.readOnly() && !this.active());
  // Something on screen differs from what was loaded. The arrows select as you
  // pass them, so this is the only thing between an edit and its silent loss.
  readonly dirty = input(false);
  protected readonly renaming = signal(false);

  readonly hoverToken = output<string>();
  readonly save = output<void>();
  readonly duplicate = output<void>();
  readonly remove = output<void>();
  readonly imported = output<ImportedRecipe>();
  readonly saving = input(false);

  protected readonly coreRows = CORE_ROWS;
  protected readonly roleGroups = ROLE_GROUPS;
  protected readonly roleLabel = roleLabel;
  protected readonly contrasts = CONTRASTS;
  protected readonly contrastLabel: Record<Contrast, string> = {
    standard: $localize`:@@Theme_contrast_standard:Standard`,
    medium: $localize`:@@Theme_contrast_medium:Medium`,
    high: $localize`:@@Theme_contrast_high:High`,
  };
  // What the builder shows for a colour nobody set: its derived value.
  protected readonly shown = computed(() => {
    const d = derivedCore(this.colors());
    return Object.fromEntries(Object.entries(d).map(([k, v]) => [k, v.toLowerCase()])) as typeof d;
  });
  protected readonly tipReadOnly = $localize`:@@Theme_builtin_hint2:Built-in palette: duplicate it first`;
  protected readonly tipClean = $localize`:@@Theme_nothing_to_save:Nothing changed`;

  private readonly snack = inject(MatSnackBar);

  protected startRename(): void {
    if (!this.readOnly()) this.renaming.set(true);
  }

  protected isSet(key: CoreKey): boolean {
    return key === 'primary' || !!this.colors()[key];
  }

  protected setColor(key: CoreKey, value: string): void {
    const v = value.trim().toLowerCase();
    if (v && !isHex(v)) return;
    this.colors.update((c) => {
      const next = { ...c };
      if (v) next[key] = v;
      else if (key !== 'primary') delete next[key];
      return next;
    });
  }

  // Back to "derived from the primary". The primary has nothing to be derived
  // from, so it has no such button.
  protected unset(key: CoreKey): void {
    if (key !== 'primary') this.setColor(key, '');
  }

  // Export in the builder's own JSON format, so the theme opens wherever a
  // builder export does. Meerkat's switches ride along under a key of their
  // own.
  protected exportTheme(): void {
    const json = toBuilderJson(this.colors(), {
      name: this.name(),
      contrast: this.contrast(),
      fonts: this.fonts(),
      colorMatch: this.colorMatch(),
      flat: this.flat(),
      pagesScheme: this.pagesScheme(),
    });
    const blob = new Blob([json], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = (this.name().trim().toLowerCase().replace(/[^a-z0-9]+/g, '-') || 'theme') + '.json';
    a.click();
    URL.revokeObjectURL(url);
  }

  // Import fills the editor (it does NOT save): the operator reviews the
  // colours in the preview, then saves. A builder export carries the six
  // colours; a palette file of the old editor carries tokens, and its colours
  // are read off them the way a typed theme's are.
  protected importTheme(file: File): void {
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
        const recipe = this.readRecipe(data);
        if (!recipe) {
          this.fail(
            $localize`:@@Theme_import_empty2:No theme found in that file: expected a Material Theme Builder export (coreColors)`,
          );
          return;
        }
        this.imported.emit(recipe.recipe);
        const said = {
          exact: $localize`:@@Theme_imported2:Theme imported - review it, then save`,
          older: $localize`:@@Theme_imported_regenerated:Theme imported from its colours. The file is an older builder export: its contrast rules have changed since, so some roles differ from the file`,
          palette: $localize`:@@Theme_imported_palette:Old palette file: its six colours were read off its tokens - review them, then save`,
        }[recipe.kind];
        this.snack.open(said, undefined, { duration: recipe.kind === 'exact' ? 3000 : 8000 });
      })
      .catch(() => this.fail($localize`:@@Theme_import_read:Could not read that file`));
  }

  private readRecipe(data: unknown): { recipe: ImportedRecipe; kind: 'exact' | 'older' | 'palette' } | null {
    const built = fromBuilderJson(data);
    if (built) {
      const m = built.meerkat;
      const recipe: ImportedRecipe = { colors: built.core, colorMatch: built.colorMatch };
      if (CONTRASTS.includes(m['contrast'] as Contrast)) recipe.contrast = m['contrast'] as Contrast;
      // Only families this gateway ships: a name from another one, or a
      // typo, would be a font the pages cannot draw.
      const f = m['fonts'] as Record<string, unknown> | undefined;
      if (f && typeof f === 'object') {
        const known = new Set(this.fontCatalogue().map((c) => c.family));
        const fonts: ThemeFonts = {};
        for (const k of ['display', 'body', 'code'] as const) {
          if (typeof f[k] === 'string' && known.has(f[k] as string)) fonts[k] = f[k] as string;
        }
        recipe.fonts = fonts;
      }
      if (typeof m['flat'] === 'boolean') recipe.flat = m['flat'];
      if (m['pagesScheme'] === '' || m['pagesScheme'] === 'light' || m['pagesScheme'] === 'dark') {
        recipe.pagesScheme = m['pagesScheme'];
      }
      return { recipe, kind: built.exact ? 'exact' : 'older' };
    }
    // The old editor's own file: two maps of tokens, read as the colours that
    // make them - the rule the gateway applies (store.ColorsFromTokens).
    const o = data as { dark?: Record<string, unknown>; light?: Record<string, unknown>; flat?: unknown };
    const light = o?.light ?? {};
    const dark = o?.dark ?? {};
    const first = (...vs: unknown[]) => (vs.find(isHex) as string | undefined)?.toLowerCase();
    const primary = first(dark['primary'], light['primary']);
    if (!primary) return null;
    const colors: ThemeColors = { primary };
    const neutral = first(dark['surface'], light['surface']);
    const error = first(light['error'], dark['error']);
    const variant = first(dark['onSurfaceVariant'], light['onSurfaceVariant']);
    if (neutral) colors.neutral = neutral;
    if (error) colors.error = error;
    if (variant) colors.neutralVariant = variant;
    const recipe: ImportedRecipe = { colors, colorMatch: true };
    if (typeof o.flat === 'boolean') recipe.flat = o.flat;
    return { recipe, kind: 'palette' };
  }

  private fail(message: string): void {
    this.snack.open(message, undefined, { duration: 6000 });
  }
}
