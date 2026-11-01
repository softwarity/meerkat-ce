import { Component, computed, effect, inject, signal } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { map } from 'rxjs/operators';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatTabsModule } from '@angular/material/tabs';
import { MatSidenavModule } from '@angular/material/sidenav';
import { ActivatedRoute, Router, RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';
import { EeLockComponent } from '../../shared/ee-lock.component';
import { GitLocationsPanelComponent } from './git-locations-panel.component';
import { ConfigurationYamlComponent } from './configuration-yaml.component';
import { ApiService, SavedConfiguration } from '../../api.service';
import { MatSnackBar } from '@angular/material/snack-bar';

// Configuration - INFRA plane, root only, because everything under it crosses
// both planes at once: routes belong to infra, authorities and settings to the
// application, and whoever moves a configuration administers both.
//
// Three tabs, because three questions were sharing one page and only the first
// two were even about the same object:
//
//   Management     the configurations: what is running on the first row, the
//                  saved ones under it, and the export/import of both through
//                  the dialogs that say what leaves and what would change.
//   History        a restore point per change, to go back to any moment of
//                  this gateway's configuration (CFG-06).
//   Database       the whole DATABASE, accounts and sessions and audit
//                  included: backed up, restored, moved to PostgreSQL and
//                  back. A different artifact for a different day: a
//                  configuration reproduces this gateway elsewhere, a
//                  snapshot restores this one.
//
// There WAS an Import/export tab. Everything it held has a better place: the
// export report is a sentence before a download, the plan and the vault holes
// belong to the import that produces them, and the pruning checkbox became one
// of three destinations. A tab holding nothing of its own is a tab.
//
// Routed tabs (mat-tab-nav-bar), like the built-in pages: a bookmark on the
// database procedure must come back to the database procedure.
@Component({
  selector: 'app-configuration-page',
  imports: [
    MatButtonModule,
    MatIconModule,
    MatSidenavModule,
    MatTabsModule,
    EeLockComponent,
    GitLocationsPanelComponent,
    ConfigurationYamlComponent,
    RouterLink,
    RouterLinkActive,
    RouterOutlet,
  ],
  styles: [
    `
      /* The screen owns the height: the heading and the tabs stay put, the
         panel underneath scrolls. That is what lets the Compose tab hold a
         drawer at all - a drawer inside a 900px text column is a drawer with
         nowhere to open. The reading width lives on the TABS instead, each
         setting its own. */
      :host {
        display: block;
        height: 100%;
        overflow: hidden;
      }
      /* The drawer is the PAGE's, not a tab's, and that is what gives it the
         whole height: the git locations are global to this screen - the same
         repositories whichever tab is open - so hanging them off Management
         would have cost them the heading and the tab bar for nothing. */
      mat-drawer-container {
        background: transparent;
        height: 100%;
      }
      mat-drawer-content {
        display: flex;
        flex-direction: column;
        padding: 24px 24px 0;
        box-sizing: border-box;
        overflow: hidden;
      }
      mat-drawer {
        width: min(760px, 96vw);
        border-left: 1px solid var(--mat-sys-outline-variant);
        background: var(--mat-sys-surface-container-high);
      }
      .title {
        display: flex;
        align-items: flex-start;
        gap: 16px;
      }
      .title .grow {
        flex: 1;
        min-width: 0;
      }
      h1 {
        margin-top: 0;
      }
      h1,
      .hint {
        max-width: 900px;
      }
      .hint {
        margin: 0 0 16px;
        font-size: 0.85rem;
        color: var(--mat-sys-on-surface-variant);
      }
      nav {
        flex: none;
      }
      mat-tab-nav-panel {
        display: block;
        flex: 1;
        min-height: 0;
        overflow: auto;
      }
    `,
  ],
  template: `
    <mat-drawer-container>
      <mat-drawer-content>
        <div class="title">
          <div class="grow">
            <h1 i18n="@@Configuration">Configuration</h1>
            <p class="hint" i18n="@@Configuration_intro">
              Routes, roles, authorities, OIDC clients, tenants and their groups, mail relay,
              themes and gateway settings, in one file. Users, memberships, sessions and the vault
              are not part of it.
            </p>
          </div>
          <!-- Above the tabs because it belongs to none of them: the same
               repositories answer whichever tab is open. -->
          <button matButton ee-feature="configurations" (click)="openGit()">
            <mat-icon>hub</mat-icon>
            <ng-container i18n="@@Git_locations">Git locations</ng-container>
            <app-ee-lock
              feature="configurations"
              i18n-why="@@Git_ee_why"
              why="Keep your configurations in a git repository, one directory per platform."
            />
          </button>
        </div>

    <nav mat-tab-nav-bar [tabPanel]="panel" mat-stretch-tabs="false">
      <a
        mat-tab-link
        routerLink="compose"
        routerLinkActive
        #tCompose="routerLinkActive"
        [active]="tCompose.isActive"
        i18n="@@Compose"
        >Compose</a
      >
      <a
        mat-tab-link
        routerLink="history"
        routerLinkActive
        #tHist="routerLinkActive"
        [active]="tHist.isActive"
        i18n="@@History"
        >History</a
      >
      <a
        mat-tab-link
        routerLink="database"
        routerLinkActive
        #tDb="routerLinkActive"
        [active]="tDb.isActive"
        i18n="@@Database_tab"
        >Database</a
      >
    </nav>
    <mat-tab-nav-panel #panel>
      <router-outlet />
    </mat-tab-nav-panel>
      </mat-drawer-content>

      <!-- ONE drawer on the right (a container holds one per side), and
           two things it can show: the git locations, or the file of what a
           side of the loom holds. -->
      <mat-drawer position="end" mode="over" [opened]="git() || !!reading() || running()" (closedStart)="closeDrawer()">
        @if (git()) {
          <app-git-locations-panel (closed)="closeGit()" />
        } @else if (reading(); as c) {
          <!-- A saved configuration's file, to read and correct: saving
               replaces its document and applies nothing. -->
          <app-configuration-yaml
            [title]="c.name"
            [subtitle]="c.description ?? ''"
            [text]="document()"
            [canEdit]="true"
            [busy]="busy()"
            (saved)="saveDocument(c, $event)"
            (download)="download(c)"
            (closed)="closeDrawer()"
          />
        } @else if (running()) {
          <!-- What runs is read here and changed on Compose, where a change
               shows what it does before it does it. -->
          <app-configuration-yaml
            title="Running"
            i18n-title="@@Running"
            [text]="document()"
            [canEdit]="false"
            (download)="downloadRunning()"
            (closed)="closeDrawer()"
          />
        }
      </mat-drawer>
    </mat-drawer-container>
  `,
})
export class ConfigurationPageComponent {
  private readonly router = inject(Router);
  private readonly route = inject(ActivatedRoute);

  // A QUERY parameter rather than a path segment: the path belongs to the tabs,
  // and the locations are not one of them. It survives F5 and a tab change,
  // like every other drawer on this screen.
  private readonly flag = toSignal(
    this.route.queryParamMap.pipe(map((p) => p.get('git'))),
    { initialValue: null },
  );
  protected readonly git = computed(() => this.flag() === '1');
  // ?read=<id>: a saved configuration's file open in the drawer. The row and
  // its text are read when the id appears; F5-proof like the rest.
  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);
  private readonly readId = toSignal(
    this.route.queryParamMap.pipe(map((p) => p.get('read'))),
    { initialValue: null },
  );
  protected readonly reading = signal<SavedConfiguration | null>(null);
  protected readonly running = computed(() => this.readId() === 'running');
  protected readonly document = signal('');
  protected readonly busy = signal(false);

  constructor() {
    effect(() => {
      const id = this.readId();
      if (!id) {
        this.reading.set(null);
        this.document.set('');
        return;
      }
      if (id === 'running') {
        this.reading.set(null);
        this.api.currentDocument().subscribe({ next: (t) => this.document.set(t), error: () => this.closeDrawer() });
        return;
      }
      this.api.configuration(id).subscribe({ next: (c) => this.reading.set(c), error: () => this.closeDrawer() });
      this.api.configurationDocument(id).subscribe({ next: (t) => this.document.set(t), error: () => this.document.set('') });
    });
  }

  protected closeDrawer(): void {
    void this.router.navigate([], { relativeTo: this.route, queryParams: { git: null, read: null }, queryParamsHandling: 'merge' });
  }

  protected saveDocument(c: SavedConfiguration, text: string): void {
    this.busy.set(true);
    this.api.replaceConfigurationDocument(c.id, new Blob([text], { type: 'application/yaml' })).subscribe({
      next: () => {
        this.busy.set(false);
        this.snack.open($localize`:@@Configuration_saved:Configuration saved`, undefined, { duration: 2500 });
        this.closeDrawer();
      },
      error: (err: unknown) => {
        this.busy.set(false);
        const e = err as { error?: { error?: string } };
        this.snack.open(typeof e?.error?.error === 'string' ? e.error.error : $localize`:@@Request_failed:Request failed`, undefined, { duration: 6000 });
      },
    });
  }

  protected download(c: SavedConfiguration): void {
    this.api.exportConfiguration(c.id).subscribe({ next: (text) => this.save(text, c.name), error: () => undefined });
  }

  protected downloadRunning(): void {
    this.save(this.document(), 'running');
  }

  private save(text: string, name: string): void {
    const url = URL.createObjectURL(new Blob([text], { type: 'application/yaml' }));
    const a = document.createElement('a');
    a.href = url;
    a.download = `meerkat-${name.toLowerCase().replace(/[^a-z0-9]+/g, '-')}.yaml`;
    a.click();
    URL.revokeObjectURL(url);
  }

  protected openGit(): void {
    void this.router.navigate([], {
      relativeTo: this.route,
      queryParams: { git: 1 },
      queryParamsHandling: 'merge',
    });
  }

  protected closeGit(): void {
    void this.router.navigate([], {
      relativeTo: this.route,
      queryParams: { git: null },
      queryParamsHandling: 'merge',
    });
  }
}
