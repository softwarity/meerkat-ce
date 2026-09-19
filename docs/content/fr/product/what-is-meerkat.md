---
title: Ce qu'est Meerkat
section: Le produit
order: 1
summary: Une porte unique devant vos applications internes, qui prend en charge tout ce qui n'est pas le métier de vos équipes.
---

# Ce qu'est Meerkat

Meerkat est une **app-gateway** : une porte unique devant les applications de
votre organisation. Les requêtes arrivent sur Meerkat, il décide quoi en faire,
puis il les transmet.

Ce qu'il prend en charge, pour que vos services n'aient pas à le faire :

- **Qui appelle** - pages de connexion, SSO, double facteur, jetons d'API, sessions.
- **Qui a le droit de passer** - rôles, groupes, organisations, règles par route et par endpoint.
- **Comment la requête voyage** - routage, réécriture, en-têtes, limites de débit, TLS.
- **Ce qui se passe** - trafic, journal d'audit, métriques, santé des amonts.

## Pourquoi une gateway

Une application interne démarre en général sans rien de tout cela. Puis il lui
faut une page de connexion, alors quelqu'un l'écrit. Puis une deuxième
application a besoin de la même, et les deux ne s'accordent pas sur ce qu'est
une session. Meerkat est l'endroit où ces questions sont tranchées une fois.

> [!NOTE]
> Meerkat proxifie vos applications telles qu'elles sont. Il ne leur demande ni
> d'embarquer une bibliothèque, ni de parler un protocole à lui.

## Un seul binaire

La gateway est un unique binaire Go sans dépendance : elle sert le plan de
données sur un port et sa console d'administration sur un autre. Rien d'autre
n'a besoin d'être installé pour la faire tourner.
