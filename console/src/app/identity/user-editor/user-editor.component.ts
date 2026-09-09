import { Component, computed, inject, input, model, output } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatDividerModule } from '@angular/material/divider';
import { MatIconModule } from '@angular/material/icon';
import { MatListModule } from '@angular/material/list';
import { MatTooltipModule } from '@angular/material/tooltip';
import { ApiService, User } from '../../api.service';
import { DialogsService } from '../../shared/dialogs.service';
import { ExternalIdentitiesComponent } from '../external-identities.component';
import { LoginHistoryComponent } from '../login-history.component';
import { MfaSelectComponent } from '../mfa-select.component';
import { PasswordDialogComponent } from '../password-dialog.component';
import { UserFieldsComponent } from '../user-fields.component';

// Which page of the account drawer is on screen.
export type UserEditorView = 'account' | 'security' | 'history';

// A user's account, hosted in the users-page right drawer (opened by clicking
// a row).
//
// THREE pages in one drawer, and the drawer never grows a second panel: the
// account one lands on (identity, validity, this installation's own fields),
// the security one (second factor, password, authorities) and the history.
// What is FILLED stays on the first page; what is decided or read is one tap
// away and comes back. The page above owns the view so that a click outside
// gives back exactly one level - the sub-page first, the drawer after.
@Component({
  selector: 'app-user-editor',
  imports: [
    MatButtonModule,
    MatDividerModule,
    MatIconModule,
    MatListModule,
    MatTooltipModule,
    ExternalIdentitiesComponent,
    LoginHistoryComponent,
    MfaSelectComponent,
    UserFieldsComponent,
  ],
  templateUrl: './user-editor.component.html',
  styleUrl: './user-editor.component.scss',
})
export class UserEditorComponent {
  readonly user = input.required<User>();
  // Resolved global second-factor policy - what a user's "Inherited" means.
  readonly globalMfaLabel = input.required<string>();
  readonly meId = input.required<string>();

  // Which of the three pages the drawer shows. A model and not a private
  // signal: the page outside answers the click on the backdrop, and it has to
  // know whether that click owes a sub-page or the whole drawer.
  readonly view = model<UserEditorView>('account');

  // saved: a field changed, here is the fresh user (the page updates its list
  // and keeps the drawer in sync). closed: drawer dismissed or user deleted.
  readonly saved = output<User>();
  readonly closed = output<void>();

  protected readonly heading = computed(() => {
    switch (this.view()) {
      case 'security':
        return $localize`:@@Security:Security`;
      case 'history':
        return $localize`:@@Sign_in_history:Sign-in history`;
      default:
        return this.user().username;
    }
  });

  private readonly api = inject(ApiService);
  private readonly dialog = inject(MatDialog);
  private readonly dialogs = inject(DialogsService);
  private readonly snack = inject(MatSnackBar);

  protected readonly isMe = () => this.user().id === this.meId();

  protected setMfa(mfaRequired: string): void {
    this.apply({ ...this.user(), mfaRequired });
  }

  private apply(next: User): void {
    this.api.updateUser(next).subscribe({
      next: (fresh) => this.saved.emit(fresh),
      // On error, re-emit the authoritative user so the toggle snaps back.
      error: () => this.saved.emit(this.user()),
    });
  }

  protected resetPassword(): void {
    this.api.resetPassword(this.user().id).subscribe({
      next: ({ password }) =>
        this.dialog.open(PasswordDialogComponent, {
          data: { username: this.user().username, password },
        }),
    });
  }

  // Keeps the password the person already knows and refuses to go further with
  // it: they sign in as usual and land on the change page. A reset would mean
  // carrying a temporary password to them, which is a phone call per person.
  protected mustChange(): void {
    const u = this.user();
    this.api.mustChangePassword(u.id).subscribe({
      next: () => {
        this.snack.open(
          $localize`:@@USERNAME_will_change_their_password:${u.username}:USERNAME: will have to change their password at the next sign-in`,
          undefined,
          { duration: 4000 },
        );
        this.saved.emit({ ...u, mustChangePassword: true });
      },
    });
  }

  protected async remove(): Promise<void> {
    const u = this.user();
    const ok = await this.dialogs.confirm({
      title: $localize`:@@Delete_user_USERNAME:Delete user "${u.username}:USERNAME:"?`,
      confirmLabel: $localize`:@@Delete:Delete`,
      danger: true,
    });
    if (!ok) return;
    this.api.deleteUser(u.id).subscribe({ next: () => this.closed.emit() });
  }
}
