import { Component, computed, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatSidenavModule } from '@angular/material/sidenav';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTableModule } from '@angular/material/table';
import { MatTooltipModule } from '@angular/material/tooltip';
import { LoadingIndicatorComponent } from '@softwarity/loading-indicator';
import { RowActionsDirective } from '@softwarity/row-actions';
import { ApiService, VaultEntry } from '../api.service';
import { DialogsService } from '../shared/dialogs.service';
import { VaultEntryFormComponent, VaultEntryFormData } from '../shared/vault-entry-form.component';
import { VaultService } from '../shared/vault.service';
import { VaultFileDialogComponent, VaultFileDialogData } from './vault-file-dialog.component';

// The vault (VAULT-01/02): every named value the configuration refers to, in
// one place. Secrets are encrypted at rest and never shown again; plain values
// are readable. Both are referenced the same way, by $name, so promoting a
// value to a secret never touches what points at it. The "used by" column is
// what tells a live entry from a leftover.
@Component({
  selector: 'app-vault-page',
  imports: [
    MatButtonModule,
    MatIconModule,
    MatSidenavModule,
    MatTableModule,
    MatTooltipModule,
    LoadingIndicatorComponent,
    RowActionsDirective,
    VaultEntryFormComponent,
  ],
  styleUrl: './vault-page.component.scss',
  templateUrl: './vault-page.component.html',
})
export class VaultPageComponent {
  private readonly api = inject(ApiService);
  private readonly vault = inject(VaultService);
  private readonly dialog = inject(MatDialog);
  private readonly dialogs = inject(DialogsService);
  private readonly snack = inject(MatSnackBar);

  protected readonly loading = signal(true);
  protected readonly entries = computed(() => this.vault.entries());
  protected readonly columns = ['name', 'kind', 'value', 'usedBy'];

  // The editor lives in a right drawer here (the routes/users pattern), holding
  // the shared form. null = shut; a value carries what the form seeds from.
  protected readonly editor = signal<VaultEntryFormData | null>(null);

  constructor() {
    void this.vault.reload().then(() => this.loading.set(false));
  }

  // The encrypted vault file: taking the values somewhere else, or bringing
  // them in. An import may fill entries a configuration import left empty, so
  // the table is reloaded after it.
  protected exportFile(): void {
    this.dialog.open<VaultFileDialogComponent, VaultFileDialogData>(VaultFileDialogComponent, {
      data: { mode: 'export' },
      width: '560px',
      restoreFocus: true,
    });
  }

  protected importFile(): void {
    this.dialog
      .open<VaultFileDialogComponent, VaultFileDialogData>(VaultFileDialogComponent, {
        data: { mode: 'import' },
        width: '560px',
        restoreFocus: true,
      })
      .afterClosed()
      .subscribe(() => void this.vault.reload());
  }

  protected create(): void {
    this.editor.set({});
  }

  protected edit(entry: VaultEntry): void {
    this.editor.set({ entry });
  }

  // The form reloads the vault itself; here we just shut the drawer.
  protected onSaved(): void {
    this.editor.set(null);
  }

  protected async remove(entry: VaultEntry): Promise<void> {
    const ok = await this.dialogs.confirm({
      title: $localize`:@@Delete_vault_entry_NAME:Delete "${entry.name}:NAME:"?`,
      confirmLabel: $localize`:@@Delete:Delete`,
      danger: true,
    });
    if (!ok) return;
    this.api.deleteVaultEntry(entry.scope, entry.name).subscribe({
      next: () => void this.vault.reload(),
      error: (err: unknown) => {
        const e = err as { error?: { error?: string } };
        this.snack.open(
          typeof e?.error?.error === 'string' ? e.error.error : $localize`:@@Request_failed:Request failed`,
          undefined,
          { duration: 5000 },
        );
      },
    });
  }
}
