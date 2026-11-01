import { Component, computed, inject, signal } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MAT_DIALOG_DATA, MatDialogModule } from '@angular/material/dialog';
import { MatRadioModule } from '@angular/material/radio';
import { ComposeLeaves } from '../../../api.service';

export interface SaveFileData {
  leaves: ComposeLeaves;
  // The routes of the result with a deposited spec, and with files.
  specs: string[];
  files: string[];
}

// The two shapes of a result on a filesystem, and what each one leaves behind:
// the moment the operator can still act on it. Closes with 'file', 'package',
// or nothing (cancelled).
@Component({
  selector: 'app-save-file-dialog',
  imports: [MatButtonModule, MatDialogModule, MatRadioModule],
  template: `
    <h2 mat-dialog-title i18n="@@Save_on_the_filesystem">Save on the filesystem</h2>
    <mat-dialog-content>
      <mat-radio-group class="shapes" [value]="shape()" (change)="shape.set($event.value)" aria-label="Format" i18n-aria-label="@@Format">
        <mat-radio-button value="file" i18n="@@As_a_file">As a file</mat-radio-button>
        <mat-radio-button value="package" [disabled]="!carries" i18n="@@As_a_package">As a package with its assets</mat-radio-button>
      </mat-radio-group>
      @if (!carries) {
        <p class="note" i18n="@@Package_nothing_to_carry">No image, spec or file to carry: a package would add nothing.</p>
      }
      @if (behind().length) {
        <h3 i18n="@@Left_behind">Left behind</h3>
        <ul>
          @for (line of behind(); track line) {
            <li>{{ line }}</li>
          }
        </ul>
      } @else {
        <p class="note" i18n="@@Nothing_left_behind">Nothing is left behind.</p>
      }
    </mat-dialog-content>
    <mat-dialog-actions align="end">
      <button matButton mat-dialog-close i18n="@@Cancel">Cancel</button>
      <button matButton="filled" [mat-dialog-close]="shape()" i18n="@@Save">Save</button>
    </mat-dialog-actions>
  `,
  styles: `
    .shapes {
      display: flex;
      flex-direction: column;
    }
    h3 {
      margin: 16px 0 4px;
      font: var(--mat-sys-title-small);
    }
    ul {
      margin: 0;
      padding-left: 20px;
    }
    li + li {
      margin-top: 4px;
    }
    .note {
      font-size: 0.85rem;
      color: var(--mat-sys-on-surface-variant);
    }
  `,
})
export class SaveFileDialogComponent {
  protected readonly data = inject<SaveFileData>(MAT_DIALOG_DATA);
  // What a package would carry that a plain file loses.
  protected readonly carries = this.data.leaves.images || this.data.specs.length > 0 || this.data.files.length > 0;
  // Nothing lost by default: the package when there is something to carry.
  protected readonly shape = signal<'file' | 'package'>(this.carries ? 'package' : 'file');

  protected readonly behind = computed(() => {
    const { leaves, specs, files } = this.data;
    const out: string[] = [];
    if (this.shape() === 'file') {
      if (leaves.images) out.push($localize`:@@Left_images:The images: importing the file keeps the ones in place.`);
      if (specs.length) out.push($localize`:@@Left_specs:The API specs of ${specs.join(', ')}:routes:.`);
      if (files.length) out.push($localize`:@@Left_files:The files of ${files.join(', ')}:routes:.`);
    }
    if (leaves.refs.length) {
      out.push($localize`:@@Left_vault:The values of ${leaves.refs.join(', ')}:names:: the file names these vault entries, the gateway that reads it needs them in its own vault.`);
    }
    if (leaves.literals.length) {
      const held = leaves.literals.map((l) => `${l.label} ${l.field}`).join(', ');
      out.push($localize`:@@Left_literals:${held}:secrets:: held in clear here, never exported. Move them to the vault to carry a reference.`);
    }
    return out;
  });
}
