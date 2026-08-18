import { Component, inject } from '@angular/core';
import { MatTabsModule } from '@angular/material/tabs';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { LoadingIndicatorComponent } from '@softwarity/loading-indicator';
import { ThemeCarouselComponent } from '../theme-carousel/theme-carousel.component';
import { ThemePreviewComponent } from '../theme-preview/theme-preview.component';
import { BuiltInPagesScope } from './built-in-pages.scope';

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
  ],
  templateUrl: './built-in-pages.component.html',
  styleUrl: './built-in-pages.component.scss',
})
export class BuiltInPagesComponent {
  protected readonly scope = inject(BuiltInPagesScope);
}
