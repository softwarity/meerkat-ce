---
title: Installation
section: Démarrer
order: 4
summary: Les images Docker, la construction du binaire, et les réglages que la gateway lit au démarrage.
---

# Installation

Meerkat est un binaire Go unique, sans CGO et sans dépendance externe. Son
stockage embarqué est un fichier dans un répertoire ; une base externe est une
option, pas un prérequis.

## Image Docker

Deux images, construites depuis le même commit, portant le même nom : c'est
le **registre** qui dit l'édition.

| Image | Édition |
|---|---|
| `docker.io/softwarity/meerkat:latest` | communautaire, publique |
| `ghcr.io/softwarity/meerkat:latest` | Enterprise, registre privé |

```yaml
services:
  meerkat:
    image: docker.io/softwarity/meerkat:latest
    ports:
      - "8080:8080" # plan de données
      - "9090:9090" # plan de contrôle - à garder interne
    environment:
      MEERKAT_ADMIN_PASSWORD: ${MEERKAT_ADMIN_PASSWORD:?set a password for the first admin}
    volumes:
      - meerkat-data:/data
    restart: unless-stopped

volumes:
  meerkat-data:
```

L'image pose `MEERKAT_DATA=/data` et déclare ce volume. Elle tourne sous un
utilisateur non privilégié (uid 65532), et le point d'entrée est le binaire
lui-même : tout ce qui suit le nom de l'image est donc un drapeau.

## Vérifier la signature

Les deux images sont signées à la construction, avec
[cosign](https://docs.sigstore.dev/) et sans clé. Il n'y a aucune clé publique
à aller chercher : la signature porte l'identité du workflow GitHub Actions qui
a produit l'image, et c'est cette identité que l'on vérifie.

```bash
cosign verify \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github\.com/softwarity/meerkat-ce/\.github/workflows/' \
  docker.io/softwarity/meerkat:latest
```

La même commande vérifie l'image Enterprise sur `ghcr.io/softwarity/meerkat`,
une fois connecté à ce registre. Les deux sont construites par des workflows qui
vivent dans le miroir public, `softwarity/meerkat-ce`, et c'est donc ce dépôt
que nomme l'identité : les sources Enterprise sont privées, la chaîne qui les
construit ne l'est pas. La commande affiche l'identité exacte qu'elle a
acceptée, de quoi épingler un fichier précis plutôt qu'un répertoire.

Ce qui est signé, c'est le **digest**, et récursivement : la liste de manifestes
plus chaque image par architecture qu'elle désigne. Deux conséquences utiles. Un
client qui a tiré l'image arm64 vérifie ce qu'il exécute vraiment, pas sa
voisine. Et une release, qui réétiquette sous son numéro de version un digest
déjà testé sans rien reconstruire, emporte la signature avec elle : rien n'est
re-signé puisque rien n'est reconstruit.

Pour refuser une image non signée à l'échelle d'un cluster plutôt que de
vérifier à la main, les deux mêmes valeurs vont dans une politique Kyverno :

```yaml
apiVersion: kyverno.io/v1
kind: ClusterPolicy
metadata:
  name: meerkat-signed
spec:
  validationFailureAction: Enforce
  rules:
    - name: verify-meerkat
      match:
        any:
          - resources:
              kinds: [Pod]
      verifyImages:
        - imageReferences:
            - "docker.io/softwarity/meerkat*"
            - "ghcr.io/softwarity/meerkat*"
          attestors:
            - entries:
                - keyless:
                    issuer: https://token.actions.githubusercontent.com
                    subject: "https://github.com/softwarity/meerkat-ce/.github/workflows/*"
```

Vérifiez `validationFailureAction` contre votre version de Kyverno : les
récentes ont déplacé cet interrupteur dans la règle, et une politique qui
atterrit en mode audit signale au lieu de refuser.

## Construire le binaire

Ce qui est publié, ce sont les images ; le binaire se construit depuis les
sources.

```bash
git clone https://github.com/softwarity/meerkat-ce.git
cd meerkat-ce
make ui      # construit la console et la prepare pour l'embarquement (Node requis)
make build   # -> bin/meerkat
./bin/meerkat --help
```

`make ui` est ce qui met la console **dans** le binaire. Sautez-le et la gateway
route toujours, mais le plan de contrôle répond une page d'état JSON au lieu
d'une console.

La version de Go vient de `go.mod`, celle de Node de `.node-version`. Ce dépôt
est l'arbre communautaire : les sources Enterprise n'y sont pas, donc ce qu'il
construit est l'édition communautaire. Voir [Éditions](/product/editions).

## Ce qu'elle lit au démarrage

Chaque drapeau a un équivalent `MEERKAT_*`, et le drapeau gagne. `./bin/meerkat
--help` en donne la liste ; la voici.

### Ports

| Drapeau | Variable | Défaut | Quoi |
|---|---|---|---|
| `-addr` | `MEERKAT_ADDR` | `:8080` | plan de données, HTTP en clair |
| `-admin-addr` | `MEERKAT_ADMIN_ADDR` | `:9090` | plan de contrôle, HTTP en clair |
| `-tls-addr` | `MEERKAT_TLS_ADDR` | `:8443` | HTTPS du plan de données, ouvert quand TLS est activé |
| `-admin-tls-addr` | `MEERKAT_ADMIN_TLS_ADDR` | `:9443` | HTTPS du plan de contrôle, pareil |

Les ports HTTPS s'ouvrent et se ferment à chaud, depuis la console. Un échec y
est journalisé et n'arrête pas les ports en clair : une faute de frappe dans une
adresse coûte une porte, pas l'installation.

### Stockage

| Drapeau | Variable | Défaut | Quoi |
|---|---|---|---|
| `-data` | `MEERKAT_DATA` | `data` | répertoire du stockage embarqué |
| `-database-url` | `MEERKAT_DATABASE_URL` | vide | URL PostgreSQL pour plusieurs gateways servant une seule installation |

> [!WARNING]
> Le défaut de `-data` est **relatif**. Sous Docker l'image le pose déjà à
> `/data` ; ailleurs, donnez un chemin absolu, sinon la base atterrit là où le
> processus a été lancé.

> [!NOTE]
> Édition Enterprise. Le pilote PostgreSQL n'est compilé que dans l'image
> Enterprise. Le binaire communautaire n'a aucun pilote enregistré :
> `MEERKAT_DATABASE_URL` y reçoit une phrase qui nomme ce qui est disponible,
> plutôt qu'une erreur de pilote.

### Au premier démarrage seulement

| Variable | Quoi |
|---|---|
| `MEERKAT_ADMIN_PASSWORD` | le mot de passe du premier administrateur, lu seulement tant qu'aucun compte n'existe |
| `MEERKAT_CONFIG_FILE` (`-config`) | une configuration YAML ou JSON qui amorce une gateway **vide**. Une gateway configurée ne l'applique pas : quand le fichier diffère de ce qui tourne, il est rangé comme **configuration enregistrée** (nommée d'après le fichier), à comparer et à rendre courante depuis l'écran Configuration |
| `MEERKAT_VAULT_FILE` (`-vault`) | un fichier de coffre chiffré, ingéré une seule fois |
| `MEERKAT_VAULT_PASSPHRASE`, `MEERKAT_VAULT_PASSPHRASE_FILE` | la phrase secrète de ce fichier |
| `MEERKAT_TENANCY` (`-tenancy`) | `single` ou `multi`, tranché au premier démarrage ; ensuite la console est maîtresse du mode |

Sans fichier de configuration, la gateway démarre vide - aucune route - et le
compte administrateur est la seule chose qu'elle crée.

`-tenancy` est fait pour l'amorçage - un premier démarrage, une installation
livrée avec sa configuration, du GitOps. Une fois le mode choisi, le drapeau est
ignoré, avec une ligne dans le journal qui le dit plutôt qu'un remplacement
silencieux. Demander `multi` sur une image communautaire démarre en
mono-organisation, et le dit aussi.

### Interrupteurs d'exploitation

| Drapeau | Variable | Quoi |
|---|---|---|
| `-production` | `MEERKAT_PRODUCTION` | déclare cette gateway de production : toute la surface développeur reste fermée quoi que disent les réglages stockés |
| `-plug-addr` | `MEERKAT_PLUG_ADDR` | adresse d'écoute du tunnel développeur, `:22222` par défaut, Enterprise seulement |
| `-console-url` | `MEERKAT_CONSOLE_URL` | contournement de développement : proxifier la console vers un serveur de dev front |
| `-version` | - | afficher la version et sortir |

`MEERKAT_PRODUCTION` est une décision d'environnement et non un réglage stocké,
pour une raison : le réglage stocké voyage. Un export de configuration, une
sauvegarde restaurée ou une base copiée depuis la recette peuvent chacun
emporter le mode développeur en production, et personne ne s'en aperçoit avant
qu'il y ait un port de tunnel devant des clients.

### Journaux et traces

| Drapeau | Variable | Défaut | Quoi |
|---|---|---|---|
| `-log-level` | `MEERKAT_LOG_LEVEL` | `info` | `debug`, `info`, `warn` ou `error` ; une faute de frappe retombe sur `info` plutôt que d'empêcher le démarrage |
| `-log-format` | `MEERKAT_LOG_FORMAT` | vide | `json` ou `text` ; vide choisit JSON sur une gateway de production et texte ailleurs |
| `-access-log` | `MEERKAT_ACCESS_LOG` | éteint | une ligne par requête franchissant la porte d'entrée, sur la sortie standard - voir [Journaux](/docs/operations/logs) |
| `-otlp-endpoint` | `MEERKAT_OTLP_ENDPOINT` | vide | un collecteur vers lequel exporter les traces dès la première seconde, par exemple `http://otel-collector:4318` (Enterprise) |
| `-otlp-sample` | `MEERKAT_OTLP_SAMPLE` | `0.1` | la part des parcours ouverts par cette gateway qui sont enregistrés, de 0 à 1 |

Le niveau est fixé au démarrage ; le changer demande un redémarrage.

Les deux réglages `otlp` servent à une gateway qui doit tracer avant que
quiconque ait ouvert la console - une image lancée par une chaîne de
déploiement, par exemple. Sinon c'est le réglage de la console (**Infra,
OpenTelemetry**) qu'il faut prendre : il ajoute les métriques, la crédentiale au
coffre, le choix route par route et le bouton Test. Tant qu'il n'a jamais été allumé,
l'exportateur du démarrage tourne ; une fois allumé, il prend la main, et
l'éteindre de nouveau arrête l'export jusqu'au prochain démarrage. Voir
[Traces](/docs/operations/tracing).

## Sondes

Les deux plans répondent aux deux sondes.

| Chemin | Réponse |
|---|---|
| `/healthz` | vivacité - UP inconditionnellement, parce que la vivacité décide s'il faut tuer le processus |
| `/readyz` | disponibilité - 503 avec le motif quand le stockage ne répond pas, ou quand la table de routage n'est pas encore compilée |

> [!WARNING]
> Faites pointer votre répartiteur sur `/readyz`, jamais sur `/healthz`. Une
> sonde de vivacité qui échouerait parce que la base est injoignable
> transformerait un hoquet de base de données en redémarrage simultané de tous
> les noeuds.

## Signaux

- `SIGHUP` recharge les routes.
- `SIGINT` et `SIGTERM` arrêtent la gateway, avec un drainage de dix secondes au plus.
