import { HttpErrorResponse } from '@angular/common/http';
import { Component, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MAT_DIALOG_DATA, MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatIconModule } from '@angular/material/icon';
import { MatProgressBarModule } from '@angular/material/progress-bar';
import { MatRadioModule } from '@angular/material/radio';
import { ApiService, Certificate, MountedCert, MountedPki } from '../../api.service';
import { coversOne } from './cert-names';
import { ZonedDatePipe } from '../../shared/zoned-date.pipe';
import { MeService } from '../../me.service';

// The file system door of the pool (SSL-11): what the runtime MOUNTED in this
// container - a Kubernetes Secret, a Swarm secret, a compose volume - read
// from the mount table, wherever the deployment put it. Each certificate is
// shown with the key found beside it; one without a key serves nothing and
// cannot be picked. Only the paths are kept: the key stays in its file, and a
// renewal written over it is served on its own.
@Component({
  selector: 'app-file-certificate-dialog',
  imports: [MatButtonModule, MatDialogModule, MatIconModule, MatProgressBarModule, MatRadioModule, ZonedDatePipe],
  templateUrl: './file-certificate-dialog.component.html',
  styleUrl: './file-certificate-dialog.component.scss',
})
export class FileCertificateDialogComponent {
  private readonly api = inject(ApiService);
  protected readonly zone = inject(MeService).timezone;
  private readonly ref = inject(MatDialogRef<FileCertificateDialogComponent, Certificate>);
  private readonly data = inject<{ taken: string[]; internalNames?: string[] }>(MAT_DIALOG_DATA);
  // The certificate files the pool already reads.
  private readonly taken = new Set(this.data?.taken ?? []);
  // The names the runtime resolves to the internal plane: each certificate
  // says at once whether it can be served there.
  protected readonly internalNames = this.data?.internalNames ?? [];

  protected fitsInternal(c: MountedCert): boolean {
    return coversOne(c.names, this.internalNames);
  }

  protected inPool(c: MountedCert): boolean {
    return this.taken.has(c.path);
  }

  protected readonly loading = signal(true);
  protected readonly mounts = signal<MountedPki[]>([]);
  protected readonly why = signal('');
  protected readonly picked = signal<MountedCert | null>(null);
  protected readonly saving = signal(false);
  protected readonly error = signal('');

  constructor() {
    this.api.mountedCertificates().subscribe({
      next: (v) => {
        this.mounts.set(v.mounts);
        this.why.set(v.why ?? '');
        this.loading.set(false);
      },
      error: () => {
        this.why.set($localize`:@@Request_failed:Request failed`);
        this.loading.set(false);
      },
    });
  }

  protected submit(): void {
    const c = this.picked();
    if (!c?.keyPath) return;
    this.saving.set(true);
    this.error.set('');
    this.api.createFileCertificate({ certPath: c.path, keyPath: c.keyPath }).subscribe({
      next: (created) => this.ref.close(created),
      error: (e: HttpErrorResponse) => {
        this.saving.set(false);
        this.error.set(e.error?.error ?? $localize`:@@Request_failed:Request failed`);
      },
    });
  }
}
