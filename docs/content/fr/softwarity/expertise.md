---
title: Notre expertise
section: Softwarity
order: 2
summary: Angular et Go, les gateways et l'identité, et cette part d'une application interne que personne ne veut écrire deux fois.
---

# Notre expertise

Nous travaillons aux deux extrémités d'une application interne : la **porte
d'entrée** par laquelle arrive une requête, et l'**écran** dont un exploitant
se sert réellement. Ce sont les deux endroits où une équipe informatique perd
le plus de temps et en retire le moins de reconnaissance.

## La porte d'entrée

Une gateway n'est pas un routeur. Devant une application interne, c'est
l'endroit où l'identité, les règles d'accès, les organisations, les quotas et
l'audit existent - faute de quoi ils sont dispersés dans chacun des services
qui se trouvent derrière. Ce terrain, nous le connaissons :

- **Un reverse proxy bien fait** : prédicats et filtres, rechargement à chaud,
  propreté des en-têtes, et ces réponses de longue durée (WebSocket, canaux en
  direct) que toutes les configurations par défaut cassent.
- **L'identité** : sessions, jetons signés transmis aux upstreams, OpenID Connect,
  LDAP et Active Directory, second facteur, passkeys. Et la ligne que nous ne
  franchissons pas : un annuaire externe authentifie, il ne décide jamais des
  rôles.
- **Des autorisations qui résistent à un audit** : rôles hiérarchiques, groupes
  par organisation, une règle par route et par endpoint, et un journal qui dit
  qui a changé quoi, avec l'avant et l'après.

## L'écran

Une console d'administration est un produit, pas un formulaire posé sur une
base de données. Nous construisons les nôtres avec Angular, sur la version
majeure en cours, en donnant la priorité aux signaux, avec Material comme
système de design et non comme un catalogue de composants - et nous publions
les composants que nous avons dû écrire, au lieu de les transporter d'un projet
à l'autre.

Nous apportons le même soin aux pages que voit un utilisateur final : le
parcours de connexion, le second facteur, le menu du compte, la navigation
entre les applications. Ce sont ces pages qui décident de ce que les gens
pensent de tout le système, et ce sont en général les dernières auxquelles on
s'intéresse.

## Les langages que nous choisissons, et pourquoi

::: grid
### Go pour ce qui s'exécute

Un seul binaire statique, aucun environnement d'exécution à installer, aucune
dépendance à corriger à trois heures du matin. Une gateway qui démarre en
quelques millisecondes et garde une connexion ouverte pendant des heures, c'est
un programme Go, et la bibliothèque standard fait l'essentiel du travail.

### Angular pour ce qui se regarde

Les grandes applications internes vivent dix ans et changent quatre fois de
mains. Les partis pris d'Angular - une structure, un typage, un chemin de mise
à niveau documenté plutôt qu'improvisé - valent davantage, sur cette durée, que
la liberté d'organiser un projet selon l'humeur du dernier développeur.
:::

## Travailler avec nous

> [!NOTE]
> Notre façon d'intervenir - support, intégration, développements sur mesure
> autour des produits - sera décrite ici ; le texte est en cours de rédaction.
> En attendant, vous trouverez comment nous joindre
> [sur la page de l'équipe](/softwarity/team).
