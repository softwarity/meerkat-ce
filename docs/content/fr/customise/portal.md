---
title: Le portail de navigation
section: Personnalisation
order: 252
summary: Un en-tête ou un rail injecté dans vos applications pour que plusieurs routes se lisent comme un seul produit.
---

# Le portail de navigation

Plusieurs routes UI derrière une passerelle, ce sont plusieurs applications : un visiteur atterrit sur
l'une et n'a aucun moyen d'atteindre la suivante. Le portail est la barre qui corrige ça - injectée dans
chaque page UI proxifiée, elle circule entre les routes que cette passerelle sert comme si elles étaient
un seul produit (PORTAL-01).

Elle promeut ce qui était un sous-menu du bouton utilisateur en vraie navigation : des modules parents
et leurs enfants, un lanceur en gaufre, des flèches de débordement, le logo de la marque et le menu du
compte, tout dans un seul bandeau.

Elle se règle sous **Application > Portal**, elle est **globale** comme le thème, et elle est livrée
**éteinte**.

![L'éditeur du portail et sa maquette live](img/console/portal.webp)

## En-tête ou rail

Un axe, et il se bascule :

| Disposition | Les parents | Les enfants |
|---|---|---|
| `header` | un bandeau d'onglets en haut | un rail |
| `rail` | un rail | un bandeau d'onglets en haut |

Le rail prend le côté que vous choisissez, et ce côté veut toujours dire quelque chose puisque l'une des
deux surfaces est toujours un rail.

Les entrées se rendent en icône seule, en libellé seul, ou les deux - un seul réglage pour toute la
barre. Le nom d'application de la marque peut se poser à côté du logo ; c'est éteint par défaut, parce
que le logo seul est la marque.

## Les modules

Un **parent** est une application complète à part entière : il se lie à une route UI, il est navigable,
et il peut rassembler des enfants montrés sur la surface secondaire. Quand il a des enfants, la barre
offre un retour vers le parent lui-même.

| Champ | Ce qu'il fait |
|---|---|
| Route | la route UI que cette entrée ouvre. L'entrée hérite de son adresse **et de son accès** |
| Icône | un glyphe choisi dans la banque de la console, stocké en SVG et dessiné en masque CSS - donc aucune police d'icônes n'est jamais chargée. Vide retombe sur l'initiale du libellé |
| Libellé | remplace le nom propre de la route dans la barre |
| Libellé d'accueil | ce que lit la ligne « retour à ce module », quand le parent a des enfants |
| Description | l'infobulle de l'entrée |
| Désactivé | éteint pour tout le monde, sans le retirer de la configuration |

Un **enfant** est la même chose, sans le libellé d'accueil.

## Chaque visiteur voit son propre portail

Un module hérite de l'**accès de la route à laquelle il se lie**, donc un visiteur ne voit offert que ce
que ses droits autorisent. La charge utile servie au navigateur ne porte aucune règle d'accès : le
filtrage se fait avant qu'elle soit écrite, ce qui est pourquoi il n'y a rien à lire ni à trafiquer dans
la page.

C'est aussi pourquoi le portail est global plutôt que par organisation : il est déjà personnalisé, par
les routes.

## Ce qu'il remplace

Quand le portail est allumé, il prend la place du bouton utilisateur par route sur **toutes** les routes
UI. Le bouton ne disparaît pas - il déménage **dans** la barre, et il y perd son sous-menu Applications,
puisque c'est désormais la barre elle-même. La navigation et le menu du compte sont une seule surface,
pas deux coins.

La barre n'est jamais dessinée dans une iframe : une page embarquée ailleurs n'est pas l'endroit d'une
navigation.

Techniquement, c'est un simple élément personnalisé à shadow DOM, servi comme le bouton utilisateur,
portant le thème du plan de données et le clair ou sombre du visiteur.

## L'éditer

La console montre une maquette live de la barre pendant que vous construisez le catalogue, pour que la
disposition se juge là où elle sera lue plutôt que dans une liste de champs.

## Ce qui n'est pas construit

- **Le canal de badge.** Un module porte une clé de badge et l'emplacement est réservé, mais rien ne
  pousse encore un compte dessus - c'est prévu sur le canal live.
- **L'atterrissage sur la première application accessible.** Un visiteur qui ne peut pas ouvrir le module
  sur lequel il arrive n'est pas redirigé vers un module qu'il peut ouvrir.
- **Le glisser-déposer dans le catalogue** : réordonner, ou faire glisser une entrée pour changer de
  niveau.
- **L'arrangement par organisation** (PORTAL-02). Ce serait la première surcharge visuelle par tenant du
  produit, et le thème et la marque sont globaux aujourd'hui.
