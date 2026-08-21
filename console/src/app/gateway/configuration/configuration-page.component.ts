import { Component } from '@angular/core';
import { MatTabsModule } from '@angular/material/tabs';
import { RouterLink, RouterLinkActive, RouterOutlet } from '@angular/router';

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
//   Snapshot       a copy of the whole DATABASE, accounts and sessions and
//                  audit included. A different artifact for a different day:
//                  a configuration reproduces this gateway elsewhere, a
//                  snapshot restores this one.
//
// There WAS an Import/export tab. Everything it held has a better place: the
// export report is a sentence before a download, the plan and the vault holes
// belong to the import that produces them, and the pruning checkbox became one
// of three destinations. A tab holding nothing of its own is a tab.
//
// Routed tabs (mat-tab-nav-bar), like the built-in pages: a bookmark on the
// snapshot procedure must come back to the snapshot procedure.
@Component({
  selector: 'app-configuration-page',
  imports: [MatTabsModule, RouterLink, RouterLinkActive, RouterOutlet],
  styles: [
    `
      /* The screen owns the height: the heading and the tabs stay put, the
         panel underneath scrolls. That is what lets the Management tab hold a
         drawer at all - a drawer inside a 900px text column is a drawer with
         nowhere to open. The reading width lives on the TABS instead, each
         setting its own. */
      :host {
        display: flex;
        flex-direction: column;
        height: 100%;
        padding: 24px 24px 0;
        box-sizing: border-box;
        overflow: hidden;
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
    <h1 i18n="@@Configuration">Configuration</h1>
    <p class="hint" i18n="@@Configuration_intro">
      Routes, roles, authorities, mail relay, themes and gateway settings travel as one file.
      Users, organisations, sessions and the vault stay where they are: they are not
      configuration, they are what this gateway lives with.
    </p>

    <nav mat-tab-nav-bar [tabPanel]="panel" mat-stretch-tabs="false">
      <a
        mat-tab-link
        routerLink="management"
        routerLinkActive
        #tMgmt="routerLinkActive"
        [active]="tMgmt.isActive"
        i18n="@@Management"
        >Management</a
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
        routerLink="snapshot"
        routerLinkActive
        #tSnap="routerLinkActive"
        [active]="tSnap.isActive"
        i18n="@@Snapshot"
        >Snapshot</a
      >
    </nav>
    <mat-tab-nav-panel #panel>
      <router-outlet />
    </mat-tab-nav-panel>
  `,
})
export class ConfigurationPageComponent {}
