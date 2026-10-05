---
title: Rôles
section: La console
order: 168
summary: Le catalogue global des rôles, sa hiérarchie et les tags qui le rendent utilisable.
---

# Rôles

**Application > Roles.** Un seul catalogue pour toute la gateway : ce sont les noms
qu'emploient vos règles d'accès et vos services. Les rôles sont globaux ; c'est un
[groupe](/docs/console/organisation), au sein d'une organisation, qui en attribue à
une personne.

![L'écran Roles : le catalogue présenté en arbre sous une ligne All roles, avec les descriptions et les tags](img/console/roles.webp)

Trois rôles de premier niveau et leurs enfants. Chacun porte la phrase que lira la
personne chargée d'attribuer les rôles, et les tags qui servent de filtre dans la
matrice des groupes.

## L'arbre

Le tableau présente le catalogue sous forme d'arbre, la ligne racine en tête : tout
rôle rattaché directement à la racine est un rôle de premier niveau.

- **Le + d'une ligne** crée un enfant de ce rôle. Le + de la ligne racine crée un rôle
  de premier niveau.
- **La poignée de déplacement** déplace un rôle : déposez-le sur un autre rôle pour en
  faire son enfant, ou sur la ligne racine pour le ramener au premier niveau. Elle est
  désactivée tant qu'un filtre est actif, car la liste affichée n'est alors plus
  l'arbre.
- **Un clic sur une ligne** ouvre le rôle dans le tiroir.
- La mention **system** signale un rôle dont Meerkat lui-même a besoin.

La hiérarchie n'est pas là pour faire joli : **un parent implique ses enfants**.
Cocher un parent dans la matrice des groupes coche tout ce qui en dépend. Un catalogue
calqué sur votre organisation s'attribue donc en un clic.

## L'éditeur

- **Role name** - le nom technique, celui que lit une règle ou un service. Il est fixé
  à la création du rôle et ne peut plus être modifié ensuite.
- **Description** - la phrase, écrite pour un humain, que mettent en avant les écrans
  d'organisation. Prenez le temps de la rédiger : la personne qui attribue les rôles
  lit cette phrase, pas le nom.
- **Tags** - des étiquettes libres. Le champ suggère celles que le catalogue utilise
  déjà, pour éviter qu'une seconde orthographe ne s'installe. La matrice des groupes et
  ce tableau se filtrent par tag : c'est ce qui rend exploitable un catalogue de deux
  cents rôles.
- **Danger zone** - la suppression.

**Le parent n'est pas un champ** : pour déplacer un rôle, il y a le glisser-déposer, et
deux moyens de le faire finiraient tôt ou tard par se contredire. Un rôle créé avec
le + d'une ligne naît sous cette ligne.

## Pièges

- **Un rôle ne se renomme pas.** Un rôle, c'est son nom : les règles, les surcharges
  par endpoint et les services qui le lisent dans un jeton ne connaissent que ce nom.
  Pour le changer, ajoutez le nouveau rôle, faites pointer les règles dessus, puis
  supprimez l'ancien.
- **Un rôle seul ne donne aucun droit.** Il doit figurer dans un groupe, et ce groupe
  doit compter au moins un membre.
- **Les tags ne servent que s'ils sont orthographiés de la même façon.** Retenez la
  suggestion que propose le champ.
- **Filtrer puis glisser** ne fonctionne pas : effacez le filtre avant de réorganiser.
