import { Component, computed, inject, input, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatExpansionModule } from '@angular/material/expansion';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';
import { ApiService, Route, SigningKeys } from '../../api.service';

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
    MatButtonModule,
    MatExpansionModule,
    MatIconModule,
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

  constructor() {
    this.reload();
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
