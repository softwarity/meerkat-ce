---
title: Santé des upstreams
section: Exploitation
order: 218
summary: Combien de temps une route attend son service, quand elle cesse de l'appeler, et comment la console le sait.
---

# Santé des upstreams

Un service lent n'est pas seulement le problème de ce service : la gateway se trouve
sur le chemin de chaque requête. Une connexion maintenue ouverte pendant trois minutes
est une connexion qui ne sert personne d'autre, et la panne se manifeste loin de son
origine.

Trois mécanismes y répondent, et ils sont volontairement distincts : une **borne** sur
l'attente, un **disjoncteur** qui cesse d'appeler, et un **verdict** que lit la console.

## Combien de temps une route attend

Deux bornes, héritées chacune séparément, sur trois niveaux :

1. ce qu'indique la route ;
2. à défaut, l'installation, dans **Routes > Global** ;
3. à défaut, le produit : `5s` pour se connecter, `15s` pour la première réponse.

| Borne | Ce qu'elle couvre | Plage |
|---|---|---|
| Connexion | l'acceptation de la connexion, négociation TLS comprise | `1s` à `30s` |
| Première réponse | l'attente de la première ligne de la réponse - le service réfléchit | `1s` à `10m` |

Une fois l'une ou l'autre dépassée, l'appelant reçoit un `502` au lieu d'attendre.

Ce qui n'est **jamais** borné, c'est le corps. Un téléchargement ou un websocket déjà
en cours dure aussi longtemps que nécessaire : la borne porte sur le temps dont dispose
un service pour accepter une connexion et pour *commencer* à répondre.

Un maximum existe parce que ces bornes sont précisément les garde-fous : un timeout de
réponse d'une heure, c'est une route sans aucune borne, écrite de façon à en avoir
l'air.

Les routes qui déclarent la même paire de bornes partagent un pool de connexions : les
écrire route par route ne multiplie donc pas les transports.

## Le disjoncteur

Borner l'attente empêche un service lent de retenir une connexion indéfiniment. Cela
n'empêche pas la gateway de lui envoyer mille requêtes de plus, qui attendront
chacune jusqu'à leur propre borne - c'est ainsi qu'un service simplement arrêté entraîne
avec lui la capacité de la gateway, et qu'il essuie une ruée de requêtes à l'instant
où il redémarre.

D'où, route par route et **désactivé par défaut** :

- après N échecs **consécutifs**, la route cesse d'appeler le service et sert
 immédiatement la page d'indisponibilité ;
- après un temps de repos, **une seule** requête est autorisée à passer. Si elle
 aboutit, le circuit se referme ; si elle échoue, le temps de repos recommence.

| Réglage | Valeur par défaut | Plage |
|---|---|---|
| Échecs consécutifs qui déclenchent le disjoncteur | cinq | de deux à cent |
| Temps de repos | `15s` | `1s` à `5m` |

Le mot "consécutifs" est voulu : le disjoncteur cherche un service qui a cessé de
répondre, pas un service qui échoue de temps à autre sous la charge. Le moindre succès
remet le compte à zéro.

Ce qui compte comme un échec est volontairement restreint : une erreur de transport, ou
un `502`, un `503` ou un `504` - les réponses que la gateway produit elle-même quand
elle n'a pas pu joindre le service ou pas pu l'attendre. **Pas un `500`** : un service
qui répond `500` fonctionne et a un bogue. L'écarter transformerait un endpoint
défaillant en une route entière que plus personne ne peut joindre, y compris ses
endpoints qui fonctionnent.

Il est livré désactivé parce qu'un disjoncteur remplace un type de panne par un autre :
un service simplement lent à redémarrer est refusé pendant tout le temps de repos. Une
installation ne doit connaître ce comportement que si quelqu'un l'a choisi.

## Ce que montre la console

Dans la liste Routes, chaque route porte un **cœur**, avec la raison dans son infobulle :

| Cœur | Signifie |
|---|---|
| Vert | toutes les répliques du service sont prêtes, ou un hôte externe accepte une connexion |
| Orange | une partie des répliques est prête, pas toutes : le service répond, avec moins que ce qu'on lui a donné |
| Rouge, brisé | aucune n'est prête, aucune n'est demandée (mis à zéro), l'hôte refuse, ou le circuit est ouvert |

Pour un service que le runtime exécute, la ligne indique aussi **combien de répliques
sont prêtes** (`2/3`, à côté du nom) et **quelle image elles exécutent** (après
l'upstream : le tag, ou le début de l'empreinte quand le tag est `latest`). Deux images
sur un même service, c'est un déploiement en cours, ou arrêté à mi-chemin ; l'infobulle
liste chaque image avec son empreinte et le nombre de répliques qui l'exécutent.

### Sans minuteur

La gateway n'interroge pas le runtime à intervalle régulier : elle l'**écoute**.

- **Kubernetes** : elle liste une fois les Services et les pods de son propre namespace,
  puis les surveille. Un pod qui démarre, devient prêt, plante ou est remplacé arrive
  comme un événement de l'API server, et l'écran change avec lui. Les répliques d'un
  Service sont les pods que choisit son sélecteur. Un Service sans sélecteur n'a pas de
  pods à compter, et il est vérifié comme un hôte externe.
- **Docker et Swarm** : un inventaire, puis le flux d'événements de Docker. Dans un Swarm,
  les événements d'un conteneur ne viennent que du nœud qui l'exécute : la gateway écoute
  donc chaque nœud, à travers le proxy de socket en lecture seule que la stack déploie
  sur chacun d'eux.
- Un flux qui coupe est rouvert, en commençant par un nouvel inventaire : rien de ce qui
  s'est passé entre-temps n'est perdu.

Ce qu'aucun événement ne peut dire, c'est qu'un **hôte externe** a disparu. Ceux-là sont
vérifiés par une simple connexion TCP toutes les 30 secondes, avec un timeout de
2 secondes : une connexion et rien d'autre, aucune requête HTTP, donc rien n'apparaît
dans les journaux du service ni ne compte dans ses limites. Derrière un `HTTP_PROXY`,
c'est le proxy qui est contacté. Une gateway sans cible externe ne fait tourner aucun
minuteur. Un service externe n'affiche jamais de répliques ni d'images : ce n'est pas
l'affaire de la gateway.

Un circuit ouvert équivaut à une cible indisponible, quoi qu'aient dit le runtime ou la
connexion : le trafic réel l'emporte.

### Ce qu'il faut

| Runtime | Droit |
|---|---|
| Kubernetes | `list` et `watch` sur les `pods` et les `services` du namespace de la gateway. Le chart l'accorde avec `rbac.watch`, actif par défaut - voir [Kubernetes](/docs/deploy/kubernetes) |
| Docker | l'API Docker, en lecture seule : le proxy de socket du fichier compose, ou le socket monté pour le tunnel |
| Swarm | le proxy de socket sur chaque nœud (`mode: global`), que la stack déploie - voir [Quelle architecture déployer](/docs/deploy/shapes) |

Sans ce droit, les cibles sont vérifiées par une connexion, et la gateway indique une fois
le droit qui lui manque.

## En cluster

L'état du disjoncteur et la vérification des cibles sont propres à **chaque nœud**, et
c'est le bon choix. Deux gateways peuvent réellement être en désaccord sur un upstream -
un chemin réseau, une réponse DNS, un sidecar - et un verdict partagé laisserait la
mauvaise passe d'un seul nœud ouvrir un circuit pour tout le monde.

Le coût est connu : le retour d'un service est constaté une fois par nœud plutôt qu'une
seule fois, soit une requête d'essai par nœud. Et l'écran de santé répond pour **le
nœud interrogé**.
