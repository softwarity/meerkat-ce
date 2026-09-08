import { inject, Injectable } from '@angular/core';
import { MatSnackBar } from '@angular/material/snack-bar';

// A dialog that arrives on demand is a FILE fetched on the click, named by its
// content hash. Deploy the console again and that name is gone: a tab opened
// before the deploy asks the server for a chunk it no longer has, the import
// rejects, and the click does NOTHING - no dialog, no message, nothing saying
// the console moved underneath. Which reads as a broken feature, and is the
// worst of the three possible outcomes.
@Injectable({ providedIn: 'root' })
export class Lazy {
  private readonly snack = inject(MatSnackBar);

  // Load a lazy chunk, or say why nothing opened. Undefined means the chunk is
  // gone and the caller stops - the snackbar has already explained it, and its
  // action is the only real fix: this page is running code that no longer
  // exists on the server.
  async load<T>(chunk: () => Promise<T>): Promise<T | undefined> {
    try {
      return await chunk();
    } catch {
      this.snack
        .open(
          $localize`:@@Console_updated:The console has been updated since this page was opened`,
          $localize`:@@Reload:Reload`,
          { duration: 15000 },
        )
        .onAction()
        .subscribe(() => location.reload());
      return undefined;
    }
  }
}
