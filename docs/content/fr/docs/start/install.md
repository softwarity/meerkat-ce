---
title: Installation
section: Démarrer
order: 4
summary: Les images Docker, la construction du binaire par vos soins, et les réglages que la gateway lit au démarrage.
---

# Installation

Meerkat tient en un seul binaire Go, sans CGO ni dépendance externe. Son
stockage embarqué est un fichier dans un répertoire ; une base de données
externe est une option, pas un prérequis.

## Image Docker

Trois images, construites à partir du même commit et portant le même nom : ce
sont le **registre** et le tag qui indiquent l'édition.

| Image | Édition |
|---|---|
| `docker.io/softwarity/meerkat:latest` | Community, publique |
| `docker.io/softwarity/meerkat:eval` | évaluation, publique : tout ce que fait Enterprise, avec une mention d'évaluation - pas pour la production |
| `ghcr.io/softwarity/meerkat:latest` | Enterprise, registre privé |

```yaml
services:
  meerkat:
    image: docker.io/softwarity/meerkat:latest
    ports:
      - "8080:8080" # plan de données
      - "9090:9090" # plan de contrôle - à garder en interne
    environment:
      MEERKAT_ADMIN_PASSWORD: ${MEERKAT_ADMIN_PASSWORD:?set a password for the first admin}
    volumes:
      - meerkat-data:/data
    restart: unless-stopped

volumes:
  meerkat-data:
```

L'image définit `MEERKAT_DATA=/data` et déclare ce volume. Elle s'exécute sous
un utilisateur non root (uid 65532), et son point d'entrée est le binaire
lui-même : tout ce qui suit le nom de l'image est donc une option de ligne de
commande. Elle est distroless : ni shell ni `wget`, donc `docker exec ... sh` n'a
rien à ouvrir, et un healthcheck lance `/meerkat health`
([Health checks](/docs/operations/health#depuis-lintrieur-du-conteneur)).

## Vérifier la signature

Les trois images sont signées au moment de leur construction, avec
[cosign](https://docs.sigstore.dev/) et sans clé. Il n'y a donc aucune clé
publique à récupérer : la signature porte l'identité du workflow GitHub Actions
qui a produit l'image, et c'est cette identité que vous vérifiez.

```bash
cosign verify \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github\.com/softwarity/meerkat-ce/\.github/workflows/(ci-ee|_docker)\.yml@refs/heads/main$' \
  docker.io/softwarity/meerkat:latest
```

La même commande vérifie l'image d'évaluation,
`docker.io/softwarity/meerkat:eval`, ainsi que l'image Enterprise sur
`ghcr.io/softwarity/meerkat` une fois que vous êtes authentifié auprès de ce
registre. Les trois sont construites par des workflows hébergés dans le miroir
public, `softwarity/meerkat-ce`, et c'est ce dépôt que nomme l'identité : les
sources Enterprise sont privées, la chaîne qui les construit ne l'est pas.
L'identité nomme deux fichiers de ce dépôt, sur `main` et nulle part ailleurs :
`_docker.yml` construit le `latest` communautaire, `ci-ee.yml` les images
Enterprise et d'évaluation ainsi que chaque tag de version. Un workflow d'une
autre branche, ou un autre fichier, ne passe pas. Les versions jusqu'à 1.6.0
ne portent pas de signature : seuls `latest` et `eval` étaient signés alors.

Ce qui est signé, c'est le **digest**, et de façon récursive : la liste de
manifestes et chacune des images par architecture qu'elle référence. Deux
conséquences à connaître. Un client qui a récupéré l'image arm64 vérifie ce
qu'il exécute réellement, et non l'image voisine. Et une version publiée, qui
pose son numéro sur une image déjà testée sans la reconstruire (une variable
d'environnement et un label sur les mêmes couches), produit un nouveau
manifeste : elle le signe à son tour, et échoue si l'un des tags qu'elle a
poussés ne se vérifie pas.

Pour refuser les images non signées à l'échelle d'un cluster, plutôt que de les
vérifier une par une à la main, reportez ces deux mêmes valeurs dans une
politique Kyverno :

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
                    subjectRegExp: '^https://github\.com/softwarity/meerkat-ce/\.github/workflows/(ci-ee|_docker)\.yml@refs/heads/main$'
```

Vérifiez `validationFailureAction` au regard de votre version de Kyverno : les
versions récentes ont déplacé ce réglage dans la règle, et une politique qui se
retrouve en mode audit signale au lieu de refuser.

## Construire le binaire vous-même

Seules les images sont publiées ; le binaire, lui, se construit à partir des
sources.

```bash
git clone https://github.com/softwarity/meerkat-ce.git
cd meerkat-ce
make ui      # construit la console et la prépare pour l'embarquer (Node requis)
make build   # -> bin/meerkat
./bin/meerkat --help
```

C'est `make ui` qui place la console **dans** le binaire. Sans cette étape, la
gateway route toujours, mais le plan de contrôle renvoie une page d'état en
JSON à la place de la console.

La version de Go est celle de `go.mod`, celle de Node est dans `.node-version`.
Ce dépôt est l'arbre Community : les sources Enterprise n'y figurent pas, et ce
qu'il construit est donc l'édition Community. Voir
[Éditions](/product/editions).

## Ce que la gateway lit au démarrage

Chaque option de ligne de commande a son équivalent `MEERKAT_*`, et c'est
l'option qui l'emporte. `./bin/meerkat --help` en affiche la liste ; la voici.

### Ports

| Option | Variable | Valeur par défaut | Rôle |
|---|---|---|---|
| `-addr` | `MEERKAT_ADDR` | `:8080` | plan de données, HTTP en clair |
| `-admin-addr` | `MEERKAT_ADMIN_ADDR` | `:9090` | plan de contrôle, HTTP en clair |
| `-internal-addr` | `MEERKAT_INTERNAL_ADDR` | `:7070` | plan interne, HTTP en clair : ce qu'appellent les services placés derrière la gateway, jamais publié |
| `-internal-tls-addr` | `MEERKAT_INTERNAL_TLS_ADDR` | `:7443` | HTTPS du plan interne, ouvert quand un certificat y est placé |
| `-tls-addr` | `MEERKAT_TLS_ADDR` | `:8443` | HTTPS du plan de données, ouvert quand TLS est activé |
| `-admin-tls-addr` | `MEERKAT_ADMIN_TLS_ADDR` | `:9443` | HTTPS du plan de contrôle, de même |

Les ports HTTPS s'ouvrent et se ferment pendant que la gateway tourne, depuis
la console. Un échec à ce niveau est journalisé et n'arrête pas les ports en
clair : une faute de frappe dans une adresse coûte une porte, pas
l'installation.

### Stockage

| Option | Variable | Valeur par défaut | Rôle |
|---|---|---|---|
| `-data` | `MEERKAT_DATA` | `data` | répertoire du stockage embarqué |
| `-database-url` | `MEERKAT_DATABASE_URL` | vide | URL PostgreSQL, pour plusieurs instances de la gateway qui partagent une même installation |

> [!WARNING]
> La valeur par défaut de `-data` est un chemin **relatif**. Sous Docker,
> l'image le fixe déjà à `/data` ; partout ailleurs, indiquez un chemin absolu,
> sans quoi la base se retrouve dans le répertoire d'où le processus a été
> lancé.

> [!NOTE]
> Édition Enterprise. Le pilote PostgreSQL n'est compilé que dans l'image
> Enterprise. Le binaire Community n'enregistre aucun pilote : si vous y
> définissez `MEERKAT_DATABASE_URL`, il répond par une phrase qui indique ce
> qui est disponible, et non par une erreur de pilote.

### Au premier démarrage uniquement

| Variable | Rôle |
|---|---|
| `MEERKAT_ADMIN_PASSWORD` | le mot de passe du premier administrateur, lu uniquement tant qu'il n'existe aucun compte |
| `MEERKAT_CONFIG_FILE` (`-config`) | une configuration YAML ou JSON qui initialise une gateway **vide**. Une gateway déjà configurée ne l'applique pas : si le fichier diffère de ce qui tourne, il est mis de côté comme **configuration enregistrée** (au nom du fichier), que vous pouvez comparer et rendre courante depuis l'écran Configuration |
| `MEERKAT_VAULT_FILE` (`-vault`) | un fichier de coffre chiffré, importé une seule fois |
| `MEERKAT_VAULT_PASSPHRASE`, `MEERKAT_VAULT_PASSPHRASE_FILE` | la phrase secrète de ce fichier |
| `MEERKAT_TENANCY` (`-tenancy`) | `single` ou `multi`, fixé au premier démarrage ; ensuite, c'est la console qui en décide |

Sans fichier de configuration, la gateway démarre vide - sans aucune route -
et ne crée rien d'autre que le compte de l'administrateur.

`-tenancy` sert à l'amorçage : un premier démarrage, une installation
préconfigurée, du GitOps. Une fois le mode choisi, l'option est ignorée, et une
ligne du journal le signale : la valeur n'est jamais remplacée en silence.
Demander `multi` sur une image Community démarre avec une seule organisation,
et le journal le dit aussi.

### Options d'exploitation

| Option | Variable | Rôle |
|---|---|---|
| `-plug-addr` | `MEERKAT_PLUG_ADDR` | adresse d'écoute du tunnel développeur, `:22222` par défaut, Enterprise uniquement |
| `-license` | `MEERKAT_LICENSE_FILE` | le fichier de licence envoyé par Softwarity, monté depuis un secret (`license.existingSecret` dans le chart Helm). Lu au démarrage puis à chaque consultation : un secret mis à jour sur place ne demande aucun redémarrage ; l'écran License l'affiche alors et n'en accepte pas d'autre. Image Enterprise uniquement |
| `-console-url` | `MEERKAT_CONSOLE_URL` | pour le développement : relaie la console vers un serveur de développement front |
| `-version` | - | affiche la version et quitte |

La surface développeur - l'outillage, la documentation des API, le mode de
test des interfaces, le tunnel - s'active dans la console (**Application,
General** et **Infra, Plug**), sur une gateway de production comme ailleurs.
`MEERKAT_PRODUCTION`, qui la fermait depuis l'environnement, n'est plus lue :
posée, elle ne fait qu'écrire un avertissement dans le journal.

### Journaux et traces

| Option | Variable | Valeur par défaut | Rôle |
|---|---|---|---|
| `-log-level` | `MEERKAT_LOG_LEVEL` | `info` | `debug`, `info`, `warn` ou `error` ; une faute de frappe ramène à `info` au lieu d'empêcher le démarrage |
| `-log-format` | `MEERKAT_LOG_FORMAT` | text | `json` ou `text` : ce qui s'écrit avant que la gateway ait lu ses réglages. Le JSON pour un agent est l'interrupteur de la console, **Infra, OpenTelemetry**, journaux écrits pour un agent |

Ce niveau est celui auquel revient un démarrage ; l'écran **Logs** de la
console le change à chaud, pour un temps, sur tous les nœuds.

Le journal d'accès, l'export des traces, des métriques et des journaux vers un
collecteur, et le format des journaux pour un agent se règlent dans la console,
**Infra, OpenTelemetry** : stockés, appliqués à chaque démarrage et à chaud sur
tous les nœuds. `MEERKAT_ACCESS_LOG`, `MEERKAT_OTLP_ENDPOINT` et
`MEERKAT_OTLP_SAMPLE` ne sont plus lues : posée, chacune ne fait qu'écrire un
avertissement. Voir [Traces](/docs/operations/tracing) et
[Journaux](/docs/operations/logs).

## Health checks

Le plan de données et le plan de contrôle répondent aux deux health checks ; le plan interne répond à `/healthz`.

| Chemin | Réponse |
|---|---|
| `/healthz` | vivacité (liveness) - UP sans condition, car ce health check décide s'il faut tuer le processus |
| `/readyz` | disponibilité (readiness) - 503 accompagné d'un motif quand le stockage ne répond pas, ou quand la table de routage n'est pas encore compilée |

> [!WARNING]
> Faites pointer votre load balancer sur `/readyz`, jamais sur
> `/healthz`. Une liveness probe qui échouerait dès que la base est
> injoignable transformerait un simple incident de base de données en
> redémarrage de tous les nœuds à la fois.

## Signaux

- `SIGHUP` recharge les routes.
- `SIGINT` et `SIGTERM` arrêtent la gateway, en laissant jusqu'à dix secondes aux requêtes en cours pour se terminer.
