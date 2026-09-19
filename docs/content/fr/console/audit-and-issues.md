---
title: Audit et Issues
section: La console
order: 184
summary: Les deux écrans que l'on lit après coup - qui a changé quoi, et ce que vos utilisateurs ont signalé.
---

# Audit et Issues

Deux écrans transverses, côte à côte dans le rail, lus tous les deux après coup et limités
côté serveur à ce que l'appelant administre : root voit tout, un infra admin le plan de
routage, un app admin l'identité, un tenant admin ses propres organisations.

## Audit

Chaque changement administratif, avec **les champs exacts qui ont bougé et leur avant et
après**. En lecture seule.

![L'écran Audit : une liste d'événements, chacun avec son acteur et les champs qui ont bougé](img/console/audit.webp)

Deux configurations capturées, deux routes dont les quotas ont été écrits, un groupe
renommé, une accroche réécrite et un TTL de session raccourci - chacun avec son avant barré
et son après à côté.

Chaque événement se lit ainsi : quand, quelle action, par qui, sur quoi, et le diff champ
par champ en dessous. Là où un agent ou un script a agi, le nom du jeton apparaît à côté de
celui du compte : *admin, via claude-desktop* plutôt que *admin*. Cette différence est toute
la raison de le nommer.

Trois filtres : le genre de **cible** (`tenant`, `user`, `membership`, `group`, `role`,
`settings`, `route`, `theme`), la **période** (24 heures, 7 jours, 30 jours, tout), et une
recherche libre qui restreint ce qui est déjà chargé.

- Les secrets sont masqués : la trace dit qu'un champ a changé, pas vers quoi.
- Un compte supprimé laisse une trace anonymisée plutôt qu'un trou.
- Les mots d'un changement sont ceux que le
  [point de reprise](/#/docs/console/configuration) de ce changement emploie, à la même
  seconde. Un événement, un vocabulaire.

## Issues

Les signalements que vos utilisateurs déposent depuis le bouton utilisateur injecté : une
description, une capture d'écran, et le contexte que le navigateur a relevé.

En haut, **Collect issue reports** - l'interrupteur d'un infra admin, livré **éteint**. Il
est ici plutôt que sur un écran de réglages divers, parce qu'une liste vide ne veut rien dire
tant qu'on ignore si quelque chose est collecté. Eteint, l'entrée disparaît du bouton
utilisateur.

La liste reste légère ; ouvrir un signalement va chercher son détail dans le tiroir, et le
tiroir est dans l'URL, donc un rafraîchissement y revient. Un signalement porte :

- Son **statut** - open, in progress, closed - changé depuis le tiroir, et filtre de la
  liste.
- Qui l'a signalé, depuis quelle organisation, et quand.
- L'**URL**, la taille de la fenêtre et le ratio de pixels, la langue, l'agent utilisateur.
- La **capture d'écran**, cliquable pour l'ouvrir en grand.
- La **sortie console** que la page avait relevée, avec le niveau de chaque ligne.
- Les **commentaires**, et une boîte pour en ajouter un.
- La suppression, dans la zone de danger.

Il n'y a pas de connecteur vers GitHub, GitLab ou Jira : un signalement vit ici.

## Pièges

- **Audit n'est pas un journal de requêtes.** Il enregistre les changements
  administratifs. Ce qui a traversé la passerelle est dans
  [Metrics](/#/docs/console/traffic).
- **Vous voyez votre propre périmètre.** Deux administrateurs peuvent lire le même écran et
  compter un nombre d'événements différent ; c'est le cadrage, pas un défaut.
- **Une liste Issues vide peut vouloir dire que la collecte est éteinte.** Vérifiez
  l'interrupteur avant de conclure que vos utilisateurs n'ont rien à dire.
- **Une capture est la page telle que l'utilisateur la voyait**, et elle peut porter ses
  données. Traitez un signalement comme une donnée personnelle.
