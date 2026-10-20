import { Component, inject } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MAT_DIALOG_DATA, MatDialogModule } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';

export interface ResetChoiceData {
  username: string;
  email: string;
  // Whether a link can be mailed at all, and why not (from the edition).
  mailReady: boolean;
  mailWhy?: string;
}

// How a person gets back in after a reset (AUTH-25): a temporary password
// shown once, or a link mailed to them to choose their own - one or the other.
// Closes with 'show', 'mail', or nothing (cancelled).
@Component({
  selector: 'app-reset-choice-dialog',
  imports: [MatButtonModule, MatDialogModule, MatIconModule],
  template: `
    <h2 mat-dialog-title i18n="@@Reset_password_USERNAME">Reset password for "{{ data.username }}"?</h2>
    <mat-dialog-content>
      <p i18n="@@Reset_choice_hint">
        Show a temporary password to hand over, or mail a link to choose a new one. With the link,
        the current password keeps working until it is followed.
      </p>
      @if (why) {
        <p class="why">{{ why }}</p>
      }
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button matButton mat-dialog-close i18n="@@Cancel">Cancel</button>
      <button matButton [mat-dialog-close]="'show'">
        <mat-icon>password</mat-icon>
        <ng-container i18n="@@Show_a_temporary_password">Show a temporary password</ng-container>
      </button>
      <button matButton="filled" [disabled]="!!why" [mat-dialog-close]="'mail'">
        <mat-icon>forward_to_inbox</mat-icon>
        <ng-container i18n="@@Mail_a_reset_link">Mail a reset link</ng-container>
      </button>
    </mat-dialog-actions>
  `,
  styles: `
    .why {
      font-size: 0.85rem;
      color: var(--mat-sys-on-surface-variant);
    }
  `,
})
export class ResetChoiceDialogComponent {
  protected readonly data = inject<ResetChoiceData>(MAT_DIALOG_DATA);
  // Why the link cannot be mailed, said under the choice rather than as a
  // button that does nothing.
  protected readonly why = !this.data.email
    ? $localize`:@@No_email_to_mail_to:This account has no e-mail address to mail a link to.`
    : this.data.mailReady
      ? ''
      : $localize`:@@No_link_can_be_mailed_WHY:No link can be mailed: ${this.data.mailWhy}:WHY:.`;
}
