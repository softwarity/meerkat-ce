import { Component, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatIconModule } from '@angular/material/icon';
import { MatSnackBar } from '@angular/material/snack-bar';
import { ApiService, BackupInfo } from '../../api.service';

// The full snapshot (STORE-05): a coherent copy of the whole database, taken
// while the gateway runs.
//
// Its own tab because it answers a different question from the two others. A
// configuration says how this gateway is SET UP and reproduces it elsewhere; a
// snapshot holds everything it LIVES WITH - accounts, sessions, the vault, the
// audit trail - and restores this one. Sitting them side by side under one
// heading is how an operator ends up taking the wrong one to a disaster.
//
// There is no restore button, deliberately: a database cannot be swapped under
// the process holding it open, and accepting an arbitrary one as trusted state
// would turn a borrowed admin session into permanent control of the gateway.
// The console prints the exact commands instead, with THIS installation's
// paths.
@Component({
  selector: 'app-configuration-snapshot',
  imports: [MatButtonModule, MatCardModule, MatIconModule],
  styleUrl: './configuration-cards.scss',
  styles: [
    `
      pre {
        background: var(--mat-sys-surface-container);
        padding: 12px 16px;
        border-radius: 8px;
        font-family: var(--mk-mono);
        font-size: 0.8rem;
        overflow-x: auto;
      }
    `,
  ],
  template: `
  <mat-card appearance="outlined">
    <h2 i18n="@@Full_snapshot">Full snapshot</h2>
    <p class="hint" i18n="@@Snapshot_hint">
      A coherent copy of the whole database, taken while the gateway runs: routes and vault,
      but also users, organisations, sessions and the audit trail. This is what a backup
      restores; a configuration export is what a second gateway reproduces.
    </p>
    <div class="note">
      <mat-icon>schedule</mat-icon>
      <p i18n="@@Snapshot_scope_note">
        Meerkat takes the snapshot, because copying a live database with cp can catch it
        mid-write and nothing says so until the day you restore it. Scheduling, retention and
        shipping it off the host belong to your backup tool, not here.
      </p>
    </div>

    <div class="actions">
      <button matButton="filled" (click)="takeSnapshot()" [disabled]="snapshotting()">
        <!-- No whitespace inside the branches (NG8011). -->
        @if (snapshotting()) {<mat-icon spin>autorenew</mat-icon>} @else {<mat-icon>database</mat-icon>}
        <ng-container i18n="@@Download_a_snapshot">Download a snapshot</ng-container>
      </button>
      <button matButton (click)="showRestore.set(!showRestore())">
        <mat-icon>{{ showRestore() ? 'expand_less' : 'expand_more' }}</mat-icon>
        <ng-container i18n="@@How_to_restore">How to restore</ng-container>
      </button>
    </div>

    @if (showRestore()) {
      @if (backup(); as b) {
        <p class="hint" style="margin-top: 16px" i18n="@@Restore_hint">
          Restoring happens with the service stopped: a database cannot be replaced under the
          process holding it open, and the sessions and users it brings back are the ones the
          request doing it would depend on. Keep the old file until the new one has proven
          itself.
        </p>
        <pre>{{ restoreProcedure(b) }}</pre>
        @if (b.keyFromEnv) {
          <p class="hint" i18n="@@Key_from_env_note">
            The master key of this gateway comes from MEERKAT_VAULT_KEY, so it is not in that
            directory: the snapshot holds the vault encrypted and is worth nothing without
            that variable. Keep it that way.
          </p>
        } @else {
          <div class="note warn">
            <mat-icon>key</mat-icon>
            <p i18n="@@Key_beside_db_note">
              The master key sits next to the database. Backing them up together undoes the
              encryption at rest, exactly as a vault file stored beside its passphrase would:
              keep the key in a secret manager, or supply it through MEERKAT_VAULT_KEY.
            </p>
          </div>
        }
      }
    }
  </mat-card>

  `,
})
export class ConfigurationSnapshotComponent {
  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);

  protected readonly snapshotting = signal(false);
  protected readonly showRestore = signal(false);
  protected readonly backup = signal<BackupInfo | null>(null);

  constructor() {
    this.api.backupInfo().subscribe({ next: (b) => this.backup.set(b) });
  }

  // A snapshot is a binary file: fetched as a Blob and handed straight to the
  // browser. Reading it as text would corrupt it silently.
  protected takeSnapshot(): void {
    this.snapshotting.set(true);
    this.api.snapshot().subscribe({
      next: (blob) => {
        this.snapshotting.set(false);
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `meerkat-${new Date().toISOString().slice(0, 10)}.db`;
        a.click();
        URL.revokeObjectURL(url);
      },
      error: (err: unknown) => {
        this.snapshotting.set(false);
        const e = err as { error?: { error?: string } };
        this.snack.open(
          typeof e?.error?.error === 'string' ? e.error.error : $localize`:@@Request_failed:Request failed`,
          undefined,
          { duration: 6000 },
        );
      },
    });
  }

  // The real paths of THIS installation, so the procedure is a copy-paste and
  // not a puzzle. The old file is kept aside: a restore one regrets must have a
  // way back.
  protected restoreProcedure(b: BackupInfo): string {
    return [
      '# 1. stop meerkat',
      '# 2. keep the current database aside',
      `mv ${b.dbFile} ${b.dbFile}.before-restore`,
      '# 3. put the snapshot in its place',
      `cp meerkat-YYYY-MM-DD.db ${b.dbFile}`,
      '# 4. start meerkat, then check a route and a sign-in',
    ].join('\n');
  }
}
