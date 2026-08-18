import { Injectable } from '@angular/core';

// The console's half of the session watch. The gateway measures the lifetime
// as INACTIVITY - every request pushes the deadline - and reposes it in a
// readable cookie beside the httpOnly session one, because a page could not
// otherwise tell whether it is still signed in. Here we read that cookie, leave
// on time, and tell the other tabs.
//
// The data plane has the same watch inside its injected page agent
// (internal/auth/page.go). The two never meet: BroadcastChannel is scoped to
// an origin, and the two planes listen on different ports.
const UNTIL_COOKIE = 'MEERKAT_ADMIN_UNTIL';
const CHANNEL = 'meerkat-session';

// ONCE, and that "once" is the whole point. A cold start fires several calls at
// the same time (/api/me, /api/tenants...) and they all come back 401 together.
// Assigning location.href per failure does not redirect harder: each assignment
// CANCELS the navigation the previous one started, so the browser keeps
// restarting it, never leaves the page, and shows a blank screen with nothing
// in the console to explain it. Cost: an afternoon of looking everywhere else.
let leaving = false;

/** Hand the browser to the gateway's login page, with a way back. */
export function goToLogin(): void {
  if (leaving) return;
  leaving = true;
  location.href = `/login?next=${encodeURIComponent(location.pathname + location.search)}`;
}

function deadline(): number {
  const raw = (document.cookie.split('; ').find((c) => c.startsWith(`${UNTIL_COOKIE}=`)) || '')
    .split('=')[1];
  return Number(raw || 0) * 1000;
}

@Injectable({ providedIn: 'root' })
export class SessionWatchService {
  private channel: BroadcastChannel | null = null;

  /**
   * Start watching. Without this the console sat on a live-looking screen until
   * someone clicked, and only then learned the session was gone - the 401
   * interceptor cannot fire before a request does.
   */
  start(): void {
    // No deadline means no session at all: the interceptor will send the first
    // call to the login page, and there is nothing here to count down.
    if (!deadline()) return;
    this.open();
    // Say it out loud: a tab of this console sitting on the login form, whose
    // session ended at the same moment as ours, goes back to where it was
    // rather than ask for a second sign-in.
    this.tell('alive');

    const check = (): void => {
      if (deadline() > Date.now()) return;
      this.tell('ended');
      goToLogin();
    };
    setInterval(check, 30_000);
    // A laptop that slept through the deadline: timers do not fire while it is
    // away, so the question is asked again the moment it comes back.
    addEventListener('visibilitychange', () => {
      if (!document.hidden) check();
    });
    if (this.channel) {
      this.channel.onmessage = (e: MessageEvent<{ state?: string }>) => {
        // Signing out in another tab ends it here too, at once, instead of
        // leaving this screen looking signed in until its next call fails.
        if (e.data?.state === 'ended' && !deadline()) goToLogin();
      };
    }
  }

  /** Tell the other tabs the session just ended (called on sign-out). */
  signedOut(): void {
    this.open();
    this.tell('ended');
  }

  private open(): void {
    if (this.channel || typeof BroadcastChannel !== 'function') return;
    try {
      this.channel = new BroadcastChannel(CHANNEL);
    } catch {
      this.channel = null;
    }
  }

  private tell(state: 'alive' | 'ended'): void {
    this.channel?.postMessage({ state });
  }
}
