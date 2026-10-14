import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MAT_DIALOG_DATA, MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatRadioModule } from '@angular/material/radio';
import { ApiService, Certificate, CertManagerIssuer, CertManagerView } from '../../api.service';

// The internal plane's certificate asked of cert-manager (SSL-13). Only the
// issuer is chosen here, among the ones the gateway found: the names are the
// ones the Internal door already lists, the Certificate and its Secret the
// name the chart granted - nothing typed.
@Component({
  selector: 'app-cert-manager-dialog',
  imports: [MatButtonModule, MatDialogModule, MatIconModule, MatRadioModule],
  templateUrl: './cert-manager-dialog.component.html',
  styleUrl: './cert-manager-dialog.component.scss',
})
export class CertManagerDialogComponent {
  private readonly api = inject(ApiService);
  private readonly ref = inject(MatDialogRef<CertManagerDialogComponent, Certificate>);
  // The plane rides along with the view: the internal one, or the console or
  // the application behind a proxy (SSL-14).
  protected readonly view = inject<CertManagerView & { plane?: string }>(MAT_DIALOG_DATA);

  // The only ready one is picked already.
  protected readonly picked = signal<CertManagerIssuer | null>(
    this.view.issuers.filter((i) => i.ready).length === 1 ? this.view.issuers.find((i) => i.ready)! : null,
  );
  protected readonly saving = signal(false);
  protected readonly error = signal('');

  protected submit(): void {
    const i = this.picked();
    if (!i) return;
    this.saving.set(true);
    this.error.set('');
    this.api.askCertManager({ kind: i.kind, name: i.name, plane: this.view.plane }).subscribe({
      next: (created) => this.ref.close(created),
      error: (e: HttpErrorResponse) => {
        this.saving.set(false);
        this.error.set(e.error?.error ?? $localize`:@@Request_failed:Request failed`);
      },
    });
  }
}
