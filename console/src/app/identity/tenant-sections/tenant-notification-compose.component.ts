import { NgTemplateOutlet } from '@angular/common';
import { HttpErrorResponse } from '@angular/common/http';
import {
  afterNextRender,
  Component,
  computed,
  effect,
  ElementRef,
  inject,
  Injector,
  input,
  signal,
  untracked,
} from '@angular/core';
import { toSignal } from '@angular/core/rxjs-interop';
import { applyEach, disabled, form, FormField, maxLength, validate } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatButtonToggleModule } from '@angular/material/button-toggle';
import { MatExpansionModule } from '@angular/material/expansion';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatInputModule } from '@angular/material/input';
import { MatSelectModule } from '@angular/material/select';
import { MatSlideToggleModule } from '@angular/material/slide-toggle';
import { MatSnackBar } from '@angular/material/snack-bar';
import { MatTabsModule } from '@angular/material/tabs';
import { MatTooltipModule } from '@angular/material/tooltip';
import { ApiService, NotificationAck, NotificationLevel, NotifyCompose, NotifyRoute } from '../../api.service';
import { FormFieldComponent } from '../../shared/form-field.component';
import { JsonEditorComponent } from '../../shared/json-editor.component';
import { SnippetComponent } from '../../shared/snippet.component';
import { IdentityDrawer } from '../identity-drawer';

// The notification builder on a tenant's page (NOTIF-05): a guided form, the
// message it writes on a second tab - the very JSON a service publishes on
// the internal plane - and a Send that publishes it to this tenant now,
// through the same checks, so its administrator sees it arrive.
//
// A link is chosen as a route and a path under it; the form writes the
// {service, path} the message carries, the service named as the route's
// upstream and the path as that service sees it (the route's base before
// what is typed).

type LinkKind = 'none' | 'route' | 'url';

interface Link {
  kind: LinkKind;
  route: string;
  // What is typed after the route's base.
  path: string;
  url: string;
}

interface Draft {
  roles: string[];
  users: string[];
  level: NotificationLevel;
  title: string;
  body: string;
  detail: Link;
  // Each action: how the reader's browser calls it, and the JSON it sends
  // with any method but GET.
  actions: { label: string; link: Link; method: Method; body: string }[];
  // Asked of the readers, and the service called at each acknowledgement.
  ack: 'none' | NotificationAck;
  onAck: Link;
  expires: string;
  id: string;
  meta: { key: string; value: string }[];
  mail: boolean;
  // Sent to practise: the readers see it labelled EXERCISE.
  exercise: boolean;
}

const emptyLink = (kind: LinkKind): Link => ({ kind, route: '', path: '', url: '' });

// The methods an action is called with, as the server takes them.
const METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'] as const;
type Method = (typeof METHODS)[number];

// The body as it is sent: nothing when empty, undefined when it does not
// parse - the form says so under it.
function parseBody(text: string): { value?: unknown; error?: string } {
  if (!text.trim()) return {};
  try {
    return { value: JSON.parse(text) };
  } catch (e) {
    return { error: e instanceof Error ? e.message : String(e) };
  }
}

const HTTPS = /^https:\/\/[^\s/?#]+/;

// The lives offered; '' is the server's default, 30 days.
const LIVES: { key: string; label: string }[] = [
  { key: 'PT1H', label: $localize`:@@Notify_life_1h:1 hour` },
  { key: 'P1D', label: $localize`:@@Notify_life_1d:1 day` },
  { key: 'P7D', label: $localize`:@@Notify_life_7d:7 days` },
  { key: '', label: $localize`:@@Notify_life_30d:30 days` },
  { key: 'P90D', label: $localize`:@@Notify_life_90d:90 days` },
];

@Component({
  selector: 'app-tenant-notification-compose',
  imports: [
    FormField,
    FormFieldComponent,
    JsonEditorComponent,
    MatButtonModule,
    MatButtonToggleModule,
    MatExpansionModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatSelectModule,
    MatSlideToggleModule,
    MatTabsModule,
    MatTooltipModule,
    NgTemplateOutlet,
    SnippetComponent,
  ],
  templateUrl: './tenant-notification-compose.component.html',
  styleUrl: './tenant-notification-compose.component.scss',
})
export class TenantNotificationComposeComponent {
  readonly tenantId = input.required<string>();

  private readonly api = inject(ApiService);
  private readonly snack = inject(MatSnackBar);
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);
  protected readonly drawer = inject(IdentityDrawer);

  protected readonly opts = signal<NotifyCompose | null>(null);
  protected readonly lives = LIVES;
  protected readonly methods = METHODS;
  // The form, or the message it writes.
  protected readonly tab = signal<'edit' | 'json'>('edit');
  protected readonly moreOpen = signal(false);
  protected readonly sending = signal(false);
  // What the server refused, and the field it named.
  protected readonly refusal = signal<{ field: string; message: string } | null>(null);

  // Where a service reaches the internal plane, as the runtime names it.
  private readonly edition = toSignal(this.api.edition());
  protected readonly endpoint = computed(() => {
    const e = this.edition();
    const origin = e?.internalOrigin || `http://meerkat:${e?.internalPort || '7070'}`;
    return `POST ${origin}${this.opts()?.path ?? '/api/notifications'}`;
  });

  protected readonly draft = signal<Draft>({
    roles: [],
    users: [],
    level: 'info',
    title: '',
    body: '',
    detail: emptyLink('none'),
    actions: [],
    ack: 'none',
    onAck: emptyLink('none'),
    expires: '',
    // Required: the console is the sender here, and gives a fresh UUID the
    // administrator may change.
    id: crypto.randomUUID(),
    meta: [],
    mail: false,
    exercise: false,
  });

  protected readonly f = form(this.draft, (p) => {
    // The server's bounds, said while typing.
    maxLength(p.title, 200, { message: $localize`:@@Notify_title_long:200 characters at most.` });
    maxLength(p.body, 4000, { message: $localize`:@@Notify_text_long:4000 characters at most.` });
    applyEach(p.actions, (a) => {
      maxLength(a.label, 60, { message: $localize`:@@Notify_label_long:60 characters at most.` });
    });
    maxLength(p.id, 200, { message: $localize`:@@Notify_title_long:200 characters at most.` });
    validate(p.detail.url, ({ value, valueOf }) =>
      valueOf(p.detail.kind) === 'url' && value().trim() && !HTTPS.test(value().trim())
        ? [{ kind: 'https', message: $localize`:@@Notify_https_only:An address outside the gateway starts with https://` }]
        : [],
    );
    applyEach(p.actions, (a) => {
      validate(a.body, ({ value, valueOf }) => {
        if (valueOf(a.method) === 'GET') return [];
        return parseBody(value()).error
          ? [{ kind: 'json', message: $localize`:@@Notify_body_not_json:Not valid JSON. Fix it, or empty it to send no body.` }]
          : [];
      });
      validate(a.link.url, ({ value, valueOf }) =>
        valueOf(a.link.kind) === 'url' && value().trim() && !HTTPS.test(value().trim())
          ? [{ kind: 'https', message: $localize`:@@Notify_https_only:An address outside the gateway starts with https://` }]
          : [],
      );
    });
    disabled(p.mail, () => !this.opts()?.mail);
  });

  constructor() {
    effect(() => {
      const id = this.tenantId();
      this.api.tenantNotifyCompose(id).subscribe((o) => this.opts.set(o));
    });
    // A refusal speaks of the message it was given: the next edit drops it.
    effect(() => {
      this.draft();
      untracked(() => this.refusal.set(null));
    });
  }

  // ---- links ----

  protected readonly uiRoutes = computed(() => (this.opts()?.routes ?? []).filter((r) => r.ui));
  protected readonly allRoutes = computed(() => this.opts()?.routes ?? []);

  protected routeOf(link: Link): NotifyRoute | undefined {
    return link.kind === 'route' ? this.opts()?.routes.find((r) => r.route === link.route) : undefined;
  }

  // Why the server would refuse the chosen route, said before it does.
  protected routeProblem(link: Link, detail: boolean): string {
    const r = this.routeOf(link);
    return (detail ? r?.detailProblem : r?.problem) ?? '';
  }

  // The path the service sees: the route's base, then what was typed.
  private servicePath(link: Link): string {
    const base = this.routeOf(link)?.base ?? '';
    const rest = link.path.trim().replace(/^\/+/, '');
    return `${base}/${rest}`;
  }

  protected serviceHint(link: Link): string {
    const r = this.routeOf(link);
    return r ? $localize`:@@Notify_service_hint:Service ${r.service}:SERVICE:` : '';
  }

  protected openHint(url: string): string {
    return $localize`:@@Notify_opens_at:Opens ${url}:URL:`;
  }

  // Where the service is called at each acknowledgement: nobody opens it.
  protected callHint(url: string): string {
    return $localize`:@@Notify_calls_at:Calls ${url}:URL:`;
  }

  // What the reader's browser opens.
  protected opens(link: Link): string {
    const r = this.routeOf(link);
    if (!r?.public) return '';
    return r.public.replace(/\/+$/, '') + '/' + link.path.trim().replace(/^\/+/, '');
  }

  private target(link: Link): Record<string, string> | undefined {
    switch (link.kind) {
      case 'route':
        return { service: this.routeOf(link)?.service ?? '', path: this.servicePath(link) };
      case 'url':
        return { url: link.url.trim() };
    }
    return undefined;
  }

  protected setKind(link: 'detail' | 'onAck' | number, kind: LinkKind): void {
    this.draft.update((d) => {
      if (link === 'detail') return { ...d, detail: { ...d.detail, kind } };
      if (link === 'onAck') return { ...d, onAck: { ...d.onAck, kind } };
      return { ...d, actions: d.actions.map((a, i) => (i === link ? { ...a, link: { ...a.link, kind } } : a)) };
    });
  }

  protected setLevel(level: NotificationLevel): void {
    this.draft.update((d) => ({ ...d, level }));
  }

  protected setAck(ack: 'none' | NotificationAck): void {
    this.draft.update((d) => ({ ...d, ack }));
  }

  // ---- actions, metadata ----

  protected addAction(): void {
    this.draft.update((d) => ({
      ...d,
      actions: [...d.actions, { label: '', link: emptyLink('route'), method: 'GET', body: '' }],
    }));
  }

  protected removeAction(i: number): void {
    this.draft.update((d) => ({ ...d, actions: d.actions.filter((_, j) => j !== i) }));
  }

  protected addMeta(): void {
    this.draft.update((d) => ({ ...d, meta: [...d.meta, { key: '', value: '' }] }));
  }

  protected removeMeta(i: number): void {
    this.draft.update((d) => ({ ...d, meta: d.meta.filter((_, j) => j !== i) }));
  }

  // ---- the message ----

  // The message, as a service would send it: only what was said, in the
  // order the reference lists it.
  protected readonly message = computed(() => {
    const d = this.draft();
    const m: Record<string, unknown> = { tenant: this.opts()?.tenant ?? '' };
    if (d.roles.length) m['roles'] = d.roles;
    if (d.users.length) m['users'] = d.users;
    m['level'] = d.level;
    if (d.exercise) m['exercise'] = true;
    m['title'] = d.title.trim();
    if (d.body.trim()) m['body'] = d.body.trim();
    const detail = this.target(d.detail);
    if (detail) m['detail'] = detail;
    if (d.actions.length) {
      m['actions'] = d.actions.map((a) => {
        const out: Record<string, unknown> = { label: a.label.trim(), ...this.target(a.link) };
        if (a.method !== 'GET') {
          out['method'] = a.method;
          const body = parseBody(a.body);
          if (body.value !== undefined) out['body'] = body.value;
        }
        return out;
      });
    }
    if (d.ack !== 'none') {
      m['ack'] = d.ack;
      const onAck = this.target(d.onAck);
      if (onAck) m['onAck'] = onAck;
    }
    if (d.mail) m['mail'] = true;
    if (d.expires) m['expires'] = d.expires;
    if (d.id.trim()) m['id'] = d.id.trim();
    const meta: Record<string, string> = {};
    for (const row of d.meta) {
      if (row.key.trim()) meta[row.key.trim()] = row.value;
    }
    if (Object.keys(meta).length) m['meta'] = meta;
    return m;
  });

  protected readonly json = computed(() => JSON.stringify(this.message(), null, 2));

  // A title, and nothing the form already knows the server would refuse.
  protected readonly ready = computed(
    () => this.draft().title.trim() !== '' && this.draft().id.trim() !== '' && !this.f().invalid(),
  );

  // ---- what the server said ----

  protected errorOn(field: string): string {
    const r = this.refusal();
    return r && r.field === field ? r.message : '';
  }

  protected readonly audienceError = computed(
    () => this.errorOn('roles') || this.errorOn('users') || this.errorOn('audience'),
  );

  // A refusal no field of the form shows: said under the message.
  protected readonly loose = computed(() => {
    const r = this.refusal();
    if (!r) return '';
    const shown = ['title', 'body', 'level', 'exercise', 'roles', 'users', 'audience', 'detail', 'actions', 'ack', 'onAck', 'expires', 'id', 'mail'];
    return shown.includes(r.field) || /^actions\.\d+(\.(label|method|body))?$/.test(r.field) ? '' : r.message;
  });

  protected send(): void {
    if (!this.ready() || this.sending()) return;
    this.sending.set(true);
    this.api.publishTenantNotification(this.tenantId(), this.message()).subscribe({
      next: (p) => {
        this.sending.set(false);
        this.drawer.withdrawn.update((v) => v + 1);
        // The next one is another notification: another id.
        if (!p.existing) this.draft.update((d) => ({ ...d, id: crypto.randomUUID() }));
        this.snack.open(
          p.existing
            ? $localize`:@@Notify_sent_existing:Already sent with this id: nobody was told again`
            : p.recipients === 1
              ? $localize`:@@Notify_sent_one:Sent to 1 person`
              : $localize`:@@Notify_sent_many:Sent to ${p.recipients}:COUNT: people`,
          undefined,
          { duration: 3000 },
        );
      },
      error: (e: HttpErrorResponse) => {
        this.sending.set(false);
        const field = typeof e.error?.field === 'string' ? e.error.field : '';
        const message = typeof e.error?.error === 'string' ? e.error.error : $localize`:@@Request_failed:Request failed`;
        // The server's sentence, shown as one.
        this.refusal.set({ field, message: message.charAt(0).toUpperCase() + message.slice(1) });
        if (['expires', 'id', 'mail'].includes(field)) this.moreOpen.set(true);
        // Said where it can be fixed - on the form, whichever tab was on
        // screen - and brought into sight.
        if (!this.loose()) this.tab.set('edit');
        afterNextRender(
          () => this.host.nativeElement.querySelector('.refusal')?.scrollIntoView({ block: 'nearest', behavior: 'smooth' }),
          { injector: this.injector },
        );
      },
    });
  }
}
