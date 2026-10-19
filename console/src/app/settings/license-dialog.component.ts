import { Component, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatDialogModule, MatDialogRef } from '@angular/material/dialog';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { ApiService, Edition } from '../api.service';
import { FileButtonComponent, FileDropDirective } from '../shared/file-pick';

// The license Softwarity sent, pasted, dropped or chosen. The server verifies
// it before keeping it, and its refusal says why in a sentence: that sentence
// is shown here as is, the dialog staying open on what was given.
@Component({
  selector: 'app-license-dialog',
  imports: [
    MatButtonModule,
    MatDialogModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    FileButtonComponent,
    FileDropDirective,
  ],
  template: `
    <h2 mat-dialog-title i18n="@@Change_license">Change license</h2>
    <mat-dialog-content>
      <p class="hint" i18n="@@Change_license_hint">
        Paste the license file Softwarity sent, drop it here, or choose it. It is checked before it is kept.
      </p>
      <div class="drop" appFileDrop (filesDropped)="read($event[0])">
        <mat-form-field subscriptSizing="dynamic">
          <mat-label i18n="@@License_file">License file</mat-label>
          <textarea
            matInput
            rows="10"
            spellcheck="false"
            [value]="text()"
            (input)="text.set($any($event.target).value); error.set('')"
          ></textarea>
        </mat-form-field>
      </div>
      @if (error()) {
        <p class="error">{{ error() }}</p>
      }
    </mat-dialog-content>
    <mat-dialog-actions>
      <app-file-button appearance="text" (picked)="read($event[0])" i18n="@@Choose_a_file">Choose a file</app-file-button>
      <span class="grow"></span>
      <button matButton mat-dialog-close i18n="@@Cancel">Cancel</button>
      <button matButton="filled" [disabled]="!text().trim() || saving()" (click)="save()" i18n="@@Save">Save</button>
    </mat-dialog-actions>
  `,
  styles: `
    .hint {
      margin: 0 0 12px;
      color: var(--mat-sys-on-surface-variant);
      font-size: 0.85rem;
    }
    mat-form-field {
      width: 100%;
    }
    textarea {
      font-family: var(--mk-mono);
      font-size: 0.8rem;
    }
    .drop {
      border-radius: 8px;
    }
    .drop.dragging {
      outline: 2px dashed var(--mat-sys-primary);
      outline-offset: 4px;
    }
    .error {
      margin: 12px 0 0;
      color: var(--mat-sys-error);
      font-size: 0.85rem;
    }
    .grow {
      flex: 1;
    }
  `,
})
export class LicenseDialogComponent {
  private readonly api = inject(ApiService);
  private readonly ref = inject(MatDialogRef<LicenseDialogComponent, Edition>);

  protected readonly text = signal('');
  protected readonly error = signal('');
  protected readonly saving = signal(false);

  // A file dropped or chosen lands in the text area, where it can be read
  // before it is sent: the same path as a paste.
  protected async read(file: File | undefined): Promise<void> {
    if (!file) return;
    this.text.set(await file.text());
    this.error.set('');
  }

  protected save(): void {
    this.saving.set(true);
    this.api.putLicense(this.text()).subscribe({
      next: (e) => this.ref.close(e),
      error: (e) => {
        this.saving.set(false);
        this.error.set(typeof e?.error?.error === 'string' ? e.error.error : $localize`:@@Save_failed:Save failed`);
      },
    });
  }
}
