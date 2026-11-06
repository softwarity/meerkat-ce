---
title: Rôles et groupes
section: Contrôle d'accès
order: 132
summary: Un catalogue de rôles pour toute la gateway, des groupes par tenant, et la manière dont se calculent les rôles effectifs d'une personne.
---

# Rôles et groupes

Les rôles sont les noms dans lesquels s'écrivent vos règles - `orders-reader`,
`support`, `billing-admin`. Les groupes sont le moyen par lequel les personnes
les obtiennent.

Ce découpage est voulu : **le catalogue est commun à toute la gateway, les
groupes appartiennent à un tenant**. Un seul vocabulaire pour toute
l'installation, assemblé différemment dans chaque tenant.

## Le catalogue

**Application > Roles.** Un rôle a un nom, unique dans toute l'installation,
une description, des étiquettes (tags) facultatives et au plus un parent.

Les noms de rôle s'écrivent aussi dans les règles des routes : ils sont donc
limités aux lettres, aux chiffres, à `-` et à `_`. Un rôle dont le nom ne
respecte pas cette contrainte est refusé quand une route tente de l'utiliser.

**Un parent implique ses enfants.** Détenir un rôle parent, c'est détenir ce
rôle et tout ce qui se trouve en dessous, jusqu'en bas :

```
staff
  support
    support-lead
  billing
```

Une personne qui détient `staff` satisfait une règle exigeant `support-lead`.
Une personne qui détient `support` satisfait `support-lead`, mais pas
`billing`. Pour accorder une branche, accordez son sommet.

Un rôle ne peut pas devenir son propre ancêtre : un cycle est refusé. Supprimer
un rôle fait remonter ses enfants au premier niveau, au lieu de les supprimer
avec lui, et retire ce rôle de tous les groupes qui le portaient. Certains
rôles sont marqués comme rôles **système** et ne peuvent pas être supprimés du
tout.

Les étiquettes sont un classement libre - une par microservice, par exemple -
et elles accompagnent le rôle partout où des expressions peuvent les lire.
Elles n'ont aucun effet sur les décisions d'accès.

![Le catalogue des rôles : une hiérarchie, et ce que chaque rôle ouvre](img/console/roles.webp)

## Les groupes

**Un groupe appartient à un seul tenant** et réunit un ensemble de
rôles du catalogue. Son nom est unique au sein de ce tenant. Des
groupes sont ensuite attribués à chaque membre du tenant, et ses rôles
sont ceux de ces groupes.

Rien d'autre n'accorde un rôle. Il n'y a pas de rôle porté par un compte, ni de
rôle en dehors d'un tenant :

> [!WARNING]
> Une session **sans tenant actif ne détient aucun rôle**. Ni les rôles
> d'un autre tenant, ni un sous-ensemble : aucun. Toute règle qui exige
> un rôle exige donc aussi un tenant actif, même si elle n'en mentionne
> pas.

![Les groupes d'un tenant, chacun accordant un ensemble de rôles à ses membres](img/console/groups.webp)

## Un groupe à la fois, ou tous

Chaque tenant choisit la manière dont ses groupes se combinent :

| Mode | Ce qu'il fait |
|---|---|
| Cumulative (valeur par défaut) | tous les groupes attribués à la personne comptent en même temps |
| Exclusive | un seul groupe s'applique, choisi à la connexion |

Le mode Exclusive répond au cas d'une personne qui porte deux casquettes dans
le même tenant, sans pouvoir les porter en même temps. Quand il est
activé et que la personne a plusieurs groupes, elle est envoyée sur
`/select-group`, et peut en changer plus tard depuis le bouton utilisateur.
Une session en mode Exclusive qui n'a pas encore choisi ne porte **aucun
rôle** - la même règle que sans tenant.

Il n'y a pas de valeur par défaut à l'échelle de l'installation : c'est une
propriété du tenant, et un tenant qui ne précise rien est en
mode Cumulative.

## Comment se calculent les rôles effectifs

À chaque requête, pour le compte, le tenant actif et le groupe actif :

1. prendre les groupes attribués dans **ce** tenant - ou seulement le groupe actif, en mode Exclusive ;
2. prendre les rôles de ces groupes ;
3. développer chaque rôle vers le bas de la hiérarchie, en ajoutant tous ses descendants ;
4. trier, et éliminer les doublons.

C'est à ce résultat que sont confrontées les règles de route et les règles
d'endpoint, et c'est lui qui est transmis à l'upstream avec l'identité : votre
service reçoit la liste développée et n'a jamais à connaître la hiérarchie.

Le calcul est conservé quelques secondes par compte, tenant et groupe. Un
rôle accordé ou retiré prend donc effet en quelques secondes, sans déconnecter
personne.

## Laisser décider une autorité externe

Les groupes peuvent être attribués à la main, ou déduits de ce que rapporte une
autorité externe : un groupe d'annuaire, une équipe GitHub, un claim d'un
fournisseur d'identité. Une règle fait correspondre un groupe rapporté à un
groupe d'un tenant, et s'exécute à chaque connexion externe : les
appartenances suivent ainsi l'annuaire plutôt qu'un tableur.

Une règle ne gère jamais que ce qu'une règle a placé. Une appartenance ou un
groupe attribué à la main par un administrateur n'est jamais retiré par une
règle, et une règle qui correspondrait à tout est refusée.

> [!NOTE] **Édition Enterprise.**
> La création et la modification des règles de groupe font partie de l'édition
> Enterprise. Les lire, et en supprimer une, reste possible partout : une image
> Community peut ainsi toujours voir et effacer ce qu'une image Enterprise a
> laissé derrière elle.
