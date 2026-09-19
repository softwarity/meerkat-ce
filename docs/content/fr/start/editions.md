---
title: Éditions
section: Demarrer
order: 5
summary: Ce que fait l'image communautaire, ce que l'image Enterprise ajoute, et pourquoi la différence est l'image et non un réglage.
---

# Éditions

Meerkat est livré en deux images construites depuis le même commit :

| Image | Édition | Où |
|---|---|---|
| `docker.io/softwarity/meerkat` | communautaire | Docker Hub, public |
| `ghcr.io/softwarity/meerkat-ee` | Enterprise | registre privé |

Ce qui les sépare est le tag de compilation `ee`, décidé à la construction. Il
n'y a pas de fichier de licence à lire et pas de clé par fonctionnalité : la
plupart du code Enterprise n'est simplement pas dans le binaire communautaire,
donc il refuse par absence plutôt qu'en vérifiant quoi que ce soit. La ligne de
démarrage dit laquelle a démarré :

```
level=INFO msg=edition edition=ce enterprise=false
```

## Ce que seule l'image Enterprise fait

| Capacité | État |
|---|---|
| Plus d'une organisation - l'image communautaire en sert une, que la console ne nomme jamais | livré |
| LDAP et Active Directory comme autorité d'authentification (search-then-bind) | livré |
| Règles de groupe - ce qu'un annuaire, une équipe GitHub ou un claim OIDC déclare devient une appartenance et des rôles | livré |
| Cluster : plusieurs gateways sur une base PostgreSQL partagée, avec le bus de changement et le verrou consultatif | livré |
| Exposition Prometheus des compteurs, sur le plan de contrôle, éteinte par défaut | livré |
| Retirer la marque Meerkat des pages servies, et les dispositions de page au-delà de la centrée (split, drawer, banner, bare) | livré |
| Plus de trois configurations enregistrées à la fois | livré |
| Heures ouvrées - plages horaires, jours de semaine, fuseau | partiel : aucun contrôle en cours de session, et la fenêtre par membre n'est pas éditable dans la console |
| Le tunnel développeur : la machine d'un développeur répond sous un nom de service du cluster | partiel - voir les pages du mode développeur |

Deux autres sont **annoncées et pas écrites** : SAML 2.0, et l'export du journal
d'audit vers des formats analytiques. Le genre SAML peut être enregistré, et la
fabrique le refuse ensuite.

## Ce que font les deux images

Tout le reste, c'est-à-dire l'essentiel du produit : le routage avec ses
prédicats et ses filtres, TLS et ACME, le coffre, les mots de passe et leur
politique, le TOTP, les passkeys, OIDC et GitHub comme autorités, le catalogue de
rôles, les groupes, les règles d'accès par route et par endpoint, les limites de
débit, le journal d'audit, les tableaux de bord de trafic, le signalement
d'anomalies, le document de configuration, et la console elle-même.

Deux points méritent une phrase, parce qu'ils sont d'habitude la partie payante
ailleurs :

- **Les compteurs et les tableaux de bord sont dans les deux images.** Ce que vend Enterprise, c'est de les exporter vers la stack de supervision que vous faites déjà tourner. Une installation communautaire a ses courbes sans rien installer.
- **Le stockage embarqué est dans les deux images.** Ce que vend Enterprise, c'est la base externe, qui est ce qui fait de plusieurs gateways une seule installation.

## À quoi ressemble un refus

Une capacité Enterprise refusée par une image communautaire nomme l'acte, pas un
mot de catalogue tarifaire :

```
more than one organisation is part of the Enterprise edition,
and this is the community image
```

Deux refus sont volontairement souples, parce qu'un refus dur laisserait une
installation coincée :

- Passer en mono-organisation avec plusieurs organisations est **autorisé** et ne supprime rien ; les autres cessent d'être servies jusqu'à ce que quelqu'un revienne en arrière.
- Une disposition de page déjà en place continue d'être servie, et revenir à la disposition par défaut est toujours autorisé.

## Licences

Le tronc est sous
[FSL-1.1-Apache-2.0](https://github.com/softwarity/meerkat-ce/blob/main/LICENSE.md) :
libre d'usage, de copie, de modification et de redistribution pour tout usage
sauf construire un produit ou un service concurrent - l'usage interne et en
production dans votre entreprise est explicitement permis. Chaque version passe
sous Apache 2.0 deux ans après sa publication.

Les sources Enterprise ne sont pas dans l'arbre public. Elles sont visibles par
les clients et utilisables seulement sous accord commercial avec Softwarity.
