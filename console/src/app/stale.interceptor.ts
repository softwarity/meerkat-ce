import { HttpErrorResponse, HttpInterceptorFn } from '@angular/common/http';
import { inject } from '@angular/core';
import { MatSnackBar } from '@angular/material/snack-bar';
import { catchError, throwError } from 'rxjs';

// A 409 means the thing being saved moved under this screen: somebody else
// wrote it since it was opened, and the server refused a save built on the
// version this page is still showing.
//
// Handled HERE and not in each editor for one reason: it can happen on every
// screen that saves, it always means the same thing, and it always has the same
// answer - look at what is there now, then redo the change. A message written
// twelve times is a message that says something slightly different in twelve
// places, and the one nobody updates is the one somebody reads.
//
// The error still travels on: the screen shows its own line (the server says
// which revision was held and which one the object is on), and this adds the
// way out.
export const staleInterceptor: HttpInterceptorFn = (req, next) => {
  const snack = inject(MatSnackBar);
  return next(req).pipe(
    catchError((err: unknown) => {
      if (err instanceof HttpErrorResponse && err.status === 409) {
        snack
          .open(
            $localize`:@@Changed_since_you_opened_it:Somebody changed this since you opened it. Your change was not saved - reload to see the current version, then apply it again.`,
            $localize`:@@Reload:Reload`,
            { duration: 15000, panelClass: 'stale-snack' },
          )
          .onAction()
          .subscribe(() => location.reload());
      }
      return throwError(() => err);
    }),
  );
};
