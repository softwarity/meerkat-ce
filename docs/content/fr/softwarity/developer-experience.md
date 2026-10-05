---
title: L'expérience développeur
section: Softwarity
order: 3
summary: Ici, l'expérience développeur n'est pas un slogan : elle se mesure à ce qu'un développeur n'a pas à faire, et au temps qu'il faut pour arriver à la première minute utile.
---

# L'expérience développeur

L'expérience développeur est cette partie d'un produit qui ne figure dans
aucune liste de fonctionnalités, et qui décide pourtant si le produit sera
utilisé. Nous la traitons comme une exigence, et elle a son test : **combien de
temps faut-il pour obtenir quelque chose d'utile, et combien avez-vous dû lire
avant d'y arriver ?**

## Une commande, et quelque chose fonctionne

```bash
docker run -p 8080:8080 -p 9090:9090 \
  -e MEERKAT_ADMIN_PASSWORD=choose-one softwarity/meerkat
```

Pas de base de données à créer, pas de serveur de messages, pas de fichier de
configuration, pas de chart à générer. La gateway démarre avec son propre
stockage, sert ses propres pages de connexion et sa propre console. Le jour où
il vous faut plusieurs gateways, elle demande un PostgreSQL - et pas avant.

## Une erreur dit ce qui est accepté

Une valeur rejetée avec pour seul message `invalid argument` ne vous apprend
rien. Partout où c'est possible, un refus dit ce qui a été fourni, ce qui est
accepté, et où le modifier. Cette règle est écrite dans le projet, elle est
vérifiée en relecture, et c'est ce qu'un produit peut faire de moins coûteux
pour ceux qui l'utilisent.

## Le poste de travail rejoint le cluster

Le cluster contient tous les services et toutes les données, et le reproduire
sur un ordinateur portable va du pénible à l'interdit.
[plug](https://github.com/softwarity/plug) renverse le problème : le poste de
travail rejoint le maillage, un service qui tourne en local répond sous son nom
de cluster, et tous ceux qui regardent l'application savent quel service est
substitué, et par qui. Voir [Mode développement](/product/dev-mode).

## La configuration se fait, elle ne s'écrit pas

Un exploitant configure dans la console, exporte, puis rejoue l'export
ailleurs. Il n'y a pas de dialecte YAML à apprendre pour les cas courants, et
ce que vous voudrez versionner - les routes, les rôles, les réglages - sort
sous la forme d'un seul document, lisible lors d'une relecture.

## La documentation correspond à la version que vous utilisez

Une documentation se périme en silence, et celui qui s'en aperçoit est celui
qui la suit à deux heures du matin. Les pages que vous lisez sont donc
versionnées avec le produit : la documentation de la 1.3 est celle qui existait
à la sortie de la 1.3, pas celle d'aujourd'hui avec les nouveaux écrans. Un
sélecteur de version figure sur chaque page de la documentation.

Ce que vous lisez se vérifie, au lieu de se promettre : la page
[Couverture de tests](/project/tests) est exactement le fichier qu'exécute la
suite d'intégration, et ce qui est construit ou non tient dans
[un tableau lu dans le code](/project/roadmap).
