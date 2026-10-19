import { DatePipe } from '@angular/common';
import { Component, computed, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatTooltipModule } from '@angular/material/tooltip';
import { RouterLink } from '@angular/router';
import { LoadingIndicatorComponent } from '@softwarity/loading-indicator';
import { ApiService, Edition } from '../api.service';
import { MeService } from '../me.service';
import { EE_FEATURES } from './ee-features';

// What this installation IS, in one place - and the only screen that talks
// about editions at all. Everywhere else, an Enterprise control simply carries
// its [Enterprise] cap and links here; repeating the pitch on ten screens
// would turn the console into an advert.
//
// Read-only but for one thing: the license, which root pastes here. The mode
// is not switched from here: it is switched where organisations are
// administered, because that is where someone can see what it costs them.
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
  imports: [
    DatePipe,
    MatButtonModule,
    MatCardModule,
    MatIconModule,
    MatTooltipModule,
    LoadingIndicatorComponent,
    RouterLink,
  ],
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
                @if (e.notice) {
                  <ng-container i18n="@@Evaluation_edition">Evaluation edition</ng-container>
                } @else if (e.enterprise) {
                  <ng-container i18n="@@Enterprise_edition">Enterprise edition</ng-container>
                } @else {
                  <ng-container i18n="@@Community_edition">Community edition</ng-container>
                }
              </div>
              <!-- The perpetual promise is for the images one BUYS. Printing
                   it on an evaluation build would promise the opposite of what
                   that build is licensed for, so the notice takes its place -
                   and it is the notice itself, straight from the binary, not a
                   second sentence that could drift from it. -->
              @if (e.notice) {
                <p class="hint">{{ e.notice }}</p>
              } @else {
                <p class="hint" i18n="@@License_perpetual_hint">
                  A license is perpetual: its term only covers updates, nothing gets switched off.
                </p>
              }
              @if (e.license; as l) {
                <dl class="license">
                  <dt i18n="@@Licensee">Licensee</dt>
                  <dd>{{ l.licensee }}</dd>
                  @if (l.id) {
                    <dt i18n="@@License_id">License</dt>
                    <dd>{{ l.id }}</dd>
                  }
                  <dt i18n="@@Updates_until">Updates until</dt>
                  <dd>
                    {{ l.updatesUntil | date: 'longDate' }}
                    @if (l.covered === false) {
                      <span class="late" i18n="@@Build_not_covered">this build was released after it</span>
                    }
                  </dd>
                </dl>
              }
              @if (e.licenseProblem) {
                <p class="problem">{{ e.licenseProblem }}</p>
              }
            </div>
            <!-- Root's, and only on the image a license applies to. A license
                 the deployment mounts is changed where it is mounted. -->
            @if (e.enterprise && me.isRoot()) {
              <button
                matButton="tonal"
                type="button"
                [disabled]="e.licenseSource === 'file'"
                [matTooltip]="e.licenseSource === 'file' ? fromFile : ''"
                (click)="change()"
                i18n="@@Change_license"
              >
                Change license
              </button>
            }
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
          <!-- How far each one is built, said once here and then by its icon:
               a word on every row read as a list of excuses. -->
          <p class="legend">
            @for (s of states; track s.status) {
              <span><mat-icon>{{ s.icon }}</mat-icon>{{ s.label }}</span>
            }
          </p>
          <div class="features">
            @for (f of rows(); track f.key) {
              <div class="feature" [class.on]="f.on">
                <mat-icon [matTooltip]="stateOf(f).label">{{ stateOf(f).icon }}</mat-icon>
                <div class="grow">
                  <div class="name">{{ f.label }}</div>
                  <div class="what">
                    {{ f.what }}
                    @if (f.where) {
                      <a [routerLink]="f.where">{{ f.whereLabel }}</a>
                    } @else if (f.status !== 'planned') {
                      <span class="noscreen" i18n="@@Feature_no_screen">No screen: it is in the image.</span>
                    }
                  </div>
                </div>
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
      .license {
        display: grid;
        grid-template-columns: max-content 1fr;
        gap: 2px 12px;
        margin: 12px 0 0;
        font-size: 0.85rem;
      }
      .license dt {
        color: var(--mat-sys-on-surface-variant);
      }
      .license dd {
        margin: 0;
      }
      .late {
        margin-left: 6px;
        color: var(--mat-sys-on-surface-variant);
        font-style: italic;
      }
      .problem {
        margin: 8px 0 0;
        font-size: 0.85rem;
        color: var(--mat-sys-error);
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
      .legend {
        display: flex;
        flex-wrap: wrap;
        gap: 4px 16px;
        margin: 0 0 12px;
        font-size: 0.8rem;
        color: var(--mat-sys-on-surface-variant);
      }
      .legend span {
        display: inline-flex;
        align-items: center;
        gap: 4px;
      }
      .legend mat-icon {
        font-size: 16px;
        width: 16px;
        height: 16px;
      }
    `,
  ],
})
export class LicensePageComponent {
  private readonly api = inject(ApiService);
  private readonly dialog = inject(MatDialog);
  protected readonly me = inject(MeService);

  protected readonly fromFile = $localize`:@@License_from_file:Set by the deployment (MEERKAT_LICENSE_FILE): change it where it is mounted.`;

  protected readonly loading = signal(true);
  protected readonly edition = signal<Edition | null>(null);
  protected readonly hidden = computed(() => this.edition()?.hiddenTenants ?? 0);

  // How far a feature is built, as an icon. The row's ID (AUTH-03...) is the
  // contract's key, not a name anyone reading this screen knows it by.
  protected readonly states: { status: FeatureRow['status']; icon: string; label: string }[] = [
    { status: 'done', icon: 'check_circle', label: $localize`:@@Feature_done:built` },
    { status: 'partial', icon: 'incomplete_circle', label: $localize`:@@Feature_partial:partly built` },
    { status: 'planned', icon: 'schedule', label: $localize`:@@Feature_planned:planned` },
  ];
  protected stateOf(f: FeatureRow) {
    return this.states.find((s) => s.status === f.status) ?? this.states[0];
  }

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

  protected async change(): Promise<void> {
    const { LicenseDialogComponent } = await import('./license-dialog.component');
    this.dialog
      .open(LicenseDialogComponent, { width: '640px' })
      .afterClosed()
      .subscribe((e: Edition | undefined) => {
        if (e) this.edition.set(e);
      });
  }

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
