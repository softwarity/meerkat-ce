---
title: Plateforme de test
section: Qualité
order: 33
summary: Comment l'image fait ses preuves là où elle sera déployée : compose, Swarm, Kubernetes, en instance unique et en cluster PostgreSQL, en édition Community et en édition Enterprise.
---

# Plateforme de test

Les suites actuelles prouvent le **code** : 1 137 tests Go, une suite PostgreSQL,
trois vrais annuaires (Dex, OpenLDAP, Samba AD) et 87 scénarios Playwright joués
contre un binaire construit à partir des sources. Aucune ne prouve
l'**image telle qu'elle est livrée**, déployée avec **les fichiers que nous
publions**, sur les cibles où elle tourne réellement. La plateforme de test comble
ce manque. Ce que chaque fonctionnalité doit y prouver figure sur la page
[Plan de test](/project/test-plan) ; celle-ci explique comment.

## Le principe

1. **Tester l'image, pas les sources.** Une image par édition est construite une
   seule fois, sous un tag immuable (`sha-<short>`), et c'est ce digest qui part
   sur chaque cible. Les tags mobiles ne sont posés qu'une fois tout au vert, et le
   verdict se lit dans des fichiers marqueurs, pas dans les `needs` : un job ignoré
   satisfait un `needs`.
2. **Déployer avec ce que nous publions.** `deploy/docker-compose.yml`, `.ee.yml`,
   `stack.swarm.yml` et le chart Helm avec ses trois fichiers de valeurs, réécrits
   uniquement pour le tag de l'image - et cette réécriture est vérifiée avant toute
   application. Une copie faite à la main pour les tests finit par diverger : plug
   l'a payé pendant huit versions.
3. **Une suite, plusieurs cibles.** La suite Playwright et `scenarios.json` restent
   la source ; la plateforme les dirige vers une gateway déjà déployée au lieu
   d'en démarrer une. Ce qui n'est pas un scénario (tuer un nœud, monter de
   version) est une **cellule** : un script, un nom, une étape à part entière dans
   le job, rouge ou verte.
4. **Community et Enterprise jouent le même fichier.** Chaque édition attend ses
   propres réponses : Enterprise réussit là où Community répond 422 ou affiche le
   cadenas. Une fonctionnalité Enterprise qui marche sur l'image Community est un
   échec, au même titre qu'une fonctionnalité Community qui casse.

## Les cibles

| | Cible | Topologie | Éditions | Base de données |
|---|---|---|---|---|
| `C1` | docker compose | une gateway + les services de test | CE, EE | SQLite (volume) |
| `CC` | cluster docker compose | 3 gateways derrière Traefik | EE | PostgreSQL 17 |
| `S1` | Docker Swarm | une réplique, `stack deploy` | CE, EE | SQLite |
| `SC` | Docker Swarm | 3 répliques | EE | PostgreSQL |
| `K1` | kind + Helm | `values-ce-one-node` / `values-ee-one-node` | CE, EE | SQLite (PVC) |
| `KC` | kind + Helm | `values-ee-cluster`, 3 répliques | EE | PostgreSQL |
| `KO` | kind soumis aux règles d'OpenShift | `values-*-one-node` inchangés, Pod Security `restricted`, uid arbitraire + groupe 0 | CE, EE | SQLite (PVC) |
| `OK` | MicroShift (OKD) dans un conteneur | la vraie SCC `restricted-v2`, exposition par des Routes | CE, EE | SQLite (emptyDir) |

Swarm et kind tournent sur l'unique nœud du runner (`swarm init --advertise-addr
127.0.0.1`, kind avec des `extraPortMappings` vers des ports fixes de l'hôte) : ce
qui est testé, c'est l'orchestrateur et nos fichiers, pas un réseau réparti sur
plusieurs hôtes. La version de kind est figée et contrôlée par SHA256, l'image est
chargée avec `kind load` et `imagePullPolicy: Never` ; chaque déploiement est
attendu avec son propre `kubectl rollout status`, et en cas d'échec la sortie de
`describe` et les journaux sont affichés.

### OKD / OpenShift

OpenShift ne démarre pas un pod sous l'uid de l'image : la SCC `restricted` en tire
un dans la plage du namespace, avec le groupe 0, et rejette un pod qui impose le
sien. C'est pour cette raison que le chart ne définit ni `runAsUser` ni `fsGroup`.
Il y a deux façons de le prouver, de la moins coûteuse à la plus fidèle :

1. **`KO`, sur kind, chaque nuit** : le namespace porte le label
   `pod-security.kubernetes.io/enforce=restricted`, et une surcouche de test fait ce
   que fait la SCC (uid `1000680000`, groupe 0). Si la gateway écrit dans
   `/data`, migre, sert le trafic et fournit le snapshot, elle tournera sur
   OpenShift. S'y ajoute, à chaque push, un contrôle statique du chart rendu : aucun
   uid imposé, rien de privilégié, aucun port inférieur à 1024.
2. **`OK`, MicroShift, chaque nuit** : la petite distribution d'OpenShift,
   construite à partir d'OKD, un seul nœud dans un conteneur privilégié - elle tient
   sur un runner GitHub standard. Elle apporte ce que kind ne sait pas imiter : la
   SCC `restricted-v2` qui ADMET le pod et lui attribue son uid, et le routeur
   derrière les Routes. `e2e/platform/okd/run.sh <image> <ce|ee>` y installe le
   chart publié, puis vérifie la SCC, l'uid, l'écriture dans `/data`, les deux plans
   derrière leurs Routes et - en édition Enterprise - un assistant de montage tel
   que plug le crée, avec un vrai client SMB. Le workflow `okd.yml` le joue pour les
   deux images. Une seule chose est désactivée : la demande de volume, parce que le
   pilote de stockage de MicroShift exige un groupe de volumes LVM sur l'hôte.

## Les services de test

Tout se trouve sur le réseau de la plateforme, et rien n'est appelé sur Internet :
un verdict qui dépend du site de quelqu'un d'autre ne dit rien du produit.

| Service | Image | Rôle |
|---|---|---|
| httpbin | `mccutchen/go-httpbin` | l'upstream des routes : en-têtes, délais, statuts, flux |
| écho WebSocket et gRPC | un petit binaire Go dans `e2e/platform/services` | ROUTE-13, ROUTE-20 (unaire et en flux, h2c et TLS) |
| Mailpit | `axllent/mailpit` | le serveur SMTP de test, lu par son API HTTP (code par e-mail, mot de passe oublié, récapitulatif) |
| Dex | `dexidp/dex` | OIDC |
| OpenLDAP, Samba AD | les images de la CI Enterprise | LDAP / AD et règles de groupe (EE) |
| PostgreSQL | `postgres:17` | les cibles en cluster |
| Traefik | `traefik` | le load balancer devant le cluster compose, et le reverse proxy de X-17 |
| Pebble + challtestsrv | `ghcr.io/letsencrypt/pebble` | ACME (EE), voir plus bas |
| Gitea | `gitea/gitea` | les emplacements git (EE) |
| OpenTelemetry Collector | `otel/opentelemetry-collector-contrib`, exporteur fichier | audit, traces, métriques (EE) |

## Les phases

Chaque phase livre quelque chose qui tourne et qui détecte déjà des régressions ;
la suivante ne commence qu'une fois la précédente restée verte deux nuits de suite.

| Phase | Ce qu'elle livre | Cibles | Coût estimé |
|---|---|---|---|
| **0. Une suite indépendante de la cible** | `DATA_URL`, `ADMIN_URL`, le mot de passe root et le serveur SMTP de test lus dans l'environnement ; les entrées `webServer` ignorées quand une cible est fournie ; un jeu de données initial qui ne suppose plus un disque local | `I` | une demi-journée |
| **1. L'image sur compose** | `e2e/platform/compose/` (les services de test en surcouche des fichiers publiés), les cellules `smoke`, `edition`, `matrix` (toute la suite contre l'image), en Community et en Enterprise | `C1` | deux jours |
| **2. Le cluster** | 3 gateways + PostgreSQL + Traefik ; les cellules `sticky-free` (une session ouverte sur A, servie par B), `bus` (une route enregistrée sur A, servie par C), `once` (planifications, récapitulatif, purge), `kill-node`, `pg-restart`, `vault-key-mismatch` | `CC` | trois jours |
| **3. Kubernetes** | kind + le chart, les trois fichiers de valeurs ; `helm upgrade --reuse-values`, mise à jour progressive sous trafic, découverte des Services ; les règles d'OpenShift sur kind et le contrôle statique du chart | `K1`, `KC`, `KO` | quatre jours |
| **4. Swarm** | `stack deploy` du fichier publié, secrets et configs, convergence, mise à jour start-first | `S1`, `SC` | un jour |
| **5. La montée de version** | la version précédente retrouvée (et non figée), déployée et alimentée, puis remplacée par l'image du commit ; une base de référence par version ; le retour en arrière refusé | `C1`, `CC`, `K1` | deux jours |
| **6. Les intégrations Enterprise** | ACME face à Pebble, git face à Gitea, OpenTelemetry vers le Collector | `C1`, `CC` | trois jours |
| **7. Endurance et arm64** | 4 heures de trafic mixte chaque semaine ; la suite de tests de fumée sur un runner arm64 natif | `CC`, `C1` | deux jours |

### Pourquoi la phase 0 d'abord

`e2e/playwright.config.ts` code en dur `localhost:18082` et `localhost:19092`,
démarre la gateway, httpbin et le serveur SMTP de test, et le parcours
d'inscription lit son e-mail dans `.tmp/mail`, sur le disque. Tant que c'est le
cas, la suite ne peut viser qu'un processus local. Une fois ces quatre points lus
dans l'environnement, la même suite, sans une ligne de plus, se joue contre
n'importe quelle cible : l'essentiel du bénéfice pour le moindre effort.

## ACME, la phase la plus difficile

Pebble est le serveur ACME de test de Let's Encrypt : il émet de vrais certificats,
signés par une autorité jetable, et `pebble-challtestsrv` lui sert de DNS. Trois
obstacles, tous surmontables :

1. **La confiance.** L'annuaire de Pebble est servi en HTTPS sous sa propre
   autorité. La gateway doit lui faire confiance : `SSL_CERT_FILE` sur le
   conteneur suffit pour Go sous Linux, sans rien changer au produit.
2. **Le défi.** Meerkat valide en TLS-ALPN-01 : Pebble doit donc atteindre la
   porte HTTPS de l'application **sur le port 443**, pour chaque nom demandé.
   challtestsrv résout ces noms vers la gateway, et la cible publie
   l'application sur le port 443 à l'intérieur du réseau de la plateforme
   (`PEBBLE_VA_ALWAYS_VALID=1` permet d'écrire la cellule avant que le réseau soit
   correct ; on le retire ensuite).
3. **Le cluster.** Trois nœuds, une seule commande : le certificat doit être émis
   **une seule fois**, sous le verrou consultatif, et servi par les trois. C'est le
   test qui compte, et celui qu'aucun test unitaire ne peut faire.

La cellule se déroule ainsi : enregistrer l'autorité Pebble, créer une commande
placée sur l'application, attendre le statut "émis", lire le certificat que sert
chaque nœud, puis forcer un renouvellement (durée de vie courte côté Pebble). Sur
l'image Community, la même cellule vérifie le refus.

## La CI

| Quand | Quoi | Où |
|---|---|---|
| à chaque push | la phase 0 et le test de fumée de `C1`, dans les deux éditions | `ci.yml` (meerkat-ce) et `ci-ee.yml` |
| chaque nuit sur `main` | toutes les cibles, toute la suite, la montée de version | un workflow `platform.yml` planifié, déclenchable aussi à la main |
| avant une version publiée | la nuit précédente doit être verte, sinon elle est rejouée | le verrou de `ci-ee.yml` lit les marqueurs |
| chaque semaine | le test d'endurance de 4 heures | `soak.yml` |

Ce que nous gardons de plug :

- chaque cellule est une étape du job, avec son propre rouge ;
- un script vérifie que l'ordre des cellules, leurs fichiers et les noms des jobs
  concordent, et qu'aucune ne porte `continue-on-error` ;
- une cellule ignorée est comptée et remontée en avertissement, jamais présentée
  comme réussie ;
- chaque appel est borné par un chien de garde, lui-même plafonné au temps qui
  reste au job (un job tué par son `timeout` perd son journal) ;
- un job `abort-on-fail` annule le reste dès qu'une branche échoue, pour que le
  rouge reste rouge au lieu de passer à "cancelled" ;
- la connexion à Docker Hub, même pour un simple pull, à cause des rate limits.

Ce que nous ne reprenons pas : le tailnet. Les cibles de Meerkat tournent dans le
job même qui les teste ; il n'y a aucun client à connecter sur d'autres systèmes.

## La disposition dans le dépôt

```
e2e/platform/
  plan.json            le plan, une entrée par ligne de FEATURES.md + les contrôles X-xx
  run.sh               <target> <edition> <cell...>
  lib.sh               attentes, bornes, résumé du job
  cells/               une cellule par fichier : smoke.sh, edition.sh, kill-node.sh...
  compose/             surcouches des fichiers publiés : services de test, cluster
  kind/                kind-config.yaml, valeurs de test
  swarm/               surcouche de la stack
  services/            écho WebSocket et gRPC
```

## Tenir le plan à jour

`plan.json` se maintient à côté de `FEATURES.md`. **Une fonctionnalité ajoutée
là-bas ajoute son entrée ici, dans le même commit** ; en cas d'oubli, la page
[Plan de test](/project/test-plan) la liste tout en bas, sous "Dans FEATURES.md et
pas encore dans le plan". La colonne "Tests qui la citent aujourd'hui" est lue dans
les sources des tests : une fonctionnalité qu'aucun test ne nomme est une
fonctionnalité que personne n'a vérifiée, et c'est exactement ce que cette colonne
sert à montrer.
