import { Component, computed, inject, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTableModule } from '@angular/material/table';
import { MatTooltipModule } from '@angular/material/tooltip';
import { RowActionsDirective } from '@softwarity/row-actions';
import { ApiService, ConfigForge, ConfigRemote, ConfigRemoteCheck } from '../../api.service';
import { DialogsService } from '../../shared/dialogs.service';
import { FormFieldComponent } from '../../shared/form-field.component';
import { SecretFieldComponent } from '../../shared/secret-field.component';

// The named git locations (CFG-07), in the drawer: the form at the top, the
// locations already there underneath.
//
// One panel rather than a dialog, and not only for consistency with the rest of
// this screen: setting these up is a LIST-EDITING job - add Acme, duplicate it
// for Foo with one directory changed, fix a branch name - and a modal that
// covers the list while you fill the form makes you close it to see what you
// already have. The form stays put, the rows stay visible, and duplicating one
// fills the form from it.
//
// A LOCATION IS A DIRECTORY, not a file, and that is the whole multi-platform
// story: several locations share one repository, one directory each, and the
// layout inside is the one an export already produces - meerkat.yaml with its
// assets/ beside it. So there is no file to pick and no name to agree on.
//
// WHAT THE FORGE WANTS IS SHOWN, NOT ASSUMED. Every forge decided the basic-auth
// username differently - GitHub ignores it, GitLab wants oauth2, Bitbucket
// refuses anything but x-token-auth - and all of them answer a wrong one with
// the same "authentication failed" a wrong token gives. An operator with a
// perfectly good token can lose an afternoon to that, so the username has a
// default per host and the permissions to grant are written beside the field.
@Component({
  selector: 'app-git-locations-panel',
  imports: [
    MatButtonModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatSelectModule,
    MatTableModule,
    MatTooltipModule,
    RowActionsDirective,
    FormFieldComponent,
    SecretFieldComponent,
  ],
  styles: [
    `
      :host {
        display: flex;
        flex-direction: column;
        height: 100%;
      }
      header {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 12px 16px;
        border-bottom: 1px solid var(--mat-sys-outline-variant);
      }
      header .grow {
        flex: 1;
        min-width: 0;
      }
      h2 {
        margin: 0;
        font-size: 1.05rem;
        font-weight: 500;
      }
      .sub {
        margin: 2px 0 0;
        font-size: 0.8rem;
        color: var(--mat-sys-on-surface-variant);
      }
      .body {
        flex: 1;
        min-height: 0;
        display: flex;
        flex-direction: column;
        padding: 16px;
      }
      /* The list takes every row the form leaves, and scrolls on its own: a
         drawer that scrolls as one block hides the rows behind the form the
         moment there are more than a handful. */
      .list {
        flex: 1;
        min-height: 0;
        overflow: auto;
      }
      /* A GRID, and that is what sizes every field including the secret one.
         app-form-field is an inline-block and app-secret-field is
         display:contents on purpose, so neither takes a width from a rule this
         component could write - encapsulation stops at the child's template. A
         grid sizes its items by TRACK instead, so they fill their column
         without anything reaching into them. (Same reason the OpenTelemetry
         screen uses one.) */
      .form {
        display: grid;
        grid-template-columns: repeat(2, minmax(0, 1fr));
        align-items: start;
        column-gap: 12px;
      }
      .form > .full {
        grid-column: 1 / -1;
      }
      .actions {
        display: flex;
        align-items: center;
        gap: 12px;
        margin: 4px 0 8px;
      }
      .actions .grow {
        flex: 1;
      }
      /* What the forge wants is ONE line with a link, not a paragraph: the
         detail is a hover away on the token's info icon, and every line spent
         here is a line the list below does not get. */
      .forge {
        display: flex;
        align-items: center;
        gap: 8px;
        margin: -4px 0 12px;
        font-size: 0.8rem;
        color: var(--mat-sys-on-surface-variant);
      }
      .forge mat-icon {
        font-size: 18px;
        width: 18px;
        height: 18px;
      }
      .creds {
        margin: 0 0 12px;
        font-size: 0.82rem;
        color: var(--mat-sys-on-surface-variant);
      }
      .creds h3 {
        margin: 4px 0 4px;
        font-size: 0.85rem;
        font-weight: 500;
        color: var(--mat-sys-on-surface);
      }
      .creds ol {
        margin: 0 0 6px;
        padding-left: 20px;
        line-height: 1.45;
      }
      .creds a {
        display: inline-flex;
        align-items: center;
        gap: 4px;
        color: var(--mat-sys-primary);
      }
      .creds a mat-icon {
        font-size: 16px;
        width: 16px;
        height: 16px;
      }
      .fixed-user {
        margin: -4px 0 12px;
        font-size: 0.8rem;
        color: var(--mat-sys-on-surface-variant);
      }
      .verdict {
        display: flex;
        align-items: center;
        gap: 12px;
        padding: 10px 14px;
        border-radius: 8px;
        background: var(--mat-sys-surface-container);
        margin: 0 0 12px;
        font-size: 0.85rem;
      }
      .verdict.bad {
        background: var(--mat-sys-error-container);
        color: var(--mat-sys-on-error-container);
      }
      .verdict mat-icon {
        flex-shrink: 0;
      }
      .verdict p {
        margin: 0;
      }
      h3 {
        margin: 24px 0 4px;
        font-size: 0.9rem;
        font-weight: 500;
        color: var(--mat-sys-on-surface-variant);
      }
      mat-table {
        background: transparent;
      }
      mat-cell,
      mat-header-cell {
        padding: 0 8px;
      }
      .mat-column-name {
        flex: 0 0 180px;
      }
      .where {
        display: flex;
        flex-direction: column;
        min-width: 0;
        font-family: var(--mk-mono, monospace);
        font-size: 0.75rem;
      }
      .where .url,
      .where .ref {
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .where .ref {
        color: var(--mat-sys-on-surface-variant);
      }
      .who {
        display: flex;
        flex-direction: column;
        min-width: 0;
      }
      .used {
        font-size: 0.75rem;
        color: var(--mat-sys-on-surface-variant);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
      }
      .empty {
        margin: 8px 0 0;
        font-size: 0.85rem;
        color: var(--mat-sys-on-surface-variant);
      }
    `,
  ],
  template: `
    <header>
      <h2 class="grow" i18n="@@Git_locations">Git locations</h2>
      <button
        matIconButton
        [matTooltip]="help()"
        i18n-aria-label="@@What_is_a_git_location"
        aria-label="What is a git location"
      >
        <mat-icon>help_outline</mat-icon>
      </button>
      <button matIconButton (click)="closed.emit()" i18n-aria-label="@@Close" aria-label="Close">
        <mat-icon>close</mat-icon>
      </button>
    </header>

    <div class="body">
      <div class="form">
        <!-- The forge first: it decides what is left to type, the username sent
             beside the token, and how that token is made. -->
        <mat-form-field class="full">
          <mat-label i18n="@@Git_forge">Forge</mat-label>
          <mat-select [value]="provider().id" (valueChange)="pick($event)">
            @for (f of choices(); track f.id) {
              <mat-option [value]="f.id">{{ f.name }}</mat-option>
            }
          </mat-select>
        </mat-form-field>

        <app-form-field
          class="full"
          i18n-label="@@Name"
          label="Name"
          i18n-info="@@Git_location_name_info"
          info="What you call this platform. It is the name you pick from when pulling or pushing."
        >
          <input matInput [value]="form().name || ''" (input)="set('name', $any($event.target).value)" />
        </app-form-field>

        @if (provider().host) {
          <!-- A known forge: its host is filled in, only the repository is
               typed. The URL is made from the two. -->
          @if (provider().hostEditable) {
            <app-form-field
              i18n-label="@@Git_host"
              label="Host"
              i18n-info="@@Git_host_info"
              info="This forge's own host is filled in; change it for a self-hosted instance."
            >
              <input
                matInput
                [value]="host()"
                (input)="setHost($any($event.target).value)"
                spellcheck="false"
              />
            </app-form-field>
          }
          <app-form-field
            [class.full]="!provider().hostEditable"
            icon="link"
            i18n-label="@@Git_repository"
            label="Repository"
            [info]="urlPreview()"
          >
            <input
              matInput
              [value]="path()"
              (input)="setPath($any($event.target).value)"
              [placeholder]="provider().path ?? ''"
              spellcheck="false"
            />
          </app-form-field>
        } @else {
          <app-form-field
            class="full"
            icon="link"
            i18n-label="@@Repository_URL"
            label="Repository URL"
            i18n-info="@@Repository_URL_info"
            info="An https URL. SSH is not supported: it would need a private key in the vault and a host-key policy."
          >
            <input
              matInput
              [value]="form().url || ''"
              (input)="set('url', $any($event.target).value)"
              placeholder="https://github.com/acme/meerkat-config.git"
              spellcheck="false"
            />
          </app-form-field>
        }

        <!-- An info icon it would not strictly need: that icon is a GUTTER,
             and a field without one runs past the field beside it. -->
        <app-form-field
          i18n-label="@@Branch"
          label="Branch"
          i18n-info="@@Branch_info"
          info="The branch this directory lives on. A push is refused if somebody else has moved it since this configuration was read - there is no force."
        >
          <input
            matInput
            [value]="form().branch || ''"
            (input)="set('branch', $any($event.target).value)"
            spellcheck="false"
          />
        </app-form-field>

        <app-form-field
          i18n-label="@@Directory"
          label="Directory"
          i18n-info="@@Directory_info"
          info="The directory inside the repository, empty for its root. This is what lets one repository hold one directory per platform."
        >
          <input
            matInput
            [value]="form().dir || ''"
            (input)="set('dir', $any($event.target).value)"
            placeholder="platforms/acme"
            spellcheck="false"
          />
        </app-form-field>

        <!-- How the token is made, in this forge's own menus, with the link to
             the very page - the steps are the afternoon this screen saves. -->
        <div class="full creds">
          <h3 i18n="@@Git_token_steps">The access token</h3>
          <ol>
            @for (step of provider().steps ?? []; track step) {
              <li>{{ step }}</li>
            }
          </ol>
          @if (createLink(); as link) {
            <a [href]="link" target="_blank" rel="noopener">
              <mat-icon>open_in_new</mat-icon>
              <ng-container i18n="@@Git_open_token_page">Open the page that makes it</ng-container>
            </a>
          } @else if (provider().create) {
            <span class="wait" i18n="@@Git_fill_repository_first"
              >Fill in the repository first: the link opens its own token page.</span
            >
          }
        </div>

        <!-- No at= here: unlike a relay password or a client secret, a token
             never sits in this table as a literal to be moved out - the API
             refuses one outright - so the only path is paste it, stash it in
             the vault, keep the reference. -->
        <app-secret-field
          [class.full]="!userShown()"
          i18n-label="@@Access_token"
          label="Access token"
          [info]="tokenInfo()"
          scope="infra"
          [value]="form().tokenRef || ''"
          (valueChange)="set('tokenRef', $event)"
        />

        @if (userShown()) {
          <app-form-field
            i18n-label="@@Token_username"
            label="Token username"
            i18n-info="@@Token_username_info2"
            info="Sent beside the token: on Gitea and Forgejo, the account the token belongs to; on another server, the name it expects."
          >
            <input
              matInput
              [value]="form().tokenUser || ''"
              (input)="set('tokenUser', $any($event.target).value)"
              [placeholder]="provider().user"
              spellcheck="false"
            />
          </app-form-field>
        } @else if (provider().userFixed) {
          <p class="full fixed-user" i18n="@@Git_user_fixed">
            Sent with the username <code>{{ provider().user }}</code>, the only one {{ provider().name }} accepts beside this token.
          </p>
        }

        <app-form-field
          class="full"
          i18n-label="@@Fallback_commit_author"
          label="Fallback commit author"
          i18n-info="@@Fallback_commit_author_info"
          info="A commit is attributed to whoever pushed it. This address is used only for an account that has none of its own."
        >
          <input
            matInput
            [value]="form().authorEmail || ''"
            (input)="set('authorEmail', $any($event.target).value)"
            placeholder="ops@acme.com"
            spellcheck="false"
          />
        </app-form-field>
      </div>

      @if (verdict(); as v) {
        <div class="verdict" [class.bad]="!v.ok">
          <mat-icon>{{ v.ok ? 'check_circle' : 'error' }}</mat-icon>
          @if (v.ok) {
            @if (v.holds) {
              <p i18n="@@Check_ok_holds">It answers, and it already holds a configuration you can pull.</p>
            } @else {
              <p i18n="@@Check_ok_empty">It answers, and there is nothing there yet: push one to it.</p>
            }
          } @else {
            <p>{{ v.error }}</p>
          }
        </div>
      }

      <div class="actions">
        <div class="grow"></div>
        @if (dirty()) {
          <button matButton (click)="clear()" i18n="@@Cancel">Cancel</button>
        }
        <button matButton="tonal" [disabled]="!complete() || busy()" (click)="check()">
          <mat-icon>wifi_tethering</mat-icon>
          <ng-container i18n="@@Check">Check</ng-container>
        </button>
        <button matButton="filled" [disabled]="!complete() || busy()" (click)="save()">
          <mat-icon>{{ editingId() ? 'check' : 'add' }}</mat-icon>
          @if (editingId()) {
            <ng-container i18n="@@Save">Save</ng-container>
          } @else {
            <ng-container i18n="@@Add">Add</ng-container>
          }
        </button>
      </div>

      <div class="list">
        @if (remotes().length === 0) {
          <p class="empty" i18n="@@No_git_location">
            None yet. Fill the form above.
          </p>
        } @else {
          <mat-table [dataSource]="remotes()">
            <ng-container matColumnDef="name">
              <mat-header-cell *matHeaderCellDef i18n="@@Name">Name</mat-header-cell>
              <mat-cell *matCellDef="let r">
                <span class="who">
                  <span>{{ r.name }}</span>
                  <!-- What uses it, in words. It was a tooltip repeating the
                       cell beside it and a "(1)" nobody could read. -->
                  @if (r.bound?.length) {
                    <span class="used" i18n="@@Used_by">used by {{ r.bound.join(', ') }}</span>
                  }
                </span>
              </mat-cell>
            </ng-container>

            <!-- The row actions live in the LAST existing column: a column of
                 nothing but buttons is a column of wasted width. -->
            <ng-container matColumnDef="where">
              <mat-header-cell *matHeaderCellDef i18n="@@Where">Where</mat-header-cell>
              <mat-cell *matCellDef="let r">
                <!-- Two lines rather than one long one: the repository above,
                     what to read in it below. A row that runs to 90 characters
                     is a row whose end is under the hover toolbar. -->
                <span class="where">
                  <span class="url">{{ url(r) }}</span>
                  <span class="ref">{{ ref(r) }}</span>
                </span>
                <span rowActions="tonal">
                  <button
                    matIconButton
                    (click)="edit(r)"
                    i18n-matTooltip="@@Edit"
                    matTooltip="Edit"
                    i18n-aria-label="@@Edit"
                    aria-label="Edit"
                  >
                    <mat-icon>edit</mat-icon>
                  </button>
                  <!-- Duplicating is how the second platform gets made: the
                       same repository and branch, one directory changed. -->
                  <button
                    matIconButton
                    (click)="duplicate(r)"
                    i18n-matTooltip="@@Duplicate"
                    matTooltip="Duplicate"
                    i18n-aria-label="@@Duplicate"
                    aria-label="Duplicate"
                  >
                    <mat-icon>content_copy</mat-icon>
                  </button>
                  <button
                    matIconButton
                    (click)="remove(r)"
                    i18n-matTooltip="@@Delete"
                    matTooltip="Delete"
                    i18n-aria-label="@@Delete"
                    aria-label="Delete"
                  >
                    <mat-icon>delete</mat-icon>
                  </button>
                </span>
              </mat-cell>
            </ng-container>

            <mat-header-row *matHeaderRowDef="columns"></mat-header-row>
            <mat-row *matRowDef="let row; columns: columns"></mat-row>
          </mat-table>
        }
      </div>
    </div>
  `,
})
export class GitLocationsPanelComponent {
  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);
  private readonly dialogs = inject(DialogsService);

  readonly closed = output<void>();

  protected readonly columns = ['name', 'where'];
  protected readonly remotes = signal<ConfigRemote[]>([]);
  // The row being edited, "" for a new one. The form is always there; this only
  // says whether saving adds or replaces.
  protected readonly editingId = signal('');
  protected readonly form = signal<Partial<ConfigRemote>>({ branch: 'main' });
  protected readonly busy = signal(false);
  protected readonly verdict = signal<ConfigRemoteCheck | null>(null);
  private readonly forges = signal<ConfigForge[]>([]);
  private readonly generic = signal<ConfigForge | null>(null);

  // The forge picked - for a row from before the choice, the one its URL's host
  // gives away. It decides what is left to type, the username sent beside the
  // token, and the steps that make that token.
  protected readonly choices = computed(() => [
    ...this.forges(),
    ...(this.generic() ? [this.generic()!] : []),
  ]);
  protected readonly provider = computed<ConfigForge>(() => {
    const id = this.form().provider;
    return (
      this.choices().find((f) => f.id === id) ??
      this.byHost(this.form().url ?? '') ??
      this.forges().find((f) => f.id === 'github') ?? {
        id: 'generic', name: '', user: 'git', needs: '',
      }
    );
  });

  // A known forge's URL is made of its host and the repository typed after it.
  protected readonly host = signal('');
  protected readonly path = signal('');
  protected readonly urlPreview = computed(() => this.form().url || (this.provider().path ?? ''));

  // The token page of THIS repository, once enough is typed to name it.
  protected readonly createLink = computed(() => {
    const create = this.provider().create;
    if (!create) return '';
    const path = this.path().replace(/\.git$/, '');
    if (create.includes('{path}') && !path) return '';
    return create.replace('{host}', this.host() || this.provider().host || '').replace('{path}', path);
  });

  // The username is a choice only where the forge leaves it to you: Gitea and
  // Forgejo want the token's own account, another server its own rule. GitHub
  // and Azure ignore it; GitLab and Bitbucket accept only theirs.
  protected readonly userShown = computed(() => ['gitea', 'generic'].includes(this.provider().id));

  // The explanation, once, behind the header's help icon rather than as a
  // paragraph under the title: it is read on the first visit and never again,
  // and every line it takes is a line the list below does not get. Two
  // sentences, because a tooltip is not where a page of prose goes - the rest
  // is in the documentation.
  protected readonly help = () =>
    $localize`:@@Git_location_help:One directory of one branch, so the same repository can hold one per platform. Pulling from a location applies nothing.`;

  // What the forge wants, on the token's own info icon. It is the one piece of
  // text that saves an afternoon, and a tooltip is where a long sentence can
  // live without pushing the list off the screen.
  protected readonly tokenInfo = computed(() => {
    const f = this.provider();
    const rule = $localize`:@@Token_is_a_reference:A vault reference, never the token itself: paste it and the key button puts it in the vault.`;
    return f ? rule + ' ' + f.needs + (f.note ? ' ' + f.note : '') : rule;
  });

  protected readonly complete = computed(() => {
    const f = this.form();
    return !!(f.name ?? '').trim() && !!(f.url ?? '').trim() && !!(f.branch ?? '').trim();
  });

  // Whether there is anything to cancel: an untouched form has nothing to
  // throw away, and a Cancel button beside it would be asking about nothing.
  protected readonly dirty = computed(() => {
    const f = this.form();
    return !!this.editingId() || !!(f.name ?? '').trim() || !!(f.url ?? '').trim();
  });

  constructor() {
    this.reload();
    this.api.configForges().subscribe({
      next: (t) => {
        this.forges.set(t.forges);
        this.generic.set(t.generic);
        if (!this.form().provider) this.pick('github');
      },
      error: () => this.forges.set([]),
    });
  }

  private reload(): void {
    this.api.configRemotes().subscribe({
      next: (list) => this.remotes.set(list),
      error: () => this.remotes.set([]),
    });
  }

  // The repository, trimmed to what tells one from another: the scheme and the
  // trailing .git say nothing.
  protected url(r: ConfigRemote): string {
    return r.url.replace(/^https?:\/\//, '').replace(/\.git$/, '');
  }

  // Where to read inside it, in git's own shorthand: branch@directory, and the
  // branch alone when the directory is the repository root.
  protected ref(r: ConfigRemote): string {
    return r.dir ? r.branch + '@' + r.dir : r.branch;
  }

  protected clear(): void {
    this.editingId.set('');
    this.form.set({ branch: 'main', provider: this.provider().id });
    this.splitUrl('');
    this.verdict.set(null);
  }

  protected edit(r: ConfigRemote): void {
    this.editingId.set(r.id);
    this.form.set({ ...r, provider: r.provider || this.byHost(r.url)?.id || 'generic' });
    this.splitUrl(r.url);
    this.verdict.set(null);
  }

  // Another forge: its host is filled in, the repository typed so far kept.
  protected pick(id: string): void {
    const f = this.choices().find((c) => c.id === id);
    this.form.update((x) => ({ ...x, provider: id, tokenUser: '' }));
    if (f?.host) {
      this.host.set(f.host);
      this.compose();
    }
    this.verdict.set(null);
  }

  protected setHost(v: string): void {
    this.host.set(v.trim().replace(/^https?:\/\//, '').replace(/\/+$/, ''));
    this.compose();
  }

  protected setPath(v: string): void {
    this.path.set(v.trim().replace(/^\/+/, '').replace(/\.git$/, ''));
    this.compose();
  }

  // The URL a known forge's fields make: https://host/path.git.
  private compose(): void {
    const host = this.host() || this.provider().host || '';
    const path = this.path();
    this.set('url', host && path ? `https://${host}/${path}.git` : '');
  }

  // The other way round, for a row being edited.
  private splitUrl(url: string): void {
    try {
      const u = new URL(url);
      this.host.set(u.host);
      this.path.set(u.pathname.replace(/^\/+/, '').replace(/\.git$/, ''));
    } catch {
      this.host.set(this.provider().host ?? '');
      this.path.set('');
    }
  }

  private byHost(url: string): ConfigForge | undefined {
    const host = this.hostOf(url);
    if (!host) return undefined;
    return this.forges().find((f) => (f.hosts ?? []).some((h) => host === h || host.endsWith('.' + h)));
  }

  // A copy keeps the repository, the branch and the credential, and takes a new
  // name - the directory is the one thing meant to change, so it is left as it
  // was for the operator to edit rather than blanked.
  protected duplicate(r: ConfigRemote): void {
    this.editingId.set('');
    this.form.set({
      ...r,
      id: '',
      name: r.name + ' (copy)',
      provider: r.provider || this.byHost(r.url)?.id || 'generic',
    });
    this.splitUrl(r.url);
    this.verdict.set(null);
  }

  protected set<K extends keyof ConfigRemote>(key: K, value: ConfigRemote[K]): void {
    this.form.update((f) => ({ ...f, [key]: value }));
    // A field changed makes the last verdict about something else.
    this.verdict.set(null);
  }

  protected save(): void {
    this.busy.set(true);
    this.api.saveConfigRemote(this.form(), this.editingId() || undefined).subscribe({
      next: () => {
        this.busy.set(false);
        this.clear();
        this.reload();
      },
      error: (err: unknown) => {
        this.busy.set(false);
        this.fail(err);
      },
    });
  }

  // Checking SAVES first: the check reaches a repository with a token taken out
  // of the vault, and the server needs a row to resolve it from. Saving a
  // location changes nothing anywhere - it is a destination, not a document -
  // so this costs nothing and it keeps the form from needing a second,
  // credential-carrying endpoint of its own.
  protected check(): void {
    this.busy.set(true);
    this.api.saveConfigRemote(this.form(), this.editingId() || undefined).subscribe({
      next: (saved) => {
        this.editingId.set(saved.id);
        this.form.set({ ...saved });
        this.reload();
        this.api.checkConfigRemote(saved.id).subscribe({
          next: (v) => {
            this.busy.set(false);
            this.verdict.set(v);
          },
          error: (err: unknown) => {
            this.busy.set(false);
            this.fail(err);
          },
        });
      },
      error: (err: unknown) => {
        this.busy.set(false);
        this.fail(err);
      },
    });
  }

  protected async remove(r: ConfigRemote): Promise<void> {
    const ok = await this.dialogs.confirm({
      title: $localize`:@@Delete_git_location:Delete ${r.name}:name:?`,
      message: r.bound?.length
        ? $localize`:@@Delete_git_location_bound:${r.bound.join(', ')}:bound: stay as saved copies, no longer bound anywhere. Nothing is removed from the repository.`
        : $localize`:@@Delete_git_location_message:Nothing is removed from the repository: this forgets where it is.`,
      confirmLabel: $localize`:@@Delete:Delete`,
      danger: true,
    });
    if (!ok) return;
    this.api.deleteConfigRemote(r.id).subscribe({
      next: () => {
        if (this.editingId() === r.id) this.clear();
        this.reload();
      },
      error: (err: unknown) => this.fail(err),
    });
  }

  private hostOf(url: string): string {
    try {
      return new URL(url.trim()).hostname.toLocaleLowerCase();
    } catch {
      return '';
    }
  }

  private fail(err: unknown): void {
    const e = err as { error?: { error?: string } };
    this.snack.open(
      typeof e?.error?.error === 'string' ? e.error.error : $localize`:@@Request_failed:Request failed`,
      undefined,
      { duration: 6000 },
    );
  }
}
