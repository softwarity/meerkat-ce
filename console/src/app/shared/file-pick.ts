import { booleanAttribute, Component, Directive, input, output, signal } from '@angular/core';
import { MatButtonAppearance, MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';

// Choosing and dropping files, written once. Every screen that takes a file -
// a palette, a language, a logo, a certificate, a spec, a route's files - used
// to carry its own hidden <input type="file">, its own reset of the input so
// the same file can be picked twice, and its own dragover/drop handlers. Three
// pieces here replace them:
//
//   pickFiles()        the system's file chooser, from any click - a button
//                      or a menu item, which cannot hold an input;
//   <app-file-button>  a Material button that opens it;
//   [appFileDrop]      makes any element a drop target.

export interface PickOptions {
  // The input's accept list: extensions (".json") and types ("image/*").
  accept?: string;
  multiple?: boolean;
}

// Opens the file chooser and resolves with what was chosen - an empty list
// when the chooser is dismissed. The input is created for the occasion and
// thrown away, so there is nothing to reset and nothing hidden in the page.
export function pickFiles(opts: PickOptions = {}): Promise<File[]> {
  return new Promise((resolve) => {
    const input = document.createElement('input');
    input.type = 'file';
    if (opts.accept) input.accept = opts.accept;
    input.multiple = !!opts.multiple;
    input.addEventListener('change', () => resolve(Array.from(input.files ?? [])), { once: true });
    input.addEventListener('cancel', () => resolve([]), { once: true });
    input.click();
  });
}

// Whether a file matches an accept list, the way the chooser filters: a drop
// bypasses the chooser, so the same list has to be checked by hand.
export function accepts(file: File, accept: string | undefined): boolean {
  if (!accept) return true;
  const name = file.name.toLowerCase();
  const type = file.type.toLowerCase();
  return accept
    .split(',')
    .map((a) => a.trim().toLowerCase())
    .filter(Boolean)
    .some((a) => (a.startsWith('.') ? name.endsWith(a) : a.endsWith('/*') ? type.startsWith(a.slice(0, -1)) : type === a));
}

// A button that opens the file chooser. The label is projected; the icon
// defaults to the upload one.
//
//   <app-file-button accept=".json" (picked)="import($event[0])" i18n="@@Import">Import</app-file-button>
@Component({
  selector: 'app-file-button',
  imports: [MatButtonModule, MatIconModule],
  template: `
    <button [matButton]="appearance()" type="button" [disabled]="disabled()" (click)="open()">
      <mat-icon>{{ icon() }}</mat-icon>
      <ng-content />
    </button>
  `,
  styles: `:host { display: inline-flex; }`,
})
export class FileButtonComponent {
  readonly accept = input<string>();
  readonly multiple = input(false, { transform: booleanAttribute });
  readonly disabled = input(false, { transform: booleanAttribute });
  readonly icon = input('upload_file');
  readonly appearance = input<MatButtonAppearance>('tonal');
  // The files chosen - never empty: a dismissed chooser emits nothing.
  readonly picked = output<File[]>();

  protected async open(): Promise<void> {
    const files = await pickFiles({ accept: this.accept(), multiple: this.multiple() });
    if (files.length) this.picked.emit(files);
  }
}

// Makes the host a drop target. While something is dragged over it the host
// wears the class named by dropClass ("dragging" unless told otherwise), and a
// drop emits the files the accept list lets through - none emits nothing.
//
//   <div class="dropzone" appFileDrop accept="image/*" (filesDropped)="use($event[0])">
@Directive({
  selector: '[appFileDrop]',
  host: {
    '(dragover)': 'over($event)',
    '(dragleave)': 'dragging.set(false)',
    '(drop)': 'drop($event)',
    '[class]': 'dragging() ? dropClass() : ""',
  },
})
export class FileDropDirective {
  readonly accept = input<string>();
  readonly multiple = input(false, { transform: booleanAttribute });
  readonly dropClass = input('dragging');
  readonly filesDropped = output<File[]>();
  protected readonly dragging = signal(false);

  protected over(e: DragEvent): void {
    if (!e.dataTransfer?.types.includes('Files')) return;
    e.preventDefault();
    this.dragging.set(true);
  }

  protected drop(e: DragEvent): void {
    if (!e.dataTransfer?.files.length) return;
    e.preventDefault();
    this.dragging.set(false);
    const files = Array.from(e.dataTransfer.files).filter((f) => accepts(f, this.accept()));
    if (files.length) this.filesDropped.emit(this.multiple() ? files : files.slice(0, 1));
  }
}
