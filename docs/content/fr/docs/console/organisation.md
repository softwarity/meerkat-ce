---
title: Groupes, membres et règles de groupe
section: La console
order: 172
summary: Relier les personnes aux rôles - les groupes d'une organisation, leurs membres, et les règles qui les alimentent depuis un annuaire.
---

# Groupes, membres et règles de groupe

Trois écrans pour administrer **l'organisation servie**. En mode mono-organisation,
ils se trouvent sous **Application**, puisqu'il n'y a qu'une organisation et que
personne ne la nomme. Quand il y en a plusieurs, les mêmes écrans s'ouvrent pour chaque
organisation depuis [Tenants](/docs/console/tenants), là où se pose la question
*laquelle*.

La chaîne est courte : un **rôle** est global, un **groupe** est un ensemble de rôles
au sein d'une organisation, et une **appartenance** place une personne dans des
groupes.

## Groups

![L'écran Groups : le catalogue des rôles en lignes, trois colonnes de groupes, et une coche là où un rôle appartient à un groupe](img/console/groups.webp)

Trois groupes - Finance, Front desk, Warehouse - en regard du catalogue. Chaque ligne
affiche la description du rôle au-dessus de son nom technique, et la ligne *Whole
column* se trouve juste sous les en-têtes.

Une matrice : le **catalogue des rôles** en lignes, les **groupes** de cette
organisation en colonnes, et une coche là où un rôle appartient à un groupe.

- **Cocher un rôle parent coche tous ceux qui en dépendent.** C'est la hiérarchie du
  catalogue qui fait le travail.
- **Le menu d'une colonne de groupe** renomme ou supprime ce groupe ; le bouton rond +
  en ajoute un.
- **La ligne Whole column**, sous les en-têtes, coche ou décoche un groupe entier pour
  les lignes actuellement affichées : vous cochez ce que vous voyez.
- **La recherche et le sélecteur de tags** réduisent le nombre de lignes. Sur un grand
  catalogue, ce sont les tags qui rendent cet écran utilisable.
- Tout est enregistré dès le clic.

Au-dessus de la matrice, **Group mode** détermine comment se combinent les groupes
d'une personne :

| Mode | Effet |
|---|---|
| **Cumulative** | Les rôles de tous les groupes de la personne sont réunis |
| **Exclusive** | Un seul groupe est choisi à la connexion |

## Members

![L'écran Members : les comptes en lignes, les trois mêmes colonnes de groupes, et la dernière connexion](img/console/members.webp)

La même installation en mode mono-organisation : la matrice est identique à celle d'une
organisation, appartenance et badge admin compris - une organisation unique reste une
organisation.

Encore une matrice : les comptes en lignes, les groupes de cette organisation en
colonnes.

- La **première colonne** représente l'appartenance elle-même, dans les deux modes : la
  coche fait entrer dans l'organisation ou en fait sortir, et le badge **admin**
  voisin promeut un membre au rang d'administrateur de l'organisation. Le propriétaire
  affiche à la place un badge **owner** en lecture seule ; la propriété se transfère
  depuis la Danger zone de l'organisation.
- Cocher un groupe pour une personne qui n'est pas encore membre en fait un membre ; la
  case de colonne entière agit sur les membres affichés.
- La dernière colonne indique la dernière connexion et donne accès à la
  réinitialisation du mot de passe, limitée à cette organisation.

Members et [Users](/docs/console/users) répondent à deux questions différentes et
restent séparés : Users traite du compte, cet écran de son affectation.

## Group rules

![Une règle de groupe ouverte dans son tiroir : l'autorité, le groupe amont, et le groupe de l'organisation qu'elle accorde](img/console/group-rules.webp)

Ce que déclare une **autorité**, converti ici en appartenance et en groupes. Une
organisation ou une équipe GitHub, un groupe LDAP, une revendication (claim) émise par
un fournisseur d'identité : une règle énonce *toute personne venant de cette autorité*,
ou *toute personne dont le groupe est X*, et attribue les groupes indiqués à droite.

L'écran refuse d'afficher un tableau vide avec un bouton qui ne mène nulle part : sans
autorité activée, il invite à en ajouter une, et sans groupe, à en créer un d'abord.

> [!NOTE]
> Édition Enterprise : les règles de groupe sont fournies avec la fonctionnalité des
> annuaires.

Deux caractéristiques des règles qui surprennent :

- **Rien ne change tant que les personnes ne se sont pas reconnectées.** Une règle
  s'applique à l'arrivée : en supprimer une retire donc ce qu'elle accordait à la
  prochaine connexion de chaque personne.
- **Ce qu'un administrateur a attribué à la main est conservé.** Une règle alimente
  les groupes ; ils ne lui appartiennent pas.

## Pièges

- **En mode multi-organisation, ces entrées disparaissent d'Application.** Elles sont
  propres à chaque organisation, sous Tenants.
- **Le mode Exclusive modifie ce que les personnes voient à la connexion** : on leur
  demande de choisir un groupe. Ne l'activez pas sur une installation en service sans
  prévenir personne.
- **Un groupe sans rôle n'accorde rien**, et un rôle qui n'est dans aucun groupe ne
  parvient à personne.
- **Supprimer un groupe fait perdre ses affectations.** Les personnes restent, mais pas
  les rôles qu'elles tenaient de ce groupe.
