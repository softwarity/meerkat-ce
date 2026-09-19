---
title: Capacités d'administration
section: Contrôle d'accès
order: 140
summary: Les cinq drapeaux d'un compte qui ouvrent la console et l'API d'administration, et ce que chacun débloque.
---

# Capacités d'administration

Les rôles décident ce que quelqu'un atteint **à travers** la gateway. Les capacités
décident ce qu'il peut administrer **de** la gateway. Ce sont deux mécanismes
séparés : une capacité ouvre la console et l'API d'administration, et n'accorde rien
sur le plan de données.

Ce sont des drapeaux sur le compte, basculés depuis la ligne dans **Application >
Users**.

| Capacité | Ce qu'elle administre |
|---|---|
| **root** | toute la gateway. Implique les deux suivantes |
| **infra admin** | le plan de routage : routes, amonts, autorités, TLS, relais mail |
| **app admin** | l'identité de l'application : comptes, rôles, réglages, pages servies |
| **dev** | l'outillage développeur : clés de dev, substitution d'un service |
| **tenant creator** | peut créer des organisations, et possède celles qu'il crée |

*tenant creator* n'apparaît que sur les installations à plus d'une organisation : là
où il y en a une et où une seconde ne peut pas être créée, le badge n'accorderait
rien.

## Ce que chacune ouvre dans la console

**infra admin** - le plan *Infra* :

- Les routes, avec tout l'éditeur de route, le testeur de routage et la santé des amonts
- La sécurité et les limites de débit par endpoint
- Authentication : les autorités et leurs paramètres
- Mail relay, TLS et les certificats
- Le modèle de compte : quels champs supplémentaires un compte porte
- Les métriques

**app admin** - le plan *Application* :

- Les réglages généraux et les langues
- Les comptes, et le catalogue global de rôles
- Security : la politique de mot de passe, l'étranglement, le second facteur, les passkeys, les jetons, la durée de session
- Built-in pages : thème, disposition, marque - et le portail de navigation
- Sur une installation à une organisation, les groupes, membres et règles de groupe de cette organisation

**root seulement**, en plus des deux :

- Les jetons d'accès du plan de contrôle, et le branchement d'un agent
- La configuration : export, import, points de restauration, historique, sauvegarde
- Le basculement entre une et plusieurs organisations
- Forcer un changement de mot de passe sur tous les comptes d'un coup
- Accorder ou retirer **root**, et modifier un compte root du tout

Le dernier root actif ne peut être ni rétrogradé ni désactivé. La gateway se garde
une porte ouverte.

![Un compte et les capacités qu'il porte, sur l'écran Utilisateurs](img/console/users.webp)

## Des écrans qui n'appartiennent à aucun plan

Certaines sections sont ouvertes à plusieurs capacités, le **contenu** étant filtré
côté serveur plutôt que la page cachée :

| Section | Qui peut l'ouvrir | Ce qu'il voit |
|---|---|---|
| Vault | root, infra admin, app admin | les entrées du plan qu'il administre |
| Audit | root, infra admin, app admin, l'admin d'une organisation | les événements de son propre domaine |
| Anomalies | les mêmes | les signalements de son propre domaine |
| API docs | root, infra admin, app admin | les specs qu'il peut lire |
| Métriques | root, infra admin | tout ce qui est passé par la gateway |
| Organisations | tout utilisateur connecté à la console | les organisations qu'il administre |
| Licence | tout le monde | l'édition, et ce qu'elle débloque |

## Administrer une organisation n'est pas une capacité

Il n'y a pas de drapeau *tenant admin*. Quelqu'un administre une organisation quand
il la **possède**, ou qu'il y détient une appartenance **ADMIN** activée - voir
[Organisations](/docs/access/tenants). La console calcule cela côté serveur et lui
montre les sections des organisations concernées, et rien d'autre.

## La navigation est un confort, l'API est le contrat

La console cache ce que vous ne pouvez pas utiliser - le menu de gauche est construit
depuis les capacités que la gateway estampille sur la page, et un signet vers une
section que vous ne pouvez pas ouvrir rebondit vers la première que vous pouvez.
C'est de l'ergonomie.

La règle est réappliquée à chaque appel de l'API d'administration, qui répond `403`
avec la phrase nommant ce qui est requis - *infrastructure administration requires
root or the infra-admin capability*. Un appelant qui contourne la console ne gagne
rien.

## Les jetons rétrécissent, ils n'élargissent jamais

Un jeton du plan de contrôle agit avec les capacités de son propriétaire, restreintes
par son propre périmètre : un jeton limité au plan de routage ne garde qu'*infra
admin* et **perd root**, même si c'est root qui l'a émis. Un domaine qui laisserait
root debout ne confinerait rien. Voir [Jetons d'API](/docs/auth/api-tokens).
