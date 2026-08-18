import { Component, inject } from '@angular/core';
import { MatCardModule } from '@angular/material/card';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { EeLockComponent } from '../../shared/ee-lock.component';
import { BrandingCardComponent } from '../branding-card/branding-card.component';
import { BuiltInPagesScope } from './built-in-pages.scope';

// The Branding tab (THEME-02/06): the application's identity - name, tagline,
// logo, tab icon, background - plus the Meerkat mark, which is a licence
// question rather than an identity one and keeps its own card.
@Component({
  selector: 'app-branding-tab',
  imports: [MatCardModule, MatSlideToggleModule, EeLockComponent, BrandingCardComponent],
  styles: [
    `
      .mark-card {
        padding: 16px 20px;
      }
      .mark-card .title {
        display: flex;
        align-items: center;
        gap: 8px;
        font-size: 0.95rem;
        font-weight: 500;
      }
      .mark-card .hint {
        margin: 6px 0 12px;
        font-size: 0.8rem;
        line-height: 1.45;
      }
    `,
  ],
  template: `
    <app-branding-card
      [(appName)]="scope.appName"
      [(tagline)]="scope.tagline"
      [(logo)]="scope.logo"
      [(favicon)]="scope.favicon"
      [(background)]="scope.background"
      [(backgroundFit)]="scope.backgroundFit"
      [(backgroundDim)]="scope.backgroundDim"
      (changed)="scope.brandingChanged()"
    />

    <mat-card appearance="outlined" class="mark-card" ee-feature="white-label">
      <div class="title">
        <ng-container i18n="@@Meerkat_mark">Meerkat mark</ng-container>
        <app-ee-lock
          feature="white-label"
          i18n-why="@@White_label_ee_why"
          why="Serve these pages under your own name alone."
        />
      </div>
      <p class="hint text-muted" i18n="@@Meerkat_mark_hint">
        The pages this gateway serves carry a "powered by softwarity/meerkat" line at the foot.
      </p>
      <mat-slide-toggle
        [checked]="scope.hideMark()"
        (change)="scope.hideMark.set($event.checked); scope.brandingChanged()"
      >
        <ng-container i18n="@@Remove_the_mark">Remove the mark</ng-container>
      </mat-slide-toggle>
    </mat-card>
  `,
})
export class BrandingTabComponent {
  protected readonly scope = inject(BuiltInPagesScope);
}
