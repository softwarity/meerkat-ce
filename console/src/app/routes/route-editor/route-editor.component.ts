import { HttpErrorResponse } from '@angular/common/http';
import { Component, ElementRef, computed, inject, input, linkedSignal, output, signal, viewChild } from '@angular/core';
import { humanIso, withCurrent } from '../../shared/iso-duration';
import { FormField, type ValidationError, form, required, validate } from '@angular/forms/signals';
import { MatButtonModule } from '@angular/material/button';
import { MatDialog } from '@angular/material/dialog';
import { MatCheckboxModule } from '@angular/material/checkbox';
import { MatDividerModule } from '@angular/material/divider';
import { MatFormFieldModule } from '@angular/material/form-field';
import { MatIconModule } from '@angular/material/icon';
import { MatMenuModule } from '@angular/material/menu';
import { MatInputModule } from '@angular/material/input';
import { MatListModule } from '@angular/material/list';
import { MatSelectModule } from '@angular/material/select';
import { MatTooltipModule } from '@angular/material/tooltip';
import { Router, RouterLink } from '@angular/router';
import { firstValueFrom } from 'rxjs';
import { LOCALE_ID } from '@angular/core';
import { Access, ApiService, CatalogEntry, DiscoveredService, Spec, IDENTITY_FIELDS, IdentityAttr, IdentityForward, LocaleMechanism, PAGE_USER_FIELDS, Role, Route, Tenant, User, USER_BUTTON_POSITIONS } from '../../api.service';
import { MaintenanceFilterComponent, RedirectFilterComponent, RespondFilterComponent } from '../filters/filter-fields.component';
import { runRoleExpr } from '../role-expr-dialog.component';
import { ROLE_SYNTAX } from '../template-highlight';
import { TplCodeComponent } from '../tpl-code.component';
import { MeService } from '../../me.service';
import { humanDuration } from '../../shared/duration';
import { EeLockComponent } from '../../shared/ee-lock.component';
import { FormFieldComponent } from '../../shared/form-field.component';
import { UrlInputComponent, UrlSuggestion } from '../../shared/url-input.component';
import { ACCESS_LEVELS, AccessEditorComponent, AccessState, emptyAccess, levelShort } from '../endpoint-security/access-editor.component';
import { FiltersComponent } from '../filters/filters.component';
import { IdentityPreviewData, IdentityPreviewDialogComponent } from '../identity-preview-dialog.component';
import { argStr, cleanPredicates, cleanSpecs, humanize } from '../predicates/args';
import { missingArgs, upstreamProblem } from './gaps';
import { PredicatesComponent } from '../predicates/predicates.component';

// What an empty language script opens on. A sentence describing an argument
// is read once and forgotten; a line of code that runs names it, shows its
// shape, and can be kept - and the commented reload says what this block is
// standing in for, so nobody saves the example and wonders why the page went
// quiet.
const LOCALE_SCRIPT_SEED = `// The language arrives in \`locale\` ("fr", "en-US"...).
// Called on every change, and once when a page loads.
// Apply it in place - do not reload.
myApp.setLocale(locale);
`;

// The locale mechanisms, each with what it looks like on the wire. The example
// is the explanation: "?lg=fr" says more than a sentence about query strings.
const LOCALE_MECHANISMS: { value: LocaleMechanism; label: string; example: string; ui?: boolean }[] = [
  {
    value: 'accept',
    label: $localize`:@@Mech_accept:Accept-Language only, and reload`,
    example: 'Accept-Language: fr, en;q=0.8',
  },
  {
    value: 'custom',
    label: $localize`:@@Mech_custom:Custom header, and reload`,
    example: 'X-Locale: fr',
  },
  {
    value: 'query',
    label: $localize`:@@Mech_query:Query parameter, and navigate`,
    example: '/orders?lg=fr',
  },
  {
    value: 'path',
    label: $localize`:@@Mech_path:Path segment, and navigate`,
    example: '/app/fr/orders',
    ui: true,
  },
  {
    value: 'script',
    label: $localize`:@@Mech_script:Script, without reload`,
    example: 'myApp.setLocale("fr")',
    ui: true,
  },
];

// The three code blocks a route can carry, and the draft field each edits.
// One map, so the dialog, the line count and the save all name the same field.
type CodeKind = 'css' | 'js' | 'onLocaleChange';
const CODE_FIELD: Record<CodeKind, 'customCss' | 'customJs' | 'localesOnChange'> = {
  css: 'customCss',
  js: 'customJs',
  onLocaleChange: 'localesOnChange',
};

type Section =
  | 'security'
  | 'predicates'
  | 'gates'
  | 'target'
  | 'modin'
  | 'modout'
  | 'identity'
  | 'button'
  | 'locales'
  | 'userinfo'
  | 'inject';

// Every section the drawer knows. A url naming anything else - an old
// bookmark on the General section that no longer exists - lands on Target
// rather than on an empty panel.
const SECTIONS: Section[] = [
  'security', 'predicates', 'gates', 'target', 'modin', 'modout',
  'identity', 'button', 'locales', 'userinfo', 'inject',
];

// Sections that only make sense for one route type - they show disabled (not
// hidden) when the other type is selected.
const UI_SECTIONS: Section[] = ['button', 'userinfo', 'inject'];

// Sections a route that answers BY ITSELF has no use for. This is not a
// tidying preference: CompileFilters drops every request filter when the route
// carries a terminal one ("incoming filters ignored: this route answers by
// itself, nothing is proxied") and identity forwarding compiles down to one.
// Editing them on a redirect wrote settings the gateway throws away without
// telling anyone but its log.
const PROXY_SECTIONS: Section[] = ['modin', 'identity'];

// What the drawer calls each section, for the list of what is missing: a gap
// has to say WHERE, and "predicates" is not what the reader sees on the left.
const SECTION_LABEL: Record<Section, string> = {
  security: $localize`:@@Section_security:Security`,
  predicates: $localize`:@@Predicates:Predicates`,
  gates: $localize`:@@Gates:Gates`,
  target: $localize`:@@Section_target:Target`,
  modin: $localize`:@@Incoming:Incoming`,
  modout: $localize`:@@Outgoing:Outgoing`,
  identity: $localize`:@@Section_identity:Identity`,
  button: $localize`:@@Section_user_button:User button`,
  locales: $localize`:@@Section_locales:Locales`,
  userinfo: $localize`:@@Section_user_info:User info`,
  inject: $localize`:@@Section_injections:Injections`,
};

// Where a brick's gap is edited, from the phase the catalogue gives it.
function sectionOfPhase(phase: string | undefined): Section {
  switch (phase) {
    case 'gate':
      return 'gates';
    case 'response':
      return 'modout';
    case 'terminal':
      return 'target';
    default:
      return 'modin';
  }
}

// One thing standing between this route and Save.
//
// The section travels in the error KIND rather than beside the message: a
// Signal Forms error carries a kind and a message and nothing else, and a
// filter's section is not known until its phase is read. Parsed back out in
// `gaps` below.
interface Gap {
  section: Section | '';
  message: string;
}

function gapError(section: Section | '', message: string): ValidationError {
  return { kind: section ? `gap:${section}` : 'gap', message };
}

// The blank lines a respond template carries for comfort in the editor are
// not content: trailing whitespace would ride into the answer an application
// receives, so it is cut on the way out.
// The static head a caller's path must carry to enter the route ("/api/**" ->
// "/api"), which is where the route's own documents live. The SERVER is the
// authority (routing.MatchPrefix); this mirrors it so the screen can say where
// a deposited file will answer before it is deposited.
function matchPrefix(predicates: Spec[]): string {
  for (const p of predicates) {
    if (p.type !== 'path') continue;
    const patterns = p.args?.['patterns'];
    const first = Array.isArray(patterns) ? String(patterns[0] ?? '') : '';
    const kept: string[] = [];
    for (const segment of first.split('/')) {
      if (!segment) continue;
      if (segment.includes('*') || segment.includes('{')) break;
      kept.push(segment);
    }
    return kept.length ? '/' + kept.join('/') : '';
  }
  return '';
}

function trimTemplates(specs: Spec[]): Spec[] {
  return specs.map((s) =>
    s.type === 'respond' && typeof s.args?.['body'] === 'string'
      ? { ...s, args: { ...s.args, body: (s.args['body'] as string).trimEnd() } }
      : s,
  );
}

// What a new respond template starts as: the identity answer most applications
// expect, laid out over several lines so the shape is readable. JSON ignores
// the whitespace between its tokens, so the indentation costs nothing.
const RESPOND_EXAMPLE = `{
  "name": {{json .Username}},
  "roles": {{json .Roles}}
}


`;

// Predicate types whose server contract requires a \`name\` arg.
const MATCHER_TYPES = ['header', 'cookie', 'query'];

// Identity-token lifetimes (short by design): the select offers these, humanized.
const IDENTITY_TTL_CHOICES = ['PT1M', 'PT2M', 'PT5M', 'PT10M', 'PT15M', 'PT30M', 'PT1H'];

// The route's base Access as the editor's non-optional shape.
function toAccessState(a: Access | undefined): AccessState {
  if (!a) return emptyAccess();
  const level = a.level ?? '';
  const roles = a.roles ?? [];
  // Normalised on the way IN: exceptions with nothing to except them from are
  // dropped, so a leftover list stops showing a rule that conditions nothing -
  // and stops being saved back the next time the route is touched.
  const users = level === '' && roles.length === 0 ? [] : (a.users ?? []);
  return { level, tenants: a.tenants ?? [], roles, users };
}

// The editable shape of one route: every field the drawer can change, with the
// defaults a fresh route starts on. A pure function of the stored route, so
// the same call seeds the draft and rebuilds the reference the draft is
// compared against to know whether anything was touched.
function draftOf(r: Route | null) {
  return {
    name: r?.name ?? '',
    upstream: r?.upstream ?? '',
    enabled: r?.enabled ?? true,
    access: toAccessState(r?.access),
    isUi: r?.isUi ?? false,
    predicates: r?.predicates ?? [],
    filters: r?.filters ?? [],
    // How long this upstream may take (ROUTE-07). Undefined is the
    // installation's defaults, and stays undefined rather than becoming a pair
    // of empty strings - a route back on the defaults stores nothing.
    timeouts: r?.timeouts as { connect?: string; response?: string } | undefined,
    // ROUTE-09. Off unless somebody turned it on: a breaker turns one kind of
    // failure into another, so it is met because it was chosen.
    breakerOn: r?.breaker?.enabled ?? false,
    breakerTrip: String(r?.breaker?.trip ?? ''),
    breakerCool: r?.breaker?.cool ?? '',
    // API section (SVC-06): the source, then what it needs. specPath is the
    // upstream url for one, the served file name for the other - the field
    // means "where the spec is, relative to this route" in both cases.
    specSource: r?.api?.spec?.type ?? '',
    specPath: r?.api?.spec?.path ?? '',
    specFilename: r?.api?.spec?.filename ?? '',
    // UI section
    schemeSelect: r?.ui?.scheme?.select ?? false,
    schemeMechanism: r?.ui?.scheme?.mechanism ?? '',
    schemeAttribute: r?.ui?.scheme?.attribute ?? '',
    schemeLight: r?.ui?.scheme?.light ?? '',
    schemeDark: r?.ui?.scheme?.dark ?? '',
    rolesEnabled: r?.ui?.roles?.enabled ?? false,
    // ONE mode: an attribute on a tag, classes on a tag, or a meta tag.
    rolesMode: (r?.ui?.roles?.mechanism || 'class') as 'class' | 'attribute' | 'meta',
    rolesTag: r?.ui?.roles?.tag || 'body',
    rolesAttribute:
      r?.ui?.roles?.attribute || ((r?.ui?.roles?.mechanism ?? 'class') === 'meta' ? 'meerkat-roles' : 'data-roles'),
    userInfoEnabled: r?.ui?.userInfo?.enabled ?? false,
    userInfoMode: (r?.ui?.userInfo?.mechanism || 'attribute') as 'attribute' | 'meta',
    userInfoTag: r?.ui?.userInfo?.tag || 'body',
    // One row per stampable fact, ALL selected by default on a fresh route;
    // the name defaults to the field itself (username stamps as username).
    userInfoFields: Object.fromEntries(
      PAGE_USER_FIELDS.map((f) => {
        const stored = r?.ui?.userInfo?.fields;
        const enabled = stored ? f in stored : true;
        return [f, { enabled, name: stored?.[f] || f }];
      }),
    ) as Record<string, { enabled: boolean; name: string }>,
    btnEnabled: r?.ui?.userButton?.enabled ?? false,
    btnHeight: r?.ui?.userButton?.height ?? 24,
    btnPosition: r?.ui?.userButton?.position ?? 'top-right',
    btnShape: r?.ui?.userButton?.shape || 'round',
    btnName: r?.ui?.userButton?.name ?? '',
    btnPadX: r?.ui?.userButton?.padX ?? 12,
    btnPadY: r?.ui?.userButton?.padY ?? 12,
    btnInFrame: r?.ui?.userButton?.inFrame ?? false,
    localeMechanism: (r?.locales?.mechanism || 'accept') as LocaleMechanism,
    localesDisabled: r?.locales?.disabled ?? [],
    localesHeader: r?.locales?.header ?? '',
    localesParam: r?.locales?.param ?? '',
    localesOnChange: r?.locales?.onChange ?? '',
    customCss: r?.ui?.customCss ?? '',
    customJs: r?.ui?.customJs ?? '',
    // The app's menu label: when set, the route shows in the user's apps menu
    // (subject to access). Empty = the app is reachable but not listed.
    uiLink: r?.ui?.link ?? '',
    identityMechanism: r?.identity?.mechanism ?? '',
    identityTtl: r?.identity?.ttl || 'PT2M',
    identityAlgorithm: r?.identity?.algorithm || 'ES256',
    // One row per forwardable fact. On a route that already forwards, a fact is
    // selected when it appears in attributes (with its stored mapping); on a
    // fresh route everything is selected by default (nothing hidden from the
    // upstream unless the admin opts a fact out).
    identityAttrs: Object.fromEntries(
      IDENTITY_FIELDS.map((f) => {
        const stored = r?.identity?.attributes;
        const found = stored?.find((a) => a.field === f);
        const selected = stored ? !!found : true;
        // The mapping input starts on the attribute's own name (the default a
        // fact travels under); the admin overwrites it to rename. buildRoute
        // drops it again when it still equals the field name.
        return [f, { selected, as: found?.as || f, expr: found?.expr ?? '' }];
      }),
    ) as Record<string, { selected: boolean; as: string; expr: string }>,
  };
}

// Route editor - a side-drawer inspector (not a modal). Sections down the left,
// one panel each. One signal form over the whole draft: scalars bind field by
// field, predicates/filters bind as FormValueControl sections. The schema
// mirrors the server's required predicate args so Save disables before the API
// would 422. Editable state is a linkedSignal off the `route` input so
// re-opening the drawer on another route reseeds it. Saving PUTs to the admin
// API (validates by compiling - its 422 is surfaced verbatim) then emits `saved`.
@Component({
  selector: 'app-route-editor',
  imports: [
    FormField,
    MatButtonModule,
    MatCheckboxModule,
    MatDividerModule,
    MatFormFieldModule,
    MatIconModule,
    MatInputModule,
    MatListModule,
    MatMenuModule,
    MatSelectModule,
    MatTooltipModule,
    RouterLink,
    PredicatesComponent,
    FiltersComponent,
    MaintenanceFilterComponent,
    RedirectFilterComponent,
    RespondFilterComponent,
    UrlInputComponent,
    FormFieldComponent,
    EeLockComponent,
    AccessEditorComponent,
    TplCodeComponent,
  ],
  // Opts this drawer into showing a required-and-empty field in the error
  // colour without waiting to be touched: creating a route is precisely the
  // moment "what is still missing" has to be readable at a glance.
  host: { class: 'mk-show-missing' },
  templateUrl: './route-editor.component.html',
  styleUrl: './route-editor.component.scss',
})
export class RouteEditorComponent {
  readonly route = input<Route | null>(null);
  readonly catalog = input.required<CatalogEntry[]>();
  // The other routes, for what can only be answered by looking next door: the
  // role expression a sibling already sends.
  readonly siblings = input<Route[]>([]);
  // The URL owns the active section: it seeds this input, and local picks are
  // emitted back so the page can navigate (F5-proof deep links).
  readonly initialSection = input<string>('target');
  readonly sectionChange = output<string>();
  readonly saved = output<Route>();
  readonly closed = output<void>();

  private readonly api = inject(ApiService);
  private readonly dialog = inject(MatDialog);
  private readonly router = inject(Router);

  protected readonly filterEntries = () => this.catalog().filter((e) => e.kind === 'filter');
  protected readonly predicateEntries = () => this.catalog().filter((e) => e.kind === 'predicate');

  // Per-phase counters for the Modifiers nav entries.
  // What the Security entry shows beside its name: the level in short, plus
  // the count of what narrows it. Empty when nothing is posed - an unguarded
  // route says so by saying nothing, like every other section here.
  // The rule at a glance, in the LIST's own words - AUTH, ORG, DENY - rather
  // than the dot that stood here: a dot says "something is posed" and leaves
  // "signed in", "in an organisation" and "closed to everyone" looking
  // identical, one click away from a table that tells them apart. The two
  // lists stay in the tooltip, where a count belongs when there is no room to
  // draw its icon.
  protected securityMark(): { text: string; tip: string } | null {
    const a = this.draft().access ?? emptyAccess();
    const bits: string[] = [];
    // isEmpty is the same judgement the gateway makes, so the mark cannot
    // claim a rule the engine ignores.
    if (a.roles?.length) bits.push($localize`:@@N_roles:${a.roles.length}:COUNT: roles`);
    if (a.users?.length) bits.push($localize`:@@N_users:${a.users.length}:COUNT: users`);
    const endpoints = this.route()?.api?.security?.endpoints?.length ?? 0;
    if (endpoints) bits.push($localize`:@@N_endpoints:${endpoints}:COUNT: endpoints`);
    const level = levelShort(a);
    const delegated = level === '\u2014';
    if (delegated && !bits.length) return null;
    const label = ACCESS_LEVELS.find((l) => l.value === (a.level ?? ''))?.label ?? '';
    return { text: delegated ? '\u25cf' : level, tip: bits.length ? label + ' - ' + bits.join(' \u00b7 ') : label };
  }

  // Sections whose whole panel is a palette drawer (see the template): they
  // take the padding off the panel and put it on their own content.
  protected readonly hasPalette = computed(() =>
    ['predicates', 'gates', 'modin', 'modout'].includes(this.section()),
  );

  protected countPhase(phase: string): number {
    const phaseOf = (t: string) => this.catalog().find((e) => e.type === t)?.phase ?? 'request';
    return this.draft().filters.filter((s) => phaseOf(s.type) === phase).length;
  }

  // How this route answers. The mode is DERIVED, never stored: a terminal
  // filter in the list is the mode (its type), none is "proxy". So switching
  // mode is adding or dropping that one filter, the model does not gain a
  // field, and an exported configuration reads exactly as before.
  protected readonly terminalTypes = computed(
    () => new Set(this.catalog().filter((e) => e.phase === 'terminal').map((e) => e.type)),
  );
  protected readonly terminalSpec = computed(
    () => this.draft().filters.find((f) => this.terminalTypes().has(f.type)) ?? null,
  );
  protected readonly mode = computed(() => this.terminalSpec()?.type ?? 'proxy');
  protected readonly hasTerminalFilter = () => this.terminalSpec() !== null;
  protected setMode(m: string): void {
    this.draft.update((d) => {
      const rest = d.filters.filter((f) => !this.terminalTypes().has(f.type));
      // The upstream is KEPT when leaving proxy: switching back must not cost
      // the address someone typed.
      if (m === 'proxy') return { ...d, filters: rest };
      // respond opens on application/json, so it cannot carry pages either
      // until someone says otherwise.
      const ui = m === 'proxy' || m === 'maintenance' ? {} : { isUi: false };
      // The server's defaults, written down rather than left to a placeholder:
      // a field showing "application/json" in grey and holding nothing is a
      // field nobody can tell is empty. The template starts as a WORKING
      // example on several lines - for a syntax nobody guesses, something to
      // edit beats an empty box with instructions beside it.
      const args = m === 'respond' ? { contentType: 'application/json', status: 200, body: RESPOND_EXAMPLE } : {};
      return { ...d, ...ui, filters: [...rest, { type: m, args }] };
    });
  }

  protected patchTerminal(spec: Spec): void {
    this.draft.update((d) => ({
      ...d,
      filters: d.filters.map((f) => (this.terminalTypes().has(f.type) ? spec : f)),
    }));
  }

  // The active section: a NEW url value wins, the UI toggle kicks disabled
  // sections back to Target, local picks flow through onSectionPick.
  protected readonly section = linkedSignal<{ ui: boolean; proxy: boolean; ini: string }, Section>({
    source: () => ({ ui: this.draft().isUi, proxy: this.mode() === 'proxy', ini: this.initialSection() }),
    computation: (src, previous) => {
      let s = (previous && previous.source.ini === src.ini ? previous.value : src.ini) as Section;
      if (!SECTIONS.includes(s)) s = 'target';
      if (!src.ui && UI_SECTIONS.includes(s)) s = 'target';
      // Same reason as the UI ones: switching to a mode that answers by itself
      // must not leave the reader editing a section the gateway will ignore.
      if (!src.proxy && PROXY_SECTIONS.includes(s)) s = 'target';
      return s;
    },
  });

  // Said on the sections a terminal route silences, so the reason is where the
  // question is rather than in a log nobody reads.
  protected readonly proxies = computed(() => this.mode() === 'proxy');
  protected readonly notProxiedTip = $localize`:@@Not_proxied_tip:This route answers by itself, so nothing is sent upstream: the gateway drops incoming filters and forwards no identity.`;

  protected onSectionPick(s: Section): void {
    this.section.set(s);
    this.sectionChange.emit(s);
  }

  protected readonly positions = USER_BUTTON_POSITIONS;

  // Editable copy, reseeded whenever the drawer is (re)opened on another route.
  protected readonly draft = linkedSignal(() => draftOf(this.route()));

  // Whether anything was touched. Compared against a draft rebuilt from the
  // stored route by the SAME function, so the two objects carry their keys in
  // the same order and a stringify is a fair test. This is what tells Save it
  // has work to do, and what stops a stray click on the backdrop from throwing
  // the work away.
  readonly dirty = computed(() => JSON.stringify(this.draft()) !== JSON.stringify(draftOf(this.route())));

  // The APPLICATION's locale offer, shown read-only (managed in Application
  // General). Display names come from Intl: the console's locale for the
  // reader, the code's own locale for what the button menu shows (endonym).
  protected readonly identityFields = IDENTITY_FIELDS;
  // In single-organisation mode the tenant is implicit and never named, so
  // stamping it on a page says nothing: one value, always the same.
  // The two facts that only mean something when there is more than one
  // organisation. On a single-organisation instance they are SHOWN and frozen,
  // not hidden: hiding them left them ticked, so they kept travelling while
  // the screen said nothing about them.
  protected frozen(field: string): boolean {
    return !this.me.multiTenant() && (field === 'tenant' || field === 'tenantid');
  }
  // Why it is frozen, and where that is decided. Two different sentences: on a
  // licensed instance the mode is simply set to single, and blaming Enterprise
  // would be a lie; on a community one, the mode is what the licence covers.
  protected readonly frozenTip = computed(() =>
    this.me.enterprise()
      ? $localize`:@@Frozen_single_mode:This instance runs in single-organisation mode: the value would be the same on every request. The mode is in Application, General.`
      : $localize`:@@Frozen_single_mode_ce:This instance serves one organisation, so the value would be the same on every request. Several organisations is an Enterprise feature - see Application, General.`,
  );
  protected readonly pageUserFields = PAGE_USER_FIELDS;
  // The scheme the built-in pages impose, if any: offering a switch on the
  // application's pages while its own sign-in page has none would promise
  // something the gateway will not honour.
  protected readonly imposedScheme = signal<'' | 'light' | 'dark'>('');
  protected readonly appLanguages = signal<string[]>([]);
  // Roles, users and organisations feed the Security section's access editor.
  // All three are app-scoped, so a pure infra-admin may get empty lists
  // (tolerated: the rule can still be set to a level that names nothing).
  protected readonly roles = signal<Role[]>([]);
  protected readonly users = signal<User[]>([]);
  protected readonly tenants = signal<Tenant[]>([]);
  private readonly consoleNames = new Intl.DisplayNames([inject(LOCALE_ID)], { type: 'language' });

  constructor() {
    // What the runtime can route to. Silent on failure: an installation with
    // no runtime to ask still types an upstream by hand, and a red box in
    // front of a working field would be the wrong trade.
    this.api.services().subscribe({
      next: (d) => {
        this.discovered.set(d.services ?? []);
        this.reach.set(d.reach ?? []);
      },
      error: () => {},
    });
    // What the installation waits, for the "inherited" entries. Read once and
    // failing silently: a label is not worth an error box in front of a route.
    this.api.proxyLimits().subscribe({
      next: (l) => this.houseTimeouts.set(l.timeouts ?? {}),
      error: () => {},
    });
    this.api.settings().subscribe({
      next: (s) => {
        this.appLanguages.set(s.languages ?? []);
        this.imposedScheme.set(s.pagesScheme ?? '');
      },
    });
    this.api.listRoles().subscribe({ next: (r) => this.roles.set(r) });
    this.api.listUsers().subscribe({ next: (u) => this.users.set(u), error: () => this.users.set([]) });
    this.api.listTenants().subscribe({ next: (t) => this.tenants.set(t), error: () => this.tenants.set([]) });
  }

  protected setAccess(a: AccessState): void {
    this.draft.update((d) => ({ ...d, access: a }));
  }

  private readonly ttlLocale = inject(LOCALE_ID);
  // The token-TTL choices, plus the stored value if it is off the preset list.
  protected readonly ttlChoices = computed(() => {
    const cur = this.draft().identityTtl;
    return cur && !IDENTITY_TTL_CHOICES.includes(cur) ? [cur, ...IDENTITY_TTL_CHOICES] : IDENTITY_TTL_CHOICES;
  });
  protected humanTtl(iso: string): string {
    return humanDuration(iso, this.ttlLocale);
  }

  protected localeName(code: string): string {
    try {
      const n = this.consoleNames.of(code) ?? code;
      return n.charAt(0).toUpperCase() + n.slice(1);
    } catch {
      return code;
    }
  }

  protected localeNative(code: string): string {
    try {
      const n = new Intl.DisplayNames([code], { type: 'language' }).of(code) ?? code;
      return n.charAt(0).toUpperCase() + n.slice(1);
    } catch {
      return code;
    }
  }

  private readonly me = inject(MeService);

  protected setMechanism(v: string): void {
    this.draft.update((d) => ({ ...d, identityMechanism: v as '' | 'headers' | 'jwt' | 'signed-jwt' }));
  }

  // The roles format button states: what the value looks like right now.
  protected readonly stringTip = $localize`:@@Roles_as_string_tip:Sent as a comma-separated string (click for a JSON array)`;
  protected readonly jsonTip = $localize`:@@Roles_as_json_tip:Sent as a JSON array (click for a comma-separated string)`;

  protected toggleAttr(field: string, selected: boolean): void {
    this.draft.update((d) => ({
      ...d,
      identityAttrs: { ...d.identityAttrs, [field]: { ...d.identityAttrs[field], selected } },
    }));
  }
  protected setAttrAs(field: string, as: string): void {
    this.draft.update((d) => ({
      ...d,
      identityAttrs: { ...d.identityAttrs, [field]: { ...d.identityAttrs[field], as } },
    }));
  }
  // A catalogue mirroring another system carries a common head on every role.
  // Dropping it saves it on every request - 320 roles is 1600 bytes of the
  // same five characters - and it is opt-in: a service testing for ROLE_ADMIN
  // has to keep receiving it.
  // Why the Link field is greyed, said where a greyed field can still speak.
  protected readonly linkHint = computed(() =>
    this.draft().isUi
      ? $localize`:@@Link_hint:Empty: reachable, but not listed - the portal's own case.`
      : $localize`:@@Link_needs_ui_hint:Needs the UI option, ticked in the list on the left.`,
  );

  protected readonly defaultRoleExpr = '{{join "," .Roles}}';
  // The expression under the row is coloured like the editor colours it.
  protected readonly SYNTAX = ROLE_SYNTAX;


  // What the other routes send, for the dialog to offer as a starting point.
  private otherExprs(): { name: string; expr: string }[] {
    const mine = this.route()?.name;
    return this.siblings()
      .filter((r) => r.name !== mine)
      .map((r) => ({
        name: r.name,
        expr: r.identity?.attributes?.find((a) => a.field === 'roles')?.expr?.trim() ?? '',
      }))
      .filter((o) => o.expr);
  }

  protected openRoleExpr(): void {
    void import('../role-expr-dialog.component').then(({ RoleExprDialogComponent }) => {
      this.dialog
        .open(RoleExprDialogComponent, {
          data: { expr: this.draft().identityAttrs['roles'].expr, roles: this.roles(), others: this.otherExprs() },
          width: '1120px',
          maxWidth: '92vw',
        })
        .afterClosed()
        .subscribe((expr?: string) => {
          if (expr === undefined) return;
          this.draft.update((d) => ({
            ...d,
            identityAttrs: { ...d.identityAttrs, roles: { ...d.identityAttrs['roles'], expr: expr.trim() } },
          }));
        });
    });
  }


  // A live example of the forwarded value, from the editor's OWN session. The
  // facts the console cannot see here (tenant, tenantid, timezone, roles) show
  // an illustrative placeholder - the mapping and the list format are what the
  // preview is really about.
  protected mappedExample(field: string, _asJson = false): string {
    if (field === 'roles') {
      // Two stand-in roles run through the SHAPE, not through the filter: a
      // stand-in carries none of this estate's tags, so keeping the filter
      // emptied the preview the moment one was written - which is when the
      // format is worth showing.
      const shape = (this.draft().identityAttrs['roles'].expr || this.defaultRoleExpr)
        .replace(/\|\s*\.(Keep|Drop)(\s+"[^"]*")*/g, '');
      try {
        return runRoleExpr(shape, ['ROLE_ADMIN', 'ROLE_OPS'], new Map()).text;
      } catch {
        return '';
      }
    }
    const u = this.me.user();
    switch (field) {
      case 'username':
        return u?.username || 'admin';
      case 'userid':
        return u?.id || 'usr_123';
      case 'fullname':
        return u?.fullname || 'Ada Lovelace';
      case 'email':
        return u?.email || 'admin@example.com';
      case 'tenant':
        return 'acme';
      case 'tenantid':
        return 'tnt_123';
      case 'timezone':
        return 'Europe/Paris';
    }
    return '';
  }
  // Everything that has to be true before the gateway will take this route.
  //
  // One schema, and the drawer reads it three ways: Save is disabled, the left
  // list marks the sections at fault, and the footer lists them. The rules
  // themselves mirror the server's - a required argument the catalogue names,
  // an upstream gateway.Validate would parse - so the console refuses what the
  // gateway refuses instead of discovering it in a 422.
  protected readonly f = form(this.draft, (p) => {
    required(p.name, {
      error: gapError('', $localize`:@@Gap_name:a name`),
    });
    validate(p.upstream, ({ value }) => {
      if (this.mode() !== 'proxy') return [];
      const problem = upstreamProblem(value());
      return problem ? [gapError('target', problem)] : [];
    });
    validate(p.predicates, ({ value }) => {
      // On what will be SENT, not on the draft. A predicate row someone has
      // just added and not typed into yet is dropped on the way out
      // (cleanPredicates), so flagging it would accuse the form of a fault the
      // payload does not have - and it would do it the instant "add" is
      // clicked, before anyone could have filled anything.
      const specs = cleanPredicates(value());
      const errors: ValidationError[] = [];
      // A route with no predicate accepts every request - which is a real
      // thing to want, and is written "/**": the two match identically, down
      // to "/" and "//". Written down it shows in the routes table and reads
      // as a decision; left empty it is indistinguishable from an oversight,
      // and it silences every route below it.
      if (!specs.length) {
        errors.push(
          gapError(
            'predicates',
            $localize`:@@Gap_no_predicate:at least one - a path of "/**" if this route is meant to catch everything`,
          ),
        );
      }
      if (specs.some((s) => MATCHER_TYPES.includes(s.type) && !argStr(s, 'name') && argStr(s, 'regexp'))) {
        errors.push(
          gapError(
            'predicates',
            $localize`:@@A_header_cookie_or_query_matcher_needs_a_name:a header, cookie or query matcher needs a name`,
          ),
        );
      }
      if (specs.some((s) => s.type === 'weight' && !argStr(s, 'group') !== !argStr(s, 'weight'))) {
        errors.push(
          gapError(
            'predicates',
            $localize`:@@Weight_needs_both_a_group_and_a_weight:weight needs both a group and a weight`,
          ),
        );
      }
      return [...errors, ...this.brickGaps(specs, () => 'predicates')];
    });
    // Gates, incoming, outgoing AND the terminal live in one list, so one rule
    // covers four sections: the phase says which one to send the reader to.
    // A filter is kept whatever it holds - cleanSpecs only empties it - so one
    // added and left blank IS what gets sent, and IS what the gateway will
    // refuse. Flagged from the moment it is posed, therefore.
    validate(p.filters, ({ value }) =>
      this.brickGaps(cleanSpecs(value()), (e) => sectionOfPhase(e?.phase)),
    );
  });

  // A brick missing a required argument, named the way the palette names it.
  private brickGaps(
    specs: Spec[],
    where: (entry: CatalogEntry | undefined) => Section,
  ): ValidationError[] {
    const out: ValidationError[] = [];
    for (const s of specs) {
      const entry = this.catalog().find((e) => e.type === s.type);
      const missing = missingArgs(s, entry);
      if (missing.length) {
        out.push(gapError(where(entry), `${humanize(s.type)}: ${missing.join(', ')}`));
      }
    }
    return out;
  }

  // What is missing, in the order the schema declares it. Read by the footer
  // list and, grouped, by the marks in the section list.
  protected readonly gaps = computed<Gap[]>(() =>
    this.f()
      .errorSummary()
      .map((e) => ({
        section: (e.kind.startsWith('gap:') ? e.kind.slice(4) : '') as Section | '',
        message: e.message ?? '',
      })),
  );
  private readonly gapsBySection = computed(() => {
    const by = new Map<string, string[]>();
    for (const g of this.gaps()) {
      by.set(g.section, [...(by.get(g.section) ?? []), g.message]);
    }
    return by;
  });
  // A tooltip rather than a count: one mark says a section needs attention,
  // and hovering says what - which is the whole question.
  protected gapTip(s: Section): string {
    return (this.gapsBySection().get(s) ?? []).join('\n');
  }
  protected hasGap(s: Section): boolean {
    return this.gapsBySection().has(s);
  }
  protected sectionLabel(s: Section | ''): string {
    return s ? SECTION_LABEL[s] : '';
  }
  // The name is not a section: it lives in the header, above the list.
  private readonly nameInput = viewChild<ElementRef<HTMLInputElement>>('nameInput');
  protected gapWhere(g: Gap): string {
    return g.section ? SECTION_LABEL[g.section] : $localize`:@@Name:Name`;
  }
  protected goToGap(g: Gap): void {
    if (g.section) {
      this.onSectionPick(g.section);
      return;
    }
    this.nameInput()?.nativeElement.focus();
  }

  protected readonly error = signal('');
  protected readonly saving = signal(false);

  protected setFlag(
    flag:
      | 'enabled'
      | 'schemeSelect'
      | 'rolesEnabled'
      | 'userInfoEnabled'
      | 'btnEnabled'
      | 'btnInFrame',
    value: boolean,
  ): void {
    this.draft.update((d) => ({ ...d, [flag]: value }));
  }

  // The UI toggle: a route is always a service, UI options come on top. The
  // path locale mechanism is a UI privilege and drops with the toggle.
  // What UI does is rewrite an HTML document, and the gateway only rewrites one
  // when the answer SAYS it is HTML (filters.isHTML, on text/html). So the
  // question is never the mode alone: a redirect has no body at all, and a
  // respond has whatever content type it declares - JSON on almost every one.
  protected readonly uiPossible = computed(() => {
    if (this.mode() === 'redirect') return false;
    if (this.mode() !== 'respond') return true;
    const ct = String(this.terminalSpec()?.args?.['contentType'] ?? '');
    return ct.trim().toLowerCase().startsWith('text/html');
  });
  protected readonly uiTip = $localize`:@@UI_serves_pages_in_a_browser:UI: serves pages in a browser`;
  protected readonly uiImpossibleTip = computed(() =>
    this.mode() === 'redirect'
      ? $localize`:@@UI_not_for_a_redirect:A redirect answers with a Location and no page: there is nothing to inject into.`
      : $localize`:@@UI_needs_html:The gateway only rewrites an answer that says it is HTML. Set the content type to text/html for this route to carry page injections.`,
  );

  protected setIsUi(value: boolean): void {
    this.draft.update((d) => ({
      ...d,
      isUi: value,
      // The two that only mean something on a page go back to the floor when
      // the route stops being a UI.
      localeMechanism: value || !['path', 'script'].includes(d.localeMechanism) ? d.localeMechanism : 'accept',
    }));
  }

  // The mechanisms this route may pick: two of them only mean something on a
  // page, so a service route is not offered them.
  protected readonly mechanisms = computed(() =>
    LOCALE_MECHANISMS.filter((m) => !m.ui || this.draft().isUi),
  );

  protected mechanismLabel(): string {
    return LOCALE_MECHANISMS.find((m) => m.value === this.draft().localeMechanism)?.label ?? '';
  }

  protected setLocalesMechanisms(value: LocaleMechanism): void {
    this.draft.update((d) => ({ ...d, localeMechanism: value }));
  }

  protected hasMech(m: string): boolean {
    return this.draft().localeMechanism === m;
  }

  // A route may exclude application locales its UI does not support.
  protected isLocaleDisabled(code: string): boolean {
    return this.draft().localesDisabled.some((c) => c.toLowerCase() === code.toLowerCase());
  }

  protected toggleLocale(code: string, enabled: boolean): void {
    this.draft.update((d) => ({
      ...d,
      localesDisabled: enabled
        ? d.localesDisabled.filter((c) => c.toLowerCase() !== code.toLowerCase())
        : [...d.localesDisabled, code],
    }));
  }

  protected setUserField(field: string, enabled: boolean): void {
    this.draft.update((d) => ({
      ...d,
      userInfoFields: {
        ...d.userInfoFields,
        [field]: { enabled, name: d.userInfoFields[field].name || field },
      },
    }));
  }

  protected setUserFieldName(field: string, name: string): void {
    this.draft.update((d) => ({
      ...d,
      userInfoFields: { ...d.userInfoFields, [field]: { ...d.userInfoFields[field], name } },
    }));
  }

  // Switching the roles mode retargets a name still in its default form.
  protected setRolesMode(value: string): void {
    this.draft.update((d) => {
      const oldDef = d.rolesMode === 'meta' ? 'meerkat-roles' : 'data-roles';
      const newDef = value === 'meta' ? 'meerkat-roles' : 'data-roles';
      return {
        ...d,
        rolesMode: value as 'class' | 'attribute' | 'meta',
        rolesAttribute: !d.rolesAttribute || d.rolesAttribute === oldDef ? newDef : d.rolesAttribute,
      };
    });
  }

  // How long this upstream may take (ROUTE-07). Offered rather than typed, for
  // the reason every other duration in this product is: a list is read at a
  // glance and cannot be spelled wrong. The empty entry is the installation's
  // default, which is what every route ran with when the bounds were global.
  // What the runtime says exists beside this gateway (SVC-02). One entry per
  // service AND PORT: a service exposing three is three choices, and offering
  // it once would mean guessing which - the guess being exactly the mistake
  // this list exists to remove.
  private readonly discovered = signal<DiscoveredService[]>([]);
  // Said under the field, because a list that only appears on focus needs to
  // announce itself once. Short and right-aligned, so it clears the floating
  // label of the row underneath. Empty when there is no runtime to ask: the
  // hint row is reserved either way, so the silence costs nothing.
  //
  // It says WHICH list when that matters. A gateway that could not find its
  // own networks is offering the cluster's inventory rather than what it can
  // reach - true of every developer instance running outside the cluster, and
  // exactly the moment somebody would trust a name that resolves nowhere.
  protected readonly upstreamHint = computed(() => {
    const n = this.upstreamChoices().length;
    if (!n) return '';
    if (!this.reach().length) return $localize`:@@Upstream_all:${n}:COUNT: in the cluster`;
    const out = this.discovered().filter((s) => s.reachable === false).length;
    return out
      ? $localize`:@@Upstream_some_out:${n}:COUNT: offered, ${out}:OUT: off your network`
      : $localize`:@@Upstream_known:${n}:COUNT: services offered`;
  });
  // Reachable first: an upstream this gateway shares no network with is still
  // shown - it may be one network away from working - but it does not sit
  // above the ones that answer today.
  protected readonly upstreamChoices = computed<UrlSuggestion[]>(() =>
    [...this.discovered()]
      .sort((a, b) => Number(b.reachable !== false) - Number(a.reachable !== false))
      .flatMap((s) => {
      // The name that RESOLVES, not the identity: a stack service answers to
      // its short alias, and that is what an operator writes. The full name
      // rides along in the hint, so the identity is never hidden.
      const host = s.names?.[0] || s.name;
      // The declared state, beside the name, ALWAYS - zero of zero is a
      // service somebody stopped, and hiding it would make a deliberate
      // decision look like a missing answer. Pointing a route at one is a
      // choice; pointing at one by accident is not.
      const state = `${s.ready}/${s.wanted}`;
      // Out of reach: the network is named, because that is the answer to
      // "why can I not use this one" - and the answer is never the stack.
      const out = s.reachable === false;
      const parts = [host === s.name ? '' : s.name, out ? (s.networks ?? []).join(' ') : '', state];
      return (s.ports ?? [])
        .filter((p) => p.target > 0)
        .map((p) => ({
          // The HOST only: the scheme is the field's own select, so picking a
          // service keeps the scheme already chosen instead of imposing one.
          value: `${host}:${p.target}`,
          hint: parts.filter(Boolean).join('  '),
          muted: out,
        }));
      }),
  );

  private readonly reach = signal<string[]>([]);

  // What the installation waits, so the "inherited" entry names the real
  // number rather than a constant that may not be in force. A label that says
  // "Default (5 s)" on an installation running 30 is worse than no label.
  private readonly houseTimeouts = signal<{ connect?: string; response?: string }>({});
  protected readonly inheritedConnect = computed(() =>
    labelFor(this.houseTimeouts().connect, '5 s'),
  );
  protected readonly inheritedResponse = computed(() =>
    labelFor(this.houseTimeouts().response, '15 s'),
  );

  protected readonly connectOffered = computed(() =>
    withCurrent(this.connectChoices, this.timeout('connect')),
  );
  protected readonly responseOffered = computed(() =>
    withCurrent(this.responseChoices, this.timeout('response')),
  );
  private readonly connectChoices = [
    { key: '', label: '' },
    { key: 'PT2S', label: '2 s' },
    { key: 'PT5S', label: '5 s' },
    { key: 'PT10S', label: '10 s' },
    { key: 'PT30S', label: '30 s' },
  ];
  private readonly responseChoices = [
    { key: '', label: '' },
    { key: 'PT5S', label: '5 s' },
    { key: 'PT15S', label: '15 s' },
    { key: 'PT30S', label: '30 s' },
    { key: 'PT1M', label: '1 min' },
    { key: 'PT5M', label: '5 min' },
    { key: 'PT10M', label: '10 min' },
  ];

  // An empty first entry, as the cool-down has: a route that never chose one
  // carries no number, and a select with nothing in it reads as a field the
  // screen forgot to fill rather than as the default it is.
  protected readonly tripChoices = [
    { key: '', label: $localize`:@@Default_5_trip:Default (5)` },
    { key: '3', label: '3' },
    { key: '5', label: '5' },
    { key: '10', label: '10' },
    { key: '20', label: '20' },
  ];
  protected readonly coolChoices = [
    { key: '', label: $localize`:@@Default_15s_cool:Default (15 s)` },
    { key: 'PT5S', label: '5 s' },
    { key: 'PT15S', label: '15 s' },
    { key: 'PT30S', label: '30 s' },
    { key: 'PT1M', label: '1 min' },
    { key: 'PT5M', label: '5 min' },
  ];

  protected timeout(which: 'connect' | 'response'): string {
    return this.draft().timeouts?.[which] ?? '';
  }

  // Both bounds live in one object, and an object with nothing in it is no
  // object at all: a route back on the defaults stores nothing rather than a
  // pair of empty strings.
  protected setTimeout(which: 'connect' | 'response', value: string): void {
    const next: { connect?: string; response?: string } = {
      ...(this.draft().timeouts ?? {}),
      [which]: value || undefined,
    };
    const empty = !next.connect && !next.response;
    this.draft.update((d) => ({ ...d, timeouts: empty ? undefined : next }));
  }

  protected patch(
    key:
      | 'specPath'
      | 'uiLink'
      | 'upstream'
      | 'identityTtl'
      | 'identityAlgorithm'
      | 'btnPosition'
      | 'btnShape'
      | 'btnName'
      | 'schemeMechanism'
      | 'schemeAttribute'
      | 'schemeLight'
      | 'schemeDark'
      | 'rolesTag'
      | 'rolesAttribute'
      | 'userInfoMode'
      | 'userInfoTag'
      | 'localesHeader'
      | 'localesParam'
      | 'breakerTrip'
      | 'breakerCool',
    value: string,
  ): void {
    this.draft.update((d) => ({ ...d, [key]: value }));
  }

  // Its own setter because the flag is a boolean and patch takes strings -
  // the shape the rest of this editor already uses for a checkbox.
  protected setBreaker(on: boolean): void {
    this.draft.update((d) => ({ ...d, breakerOn: on }));
  }

  protected setBtnHeight(value: string): void {
    this.draft.update((d) => ({ ...d, btnHeight: Math.max(16, Math.min(96, parseInt(value, 10) || 24)) }));
  }

  // Per-corner gaps: X from the side edge, Y from the top/bottom one.
  protected setBtnPad(axis: 'btnPadX' | 'btnPadY', value: string): void {
    const n = parseInt(value, 10);
    this.draft.update((d) => ({ ...d, [axis]: Math.max(0, Math.min(500, Number.isFinite(n) ? n : 12)) }));
  }

  // The preview mirrors the configured gaps, softened to its reduced scale.
  protected anchorStyle(): Record<string, string> {
    const d = this.draft();
    const [edge, align] = (d.btnPosition || 'top-right').split('-');
    return {
      [edge]: Math.round(Math.min(d.btnPadY, 120) / 2 + 6) + 'px',
      [align]: Math.round(Math.min(d.btnPadX, 120) / 2 + 6) + 'px',
    };
  }

  // Preview sizing mirrors the component's own scale rules.
  protected btnFontPx(): number {
    return Math.max(11, Math.round(this.draft().btnHeight * 0.42));
  }

  protected btnRadiusPx(): string {
    return this.draft().btnShape === 'square'
      ? Math.max(4, Math.round(this.draft().btnHeight * 0.18)) + 'px'
      : '999px';
  }

  protected avatarRadiusPx(): string {
    return this.draft().btnShape === 'square'
      ? Math.max(3, Math.round(this.draft().btnHeight * 0.14)) + 'px'
      : '50%';
  }

  // The code editor (CodeMirror) is LAZY-imported: it never weighs on the
  // initial bundle, only on the first "Add CSS/JavaScript" click.
  protected async editCode(kind: CodeKind): Promise<void> {
    const { CodeDialogComponent } = await import('../code-dialog.component');
    const language = kind === 'css' ? 'css' : 'js';
    // The language script is handed the choice; it has to be told under what
    // name, or it is written by guesswork.
    const data =
      kind === 'onLocaleChange'
        ? {
            // Empty opens on the example, so the variable is named by working
            // code rather than by a sentence about it.
            code: this.codeOf(kind) || LOCALE_SCRIPT_SEED,
            language,
            title: $localize`:@@On_language_change:On language change`,
            hint: $localize`:@@Locale_script_hint:Called with the language in locale: on every change, and once when a page loads. Apply it in place - no reload.`,
          }
        : { code: this.codeOf(kind), language };
    const result = await firstValueFrom(
      this.dialog
        .open<unknown, typeof data, string | undefined>(CodeDialogComponent, {
          data,
          maxWidth: '90vw',
          restoreFocus: true,
        })
        .afterClosed(),
    );
    if (result === undefined) return;
    this.draft.update((d) => ({ ...d, [CODE_FIELD[kind]]: result }));
    // The dialog's button says Save and MEANS it: the route is saved (and
    // applied) right away, no second Save in the drawer needed.
    this.save();
  }

  private codeOf(kind: CodeKind): string {
    return this.draft()[CODE_FIELD[kind]];
  }

  protected codeLines(kind: CodeKind): number {
    const code = this.codeOf(kind).trim();
    return code ? code.split('\n').length : 0;
  }

  protected save(after?: (out: Route) => void): void {
    this.error.set('');
    this.saving.set(true);
    const d = this.draft();
    const route: Route = {
      id: this.route()?.id ?? crypto.randomUUID(),
      name: d.name.trim(),
      order: this.route()?.order ?? 0,
      enabled: d.enabled,
      access: d.access,
      // Never stored ON when the answer could not carry a page: the checkbox
      // shows that, and what is saved has to agree with what is shown.
      isUi: d.isUi && this.uiPossible(),
      upstream: d.upstream.trim(),
      // Two cleaners, because the two answer different questions: a predicate
      // with nothing in it is a row somebody started and left, a filter with
      // nothing in it is a brick that needs nothing.
      predicates: cleanPredicates(d.predicates),
      filters: trimTemplates(cleanSpecs(d.filters)),
      timeouts: d.timeouts,
      breaker: d.breakerOn
        ? {
            enabled: true,
            trip: Number(d.breakerTrip) || undefined,
            cool: d.breakerCool || undefined,
          }
        : undefined,
    };
    // The endpoint policies are NOT edited here - they have their own screen -
    // so they are carried over rather than dropped: a screen that saves what it
    // only meant to display is how a rule disappears without anyone deciding.
    const security = this.route()?.api?.security;
    if (d.specSource === 'upstream' && d.specPath.trim()) {
      route.api = { spec: { type: 'upstream', path: d.specPath.trim() } };
    } else if (d.specSource === 'file' && d.specFilename) {
      // A file is only declared once one has been deposited: declaring one
      // that is not there is the dangling reference this feature avoids.
      route.api = { spec: { type: 'file', path: d.specPath.trim() || 'openapi.json', filename: d.specFilename } };
    }
    if (security) {
      route.api = { ...(route.api ?? {}), security };
    }
    if (d.isUi && this.uiPossible()) {
      route.ui = {
        scheme: {
          select: d.schemeSelect,
          mechanism: d.schemeMechanism as '' | 'attribute' | 'class',
          attribute: d.schemeAttribute.trim(),
          light: d.schemeLight.trim(),
          dark: d.schemeDark.trim(),
        },
        roles: {
          enabled: d.rolesEnabled,
          mechanism: d.rolesMode,
          tag: d.rolesTag.trim(),
          attribute: d.rolesAttribute.trim(),
        },
        userInfo: {
          enabled: d.userInfoEnabled,
          mechanism: d.userInfoMode,
          tag: d.userInfoTag.trim(),
          fields: Object.fromEntries(
            Object.entries(d.userInfoFields)
              .filter(([f, v]) => v.enabled && !this.frozen(f))
              .map(([f, v]) => [f, v.name.trim()]),
          ),
        },
        userButton: {
          enabled: d.btnEnabled,
          height: d.btnHeight,
          position: d.btnPosition,
          shape: d.btnShape as '' | 'round' | 'square',
          name: d.btnName as '' | 'before' | 'after',
          padX: d.btnPadX,
          padY: d.btnPadY,
          inFrame: d.btnInFrame,
        },
        customCss: d.customCss,
        customJs: d.customJs,
        link: d.uiLink.trim(),
      };
    }
    const identity = this.buildIdentity();
    if (identity) route.identity = identity;
    route.locales = {
      mechanism: d.localeMechanism,
      header: d.localesHeader.trim(),
      param: d.localesParam.trim(),
      disabled: d.localesDisabled,
      // The hook only exists for pages, and only pages get it injected.
      onChange: d.isUi ? d.localesOnChange : '',
    };
    this.api.putRoute(route).subscribe({
      next: (out) => {
        this.saving.set(false);
        if (after) after(out);
        else this.saved.emit(out);
      },
      error: (err: unknown) => {
        this.saving.set(false);
        const msg =
          err instanceof HttpErrorResponse && typeof err.error?.error === 'string'
            ? err.error.error
            : $localize`:@@Save_failed:Save failed`;
        this.error.set(msg);
      },
    });
  }

  // Identity forwarding as the wire shape: only when a transport is picked,
  // only the selected facts, a mapping equal to the field name dropped (it is
  // the default), and roles carry their shaping expression. Shared by Save and
  // the preview dialog.
  private buildIdentity(): IdentityForward | null {
    const d = this.draft();
    if (!d.identityMechanism) return null;
    const attributes: IdentityAttr[] = IDENTITY_FIELDS.filter(
      (f) => d.identityAttrs[f].selected && !this.frozen(f),
    ).map((f) => {
      const a: IdentityAttr = { field: f };
      const as = d.identityAttrs[f].as.trim();
      if (as && as !== f) a.as = as;
      if (f === 'roles' && d.identityAttrs[f].expr.trim()) a.expr = d.identityAttrs[f].expr.trim();
      return a;
    });
    const identity: IdentityForward = { mechanism: d.identityMechanism, attributes };
    if (d.identityMechanism === 'jwt' || d.identityMechanism === 'signed-jwt') {
      if (d.identityTtl.trim()) identity.ttl = d.identityTtl.trim();
    }
    if (d.identityMechanism === 'signed-jwt' && d.identityAlgorithm) {
      identity.algorithm = d.identityAlgorithm;
    }
    return identity;
  }

  // Show what the upstream would receive, from the DRAFT config (no save).
  protected openIdentityPreview(): void {
    const identity = this.buildIdentity();
    if (!identity) return;
    this.dialog.open<IdentityPreviewDialogComponent, IdentityPreviewData>(IdentityPreviewDialogComponent, {
      width: '700px',
      restoreFocus: true,
      data: { routeName: this.draft().name.trim(), identity },
    });
  }

  // Switching source clears what belonged to the other one: an upstream url
  // left behind under "deposited" would be read by nobody and understood by
  // everybody as still in force.
  protected setSpecSource(value: '' | 'upstream' | 'file'): void {
    this.draft.update((d) => ({ ...d, specSource: value, specPath: '', specFilename: value === 'file' ? d.specFilename : '' }));
    if (value !== 'file') {
      this.specShadow.set(false);
    }
  }

  // Deposit: the route has to exist first (the file is stored against it), so
  // this saves it the same way the endpoint-security button does.
  protected depositSpec(files: FileList | null): void {
    const file = files?.[0];
    if (!file) return;
    this.error.set('');
    this.save((out) => {
      this.saving.set(true);
      this.api.depositRouteSpec(out.id, file, this.draft().specPath.trim()).subscribe({
        next: (deposit) => {
          this.saving.set(false);
          this.specShadow.set(!!deposit.shadows);
          this.draft.update((d) => ({
            ...d,
            specSource: 'file',
            specPath: deposit.path,
            specFilename: deposit.filename,
          }));
          this.saved.emit({
            ...out,
            api: { ...(out.api ?? {}), spec: { type: 'file', path: deposit.path, filename: deposit.filename } },
          });
        },
        error: (err: HttpErrorResponse) => {
          this.saving.set(false);
          this.error.set(err.error?.error ?? 'the file could not be read as an OpenAPI specification');
        },
      });
    });
  }

  // Removing drops the file AND the declaration: the server refuses to leave
  // one without the other, and so does this screen.
  protected removeSpec(): void {
    const id = this.route()?.id;
    if (!id) return;
    this.saving.set(true);
    this.api.deleteRouteSpec(id).subscribe({
      next: () => {
        this.saving.set(false);
        this.specShadow.set(false);
        this.draft.update((d) => ({ ...d, specSource: '', specPath: '', specFilename: '' }));
        const current = this.route();
        if (current) {
          this.saved.emit({ ...current, api: { ...(current.api ?? {}), spec: undefined } });
        }
      },
      error: (err: HttpErrorResponse) => {
        this.saving.set(false);
        this.error.set(err.error?.error ?? 'the file could not be removed');
      },
    });
  }

  // Whether the upstream already answers where the deposited file now does.
  // Set by the deposit itself: that is the moment it can be checked, and the
  // moment it matters.
  protected readonly specShadow = signal(false);

  // Where the deposited file answers. The server is the authority
  // (routing.MatchPrefix); this mirrors it so nobody has to deposit a file to
  // find out where it landed.
  protected specUrl(): string {
    const d = this.draft();
    return matchPrefix(d.predicates) + '/' + (d.specPath.trim() || 'openapi.json');
  }

  // Save the route, then jump to its endpoint-security screen (which needs the
  // route persisted, with its spec, to fetch the operations).
  protected goEndpointSecurity(): void {
    this.save((out) => void this.router.navigate(['/infra/endpoint-security'], { queryParams: { route: out.id } }));
  }
}

// The inherited entry's wording. It names what the INSTALLATION is running, so
// somebody choosing "inherit" knows what they are inheriting - a label reading
// "Default (5 s)" on an installation set to 30 is worse than no label at all.
function labelFor(iso: string | undefined, builtIn: string): string {
  if (!iso) return $localize`:@@Inherited_builtin:Inherited (${builtIn}:VALUE:)`;
  return $localize`:@@Inherited_house:Inherited (${humanIso(iso)}:VALUE:)`;
}
