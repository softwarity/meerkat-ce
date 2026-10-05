import { Component, computed, input, signal } from '@angular/core';

// "Start the gateway", for the edition and the platform the reader actually
// has. One page used to give one command - the community image under plain
// Docker - and everybody else translated it in their head: another image,
// another file, a pull secret or not. Here the reader says which of the two
// they are on and gets the commands as they would type them, and the files
// this repository ships (deploy/, copied onto the site by gen-deploy.mjs)
// rather than a block pasted into a page.
//
// A Markdown page asks for it with a `::: widget install` block; the page
// component mounts it where the block stands.

type Lang = 'en' | 'fr';
type Edition = 'ce' | 'eval' | 'licensed';
type Platform = 'docker' | 'swarm' | 'k8s';

const FILES = 'https://www.softwarity.io/deploy';
const PASSWORD = "'choose-one-now'";

// The image each edition is pulled as. A licensed image is served for one
// licence: the key is part of its address, which is what every client -
// Docker, containerd, a kubelet - can carry without a login step.
const IMAGE: Record<Edition, string> = {
  ce: 'softwarity/meerkat',
  eval: 'softwarity/meerkat:eval',
  licensed: 'registry.softwarity.io/YOUR_LICENCE_KEY/softwarity/meerkat',
};

interface Step {
  say: string;
  code?: string;
  file?: string; // a file of deploy/, offered as a download beside the command
}

const WORDS = {
  en: {
    edition: 'Edition',
    platform: 'Platform',
    editions: { ce: 'Community', eval: 'Evaluation', licensed: 'Team or Enterprise' } as Record<Edition, string>,
    platforms: { docker: 'Docker', swarm: 'Docker Swarm', k8s: 'Kubernetes' } as Record<Platform, string>,
    about: {
      ce: 'The free edition: the whole gateway for one organisation, production included.',
      eval: 'Everything Enterprise does, free, with an evaluation notice. Not for production.',
      licensed:
        'The image built for your licence. Replace YOUR_LICENCE_KEY with the key you were given: it is part of the image address, so there is no registry to log in to.',
    } as Record<Edition, string>,
    file: 'The file',
    copy: 'Copy',
    copied: 'Copied',
  },
  fr: {
    edition: 'Édition',
    platform: 'Plateforme',
    editions: { ce: 'Community', eval: 'Évaluation', licensed: 'Team ou Enterprise' } as Record<Edition, string>,
    platforms: { docker: 'Docker', swarm: 'Docker Swarm', k8s: 'Kubernetes' } as Record<Platform, string>,
    about: {
      ce: "L'édition gratuite : toute la gateway pour une organisation, production comprise.",
      eval: "Tout ce que fait Enterprise, gratuitement, avec une mention d'évaluation. Pas pour la production.",
      licensed:
        "L'image construite pour votre licence. Remplacez YOUR_LICENCE_KEY par la clé que vous avez reçue : elle fait partie de l'adresse de l'image, il n'y a donc aucun registre auquel se connecter.",
    } as Record<Edition, string>,
    file: 'Le fichier',
    copy: 'Copier',
    copied: 'Copié',
  },
};

// What to type, per platform and edition. Written out rather than assembled
// from fragments: nine cases, each one a thing somebody will paste.
function steps(lang: Lang, edition: Edition, platform: Platform): Step[] {
  const fr = lang === 'fr';
  const image = IMAGE[edition];
  const ee = edition !== 'ce';

  if (platform === 'docker') {
    const compose = ee ? 'docker-compose.ee.yml' : 'docker-compose.yml';
    return [
      {
        say: fr ? 'Une commande, un conteneur :' : 'One command, one container:',
        code: [
          'docker run -d --name meerkat \\',
          '  -p 8080:8080 -p 9090:9090 \\',
          `  -e MEERKAT_ADMIN_PASSWORD=${PASSWORD} \\`,
          '  -v meerkat-data:/data \\',
          `  ${image}`,
        ].join('\n'),
      },
      {
        say: fr
          ? ee
            ? 'Ou avec Compose, qui ouvre aussi le tunnel développeur :'
            : 'Ou avec Compose :'
          : ee
            ? 'Or with Compose, which also opens the developer tunnel:'
            : 'Or with Compose:',
        file: compose,
        code: [
          `curl -O ${FILES}/${compose}`,
          ee
            ? `MEERKAT_IMAGE=${image} \\\nMEERKAT_ADMIN_PASSWORD=${PASSWORD} \\\n  docker compose -f ${compose} up -d`
            : `MEERKAT_ADMIN_PASSWORD=${PASSWORD} docker compose up -d`,
        ].join('\n'),
      },
    ];
  }

  if (platform === 'swarm') {
    if (!ee) {
      return [
        {
          say: fr
            ? "L'édition Community fait tourner une seule gateway : sur Swarm, déployez le fichier Compose comme une stack. Son volume vit sur le nœud où elle tourne."
            : 'The Community edition runs one gateway: on Swarm, deploy the Compose file as a stack. Its volume lives on the node it runs on.',
          file: 'docker-compose.yml',
          code: [
            `curl -O ${FILES}/docker-compose.yml`,
            `MEERKAT_ADMIN_PASSWORD=${PASSWORD} \\\n  docker stack deploy -c docker-compose.yml meerkat`,
          ].join('\n'),
        },
      ];
    }
    return [
      {
        say: fr
          ? 'Trois gateways derrière le maillage de Swarm, sur un PostgreSQL que tous les nœuds atteignent :'
          : "Three gateways behind Swarm's routing mesh, on a PostgreSQL every node can reach:",
        file: 'stack.swarm.yml',
        code: [
          `curl -O ${FILES}/stack.swarm.yml`,
          [
            `MEERKAT_IMAGE=${image} \\`,
            "MEERKAT_DATABASE_URL='postgres://user:password@db:5432/meerkat' \\",
            'MEERKAT_VAULT_KEY=$(openssl rand -hex 32) \\',
            `MEERKAT_ADMIN_PASSWORD=${PASSWORD} \\`,
            '  docker stack deploy -c stack.swarm.yml meerkat',
          ].join('\n'),
        ].join('\n'),
      },
      {
        say: fr
          ? 'Gardez la clé du coffre : elle doit être la même à chaque déploiement. Pour une seule gateway sur un nœud, prenez plutôt le fichier Compose (plateforme Docker).'
          : 'Keep the vault key: it has to be the same on every deployment. For one gateway on one node, take the Compose file instead (the Docker platform).',
      },
    ];
  }

  // Kubernetes, with the chart.
  const values = ee ? 'values-ee-one-node.yaml' : 'values-ce-one-node.yaml';
  const set: string[] = [];
  if (edition === 'eval') {
    set.push('  --set image.repository=docker.io/softwarity/meerkat --set image.tag=eval \\');
  }
  if (edition === 'licensed') {
    set.push(`  --set image.repository=${image} \\`);
  }
  if (ee) {
    // The key is in the address, and the evaluation image is public: neither
    // needs the pull secret the values file names.
    set.push("  --set-json 'image.pullSecrets=[]' \\");
  }
  const out: Step[] = [
    {
      say: fr ? 'Une gateway sur son volume, avec le chart :' : 'One gateway on its own volume, with the chart:',
      file: values,
      code: [
        `helm repo add meerkat ${FILES}`,
        'helm install meerkat meerkat/meerkat \\',
        `  -f ${FILES}/${values} \\`,
        ...set,
        `  --set admin.password=${PASSWORD}`,
      ].join('\n'),
    },
    {
      say: fr ? 'Puis ouvrez la console depuis votre poste :' : 'Then reach the console from your machine:',
      code: 'kubectl port-forward svc/meerkat-meerkat-admin 9090:9090',
    },
  ];
  if (ee) {
    out.push({
      say: fr
        ? 'Pour un cluster de gateways sur PostgreSQL, les mêmes commandes avec cet autre fichier de valeurs :'
        : 'For a cluster of gateways on PostgreSQL, the same commands with this other values file:',
      file: 'values-ee-cluster.yaml',
    });
  }
  return out;
}

@Component({
  selector: 'app-install',
  template: `
    <div class="pick">
      <div class="row" role="group" [attr.aria-label]="w().edition">
        <span class="label">{{ w().edition }}</span>
        @for (e of editions; track e) {
          <button type="button" [class.on]="edition() === e" [attr.aria-pressed]="edition() === e" (click)="edition.set(e)">
            {{ w().editions[e] }}
          </button>
        }
      </div>
      <div class="row" role="group" [attr.aria-label]="w().platform">
        <span class="label">{{ w().platform }}</span>
        @for (p of platforms; track p) {
          <button type="button" [class.on]="platform() === p" [attr.aria-pressed]="platform() === p" (click)="platform.set(p)">
            {{ w().platforms[p] }}
          </button>
        }
      </div>
    </div>

    <p class="about">{{ w().about[edition()] }}</p>

    @for (s of steps(); track $index) {
      <p>
        {{ s.say }}
        @if (s.file) {
          <span class="file">{{ w().file }} : <a [href]="'deploy/' + s.file" download>{{ s.file }}</a></span>
        }
      </p>
      @if (s.code) {
        <div class="code">
          <pre><code [innerHTML]="paint(s.code)"></code></pre>
          <button type="button" class="copy" (click)="copy(s.code, $index)">
            {{ copied() === $index ? w().copied : w().copy }}
          </button>
        </div>
      }
    }
  `,
  styles: `
    :host {
      display: block;
      margin: 0 0 22px;
    }
    .pick {
      display: grid;
      gap: 10px;
      margin: 0 0 14px;
    }
    .row {
      display: flex;
      flex-wrap: wrap;
      align-items: center;
      gap: 8px;
    }
    .label {
      min-width: 92px;
      font-size: 0.86rem;
      color: var(--muted);
    }
    .row button {
      padding: 6px 14px;
      border: 1px solid var(--line);
      border-radius: 999px;
      background: transparent;
      color: var(--text);
      font: inherit;
      font-size: 0.9rem;
      cursor: pointer;
    }
    .row button.on {
      border-color: var(--accent);
      background: color-mix(in srgb, var(--accent) 14%, transparent);
      font-weight: 600;
    }
    .about {
      color: var(--muted);
    }
    .file {
      white-space: nowrap;
      margin-left: 6px;
    }
    .code {
      position: relative;
    }
    .copy {
      position: absolute;
      top: 8px;
      right: 8px;
      padding: 3px 10px;
      border: 1px solid var(--line);
      border-radius: 6px;
      background: var(--surface);
      color: var(--muted);
      font: inherit;
      font-size: 0.78rem;
      cursor: pointer;
    }
  `,
})
export class InstallComponent {
  readonly lang = input<Lang>('en');

  protected readonly editions: Edition[] = ['ce', 'eval', 'licensed'];
  protected readonly platforms: Platform[] = ['docker', 'swarm', 'k8s'];
  protected readonly edition = signal<Edition>('ce');
  protected readonly platform = signal<Platform>('docker');
  protected readonly copied = signal(-1);

  protected readonly w = computed(() => WORDS[this.lang()]);
  protected readonly steps = computed(() => steps(this.lang(), this.edition(), this.platform()));

  // The same colours as the blocks the build paints (styles.scss, .mk-sh-*),
  // from a few rules rather than a highlighter: these are shell commands this
  // file wrote itself, not arbitrary code. Escaped first - what goes into
  // innerHTML is only what the rules below wrap.
  protected paint(code: string): string {
    const esc = code.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
    return esc
      .split('\n')
      .map((line) =>
        line
          .replace(/('[^']*')/g, '<span class="mk-sh-str">$1</span>')
          .replace(/(^|\s)(--?[a-zA-Z][\w-]*)/g, '$1<span class="mk-sh-flag">$2</span>')
          .replace(/^(\s*)([A-Z_]+)(=)/, '$1<span class="mk-sh-var">$2</span>$3')
          .replace(/^(docker|helm|kubectl|curl)\b/, '<span class="mk-sh-cmd">$1</span>')
          .replace(/^(\s+)(docker)\b/, '$1<span class="mk-sh-cmd">$2</span>')
          .replace(/(\\)$/, '<span class="mk-sh-op">$1</span>'),
      )
      .join('\n');
  }

  protected copy(text: string, index: number): void {
    void navigator.clipboard?.writeText(text).then(() => {
      this.copied.set(index);
      setTimeout(() => this.copied.set(-1), 1500);
    });
  }
}
