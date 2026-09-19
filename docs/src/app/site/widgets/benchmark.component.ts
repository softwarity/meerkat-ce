import { Component, computed, input, signal } from '@angular/core';

// The benchmark table, and the ONE page of the site that is not Markdown -
// because it is not prose. It renders the LATEST run the CI made (tools/bench,
// ci.yml "Benchmark"), fetched on every visit rather than built in: the CI
// rewrites latest.json on the `bench` branch of the community mirror after
// each commit to main, so the figures follow the code without a redeploy.
//
// A Markdown page asks for it with `widget: benchmark` in its front matter,
// and gives it the words around it - see content/<lang>/product/performance.md
// and the page component that mounts this.

const LATEST_URL = 'https://raw.githubusercontent.com/softwarity/meerkat-ce/bench/latest.json';
const SOURCES_URL = 'https://github.com/softwarity/meerkat-ce/tree/main/tools/bench';
const CONFIGS_URL = 'https://github.com/softwarity/meerkat-ce/tree/main/tools/bench/gateways';
const RUNNERS_URL = 'https://docs.github.com/en/actions/reference/runners/github-hosted-runners';

type Lang = 'en' | 'fr';

interface Figures {
  rps: number;
  p50: number;
  p90: number;
  p99: number;
  success: number;
  overheadP50?: number;
  overheadP99?: number;
}

interface ScenarioResult {
  kind: 'comparable' | 'deployed';
  fixed: Figures;
  max: Figures;
}

interface GatewayResult {
  name: string;
  version: string;
  startupMs: number;
  memoryIdleMiB: number | null;
  memoryPeakMiB: number | null;
  scenarios: Record<string, ScenarioResult>;
}

interface Run {
  date: string;
  commit: string;
  sourceCommit: string | null;
  machine: {
    runner: string;
    arch: string;
    cpuModel: string;
    cpus: number;
    threadsPerCore: number;
    memoryGiB: number;
  };
  protocol: { gatewayMemory: string; rate: number; fixedDuration: string; maxDuration: string };
  direct: { fixed: Figures; max: Figures };
  gateways: GatewayResult[];
  goBench: { name: string; nsPerOp: number; allocsPerOp: number | null; vsBareProxy: number | null }[];
}

// Where a value stands among the compared products in its column: the best is
// a good student, the worst a bad mark, the rest in between. The reference and
// the signed-JWT row are not graded (they are not part of the comparison).
type Tier = 'good' | 'medium' | 'bad';

interface Cell {
  added50: number;
  added99: number;
  rps: number;
  // Share of the max run that did not come back 200. Only a deployed scenario
  // is ever recorded with one (tools/bench/report.py refuses the others), and
  // a throughput reached with errors must not read as a clean one.
  errors: number;
  tier: Partial<Record<'added50' | 'added99' | 'rps', Tier>>;
}

interface FootCell {
  startupMs: number;
  idle: number | null;
  peak: number | null;
  tier: Partial<Record<'startup' | 'idle' | 'peak', Tier>>;
}

const GATEWAYS = ['meerkat', 'gostd', 'kong', 'apisix', 'traefik'];

const LABELS: Record<string, Record<Lang, string>> = {
  meerkat: { en: 'Meerkat', fr: 'Meerkat' },
  gostd: { en: 'Go standard library (reference)', fr: 'Bibliothèque standard Go (référence)' },
  kong: { en: 'Kong Gateway OSS', fr: 'Kong Gateway OSS' },
  apisix: { en: 'Apache APISIX', fr: 'Apache APISIX' },
  traefik: { en: 'Traefik Proxy', fr: 'Traefik Proxy' },
};

// How each product is set up, in the reader's language.
const HOW: Record<string, Record<string, Record<Lang, string>>> = {
  proxy: {
    gostd: {
      en: 'httputil.ReverseProxy alone: no routes, no identity, no metrics',
      fr: 'httputil.ReverseProxy seul : ni routes, ni identité, ni métriques',
    },
  },
  auth: {
    meerkat: {
      en: "personal API token resolved by the gateway, caller's name and id forwarded as headers, no roles",
      fr: "jeton d'API personnel résolu par la gateway, nom et identifiant de l'appelant transmis en en-têtes, aucun rôle",
    },
    kong: {
      en: "key-auth plugin, consumer's name and id forwarded as headers, no roles",
      fr: 'plugin key-auth, nom et identifiant du consommateur transmis en en-têtes, aucun rôle',
    },
    apisix: {
      en: "key-auth plugin, consumer's name forwarded as headers, no roles",
      fr: 'plugin key-auth, nom du consommateur transmis en en-têtes, aucun rôle',
    },
    traefik: {
      en: 'ForwardAuth: an external service decides on every request',
      fr: 'ForwardAuth : un service externe décide à chaque requête',
    },
  },
  'auth-jwt': {
    meerkat: {
      en: 'the same token, caller forwarded as an ES256-signed JWT (the bench account holds no role) - Meerkat only, not part of the comparison',
      fr: 'le même jeton, appelant transmis dans un JWT signé ES256 (le compte du banc ne détient aucun rôle) - Meerkat seul, hors comparaison',
    },
  },
};

const T = {
  title: { en: 'Performance', fr: 'Performance' },
  intro: {
    en: 'The figures below come from the latest commit the CI benchmarked, read live. Meerkat is measured next to three open-source gateways, each given the same single CPU, in front of the same service, under the same load, in the same run: they compare products, not machines.',
    fr: "Les chiffres ci-dessous viennent du dernier commit mesuré par la CI, lus en direct. Meerkat est mesuré à côté de trois gateways libres, chacune avec le même CPU unique, devant le même service, sous la même charge, dans la même exécution : ils comparent des produits, pas des machines.",
  },

  howTitle: { en: 'How it is measured', fr: 'Comment on mesure' },
  how1: {
    en: 'Every product runs in a container pinned to one CPU with 1 GB of memory, and is told it has one core: a single nginx worker for Kong and APISIX, GOMAXPROCS=1 for the Go ones. The service behind them is a minimal Go server pinned to another CPU; the load generator (oha) uses the remaining CPUs. Access logs are off everywhere, since Meerkat writes none by default.',
    fr: "Chaque produit tourne dans un conteneur épinglé sur un CPU avec 1 Go de mémoire, et sait qu'il n'a qu'un cœur : un seul worker nginx pour Kong et APISIX, GOMAXPROCS=1 pour les produits en Go. Le service derrière eux est un serveur Go minimal épinglé sur un autre CPU ; le générateur de charge (oha) utilise les CPU restants. Les journaux d'accès sont coupés partout, puisque Meerkat n'en écrit pas par défaut.",
  },
  how2: {
    en: 'Two runs per scenario. At a fixed 1,000 requests per second, with latency correction so a stall is not hidden, the percentiles say what a user waits; "added" is that latency minus a direct call to the service in the same run. At the maximum rate, the throughput says what one core carries.',
    fr: "Deux mesures par scénario. À 1 000 requêtes par seconde fixes, avec correction de latence pour qu'un blocage ne soit pas masqué, les percentiles disent ce qu'attend un utilisateur ; « ajouté » est cette latence moins un appel direct au service dans la même exécution. Au débit maximal, le débit dit ce que porte un cœur.",
  },
  how3: {
    en: 'Every scenario is checked before it is measured: the route must answer 200 with its credential and refuse without it, so a plugin that silently failed to load cannot pass for a fast gateway. A run where fewer than 99% of the requests answered 200 is refused rather than published.',
    fr: "Chaque scénario est vérifié avant d'être mesuré : la route doit répondre 200 avec son jeton et refuser sans, pour qu'un plugin qui n'a pas chargé ne passe pas pour une gateway rapide. Une mesure où moins de 99 % des requêtes ont répondu 200 est refusée plutôt que publiée.",
  },

  othersTitle: { en: 'How the other products are tested', fr: 'Comment les autres produits sont testés' },
  others1: {
    en: 'Each in its free edition, at a pinned version, configured from files anyone can read and replay, with nothing tuned beyond what fairness needs (one worker per core, logs off):',
    fr: "Chacun dans son édition libre, à une version figée, configuré par des fichiers que chacun peut lire et rejouer, sans autre réglage que ce que l'équité impose (un worker par cœur, journaux coupés) :",
  },
  othersKong: {
    en: 'Kong Gateway OSS, without a database (declarative file): key-auth for the authenticated route, rate-limiting with the local policy.',
    fr: 'Kong Gateway OSS, sans base de données (fichier déclaratif) : key-auth pour la route authentifiée, rate-limiting en politique locale.',
  },
  othersApisix: {
    en: 'Apache APISIX, standalone (no etcd): key-auth, limit-count with the local policy.',
    fr: 'Apache APISIX, en mode autonome (sans etcd) : key-auth, limit-count en politique locale.',
  },
  othersTraefik: {
    en: 'Traefik Proxy, open source: its rateLimit middleware; it has no token check of its own, so authentication goes through ForwardAuth.',
    fr: "Traefik Proxy, libre : son middleware rateLimit ; il ne vérifie aucun jeton lui-même, l'authentification passe donc par ForwardAuth.",
  },
  othersGo: {
    en: "And a reference that is not a product: Go's standard reverse proxy with nothing around it, on the proxy route only.",
    fr: "Et une référence qui n'est pas un produit : le proxy standard de Go, sans rien autour, sur la seule route proxy.",
  },
  comparable: {
    en: 'Comparable: proxying, a credential checked by the gateway itself, and a rate limit set far above the load (the cost of counting, not a refusal).',
    fr: "Comparable : le proxy, un jeton vérifié par la gateway elle-même, et une limite de débit fixée bien au-dessus de la charge (le coût du comptage, pas un refus).",
  },
  asDeployed: {
    en: 'As deployed: when authentication is delegated to an external service, as with Traefik here, every request pays a network round trip before reaching the service. The one used here is the fastest possible, so the hop measured is a floor: a real oauth2-proxy, Authelia or identity provider only adds to it. Meerkat decides inside the gateway, with no hop.',
    fr: "Tel que déployé : quand l'authentification est déléguée à un service externe, comme avec Traefik ici, chaque requête paie un aller-retour réseau avant d'atteindre le service. Celui utilisé ici est le plus rapide possible, le rebond mesuré est donc un minimum : un vrai oauth2-proxy, Authelia ou fournisseur d'identité ne fait qu'y ajouter. Meerkat décide dans la gateway, sans rebond.",
  },
  sameAsOthers: {
    en: "In the comparable scenario, Meerkat forwards the caller the way Kong and APISIX do: name and id as headers, and no roles, which their key-auth does not forward either. One more row, set apart, shows what Meerkat usually does in production and the others do not: the caller forwarded as a signed JWT the service can verify on its own.",
    fr: "Dans le scénario comparable, Meerkat transmet l'appelant comme Kong et APISIX : nom et identifiant en en-têtes, et aucun rôle, que leur key-auth ne transmet pas non plus. Une ligne de plus, mise à part, montre ce que Meerkat fait d'habitude en production et que les autres ne font pas : l'appelant transmis dans un JWT signé que le service peut vérifier seul.",
  },

  whyTitle: { en: 'Why Meerkat cannot go much further', fr: 'Pourquoi Meerkat ne peut guère aller plus loin' },
  why1: {
    en: 'Kong and APISIX are nginx and LuaJIT: an event loop written in C and tuned for twenty years. Meerkat is written in Go on net/http, the library that gives it HTTP/2, gRPC trailers, WebSockets and TLS from one audited code base. The reference row measures that library alone, with none of Meerkat around it:',
    fr: "Kong et APISIX, c'est nginx et LuaJIT : une boucle d'événements écrite en C et réglée depuis vingt ans. Meerkat est écrit en Go sur net/http, la bibliothèque qui lui donne HTTP/2, les trailers gRPC, les WebSockets et TLS depuis un seul code audité. La ligne de référence mesure cette bibliothèque seule, sans rien de Meerkat autour :",
  },
  ratio: {
    en: 'on {arch}, Meerkat carries {pct} of what the bare Go proxy carries.',
    fr: 'sur {arch}, Meerkat porte {pct} de ce que porte le proxy Go nu.',
  },
  why2: {
    en: 'What remains between them is what Meerkat does on every request - choosing the route among its predicates, counting per route and per endpoint, checking the session. Going past the reference would mean leaving net/http, and HTTP/2, gRPC and WebSockets with it: not a trade for a gateway whose job is identity. At a normal rate, all of them add well under a millisecond; the ceiling only matters for one saturated core - and Meerkat serves the sign-in pages, the second factor, the organisations, the roles and the audit trail in that same process, which the others do not do at all.',
    fr: "Ce qui reste entre les deux, c'est ce que Meerkat fait à chaque requête : choisir la route parmi ses prédicats, compter par route et par endpoint, vérifier la session. Dépasser la référence voudrait dire quitter net/http, et HTTP/2, gRPC et les WebSockets avec : pas un échange raisonnable pour une gateway dont le métier est l'identité. À débit normal, toutes ajoutent bien moins d'une milliseconde ; le plafond ne compte que pour un cœur saturé - et Meerkat sert dans ce même processus les pages de connexion, le second facteur, les organisations, les rôles et le journal d'audit, ce que les autres ne font pas du tout.",
  },

  whereTitle: { en: 'Where it runs', fr: 'Où ça tourne' },
  where1: {
    en: "On GitHub's hosted runners for public repositories, x64 and arm64: 4 vCPU and 16 GB each, shared virtual machines whose processor model GitHub does not document and which varies, so each run records its own below. Neighbours on the same host make figures move by several percent from one run to the next; comparisons are therefore made within one run, never across runs.",
    fr: "Sur les runners hébergés de GitHub pour dépôts publics, x64 et arm64 : 4 vCPU et 16 Go chacun, des machines virtuelles partagées dont GitHub ne documente pas le modèle de processeur et qui varie ; chaque exécution relève donc le sien ci-dessous. Les voisins sur le même hôte font bouger les chiffres de plusieurs pour cent d'une exécution à l'autre ; les comparaisons se font donc dans une même exécution, jamais entre deux.",
  },
  where2: {
    en: 'A production gateway usually runs on dedicated cores, several of them, with a faster network than a shared VM: its absolute figures can be expected to be higher. What carries over is the gap between products.',
    fr: "Une gateway de production tourne en général sur des cœurs dédiés, plusieurs, avec un réseau plus rapide qu'une VM partagée : ses chiffres absolus devraient y être meilleurs. Ce qui se transpose, ce sont les écarts entre produits.",
  },

  loading: { en: 'Loading the latest benchmark...', fr: 'Chargement de la dernière mesure...' },
  none: {
    en: 'No benchmark published yet: the first one appears after the next commit the CI measures.',
    fr: "Aucune mesure publiée pour l'instant : la première apparaît après le prochain commit mesuré par la CI.",
  },
  machines: { en: 'Machines of this run', fr: 'Machines de cette exécution' },
  proxy: { en: 'Proxy', fr: 'Proxy' },
  auth: { en: 'Authenticated request', fr: 'Requête authentifiée' },
  limit: { en: 'Rate limit', fr: 'Limitation de débit' },
  footprint: { en: 'Footprint', fr: 'Empreinte' },
  gobench: { en: 'Go micro-benchmarks', fr: 'Micro-bancs Go' },
  gobenchNote: {
    en: 'In-process, from internal/gateway/bench_test.go: read against the standard library proxy measured in the same run; allocations are a count, not a duration, so they do not move with the machine.',
    fr: "En processus, depuis internal/gateway/bench_test.go : à lire par rapport au proxy de la bibliothèque standard mesuré dans la même exécution ; les allocations sont un compte, pas une durée, elles ne bougent donc pas avec la machine.",
  },
  delegated: { en: 'delegated', fr: 'déléguée' },
  signedJwt: { en: 'signed JWT', fr: 'JWT signé' },
  errors: { en: 'errors', fr: "d'erreurs" },
  gateway: { en: 'Gateway', fr: 'Gateway' },
  added: { en: 'added', fr: 'ajouté' },
  legend: {
    en: 'In each column, of the compared products: best in green, worst in red, the rest amber. The bare-Go reference and the signed-JWT row are not graded.',
    fr: 'Dans chaque colonne, parmi les produits comparés : le meilleur en vert, le pire en rouge, les autres en ambre. La référence Go nue et la ligne JWT signé ne sont pas notées.',
  },
  ready: { en: 'ready in', fr: 'prête en' },
  idle: { en: 'idle', fr: 'au repos' },
  peak: { en: 'under load', fr: 'sous charge' },
  sources: { en: 'Benchmark sources', fr: 'Sources du banc' },
  configs: { en: 'configurations of the other products', fr: 'configurations des autres produits' },
  runners: { en: 'GitHub runner specifications', fr: 'caractéristiques des runners GitHub' },
};

@Component({
  selector: 'app-benchmark',
  styles: [
    `
      .machines {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
        gap: 12px;
        margin: 10px 0 18px;
      }
      .machine {
        border: 1px solid var(--line);
        border-radius: 8px;
        padding: 10px 14px;
        background: var(--surface-2);
        font-size: 0.88em;
      }
      .machine strong {
        display: block;
        margin-bottom: 4px;
      }
      .muted {
        color: var(--muted);
      }
      .mono {
        font-family: 'Courier New', Consolas, monospace;
      }
      .scroll {
        overflow-x: auto;
      }
      td.num,
      th.num {
        text-align: right;
        font-variant-numeric: tabular-nums;
        white-space: nowrap;
      }
      tr.mk td {
        color: var(--accent);
        font-weight: 600;
      }
      tr.ref td {
        font-style: italic;
      }
      .how {
        font-size: 0.85em;
        color: var(--muted);
        font-style: normal;
        font-weight: normal;
      }
      .err {
        font-size: 0.8em;
        color: var(--danger);
      }
      /* Good student / middle / bad mark, per column. A background tint so it
         reads next to the Meerkat row's own text colour instead of fighting it. */
      td.num[data-tier='good'] {
        background: color-mix(in srgb, #1a7f37 16%, transparent);
      }
      td.num[data-tier='medium'] {
        background: color-mix(in srgb, #bf8700 15%, transparent);
      }
      td.num[data-tier='bad'] {
        background: color-mix(in srgb, var(--danger) 16%, transparent);
      }
      .legend {
        font-size: 0.82em;
        color: var(--muted);
        margin: 4px 0 0;
      }
      .legend .swatch {
        display: inline-block;
        width: 0.8em;
        height: 0.8em;
        border-radius: 3px;
        vertical-align: -1px;
        margin: 0 3px 0 8px;
      }
      .legend .swatch.good {
        background: color-mix(in srgb, #1a7f37 55%, transparent);
      }
      .legend .swatch.medium {
        background: color-mix(in srgb, #bf8700 55%, transparent);
      }
      .legend .swatch.bad {
        background: color-mix(in srgb, var(--danger) 55%, transparent);
      }
      .ratio {
        margin: 6px 0 6px 18px;
      }
    `,
  ],
  template: `
    <h2>{{ t('howTitle') }}</h2>
    <p>{{ t('how1') }}</p>
    <p>{{ t('how2') }}</p>
    <p>{{ t('how3') }}</p>

    <h2>{{ t('othersTitle') }}</h2>
    <p>{{ t('others1') }}</p>
    <ul>
      <li>{{ t('othersKong') }}</li>
      <li>{{ t('othersApisix') }}</li>
      <li>{{ t('othersTraefik') }}</li>
      <li>{{ t('othersGo') }}</li>
    </ul>
    <p>{{ t('comparable') }}</p>
    <p>{{ t('sameAsOthers') }}</p>
    <p>{{ t('asDeployed') }}</p>

    <h2>{{ t('whyTitle') }}</h2>
    <p>{{ t('why1') }}</p>
    @for (line of ratios(); track line) {
      <p class="ratio"><strong>{{ line }}</strong></p>
    }
    <p>{{ t('why2') }}</p>

    <h2>{{ t('whereTitle') }}</h2>
    <p>{{ t('where1') }}</p>
    <p>{{ t('where2') }}</p>

    @switch (state()) {
      @case ('loading') {
        <p class="muted">{{ t('loading') }}</p>
      }
      @case ('empty') {
        <p class="muted">{{ t('none') }}</p>
      }
      @case ('ready') {
        <h2>{{ t('machines') }}</h2>
        <div class="machines">
          @for (run of runs(); track run.machine.runner) {
            <div class="machine">
              <strong>{{ run.machine.runner }}</strong>
              <div>{{ run.machine.cpuModel }}</div>
              <div>{{ run.machine.cpus }} vCPU, {{ run.machine.memoryGiB }} GiB, {{ run.machine.arch }}</div>
              <div class="muted mono">{{ runDate(run) }} UTC - {{ commitLabel(run) }}</div>
            </div>
          }
        </div>

        @for (sc of scenarioTables(); track sc.key) {
          <h2>{{ t(sc.key) }}</h2>
          <div class="scroll">
            <table>
              <thead>
                <tr>
                  <th>{{ t('gateway') }}</th>
                  @for (run of runs(); track run.machine.runner) {
                    <th class="num">p50 {{ t('added') }}<br /><span class="muted">{{ run.machine.arch }}</span></th>
                    <th class="num">p99 {{ t('added') }}<br /><span class="muted">{{ run.machine.arch }}</span></th>
                    <th class="num">req/s max<br /><span class="muted">{{ run.machine.arch }}</span></th>
                  }
                </tr>
              </thead>
              <tbody>
                @for (row of sc.rows; track row.label) {
                  <tr [class.mk]="row.meerkat" [class.ref]="row.reference">
                    <td>
                      {{ row.label }}
                      @if (row.how) {
                        <div class="how">{{ row.how }}</div>
                      }
                    </td>
                    @for (cell of row.cells; track $index) {
                      @if (cell) {
                        <td class="num" [attr.data-tier]="cell.tier.added50">{{ ms(cell.added50) }}</td>
                        <td class="num" [attr.data-tier]="cell.tier.added99">{{ ms(cell.added99) }}</td>
                        <td class="num" [attr.data-tier]="cell.tier.rps">
                          {{ cell.rps.toLocaleString(locale()) }}
                          @if (cell.errors >= 0.01) {
                            <div class="err">{{ percent(cell.errors) }} {{ t('errors') }}</div>
                          }
                        </td>
                      } @else {
                        <td class="num">-</td>
                        <td class="num">-</td>
                        <td class="num">-</td>
                      }
                    }
                  </tr>
                }
              </tbody>
            </table>
          </div>
          <p class="legend">
            {{ t('legend') }}
          </p>
        }

        <h2>{{ t('footprint') }}</h2>
        <div class="scroll">
          <table>
            <thead>
              <tr>
                <th>{{ t('gateway') }}</th>
                @for (run of runs(); track run.machine.runner) {
                  <th class="num">{{ t('ready') }}<br /><span class="muted">{{ run.machine.arch }}</span></th>
                  <th class="num">MiB {{ t('idle') }}<br /><span class="muted">{{ run.machine.arch }}</span></th>
                  <th class="num">MiB {{ t('peak') }}<br /><span class="muted">{{ run.machine.arch }}</span></th>
                }
              </tr>
            </thead>
            <tbody>
              @for (row of footprint(); track row.label) {
                <tr [class.mk]="row.meerkat" [class.ref]="row.reference">
                  <td>{{ row.label }} <span class="muted mono">{{ row.version }}</span></td>
                  @for (cell of row.cells; track $index) {
                    <td class="num" [attr.data-tier]="cell?.tier?.startup">{{ cell ? cell.startupMs + ' ms' : '-' }}</td>
                    <td class="num" [attr.data-tier]="cell?.tier?.idle">{{ cell?.idle ?? '-' }}</td>
                    <td class="num" [attr.data-tier]="cell?.tier?.peak">{{ cell?.peak ?? '-' }}</td>
                  }
                </tr>
              }
            </tbody>
          </table>
        </div>
        <p class="legend">
          {{ t('legend') }}
        </p>

        @if (goBench().length) {
          <h2>{{ t('gobench') }}</h2>
          <p class="muted">{{ t('gobenchNote') }}</p>
          <div class="scroll">
            <table>
              <thead>
                <tr>
                  <th>Benchmark</th>
                  @for (run of runs(); track run.machine.runner) {
                    <th class="num">ns/op<br /><span class="muted">{{ run.machine.arch }}</span></th>
                    <th class="num">x bare proxy<br /><span class="muted">{{ run.machine.arch }}</span></th>
                  }
                  <th class="num">allocs/op</th>
                </tr>
              </thead>
              <tbody>
                @for (row of goBench(); track row.name) {
                  <tr>
                    <td class="mono">{{ row.name }}</td>
                    @for (cell of row.cells; track $index) {
                      <td class="num">{{ cell ? cell.nsPerOp.toLocaleString(locale()) : '-' }}</td>
                      <td class="num">{{ cell?.vsBareProxy ?? '-' }}</td>
                    }
                    <td class="num">{{ row.allocs ?? '-' }}</td>
                  </tr>
                }
              </tbody>
            </table>
          </div>
        }
      }
    }

    <p class="muted">
      <a [href]="sourcesUrl" target="_blank" rel="noopener">{{ t('sources') }}</a>
      - <a [href]="configsUrl" target="_blank" rel="noopener">{{ t('configs') }}</a>
      - <a [href]="runnersUrl" target="_blank" rel="noopener">{{ t('runners') }}</a>
      - <a [href]="latestUrl" target="_blank" rel="noopener">latest.json</a>
    </p>
  `,
})
export class BenchmarkComponent {
  // The reader's language comes from the address, through the page.
  readonly lang = input.required<Lang>();
  protected readonly sourcesUrl = SOURCES_URL;
  protected readonly configsUrl = CONFIGS_URL;
  protected readonly runnersUrl = RUNNERS_URL;
  protected readonly latestUrl = LATEST_URL;

  private readonly latest = signal<Run[] | null>(null);

  protected readonly state = computed(() => {
    const runs = this.latest();
    if (runs === null) return 'loading';
    return runs.length ? 'ready' : 'empty';
  });

  protected readonly locale = computed(() => (this.lang() === 'fr' ? 'fr-FR' : 'en-US'));

  // One run per machine, amd64 before arm64.
  protected readonly runs = computed(() =>
    [...(this.latest() ?? [])].sort((a, b) => a.machine.arch.localeCompare(b.machine.arch)),
  );

  // Meerkat's proxy throughput against the bare Go proxy, per machine.
  protected readonly ratios = computed(() =>
    this.runs().flatMap((run) => {
      const mk = run.gateways.find((g) => g.name === 'meerkat')?.scenarios['proxy'];
      const std = run.gateways.find((g) => g.name === 'gostd')?.scenarios['proxy'];
      if (!mk || !std || !std.max.rps) return [];
      const pct = `${Math.round((mk.max.rps / std.max.rps) * 100)} %`;
      return [this.t('ratio').replace('{arch}', run.machine.arch).replace('{pct}', pct)];
    }),
  );

  protected readonly scenarioTables = computed(() => {
    const runCount = this.runs().length;
    return (['proxy', 'auth', 'limit'] as const).map((key) => {
      const rows = GATEWAYS.flatMap((gw) => {
        // The auth table carries, besides the comparable row, Traefik's
        // delegated one in its place and Meerkat's signed-JWT one below its own.
        const scenarios =
          key !== 'auth' ? [key] : gw === 'traefik' ? ['auth-delegated'] : gw === 'meerkat' ? ['auth', 'auth-jwt'] : ['auth'];
        return scenarios.flatMap((scenario) => this.row(gw, key, scenario));
      });
      // Grade only the products against each other, per column (per run): the
      // reference and the signed-JWT row are set apart and stay ungraded.
      const graded = rows.filter((r) => !r.reference).map((r) => r.cells);
      for (let ri = 0; ri < runCount; ri++) {
        const col = graded.map((cells) => cells[ri]);
        this.tierCells(col, 'added50', true);
        this.tierCells(col, 'added99', true);
        this.tierCells(col, 'rps', false);
      }
      return { key, rows };
    });
  });

  // Mark each product's cell in a column good (best), bad (worst) or medium. A
  // throughput reached with errors is not a clean number - always a bad mark.
  private tierCells(cells: (Cell | null)[], metric: 'added50' | 'added99' | 'rps', lowerBetter: boolean): void {
    const present = cells.filter((c): c is Cell => c !== null);
    const clean = present.filter((c) => !(metric === 'rps' && c.errors >= 0.01));
    if (clean.length < 2) return;
    const vals = clean.map((c) => c[metric]);
    const best = lowerBetter ? Math.min(...vals) : Math.max(...vals);
    const worst = lowerBetter ? Math.max(...vals) : Math.min(...vals);
    for (const c of present) {
      if (metric === 'rps' && c.errors >= 0.01) {
        c.tier[metric] = 'bad';
        continue;
      }
      const v = c[metric];
      c.tier[metric] = best === worst ? 'good' : v === best ? 'good' : v === worst ? 'bad' : 'medium';
    }
  }

  private row(gw: string, key: string, scenario: string) {
    const cells = this.runs().map((run): Cell | null => {
      const s = run.gateways.find((g) => g.name === gw)?.scenarios[scenario];
      return s
        ? {
            added50: s.fixed.overheadP50 ?? 0,
            added99: s.fixed.overheadP99 ?? 0,
            rps: Math.round(s.max.rps),
            errors: 1 - s.max.success,
            tier: {},
          }
        : null;
    });
    if (!cells.some((c) => c)) return [];
    const suffix =
      scenario === 'auth-delegated' ? ` (${this.t('delegated')})` : scenario === 'auth-jwt' ? ` (${this.t('signedJwt')})` : '';
    return [
      {
        label: LABELS[gw][this.lang()] + suffix,
        meerkat: gw === 'meerkat' && scenario !== 'auth-jwt',
        reference: gw === 'gostd' || scenario === 'auth-jwt',
        how: HOW[scenario === 'auth-jwt' ? 'auth-jwt' : key]?.[gw]?.[this.lang()] ?? '',
        cells,
      },
    ];
  }

  protected readonly footprint = computed(() => {
    const runCount = this.runs().length;
    const rows = GATEWAYS.flatMap((gw) => {
      const raw = this.runs().map((run) => run.gateways.find((g) => g.name === gw) ?? null);
      const anyG = raw.find((g) => g);
      if (!anyG) return [];
      const cells = raw.map((g): FootCell | null =>
        g ? { startupMs: g.startupMs, idle: g.memoryIdleMiB, peak: g.memoryPeakMiB, tier: {} } : null,
      );
      return [
        {
          label: LABELS[gw][this.lang()],
          meerkat: gw === 'meerkat',
          reference: gw === 'gostd',
          version: anyG.version,
          cells,
        },
      ];
    });
    // Lower is better for all three (ready-in, idle, peak); grade the products.
    const graded = rows.filter((r) => !r.reference).map((r) => r.cells);
    for (let ri = 0; ri < runCount; ri++) {
      const col = graded.map((cells) => cells[ri]);
      this.tierFoot(col, 'startup', 'startupMs');
      this.tierFoot(col, 'idle', 'idle');
      this.tierFoot(col, 'peak', 'peak');
    }
    return rows;
  });

  private tierFoot(cells: (FootCell | null)[], key: 'startup' | 'idle' | 'peak', field: 'startupMs' | 'idle' | 'peak'): void {
    const present = cells.filter((c): c is FootCell => c !== null && c[field] != null);
    if (present.length < 2) return;
    const vals = present.map((c) => c[field] as number);
    const best = Math.min(...vals);
    const worst = Math.max(...vals);
    for (const c of present) {
      const v = c[field] as number;
      c.tier[key] = best === worst ? 'good' : v === best ? 'good' : v === worst ? 'bad' : 'medium';
    }
  }

  protected readonly goBench = computed(() => {
    const names = [...new Set(this.runs().flatMap((run) => run.goBench.map((b) => b.name)))];
    return names.map((name) => {
      const cells = this.runs().map((run) => run.goBench.find((b) => b.name === name) ?? null);
      return { name, cells, allocs: cells.find((c) => c)?.allocsPerOp ?? null };
    });
  });

  constructor() {
    fetch(LATEST_URL, { cache: 'no-cache' })
      .then((r) => (r.ok ? r.json() : { runs: [] }))
      .then((l: { runs?: Run[] }) => this.latest.set(l.runs ?? []))
      .catch(() => this.latest.set([]));
  }

  protected t(key: keyof typeof T): string {
    return T[key][this.lang()];
  }

  // A negative "added" latency is noise: the direct call happened to be the
  // slower one on that percentile. Shown as zero rather than as a gateway
  // making the network faster.
  protected ms(v: number): string {
    return v <= 0.005 ? '~0 ms' : `${v.toFixed(2)} ms`;
  }

  protected percent(v: number): string {
    return `${(v * 100).toFixed(1)} %`;
  }

  protected runDate(run: Run): string {
    return run.date.slice(0, 16).replace('T', ' ');
  }

  protected commitLabel(run: Run): string {
    return run.sourceCommit ? `meerkat@${run.sourceCommit}` : run.commit.slice(0, 7);
  }
}
