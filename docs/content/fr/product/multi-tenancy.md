---
title: Multi-tenance
section: Le produit
order: 6.5
summary: Plusieurs organisations derrière une même gateway - clients, filiales ou services - chacune avec ses membres, ses groupes, ses rôles et ses administrateurs, et l'organisation transmise à vos services avec chaque requête.
---

# Multi-tenance

Plusieurs clients, filiales ou services utilisent souvent les mêmes
applications internes, et ne doivent pas se voir. Meerkat les sépare à la
porte : chacun est une **organisation**, et chaque requête est faite dans une
seule d'entre elles.

> [!NOTE] **Édition Enterprise.**
> Plus d'une organisation relève de l'édition Enterprise. L'image Community sert
> une seule organisation, qu'aucun écran ne nomme.

## Ce que contient une organisation

- **Des membres**, chacun administrateur ou utilisateur de cette organisation,
  actif ou non.
- **Des groupes**, qui donnent des rôles du catalogue de la gateway. Les rôles
  d'une personne sont ceux de ses groupes dans l'organisation où elle travaille.
- **Un propriétaire**, qui n'a pas besoin d'en être membre.
- **Des réglages à elle** : durée de session, mode des groupes, et en
  Enterprise ses heures d'accès.

Un administrateur d'organisation gère ses membres, ses groupes et ses règles de
groupe depuis la console, et rien en dehors.

## À la connexion

Une personne qui n'a qu'une appartenance est placée dans cette organisation sans
question. Une personne qui en a plusieurs en choisit une, et peut en changer
ensuite depuis le bouton utilisateur. En Enterprise, une règle de groupe peut
faire d'un groupe d'annuaire, d'une équipe GitHub ou d'un claim OpenID Connect
une appartenance, à chaque connexion.

## Ce que reçoivent vos services

L'organisation voyage avec la requête, à côté de l'identité et des rôles, dans
les en-têtes ou le JWT signé qu'envoie la route. Les règles de route et les
règles par endpoint sont jugées dans cette organisation. Un service apprend
dans quelle organisation travaille l'appelant comme un fait, pas comme un
paramètre choisi par l'appelant.

## Pour aller plus loin

- [Organisations](/docs/access/tenants), dans la documentation : les
  appartenances, le choix à la connexion, ce qu'une organisation peut
  surcharger, l'isolation.
- [Rôles et groupes](/docs/access/roles).
- [Éditions](/product/editions), et comment essayer Enterprise gratuitement.
