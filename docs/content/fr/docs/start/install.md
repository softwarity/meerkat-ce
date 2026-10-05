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
commande.

## Vérifier la signature

Les trois images sont signées au moment de leur construction, avec
[cosign](https://docs.sigstore.dev/) et sans clé. Il n'y a donc aucune clé
publique à récupérer : la signature porte l'identité du workflow GitHub Actions
qui a produit l'image, et c'est cette identité que vous vérifiez.

```bash
cosign verify \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com \
  --certificate-identity-regexp '^https://github\.com/softwarity/meerkat-ce/\.github/workflows/' \
  docker.io/softwarity/meerkat:latest
```

La même commande vérifie l'image d'évaluation,
`docker.io/softwarity/meerkat:eval`, ainsi que l'image Enterprise sur
`ghcr.io/softwarity/meerkat` une fois que vous êtes authentifié auprès de ce
registre. Les trois sont construites par des workflows hébergés dans le miroir
public, `softwarity/meerkat-ce`, et c'est ce dépôt que nomme l'identité : les
sources Enterprise sont privées, la chaîne qui les construit ne l'est pas. La
commande affiche l'identité exacte qu'elle a acceptée ; une politique qui veut
épingler un fichier précis plutôt qu'un répertoire peut la reprendre de là.

Ce qui est signé, c'est le **digest**, et de façon récursive : la liste de
manifestes et chacune des images par architecture qu'elle référence. Deux
conséquences à connaître. Un client qui a récupéré l'image arm64 vérifie ce
qu'il exécute réellement, et non l'image voisine. Et une version publiée, qui
se contente de poser son numéro de version sur un digest déjà testé, sans rien
reconstruire, emporte la signature avec elle : rien n'est signé à nouveau,
puisque rien n'est reconstruit.

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
                    subject: "https://github.com/softwarity/meerkat-ce/.github/workflows/*"
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
| `-database-url` | `MEERKAT_DATABASE_URL` | vide | URL PostgreSQL, pour plusieurs gateways qui partagent une même installation |

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
| `-production` | `MEERKAT_PRODUCTION` | déclare cette gateway comme étant en production : tout ce qui est destiné aux développeurs reste fermé, quoi que disent les réglages stockés |
| `-plug-addr` | `MEERKAT_PLUG_ADDR` | adresse d'écoute du tunnel développeur, `:22222` par défaut, Enterprise uniquement |
| `-console-url` | `MEERKAT_CONSOLE_URL` | pour le développement : relaie la console vers un serveur de développement front |
| `-version` | - | affiche la version et quitte |

Si `MEERKAT_PRODUCTION` relève de l'environnement et non d'un réglage stocké,
c'est pour une raison précise : un réglage stocké voyage. Un export de
configuration, une sauvegarde restaurée ou une base copiée depuis la
préproduction peuvent chacun amener le mode développeur en production, et
personne ne s'en aperçoit avant qu'un port de tunnel se retrouve exposé aux
clients.

### Journaux et traces

| Option | Variable | Valeur par défaut | Rôle |
|---|---|---|---|
| `-log-level` | `MEERKAT_LOG_LEVEL` | `info` | `debug`, `info`, `warn` ou `error` ; une faute de frappe ramène à `info` au lieu d'empêcher le démarrage |
| `-log-format` | `MEERKAT_LOG_FORMAT` | vide | `json` ou `text` ; une valeur vide donne du JSON sur une gateway de production et du texte ailleurs |
| `-access-log` | `MEERKAT_ACCESS_LOG` | désactivé | une ligne par requête qui franchit la porte d'entrée, sur la sortie standard - voir [Journaux](/docs/operations/logs) |
| `-otlp-endpoint` | `MEERKAT_OTLP_ENDPOINT` | vide | un collecteur vers lequel exporter les traces dès la première seconde, par exemple `http://otel-collector:4318` (Enterprise) |
| `-otlp-sample` | `MEERKAT_OTLP_SAMPLE` | `0.1` | la proportion enregistrée des parcours ouverts par cette gateway, de 0 à 1 |

Le niveau de journalisation est fixé au démarrage ; le modifier impose un
redémarrage.

Les deux réglages `otlp` s'adressent à une gateway qui doit tracer avant
même que quelqu'un ait ouvert la console - une image lancée par un pipeline,
par exemple. Dans les autres cas, utilisez le réglage de la console (**Infra,
OpenTelemetry**) : il ajoute les métriques, l'identifiant d'accès rangé dans le
coffre, le choix route par route et le bouton Test. Tant que ce réglage n'a
jamais été activé, c'est l'export configuré au démarrage qui tourne ; dès qu'il
l'est, il prend le relais, et le désactiver ensuite arrête l'export jusqu'au
prochain démarrage. Voir [Traces](/docs/operations/tracing).

## Health checks

Les deux plans répondent aux deux health checks.

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
