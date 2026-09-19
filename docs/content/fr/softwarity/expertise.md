---
title: Notre expertise
section: Softwarity
order: 2
summary: Angular et Go, les passerelles et l'identité, et la part d'une application interne que personne ne veut écrire deux fois.
---

# Notre expertise

Nous travaillons aux deux extrémités d'une application interne : la **porte
d'entrée** par laquelle une requête arrive, et l'**écran** qu'un opérateur
utilise vraiment. Ce sont les deux endroits où une équipe IT perd le plus de
temps et reçoit le moins de reconnaissance.

## La porte d'entrée

Une passerelle n'est pas un routeur. Devant une application interne, c'est
l'endroit où l'identité, les règles d'accès, les organisations, les quotas et
l'audit existent - ou bien sont éparpillés dans chacun des services derrière.
Nous connaissons ce terrain :

- **Le reverse proxy fait correctement** : prédicats et filtres, rechargement à
  chaud, hygiène des en-têtes, et les réponses longues (WebSocket, canaux live)
  que toutes les configurations par défaut cassent.
- **L'identité** : sessions, jetons signés vers l'amont, OpenID Connect, LDAP et
  Active Directory, second facteur, passkeys. Et la ligne que nous ne
  franchissons pas : un annuaire externe authentifie, il ne décide jamais des
  rôles.
- **Une autorisation qui survit à un audit** : rôles hiérarchiques, groupes par
  organisation, une règle par route et par endpoint, et une trace qui dit qui a
  changé quoi, avec l'avant et l'après.

## L'écran

Une console d'administration est un produit, pas un formulaire posé sur une
base. Nous les construisons en Angular, sur la version majeure courante, en
signal-first, avec Material comme système de design plutôt que comme magasin de
composants - et nous publions ceux qu'il a fallu écrire, au lieu de les
transporter de projet en projet.

Le même soin va aux pages que rencontre un utilisateur final : le flux de
connexion, le second facteur, le menu de compte, la navigation entre
applications. Ce sont ces pages qui décident de ce que les gens pensent de tout
le système, et ce sont généralement les dernières que quelqu'un regarde.

## Les langages que nous choisissons, et pourquoi

::: grid
### Go pour ce qui tourne

Un binaire statique, aucun runtime à installer, aucune dépendance à corriger à
trois heures du matin. Une passerelle qui démarre en millisecondes et tient une
connexion pendant des heures est un programme Go, et la bibliothèque standard
fait l'essentiel du travail.

### Angular pour ce qui est regardé

Les grandes applications internes vivent dix ans et changent quatre fois de
mains. Les partis pris d'Angular - une structure, du typage, un chemin de mise à
jour documenté plutôt qu'improvisé - valent plus sur cette durée que la liberté
d'arranger un projet comme le dernier développeur le sentait.
:::

## Travailler avec nous

> [!NOTE]
> La façon dont nous intervenons - support, intégration, développement sur
> mesure autour des produits - est en cours d'écriture ici. En attendant, la
> manière de nous joindre est [sur la page de l'équipe](/softwarity/team).
