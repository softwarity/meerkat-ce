---
title: Groupes, Membres et Règles de groupe
section: La console
order: 172
summary: Lier les gens aux rôles - les groupes d'une organisation, qui est dedans, et les règles qui les remplissent depuis un annuaire.
---

# Groupes, Membres et Règles de groupe

Trois écrans qui administrent **l'organisation servie**. En mode mono-organisation ils
vivent sous **Application**, parce qu'il y en a une et que personne ne la nomme. Avec
plusieurs, les mêmes écrans se joignent par organisation depuis
[Tenants](/#/docs/console/tenants), là où la question *laquelle* est posée.

La chaîne est courte : un **rôle** est global, un **groupe** est un sac de rôles dans une
organisation, et une **appartenance** met une personne dans des groupes.

## Groups

![L'écran Groups : le catalogue des rôles sur le côté, trois colonnes de groupes, une coche là où un rôle appartient à un groupe](img/console/groups.webp)

Trois groupes - Finance, Front desk, Warehouse - face au catalogue. Chaque ligne montre
la description du rôle au-dessus de son nom technique, et la ligne *Whole column* est
juste sous les en-têtes.

Une matrice : le **catalogue des rôles** sur le côté, les **groupes** de cette
organisation en colonnes, une coche là où un rôle appartient à un groupe.

- **Cocher un rôle parent coche ce qui pend en dessous.** C'est la hiérarchie du
  catalogue qui travaille.
- **Le menu d'une colonne** la renomme ou la supprime ; le bouton rond + ajoute un
  groupe.
- **La ligne Whole column**, sous les en-têtes, coche ou vide une colonne entière sur les
  lignes actuellement à l'écran : ce que l'on voit est ce que l'on coche.
- **La recherche et le sélecteur de tag** restreignent les lignes. Sur un grand
  catalogue, les tags sont ce qui rend cet écran praticable.
- Tout s'enregistre au clic.

Au-dessus de la matrice, **Group mode** décide comment les groupes d'une personne se
combinent :

| Mode | Effet |
|---|---|
| **Cumulative** | Les rôles de tous les groupes de la personne sont fusionnés |
| **Exclusive** | Un seul groupe est choisi à la connexion |

## Members

![L'écran Members : les comptes sur le côté, les trois mêmes colonnes de groupes, et la dernière connexion](img/console/members.webp)

La même installation en mode mono-organisation : pas de colonne d'appartenance ni de
pastille admin, seulement qui est dans quel groupe, et la dernière connexion de chaque
compte.

Une matrice encore : les comptes sur le côté, les groupes de cette organisation en
colonnes.

- En mode multi-organisations, la **première colonne** est l'appartenance elle-même : la
  coche fait entrer ou sortir, et la pastille **admin** à côté promeut un membre en
  administrateur de l'organisation. Le propriétaire porte à la place une pastille
  **owner** en lecture seule, et la propriété se transfère dans la zone de danger de
  l'organisation.
- En mode mono-organisation ces deux colonnes disparaissent : un compte activé **est**
  membre ici, et la capacité `app admin` dit déjà qui administre.
- Les colonnes de groupes sont désactivées pour un non-membre : il faut d'abord le faire
  entrer.
- La dernière colonne montre la dernière connexion et porte la réinitialisation de mot de
  passe, limitée à cette organisation.

Members et [Users](/#/docs/console/users) répondent à deux questions différentes et
restent séparés : Users, c'est le compte ; ici, c'est l'affectation.

## Group rules

Ce qu'une **autorité** déclare, transformé en appartenance et en groupes ici. Une
organisation ou une équipe GitHub, un groupe LDAP, une revendication d'un fournisseur
d'identité : une règle dit *quiconque vient de cette autorité*, ou *quiconque porte le
groupe X*, et accorde les groupes à droite.

L'écran refuse de montrer un tableau vide avec un bouton qui ne mène nulle part : sans
autorité activée il dit d'aller en ajouter une, et sans groupe il dit d'en créer un
d'abord.

> [!NOTE]
> Edition Enterprise : les règles de groupe viennent avec la fonctionnalité annuaires.

Deux choses surprennent à propos des règles :

- **Rien ne change avant que les gens se reconnectent.** Une règle s'applique à
  l'arrivée, donc en supprimer une reprend ce qu'elle accordait à la prochaine connexion
  de chacun.
- **Ce qu'un administrateur a placé à la main reste.** Une règle remplit des groupes,
  elle ne les possède pas.

## Pièges

- **En mode multi-organisations ces entrées disparaissent d'Application.** Elles sont par
  organisation, sous Tenants.
- **Le mode Exclusive change ce que les gens voient à la connexion** : on leur demande
  quel groupe. Ne le basculez pas sur une installation vivante sans prévenir.
- **Un groupe sans rôle n'accorde rien**, et un rôle dans aucun groupe n'atteint
  personne.
- **Supprimer un groupe perd ses affectations.** Les gens restent, leurs rôles issus de ce
  groupe non.
