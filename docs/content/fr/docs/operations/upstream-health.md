---
title: Santé des amonts
section: Exploitation
order: 218
summary: Combien de temps une route attend son service, quand elle cesse de l'appeler, et comment la console le sait.
---

# Santé des amonts

Un service lent n'est pas le problème de ce service : la passerelle est sur le chemin de toutes les
requêtes, donc une connexion tenue ouverte trois minutes est une connexion qui ne sert personne
d'autre, et la panne se manifeste loin de là où elle a commencé.

Trois mécanismes répondent à ça, et ils sont séparés exprès : une **borne** sur l'attente, un
**disjoncteur** qui cesse d'appeler, et un **verdict** que la console lit.

## Combien de temps une route attend

Deux bornes, chacune héritée séparément, à trois niveaux (ROUTE-07) :

1. ce que dit la route ;
2. sinon l'installation, dans **Routes > Global** ;
3. sinon le produit : `5s` pour se connecter, `15s` pour la première réponse.

| Borne | Ce qu'elle couvre | Plage |
|---|---|---|
| Connexion | accepter la connexion, poignée de main TLS comprise | `1s` à `30s` |
| Première réponse | l'attente de la première ligne de la réponse - le service réfléchit | `1s` à `10m` |

Passé l'une ou l'autre, l'appelant reçoit un `502` au lieu d'attendre.

Ce qui n'est **jamais** borné, c'est le corps. Un téléchargement ou un websocket déjà commencé vit
aussi longtemps qu'il faut : ce qui est borné, c'est le temps qu'un service met à accepter une
connexion et à *commencer* à répondre.

Un maximum existe parce que ce sont les garde-fous eux-mêmes : une borne de réponse d'une heure est une
route sans borne du tout, écrite d'une façon qui ressemble à une borne.

Les routes qui nomment la même paire de bornes partagent un pool de connexions, donc les écrire par
route ne multiplie pas les transports.

## Le disjoncteur

Borner l'attente empêche un service lent de tenir une connexion pour toujours. Ça n'empêche pas la
passerelle de lui envoyer mille requêtes de plus qui attendront chacune leur propre borne - c'est
ainsi qu'un service simplement tombé emporte la capacité de la passerelle avec lui, et qu'il est
accueilli par une ruée à l'instant où il revient.

Donc, par route et **éteint par défaut** (ROUTE-09) :

- après N échecs **consécutifs**, la route cesse d'appeler et sert immédiatement la page
  d'indisponibilité ;
- après un délai de refroidissement, **une seule** requête passe. Si elle marche, le circuit se
  referme ; si elle échoue, le refroidissement repart.

| Réglage | Défaut | Plage |
|---|---|---|
| Échecs consécutifs qui déclenchent | cinq | deux à cent |
| Refroidissement | `15s` | `1s` à `5m` |

Consécutifs exprès : ce que ça cherche, c'est un service qui a cessé de répondre, pas un service qui
échoue de temps en temps sous charge. Tout succès remet le compte à zéro.

Ce qui compte comme un échec est délibérément étroit : une erreur de transport, ou un `502`, `503` ou
`504` - les réponses que la passerelle produit elle-même quand elle n'a pas pu joindre ou pas pu
attendre. **Pas un `500`** : un service qui répond `500` est debout et a un bug, et le sortir de
rotation transformerait un endpoint cassé en une route entière que personne n'atteint, y compris les
endpoints qui marchent.

Il est livré éteint parce qu'un disjoncteur transforme un genre de panne en un autre : un service
simplement lent à revenir est refusé pendant tout le refroidissement. Une installation devrait
rencontrer ça parce que quelqu'un l'a choisi.

## Ce que la console montre

La liste des routes marque celles qui ne répondent plus et dit **pourquoi** (SVC-04, ROUTE-11). Ça ne
coûte rien à collecter : le disjoncteur regarde déjà chaque réponse réelle, donc ceci rapporte ce
qu'il sait plutôt que de sonder de son côté.

Une sonde séparée aurait été un second avis, formé sur du trafic que personne n'a envoyé, à propos
d'un chemin que les vraies requêtes n'empruntent peut-être même pas. Le coût est qu'une route que
personne n'a encore appelée n'a rien à dire - ce qui est honnête, et qui est exactement ce que la
passerelle sait.

Pas encore là (ROUTE-11, SVC-04) : la **sonde active**, qui seule peut parler d'une route que personne
n'appelle ; et un second niveau, le `/health` propre à un service.

## En cluster

L'état du disjoncteur est **par noeud**, et c'est le bon à garder. Deux passerelles peuvent réellement
ne pas être d'accord sur un amont - un chemin réseau, une réponse DNS, un sidecar - et un verdict
partagé laisserait la mauvaise minute d'un noeud ouvrir un circuit pour tout le monde.

Le coût est énoncé : un service qui revient est découvert une fois par noeud plutôt qu'une fois, ce qui
fait une requête de sondage chacun. Et l'écran de santé répond pour **le noeud qu'on a interrogé**.
