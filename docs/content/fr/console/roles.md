---
title: Rôles
section: La console
order: 168
summary: Le catalogue global des rôles, sa hiérarchie, et les tags qui le rendent utilisable.
---

# Rôles

**Application > Roles.** Un catalogue pour toute la passerelle : les noms que parlent
vos règles d'accès et vos services. Les rôles sont globaux ; ce qui lie une personne à
l'un d'eux est un [groupe](/#/docs/console/organisation) dans une organisation.

![L'écran Roles : le catalogue en arbre sous une ligne All roles, avec descriptions et tags](img/console/roles.webp)

Trois rôles de premier niveau avec leurs enfants, chacun avec la phrase que lira
la personne qui attribue les rôles, et les tags sur lesquels la matrice des
groupes filtre.

## L'arbre

Le tableau est le catalogue en arbre, avec la ligne racine en tête : ce qui pend à la
racine est un rôle de premier niveau.

- **Le + d'une ligne** crée un enfant de ce rôle. Le + de la racine en crée un de
  premier niveau.
- **La poignée de glissement** déplace un rôle : le déposer sur un autre en fait un
  enfant, sur la ligne racine le remet au premier niveau. Elle est désactivée tant qu'un
  filtre est posé, parce que la liste que vous voyez n'est pas l'arbre.
- **Cliquer une ligne** ouvre le rôle dans le tiroir.
- Une marque **system** désigne un rôle dont Meerkat lui-même dépend.

La hiérarchie n'est pas décorative : **un parent implique ses enfants**. Cocher un parent
dans la matrice des groupes coche ce qui pend en dessous, donc un catalogue qui a la
forme de votre organisation est un catalogue qui s'attribue en un clic.

## L'éditeur

- **Name** - le nom technique, celui qu'une règle ou un service lit.
- **Description** - la phrase humaine que les écrans d'organisation mettent en avant.
  Ecrivez-la : la personne qui attribue des rôles lit ceci, pas le nom.
- **Tags** - des étiquettes libres, proposées depuis ce que le catalogue emploie déjà
  pour qu'une seconde orthographe ne s'installe pas. La matrice des groupes et ce tableau
  filtrent dessus, et c'est ce qui rend praticable un catalogue de deux cents rôles.
- **Danger zone** - la suppression.

**Le parent n'est pas un champ** : déplacer un rôle, c'est à quoi sert le glisser-déposer,
et deux façons d'en déplacer un ne feraient que les inviter à se contredire. Un rôle créé
depuis le + d'une ligne naît sous cette ligne.

## Pièges

- **Renommer un rôle renomme un contrat.** Les règles, les surcharges par endpoint et les
  services qui lisent le nom suivent le nom, pas la ligne. Renommez délibérément.
- **Un rôle ne donne rien tout seul.** Il doit être dans un groupe, et quelqu'un doit être
  dans ce groupe.
- **Les tags n'aident que s'ils s'écrivent pareil.** Prenez la suggestion du champ.
- **Filtrer puis glisser** ne marche pas : effacez le filtre pour réordonner.
