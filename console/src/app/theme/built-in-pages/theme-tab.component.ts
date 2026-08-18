import { Component, inject } from '@angular/core';
import { PaletteEditorComponent } from '../palette-editor/palette-editor.component';
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
        [(dark)]="scope.dark"
        [(light)]="scope.light"
        [(flat)]="scope.flat"
        [pagesScheme]="scope.pagesScheme()"
        (pagesSchemeChange)="scope.setPagesScheme($event)"
        [saving]="scope.saving()"
        (hoverToken)="scope.hover($event)"
        (save)="scope.saveTheme()"
      />
    }
  `,
})
export class ThemeTabComponent {
  protected readonly scope = inject(BuiltInPagesScope);
}
