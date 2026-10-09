import { Component, inject } from '@angular/core';
import { ImportedRecipe, PaletteEditorComponent } from '../palette-editor/palette-editor.component';
import { BuiltInPagesScope } from './built-in-pages.scope';

// The Theme tab: the palette editor, and nothing else. Everything it needs -
// the themes, the selected copy, the save - belongs to the scope, because the
// preview beside it and the two other tabs read the same state.
@Component({
  selector: 'app-theme-tab',
  imports: [PaletteEditorComponent],
  template: `
    @if (scope.selected()) {
      <app-palette-editor
        [(colors)]="scope.colors"
        [(contrast)]="scope.contrast"
        [(colorMatch)]="scope.colorMatch"
        [(fonts)]="scope.fonts"
        [fontCatalogue]="scope.fontCatalogue()"
        [dark]="scope.dark()"
        [light]="scope.light()"
        [(flat)]="scope.flat"
        [(name)]="scope.name"
        [readOnly]="scope.readOnly()"
        [active]="!!scope.selected()?.active"
        [dirty]="scope.dirty()"
        [pagesScheme]="scope.pagesScheme()"
        (pagesSchemeChange)="scope.setPagesScheme($event)"
        [saving]="scope.saving()"
        (hoverToken)="scope.hover($event)"
        (save)="scope.saveTheme()"
        (duplicate)="scope.createFrom()"
        (remove)="scope.removeSelected()"
        (imported)="imported($event)"
      />
    }
  `,
})
export class ThemeTabComponent {
  protected readonly scope = inject(BuiltInPagesScope);

  // A file's switches land where they live: the recipe on the scope, the
  // offered schemes in the settings, written on the spot like their checkbox.
  protected imported(r: ImportedRecipe): void {
    this.scope.loadRecipe(r.colors, r.colorMatch, r.contrast, r.fonts);
    if (r.flat !== undefined) this.scope.flat.set(r.flat);
    if (r.pagesScheme !== undefined && r.pagesScheme !== this.scope.pagesScheme()) {
      this.scope.setPagesScheme(r.pagesScheme);
    }
  }
}
