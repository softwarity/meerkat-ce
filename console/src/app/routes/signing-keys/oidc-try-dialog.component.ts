import { HttpErrorResponse } from '@angular/common/http';
import { Component, computed, inject, signal } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { MatButtonModule } from '@angular/material/button';
import { MAT_DIALOG_DATA, MatDialogModule } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatSelectModule } from '@angular/material/select';
import { ApiService, OIDCClient, OIDCTrial } from '../../api.service';
import { SnippetComponent } from '../../shared/snippet.component';

// A sign-in tried through a client on someone's behalf, with nothing issued:
// which route lets them in, which ones turn them away, and what the tokens
// and userinfo would say. Built by the code that issues the real tokens, with
// the person's own roles - the answer to "why does this person see nothing in
// the application" without asking them to sign in.
@Component({
  selector: 'app-oidc-try-dialog',
  imports: [
    MatButtonModule,
    MatDialogModule,
    MatFormFieldModule,
    MatIconModule,
    MatProgressBarModule,
    MatSelectModule,
    SnippetComponent,
  ],
  styles: [
    `
      .who {
        display: flex;
        gap: 12px;
      }
      .who mat-form-field {
        flex: 1;
      }
      .verdict {
        display: flex;
        align-items: center;
        gap: 8px;
        margin: 4px 0 12px;
      }
      .ok {
        color: var(--mat-sys-primary);
      }
      .refused {
        color: var(--mat-sys-error);
      }
      .note {
        color: var(--mat-sys-on-surface-variant);
        font-size: 0.85rem;
      }
      app-snippet {
        display: block;
        margin-bottom: 12px;
      }
    `,
  ],
  template: `
    <h2 mat-dialog-title i18n="@@Try_a_sign_in_CLIENT">Try a sign-in through {{ client.id }}</h2>
    @if (running()) {
      <mat-progress-bar mode="indeterminate" />
    }
    <mat-dialog-content>
      <p class="note" i18n="@@Oidc_try_hint">
        What the application would get if this person signed in now, with their own roles. Nothing is issued.
      </p>
      <div class="who">
        <mat-form-field>
          <mat-label i18n="@@User">User</mat-label>
          <mat-select [value]="user()" (selectionChange)="user.set($event.value)">
            @for (u of users(); track u.id) {
              <mat-option [value]="u.id">{{ u.username }}{{ u.fullname ? ' - ' + u.fullname : '' }}</mat-option>
            }
          </mat-select>
        </mat-form-field>
        <mat-form-field>
          <mat-label i18n="@@Organisation">Organisation</mat-label>
          <mat-select [value]="tenant()" (selectionChange)="tenant.set($event.value)">
            <mat-option value="" i18n="@@Oidc_try_only_membership">Their only one</mat-option>
            @for (t of tenants(); track t.id) {
              <mat-option [value]="t.id">{{ t.name }}</mat-option>
            }
          </mat-select>
        </mat-form-field>
      </div>

      @if (error(); as e) {
        <p class="verdict refused"><mat-icon>error</mat-icon><span>{{ e }}</span></p>
      } @else if (trial(); as t) {
        @if (t.route; as r) {
          <p class="verdict ok">
            <mat-icon>check_circle</mat-icon>
            <span i18n="@@Oidc_try_signed_in_ROUTE">Signed in through the route {{ r.name }}</span>
          </p>
        } @else {
          <p class="verdict refused">
            <mat-icon>block</mat-icon>
            <span i18n="@@Oidc_try_refused">No route bound to this client lets this person in: the gateway refuses the sign-in.</span>
          </p>
        }
        @if (t.refused.length) {
          <p class="note" i18n="@@Oidc_try_refused_by_ROUTES">Turned away by the access rule of: {{ refusedNames() }}</p>
        }
        @if (t.idToken) {
          <app-snippet filename="id_token.json" [content]="json(t.idToken)" [downloadable]="false" />
          <app-snippet filename="access_token.json" [content]="json(t.accessToken)" [downloadable]="false" />
          <app-snippet filename="userinfo.json" [content]="json(t.userinfo)" [downloadable]="false" />
        }
      }
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button matButton mat-dialog-close i18n="@@Close">Close</button>
      <button matButton="filled" [disabled]="!user() || running()" (click)="run()">
        <mat-icon>play_arrow</mat-icon>
        <span i18n="@@Try">Try</span>
      </button>
    </mat-dialog-actions>
  `,
})
export class OidcTryDialogComponent {
  private readonly api = inject(ApiService);
  protected readonly client = inject<OIDCClient>(MAT_DIALOG_DATA);

  protected readonly users = toSignal(this.api.listUsers(), { initialValue: [] });
  protected readonly tenants = toSignal(this.api.listTenants(), { initialValue: [] });

  protected readonly user = signal('');
  protected readonly tenant = signal('');
  protected readonly running = signal(false);
  protected readonly error = signal('');
  protected readonly trial = signal<OIDCTrial | null>(null);
  protected readonly refusedNames = computed(() => (this.trial()?.refused ?? []).map((r) => r.name).join(', '));

  protected run(): void {
    this.running.set(true);
    this.error.set('');
    this.trial.set(null);
    this.api.tryOidcSignIn(this.client.id, this.user(), this.tenant()).subscribe({
      next: (t) => {
        this.trial.set(t);
        this.running.set(false);
      },
      error: (e: HttpErrorResponse) => {
        this.error.set(e.error?.error ?? $localize`:@@Request_failed:Request failed`);
        this.running.set(false);
      },
    });
  }

  protected json(v: unknown): string {
    return JSON.stringify(v ?? {}, null, 2);
  }
}
