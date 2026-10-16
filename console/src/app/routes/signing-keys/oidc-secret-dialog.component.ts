import { Component, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MAT_DIALOG_DATA, MatDialogModule } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatTooltipModule } from '@angular/material/tooltip';

export interface OidcSecretData {
  clientId: string;
  secret: string;
}

// An OIDC client's secret, in clear, the one time it exists outside the
// application: the server keeps only its hash. The client id rides along,
// since the application's configuration wants the two together.
@Component({
  selector: 'app-oidc-secret-dialog',
  imports: [MatButtonModule, MatDialogModule, MatIconModule, MatTooltipModule],
  styles: [
    `
      .hint {
        color: var(--mat-sys-on-surface-variant);
        font-size: 0.85rem;
      }
      .lbl {
        margin: 12px 0 4px;
        font-size: 0.72rem;
        text-transform: uppercase;
        letter-spacing: 0.04em;
        color: var(--mat-sys-on-surface-variant);
      }
      .secret {
        display: flex;
        align-items: center;
        gap: 8px;
        padding: 4px 4px 4px 16px;
        border: 1px solid var(--mat-sys-outline-variant);
        border-radius: 8px;
        background: var(--mat-sys-surface-container-highest);
      }
      code {
        font-family: var(--mk-mono);
        flex: 1;
        min-width: 0;
        word-break: break-all;
        user-select: all;
      }
    `,
  ],
  template: `
    <h2 mat-dialog-title i18n="@@Secret_of_CLIENT">Secret of {{ data.clientId }}</h2>
    <mat-dialog-content>
      <p class="hint" i18n="@@Oidc_secret_once">
        Copy it now and give it to the application: it will not be shown again. Lost, it can only be
        regenerated.
      </p>
      <p class="lbl" i18n="@@Client_id">Client id</p>
      <div class="secret">
        <code>{{ data.clientId }}</code>
        <button
          matIconButton
          (click)="copy('id', data.clientId)"
          i18n-matTooltip="@@Copy"
          matTooltip="Copy"
          i18n-aria-label="@@Copy"
          aria-label="Copy"
        >
          <mat-icon>{{ copied() === 'id' ? 'check' : 'content_copy' }}</mat-icon>
        </button>
      </div>
      <p class="lbl" i18n="@@Client_secret">Client secret</p>
      <div class="secret">
        <code>{{ data.secret }}</code>
        <button
          matIconButton
          (click)="copy('secret', data.secret)"
          i18n-matTooltip="@@Copy"
          matTooltip="Copy"
          i18n-aria-label="@@Copy"
          aria-label="Copy"
        >
          <mat-icon>{{ copied() === 'secret' ? 'check' : 'content_copy' }}</mat-icon>
        </button>
      </div>
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button matButton="filled" mat-dialog-close i18n="@@Done">Done</button>
    </mat-dialog-actions>
  `,
})
export class OidcSecretDialogComponent {
  protected readonly data = inject<OidcSecretData>(MAT_DIALOG_DATA);
  protected readonly copied = signal<'' | 'id' | 'secret'>('');

  protected copy(which: 'id' | 'secret', text: string): void {
    void navigator.clipboard?.writeText(text).then(() => this.copied.set(which));
  }
}
