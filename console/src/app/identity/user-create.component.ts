import { Component, computed, inject, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatDividerModule } from '@angular/material/divider';
import { MatIconModule } from '@angular/material/icon';
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

  protected readonly username = signal('');
  protected readonly fullname = signal('');
  protected readonly email = signal('');
  protected readonly fields = signal<Record<string, string>>({});
  protected readonly validFrom = signal(0);
  protected readonly validUntil = signal(0);
  protected readonly error = signal('');
  protected readonly saving = signal(false);

  // A login and nothing else: everything the server refuses beyond that comes
  // back as a sentence, and a rule copied here would be a second, worse copy.
  protected readonly valid = computed(() => this.username().trim() !== '');

  protected save(): void {
    this.error.set('');
    this.saving.set(true);
    this.api
      .createUser({
        username: this.username().trim(),
        fullname: this.fullname().trim(),
        email: this.email().trim(),
        enabled: true,
        fields: this.fields(),
        validFrom: this.validFrom(),
        validUntil: this.validUntil(),
      })
      .subscribe({
        next: ({ user, password }) => {
          this.saving.set(false);
          // Shown once and never again: the generated password exists in this
          // response and nowhere else.
          this.dialog.open(PasswordDialogComponent, {
            data: { username: user.username, password },
          });
          this.created.emit(user);
        },
        error: (e: { error?: { error?: string } }) => {
          this.saving.set(false);
          this.error.set(e?.error?.error ?? $localize`:@@Save_failed:Save failed`);
        },
      });
  }
}
