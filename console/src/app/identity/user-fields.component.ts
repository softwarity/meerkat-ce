import { Component, computed, inject, input, output, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { ApiService, User } from '../api.service';
import { UserAccountFormComponent } from './user-account-form.component';
import { UserFieldsFormComponent } from './user-fields-form.component';

// The identity section of a user's drawer: who the person is, this
// installation's own fields, and the validity window - on one account.
//
// The same widgets the creation drawer draws, in the same order: an account is
// not born through one form and corrected through another. It holds an edit
// buffer rather than saving on every keystroke - unlike the switches on the
// security page, which are one decision each. Save and Cancel stay on the page
// and grey out until something changed: buttons that appear on the first
// keystroke shift everything under them as one types.
@Component({
  selector: 'app-user-fields',
  imports: [
    MatButtonModule,
    MatIconModule,
    UserAccountFormComponent,
    UserFieldsFormComponent,
  ],
  template: `
    <app-user-account-form
      [username]="username()"
      (usernameChange)="edit({ username: $event })"
      [fullname]="fullname()"
      (fullnameChange)="edit({ fullname: $event })"
      [email]="email()"
      (emailChange)="edit({ email: $event })"
    />

    <app-user-fields-form
      [fields]="fields()"
      (fieldsChange)="edit({ fields: $event })"
      [validFrom]="validFrom()"
      (validFromChange)="edit({ validFrom: $event })"
      [validUntil]="validUntil()"
      (validUntilChange)="edit({ validUntil: $event })"
    />

    @if (error()) {
      <p class="warn">
        <mat-icon>report</mat-icon>
        <span>{{ error() }}</span>
      </p>
    }
    <!-- Always on the page, greyed until something changed: a Save that
         appears only when it can be pressed moves the rest of the form under
         the cursor the moment somebody types. -->
    <div class="actions">
      <button matButton (click)="revert()" [disabled]="!dirty() || saving()" i18n="@@Cancel">
        Cancel
      </button>
      <button matButton="filled" (click)="save()" [disabled]="!dirty() || saving()" i18n="@@Save">
        Save
      </button>
    </div>
  `,
  styles: `
    .actions {
      display: flex;
      justify-content: flex-end;
      gap: 8px;
      margin-top: 12px;
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
export class UserFieldsComponent {
  readonly user = input.required<User>();
  readonly saved = output<User>();

  private readonly api = inject(ApiService);
  private readonly draft = signal<Partial<User> | null>(null);
  protected readonly saving = signal(false);
  protected readonly error = signal('');
  protected readonly dirty = computed(() => this.draft() !== null);

  protected readonly username = computed(() => this.draft()?.username ?? this.user().username);
  protected readonly fullname = computed(() => this.draft()?.fullname ?? this.user().fullname ?? '');
  protected readonly email = computed(() => this.draft()?.email ?? this.user().email ?? '');
  protected readonly fields = computed(() => this.draft()?.fields ?? this.user().fields ?? {});
  protected readonly validFrom = computed(() => this.draft()?.validFrom ?? this.user().validFrom ?? 0);
  protected readonly validUntil = computed(() => this.draft()?.validUntil ?? this.user().validUntil ?? 0);

  protected edit(change: Partial<User>): void {
    this.draft.set({ ...(this.draft() ?? {}), ...change });
  }

  protected revert(): void {
    this.draft.set(null);
    this.error.set('');
  }

  protected save(): void {
    const d = this.draft();
    if (!d) return;
    this.error.set('');
    this.saving.set(true);
    // The WHOLE account with the changes on top: a PUT built from the fields
    // this form knows would clear every field it does not.
    this.api.updateUser({ ...this.user(), ...d }).subscribe({
      next: (fresh) => {
        this.saving.set(false);
        this.draft.set(null);
        this.saved.emit(fresh);
      },
      error: (e: { error?: { error?: string } }) => {
        this.saving.set(false);
        this.error.set(e?.error?.error ?? $localize`:@@Save_failed:Save failed`);
      },
    });
  }
}
