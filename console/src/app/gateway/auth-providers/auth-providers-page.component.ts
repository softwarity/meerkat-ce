import { Component, computed, inject, signal } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatSidenavModule } from '@angular/material/sidenav';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTableModule } from '@angular/material/table';
import { MatTooltipModule } from '@angular/material/tooltip';
import { ActivatedRoute, Router } from '@angular/router';
import { LoadingIndicatorComponent } from '@softwarity/loading-indicator';
import { ApiService, AuthProvider, Settings } from '../../api.service';
import { AuthProviderEditorComponent } from './auth-provider-editor.component';
import { LocalProviderEditorComponent } from './local-provider-editor.component';

// External authentication (AUTH-19), infra plane: the authorities people may
// sign in through. A directory or an identity provider is a third-party
// service reached by URL, with credentials, exactly like a route's upstream.
// What it grants once someone is in stays the application's business: a first
// sign-in creates an account that reaches nothing until an admin places it.
@Component({
  selector: 'app-auth-providers-page',
  imports: [
    MatButtonModule,
    MatIconModule,
    MatSidenavModule,
    MatSlideToggleModule,
    MatTableModule,
    MatTooltipModule,
    LoadingIndicatorComponent,
    AuthProviderEditorComponent,
    LocalProviderEditorComponent,
  ],
  templateUrl: './auth-providers-page.component.html',
  styleUrl: './auth-providers-page.component.scss',
})
export class AuthProvidersPageComponent {
  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);
  private readonly router = inject(Router);
  private readonly ar = inject(ActivatedRoute);

  protected readonly loading = signal(true);
  protected readonly providers = signal<AuthProvider[]>([]);
  protected readonly columns = ['name', 'kind', 'target', 'state'];
  // The row whose switch is in flight, so it cannot be clicked twice.
  protected readonly toggling = signal('');

  // The URL drives the drawer, like routes and roles.
  private readonly params = toSignal(this.ar.paramMap);
  private readonly urlSegs = toSignal(this.ar.url);
  protected readonly editing = computed<AuthProvider | 'new' | null>(() => {
    if (this.urlSegs()?.some((s) => s.path === 'new')) return 'new';
    const id = this.params()?.get('id');
    if (!id) return null;
    return this.providers().find((p) => p.id === id) ?? null;
  });
  // What a derived identifier must not collide with (the editor suffixes).
  protected readonly takenIds = computed(() => this.providers().map((p) => p.id));

  protected readonly editingProvider = computed(() => {
    const e = this.editing();
    return e === null || e === 'new' ? null : e;
  });

  // The local authority has its own, much smaller, editor.
  protected readonly editingLocal = computed(() => {
    const p = this.editingProvider();
    return p && this.isLocal(p) ? p : null;
  });

  // The master switch over every authority (AUTH-20): it lives here because
  // this is the list of doors, and it is the one that closes all of them.
  protected readonly signUpOpen = signal(false);
  protected readonly savingSignUp = signal(false);
  private settings: Settings | null = null;

  constructor() {
    this.load();
    this.api.settings().subscribe({
      next: (s) => {
        this.settings = s;
        this.signUpOpen.set(s.selfRegistration);
      },
    });
  }

  protected setSignUpOpen(on: boolean): void {
    const s = this.settings;
    if (!s) return;
    this.signUpOpen.set(on);
    this.savingSignUp.set(true);
    this.api.saveSettings({ ...s, selfRegistration: on }).subscribe({
      next: (saved) => {
        this.settings = saved;
        this.savingSignUp.set(false);
      },
      error: (err) => {
        this.signUpOpen.set(s.selfRegistration);
        this.savingSignUp.set(false);
        this.snack.open(
          typeof (err as { error?: { error?: string } })?.error?.error === 'string'
            ? (err as { error: { error: string } }).error.error
            : $localize`:@@Save_failed:Save failed`,
          undefined,
          { duration: 4000 },
        );
      },
    });
  }

  load(): void {
    this.loading.set(true);
    this.api.authProviders().subscribe({
      next: (list) => {
        this.providers.set(list);
        this.loading.set(false);
      },
      error: () => this.loading.set(false),
    });
  }

  // The accounts held here (AUTH-24) are an authority in this list with no
  // third party to reach - but two questions of their own: may someone sign
  // themselves up, and is that form guarded. Its editor is those two.
  protected isLocal(p: AuthProvider): boolean {
    return p.kind === 'local';
  }

  // What an authority points at, in one line: the issuer, or the server.
  protected target(p: AuthProvider): string {
    const cfg = p.config ?? {};
    // GitHub has no address to show: what identifies the setup is the
    // organisation it lets in.
    if (p.kind === 'github') {
      const orgs = cfg['allowedOrgs'];
      return Array.isArray(orgs) && orgs.length ? orgs.join(', ') : 'github.com';
    }
    return String(cfg['issuer'] ?? cfg['url'] ?? '');
  }

  protected kindLabel(p: AuthProvider): string {
    switch (p.kind) {
      case 'local':
        return $localize`:@@Kind_local_accounts:Local accounts`;
      case 'oidc':
        return 'OpenID Connect';
      case 'ldap':
        return String(p.config?.['dialect'] ?? '') === 'ad'
          ? 'Active Directory'
          : $localize`:@@Kind_directory:Directory`;
      case 'github':
        return 'GitHub';
      default:
        return 'SAML';
    }
  }

  // Enabling an authority is a decision, not an edit, so it happens on the row
  // and saves at once. The whole provider is sent back: the server carries the
  // secrets forward, so a toggle never costs an authority its client secret.
  protected toggle(p: AuthProvider, enabled: boolean): void {
    this.toggling.set(p.id);
    this.api.saveAuthProvider({ ...p, enabled }).subscribe({
      next: (saved) => {
        this.providers.update((list) => list.map((x) => (x.id === saved.id ? saved : x)));
        this.toggling.set('');
      },
      error: (err: unknown) => {
        this.toggling.set('');
        const e = err as { error?: { error?: string } };
        this.snack.open(
          typeof e?.error?.error === 'string' ? e.error.error : $localize`:@@Save_failed:Save failed`,
          undefined,
          { duration: 6000 },
        );
        // Put the switch back where the server left it.
        this.load();
      },
    });
  }

  protected open(p: AuthProvider): void {
    void this.router.navigate(['/infra/auth-providers', p.id]);
  }

  protected openNew(): void {
    void this.router.navigate(['/infra/auth-providers', 'new']);
  }

  protected closeEditor(): void {
    if (this.editing() !== null) void this.router.navigate(['/infra/auth-providers']);
  }

  protected onSaved(saved: AuthProvider): void {
    this.snack.open($localize`:@@Provider_NAME_saved:Authority "${saved.name}:NAME:" saved`, undefined, {
      duration: 2500,
    });
    if (this.editing() === 'new') {
      void this.router.navigate(['/infra/auth-providers', saved.id], { replaceUrl: true });
    }
    this.load();
  }

  protected onDeleted(): void {
    void this.router.navigate(['/infra/auth-providers']);
    this.load();
  }
}
