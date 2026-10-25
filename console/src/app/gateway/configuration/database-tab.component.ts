import { NgTemplateOutlet } from '@angular/common';
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
import {
  ApiService,
  BackupInfo,
  DatabaseKind,
  DatabaseProbe,
  DatabaseTarget,
  DiscoveredService,
  LastCopy,
  Platform,
  PlatformName,
} from '../../api.service';
import { EeLockComponent } from '../../shared/ee-lock.component';
import { PauseService } from '../../shared/pause.service';
import { SnippetComponent } from '../../shared/snippet.component';
import { zoned } from '../../shared/zoned-date.pipe';
import { MeService } from '../../me.service';
import {
  Snippet,
  createDatabase,
  platformNames,
  restore,
  switchToEmbedded,
  switchToPostgres,
  wayBackFromPostgres,
} from './database-procedure';

// The database tab (STORE-05): the whole database backed up, restored and
// moved between the embedded one and PostgreSQL.
//
// Its own tab because it answers a different question from the two others. A
// configuration says how this gateway is SET UP and reproduces it elsewhere; a
// snapshot holds everything it LIVES WITH - accounts, sessions, the vault, the
// audit trail - and restores this one. Sitting them side by side under one
// heading is how an operator ends up taking the wrong one to a disaster.
//
// A copy of the same kind is a backup, of the other kind a migration - SQLite
// to PostgreSQL to scale, PostgreSQL to SQLite to come back to one gateway.
// Into a file of either kind, or straight into a PostgreSQL server. The pause
// above it is what makes a migration whole: nothing is written between the
// copy and the switch.
//
// A move is ONE numbered procedure, shown from its start and not after the
// copy: create the database, test it, pause, copy, switch the deployment,
// check, and the way back - each step ticked when it is done. The copy is
// remembered by the server (store/lastcopy.go), so a reload in the middle of
// a migration, on any node, comes back to the same procedure.
//
// There is no restore button, deliberately: a database cannot be swapped under
// the process holding it open, and accepting an arbitrary one as trusted state
// would turn a borrowed admin session into permanent control of the gateway.
// The console prints the exact commands instead, with THIS installation's
// paths, for the platform it runs on.
type Step = 'create' | 'test' | 'pause' | 'copy' | 'download' | 'load' | 'switch' | 'check' | 'back';

@Component({
  selector: 'app-configuration-database',
  imports: [
    NgTemplateOutlet,
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
      All applications show the maintenance page and nothing is written, on every gateway. Pause
      before a migration, or what is written after the copy is lost.
    </p>
    <mat-slide-toggle [checked]="pause.paused()" (change)="pause.set($event.checked)">
      <ng-container i18n="@@Pause_the_gateway">Pause the gateway</ng-container>
    </mat-slide-toggle>
    <div class="note">
      <mat-icon>restart_alt</mat-icon>
      <p i18n="@@Pause_not_stored">
        Not saved: a restart ends the pause, so a gateway started on the new database works at
        once.
      </p>
    </div>
  </mat-card>

  <mat-card appearance="outlined">
    <h2 i18n="@@Database_title">Database</h2>
    <p class="hint" i18n="@@Database_hint_kinds">
      The whole database, including users, sessions, vault and audit trail. A copy of the same type
      is a backup; to the other type, it migrates the gateway.
    </p>
    @if (backup(); as b) {
      <div class="actions">
        <span class="hint">
          <ng-container i18n="@@This_gateway_runs_on">This gateway runs on</ng-container>
          <!-- No whitespace before the comma: "(SQLite), deployed with". -->
          <strong>{{ kindLabel(b.dialect) }}</strong>@if (platformLabel(b.platform); as pl) {<ng-container i18n="@@Deployed_with">, deployed with</ng-container>
            <strong>{{ midSentence(pl) }}</strong>
          }
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

      @if (server()) {
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
            <input matInput autocomplete="off" spellcheck="false" [value]="t().password" (input)="set('password', $any($event.target).value)" />
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
        <p class="hint" i18n="@@Target_fields_hint_v2">
          An empty database. The password is used for the test and the copy, and never stored. The
          audit trail records the host.
        </p>
      }

      @if (moving() && !pause.paused()) {
        <div class="note warn">
          <mat-icon>pause_circle</mat-icon>
          <p i18n="@@Pause_before_migrating">
            This is a migration: pause the gateway first, or what is written after the copy is lost.
          </p>
        </div>
      }

      @if (moving()) {
        <div class="actions written-for">
          <h3>
            @if (target() === 'postgres') {
              <ng-container i18n="@@How_to_migrate">How to migrate</ng-container>
            } @else {
              <ng-container i18n="@@How_to_migrate_back">How to come back to the embedded database</ng-container>
            }
          </h3>
          <span class="grow"></span>
          <ng-container *ngTemplateOutlet="platformPicker" />
        </div>
        <ol class="steps">
          @for (s of steps(); track s; let i = $index) {
            <li [class.done]="isDone(s)">
              <div class="step-head">
                @if (isDone(s)) {
                  <mat-icon class="tick">check_circle</mat-icon>
                } @else {
                  <span class="num">{{ i + 1 }}</span>
                }
                <span class="step-title">{{ stepTitle(s) }}</span>
              </div>
              <div class="step-body">
                @switch (s) {
                  @case ('create') {
                    <p class="hint" i18n="@@Step_create_hint">
                      For a database administrator to run, with the names and password entered above.
                    </p>
                    <app-snippet [filename]="sql().name" [downloadable]="false" [content]="sql().content" />
                  }
                  @case ('test') {
                    <div class="actions">
                      <!-- Before the copy, and before the pause: it reads only. -->
                      <button matButton="outlined" (click)="check()" [disabled]="busy() || !complete()">
                        <mat-icon>network_check</mat-icon>
                        <ng-container i18n="@@Test">Test</ng-container>
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
                            <ng-container i18n="@@Probe_in_use">This database is already used by a gateway: use an empty one.</ng-container>
                          } @else if (!pr.canCreate) {
                            <ng-container i18n="@@Probe_cannot_create">This user may not create tables here: grant it, or make it the database's owner.</ng-container>
                          } @else {
                            <ng-container i18n="@@Probe_ready">Empty, and the user may create the tables: ready for the copy.</ng-container>
                          }
                        </p>
                      </div>
                    }
                  }
                  @case ('pause') {
                    <p class="hint" i18n="@@Step_pause_hint">
                      With the switch at the top of this tab: nothing is written from then on.
                    </p>
                  }
                  @case ('copy') {
                    <div class="actions">
                      <button matButton="filled" (click)="copy()" [disabled]="busy() || !pause.paused() || !complete() || probeRefuses()">
                        @if (busy()) {<mat-icon spin>autorenew</mat-icon>} @else {<mat-icon>move_down</mat-icon>}
                        <ng-container i18n="@@Copy_the_database">Copy the database</ng-container>
                      </button>
                    </div>
                    @if (copiedHere(); as c) {
                      <div class="note">
                        <mat-icon>check_circle</mat-icon>
                        <p>
                          <ng-container i18n="@@Copied_remembered">{{ c.rows }} rows in {{ c.tables }} tables copied to {{ c.host }}:{{ c.port }}/{{ c.database }} on {{ when(c.at) }}, row counts checked on both sides.</ng-container>
                          <button matButton class="inline" (click)="forget()" i18n="@@Dismiss">Dismiss</button>
                        </p>
                      </div>
                    }
                  }
                  @case ('download') {
                    <div class="actions">
                      <button matButton="filled" (click)="download()" [disabled]="busy() || !pause.paused()">
                        @if (busy()) {<mat-icon spin>autorenew</mat-icon>} @else {<mat-icon>database</mat-icon>}
                        @if (target() === 'postgres') {
                          <ng-container i18n="@@Download_a_dump">Download a dump</ng-container>
                        } @else {
                          <ng-container i18n="@@Download_a_snapshot">Download a snapshot</ng-container>
                        }
                      </button>
                    </div>
                  }
                  @case ('load') {
                    <app-snippet filename="load-dump.sh" [downloadable]="false" [content]="loadDump()" />
                  }
                  @case ('switch') {
                    <p class="hint" i18n="@@Step_switch_hint">
                      The gateway chooses its database at startup: make the change in the deployment.
                    </p>
                    @for (sn of switchSnippets(b); track sn.name) {
                      <app-snippet [filename]="sn.name" [downloadable]="false" [content]="sn.content" />
                    }
                  }
                  @case ('check') {
                    <p class="hint" i18n="@@Step_check_hint">
                      The gateway starts unpaused on the new database. Check a route and a sign-in; this tab
                      then says which database it runs on.
                    </p>
                  }
                  @case ('back') {
                    @if (target() === 'postgres') {
                      <app-snippet [filename]="wayBack(b).name" [downloadable]="false" [content]="wayBack(b).content" />
                    } @else {
                      <p class="hint" i18n="@@Step_back_embedded_hint">
                        The PostgreSQL database is untouched: to go back, point the deployment at it
                        again.
                      </p>
                    }
                  }
                }
              </div>
            </li>
          }
        </ol>
      } @else {
        <div class="actions">
          <button matButton="filled" (click)="download()" [disabled]="busy()">
            @if (busy()) {<mat-icon spin>autorenew</mat-icon>} @else {<mat-icon>database</mat-icon>}
            <ng-container i18n="@@Download_a_snapshot">Download a snapshot</ng-container>
          </button>
        </div>
        <div class="actions written-for">
          <h3 i18n="@@How_to_restore">How to restore</h3>
          <span class="grow"></span>
          <ng-container *ngTemplateOutlet="platformPicker" />
        </div>
        <app-snippet [filename]="restoreSnippet(b).name" [downloadable]="false" [content]="restoreSnippet(b).content" />
      }

      @if (b.keyFromEnv) {
        <p class="hint key" i18n="@@Key_from_env_note">
          The master key comes from MEERKAT_VAULT_KEY: the snapshot's vault is useless without
          that variable.
        </p>
      } @else {
        <div class="note warn key">
          <mat-icon>key</mat-icon>
          <p i18n="@@Key_beside_db_note">
            The master key sits next to the database, so backing both up together defeats the
            encryption. Keep it in a secret manager, or supply it through MEERKAT_VAULT_KEY.
          </p>
        </div>
      }

      <!-- The platform the commands are written for: the one this gateway
           runs on, and another for an operator who knows better - a runtime
           that could not be asked, or a move to somewhere else. -->
      <ng-template #platformPicker>
        <mat-form-field class="field platform" subscriptSizing="dynamic">
          <mat-label i18n="@@Written_for">Written for</mat-label>
          <mat-select [value]="written()" (selectionChange)="chosen.set($event.value)">
            @for (n of platformNames; track n) {
              <mat-option [value]="n">{{ platformName(n) }}</mat-option>
            }
          </mat-select>
        </mat-form-field>
      </ng-template>
    }
  </mat-card>
  `,
  styles: `
    .fields {
      display: flex;
      flex-wrap: wrap;
      gap: 0 12px;
      margin-top: 16px;
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
      margin: 0;
      font-size: 1rem;
      font-weight: 500;
    }
    .written-for {
      margin: 20px 0 8px;
    }
    .grow {
      flex: 1;
    }
    .steps {
      list-style: none;
      margin: 0;
      padding: 0;
    }
    .steps li + li {
      margin-top: 16px;
    }
    .step-head {
      display: flex;
      align-items: center;
      gap: 10px;
      font-weight: 500;
    }
    .num {
      width: 22px;
      height: 22px;
      border-radius: 50%;
      border: 1px solid var(--mat-sys-outline);
      display: inline-flex;
      align-items: center;
      justify-content: center;
      font-size: 0.8rem;
    }
    .tick {
      color: var(--mat-sys-primary);
    }
    .step-body {
      margin: 8px 0 0 32px;
    }
    .step-body .actions {
      margin-top: 0;
      margin-bottom: 8px;
    }
    .step-body app-snippet + app-snippet {
      margin-top: 8px;
    }
    .key {
      margin-top: 16px;
    }
    .inline {
      margin-left: 8px;
    }
  `,
})
export class ConfigurationDatabaseComponent {
  private readonly api = inject(ApiService);
  private readonly zone = inject(MeService).timezone;
  private readonly snack = inject(MatSnackBar);
  protected readonly pause = inject(PauseService);
  protected readonly platformNames = platformNames;

  protected readonly busy = signal(false);
  protected readonly backup = signal<BackupInfo | null>(null);
  // The kind of the copy, and for PostgreSQL where it goes.
  protected readonly target = signal<DatabaseKind>('sqlite');
  protected readonly into = signal<'file' | 'server'>('file');
  protected readonly sslModes = ['prefer', 'require', 'verify-full', 'disable'];
  // The target server, one field each. Database and user default to the
  // product's name: what the deployment files and the chart suggest.
  protected readonly t = signal<DatabaseTarget>({
    // Sample values, replaced as they are typed: the SQL and the commands
    // below read them, so the procedure reads whole from the start.
    host: 'postgres.example.internal', port: 5432, database: 'meerkat', user: 'meerkat', password: 'change-me-now', sslmode: 'prefer',
  });
  protected readonly complete = computed(() => {
    const t = this.t();
    return !!(t.host.trim() && t.database.trim() && t.user.trim() && t.password && t.port > 0);
  });
  // What the target said when tested. Cleared as soon as a field changes: an
  // answer about another server is no answer.
  protected readonly probe = signal<DatabaseProbe | null>(null);
  // A test that found a gateway, or a user that may not create tables, is a
  // copy that would fail: not offered.
  protected readonly probeRefuses = computed(() => {
    const p = this.probe();
    return !!p && (!p.empty || !p.canCreate);
  });
  protected readonly probeOk = computed(() => {
    const p = this.probe();
    return !!p && p.empty && p.canCreate;
  });
  // PostgreSQL services the runtime lists beside this gateway, the primary
  // first; poolers and replicas listed with why they will not do.
  protected readonly detected = signal<{ name: string; host: string; port: number; refused?: string }[]>([]);
  protected readonly migration = computed(() => this.target() !== this.backup()?.dialect);
  protected readonly server = computed(() => this.target() === 'postgres' && this.into() === 'server');
  // A copy into a server is a move even from PostgreSQL: the deployment is
  // pointed at another database either way.
  protected readonly moving = computed(() => this.migration() || this.server());
  // The copy the server remembers, when it went where the form points: the
  // copy step is done, and so are the two before it.
  protected readonly copiedHere = computed(() => {
    const c = this.backup()?.lastCopy;
    const t = this.t();
    if (!c) return null;
    return c.host === t.host.trim() && c.port === t.port && c.database === t.database.trim() ? c : null;
  });
  // The platform the commands are written for: the operator's choice, else
  // the one the runtime said.
  protected readonly chosen = signal<PlatformName | null>(null);
  // A gateway run straight from its binary is written for as a container: it
  // is not a way Meerkat is deployed, so it is not offered.
  protected readonly written = computed<PlatformName>(() => {
    const seen = this.backup()?.platform?.name;
    return this.chosen() ?? (seen && seen !== 'binary' ? seen : 'docker');
  });

  protected readonly steps = computed<Step[]>(() => {
    if (this.target() === 'sqlite') return ['pause', 'download', 'switch', 'check', 'back'];
    if (this.into() === 'server') return ['create', 'test', 'pause', 'copy', 'switch', 'check', 'back'];
    return ['create', 'pause', 'download', 'load', 'switch', 'check', 'back'];
  });

  protected readonly sql = computed<Snippet>(() => {
    const t = this.t();
    return createDatabase(this.server() ? t.host.trim() : '', t.port, t.database.trim(), t.user.trim(), t.password);
  });

  constructor() {
    this.read(true);
    this.api.services().subscribe({ next: (d) => this.detected.set(postgresIn(d.services ?? [])), error: () => {} });
  }

  // Reads the backup info. On the first read, a remembered copy brings the
  // procedure back where it was: the target, the fields, the password left
  // for the operator to type again.
  private read(first: boolean): void {
    this.api.backupInfo().subscribe({
      next: (b) => {
        this.backup.set(b);
        if (!first) return;
        this.target.set(b.dialect);
        const c = b.lastCopy;
        if (c) {
          this.target.set('postgres');
          this.into.set('server');
          // The password is never kept: a sample stands in until it is typed.
          this.t.set({ host: c.host, port: c.port, database: c.database, user: c.user, password: 'change-me-now', sslmode: c.sslmode || 'prefer' });
        }
      },
    });
  }

  protected set<K extends keyof DatabaseTarget>(k: K, v: DatabaseTarget[K]): void {
    this.t.update((t) => ({ ...t, [k]: v }));
    this.probe.set(null);
  }

  protected isDone(s: Step): boolean {
    const copied = !!this.copiedHere();
    switch (s) {
      case 'create':
      case 'test':
        return copied || this.probeOk();
      case 'pause':
        return this.pause.paused();
      case 'copy':
        return copied;
      default:
        return false;
    }
  }

  protected stepTitle(s: Step): string {
    switch (s) {
      case 'create':
        return $localize`:@@Step_create:Create the database and its user`;
      case 'test':
        return $localize`:@@Step_test:Test the server`;
      case 'pause':
        return $localize`:@@Step_pause:Pause the gateway`;
      case 'copy':
        return $localize`:@@Step_copy:Copy the database`;
      case 'download':
        return this.target() === 'postgres'
          ? $localize`:@@Step_download_dump:Download the dump`
          : $localize`:@@Step_download_file:Download the database file`;
      case 'load':
        return $localize`:@@Step_load:Load the dump into the new database`;
      case 'switch':
        return $localize`:@@Step_switch:Switch the deployment`;
      case 'check':
        return $localize`:@@Step_check:Check a route and a sign-in`;
      case 'back':
        return $localize`:@@Step_back:The way back`;
    }
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

  protected platformName(n: PlatformName): string {
    switch (n) {
      case 'kubernetes':
        return 'Kubernetes';
      case 'openshift':
        return 'OpenShift';
      case 'swarm':
        return 'Docker Swarm';
      case 'compose':
        return 'Docker Compose';
      case 'docker':
        return $localize`:@@Platform_docker:A Docker container`;
      case 'binary':
        return $localize`:@@Platform_binary:A binary on a host`;
    }
  }

  // The platform inside the sentence: the list writes "A Docker container",
  // the sentence "deployed with a Docker container".
  protected midSentence(label: string): string {
    return /^An? [a-z]/i.test(label) ? label.charAt(0).toLowerCase() + label.slice(1) : label;
  }

  // What the runtime said, with the names it gave: "Kubernetes (Helm
  // release edge, namespace shop)".
  protected platformLabel(p: Platform | undefined): string {
    if (!p) return '';
    const name = this.platformName(p.name);
    if ((p.name === 'kubernetes' || p.name === 'openshift') && p.helm && p.release && p.namespace) {
      return $localize`:@@Platform_helm_release:${name}:platform: (Helm release ${p.release}:release:, namespace ${p.namespace}:namespace:)`;
    }
    if (p.name === 'swarm' && p.workload) return $localize`:@@Platform_swarm_service:${name}:platform: (service ${p.workload}:service:)`;
    if (p.name === 'compose' && p.workload) return $localize`:@@Platform_compose_service:${name}:platform: (service ${p.workload}:service:)`;
    return name;
  }

  // The platform the commands are written for, with the runtime's names when
  // it is the runtime's own, and placeholders when the operator picked another.
  private platform(b: BackupInfo): Platform {
    const n = this.written();
    if (b.platform?.name === n) return b.platform;
    return { name: n, helm: n === 'kubernetes' || n === 'openshift' };
  }

  // For a copy, the target in the form, password included - what is typed is
  // what the commands carry; for a dump, a placeholder to fill.
  private dbUrl(): string {
    const t = this.t();
    if (!this.server()) return 'postgres://meerkat:PASSWORD@HOST:5432/meerkat';
    const ssl = t.sslmode ? '?sslmode=' + t.sslmode : '';
    const pass = encodeURIComponent(t.password || 'PASSWORD');
    return `postgres://${encodeURIComponent(t.user.trim())}:${pass}@${t.host.trim()}:${t.port}/${t.database.trim()}${ssl}`;
  }

  protected switchSnippets(b: BackupInfo): Snippet[] {
    const p = this.platform(b);
    return this.target() === 'postgres' ? switchToPostgres({ b, p, dbUrl: this.dbUrl() }) : switchToEmbedded(b, p);
  }

  protected wayBack(b: BackupInfo): Snippet {
    return wayBackFromPostgres({ b, p: this.platform(b), dbUrl: this.dbUrl() });
  }

  protected restoreSnippet(b: BackupInfo): Snippet {
    return restore(b, this.platform(b));
  }

  protected loadDump(): string {
    return [
      '# into the EMPTY database created above: one transaction, refused whole if cut short',
      `psql -v ON_ERROR_STOP=1 -f meerkat-YYYY-MM-DD.sql "${this.dbUrl()}"`,
    ].join('\n');
  }

  protected when(at: number): string {
    return zoned(at, this.zone());
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

  // The answer is not kept here: the server remembers the copy, and the
  // procedure is rebuilt from what it says - the same after a reload.
  protected copy(): void {
    this.busy.set(true);
    this.api.copyDatabase({ ...this.t(), host: this.t().host.trim() }).subscribe({
      next: () => {
        this.busy.set(false);
        this.read(false);
      },
      error: (err: unknown) => {
        this.busy.set(false);
        this.fail(err);
      },
    });
  }

  protected forget(): void {
    this.api.forgetCopy().subscribe({ next: () => this.read(false), error: (err: unknown) => this.fail(err) });
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
    if (/bouncer|pooler/i.test(name)) refused = $localize`:@@Pg_pooler:a connection pooler, not supported`;
    else if (/replica|-ro$|-r$|repl/i.test(name)) refused = $localize`:@@Pg_replica:a replica, read only`;
    const rank = refused ? 2 : /primary|-rw$/i.test(name) ? 0 : 1;
    out.push({ name, host: s.names?.[0] ?? name, port, refused, rank });
  }
  return out.sort((a, b) => a.rank - b.rank || a.name.localeCompare(b.name));
}
