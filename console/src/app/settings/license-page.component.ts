import { Component, computed, inject, signal } from '@angular/core';
import { MatCardModule } from '@angular/material/card';
import { MatIconModule } from '@angular/material/icon';
import { MatTooltipModule } from '@angular/material/tooltip';
import { RouterLink } from '@angular/router';
import { LoadingIndicatorComponent } from '@softwarity/loading-indicator';
import { ApiService, Edition } from '../api.service';
import { EE_FEATURES } from './ee-features';

// What this installation IS, in one place - and the only screen that talks
// about editions at all. Everywhere else, an Enterprise control simply carries
// its [Enterprise] cap and links here; repeating the pitch on ten screens
// would turn the console into an advert.
//
// Read-only on purpose. The mode is not switched from here either: it is
// switched where organisations are administered, because that is where someone
// can see what it costs them.
interface FeatureRow {
  key: string;
  label: string;
  what: string;
  on: boolean;
  status: 'done' | 'partial' | 'planned';
  where?: string;
  whereLabel?: string;
}

@Component({
  selector: 'app-license-page',
  imports: [MatCardModule, MatIconModule, MatTooltipModule, LoadingIndicatorComponent, RouterLink],
  template: `
    <div class="banner">
      <h1 i18n="@@License">License</h1>
    </div>

    @if (loading()) {
      <loading-indicator withContainer />
    } @else if (edition(); as e) {
      <div class="content">
        <mat-card appearance="outlined" class="edition">
          <div class="line">
            <mat-icon>{{ e.enterprise ? 'workspace_premium' : 'public' }}</mat-icon>
            <div class="grow">
              <div class="title">
                @if (e.enterprise) {
                  <ng-container i18n="@@Enterprise_edition">Enterprise edition</ng-container>
                } @else {
                  <ng-container i18n="@@Community_edition">Community edition</ng-container>
                }
              </div>
              <p class="hint" i18n="@@License_perpetual_hint">
                A license is perpetual: its term only covers updates, nothing gets switched off.
              </p>
            </div>
          </div>
        </mat-card>

        @if (hidden() > 0) {
          <mat-card appearance="outlined" class="warn">
            <div class="line">
              <mat-icon>visibility_off</mat-icon>
              <div class="grow">
                <div class="title" i18n="@@Organisations_held_back">
                  {{ hidden() }} organisations are not being served
                </div>
                <p class="hint" i18n="@@Organisations_held_back_hint">
                  Single-organisation mode serves only the first. Nothing is deleted.
                </p>
              </div>
            </div>
          </mat-card>
        }

        <mat-card appearance="outlined">
          <h3>
            @if (e.enterprise) {
              <ng-container i18n="@@EE_features_bought">What the Enterprise edition carries</ng-container>
            } @else {
              <ng-container i18n="@@EE_features_offer">What the Enterprise edition adds</ng-container>
            }
          </h3>
          <div class="features">
            @for (f of rows(); track f.key) {
              <div class="feature" [class.on]="f.on">
                <mat-icon>{{ f.status === 'planned' ? 'schedule' : f.on ? 'check_circle' : 'lock' }}</mat-icon>
                <div class="grow">
                  <div class="name">
                    {{ f.label }}
                    @if (f.status === 'partial') {
                      <span class="state" i18n="@@Feature_partial">partly built</span>
                    } @else if (f.status === 'planned') {
                      <span class="state" i18n="@@Feature_planned">planned</span>
                    }
                  </div>
                  <div class="what">
                    {{ f.what }}
                    @if (f.where) {
                      <a [routerLink]="f.where">{{ f.whereLabel }}</a>
                    } @else if (f.status !== 'planned') {
                      <span class="noscreen" i18n="@@Feature_no_screen">No screen: it is in the image.</span>
                    }
                  </div>
                </div>
                <span class="id">{{ f.key }}</span>
              </div>
            }
          </div>
        </mat-card>
      </div>
    }
  `,
  styles: [
    `
      :host {
        display: flex;
        flex-direction: column;
        height: 100%;
        min-height: 0;
      }
      .banner {
        display: flex;
        align-items: center;
        gap: 16px;
        padding: 12px 24px;
        flex: none;
      }
      .banner h1 {
        font-size: 1.15rem;
        font-weight: 500;
        margin: 0;
        flex: 1;
      }
      .content {
        flex: 1 1 auto;
        min-height: 0;
        overflow-y: auto;
        padding: 0 24px 24px;
        display: grid;
        gap: 16px;
        max-width: 860px;
      }
      mat-card {
        padding: 20px 24px;
      }
      .line {
        display: flex;
        align-items: flex-start;
        gap: 16px;
      }
      .line > mat-icon {
        flex-shrink: 0;
        color: var(--mat-sys-on-surface-variant);
      }
      .edition .line > mat-icon {
        color: var(--mat-sys-tertiary);
      }
      .warn {
        border-color: var(--mat-sys-error);
      }
      .warn .line > mat-icon {
        color: var(--mat-sys-error);
      }
      .grow {
        flex: 1;
        min-width: 0;
      }
      .title {
        font-weight: 500;
      }
      h3 {
        margin: 0 0 12px;
        font-size: 1rem;
        font-weight: 500;
      }
      .hint {
        margin: 4px 0 0;
        font-size: 0.85rem;
        color: var(--mat-sys-on-surface-variant);
      }
      .features {
        display: grid;
        gap: 10px;
      }
      .feature {
        display: flex;
        align-items: flex-start;
        gap: 12px;
        opacity: 0.6;
      }
      .feature.on {
        opacity: 1;
      }
      .feature mat-icon {
        flex-shrink: 0;
        font-size: 20px;
        width: 20px;
        height: 20px;
        color: var(--mat-sys-outline);
      }
      .feature.on mat-icon {
        color: var(--mk-signal);
      }
      .name {
        font-weight: 500;
        font-size: 0.9rem;
      }
      .what {
        font-size: 0.82rem;
        color: var(--mat-sys-on-surface-variant);
      }
      .what a {
        margin-left: 6px;
        color: var(--mat-sys-primary);
      }
      .noscreen {
        margin-left: 6px;
        font-style: italic;
      }
      .state {
        margin-left: 8px;
        padding: 0 6px;
        border-radius: 8px;
        font-size: 0.72rem;
        font-weight: 400;
        color: var(--mat-sys-on-surface-variant);
        border: 1px solid var(--mat-sys-outline-variant);
      }
      .id {
        flex-shrink: 0;
        font-family: var(--mk-mono, monospace);
        font-size: 0.72rem;
        color: var(--mat-sys-outline);
      }
    `,
  ],
})
export class LicensePageComponent {
  private readonly api = inject(ApiService);

  protected readonly loading = signal(true);
  protected readonly edition = signal<Edition | null>(null);
  protected readonly hidden = computed(() => this.edition()?.hiddenTenants ?? 0);

  // The Enterprise rows of FEATURES.md, sent with the edition (CONSOLE-14),
  // named and placed by ee-features.ts. A row on the CE image is what the
  // Enterprise one would add; on the Enterprise image, what it carries. A
  // planned row is said to be one, rather than sold.
  protected readonly rows = computed<FeatureRow[]>(() => {
    const e = this.edition();
    if (!e) return [];
    return (e.features ?? [])
      .filter((f) => f.status !== 'retired')
      .map((f) => {
        const copy = EE_FEATURES[f.id];
        return {
          key: f.id,
          label: copy?.label ?? f.id,
          what: copy?.what ?? '',
          where: copy?.where,
          whereLabel: copy?.whereLabel,
          status: f.status as FeatureRow['status'],
          on: e.enterprise && f.status !== 'planned',
        };
      });
  });

  constructor() {
    this.api.edition().subscribe({
      next: (e) => {
        this.edition.set(e);
        this.loading.set(false);
      },
      error: () => this.loading.set(false),
    });
  }
}
