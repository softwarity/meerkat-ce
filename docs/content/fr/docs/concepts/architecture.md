---
title: Architecture
section: Concepts
order: 20
summary: Un seul binaire qui écoute sur deux plans - les applications sur un port, leur administration sur l'autre - et pourquoi cette ligne ne se négocie pas.
---

# Architecture

Meerkat est un seul processus. Il écoute sur **deux plans**, et ce sont deux
métiers différents servis par deux ports différents.

| | Plan de données | Plan de contrôle |
|---|---|---|
| Port par défaut | `:8080` | `:9090` |
| Port HTTPS | `:8443` | `:9443` |
| Sert | vos applications, et les pages que la gateway sert elle-même | l'API d'administration et la console |
| A sa place sur | le réseau que vos utilisateurs atteignent | un réseau interne, jamais le réseau public |

## Où elle se place

Un binaire devant les services dont votre application est faite. Ce qui l'atteint
est un navigateur ou un client d'API ; ce qu'elle transmet est une requête sur
laquelle on a déjà tranché.

::: figure mesh
La même passerelle devant les deux moitiés d'une application : les interfaces
que vos utilisateurs ouvrent, et les API que tout le reste appelle.
:::

La ligne au milieu des services est celle qui compte. Une **route UI** est une
page qu'une personne regarde, alors la passerelle l'habille : le portail de
navigation, le bouton de compte, le thème, le mode clair et sombre, tout cela
injecté dans le HTML au passage. L'application n'embarque aucune bibliothèque
pour ça et ne sait pas qu'on l'habille.

Une **route API** est appelée par un programme, il n'y a donc rien à habiller.
Ce qu'elle reçoit à la place est un jeton signé dans un en-tête - qui appelle,
ses rôles, son organisation - et elle n'authentifie personne. C'est tout
l'intérêt : vos services cessent de porter une page de connexion et une table
d'utilisateurs.

Un service peut être les deux, et l'est souvent : la même application servant
ses pages et sa propre API derrière deux routes.

## Pourquoi ils sont séparés

Parce qu'une erreur sur l'un ne doit pas être une erreur sur l'autre.

- La console n'est **jamais** joignable sur le plan de données. Aucune route ne peut l'exposer, parce qu'elle n'y est pas montée du tout.
- Chaque plan a son cookie de session - `MEERKAT_SESSION` et `MEERKAT_ADMIN_SESSION` - et chaque session stockée porte l'estampille du plan auquel elle appartient. Un jeton recopié d'un cookie dans l'autre est refusé : les deux ports ne partagent jamais une session de navigateur.
- Les jetons d'API portent aussi un plan. Un jeton du plan de données n'ouvre jamais le port d'administration, et réciproquement.
- Une conséquence vaut d'être sue avant d'en avoir besoin : le port HTTP en clair du plan de contrôle reste en clair, définitivement. C'est de là qu'on répare un certificat cassé. Seul le port en clair du plan de données redirige vers HTTPS.

## Ce qui vit sur le plan de données

Vos routes, et les pages que la gateway sert en son nom :

| Chemin | Quoi |
|---|---|
| `/login`, `/logout` | la connexion, et les étapes qui la suivent : `/update-password`, `/totp`, `/totp-enroll`, `/select-tenant`, `/select-group` |
| `/register`, `/confirm`, `/forgot-password`, `/reset-password` | les portes d'entrée d'un compte, chacune commandée par un réglage - l'auto-inscription est livrée fermée |
| `/profile/...` | ce qu'une personne peut faire de son propre compte : mot de passe, MFA, passkeys, jetons d'API, historique de connexion |
| `/refused`, `/account-pending` | pourquoi une requête a été écartée, et quoi faire ensuite |
| `/meerkat/...` | les ressources que la gateway injecte dans les pages proxifiées, servies par le moteur et non par une route |
| `/healthz`, `/readyz` | vivacité et disponibilité |

> [!WARNING]
> Ces préfixes appartiennent à la gateway. Une route dont les prédicats couvrent
> `/login` ou `/meerkat/` ne sera pas atteinte pour eux : les handlers du moteur
> sont enregistrés d'abord, et tout le reste tombe sur le routeur.

## Ce qui vit sur le plan de contrôle

- `/api/...` - l'API d'administration, environ cent cinquante endpoints.
- `/` - la console, une application Angular embarquée dans le binaire. En anglais uniquement, sans segment de langue : c'est un outil d'exploitant.
- `/login`, `/logout` - la connexion propre à la console, pour que cette origine se suffise à elle-même.
- `/mcp` - le point d'entrée auquel un agent se connecte.
- `/metrics` - l'exposition Prometheus, quand elle est activée.
- `/healthz`, `/readyz`.

> [!NOTE]
> Édition Enterprise, pour `/metrics`. Les compteurs eux-mêmes, et les tableaux
> de bord que la console en tire, sont dans les deux images.

## Un binaire, aucune dépendance

Le stockage est embarqué - un fichier dans le répertoire donné par `-data`. La
console est compilée dans le binaire. Il n'y a pas de CGO, pas de conteneur
adjoint, pas de bus de messages et pas de cache à faire tourner à côté.

Une base PostgreSQL externe est une **option**, et elle achète une chose :
plusieurs gateways servant une seule installation. Elle a été choisie pour ce
qu'elle porte au-delà du stockage - `LISTEN`/`NOTIFY`, qui est la façon dont une
gateway dit aux autres que quelque chose a changé, et les verrous consultatifs,
qui sont la seule exclusion dont le produit a besoin.

> [!NOTE]
> Édition Enterprise. Le pilote PostgreSQL n'est que dans le binaire Enterprise.

## Comment un changement prend effet

Les routes sont stockées, pas configurées dans un fichier. En enregistrer une
recompile toute la table de routage en un instantané échangé atomiquement : les
requêtes en vol terminent sur la table avec laquelle elles ont commencé. Rien ne
redémarre, et rien n'est relu sur disque.

En cluster, l'écriture est annoncée sur le bus de changement et chaque noeud
recompile. `SIGHUP` fait la même chose localement, pour les cas où quelque chose
en dehors de la console a écrit dans la base.

Une route dont les références de coffre ne résolvent rien est **laissée de côté**
dans la table compilée, avec une ligne d'avertissement, plutôt que de faire
échouer tout le rechargement. C'est l'état normal d'une gateway qui vient d'être
amorcée depuis un fichier de configuration dont le coffre n'est pas encore
rempli.

## Sondes

`/healthz` est la vivacité et répond UP inconditionnellement. C'est la bonne
réponse, pas une réponse paresseuse : la vivacité décide s'il faut **tuer** le
processus, et une sonde qui échouerait sur une base injoignable redémarrerait
tous les noeuds à la fois pour une panne qu'aucun ne peut réparer en mourant.

`/readyz` est la disponibilité et répond 503 avec le motif quand le stockage ne
répond pas, ou quand la table de routage n'est pas encore compilée. C'est sur
celle-là que doit pointer votre répartiteur.

## Ensuite

- Ce qu'une route contient : [Routes](/docs/concepts/routes)
- Qui appelle : [Identité](/docs/concepts/identity)
