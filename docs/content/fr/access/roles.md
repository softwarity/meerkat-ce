---
title: Rôles et groupes
section: Contrôle d'accès
order: 132
summary: Un catalogue de rôles pour toute la gateway, des groupes par organisation, et comment les rôles effectifs d'une personne sont calculés.
---

# Rôles et groupes

Les rôles sont les noms dans lesquels vos règles s'écrivent - `orders-reader`,
`support`, `billing-admin`. Les groupes sont la façon dont les gens les obtiennent.

Le partage est délibéré : **le catalogue est global à la gateway, les groupes
appartiennent à une organisation**. Un vocabulaire pour toute l'installation,
assemblé différemment pour chaque organisation.

## Le catalogue

**Application > Roles.** Un rôle a un nom, unique dans l'installation, une
description, des étiquettes facultatives, et au plus un parent.

Les noms de rôles s'écrivent aussi dans les règles de routes : ils sont donc limités
aux lettres, aux chiffres, à `-` et `_`. Un rôle dont le nom sort de là est refusé
quand une route essaie de s'en servir.

**Un parent implique ses enfants.** Détenir un rôle parent l'accorde, lui et tout ce
qui est en dessous, jusqu'en bas :

```
staff
  support
    support-lead
  billing
```

Qui détient `staff` satisfait une règle qui demande `support-lead`. Qui détient
`support` satisfait `support-lead` mais pas `billing`. Accordez le haut d'une
branche pour accorder la branche.

Un rôle ne peut pas devenir son propre ancêtre : un cycle est refusé. Supprimer un
rôle remonte ses enfants au premier niveau au lieu de les supprimer avec lui, et le
retire de tous les groupes qui le portaient. Certains rôles sont marqués rôles
**système** et ne peuvent pas être supprimés du tout.

Les étiquettes sont un classement libre - une par microservice, par exemple - et
elles voyagent avec le rôle là où les expressions peuvent les lire. Elles n'ont
aucun effet sur les décisions d'accès.

## Les groupes

**Un groupe appartient à une organisation** et porte un ensemble de rôles du
catalogue. Son nom est unique dans cette organisation. Un membre de l'organisation
reçoit ensuite des groupes, et ses rôles sont les rôles de ces groupes.

Rien d'autre n'accorde un rôle. Il n'y a pas de rôle sur un compte, et pas de rôle
hors d'une organisation :

> [!WARNING]
> Une session **sans organisation active ne détient aucun rôle**. Pas les rôles
> d'une autre organisation, pas un sous-ensemble : aucun. Toute règle qui demande un
> rôle exige donc aussi une organisation active, même quand la règle n'en parle pas.

## Un groupe à la fois, ou tous

Chaque organisation choisit comment ses groupes se combinent :

| Mode | Ce qu'il fait |
|---|---|
| Cumulatif (le défaut) | tous les groupes attribués comptent à la fois |
| Exclusif | un seul groupe s'applique, choisi à la connexion |

Le mode exclusif existe pour le cas où quelqu'un porte deux casquettes dans la même
organisation et où les deux ne doivent pas être portées ensemble. Quand il est
actif et que la personne a plus d'un groupe, elle est envoyée sur `/select-group`,
et elle peut basculer plus tard depuis le bouton utilisateur. Une session exclusive
qui n'a pas choisi ne porte **aucun rôle** - la même règle que sans organisation.

Il n'y a pas de défaut pour toute l'installation : c'est une propriété de
l'organisation, et une organisation qui ne dit rien est cumulative.

## Comment les rôles effectifs sont calculés

À chaque requête, pour le compte, l'organisation active et le groupe actif :

1. prendre les groupes attribués dans **cette** organisation - ou seulement le groupe actif, en mode exclusif ;
2. prendre les rôles de ces groupes ;
3. déplier chaque rôle vers le bas de la hiérarchie, en ajoutant chaque descendant ;
4. trier, dédoublonner.

Le résultat est ce contre quoi les règles de routes et d'endpoints sont jugées, et
ce qui est transmis à l'amont avec l'identité : votre service reçoit la liste
dépliée et n'a jamais à connaître la hiérarchie.

Le calcul est mémorisé quelques secondes par compte, organisation et groupe. Un rôle
accordé ou retiré prend donc effet en quelques secondes, sans déconnecter personne.

## Laisser une autorité amont décider

Les attributions de groupes se posent à la main, ou se projettent depuis ce qu'une
autorité externe rapporte : un groupe d'annuaire, une équipe GitHub, un claim d'un
fournisseur d'identité. Une règle associe un groupe rapporté à un groupe d'une
organisation, et s'exécute à chaque connexion externe - l'appartenance suit donc
l'annuaire plutôt qu'un tableur.

Une règle ne gère jamais que ce qu'une règle a posé. Une appartenance ou un groupe
attribué à la main par un administrateur n'est jamais retiré par une règle, et une
règle qui matcherait tout est refusée.

> [!NOTE] **Enterprise edition.**
> Créer et modifier des règles de groupe fait partie de l'édition Enterprise. Les
> lire, et en supprimer une, ne sont pas bridés - une image communautaire peut donc
> encore voir et nettoyer ce qu'une image Enterprise a laissé.
