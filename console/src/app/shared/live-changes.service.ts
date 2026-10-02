import { DestroyRef, Service, inject, signal } from '@angular/core';
import { MatSnackBar } from '@angular/material/snack-bar';
import { LiveList, LiveTopic, LivewireClient } from '@softwarity/livewire';
import type { LiveRow } from '@softwarity/livewire';

// The kinds of object a screen watches. The same words the gateway classes an
// administrative write under (store.AuditTargets), so a screen names what it
// shows rather than a topic invented here.
export type ChangeKind =
  | 'route'
  | 'user'
  | 'role'
  | 'tenant'
  | 'membership'
  | 'group'
  | 'grouprule'
  | 'theme'
  | 'locale'
  | 'authprovider'
  | 'certificate'
  | 'vault'
  | 'token'
  | 'settings'
  | 'schedule'
  | 'configuration'
  | 'config'
  | 'config-remote'
  | 'issue'
  | 'backup';

// One administrative write.
//
// The id is the write's own identifier, and it sorts by time - which is how a
// tab knows where it had got to with a string comparison and no clock.
//
// `targetId` names the row that moved, and that is what lets a screen reload
// ONE line instead of its whole list. It is EMPTY when the write moved rows
// nobody named - a reorder, a configuration import, "everyone must change their
// password" - and that emptiness is an instruction: read the list again.
//
// Everything after `at` is absent when the gateway will not name it: a kind
// whose rows belong to an organisation is partitioned by organisation, which
// one shared socket read cannot check per reader. The screen is then told that
// something moved, reloads through its own API - which does check - and simply
// has nobody to name.
export interface ChangeRow extends LiveRow {
  kind: ChangeKind;
  // When it happened, in seconds (the trail's own unit).
  at: number;
  action?: string;
  targetId?: string;
  targetName?: string;
  // Who did it, and - when an agent did - the control-plane token they held.
  actor?: string;
  actorToken?: string;
}

// The kind that stands for every other one: a configuration exported, imported
// or restored replays the installation, so every screen is showing something
// that may no longer be true.
const ALL_KINDS: ChangeKind = 'config';

// What somebody else just changed (CONSOLE-13).
//
// The problem it answers: two operators, or two tabs, and the second discovers
// at the moment of saving that the first got there before them. The row's
// revision already refuses that write with a 409, so nothing is lost - but the
// saving is where they find out, after typing. This makes the screen say so
// first: one socket, one line per write, and every screen reloads what it shows
// when something it shows moves.
//
// WHAT IT DOES NOT CARRY. Not the objects. What crosses is "route X was
// updated", and the screen then asks the API it already asks, with the caller's
// own session behind it - so there is exactly one place that decides who may
// read a route, and it is not this socket. See internal/live/changes.go for the
// other half of that argument.
@Service()
export class LiveChangesService {
  private readonly topic = new LiveTopic<ChangeRow>(inject(LivewireClient), 'changes');
  private readonly list = new LiveList<ChangeRow>();

  // The last write per kind, for a screen that wants to NAME it.
  private readonly rows = signal<Map<ChangeKind, ChangeRow>>(new Map());

  // Where this tab has got to in the journal: the id of the newest write it has
  // acted on. A string, compared as a string - the gateway's event ids sort by
  // time, so this needs no clock and survives a socket that comes back on
  // another node.
  private at = '';

  // Whether the first frame has landed. It is the reference point, and the
  // whole reference point: what the journal already held when this tab arrived
  // is what the screens have just read, so none of it is news.
  private primed = false;

  private readonly watchers = new Map<ChangeKind, Set<(row: ChangeRow) => void>>();

  // What is watching EVERY kind. One screen does that and it is the audit
  // trail: what it shows is the writes themselves, so every write is its news.
  private readonly all = new Set<(row: ChangeRow) => void>();

  private subscribed = false;

  private readonly snack = inject(MatSnackBar);

  // The writes THIS TAB made, by the identifier the server handed back on each
  // one (Meerkat-Change-Id, via own-writes.interceptor). A report of one of
  // them is an echo, not news, and a screen must not warn its user that
  // "admin changed this somewhere else" about the save they just made.
  //
  // Bounded: an identifier is only useful for the second or two between a save
  // and its echo, and a tab left open for a week must not grow without end.
  private readonly own: string[] = [];
  private static readonly OWN_KEPT = 200;

  // Writes sent and not yet answered. While one is in flight a report may be
  // its echo arriving ahead of the answer that would name it, so warnings wait.
  private inflight = 0;
  private readonly held: { change: ChangeRow; reload: () => void }[] = [];

  writeStarted(): void {
    this.inflight++;
  }

  writeEnded(ids: string[]): void {
    for (const id of ids) {
      this.own.push(id);
    }
    if (this.own.length > LiveChangesService.OWN_KEPT) {
      this.own.splice(0, this.own.length - LiveChangesService.OWN_KEPT);
    }
    this.inflight = Math.max(0, this.inflight - 1);
    if (this.inflight > 0) {
      return;
    }
    // Nothing in flight any more: whatever waited can now be told apart.
    const waiting = this.held.splice(0);
    for (const h of waiting) {
      this.offer(h.change, h.reload);
    }
  }

  // Whether a reported change is one this tab made - for a screen that decides
  // on its own what to do with a change, rather than through offer().
  isOwn(change: ChangeRow): boolean {
    return this.own.includes(change.id);
  }

  // The last write to one kind, as far as this tab has heard.
  //
  // Reads the signal, so a template calling it repaints on its own - which is
  // what a banner saying "changed by root" is read from.
  last(kind: ChangeKind): ChangeRow | undefined {
    return this.rows().get(kind);
  }

  // Calls back on every write to that kind - never for what happened before the
  // caller asked.
  //
  // The callback receives the write, so the screen decides what it costs: a
  // named row can be reloaded on its own, an empty `targetId` means the list
  // itself moved. A screen that does not want to tell them apart can reload
  // everything and be no worse off than before.
  //
  // Called from an injection context (a constructor or a field initialiser),
  // like takeUntilDestroyed: that is where the caller's own lifetime is
  // readable, and it is what unsubscribes a screen nobody is looking at.
  on(kind: ChangeKind, act: (row: ChangeRow) => void): void {
    this.subscribe();
    let watching = this.watchers.get(kind);
    if (!watching) {
      watching = new Set();
      this.watchers.set(kind, watching);
    }
    watching.add(act);
    inject(DestroyRef).onDestroy(() => watching?.delete(act));
  }

  // The other answer a screen can give, and the right one for a FORM: offer.
  //
  // A list can be replaced under somebody's eyes and lose them nothing. A form
  // cannot - reloading it while they are typing throws their work away for news
  // they did not ask for. So the screens that edit settings say what happened
  // and let the reader choose the moment, which is the same bargain the 409
  // interceptor offers from the other end.
  //
  // One snack bar at a time by design (Material replaces the open one), so ten
  // writes in a row leave one offer on screen rather than a queue.
  offer(change: ChangeRow, reload: () => void): void {
    // An echo of this tab's own save is not news.
    if (this.isOwn(change)) {
      return;
    }
    // A write of ours is still unanswered: this may be its echo arriving
    // first. Wait for the answer rather than warn and take it back.
    if (this.inflight > 0) {
      this.held.push({ change, reload });
      return;
    }
    this.snack
      .open(
        change.actor
          ? $localize`:@@NAME_changed_this_elsewhere:${change.actor}:NAME: changed this somewhere else.`
          : $localize`:@@This_changed_elsewhere:This was changed somewhere else.`,
        $localize`:@@Reload:Reload`,
        { duration: 12000 },
      )
      .onAction()
      .subscribe(() => reload());
  }

  // Calls back on EVERY write, whatever it touched. For the screen that shows
  // the writes themselves; anything else wants `on` and its own kind.
  onAny(act: (row: ChangeRow) => void): void {
    this.subscribe();
    this.all.add(act);
    inject(DestroyRef).onDestroy(() => this.all.delete(act));
  }

  // The one subscription, opened on the first listener and kept for the life of
  // the tab.
  //
  // Lazily, and that is not frugality: a service that subscribed when it was
  // injected would open a socket on a console nobody has signed in to yet, be
  // refused, and retry every three seconds behind a sign-in screen.
  private subscribe(): void {
    if (this.subscribed) {
      return;
    }
    this.subscribed = true;
    this.topic.open(null).subscribe((update) => {
      if (!this.list.apply(update)) {
        // The two sides disagree about what was sent. Asking again is cheap;
        // carrying on would mean deciding what moved from a list nobody trusts.
        this.topic.resync();
        return;
      }
      this.absorb(this.list.rows());
    });
  }

  // rows arrive newest first, which is the order the gateway publishes them in.
  private absorb(rows: ChangeRow[]): void {
    if (rows.length === 0) {
      this.primed = true;
      return;
    }
    const newest = rows[0].id;
    if (!this.primed) {
      this.primed = true;
      this.at = newest;
      this.remember(rows);
      return;
    }
    if (newest === this.at) {
      return;
    }

    // Did anything happen that this tab will never see? The journal is a
    // window: a laptop closed for an hour comes back to a window whose oldest
    // line is already newer than where it had got to, and the writes in between
    // are gone. Saying so is the difference between a screen that reloads once
    // too often and a screen that is quietly wrong for the rest of the day.
    const oldest = rows[rows.length - 1].id;
    const missed = this.at !== '' && this.at < oldest;

    const fresh = rows.filter((row) => row.id > this.at);
    this.at = newest;
    this.remember(rows);

    if (missed) {
      this.tellEveryone({
        id: newest,
        updatedAt: newest,
        kind: ALL_KINDS,
        at: rows[0].at,
        action: 'resync',
      });
      return;
    }
    // Oldest first: a screen applying them in order ends on the newest state,
    // and a create followed by a delete does not resurrect a row.
    for (const row of fresh.slice().reverse()) {
      if (row.kind === ALL_KINDS) {
        // tellEveryone covers the all-kinds watchers too - telling them here
        // as well would call the same screen twice for one write.
        this.tellEveryone(row);
        continue;
      }
      for (const act of this.all) {
        act(row);
      }
      for (const act of this.watchers.get(row.kind) ?? []) {
        act(row);
      }
    }
  }

  // A write under the all-kinds kind is not one screen's news, it is
  // everybody's: it names no row, so every listener is told to read its list
  // again.
  private tellEveryone(row: ChangeRow): void {
    for (const [kind, watching] of this.watchers) {
      for (const act of watching) {
        act({ ...row, kind, targetId: '' });
      }
    }
    for (const act of this.all) {
      act({ ...row, targetId: '' });
    }
  }

  // The newest write per kind, for the banners. A NEW map each time: mutating
  // the one the signal holds keeps the same reference, and a signal set to the
  // reference it already holds tells nobody - which in a zoneless application
  // means the banner never appears.
  private remember(rows: ChangeRow[]): void {
    const held = new Map<ChangeKind, ChangeRow>();
    for (const row of rows) {
      if (!held.has(row.kind)) {
        held.set(row.kind, row);
      }
    }
    this.rows.set(held);
  }
}
