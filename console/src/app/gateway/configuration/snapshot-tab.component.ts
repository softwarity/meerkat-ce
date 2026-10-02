import { Component, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatCardModule } from '@angular/material/card';
import { MatIconModule } from '@angular/material/icon';
import { MatSnackBar } from '@angular/material/snack-bar';
import { ApiService, BackupInfo } from '../../api.service';
import { SnippetComponent } from '../../shared/snippet.component';

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
  imports: [MatButtonModule, MatCardModule, MatIconModule, SnippetComponent],
  styleUrl: './configuration-cards.scss',
  template: `
  <mat-card appearance="outlined">
    <h2 i18n="@@Full_snapshot">Full snapshot</h2>
    <p class="hint" i18n="@@Snapshot_hint">
      A consistent copy of the whole database, users, sessions, vault and audit trail included,
      taken while the gateway runs. This is your backup.
    </p>
    <div class="note">
      <mat-icon>schedule</mat-icon>
      <p i18n="@@Snapshot_scope_note">
        Use this rather than copying the live database file. Scheduling and retention are up to
        your backup tool.
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
          Restore with the service stopped, and keep the old file until the new one works.
        </p>
        <app-snippet filename="restore.sh" [downloadable]="false" [content]="restoreProcedure(b)" />
        @if (b.keyFromEnv) {
          <p class="hint" i18n="@@Key_from_env_note">
            The master key comes from MEERKAT_VAULT_KEY: the snapshot's vault is useless without
            that variable.
          </p>
        } @else {
          <div class="note warn">
            <mat-icon>key</mat-icon>
            <p i18n="@@Key_beside_db_note">
              The master key sits next to the database, so backing both up together defeats the
              encryption. Keep it in a secret manager, or supply it through MEERKAT_VAULT_KEY.
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
