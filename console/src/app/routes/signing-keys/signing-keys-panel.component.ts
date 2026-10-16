import { DOCUMENT } from '@angular/common';
import { Component, computed, inject, input, output, signal } from '@angular/core';
import { HttpErrorResponse } from '@angular/common/http';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatExpansionModule } from '@angular/material/expansion';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatSelectModule } from '@angular/material/select';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTableModule } from '@angular/material/table';
import { MatTooltipModule } from '@angular/material/tooltip';
import { RouterLink } from '@angular/router';
import { RowActionsDirective } from '@softwarity/row-actions';
import { ApiService, OIDCClient, Route, SigningKeys } from '../../api.service';
import { DialogsService } from '../../shared/dialogs.service';
import { LiveChangesService } from '../../shared/live-changes.service';
import { EeLockComponent } from '../../shared/ee-lock.component';

// The identity JWT signing keys (signed-jwt): the JWKS the backends should
// verify against (the recommended path), plus each algorithm's PUBLIC key as a
// collapsed static fallback, and a rotate button. Private keys stay on the
// server - only public halves are shown.
//
// Its OWN drawer, beside Global rather than inside it. Global answers "what is
// true of every route at once" - a switch, two waits, a ceiling: settings, each
// one line long. These are keys, with a rotation, a JWKS to hand to a backend
// and three PEMs to copy, and they filled more of that drawer than the three
// settings together. A subject that large is a subject of its own.
//
// "OIDC / JWT" since the OIDC provider: its clients sit here because the
// issuer and the keys are the provider's too - the tokens an application gets
// when it signs in through Meerkat are signed with the same keys, under the
// same issuer, as the ones a route forwards.
@Component({
  selector: 'app-signing-keys-panel',
  imports: [
    EeLockComponent,
    MatButtonModule,
    MatExpansionModule,
    MatIconModule,
    MatProgressBarModule,
    MatSelectModule,
    MatTableModule,
    MatTooltipModule,
    RouterLink,
    RowActionsDirective,
  ],
  styleUrl: './signing-keys-panel.component.scss',
  templateUrl: './signing-keys-panel.component.html',
})
export class SigningKeysPanelComponent {
  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);
  private readonly dialog = inject(MatDialog);
  private readonly dialogs = inject(DialogsService);

  // The page's routes, handed down so a key can say who uses it. It is the
  // question three identical-looking rows never answered - and the reason they
  // read as one thing shown three times.
  readonly routes = input<Route[]>([]);
  // A route named here, and the section of it that put it in this drawer.
  readonly routePicked = output<{ id: string; section: 'identity' | 'authforward' }>();
  readonly closed = output<void>();

  // Which routes sign with each algorithm. A route on signed-jwt with no
  // algorithm named uses ES256: that is the server's fallback, so counting it
  // anywhere else would make this list disagree with what is actually signed.
  protected readonly users = computed(() => {
    const by = new Map<string, Route[]>();
    for (const r of this.routes()) {
      // An OIDC route's tokens are signed by the provider, with RS256: the one
      // algorithm every OIDC library verifies. Rotating that key concerns it.
      const alg =
        r.identity?.mechanism === 'signed-jwt'
          ? r.identity.algorithm || 'ES256'
          : r.identity?.mechanism === 'oidc'
            ? 'RS256'
            : '';
      if (!alg) continue;
      by.set(alg, [...(by.get(alg) ?? []), r]);
    }
    return by;
  });

  protected usersOf(alg: string): Route[] {
    return this.users().get(alg) ?? [];
  }

  // Which image serves this console (ee-lock.component.ts): the community
  // one shows the provider's public discovery locked, never as an address.
  protected readonly enterprise = inject(DOCUMENT).body.classList.contains('ee');

  protected readonly loading = signal(true);
  protected readonly renewing = signal(false);
  protected readonly confirming = signal(false);
  protected readonly data = signal<SigningKeys | null>(null);

  // Where a service inside the cluster reads the JWKS: the internal plane, by
  // the name our deployment files give the gateway. The port is the
  // gateway's answer; the name is the deployment's.
  protected readonly jwksUrl = computed(() => this.internalUrl(this.data()?.jwksPath));
  // The discovery document: the issuer, and where its keys are. What a
  // library that starts from the issuer reads first.
  protected readonly discoveryUrl = computed(() => this.internalUrl(this.data()?.discoveryPath));

  // The runtime's own name for the internal plane when it gave one; else the
  // name our deployment files give the gateway.
  private internalUrl(path: string | undefined): string {
    const d = this.data();
    if (!d || !path) return '';
    return (d.internalOrigin || `http://meerkat:${d.internalPort || '7070'}`) + path;
  }

  // The issuer, chosen among the application plane's public addresses
  // (Domains & TLS): never typed, since it has to be an address the gateway
  // answers on. Saved as soon as it is picked; the server refuses anything
  // else, naming what it accepts.
  protected readonly savingIssuer = signal(false);

  protected chooseIssuer(issuer: string): void {
    const d = this.data();
    if (!d || issuer === d.issuer) return;
    this.savingIssuer.set(true);
    this.api.setIssuer(issuer).subscribe({
      next: (out) => {
        this.data.set(out);
        this.savingIssuer.set(false);
        this.snack.open($localize`:@@Issuer_saved:Issuer saved`, undefined, { duration: 2000 });
      },
      error: (e: HttpErrorResponse) => {
        this.savingIssuer.set(false);
        // The select goes back to what is stored.
        this.data.set({ ...d });
        this.fail(e);
      },
    });
  }

  // The OpenID provider's discovery for applications, beside the internal
  // one: only while the provider answers - an issuer that is an address, and
  // a client declared.
  protected readonly publicDiscovery = computed(() =>
    this.clients().length ? (this.data()?.publicDiscovery ?? '') : '',
  );

  constructor() {
    this.reload();
    this.reloadClients();
    // Another administrator, an agent, or this console's route editor moves
    // the list while the drawer is open: a client written, or a route bound
    // to one - each row names the routes using it. A list, so it is simply
    // read again; the drawer's own writes re-read it on their own.
    const live = inject(LiveChangesService);
    live.on('oidc-client', () => this.reloadClients());
    live.on('route', () => this.reloadClients());
  }

  // OIDC clients.

  protected readonly clients = signal<OIDCClient[]>([]);
  protected readonly clientColumns = ['id', 'secret', 'routes'];

  // The provider answers only under an issuer that is an address: until the
  // application plane has a public name, the issuer is a name.
  protected readonly noPublicName = computed(() => {
    const d = this.data();
    return !!d && !d.issuers?.length;
  });

  private reloadClients(): void {
    this.api.oidcClients().subscribe({
      next: (c) => this.clients.set(c ?? []),
      error: () => this.clients.set([]),
    });
  }

  protected async editClient(c?: OIDCClient): Promise<void> {
    const { editOidcClient } = await import('./oidc-client-dialog.component');
    const saved = await editOidcClient(this.dialog, this.api, {
      client: c,
      taken: this.clients().map((x) => x.id),
    });
    if (saved) this.reloadClients();
  }

  protected async regenerate(c: OIDCClient): Promise<void> {
    const ok = await this.dialogs.confirm({
      title: $localize`:@@Regenerate_secret_of_CLIENT:Regenerate the secret of ${c.id}:client:?`,
      message: $localize`:@@Regenerate_secret_message:The current secret stops working at once: the application signs nobody in until it has the new one.`,
      confirmLabel: $localize`:@@Regenerate:Regenerate`,
      danger: true,
    });
    if (!ok) return;
    const { showOidcSecret } = await import('./oidc-client-dialog.component');
    this.api.regenerateOidcClientSecret(c.id).subscribe({
      next: (out) => {
        this.reloadClients();
        void showOidcSecret(this.dialog, out);
      },
      error: (e: HttpErrorResponse) => this.fail(e),
    });
  }

  protected async removeClient(c: OIDCClient): Promise<void> {
    const ok = await this.dialogs.confirm({
      title: $localize`:@@Delete_OIDC_client:Delete ${c.id}:client:?`,
      message: $localize`:@@Delete_OIDC_client_message:The application can no longer sign anyone in through Meerkat.`,
      confirmLabel: $localize`:@@Delete:Delete`,
      danger: true,
    });
    if (!ok) return;
    this.api.deleteOidcClient(c.id).subscribe({
      next: () => this.reloadClients(),
      // A client a route still uses is refused, and the refusal names the
      // routes: that sentence is the answer, so it is shown as it came.
      error: (e: HttpErrorResponse) => this.fail(e),
    });
  }

  private fail(e: HttpErrorResponse): void {
    this.snack.open(
      typeof e.error?.error === 'string' ? e.error.error : $localize`:@@Request_failed:Request failed`,
      $localize`:@@Close:Close`,
      { duration: 10000 },
    );
  }

  private reload(): void {
    this.loading.set(true);
    this.api.getSigningKeys().subscribe({
      next: (d) => {
        this.data.set(d);
        this.loading.set(false);
      },
      error: () => this.loading.set(false),
    });
  }

  protected copy(text: string): void {
    void navigator.clipboard?.writeText(text);
    this.snack.open($localize`:@@Copied:Copied`, undefined, { duration: 1500 });
  }

  protected renew(): void {
    this.renewing.set(true);
    this.confirming.set(false);
    this.api.renewSigningKeys().subscribe({
      next: (d) => {
        this.data.set(d);
        this.renewing.set(false);
        this.snack.open($localize`:@@Signing_keys_rotated:Signing keys rotated`, undefined, { duration: 2500 });
      },
      error: () => {
        this.renewing.set(false);
        this.snack.open($localize`:@@Request_failed:Request failed`, undefined, { duration: 3000 });
      },
    });
  }
}
