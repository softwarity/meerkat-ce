---
title: Plateforme de test
section: Qualité
order: 33
summary: Comment l'image se prouve là où elle sera déployée : compose, Swarm, Kubernetes, en instance seule et en cluster PostgreSQL, en CE et en EE.
---

# Plateforme de test

Les suites d'aujourd'hui prouvent le **code** : 1 137 tests Go, une suite PostgreSQL,
trois annuaires réels (Dex, OpenLDAP, Samba AD) et 87 scénarios Playwright joués
contre un binaire construit depuis l'arbre. Aucune ne prouve l'**image telle qu'on la
livre**, déployée par **les fichiers qu'on publie**, sur les cibles où elle tourne
vraiment. La plateforme de test comble cet écart. Ce que chaque fonction doit y
prouver est sur [Plan de test](/project/test-plan) ; cette page dit comment.

## Le principe

1. **On teste l'image, pas l'arbre.** Une image par édition est construite une fois,
   sous un tag immuable (`sha-<court>`), et c'est ce digest qui part sur chaque cible.
   Les tags mobiles ne se posent qu'une fois tout vert, et le verdict se lit dans des
   fichiers marqueurs, pas dans les `needs` : un job sauté satisfait un `needs`.
2. **On déploie avec ce qu'on publie.** `deploy/docker-compose.yml`, `.ee.yml`,
   `stack.swarm.yml` et le chart Helm avec ses trois fichiers de valeurs, réécrits
   seulement pour le tag d'image, et la réécriture est vérifiée avant d'appliquer.
   Une copie faite à la main pour les tests dérive : plug l'a payé de huit releases.
3. **Une suite, plusieurs cibles.** La suite Playwright et `scenarios.json` restent
   la source ; la plateforme les pointe sur une passerelle déployée au lieu d'en
   démarrer une. Ce qui n'est pas un scénario (tuer un nœud, monter de version) est
   une **cellule** : un script, un nom, une étape à elle dans le job, rouge ou verte.
4. **CE et EE jouent le même fichier.** Chaque édition attend ses propres réponses :
   l'EE réussit là où la CE répond 422 ou affiche le verrou. Une fonction EE qui
   passerait en CE est un échec, au même titre qu'une fonction CE qui casse.

## Les cibles

| | Cible | Topologie | Éditions | Base |
|---|---|---|---|---|
| `C1` | docker compose | une passerelle + les services de test | CE, EE | SQLite (volume) |
| `CC` | docker compose cluster | 3 passerelles derrière Traefik | EE | PostgreSQL 17 |
| `S1` | Docker Swarm | une réplique, `stack deploy` | CE, EE | SQLite |
| `SC` | Docker Swarm | 3 réplicas | EE | PostgreSQL |
| `K1` | kind + Helm | `values-ce-one-node` / `values-ee-one-node` | CE, EE | SQLite (PVC) |
| `KC` | kind + Helm | `values-ee-cluster`, 3 réplicas | EE | PostgreSQL |
| `KO` | kind aux règles d'OpenShift | `values-*-one-node` inchangés, Pod Security `restricted`, uid arbitraire + groupe 0 | CE, EE | SQLite (PVC) |
| `OK` | MicroShift (OKD) dans un conteneur | la vraie SCC `restricted-v2`, exposé par des Routes | CE, EE | SQLite (emptyDir) |

Swarm et kind tournent sur un seul nœud du runner (`swarm init --advertise-addr
127.0.0.1`, kind avec `extraPortMappings` vers des ports fixes de l'hôte) : on teste
l'orchestrateur et nos fichiers, pas un réseau multi-hôte. kind est épinglé et vérifié
par SHA256, l'image chargée par `kind load` avec `imagePullPolicy: Never` ; on attend
chaque déploiement par son propre `kubectl rollout status`, et on vide `describe` et
les logs en cas d'échec.

### OKD / OpenShift

OpenShift ne lance pas un pod sous l'uid de l'image : la SCC `restricted` en tire un
de la plage du namespace, avec le groupe 0, et refuse un pod qui fixe le sien. C'est
pour cela que le chart ne pose ni `runAsUser` ni `fsGroup`. Deux façons de le prouver,
de la moins chère à la plus fidèle :

1. **`KO`, sur kind, à chaque nuit** : le namespace étiqueté
   `pod-security.kubernetes.io/enforce=restricted`, et une surcouche de test qui fait
   ce que fait la SCC (uid `1000680000`, groupe 0). Si la passerelle écrit `/data`,
   migre, sert et rend le snapshot, elle tournera sous OpenShift. S'y ajoute un
   contrôle statique du chart rendu, à chaque push : aucun uid fixé, rien de privilégié,
   aucun port sous 1024.
2. **`OK`, MicroShift, chaque nuit** : la distribution réduite d'OpenShift, construite
   depuis OKD, un nœud dans un conteneur privilégié - elle tient sur un runner GitHub
   standard. C'est ce que kind ne sait pas imiter : la SCC `restricted-v2` qui ADMET le
   pod et lui donne son uid, et le routeur derrière des Routes. `e2e/platform/okd/run.sh
   <image> <ce|ee>` y installe le chart publié, vérifie la SCC, l'uid, l'écriture de
   `/data`, les deux plans derrière leurs Routes, et - en Enterprise - un helper de
   montage tel que plug le crée, avec un vrai client SMB. Le workflow `okd.yml` le joue
   pour les deux images. Une chose est coupée : la demande de volume, parce que le
   pilote de stockage de MicroShift veut un groupe de volumes LVM sur l'hôte.

## Les services de test

Tout vit sur le réseau de la plateforme, rien n'est joint sur internet : un verdict
sur le site de quelqu'un d'autre n'en est pas un sur le produit.

| Service | Image | Pour |
|---|---|---|
| httpbin | `mccutchen/go-httpbin` | l'amont des routes : en-têtes, délais, statuts, flux |
| echo WebSocket et gRPC | petit binaire Go dans `e2e/platform/services` | ROUTE-13, ROUTE-20 (unaire et flux, h2c et TLS) |
| Mailpit | `axllent/mailpit` | le puits SMTP, lu par son API HTTP (code par e-mail, mot de passe oublié, digest) |
| Dex | `dexidp/dex` | OIDC |
| OpenLDAP, Samba AD | les images de la CI EE | LDAP / AD et les règles de groupe (EE) |
| PostgreSQL | `postgres:17` | les cibles cluster |
| Traefik | `traefik` | le répartiteur devant le cluster compose, et le reverse proxy de X-17 |
| Pebble + challtestsrv | `ghcr.io/letsencrypt/pebble` | ACME (EE), voir plus bas |
| Gitea | `gitea/gitea` | les emplacements git (EE) |
| OpenTelemetry Collector | `otel/opentelemetry-collector-contrib`, exporteur fichier | audit, traces, métriques (EE) |

## Les phases

Chaque phase livre quelque chose qui tourne et qui attrape déjà des régressions ; la
suivante ne commence qu'une fois la précédente verte deux nuits de suite.

| Phase | Livre | Cibles | Coût estimé |
|---|---|---|---|
| **0. Suite indépendante de la cible** | `DATA_URL`, `ADMIN_URL`, le mot de passe root et le puits mail lus depuis l'environnement ; les `webServer` sautés quand une cible est donnée ; le seed qui ne suppose plus un disque local | `I` | une demi-journée |
| **1. L'image en compose** | `e2e/platform/compose/` (services de test en surcouche des fichiers publiés), cellules `smoke`, `edition`, `matrix` (la suite entière contre l'image), en CE et en EE | `C1` | deux jours |
| **2. Le cluster** | 3 passerelles + PostgreSQL + Traefik ; cellules `sticky-free` (une session sur A, servie par B), `bus` (route posée sur A, servie par C), `once` (planification, digest, purge), `kill-node`, `pg-restart`, `vault-key-mismatch` | `CC` | trois jours |
| **3. Kubernetes** | kind + chart, les trois fichiers de valeurs ; `helm upgrade --reuse-values`, mise à jour roulante sous trafic, découverte des Services ; les règles d'OpenShift sur kind et le contrôle statique du chart | `K1`, `KC`, `KO` | quatre jours |
| **4. Swarm** | `stack deploy` du fichier publié, secrets et configs, convergence, mise à jour start-first | `S1`, `SC` | un jour |
| **5. Montée de version** | la release précédente trouvée (pas épinglée), déployée et peuplée, puis remplacée par l'image du commit ; une base de référence par release ; le retour refusé | `C1`, `CC`, `K1` | deux jours |
| **6. Les intégrations EE** | ACME contre Pebble, git contre Gitea, OpenTelemetry vers le Collector | `C1`, `CC` | trois jours |
| **7. Endurance et arm64** | 4 heures de trafic mixte chaque semaine ; la fumée sur runner arm64 natif | `CC`, `C1` | deux jours |

### Pourquoi la phase 0 d'abord

`e2e/playwright.config.ts` écrit en dur `localhost:18082` et `localhost:19092`,
démarre la passerelle, httpbin et le puits SMTP, et le flux d'inscription lit son
mail dans `.tmp/mail` sur le disque. Tant que c'est vrai, la suite ne peut viser
qu'un processus local. Une fois ces quatre points lus depuis l'environnement, la
même suite, sans une ligne de plus, joue contre n'importe quelle cible : c'est
l'essentiel de la valeur pour le moins de travail.

## ACME, la phase la plus difficile

Pebble est le serveur ACME de test de Let's Encrypt : il émet de vrais certificats
signés par une CA jetable, et `pebble-challtestsrv` lui sert de DNS. Trois obstacles,
tous franchissables :

1. **La confiance.** Le répertoire de Pebble est en HTTPS sous sa propre CA. La
   passerelle doit la croire : `SSL_CERT_FILE` sur le conteneur suffit en Go sous Linux,
   sans rien changer au produit.
2. **Le défi.** Meerkat valide en TLS-ALPN-01, donc Pebble doit joindre la porte
   HTTPS de l'application **sur le port 443** pour chaque nom demandé. challtestsrv
   résout ces noms vers la passerelle, et la cible publie l'application sur 443 dans
   le réseau de la plateforme (`PEBBLE_VA_ALWAYS_VALID=1` permet d'écrire la cellule
   avant d'avoir le réseau juste, puis on l'enlève).
3. **Le cluster.** Trois nœuds, une commande : le certificat doit être émis **une
   fois**, sous le verrou consultatif, et servi par les trois. C'est le test qui
   compte, et le seul qu'aucun test unitaire ne peut faire.

La cellule enchaîne : enregistrer l'autorité Pebble, créer une commande placée sur
l'application, attendre le statut émis, lire le certificat servi par chaque nœud,
puis forcer le renouvellement (durée de vie courte côté Pebble). En CE, la même
cellule vérifie le refus.

## La CI

| Quand | Quoi | Où |
|---|---|---|
| chaque push | phase 0 et `C1` en fumée, CE et EE | `ci.yml` (meerkat-ce) et `ci-ee.yml` |
| chaque nuit sur `main` | toutes les cibles, la suite entière, la montée de version | un workflow `platform.yml` planifié, et lançable à la main |
| avant une release | la nuit précédente doit être verte ; sinon on la relance | la porte de `ci-ee.yml` lit les marqueurs |
| chaque semaine | l'endurance 4 h | `soak.yml` |

Ce qu'on garde de plug :

- chaque cellule est une étape du job, avec son propre rouge ;
- un script vérifie que l'ordre des cellules, leurs fichiers et les noms de jobs
  concordent, et qu'aucune ne porte `continue-on-error` ;
- une cellule sautée est comptée et levée en avertissement, jamais affichée comme
  réussie ;
- chaque appel est borné par un chien de garde plafonné au temps restant du job
  (un job tué par son timeout perd son journal) ;
- un job `abort-on-fail` annule le reste dès qu'une jambe échoue, pour que la
  rouge reste rouge au lieu de devenir « annulée » ;
- connexion à Docker Hub même pour tirer, à cause des limites de débit.

Ce qu'on ne reprend pas : le tailnet. Les cibles de Meerkat tournent dans le job
même qui les teste ; il n'y a pas de client sur d'autres systèmes à faire joindre.

## La disposition dans le dépôt

```
e2e/platform/
  plan.json            le plan, une entrée par ligne de FEATURES.md + les contrôles X-xx
  run.sh               <cible> <édition> <cellule...>
  lib.sh               attentes, bornes, résumé de job
  cells/               une cellule par fichier : smoke.sh, edition.sh, kill-node.sh...
  compose/             surcouches des fichiers publiés : services de test, cluster
  kind/                kind-config.yaml, valeurs de test
  swarm/               surcouche de stack
  services/            echo WebSocket et gRPC
```

## Tenir le plan à jour

`plan.json` se maintient à côté de `FEATURES.md`. **Une fonction ajoutée là ajoute
son entrée ici dans le même commit** ; si on l'oublie, la page
[Plan de test](/project/test-plan) la liste en bas, sous « Dans FEATURES.md et pas
encore dans le plan ». La colonne « Tests qui la citent aujourd'hui » est lue dans
les sources de test : une fonction qu'aucun test ne nomme est une fonction que
personne n'a vérifiée, ce qui est exactement ce qu'elle sert à montrer.
