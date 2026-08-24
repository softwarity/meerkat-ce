import { Component, computed, inject, model, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MAT_DIALOG_DATA, MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { CertRequest } from '../../api.service';
import { FormFieldComponent } from '../../shared/form-field.component';

// The three doors an operator opens by hand, in one dialog: they ask for
// almost the same thing, and splitting them into three screens would have
// meant maintaining the same host list three times.
export type CertificateDoor = 'import' | 'self-signed' | 'signing-request';

// What the dialog is opened with: which door, and the hosts to start from -
// the plane's declared names, so nobody types them a second time.
export interface CertificateDialogData {
  door: CertificateDoor;
  hosts: string;
}

export interface CertificateDialogResult {
  door: CertificateDoor;
  name: string;
  certPem?: string;
  keyPem?: string;
  keystore?: string;
  password?: string;
  request?: CertRequest;
}

@Component({
  selector: 'app-certificate-dialog',
  imports: [
    MatButtonModule,
    MatCheckboxModule,
    MatDialogModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatSelectModule,
    FormFieldComponent,
  ],
  styleUrl: './certificate-dialog.component.scss',
  templateUrl: './certificate-dialog.component.html',
})
export class CertificateDialogComponent {
  private readonly ref = inject(MatDialogRef<CertificateDialogComponent, CertificateDialogResult>);
  private readonly data = inject<CertificateDialogData>(MAT_DIALOG_DATA);
  protected readonly door = this.data.door;

  protected readonly name = model('');

  // Import, shape one: the PEM pair a CA e-mails back.
  protected readonly certPem = model('');
  protected readonly keyPem = model('');
  // Import, shape two: a .p12 or .pfx. It is read in the browser and travels
  // as base64 - a keystore is a few kilobytes, and a multipart upload for that
  // would be plumbing nobody gains from.
  protected readonly keystoreName = signal('');
  protected readonly keystore = signal('');
  protected readonly password = model('');

  // Generation, both doors.
  protected readonly hosts = model(this.data.hosts ?? '');
  protected readonly organization = model('');
  protected readonly keyType = model('ecdsa-p256');
  protected readonly days = model(397);

  protected readonly title = computed(() => {
    switch (this.door) {
      case 'import':
        return $localize`:@@Import_a_certificate:Import a certificate`;
      case 'self-signed':
        return $localize`:@@Generate_a_self_signed_certificate:Generate a self-signed certificate`;
      default:
        return $localize`:@@Create_a_signing_request:Create a signing request`;
    }
  });

  protected readonly keyTypes = [
    { value: 'ecdsa-p256', label: 'ECDSA P-256' },
    { value: 'ecdsa-p384', label: 'ECDSA P-384' },
    { value: 'rsa-2048', label: 'RSA 2048' },
    { value: 'rsa-4096', label: 'RSA 4096' },
  ];

  // A host list typed as one line, because that is how an operator holds it in
  // their head: the names this certificate answers for. Commas, spaces or new
  // lines all separate - correcting punctuation is not their job.
  protected readonly hostList = computed(() =>
    this.hosts()
      .split(/[\s,;]+/)
      .map((h) => h.trim())
      .filter(Boolean),
  );

  // An entry that parses as an address goes in as one: a certificate can carry
  // an IP, and asking for two fields would only be asking the operator to sort
  // their own list.
  private readonly ipish = /^[0-9.]+$|:/;

  protected readonly invalid = computed(() => {
    if (this.door === 'import') {
      return !this.keystore() && !(this.certPem().trim() && this.keyPem().trim());
    }
    return this.hostList().length === 0;
  });

  protected async pickKeystore(event: Event): Promise<void> {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    const buf = new Uint8Array(await file.arrayBuffer());
    let binary = '';
    for (const b of buf) binary += String.fromCharCode(b);
    this.keystore.set(btoa(binary));
    this.keystoreName.set(file.name);
    if (!this.name().trim()) this.name.set(file.name.replace(/\.(p12|pfx)$/i, ''));
  }

  protected submit(): void {
    const name = this.name().trim();
    if (this.door === 'import') {
      this.ref.close({
        door: 'import',
        name,
        certPem: this.keystore() ? undefined : this.certPem(),
        keyPem: this.keystore() ? undefined : this.keyPem(),
        keystore: this.keystore() || undefined,
        password: this.keystore() ? this.password() : undefined,
      });
      return;
    }
    const dnsNames = this.hostList().filter((h) => !this.ipish.test(h));
    const ipAddresses = this.hostList().filter((h) => this.ipish.test(h));
    this.ref.close({
      door: this.door,
      name,
      request: {
        name,
        dnsNames,
        ipAddresses,
        organization: this.organization().trim() || undefined,
        keyType: this.keyType(),
        days: this.door === 'self-signed' ? this.days() : undefined,
      },
    });
  }
}
