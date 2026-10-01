import { Component, inject } from '@angular/core';
import { MatDialog } from '@angular/material/dialog';
import { MatTabsModule } from '@angular/material/tabs';
import { toSignal } from '@angular/core/rxjs-interop';
import { NavigationEnd, Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { filter, map, startWith } from 'rxjs';
import { LoadingIndicatorComponent } from '@softwarity/loading-indicator';
import { ThemeCarouselComponent } from '../theme-carousel/theme-carousel.component';
import { TemplatePickerComponent } from '../theme-preview/template-picker.component';
import { LocalePickerComponent } from '../theme-preview/locale-picker.component';
import { ThemePreviewComponent } from '../theme-preview/theme-preview.component';
import { AddLocaleDialogComponent, AddLocaleResult } from './add-locale-dialog.component';
import { BuiltInPagesScope } from './built-in-pages.scope';
import { ChangeRow, LiveChangesService } from '../../shared/live-changes.service';

// The pages Meerkat serves, on ONE screen (THEME-02/04/06, PAGE-02).
//
// Colours, arrangement and identity were two screens and would have been
// three: each with options on the left and the same preview on the right,
// each showing a page the other two also decide. They are one subject - what
// the visitor sees - so they are one entry with three tabs, and the tabs are
// ONLY on the left. The preview never moves, and the theme carousel stays
// under it on all three: trying a colour while judging an arrangement is the
// normal way round, not a special case.
//
// The tabs are ROUTED (mat-tab-nav-bar): a bookmark on the layout gallery must
// come back to the layout gallery, like every other deep link in this console.
@Component({
  selector: 'app-built-in-pages',
  providers: [BuiltInPagesScope],
  imports: [
    MatTabsModule,
    RouterLink,
    RouterLinkActive,
    RouterOutlet,
    LoadingIndicatorComponent,
    ThemePreviewComponent,
    ThemeCarouselComponent,
    TemplatePickerComponent,
    LocalePickerComponent,
  ],
  templateUrl: './built-in-pages.component.html',
  styleUrl: './built-in-pages.component.scss',
})
export class BuiltInPagesComponent {
  private readonly router = inject(Router);
  // On the Locale tab the ring picks a language: the panes are showing one
  // language's page, and the palette is not what is being edited there.
  protected readonly onLocaleTab = toSignal(
    this.router.events.pipe(
      filter((e) => e instanceof NavigationEnd),
      map(() => this.router.url.endsWith('/locale')),
      startWith(this.router.url.endsWith('/locale')),
    ),
    { initialValue: false },
  );

  protected readonly scope = inject(BuiltInPagesScope);

  constructor() {
    // Somebody else's write (CONSOLE-13). OFFERED and not applied: these tabs
    // hold a palette, a page layout and a catalogue of strings being edited,
    // and reloading them under somebody working would throw that away for news
    // they did not ask for.
    const live = inject(LiveChangesService);
    const reload = (change: ChangeRow) =>
      live.offer(change, () => {
        this.scope.loadThemes(true);
        this.scope.reloadLocales();
      });
    live.on('theme', reload);
    live.on('locale', reload);
  }
  private readonly dialog = inject(MatDialog);

  // The bar shows the languages; adding one is the shell's business, because
  // the list and the language on screen both live in the scope.
  protected addLocale(): void {
    this.dialog
      .open(AddLocaleDialogComponent, {
        width: '30rem',
        data: { existing: this.scope.locales() },
      })
      .afterClosed()
      .subscribe((r: AddLocaleResult | undefined) => {
        if (r) this.scope.addLocale(r.code, r.from);
      });
  }
}
