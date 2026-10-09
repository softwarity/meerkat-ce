import { Component, computed, inject, model, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MAT_DIALOG_DATA, MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { FormFieldComponent } from '../../shared/form-field.component';
import { FileButtonComponent, FileDropDirective } from '../../shared/file-pick';

// The four doors an operator opens by hand, in one dialog. Each makes ONE
// certificate for the pool; where it is served is decided afterwards, by
// placing it on the console, the application, or both.
export type CertificateDoor = 'self-signed' | 'pem' | 'keystore' | 'signing-request' | 'acme';

export interface CertificateDialogData {
  door: CertificateDoor;
  // What the name field starts with: the name this console is reached by,
  // which is the name a first certificate is most often for.
  names?: string;
  // The authority is a public one (Let's Encrypt): it has to reach a name
  // from the internet to prove it, so a name that only exists here is flagged.
  publicAuthority?: boolean;
  // The authority an order is asked of, for the title.
  authorityName?: string;
}

export interface CertificateDialogResult {
  door: CertificateDoor;
  certPem?: string;
  keyPem?: string;
  keystore?: string;
  password?: string;
  request?: { names: string[]; organization?: string; keyType?: string; days?: number };
}

@Component({
  selector: 'app-certificate-dialog',
  imports: [
    FileButtonComponent,
    FileDropDirective,
    MatButtonModule,
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
  protected readonly data = inject<CertificateDialogData>(MAT_DIALOG_DATA);
  protected readonly door = this.data.door;

  // Generation, both doors: the names the certificate answers for.
  protected readonly names = model(this.data.names ?? '');
  protected readonly organization = model('');
  protected readonly keyType = model('ecdsa-p256');
  protected readonly days = model(397);

  // The PEM pair, pasted, picked or dropped.
  protected readonly certPem = model('');
  protected readonly keyPem = model('');
  protected readonly certFile = signal('');
  protected readonly keyFile = signal('');

  // A .p12 or .pfx, read in the browser and carried as base64 - a keystore is
  // a few kilobytes, and multipart for that would be plumbing nobody gains from.
  protected readonly keystoreName = signal('');
  protected readonly keystore = signal('');
  protected readonly password = model('');

  protected readonly dropError = signal('');

  protected readonly title = computed(() => {
    switch (this.door) {
      case 'pem':
        return $localize`:@@Import_a_PEM_pair:Import a PEM pair`;
      case 'keystore':
        return $localize`:@@Import_a_keystore:Import a keystore`;
      case 'self-signed':
        return $localize`:@@Generate_a_self_signed_certificate:Generate a self-signed certificate`;
      case 'acme':
        return $localize`:@@Ask_authority_name:Ask ${this.data.authorityName}:name:`;
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

  protected readonly nameList = computed(() =>
    this.names()
      .split(/[\s,;]+/)
      .map((n) => n.trim())
      .filter(Boolean),
  );

  // Names a public authority can never reach: no dot, or a suffix reserved
  // for local use.
  protected readonly unreachable = computed(() =>
    this.door === 'acme' && this.data.publicAuthority
      ? this.nameList().filter((n) => !n.includes('.') || /\.(local|internal|test|localhost|home\.arpa)$/.test(n))
      : [],
  );

  protected readonly invalid = computed(() => {
    switch (this.door) {
      case 'pem':
        return !(this.certPem().trim() && this.keyPem().trim());
      case 'keystore':
        return !this.keystore();
      default:
        return this.nameList().length === 0;
    }
  });

  // ── files ──────────────────────────────────────────────────────────────────

  // Files chosen or dropped anywhere on the dialog: a keystore door takes the
  // first, a PEM door sorts each one by what it holds.
  protected async take(files: File[]): Promise<void> {
    if (this.door === 'keystore') {
      if (files[0]) await this.readKeystore(files[0]);
      return;
    }
    for (const f of files) await this.readPem(f);
  }

  // A PEM file is sorted by what it holds, not by where it was dropped: the
  // key is the one with a PRIVATE KEY block. A file holding both - some tools
  // write one - fills both fields.
  private async readPem(file: File, slot?: 'cert' | 'key'): Promise<void> {
    this.dropError.set('');
    const text = await file.text();
    const key = text.match(/-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----/);
    const certs = text.match(/-----BEGIN CERTIFICATE-----[\s\S]*?-----END CERTIFICATE-----/g);
    if (!key && !certs) {
      this.dropError.set($localize`:@@Not_a_PEM_file:${file.name}:name: holds no PEM block`);
      return;
    }
    if (key && (slot !== 'cert' || !certs)) {
      this.keyPem.set(key[0] + '\n');
      this.keyFile.set(file.name);
    }
    if (certs && (slot !== 'key' || !key)) {
      this.certPem.set(certs.join('\n') + '\n');
      this.certFile.set(file.name);
    }
  }

  private async readKeystore(file: File): Promise<void> {
    const buf = new Uint8Array(await file.arrayBuffer());
    let binary = '';
    for (const b of buf) binary += String.fromCharCode(b);
    this.keystore.set(btoa(binary));
    this.keystoreName.set(file.name);
  }

  protected submit(): void {
    switch (this.door) {
      case 'pem':
        this.ref.close({ door: 'pem', certPem: this.certPem(), keyPem: this.keyPem() });
        return;
      case 'keystore':
        this.ref.close({ door: 'keystore', keystore: this.keystore(), password: this.password() });
        return;
    }
    this.ref.close({
      door: this.door,
      request: {
        names: this.nameList(),
        organization: this.organization().trim() || undefined,
        keyType: this.keyType(),
        days: this.door === 'self-signed' ? this.days() : undefined,
      },
    });
  }
}
