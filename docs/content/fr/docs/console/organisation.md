---
title: Groupes, membres et règles de groupe
section: La console
order: 172
summary: Relier les personnes aux rôles - les groupes d'un tenant, leurs membres, et les règles qui les alimentent depuis un annuaire.
---

# Groupes, membres et règles de groupe

Trois écrans pour administrer **le tenant servi**. En mode mono-tenant,
ils se trouvent sous **Application**, puisqu'il n'y a qu'un tenant et que
personne ne le nomme. Quand il y en a plusieurs, les mêmes écrans s'ouvrent pour chaque
tenant depuis [Tenants](/docs/console/tenants), là où se pose la question
*lequel*.

La chaîne est courte : un **rôle** est global, un **groupe** est un ensemble de rôles
au sein d'un tenant, et une **appartenance** place une personne dans des
groupes.

## Groups

![L'écran Groups : le catalogue des rôles en lignes, trois colonnes de groupes, et une coche là où un rôle appartient à un groupe](img/console/groups.webp)

Trois groupes - Finance, Support, Warehouse - en regard du catalogue. Chaque ligne
affiche la description du rôle au-dessus de son nom technique, et la ligne *Whole
column* se trouve juste sous les en-têtes.

Une matrice : le **catalogue des rôles** en lignes, les **groupes** de ce
tenant en colonnes, et une coche là où un rôle appartient à un groupe.

- **Cocher un rôle parent coche tous ceux qui en dépendent.** C'est la hiérarchie du
  catalogue qui fait le travail.
- **Le nom d'un groupe**, en tête de sa colonne, ouvre son tiroir (plus bas) ; le
  bouton rond + en ajoute un. Les groupes se créent et se modifient ici seulement :
  dans Members, ils ne font que titrer leurs colonnes.
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

Sur l'écran Groups, le nom d'un groupe (son en-tête de colonne, un bouton tonal)
ouvre **son tiroir** : nom,
description et, en mode exclusif seulement, ses **heures ouvrées** (Enterprise),
qui remplacent celles du compte et du tenant pour qui se connecte avec ce
groupe. Save enregistre ; la suppression est dans la danger zone du tiroir, après
confirmation.

## Members

![L'écran Members : les comptes en lignes, les trois mêmes colonnes de groupes, et la dernière connexion](img/console/members.webp)

La même installation en mode mono-tenant : la matrice est identique à celle d'un
tenant, appartenance et badge admin compris - un tenant unique reste un
tenant.

Encore une matrice : les comptes en lignes, les groupes de ce tenant en
colonnes.

- La **première colonne** représente l'appartenance elle-même, dans les deux modes : la
  coche fait entrer dans le tenant ou en fait sortir, et le badge **admin**
  voisin promeut un membre au rang d'administrateur du tenant. Le propriétaire
  affiche à la place un badge **owner** en lecture seule ; la propriété se transfère
  depuis la Danger zone du tenant.
- Cocher un groupe pour une personne qui n'est pas encore membre en fait un membre ; la
  case de colonne entière agit sur les membres affichés.
- La dernière colonne indique la dernière connexion et donne accès à la
  réinitialisation du mot de passe, limitée à ce tenant : un mot de passe
  temporaire affiché une fois, ou un lien envoyé par mail au membre pour en choisir
  un nouveau.
- Deux boutons sur la ligne d'un membre ouvrent **son tiroir**, chacun sur sa page :
  l'horloge sur ses **heures ouvrées** dans ce tenant (Enterprise), propres
  ou héritées, et les dates entre lesquelles son accès court, valables dans les deux
  cas ; l'historique sur son **historique de connexion** dans ce tenant.

Members et [Users](/docs/console/users) répondent à deux questions différentes et
restent séparés : Users traite du compte, cet écran de son affectation.

## Group rules

![Une règle de groupe ouverte dans son tiroir : l'autorité, le groupe amont, et le groupe du tenant qu'elle accorde](img/console/group-rules.webp)

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

- **En mode multi-tenant, ces entrées disparaissent d'Application.** Elles sont
  propres à chaque tenant, sous Tenants.
- **Le mode Exclusive modifie ce que les personnes voient à la connexion** : on leur
  demande de choisir un groupe. Ne l'activez pas sur une installation en service sans
  prévenir personne.
- **Un groupe sans rôle n'accorde rien**, et un rôle qui n'est dans aucun groupe ne
  parvient à personne.
- **Supprimer un groupe fait perdre ses affectations.** Les personnes restent, mais pas
  les rôles qu'elles tenaient de ce groupe.
