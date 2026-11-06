import { Component, inject } from '@angular/core';
import { MatCardModule } from '@angular/material/card';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatInputModule } from '@angular/material/input';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { EeLockComponent } from '../../shared/ee-lock.component';
import { BrandingCardComponent } from '../branding-card/branding-card.component';
import { BuiltInPagesScope } from './built-in-pages.scope';

// The Branding tab (THEME-02/06): the application's identity - name, tagline,
// logo, tab icon, background - plus the Meerkat mark, which is a licence
// question rather than an identity one and keeps its own card.
@Component({
  selector: 'app-branding-tab',
  imports: [MatCardModule, MatFormFieldModule, MatInputModule, MatSlideToggleModule, EeLockComponent, BrandingCardComponent],
  styles: [
    `
      /* See layout-tab: the panel's gap lands on this host, not between the
         cards inside it, so the host carries it. */
      :host {
        display: grid;
        gap: 16px;
        align-content: start;
      }
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
      .wide {
        width: 100%;
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
      [(backgroundBoth)]="scope.backgroundBoth"
      [(backgroundDark)]="scope.backgroundDark"
      [(backgroundFitDark)]="scope.backgroundFitDark"
      [(backgroundDimDark)]="scope.backgroundDimDark"
      (changed)="scope.brandingChanged()"
    />

    <!-- The pages that tell a person to ask an administrator (refused, the
         waiting room, an application's refused sign-in) link to this. -->
    <mat-card appearance="outlined" class="mark-card">
      <div class="title" i18n="@@Administrator_contact">Administrator contact</div>
      <p class="hint text-muted" i18n="@@Administrator_contact_hint">
        Offered on the pages that tell someone to ask an administrator. An e-mail address or a web address.
      </p>
      <mat-form-field class="wide" subscriptSizing="dynamic">
        <mat-label i18n="@@Email_or_web_address">E-mail or web address</mat-label>
        <input
          matInput
          [value]="scope.contact()"
          (change)="scope.setContact($any($event.target).value)"
          placeholder="it@acme.example"
        />
      </mat-form-field>
    </mat-card>

    <mat-card appearance="outlined" class="mark-card" ee-feature="white-label">
      <div class="title">
        <ng-container i18n="@@Meerkat_mark">Meerkat mark</ng-container>
        <app-ee-lock
          feature="white-label"
          i18n-why="@@White_label_ee_why"
          why="Show only your own brand on these pages."
        />
      </div>
      <p class="hint text-muted" i18n="@@Meerkat_mark_hint">
        Pages show "powered by softwarity/meerkat" at the bottom.
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
