---
title: Organisations
section: La console
order: 178
summary: Une organisation à la fois - son nom, ses heures, ses groupes, ses membres, ses règles, et les gestes destructeurs.
---

# Organisations

L'entrée **Tenants** n'existe que si la passerelle sert
[plusieurs organisations](/#/docs/console/application). La cliquer dépose sur la première
organisation que vous administrez ; le tiroir liste les autres et porte le bouton **New
tenant**. Il n'y a pas de page de liste : une organisation est une chose dans laquelle on
travaille.

Les mêmes écrans servent root, le propriétaire de l'organisation et ses administrateurs -
l'API limite chaque appel.

## Les sections

Chacune est une route à elle, donc les liens profonds fonctionnent.

| Section | Ce qu'elle porte |
|---|---|
| **General** | Le nom, la description, l'interrupteur d'activation, et les heures ouvrées |
| **Groups** | La matrice rôles/groupes, et le mode de groupe |
| **Members** | Qui est dans cette organisation et dans quels groupes |
| **Group rules** | Ce qu'une autorité déclare, transformé en appartenance ici |
| **Danger zone** | Le transfert de propriété, et la suppression |

**Groups**, **Members** et **Group rules** sont les écrans documentés dans
[Groupes, Membres et Règles de groupe](/#/docs/console/organisation) - en mode
mono-organisation ils sont sous Application, parce qu'il y a une organisation et que
personne ne la nomme.

## General

- **Name** et **Description** - le nom est ce que montrent le tiroir, les règles d'accès
  et le sélecteur d'organisation à la connexion.
- **Enabled** - fait partie du Save de cette section, ce n'est pas un interrupteur dans
  l'en-tête.
- Sous le nom, la ligne dit quand l'organisation a été créée et par qui, et qui la
  possède.
- **Working hours** - la fenêtre d'accès propre à cette organisation, ou celle de
  l'application si elle n'en définit pas. Le formulaire dit ce dont il hérite.

> [!NOTE]
> Edition Enterprise : les heures ouvrées.

## Danger zone

- **Transfer ownership** - une organisation a un seul propriétaire, et la propriété est
  indépendante de l'appartenance : le précédent propriétaire garde son appartenance
  inchangée, et le nouveau n'a pas besoin d'être administrateur.
- **Delete this tenant** - retire l'organisation, ses appartenances et ses groupes, avec
  une confirmation à recopier. Il n'y a pas de retour.

## Pièges

- **Désactiver une organisation coupe l'accès de tous ceux qui en ont besoin.** Cela ne
  supprime rien.
- **Rebasculer la passerelle en mono-organisation ne sert que la première.** Les autres ne
  sont pas supprimées, et l'écran **License** dit combien sont retenues.
- **Un membre n'est pas un compte.** Créez d'abord le compte dans
  [Users](/#/docs/console/users) ; ici, vous le placez.
