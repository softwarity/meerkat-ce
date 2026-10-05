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

Dans la liste Routes, chaque route porte un **cœur** : vert quand sa cible répond, brisé
et rouge quand elle ne répond pas, avec la raison dans l'infobulle.

La gateway vérifie en arrière-plan la cible de chaque route activée, toutes les 30
secondes, jamais sur le chemin des requêtes :

- un service connu de la découverte (Docker, Swarm) est disponible dès qu'une réplique
  au moins est prête, et indisponible à zéro ;
- tout le reste - un hôte externe, un service Kubernetes - fait l'objet d'une simple
  connexion TCP, avec un timeout de 2 secondes. Une connexion et rien d'autre : aucune
  requête HTTP, donc rien n'apparaît dans les journaux du service ni ne compte dans ses
  limites. Derrière un `HTTP_PROXY`, c'est le proxy qui est contacté ;
- un circuit ouvert équivaut à une cible indisponible, quoi qu'ait dit la connexion :
  le trafic réel l'emporte.

Tout changement est poussé aussitôt vers l'écran ouvert, sans rechargement. Une route
que personne n'appelle a désormais quelque chose à dire. Ce qui manque encore : le
`/health` propre à chaque service.

## En cluster

L'état du disjoncteur et la vérification des cibles sont propres à **chaque nœud**, et
c'est le bon choix. Deux gateways peuvent réellement être en désaccord sur un upstream -
un chemin réseau, une réponse DNS, un sidecar - et un verdict partagé laisserait la
mauvaise passe d'un seul nœud ouvrir un circuit pour tout le monde.

Le coût est connu : le retour d'un service est constaté une fois par nœud plutôt qu'une
seule fois, soit une requête d'essai par nœud. Et l'écran de santé répond pour **le
nœud interrogé**.
