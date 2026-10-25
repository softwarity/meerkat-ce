import { Component, computed, inject, linkedSignal, output, signal } from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatDialog } from '@angular/material/dialog';
import { MatDividerModule } from '@angular/material/divider';
import { MatIconModule } from '@angular/material/icon';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';
import { ApiService, User } from '../api.service';
import { PasswordDialogComponent } from './password-dialog.component';
import { UserAccountFormComponent } from './user-account-form.component';
import { UserFieldsFormComponent } from './user-fields-form.component';

// A new account, in the drawer the account itself is edited in.
//
// The same surface for creating and for editing, and that is the point: an
// account that has to be created in a small dialog and completed afterwards
// teaches two layouts for one object, and leaves half of it blank in between.
// Everything an account carries at birth is here, drawn by the very components
// the editor uses.
@Component({
  selector: 'app-user-create',
  imports: [
    MatButtonModule,
    MatButtonToggleModule,
    MatDividerModule,
    MatIconModule,
    MatTooltipModule,
    UserAccountFormComponent,
    UserFieldsFormComponent,
  ],
  template: `
    <header>
      <h2 i18n="@@New_user">New user</h2>
      <span class="grow"></span>
      <button
        matIconButton
        (click)="closed.emit()"
        i18n-matTooltip="@@Close"
        matTooltip="Close"
        i18n-aria-label="@@Close"
        aria-label="Close"
      >
        <mat-icon>close</mat-icon>
      </button>
    </header>

    <div class="body">
      <section>
        <h3 i18n="@@Account">Account</h3>
        <app-user-account-form
          [(username)]="username"
          [(fullname)]="fullname"
          [(email)]="email"
        />
      </section>

      <mat-divider />

      <!-- The window the account is valid in, and what this installation
           records about a person (Infra > Model). Here rather than only in the
           editor: somebody hired for a three-month contract has an end date on
           the day their account is created, not on the day someone remembers
           to go back and set one. -->
      <section>
        <app-user-fields-form
          [(fields)]="fields"
          [(validFrom)]="validFrom"
          [(validUntil)]="validUntil"
        />
      </section>

      <mat-divider />

      <!-- How the person gets in the first time (AUTH-25): one or the other,
           never both - a password shown once here, or a link mailed to them
           to choose their own. -->
      <section>
        <h3 i18n="@@First_sign_in">First sign-in</h3>
        <mat-button-toggle-group
          [value]="access()"
          (change)="access.set($event.value)"
          hideSingleSelectionIndicator
          i18n-aria-label="@@First_sign_in"
          aria-label="First sign-in"
        >
          <mat-button-toggle value="password">
            <mat-icon>password</mat-icon>
            <ng-container i18n="@@Show_a_temporary_password">Show a temporary password</ng-container>
          </mat-button-toggle>
          <mat-button-toggle value="invite" [disabled]="!mailReady()">
            <mat-icon>forward_to_inbox</mat-icon>
            <ng-container i18n="@@Send_an_invitation">Send an invitation</ng-container>
          </mat-button-toggle>
        </mat-button-toggle-group>
        <p class="hint">{{ accessHint() }}</p>
      </section>

      @if (error()) {
        <p class="warn">
          <mat-icon>report</mat-icon>
          <span>{{ error() }}</span>
        </p>
      }

      <!-- The superpowers are NOT here: they are one click each on the table's
           badges, and an account is created before anybody decides what it may
           do. -->
      <div class="actions">
        <button matButton (click)="closed.emit()" i18n="@@Cancel">Cancel</button>
        <button matButton="filled" (click)="save()" [disabled]="saving() || !valid()" i18n="@@Create">
          Create
        </button>
      </div>
    </div>
  `,
  styles: `
    :host {
      display: flex;
      flex-direction: column;
      height: 100%;
      min-height: 0;
    }
    header {
      display: flex;
      align-items: center;
      gap: 12px;
      padding: 16px 16px 8px 24px;
    }
    h2 {
      margin: 0;
      font-size: 1.15rem;
      font-weight: 500;
    }
    .grow {
      flex: 1;
    }
    .body {
      flex: 1 1 auto;
      min-height: 0;
      overflow: auto;
      padding: 0 24px 24px;
    }
    section {
      padding: 12px 0;
    }
    h3 {
      margin: 0 0 10px;
      font-size: 0.78rem;
      font-weight: 600;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      color: var(--mat-sys-on-surface-variant);
    }
    .actions {
      display: flex;
      justify-content: flex-end;
      gap: 8px;
      margin-top: 16px;
    }
    .hint {
      margin: 8px 0 0;
      font-size: 0.82rem;
      color: var(--mat-sys-on-surface-variant);
    }
    .warn {
      display: flex;
      align-items: center;
      gap: 8px;
      color: var(--mat-sys-error);
      font-size: 0.85rem;
    }
  `,
})
export class UserCreateComponent {
  readonly created = output<User>();
  readonly closed = output<void>();

  private readonly api = inject(ApiService);
  private readonly dialog = inject(MatDialog);
  private readonly snack = inject(MatSnackBar);

  protected readonly username = signal('');
  protected readonly fullname = signal('');
  protected readonly email = signal('');
  protected readonly fields = signal<Record<string, string>>({});
  protected readonly validFrom = signal(0);
  protected readonly validUntil = signal(0);
  protected readonly error = signal('');
  protected readonly saving = signal(false);

  // Whether an invitation can be mailed at all, and why not.
  private readonly edition = toSignal(this.api.edition());
  protected readonly mailReady = computed(() => !!this.edition()?.mailReady);
  // The invitation when it can be sent, the password otherwise.
  protected readonly access = linkedSignal<'password' | 'invite'>(() => (this.mailReady() ? 'invite' : 'password'));
  protected readonly accessHint = computed(() => {
    if (this.access() === 'password') {
      return this.mailReady() || !this.edition()
        ? $localize`:@@Access_password_hint:A password is generated and shown once, for you to give to the person. They must change it at first sign-in.`
        : $localize`:@@Access_no_invite_WHY:An invitation cannot be sent: ${this.edition()?.mailWhy}:WHY:.`;
    }
    return this.email().trim()
      ? $localize`:@@Access_invite_hint:A link to choose a password is mailed to the address above. It is valid for 48 hours.`
      : $localize`:@@Access_invite_needs_email:Enter an e-mail address above to send the invitation.`;
  });

  // A login and nothing else - and an address when it is to be mailed:
  // everything the server refuses beyond that comes back as a sentence, and a
  // rule copied here would be a second, worse copy.
  protected readonly valid = computed(
    () => this.username().trim() !== '' && (this.access() === 'password' || this.email().trim() !== ''),
  );

  protected save(): void {
    this.error.set('');
    this.saving.set(true);
    const invite = this.access() === 'invite';
    this.api
      .createUser({
        username: this.username().trim(),
        fullname: this.fullname().trim(),
        email: this.email().trim(),
        enabled: true,
        fields: this.fields(),
        validFrom: this.validFrom(),
        validUntil: this.validUntil(),
      }, invite)
      .subscribe({
        next: ({ user, password, email, validHours }) => {
          this.saving.set(false);
          if (invite) {
            this.snack.open($localize`:@@Invitation_mailed_EMAIL:Invitation mailed to ${email}:EMAIL:`, undefined, {
              duration: 4000,
            });
          } else {
            // Shown once and never again: the generated password exists in
            // this response and nowhere else.
            this.dialog.open(PasswordDialogComponent, {
              data: { username: user.username, password, validHours },
            });
          }
          this.created.emit(user);
        },
        error: (e: { error?: { error?: string } }) => {
          this.saving.set(false);
          this.error.set(e?.error?.error ?? $localize`:@@Save_failed:Save failed`);
        },
      });
  }
}
