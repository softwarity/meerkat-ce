import { Component, computed, inject, input, output, signal } from '@angular/core';
import { HttpErrorResponse } from '@angular/common/http';
import { FormField, form, required, validate } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatExpansionModule } from '@angular/material/expansion';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';
import { ApiService, Route, SigningKeys } from '../../api.service';
import { FormFieldComponent } from '../../shared/form-field.component';

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
@Component({
  selector: 'app-signing-keys-panel',
  imports: [
    FormField,
    FormFieldComponent,
    MatButtonModule,
    MatExpansionModule,
    MatIconModule,
    MatInputModule,
    MatProgressBarModule,
    MatTooltipModule,
  ],
  styleUrl: './signing-keys-panel.component.scss',
  templateUrl: './signing-keys-panel.component.html',
})
export class SigningKeysPanelComponent {
  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);

  // The page's routes, handed down so a key can say who uses it. It is the
  // question three identical-looking rows never answered - and the reason they
  // read as one thing shown three times.
  readonly routes = input<Route[]>([]);
  readonly routePicked = output<string>();
  readonly closed = output<void>();

  // Which routes sign with each algorithm. A route on signed-jwt with no
  // algorithm named uses ES256: that is the server's fallback, so counting it
  // anywhere else would make this list disagree with what is actually signed.
  protected readonly users = computed(() => {
    const by = new Map<string, Route[]>();
    for (const r of this.routes()) {
      if (r.identity?.mechanism !== 'signed-jwt') continue;
      const alg = r.identity.algorithm || 'ES256';
      by.set(alg, [...(by.get(alg) ?? []), r]);
    }
    return by;
  });

  protected usersOf(alg: string): Route[] {
    return this.users().get(alg) ?? [];
  }

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

  // The issuer, edited in place. The rules mirror the server's
  // (store.CheckJWTIssuer), so the field refuses what a save would.
  protected readonly issuerDraft = signal({ issuer: '' });
  protected readonly issuerForm = form(this.issuerDraft, (p) => {
    required(p.issuer, { message: $localize`:@@Issuer_required:An issuer is required.` });
    validate(p.issuer, ({ value }) => {
      const v = value();
      if (/\s/.test(v)) {
        return [{ kind: 'issuer', message: $localize`:@@Issuer_space:No space: a service compares it byte for byte.` }];
      }
      if (v.includes('://') && (!/^https?:\/\/[^/?#]+/.test(v) || /[?#]/.test(v) || v.endsWith('/'))) {
        return [{ kind: 'issuer', message: $localize`:@@Issuer_url:An http or https URL with a host, without a query, a fragment or a trailing slash.` }];
      }
      return [];
    });
  });
  protected readonly issuerChanged = computed(() => {
    const d = this.data();
    return !!d && this.issuerDraft().issuer !== d.issuer;
  });
  protected readonly savingIssuer = signal(false);
  protected readonly issuerError = signal('');
  // What the field says under it: the rule it breaks while typing, else what
  // the server answered.
  protected readonly issuerMessage = computed(
    () => this.issuerForm.issuer().errors()[0]?.message ?? this.issuerError(),
  );

  protected saveIssuer(): void {
    if (this.issuerForm().invalid()) return;
    this.savingIssuer.set(true);
    this.issuerError.set('');
    this.api.setIssuer(this.issuerDraft().issuer).subscribe({
      next: (d) => {
        this.setData(d);
        this.savingIssuer.set(false);
        this.snack.open($localize`:@@Issuer_saved:Issuer saved`, undefined, { duration: 2000 });
      },
      error: (e: HttpErrorResponse) => {
        this.savingIssuer.set(false);
        this.issuerError.set(e.error?.error ?? $localize`:@@Request_failed:Request failed`);
      },
    });
  }

  // The keys and the issuer field together: a reload puts the stored issuer
  // back in the field.
  private setData(d: SigningKeys): void {
    this.data.set(d);
    this.issuerDraft.set({ issuer: d.issuer });
  }

  constructor() {
    this.reload();
  }

  private reload(): void {
    this.loading.set(true);
    this.api.getSigningKeys().subscribe({
      next: (d) => {
        this.setData(d);
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
