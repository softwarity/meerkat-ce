import { HttpErrorResponse, HttpInterceptorFn } from '@angular/common/http';
import { catchError, throwError } from 'rxjs';
import { goToLogin } from './session';

// A 401 means no valid session: hand the browser to the gateway's login page
// (served on this same origin) with a return path back into the console.
//
// This is the LAST line, not the first: SessionWatchService leaves on time,
// before a call has to fail to find out. This still matters for a session that
// ended early - revoked, signed out elsewhere, an account disabled - where no
// deadline was ever reached.
export const authInterceptor: HttpInterceptorFn = (req, next) =>
  next(req).pipe(
    catchError((err: unknown) => {
      if (err instanceof HttpErrorResponse && err.status === 401) goToLogin();
      return throwError(() => err);
    }),
  );
