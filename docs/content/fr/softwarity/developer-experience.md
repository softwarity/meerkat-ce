---
title: L'experience developpeur
section: Softwarity
order: 3
summary: La DX n'est pas un slogan ici : elle se mesure à ce qu'un développeur n'a pas à faire, et au temps que met la première minute utile.
---

# L'expérience développeur

L'expérience développeur est la partie d'un produit qui n'apparaît dans aucune
liste de fonctionnalités et qui décide si le produit est utilisé. Nous la
traitons comme une exigence, et elle a un test : **combien de temps avant que
quelque chose d'utile arrive, et combien avez-vous dû lire avant ?**

## Une commande, puis quelque chose marche

```bash
docker run -p 8080:8080 -p 9090:9090 \
  -e MEERKAT_ADMIN_PASSWORD=choisissez-en-un softwarity/meerkat
```

Aucune base à créer, aucun broker, aucun fichier de configuration, aucun chart à
rendre. La passerelle démarre avec son stockage dedans, sert ses propres pages
de connexion et sa propre console. Le jour où il faut plusieurs passerelles,
elle prend un PostgreSQL - et pas avant.

## Une erreur nomme ce qui est permis

Une valeur refusée avec `argument invalide` ne vous a rien dit. Partout où nous
le pouvons, un refus dit ce qui a été donné, ce qui est accepté, et où le
changer. Cette règle est écrite dans le projet, elle est relue, et c'est la
chose la moins chère qu'un produit puisse faire pour ceux qui s'en servent.

## Le poste rejoint le cluster

Le cluster a tous les services et toutes les données, et le reproduire sur un
portable est quelque part entre pénible et interdit.
[plug](https://github.com/softwarity/plug) renverse la chose : le poste rejoint
le maillage, un service qui tourne en local répond sous son nom de cluster, et
tous ceux qui regardent l'application savent quel service est substitué et par
qui. Voir [Mode développement](/product/dev-mode).

## La configuration se fait, elle ne s'écrit pas

Un opérateur configure dans la console, exporte, et rejoue l'export ailleurs. Il
n'y a aucun dialecte YAML à apprendre pour le chemin courant, et ce que l'on
veut versionner - routes, rôles, réglages - sort en un document qui se lit dans
une relecture.

## La documentation correspond à la version que vous faites tourner

Une documentation pourrit en silence, et celui qui s'en aperçoit est celui qui
la suit à deux heures du matin. Les pages que vous lisez sont donc versionnées
avec le produit : la documentation de la 1.3 est celle du jour où la 1.3 est
sortie, pas celle d'aujourd'hui avec les nouveaux écrans dedans. Un sélecteur
de version est présent sur chaque page de documentation.

Ce que vous lisez est aussi vérifiable plutôt que promis : la page
[couverture de tests](/project/tests) est le fichier exact qu'exécute la suite
d'intégration, et ce qui est construit ou non tient dans
[un tableau lu dans le code](/project/roadmap).
