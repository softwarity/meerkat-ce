import { HttpInterceptorFn, HttpResponse } from '@angular/common/http';
import { inject } from '@angular/core';
import { finalize, tap } from 'rxjs';
import { LiveChangesService } from './shared/live-changes.service';

// Tells the live channel which writes are this tab's own (CONSOLE-13).
//
// Every write is published to every console, this one included, and a screen
// that edits something warns when that something moved under it. Without this
// it warned about its own save - "admin changed this somewhere else" - because
// the event names an account, and two tabs of one operator are exactly the case
// the warning is for. The server answers each write with the identifiers of the
// events it produced (Meerkat-Change-Id); this hands them to the channel, which
// then knows which reports are an echo.
//
// The two can arrive in either order - the event on the socket before the
// answer to the request, or after - so the channel is also told a write is IN
// FLIGHT, and holds its warnings until the answer says whose change it was.
const WRITES = new Set(['POST', 'PUT', 'PATCH', 'DELETE']);

export const ownWritesInterceptor: HttpInterceptorFn = (req, next) => {
  if (!WRITES.has(req.method) || !req.url.includes('/api/')) {
    return next(req);
  }
  const live = inject(LiveChangesService);
  live.writeStarted();
  let ids: string[] = [];
  return next(req).pipe(
    tap((event) => {
      if (event instanceof HttpResponse) {
        ids = (event.headers.get('Meerkat-Change-Id') ?? '')
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean);
      }
    }),
    // Success, failure or cancellation: the write is no longer in flight, and
    // whatever was held back can now be judged.
    finalize(() => live.writeEnded(ids)),
  );
};
