---
title: Capacités d'administration
section: Contrôle d'accès
order: 140
summary: Les cinq indicateurs d'un compte qui ouvrent la console et l'API d'administration, et ce que chacun débloque exactement.
---

# Capacités d'administration

Les rôles décident de ce qu'une personne atteint **à travers** la gateway.
Les capacités décident de ce qu'elle peut administrer **dans** la gateway.
Ce sont deux mécanismes distincts : une capacité ouvre la console et l'API
d'administration, et n'accorde rien sur le plan de données.

Ce sont des indicateurs posés sur le compte, que l'on active ou désactive
depuis sa ligne dans **Application > Users**.

| Capacité | Ce qu'elle administre |
|---|---|
| **root** | toute la gateway. Implique les deux suivantes |
| **infra admin** | le plan de routage : routes, upstreams, autorités, TLS, relais de messagerie |
| **app admin** | l'identité de l'application : utilisateurs, rôles, réglages, pages servies |
| **dev** | l'outillage développeur : clés de développement, substitution d'un service |
| **tenant creator** | peut créer des organisations, et possède celles qu'il crée |

*Tenant creator* n'apparaît que sur les installations qui comptent plusieurs
organisations : là où il n'y en a qu'une et où l'on ne peut pas en créer une
seconde, le badge n'accorderait rien.

## Ce que chacune ouvre dans la console

**infra admin** - le plan *Infra* :

- Les routes, avec tout l'éditeur de route, le testeur de routage et l'état de santé des upstreams
- La sécurité par endpoint et les rate limits par endpoint
- L'authentification : les autorités et leur configuration
- Le relais de messagerie, TLS et les certificats
- Le modèle de compte : les champs supplémentaires que porte un compte
- Les métriques

**app admin** - le plan *Application* :

- Les réglages généraux
- Les utilisateurs, et le catalogue global des rôles
- La sécurité : la politique de mot de passe, la limitation des tentatives, le second facteur, les passkeys, les jetons, la durée de vie des sessions
- Les pages intégrées : thème, disposition, marque - et le portail de navigation
- Sur une installation à une seule organisation, les groupes, les membres et les règles de groupe de cette organisation

**root uniquement**, en plus des deux :

- Les jetons d'accès au plan de contrôle, et la connexion d'un agent
- La configuration : export, import, points de reprise, historique, sauvegarde
- Le passage d'une seule organisation à plusieurs, et inversement
- L'obligation, pour tous les comptes à la fois, de changer de mot de passe
- L'attribution ou le retrait de **root**, et toute modification d'un compte root

Le dernier compte root actif ne peut être ni rétrogradé ni désactivé. La
gateway se garde toujours une porte ouverte.

![Un compte et les capacités qu'il porte, sur l'écran Users](img/console/users.webp)

## Les écrans qui n'appartiennent à aucun plan

Certaines sections sont ouvertes à plusieurs capacités. Leur **contenu** est
alors restreint côté serveur, au lieu que la page soit masquée :

| Section | Qui peut l'ouvrir | Ce qu'il y voit |
|---|---|---|
| Vault | root, infra admin, app admin | les entrées du plan qu'il administre |
| Audit trail | root, infra admin, app admin, l'administrateur d'une organisation | les événements de son propre domaine |
| Anomalies | les mêmes | les signalements de son propre domaine |
| API docs | root, infra admin, app admin | les specs qu'il a le droit de lire |
| Metrics | root, infra admin | tout ce qui est passé par la gateway |
| Organisations | tout utilisateur connecté à la console | les organisations qu'il administre |
| Licence | tout le monde | l'édition, et ce qu'elle débloque |

## Administrer une organisation n'est pas une capacité

Il n'existe pas d'indicateur *tenant admin*. Une personne administre une
organisation quand elle en est **propriétaire**, ou qu'elle y détient une
appartenance **ADMIN** active - voir [Organisations](/docs/access/tenants). La
console le calcule côté serveur et lui montre les sections d'organisation pour
les organisations concernées, et rien d'autre.

## La navigation est un confort, l'API est le contrat

La console masque ce que vous n'avez pas le droit d'utiliser : la navigation de
gauche est construite à partir des capacités que la gateway inscrit dans la
page, et un favori qui pointe vers une section interdite vous renvoie vers la
première qui vous est ouverte. C'est de l'ergonomie.

La règle est appliquée de nouveau à chaque appel de l'API d'administration, qui
répond `403` avec la phrase nommant ce qui est exigé - *infrastructure
administration requires root or the infra-admin capability*. Un appelant qui
contourne la console n'y gagne rien.

## Un jeton restreint, il n'élargit jamais

Un jeton du plan de contrôle agit avec les capacités de son propriétaire,
restreintes par son propre périmètre : un jeton limité au plan de routage ne
garde que *infra admin* et **perd root**, même si c'est un compte root qui l'a
émis. Un domaine qui laisserait root en place ne restreindrait rien. Voir
[Jetons d'API](/docs/auth/api-tokens).
