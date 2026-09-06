import { Component, inject, input } from '@angular/core';
import { MatButtonModule } from '@angular/material/button';
import { MatIconModule } from '@angular/material/icon';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTooltipModule } from '@angular/material/tooltip';

// A configuration file, shown with the two things somebody does with one.
//
// Copy AND download, because they are not the same act: copying is for the
// three lines you paste into a file you already have, downloading is for the
// file you do not - and telling somebody to select twenty lines of YAML in a
// scrolling box with their mouse is how a good example goes unused.
//
// The filename is not decoration either: `prometheus.yml` and `servicemonitor.yaml`
// have to land under those names, and a browser's "download.txt" makes the
// reader do the one step the example was meant to save.
@Component({
  selector: 'app-snippet',
  imports: [MatButtonModule, MatIconModule, MatTooltipModule],
  template: `
    <div class="head">
      <code class="file">{{ filename() }}</code>
      <span class="grow"></span>
      <button
        matIconButton
        type="button"
        (click)="copy()"
        i18n-matTooltip="@@Copy"
        matTooltip="Copy"
        i18n-aria-label="@@Copy"
        aria-label="Copy"
      >
        <mat-icon>content_copy</mat-icon>
      </button>
      <button
        matIconButton
        type="button"
        (click)="download()"
        i18n-matTooltip="@@Download"
        matTooltip="Download"
        i18n-aria-label="@@Download"
        aria-label="Download"
      >
        <mat-icon>download</mat-icon>
      </button>
    </div>
    <pre><code>{{ content() }}</code></pre>
  `,
  styles: `
    :host {
      display: block;
      border: 1px solid var(--mat-sys-outline-variant);
      border-radius: 10px;
      overflow: hidden;
      margin-bottom: 12px;
    }
    .head {
      display: flex;
      align-items: center;
      gap: 4px;
      padding: 2px 4px 2px 12px;
      background: var(--mat-sys-surface-container);
    }
    .file {
      font-size: 0.75rem;
      color: var(--mat-sys-on-surface-variant);
    }
    .grow {
      flex: 1;
    }
    /* The box scrolls on its own rather than widening the drawer: a line of
       YAML that wraps is a line somebody pastes wrong. */
    pre {
      margin: 0;
      padding: 10px 12px;
      overflow-x: auto;
      font-size: 0.74rem;
      line-height: 1.5;
    }
  `,
})
export class SnippetComponent {
  readonly filename = input.required<string>();
  readonly content = input.required<string>();

  private readonly snack = inject(MatSnackBar);

  protected copy(): void {
    void navigator.clipboard?.writeText(this.content());
    this.snack.open($localize`:@@Copied:Copied`, undefined, { duration: 1500 });
  }

  protected download(): void {
    const url = URL.createObjectURL(new Blob([this.content()], { type: 'text/plain' }));
    const a = document.createElement('a');
    a.href = url;
    a.download = this.filename();
    a.click();
    // Released on the next turn: revoking it before the click has been acted
    // on cancels the download in some browsers.
    setTimeout(() => URL.revokeObjectURL(url), 0);
  }
}
