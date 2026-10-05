---
title: Architecture
section: Concepts
order: 20
summary: Un seul binaire à l'écoute sur deux plans - les applications sur un port, leur administration sur un autre - et pourquoi cette frontière n'est pas négociable.
---

# Architecture

Meerkat est un processus unique. Il écoute sur **deux plans**, qui remplissent
deux fonctions distinctes sur deux ports distincts.

| | Plan de données | Plan de contrôle |
|---|---|---|
| Port par défaut | `:8080` | `:9090` |
| Port HTTPS | `:8443` | `:9443` |
| Ce qu'il sert | vos applications, et les pages que la gateway sert elle-même | l'API d'administration et la console |
| Sa place | le réseau auquel vos utilisateurs accèdent | un réseau interne, jamais le réseau public |

## Où elle se place

Un seul binaire devant les services qui composent votre application. Ce qui
lui parvient vient d'un navigateur ou d'un client d'API ; ce qu'elle transmet
est une requête sur laquelle la décision est déjà prise.

::: figure mesh
La même gateway devant les deux moitiés d'une application : les interfaces
qu'ouvrent vos utilisateurs, et les API qu'appelle tout le reste.
:::

La ligne qui partage les services en deux est celle qui compte. Une **route
UI** sert une page qu'une personne regarde, et la gateway l'habille donc :
le portail de navigation, le bouton du compte, le thème, le mode clair ou
sombre, tout est injecté dans le HTML au retour. L'application n'embarque
aucune bibliothèque pour cela et ne sait même pas qu'on l'habille.

Une **route d'API** est appelée par un programme : il n'y a rien à habiller. À
la place, elle reçoit dans un en-tête un jeton signé - qui appelle, avec quels
rôles, dans quelle organisation - et n'authentifie personne. Tout l'intérêt est
là : vos services n'ont plus à porter ni page de connexion ni table
d'utilisateurs.

Un service peut être les deux à la fois, et c'est fréquent : une même
application sert ses pages et sa propre API derrière deux routes.

## Pourquoi ils sont séparés

Parce qu'une erreur commise sur l'un ne doit pas en devenir une sur l'autre.

- La console n'est **jamais** accessible sur le plan de données. Aucune route ne peut l'exposer, puisqu'elle n'y est tout simplement pas montée.
- Chaque plan a son propre cookie de session - `MEERKAT_SESSION` et `MEERKAT_ADMIN_SESSION` - et chaque session stockée porte la marque du plan auquel elle appartient. Un jeton copié d'un cookie dans l'autre est refusé : les deux ports ne partagent jamais une session de navigateur.
- Les jetons d'API portent eux aussi un plan. Un jeton du plan de données n'ouvre jamais le port d'administration, et inversement.
- Une conséquence mérite d'être connue avant d'en avoir besoin : le port HTTP en clair du plan de contrôle reste en clair, définitivement. C'est par lui que l'on répare un certificat cassé. Seul le port en clair du plan de données redirige vers HTTPS.

## Ce qui vit sur le plan de données

Vos routes, et les pages que la gateway sert en son nom propre :

| Chemin | Rôle |
|---|---|
| `/login`, `/logout` | la connexion et les étapes qui la suivent : `/update-password`, `/totp`, `/totp-enroll`, `/select-tenant`, `/select-group` |
| `/register`, `/confirm`, `/forgot-password`, `/reset-password` | les points d'entrée des comptes, chacun soumis à un réglage - l'auto-inscription est livrée fermée |
| `/profile/...` | ce qu'une personne peut faire de son propre compte : mot de passe, MFA, passkeys, jetons d'API, historique de connexion |
| `/refused`, `/account-pending` | pourquoi une requête a été refusée, et que faire ensuite |
| `/meerkat/...` | les ressources que la gateway injecte dans les pages qu'elle relaie, servies par le moteur et non par une route |
| `/healthz`, `/readyz` | vivacité et disponibilité |

> [!WARNING]
> Ces préfixes appartiennent à la gateway. Une route dont les prédicats
> couvrent `/login` ou `/meerkat/` ne recevra jamais ces requêtes : les
> gestionnaires du moteur sont enregistrés en premier, et seul le reste
> parvient au routeur.

## Ce qui vit sur le plan de contrôle

- `/api/...` - l'API d'administration, environ deux cents endpoints.
- `/` - la console, une application Angular embarquée dans le binaire. En anglais uniquement, sans segment de langue dans l'URL : c'est un outil d'exploitant.
- `/login`, `/logout` - la page de connexion propre à la console, pour que cette origine se suffise à elle-même.
- `/mcp` - l'endpoint auquel un agent se connecte.
- `/healthz`, `/readyz`.

## Un binaire, aucune dépendance

Le stockage est embarqué : un fichier dans le répertoire indiqué par `-data`.
La console est compilée dans le binaire. Il n'y a ni CGO, ni sidecar, ni
courtier de messages, ni cache à faire tourner à côté.

Une base PostgreSQL externe est une **option**, et elle apporte une chose :
plusieurs gateways au service d'une même installation. PostgreSQL a été
retenu pour ce qu'il offre au-delà du stockage : `LISTEN`/`NOTIFY`, par lequel
une gateway prévient les autres qu'un changement a eu lieu, et les verrous
consultatifs, qui fournissent la seule exclusion dont le produit a besoin.

> [!NOTE]
> Édition Enterprise. Le pilote PostgreSQL ne se trouve que dans le binaire
> Enterprise.

## Comment un changement prend effet

Les routes sont stockées en base, et non décrites dans un fichier. En
enregistrer une recompile toute la table de routage en un snapshot, qui
remplace l'ancien de façon atomique : les requêtes en cours se terminent sur la
table avec laquelle elles ont commencé. Rien ne redémarre, et rien n'est relu
depuis le disque.

Dans un cluster, l'écriture est annoncée sur le bus de changement et chaque
nœud recompile. `SIGHUP` produit le même effet en local, pour les cas où autre
chose que la console a écrit dans la base.

Une route dont les références au coffre ne mènent à rien est **écartée** de la
table compilée, avec une ligne d'avertissement, au lieu de faire échouer tout le
rechargement. C'est l'état normal d'une gateway tout juste initialisée à
partir d'un fichier de configuration et dont le coffre n'est pas encore rempli.

## Health checks

`/healthz` est le health check de vivacité et répond UP sans condition. C'est la bonne
réponse, pas une facilité : la vivacité décide s'il faut **tuer** le processus,
et un health check qui échouerait parce que la base est injoignable ferait redémarrer
tous les nœuds en même temps, pour une panne qu'aucun d'eux ne peut réparer en
mourant.

`/readyz` est le health check de disponibilité et répond 503, avec un motif, quand le
stockage ne répond pas ou quand la table de routage n'est pas encore compilée.
C'est sur lui que doit pointer votre load balancer.

## Ensuite

- Ce qui compose une route : [Routes](/docs/concepts/routes)
- Qui appelle : [Identité](/docs/concepts/identity)
