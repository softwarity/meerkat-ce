import { Component, inject } from '@angular/core';
import { MatDialogModule, MatDialogRef, MAT_DIALOG_DATA } from '@angular/material/dialog';
import { VaultEntry } from '../api.service';
import { VaultEntryFormComponent, VaultEntryFormData } from './vault-entry-form.component';

// The vault editor as a MODAL. It exists for the ONE place a drawer cannot go:
// declaring or moving a secret from within another editor that is already a
// drawer (a route's field, a secret field) - two drawers on the same edge is a
// position Material refuses. On the vault page itself the same editor is shown
// in a drawer, to match the routes and users screens; both mount the shared
// VaultEntryFormComponent, so there is one editor and two frames.
export type VaultEntryDialogData = VaultEntryFormData;

@Component({
  selector: 'app-vault-entry-dialog',
  imports: [MatDialogModule, VaultEntryFormComponent],
  template: `
    <app-vault-entry-form [data]="data" (saved)="ref.close($event)" (closed)="ref.close()" />
  `,
  styles: `
    :host {
      display: block;
      width: min(480px, 92vw);
    }
    app-vault-entry-form {
      max-height: 82vh;
    }
  `,
})
export class VaultEntryDialogComponent {
  protected readonly ref = inject(MatDialogRef<VaultEntryDialogComponent, VaultEntry>);
  protected readonly data =
    inject<VaultEntryDialogData>(MAT_DIALOG_DATA, { optional: true }) ?? {};
}
