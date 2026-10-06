import { Component, computed, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatCardModule } from '@angular/material/card';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatSelectModule } from '@angular/material/select';
import { ApiService, BackupInfo, DatabaseCopy, DatabaseKind, DatabaseProbe, DatabaseTarget, DiscoveredService } from '../../api.service';
import { EeLockComponent } from '../../shared/ee-lock.component';
import { PauseService } from '../../shared/pause.service';
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
// The same tab MOVES the database (store/transfer.go): a copy of the same kind
// is a backup, of the other kind a migration - SQLite to PostgreSQL to scale,
// PostgreSQL to SQLite to come back to one gateway. Into a file of either kind,
// or straight into a PostgreSQL server. The pause above it is what makes a
// migration whole: nothing is written between the copy and the switch.
//
// There is no restore button, deliberately: a database cannot be swapped under
// the process holding it open, and accepting an arbitrary one as trusted state
// would turn a borrowed admin session into permanent control of the gateway.
// The console prints the exact commands instead, with THIS installation's
// paths.
@Component({
  selector: 'app-configuration-snapshot',
  imports: [
    MatButtonModule,
    MatButtonToggleModule,
    MatCardModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatSelectModule,
    MatSlideToggleModule,
    EeLockComponent,
    SnippetComponent,
  ],
  styleUrl: './configuration-cards.scss',
  template: `
  <mat-card appearance="outlined">
    <h2 i18n="@@Pause">Pause</h2>
    <p class="hint" i18n="@@Pause_hint">
      Every application answers the maintenance page and the gateway writes nothing, on every node.
      Pause before a migration: what is written after the copy would be lost at the switch.
    </p>
    <mat-slide-toggle [checked]="pause.paused()" (change)="pause.set($event.checked)">
      <ng-container i18n="@@Pause_the_gateway">Pause the gateway</ng-container>
    </mat-slide-toggle>
    <div class="note">
      <mat-icon>restart_alt</mat-icon>
      <p i18n="@@Pause_not_stored">
        Never stored: a restart ends it, and a gateway started on the new database starts working
        at once.
      </p>
    </div>
  </mat-card>

  <mat-card appearance="outlined">
    <h2 i18n="@@Full_snapshot">Full snapshot</h2>
    <p class="hint" i18n="@@Snapshot_hint_kinds">
      The whole database, users, sessions, vault and audit trail included. Of the same kind as this
      gateway's, it is your backup; of the other kind, it moves the gateway.
    </p>
    @if (backup(); as b) {
      <div class="actions">
        <span class="hint">
          <ng-container i18n="@@This_gateway_runs_on">This gateway runs on</ng-container>
          <strong>{{ kindLabel(b.dialect) }}</strong>
        </span>
      </div>
      <div class="actions">
        <mat-button-toggle-group [value]="target()" (change)="target.set($event.value)" hideSingleSelectionIndicator>
          <mat-button-toggle value="sqlite" i18n="@@Embedded_database">Embedded database</mat-button-toggle>
          <mat-button-toggle value="postgres" ee-feature="STORE-03">
            PostgreSQL
            <app-ee-lock
              feature="STORE-03"
              i18n-why="@@Postgres_why"
              why="Several gateways sharing one PostgreSQL."
            />
          </mat-button-toggle>
        </mat-button-toggle-group>
        @if (target() === 'postgres') {
          <mat-button-toggle-group [value]="into()" (change)="into.set($event.value)" hideSingleSelectionIndicator>
            <mat-button-toggle value="file" i18n="@@A_dump_file">A dump file</mat-button-toggle>
            <mat-button-toggle value="server" i18n="@@A_server">A server</mat-button-toggle>
          </mat-button-toggle-group>
        }
      </div>

      @if (migration() && !pause.paused()) {
        <div class="note warn">
          <mat-icon>pause_circle</mat-icon>
          <p i18n="@@Pause_before_migrating">
            This is a migration: pause the gateway first, or what is written after the copy is lost
            at the switch.
          </p>
        </div>
      }

      @if (target() === 'postgres' && into() === 'server') {
        @if (detected().length) {
          <!-- What this gateway's runtime lists on 5432. A pooler or a
               replica is shown and refused: the cluster needs LISTEN/NOTIFY
               and advisory locks, which a transaction pooler breaks and a
               replica cannot take. -->
          <mat-form-field class="field wide">
            <mat-label i18n="@@Detected_postgres">Detected in this cluster</mat-label>
            <mat-select (selectionChange)="pick($event.value)">
              @for (d of detected(); track d.name) {
                <mat-option [value]="d" [disabled]="!!d.refused">
                  {{ d.name }}
                  @if (d.refused) {
                    <span class="muted">- {{ d.refused }}</span>
                  }
                </mat-option>
              }
            </mat-select>
          </mat-form-field>
        }
        <div class="fields">
          <mat-form-field class="field wide">
            <mat-label i18n="@@Host">Host</mat-label>
            <input matInput [value]="t().host" (input)="set('host', $any($event.target).value)" />
          </mat-form-field>
          <mat-form-field class="field port">
            <mat-label i18n="@@Port">Port</mat-label>
            <input matInput type="number" [value]="t().port" (input)="set('port', +$any($event.target).value)" />
          </mat-form-field>
          <mat-form-field class="field">
            <mat-label i18n="@@Database">Database</mat-label>
            <input matInput [value]="t().database" (input)="set('database', $any($event.target).value)" />
          </mat-form-field>
          <mat-form-field class="field">
            <mat-label i18n="@@User">User</mat-label>
            <input matInput autocomplete="off" [value]="t().user" (input)="set('user', $any($event.target).value)" />
          </mat-form-field>
          <mat-form-field class="field">
            <mat-label i18n="@@Password">Password</mat-label>
            <input matInput type="password" autocomplete="new-password" [value]="t().password" (input)="set('password', $any($event.target).value)" />
          </mat-form-field>
          <mat-form-field class="field">
            <mat-label i18n="@@SSL_mode">SSL mode</mat-label>
            <mat-select [value]="t().sslmode" (selectionChange)="set('sslmode', $event.value)">
              @for (m of sslModes; track m) {
                <mat-option [value]="m">{{ m }}</mat-option>
              }
            </mat-select>
          </mat-form-field>
        </div>
        <p class="hint" i18n="@@Target_fields_hint">
          An empty database. These are used for the copy and kept nowhere; the trail records the host.
        </p>
        <div class="actions">
          <!-- Before the copy, and before the pause: it reads only. -->
          <button matButton="outlined" (click)="check()" [disabled]="busy() || !complete()">
            <mat-icon>network_check</mat-icon>
            <ng-container i18n="@@Test">Test</ng-container>
          </button>
          <button matButton="filled" (click)="copy()" [disabled]="busy() || !pause.paused() || !complete() || probeRefuses()">
            @if (busy()) {<mat-icon spin>autorenew</mat-icon>} @else {<mat-icon>move_down</mat-icon>}
            <ng-container i18n="@@Copy_the_database">Copy the database</ng-container>
          </button>
        </div>
        @if (probe(); as pr) {
          <div class="note" [class.warn]="!pr.empty || !pr.canCreate">
            <mat-icon>{{ pr.empty && pr.canCreate ? 'task_alt' : 'error' }}</mat-icon>
            <p>
              <ng-container i18n="@@Probe_answered">PostgreSQL {{ pr.version }} answered,</ng-container>
              @if (pr.ssl) {
                <ng-container i18n="@@Probe_ssl">over TLS (sslmode {{ pr.sslmode }}).</ng-container>
              } @else {
                <ng-container i18n="@@Probe_plain">without TLS (sslmode {{ pr.sslmode }}).</ng-container>
              }
              @if (!pr.empty) {
                <ng-container i18n="@@Probe_in_use">This database already holds a gateway: copy into an empty one.</ng-container>
              } @else if (!pr.canCreate) {
                <ng-container i18n="@@Probe_cannot_create">This user may not create tables here: grant it, or make it the database's owner.</ng-container>
              } @else {
                <ng-container i18n="@@Probe_ready">Empty, and the user may create the tables: ready for the copy.</ng-container>
              }
            </p>
          </div>
        }
        @if (copied(); as c) {
          <div class="note">
            <mat-icon>check_circle</mat-icon>
            <p i18n="@@Copied_summary">{{ c.rows }} rows in {{ c.tables.length }} tables copied to {{ c.target }}, every table counted on both sides.</p>
          </div>
        }
      } @else {
        <div class="actions">
          <button matButton="filled" (click)="download()" [disabled]="busy() || (migration() && !pause.paused())">
            @if (busy()) {<mat-icon spin>autorenew</mat-icon>} @else {<mat-icon>database</mat-icon>}
            <ng-container i18n="@@Download_a_snapshot">Download a snapshot</ng-container>
          </button>
        </div>
      }

      @if (!(target() === 'postgres' && into() === 'server') || copied()) {
      <h3>
        @if (migration()) {
          <ng-container i18n="@@How_to_migrate">How to migrate</ng-container>
        } @else {
          <ng-container i18n="@@How_to_restore">How to restore</ng-container>
        }
      </h3>
      <app-snippet [filename]="procedureName()" [downloadable]="false" [content]="procedure(b)" />
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
  styles: `
    .fields {
      display: flex;
      flex-wrap: wrap;
      gap: 0 12px;
    }
    .field {
      width: 200px;
    }
    .field.wide {
      width: 320px;
    }
    .field.port {
      width: 110px;
    }
    .muted {
      color: var(--mat-sys-on-surface-variant);
    }
    h3 {
      margin: 20px 0 8px;
      font-size: 1rem;
      font-weight: 500;
    }
  `,
})
export class ConfigurationSnapshotComponent {
  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);
  protected readonly pause = inject(PauseService);

  protected readonly busy = signal(false);
  protected readonly backup = signal<BackupInfo | null>(null);
  // The kind of the copy, and for PostgreSQL where it goes.
  protected readonly target = signal<DatabaseKind>('sqlite');
  protected readonly into = signal<'file' | 'server'>('file');
  protected readonly sslModes = ['prefer', 'require', 'verify-full', 'disable'];
  // The target server, one field each. Database and user default to the
  // product's name: what the deployment files and the chart suggest.
  protected readonly t = signal<DatabaseTarget>({
    host: '', port: 5432, database: 'meerkat', user: 'meerkat', password: '', sslmode: 'prefer',
  });
  protected readonly complete = computed(() => {
    const t = this.t();
    return !!(t.host.trim() && t.database.trim() && t.user.trim() && t.password && t.port > 0);
  });
  protected readonly copied = signal<DatabaseCopy | null>(null);
  // What the target said when tested. Cleared as soon as a field changes: an
  // answer about another server is no answer.
  protected readonly probe = signal<DatabaseProbe | null>(null);
  // A test that found a gateway, or a user that may not create tables, is a
  // copy that would fail: not offered.
  protected readonly probeRefuses = computed(() => {
    const p = this.probe();
    return !!p && (!p.empty || !p.canCreate);
  });
  // PostgreSQL services the runtime lists beside this gateway, the primary
  // first; poolers and replicas listed with why they will not do.
  protected readonly detected = signal<{ name: string; host: string; port: number; refused?: string }[]>([]);
  protected readonly migration = computed(() => this.target() !== this.backup()?.dialect);

  constructor() {
    this.api.backupInfo().subscribe({
      next: (b) => {
        this.backup.set(b);
        this.target.set(b.dialect);
      },
    });
    this.api.services().subscribe({ next: (d) => this.detected.set(postgresIn(d.services ?? [])), error: () => {} });
  }

  protected set<K extends keyof DatabaseTarget>(k: K, v: DatabaseTarget[K]): void {
    this.t.update((t) => ({ ...t, [k]: v }));
    this.probe.set(null);
  }

  // Asks the target what it is. The SSL mode it connected with replaces
  // prefer, so what is copied with, and what the switch prints, is the mode
  // that is known to work.
  protected check(): void {
    this.busy.set(true);
    this.probe.set(null);
    this.api.checkDatabase({ ...this.t(), host: this.t().host.trim() }).subscribe({
      next: (p) => {
        this.busy.set(false);
        if (this.t().sslmode === 'prefer') this.t.update((t) => ({ ...t, sslmode: p.sslmode }));
        this.probe.set(p);
      },
      error: (err: unknown) => {
        this.busy.set(false);
        this.fail(err);
      },
    });
  }

  protected pick(d: { host: string; port: number }): void {
    this.t.update((t) => ({ ...t, host: d.host, port: d.port }));
    this.probe.set(null);
  }

  protected kindLabel(k: DatabaseKind): string {
    return k === 'postgres' ? 'PostgreSQL' : $localize`:@@Embedded_database_sqlite:the embedded database (SQLite)`;
  }

  // A file is fetched as a Blob and handed to the browser: reading it as text
  // would corrupt a database file silently, and the error of a refused one
  // has to reach the screen rather than land in a downloaded file.
  protected download(): void {
    const kind = this.target();
    this.busy.set(true);
    this.api.snapshot(kind).subscribe({
      next: (blob) => {
        this.busy.set(false);
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `meerkat-${new Date().toISOString().slice(0, 10)}.${kind === 'postgres' ? 'sql' : 'db'}`;
        a.click();
        URL.revokeObjectURL(url);
      },
      error: (err: unknown) => {
        this.busy.set(false);
        this.fail(err);
      },
    });
  }

  protected copy(): void {
    this.busy.set(true);
    this.copied.set(null);
    this.api.copyDatabase({ ...this.t(), host: this.t().host.trim() }).subscribe({
      next: (c) => {
        this.busy.set(false);
        this.copied.set(c);
      },
      error: (err: unknown) => {
        this.busy.set(false);
        this.fail(err);
      },
    });
  }

  // Stays until closed: what a server answers is often a paragraph - an
  // authentication refused, a certificate nobody here can verify - and it is
  // read while fixing a field, not in the eight seconds a timer allows.
  private fail(err: unknown): void {
    const e = err as { error?: { error?: string } };
    this.snack.open(
      typeof e?.error?.error === 'string' ? e.error.error : $localize`:@@Request_failed:Request failed`,
      $localize`:@@Close:Close`,
    );
  }

  protected procedureName(): string {
    if (this.migration() && this.target() === 'postgres' && this.into() === 'server') return 'switch-to-postgres.sh';
    if (!this.migration()) return 'restore.sh';
    return this.target() === 'postgres' ? 'migrate-to-postgres.sh' : 'migrate-to-embedded.sh';
  }

  // The real paths of THIS installation, so the procedure is a copy-paste and
  // not a puzzle. What is kept aside is the way back.
  protected procedure(b: BackupInfo): string {
    const key = b.keyFromEnv
      ? '# the vault key: the MEERKAT_VAULT_KEY this gateway already has'
      : `# the vault key: the content of ${b.keyFile}`;
    if (!this.migration() && b.dialect === 'sqlite') {
      return [
        '# 1. stop meerkat',
        '# 2. keep the current database aside',
        `mv ${b.dbFile} ${b.dbFile}.before-restore`,
        '# 3. put the snapshot in its place',
        `cp meerkat-YYYY-MM-DD.db ${b.dbFile}`,
        '# 4. start meerkat, then check a route and a sign-in',
      ].join('\n');
    }
    if (!this.migration()) {
      return [
        '# 1. stop meerkat, every replica',
        '# 2. load the dump into an EMPTY database (a new one, or the old one dropped and created again)',
        'psql -v ON_ERROR_STOP=1 -f meerkat-YYYY-MM-DD.sql "$MEERKAT_DATABASE_URL"',
        '# 3. start meerkat, then check a route and a sign-in',
      ].join('\n');
    }
    if (this.target() === 'postgres') {
      // For a copy, the target that was just filled in; for a dump, a
      // placeholder to fill.
      const t = this.t();
      const server = this.into() === 'server';
      const dbUrl = server
        ? `postgres://${t.user}:PASSWORD@${t.host}:${t.port}/${t.database}${t.sslmode ? '?sslmode=' + t.sslmode : ''}`
        : 'postgres://meerkat:PASSWORD@host:5432/meerkat';
      const steps = server
        ? ['# done: paused, and copied into an empty database, every table counted on both sides']
        : [
            '# 1. pause the gateway (above): nothing is written from now on',
            '# 2. load the dump into an EMPTY database',
            `psql -v ON_ERROR_STOP=1 -f meerkat-YYYY-MM-DD.sql "${dbUrl}"`,
          ];
      // A key the gateway already reads from its environment is already in the
      // deployment, and stays: setting it again would only risk a different
      // one. A key in a file has to join the Secret.
      const secret = b.keyFromEnv
        ? [
            '# the new database, as a Secret (the vault key is already in the deployment, unchanged)',
            'kubectl create secret generic meerkat-database \\',
            `  --from-literal=database-url="${dbUrl}"`,
          ]
        : [
            '# the new database and the vault key, as a Secret',
            key,
            'kubectl create secret generic meerkat-database \\',
            `  --from-literal=database-url="${dbUrl}" \\`,
            '  --from-literal=vault-key="VAULT_KEY"',
          ];
      return [
        ...steps,
        '',
        ...secret,
        '',
        '# Helm: point the release at it, and scale',
        'helm upgrade meerkat softwarity/meerkat --reuse-values \\',
        '  --set database.existingSecret=meerkat-database \\',
        ...(b.keyFromEnv ? [] : ['  --set vault.existingSecret=meerkat-database \\']),
        '  --set replicaCount=2',
        '',
        '# Compose or Swarm instead: the same on the service',
        `#   MEERKAT_DATABASE_URL=${dbUrl}`,
        ...(b.keyFromEnv ? ['#   MEERKAT_VAULT_KEY unchanged'] : ['#   MEERKAT_VAULT_KEY=VAULT_KEY']),
        '#   and the data volume is no longer needed',
        '',
        '# the new pods start unpaused, on PostgreSQL: check a route and a sign-in',
        `# the volume holding ${b.dbFile} is the way back: keep it until then.`,
        '# an older chart deletes that volume on this very upgrade: download a snapshot of the same kind first',
      ].join('\n');
    }
    return [
      '# 1. pause the gateway (above), then download the embedded database',
      '# 2. one gateway, one volume: scale to 1 and drop the database URL',
      'helm upgrade meerkat softwarity/meerkat --reuse-values \\',
      '  --set database.existingSecret="" --set database.url="" --set replicaCount=1',
      `# 3. put the file in the volume as ${b.dataDir}/meerkat.db, the vault key unchanged`,
      key,
      '# 4. start meerkat, then check a route and a sign-in; the PostgreSQL database is the way back',
    ].join('\n');
  }
}

// The PostgreSQL services among what the runtime lists: a 5432, or a name that
// says so. A pooler or a replica is kept and refused, with the reason - the
// cluster needs LISTEN/NOTIFY and advisory locks, which a transaction pooler
// breaks and a replica cannot take. The primary of an operator (CrunchyData's
// -primary, CloudNativePG's -rw) comes first.
function postgresIn(services: DiscoveredService[]): { name: string; host: string; port: number; refused?: string }[] {
  const out: { name: string; host: string; port: number; refused?: string; rank: number }[] = [];
  for (const s of services) {
    const port = s.ports?.find((p) => p.target === 5432)?.target ?? (/postgres|pg/i.test(s.name) ? s.ports?.[0]?.target : undefined);
    if (!port) continue;
    const name = s.name;
    let refused: string | undefined;
    if (/bouncer|pooler/i.test(name)) refused = $localize`:@@Pg_pooler:a pooler, not for a gateway`;
    else if (/replica|-ro$|-r$|repl/i.test(name)) refused = $localize`:@@Pg_replica:a replica, read only`;
    const rank = refused ? 2 : /primary|-rw$/i.test(name) ? 0 : 1;
    out.push({ name, host: s.names?.[0] ?? name, port, refused, rank });
  }
  return out.sort((a, b) => a.rank - b.rank || a.name.localeCompare(b.name));
}
