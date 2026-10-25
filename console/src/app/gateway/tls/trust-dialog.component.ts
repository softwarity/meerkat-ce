import { HttpErrorResponse } from '@angular/common/http';
import { Component, computed, inject, signal } from '@angular/core';
import { FormField, form, required, validate } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { ApiService, TrustedAuthority } from '../../api.service';
import { FileButtonComponent, FileDropDirective } from '../../shared/file-pick';
import { FormFieldComponent } from '../../shared/form-field.component';

// Adding a trusted authority (SSL-09): a name, and its certificate - a file
// chosen, dropped anywhere on the dialog, or the PEM pasted.
//
// It saves itself and closes with what was created, so a refusal from the
// gateway is said where it can be fixed.
@Component({
  selector: 'app-trust-dialog',
  imports: [
    FileButtonComponent,
    FileDropDirective,
    FormField,
    FormFieldComponent,
    MatButtonModule,
    MatDialogModule,
    MatIconModule,
    MatInputModule,
  ],
  templateUrl: './trust-dialog.component.html',
  styleUrl: './trust-dialog.component.scss',
})
export class TrustDialogComponent {
  private readonly api = inject(ApiService);
  private readonly ref = inject(MatDialogRef<TrustDialogComponent, TrustedAuthority>);

  protected readonly draft = signal({ name: '', pem: '' });
  protected readonly f = form(this.draft, (p) => {
    required(p.name, { message: $localize`:@@Trust_name_required:A name is required.` });
    validate(p.pem, ({ value }) => {
      const v = value();
      if (!v.trim()) return [{ kind: 'pem', message: $localize`:@@Trust_pem_required:Paste a PEM, or choose or drop its file.` }];
      if (v.includes('PRIVATE KEY')) {
        return [{ kind: 'pem', message: $localize`:@@Trust_pem_key:This contains a private key: give the authority's public certificate only.` }];
      }
      if (!v.includes('-----BEGIN CERTIFICATE-----')) {
        return [{ kind: 'pem', message: $localize`:@@Trust_pem_shape:Expected blocks starting with -----BEGIN CERTIFICATE-----.` }];
      }
      return [];
    });
  });
  protected readonly pemMessage = computed(() => this.f.pem().errors()[0]?.message ?? '');
  protected readonly saving = signal(false);
  protected readonly error = signal('');

  // A file chosen or dropped becomes the PEM; its name names the entry when
  // nothing was typed yet.
  protected async take(files: File[]): Promise<void> {
    const file = files[0];
    if (!file) return;
    const text = await file.text();
    this.draft.update((d) => ({ ...d, pem: text, name: d.name || file.name.replace(/\.(pem|crt|cer)$/i, '') }));
  }

  protected submit(): void {
    if (this.f().invalid()) return;
    const d = this.draft();
    this.saving.set(true);
    this.error.set('');
    this.api.createTrustedAuthority({ name: d.name, pem: d.pem }).subscribe({
      next: (created) => this.ref.close(created),
      error: (e: HttpErrorResponse) => {
        this.saving.set(false);
        this.error.set(e.error?.error ?? $localize`:@@Request_failed:Request failed`);
      },
    });
  }
}
