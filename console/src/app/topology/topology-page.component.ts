import { NgTemplateOutlet } from '@angular/common';
import { Component, computed, inject } from '@angular/core';
import { MatCardModule } from '@angular/material/card';
import { MatIconModule } from '@angular/material/icon';
import { RouterLink } from '@angular/router';
import { LoadingIndicatorComponent } from '@softwarity/loading-indicator';
import { InfraService } from '../api.service';
import { InfrastructureService } from '../shared/infrastructure.service';
import { ZonedDatePipe, zoned } from '../shared/zoned-date.pipe';
import { MeService } from '../me.service';

// What serves the installation and what it serves, drawn. On the left the
// Meerkat instances, in sync with each other, each wired to the database below
// them; on the right the services the routes point at, the ones that want a
// look first, scrolling on their own when there are many. A view and nothing
// else: the routes are edited on their own screen. Read-only,
// and nothing here counts time: it follows the live channel (instances,
// route-health, route changes) and the hours shown are the events' own.

// The drawing's grid, in pixels: a card's height and the gap that holds the
// sync link, the fieldset's head and foot, where the cards sit across.
const CARD = 112;
const GAP = 30;
const HEAD = 40;
const FOOT_SYNCED = 56;
const FOOT_ALONE = 24;
const CARD_X = 70;
const CARD_W = 280;
const LEFT_W = 380;
const DB_GAP = 30;
const DB_H = 96;
const BAR_X = 396;
const PANEL_X = 420;

@Component({
  selector: 'app-topology-page',
  imports: [
    ZonedDatePipe,
    NgTemplateOutlet,
    MatCardModule,
    MatIconModule,
    RouterLink,
    LoadingIndicatorComponent,
  ],
  template: `
    <div class="banner">
      <h1 i18n="@@Topology">Topology</h1>
    </div>

    @if (infra.state(); as s) {
      <div class="content">
        @if (s.over) {
          <mat-card appearance="outlined" class="warn" role="note">
            <div class="title">
              <ng-container i18n="@@Infra_over">{{ s.instances.length }} instances are running and this {{ planName() }} license covers {{ s.covers }}</ng-container>
            </div>
            <p i18n="@@Infra_over_hint">
              Traffic is not touched. From {{ sec(s.over.noticeAt) | zoned: zone() : 'long' }} the evaluation notice is shown,
              until they are {{ s.covers }} again or the license covers them. A rolling update that ends before then is not an overrun.
            </p>
            <a routerLink="/system/license" i18n="@@See_the_license">See the license</a>
          </mat-card>
        }
        @if (s.versions.length > 1) {
          <mat-card appearance="outlined" class="warn" role="note">
            <div class="title" i18n="@@Infra_versions">{{ s.versions.length }} versions are running: {{ s.versions.join(', ') }}</div>
            <p i18n="@@Infra_versions_hint">An update in progress, or one that stopped half way. Each instance below says which it runs.</p>
          </mat-card>
        }
        @for (n of notReady(); track n.id) {
          <mat-card appearance="outlined" class="error" role="note">
            <div class="title" i18n="@@Infra_not_ready">{{ n.host }} is not ready</div>
            <p>{{ n.reason }}</p>
          </mat-card>
        }

        <div class="tiles">
          <mat-card appearance="outlined" [class.flag]="!!s.over || s.versions.length > 1">
            <div class="label" i18n="@@Instances">Instances</div>
            <div class="value" i18n="@@Infra_running_version">{{ s.instances.length }} running, {{ s.versions.join(' and ') }}</div>
            <div class="label">{{ coverage() }}</div>
          </mat-card>
          <mat-card appearance="outlined">
            <div class="label" i18n="@@Database">Database</div>
            <div class="value">{{ s.database.kind === 'postgres' ? 'PostgreSQL' : embedded }}</div>
            <div class="label">
              @if (s.synced) {
                <ng-container i18n="@@Infra_db_shared">Shared, schema {{ s.database.schema }}</ng-container>
              } @else {
                <ng-container i18n="@@Infra_db_file">A file in the data directory, schema {{ s.database.schema }}</ng-container>
              }
            </div>
          </mat-card>
          <mat-card appearance="outlined" [class.bad]="down() > 0">
            <div class="label" i18n="@@Services">Services</div>
            <div class="value" i18n="@@Infra_services_count">{{ s.services.length }}, for {{ routeCount() }} routes</div>
            <div class="label" [class.bad-text]="down() > 0">
              @if (down() > 0) {
                <ng-container i18n="@@Infra_services_down">{{ down() }} not answering</ng-container>
              } @else {
                <ng-container i18n="@@Infra_services_fine">All answering, as far as this node can tell</ng-container>
              }
            </div>
          </mat-card>
        </div>

        <mat-card appearance="outlined" class="diagram" aria-label="Diagram" i18n-aria-label="@@Diagram">
          <div class="topo" [style.min-height.px]="g().height">
            @for (l of g().lanes; track $index) {
              <div class="line db-line" [style.left.px]="l.x" [style.top.px]="l.y" [style.width.px]="2" [style.height.px]="l.h"></div>
              <div class="line db-line" [style.left.px]="l.x" [style.top.px]="l.y - 1" [style.width.px]="cardX - l.x" [style.height.px]="2"></div>
            }

            <fieldset class="meerkat" [style.width.px]="leftW" [style.height.px]="g().fieldset">
              <legend i18n="@@Meerkat">Meerkat</legend>
            </fieldset>

            @for (n of s.instances; track n.id; let i = $index) {
              <div class="card node" [class.self]="n.self" [class.extra]="extra(n.id)" [class.down]="!n.ready"
                [style.left.px]="cardX" [style.top.px]="g().tops[i]" [style.width.px]="cardW" [style.height.px]="card">
                <div class="row">
                  <mat-icon svgIcon="meerkat" class="kind" aria-hidden="true" />
                  <span class="mono host" [title]="n.host">{{ n.host }}</span>
                </div>
                <div class="chips">
                  @if (n.ready) {
                    <span class="chip ok" i18n="@@Ready">Ready</span>
                  } @else {
                    <span class="chip bad" i18n="@@Not_ready">Not ready</span>
                  }
                  @if (n.self) {
                    <span class="chip self" i18n="@@This_node">This node</span>
                  }
                  @if (extra(n.id)) {
                    <span class="chip warn" i18n="@@Past_the_license">Past the license</span>
                  }
                </div>
                <div class="facts label">
                  <span i18n="@@Version">Version</span><span class="mono">{{ n.version }}</span>
                  <span i18n="@@Started">Started</span><span>{{ sec(n.started) | zoned: zone() : 'long' }}</span>
                </div>
              </div>
            }

            @if (s.synced) {
              @for (y of g().syncs; track $index) {
                <div class="sync-line" [style.left.px]="cardX + cardW / 2 - 1" [style.top.px]="y" [style.height.px]="gap"></div>
                <div class="sync-icon" [style.left.px]="cardX + cardW / 2 - 11" [style.top.px]="y + 4">
                  <mat-icon aria-hidden="true">sync</mat-icon>
                </div>
                <div class="sync-label label" [style.left.px]="cardX + cardW / 2 + 20" [style.top.px]="y + 7" i18n="@@in_sync">in sync</div>
              }
              <div class="foot label" [style.left.px]="cardX" [style.top.px]="g().foot" [style.width.px]="cardW" i18n="@@Infra_sync_what">
                Routes, certificates, sessions and the license reach every instance within a second.
              </div>
            }

            <div class="card db" [style.top.px]="g().db" [style.width.px]="leftW" [style.min-height.px]="dbH">
              <div class="row">
                <mat-icon class="kind" aria-hidden="true">database</mat-icon>
                <span class="title">{{ s.database.kind === 'postgres' ? 'PostgreSQL' : embeddedDb }}</span>
              </div>
              <div class="mono label">{{ s.database.location }}</div>
              <div class="label">
                @if (s.synced) {
                  <ng-container i18n="@@Infra_db_held_by">Schema {{ s.database.schema }}, shared by the {{ s.instances.length }} instances</ng-container>
                } @else if (s.enterprise) {
                  <ng-container i18n="@@Infra_db_alone_ee">Schema {{ s.database.schema }}. Several instances share a PostgreSQL database instead.</ng-container>
                } @else {
                  <ng-container i18n="@@Infra_db_alone">Schema {{ s.database.schema }}, held by this instance alone</ng-container>
                }
              </div>
            </div>

            @if (s.services.length) {
              @for (y of g().centers; track $index) {
                <div class="line svc-line" [style.left.px]="cardX + cardW" [style.top.px]="y - 1" [style.width.px]="barX - cardX - cardW" [style.height.px]="2"></div>
              }
              <div class="line svc-line" [style.left.px]="barX" [style.top.px]="g().centers[0]" [style.width.px]="2"
                [style.height.px]="g().centers[g().centers.length - 1] - g().centers[0] + 1"></div>
              <div class="line svc-line" [style.left.px]="barX" [style.top.px]="g().middle - 1" [style.width.px]="panelX - barX" [style.height.px]="2"></div>
            }

            <section class="services" [style.left.px]="panelX"
              aria-label="Services" i18n-aria-label="@@Services">
              <div class="services-head">
                <span class="title" i18n="@@Services">Services</span>
                <span class="label" i18n="@@Infra_n_services">{{ s.services.length }} services</span>
              </div>
              <div class="services-list">
                @for (v of s.services; track v.address) {
                  <div [class]="'card svc ' + v.state">
                    <ng-container *ngTemplateOutlet="svcBody; context: { $implicit: v }" />
                  </div>
                } @empty {
                  <p class="label" i18n="@@Infra_no_services">No route points at a service yet.</p>
                }
              </div>
            </section>
          </div>
        </mat-card>

      </div>
    } @else {
      <loading-indicator withContainer />
    }

    <ng-template #svcBody let-v>
      <span class="row between">
        <span class="row">
          <mat-icon [class]="'kind ' + v.state" aria-hidden="true">{{ v.ui ? 'web' : 'api' }}</mat-icon>
          <span class="name" [title]="routeNames(v)">{{ routeNames(v) }}</span>
        </span>
        <span [class]="'chip ' + v.state">{{ stateLabel(v) }}</span>
      </span>
      <span class="mono label line2" [title]="v.address">{{ v.address }}</span>
      @if (v.state === 'down') {
        <span class="label line2 bad-text">{{ detail(v) }}</span>
      }
    </ng-template>

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
        padding: 12px 24px;
        flex: none;
      }
      .banner h1 {
        font-size: 1.15rem;
        font-weight: 500;
        margin: 0;
      }
      .content {
        flex: 1 1 auto;
        min-height: 0;
        overflow-y: auto;
        padding: 0 24px 24px;
        display: flex;
        flex-direction: column;
        gap: 16px;
      }
      mat-card {
        padding: 14px 18px;
      }
      .title {
        font-weight: 500;
      }
      p {
        margin: 4px 0 0;
      }
      .warn {
        background: var(--mat-sys-tertiary-container);
        color: var(--mat-sys-on-tertiary-container);
      }
      .error {
        background: var(--mat-sys-error-container);
        color: var(--mat-sys-on-error-container);
      }
      .tiles {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
        gap: 16px;
      }
      .tiles .flag {
        border-color: var(--mat-sys-tertiary);
      }
      .tiles .bad {
        border-color: var(--mat-sys-error);
      }
      .label {
        font: var(--mat-sys-body-small);
        color: var(--mat-sys-on-surface-variant);
      }
      .bad-text {
        color: var(--mat-sys-error);
      }
      .value {
        font: var(--mat-sys-title-large);
        margin: 4px 0;
      }
      .mono {
        font-family: var(--mat-sys-code-font, monospace);
      }
      .diagram {
        flex: 1 0 auto;
        display: flex;
        flex-direction: column;
        overflow-x: auto;
        padding: 20px 24px;
      }
      .topo {
        flex: 1 1 auto;
        position: relative;
        min-width: 860px;
      }
      .topo > * {
        position: absolute;
        box-sizing: border-box;
      }
      .line {
        background: var(--mat-sys-outline);
      }
      .line.db-line {
        background: var(--mat-sys-primary);
      }
      fieldset.meerkat {
        left: 0;
        top: 10px;
        margin: 0;
        padding: 0;
        border: 1px solid var(--mat-sys-outline);
        border-radius: 14px;
      }
      fieldset.meerkat legend {
        margin-left: 72px;
        padding: 0 8px;
        font: var(--mat-sys-title-small);
      }
      .card {
        box-sizing: border-box;
        border: 1px solid var(--mat-sys-outline-variant);
        border-radius: 12px;
        background: var(--mat-sys-surface-container-low);
        padding: 10px 12px;
        display: flex;
        flex-direction: column;
        gap: 6px;
        color: inherit;
      }
      .node.self {
        border: 2px solid var(--mat-sys-primary);
      }
      .node.extra {
        border: 2px dashed var(--mat-sys-tertiary);
      }
      .node.down {
        border: 2px solid var(--mat-sys-error);
      }
      .db {
        left: 0;
        gap: 4px;
      }
      .row {
        display: flex;
        align-items: center;
        gap: 8px;
        min-width: 0;
      }
      .row.between {
        justify-content: space-between;
      }
      .kind {
        flex: none;
        width: 20px;
        height: 20px;
        font-size: 20px;
        color: var(--mat-sys-primary);
      }
      .kind.degraded {
        color: var(--mat-sys-tertiary);
      }
      .kind.down {
        color: var(--mat-sys-error);
      }
      .kind.unknown {
        color: var(--mat-sys-on-surface-variant);
      }
      .host {
        font-size: 13px;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .facts {
        display: grid;
        grid-template-columns: auto 1fr;
        column-gap: 8px;
      }
      .facts .mono {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .chips {
        display: flex;
        flex-wrap: wrap;
        gap: 6px;
      }
      .chip {
        flex: none;
        font: var(--mat-sys-label-small);
        padding: 2px 8px;
        border-radius: 10px;
        background: var(--mat-sys-surface-container-highest);
      }
      .chip.self {
        background: var(--mat-sys-primary-container);
        color: var(--mat-sys-on-primary-container);
      }
      .chip.warn,
      .chip.degraded {
        background: var(--mat-sys-tertiary-container);
        color: var(--mat-sys-on-tertiary-container);
      }
      .chip.bad,
      .chip.down {
        background: var(--mat-sys-error-container);
        color: var(--mat-sys-on-error-container);
      }
      .sync-line {
        width: 0;
        border-left: 2px dashed var(--mat-sys-tertiary);
      }
      .sync-icon {
        width: 22px;
        height: 22px;
        border-radius: 11px;
        border: 1px solid var(--mat-sys-tertiary);
        background: var(--mat-sys-surface);
        display: grid;
        place-items: center;
      }
      .sync-icon mat-icon {
        width: 14px;
        height: 14px;
        font-size: 14px;
        color: var(--mat-sys-tertiary);
        animation: turn 3s linear infinite;
      }
      @keyframes turn {
        to {
          transform: rotate(-360deg);
        }
      }
      @media (prefers-reduced-motion: reduce) {
        .sync-icon mat-icon {
          animation: none;
        }
      }
      .sync-label {
        color: var(--mat-sys-tertiary);
      }
      .foot {
        text-align: center;
      }
      .services {
        top: 10px;
        bottom: 10px;
        right: 0;
        border: 1px solid var(--mat-sys-outline);
        border-radius: 14px;
        display: flex;
        flex-direction: column;
      }
      .services-head {
        display: flex;
        justify-content: space-between;
        align-items: baseline;
        padding: 10px 16px 8px;
      }
      .services-list {
        flex: 1 1 auto;
        min-height: 0;
        overflow-y: auto;
        padding: 0 16px 14px;
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
        gap: 10px;
        align-content: start;
      }
      .svc {
        position: static;
        min-height: 62px;
        gap: 4px;
      }
      .svc.degraded {
        border-color: var(--mat-sys-tertiary);
      }
      .svc.down {
        border: 2px solid var(--mat-sys-error);
      }
      .name {
        font: var(--mat-sys-title-small);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .line2 {
        font-size: 12px;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
    `,
  ],
})
export class TopologyPageComponent {
  protected readonly infra = inject(InfrastructureService);
  // The zone this operator reads dates in: their profile's, else the browser's.
  protected readonly zone = inject(MeService).timezone;
  protected readonly embedded = $localize`:@@Embedded:Embedded`;
  protected readonly embeddedDb = $localize`:@@Embedded_database:Embedded database`;
  protected readonly card = CARD;
  protected readonly gap = GAP;
  protected readonly cardX = CARD_X;
  protected readonly cardW = CARD_W;
  protected readonly leftW = LEFT_W;
  protected readonly dbH = DB_H;
  protected readonly barX = BAR_X;
  protected readonly panelX = PANEL_X;

  // Where everything sits, from how many instances there are: each card's top
  // and centre, the sync links between them, a lane per instance down to the
  // database - each its own, so no line crosses a card or another line.
  protected readonly g = computed(() => {
    const s = this.infra.state();
    const n = Math.max(1, s?.instances.length ?? 1);
    const synced = !!s?.synced;
    const top = 10;
    const tops = Array.from({ length: n }, (_, i) => top + HEAD + i * (CARD + GAP));
    const centers = tops.map((t) => t + CARD / 2);
    const fieldset = HEAD + n * CARD + (n - 1) * GAP + (synced ? FOOT_SYNCED : FOOT_ALONE);
    const db = top + fieldset + DB_GAP;
    const step = n > 1 ? Math.min(14, 42 / (n - 1)) : 0;
    const lanes = centers.map((c, i) => {
      const x = 18 + i * step;
      return { x, y: c, h: db - c };
    });
    return {
      tops,
      centers,
      middle: (centers[0] + centers[n - 1]) / 2,
      syncs: tops.slice(0, -1).map((t) => t + CARD),
      fieldset,
      foot: tops[n - 1] + CARD + 12,
      db,
      lanes,
      height: Math.max(db + DB_H + 10, 420),
    };
  });

  protected readonly planName = computed(() => {
    const p = this.infra.state()?.plan;
    return p === 'team' ? 'Team' : p === 'enterprise' ? 'Enterprise' : '';
  });

  protected readonly notReady = computed(() => (this.infra.state()?.instances ?? []).filter((n) => !n.ready));
  protected readonly down = computed(() => (this.infra.state()?.services ?? []).filter((v) => v.state === 'down').length);
  protected readonly routeCount = computed(() =>
    (this.infra.state()?.services ?? []).reduce((total, v) => total + v.routes.length, 0),
  );

  // What the license says of the number, in the tile.
  protected readonly coverage = computed(() => {
    const s = this.infra.state();
    if (!s) return '';
    if (!s.enterprise) return $localize`:@@Infra_community:Community edition: one instance`;
    if (!s.plan) return $localize`:@@Infra_evaluation:Evaluation: no license`;
    if (!s.covers) return $localize`:@@Infra_no_ceiling:${this.planName()}:plan: license, no instance ceiling`;
    return $localize`:@@Infra_covered:${this.planName()}:plan: license, ${s.covers}:covers: covered`;
  });

  protected stateLabel(v: InfraService): string {
    switch (v.state) {
      case 'up':
        return $localize`:@@Answering:Answering`;
      case 'down':
        return $localize`:@@Not_answering:Not answering`;
      case 'degraded':
        return v.wanted
          ? $localize`:@@Replicas_ready:${v.ready}:ready: of ${v.wanted}:wanted: ready`
          : $localize`:@@Degraded:Degraded`;
      default:
        return $localize`:@@Not_checked_yet:Not checked yet`;
    }
  }

  // An ISO time from the API, as the unix seconds the zoned pipe takes.
  protected sec(iso: string): number {
    return Math.floor(Date.parse(iso) / 1000);
  }

  // What a person knows a service by: the routes that lead to it.
  protected routeNames(v: InfraService): string {
    return v.routes.map((r) => r.name).join(', ');
  }

  // Why a service does not answer, and since when.
  protected detail(v: InfraService): string {
    const since = zoned(v.at, this.zone(), 'long');
    const why = v.why || $localize`:@@Infra_unreachable:unreachable`;
    return since ? $localize`:@@Infra_down_since:${why}:why: since ${since}:since:` : why;
  }

  // The instances past what the license covers: the newest ones, since the
  // oldest were there first.
  protected extra(id: string): boolean {
    const s = this.infra.state();
    if (!s?.over || !s.covers) return false;
    return s.instances.findIndex((n) => n.id === id) >= s.covers;
  }

  constructor() {
    // Fresh on arrival: the badge's copy may be from before a word was lost.
    this.infra.read();
  }
}
