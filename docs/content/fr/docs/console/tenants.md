---
title: Organisations
section: La console
order: 178
summary: Une organisation à la fois - son nom, ses horaires, ses groupes, ses membres, ses règles et les opérations irréversibles.
---

# Organisations

L'entrée **Tenants** n'existe que lorsque la gateway sert
[plusieurs organisations](/docs/console/application). Un clic dessus vous amène sur la
première organisation que vous administrez ; le tiroir liste les autres et contient le
bouton **New tenant**. Il n'y a pas de page de liste : une organisation est un espace
dans lequel vous travaillez.

Les mêmes écrans servent au compte root, au propriétaire de l'organisation et à ses
administrateurs : c'est l'API qui délimite le périmètre de chaque appel.

## Les sections

Chaque section est une route à part entière : vous pouvez donc créer un lien direct
vers l'une d'elles.

| Section | Ce qu'elle contient |
|---|---|
| **General** | Le nom, la description, l'interrupteur d'activation et les horaires de travail |
| **Groups** | La matrice des rôles par groupe, et le mode de groupe |
| **Members** | Qui fait partie de cette organisation, et de quels groupes |
| **Group rules** | Ce que déclare une autorité, traduit ici en appartenances |
| **Danger zone** | Le transfert de propriété et la suppression |

**Groups**, **Members** et **Group rules** sont les écrans décrits dans
[Groups, Members et Group rules](/docs/console/organisation). En mode
mono-organisation, ils se trouvent sous Application, puisqu'il n'y a qu'une seule
organisation et que personne ne la nomme.

## General

- **Name** et **Description** - le nom est ce qu'affichent le tiroir, les règles
  d'accès et le sélecteur d'organisation de la page de connexion.
- **Enabled** - il est enregistré avec le bouton Save de cette section ; ce n'est pas
  un interrupteur placé dans l'en-tête.
- Sous le nom, une ligne indique quand l'organisation a été créée, par qui, et qui en
  est le propriétaire.
- **Working hours** - la plage d'accès propre à cette organisation ou, si elle n'en
  définit pas, celle de l'application. Le formulaire indique de quoi il hérite.

> [!NOTE]
> Édition Enterprise : les horaires de travail.

## Danger zone

- **Transfer ownership** - une organisation n'a qu'un seul propriétaire, et la
  propriété ne dépend pas de l'appartenance : l'ancien propriétaire conserve son
  appartenance telle quelle, et le nouveau n'a pas besoin d'être administrateur.
- **Delete this tenant** - supprime l'organisation, ses appartenances et ses groupes,
  après une confirmation qui vous demande de saisir son nom. L'opération est
  irréversible.

## Pièges

- **Désactiver une organisation coupe l'accès de tous ses membres**, partout où une
  organisation est exigée. Rien n'est supprimé.
- **Repasser la gateway en mode mono-organisation ne sert plus que la première.**
  Les autres ne sont pas supprimées, et l'écran **License** indique combien ne sont
  plus servies.
- **Un membre n'est pas un compte.** Créez d'abord le compte sur l'écran
  [Users](/docs/console/users) ; ici, vous le rattachez à l'organisation.
