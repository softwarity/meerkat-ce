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

## Pourquoi ce nom

::: figure meerkat
Le suricate, en sentinelle.
:::


Le suricate est la sentinelle de la nature : il monte la garde à l'entrée du
terrier et donne l'alerte, pour que le reste de la colonie travaille sans avoir
à s'inquiéter de rien. C'est exactement ce que cette passerelle fait pour vos
services. Même le tunnel [plug](https://github.com/softwarity/plug) entre dans
l'image : c'est par lui que la machine d'un développeur se creuse un chemin
jusqu'au terrier. Et comme un groupe de suricates s'appelle une *mob*, vous
savez déjà comment nommer un cluster de nœuds Meerkat.
