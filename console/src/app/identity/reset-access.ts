import { MatDialog } from '@angular/material/dialog';
import { MatSnackBar } from '@angular/material/snack-bar';
import { firstValueFrom, Observable } from 'rxjs';
import { ApiService, PasswordReset } from '../api.service';
import { PasswordDialogComponent } from './password-dialog.component';

// A password reset, from the account drawer or the members table (AUTH-25):
// the person's way back in is chosen first - a temporary password shown once,
// or a link mailed to them - then the reset is made that way and its outcome
// shown. reset is the call, told which of the two was chosen.
export async function resetAccess(
  api: ApiService,
  dialog: MatDialog,
  snack: MatSnackBar,
  target: { username: string; email: string },
  reset: (byMail: boolean) => Observable<PasswordReset>,
): Promise<void> {
  const [edition, { ResetChoiceDialogComponent }] = await Promise.all([
    firstValueFrom(api.edition()),
    import('./reset-choice-dialog.component'),
  ]);
  const choice = await firstValueFrom(
    dialog
      .open(ResetChoiceDialogComponent, {
        width: '520px',
        data: { ...target, mailReady: !!edition.mailReady, mailWhy: edition.mailWhy },
      })
      .afterClosed(),
  );
  if (choice !== 'show' && choice !== 'mail') return;
  reset(choice === 'mail').subscribe({
    next: ({ password, email, validHours }) => {
      if (choice === 'mail') {
        snack.open($localize`:@@Reset_link_mailed_EMAIL:Reset link mailed to ${email}:EMAIL:`, undefined, { duration: 4000 });
        return;
      }
      dialog.open(PasswordDialogComponent, { data: { username: target.username, password, validHours } });
    },
    error: (e: { error?: { error?: string } }) =>
      snack.open(e?.error?.error ?? $localize`:@@Reset_failed:Reset failed`, undefined, { duration: 5000 }),
  });
}
