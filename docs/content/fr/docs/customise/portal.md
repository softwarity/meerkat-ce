---
title: Le catalogue d'applications
section: Personnalisation
order: 252
summary: La liste des applications que propose la gateway, et les trois façons de la présenter - rien, un menu ou une barre de navigation.
---

# Le catalogue d'applications

Plusieurs routes d'interface derrière une même gateway, ce sont plusieurs
applications : le visiteur arrive sur l'une et n'a aucun moyen de rejoindre la
suivante. Le catalogue est la liste de celles que vous proposez, dans l'ordre
que vous choisissez.

Il se règle dans **Application > Portal**, il est **global** comme le thème, et
il est livré vide.

![L'écran Portal en mode Portal : le sélecteur à trois états, l'agencement, et la vraie barre en maquette interactive](img/console/portal.webp)

## Un catalogue, trois rendus

Un sélecteur à trois états décide de ce qui est affiché. **La liste, elle, ne
change pas** : passer d'un menu à une barre est une décision d'affichage, pas
une raison de ressaisir vos applications.

| Mode | Ce qu'obtient le visiteur |
|---|---|
| **None** | Aucune liste. Le bouton utilisateur et les pages que Meerkat sert lui-même portent le nom de votre marque, sans rien à cliquer. C'est le bon choix quand il n'y a qu'une application |
| **Links** | La liste, dans l'ordre que vous avez choisi, dans le sous-menu **Applications** du bouton utilisateur et sur les pages du plan de données |
| **Portal** | Une barre de navigation sur chaque page de chaque application. Les pages intégrées ne proposent alors qu'**un seul** lien, la première entrée que l'appelant a le droit d'ouvrir : la navigation, c'est la barre, et une page extérieure aux applications n'a besoin que d'une porte d'entrée |

## Modules et conteneurs

Une entrée est l'une de deux choses :

- un **module** - une application : il ouvre une route d'interface, et hérite de
  son adresse **et de ses règles d'accès** ;
- un **conteneur** - pas de route : un libellé, une icône et une description à
  lui, et des modules à l'intérieur, ses sous-modules. Un clic dessus ouvre le
  **premier sous-module que le visiteur a le droit d'ouvrir**, et un conteneur
  dont le visiteur ne peut ouvrir aucun sous-module n'est pas proposé du tout.
  Un conteneur contient des modules, jamais un autre conteneur.

| Champ | Ce qu'il fait |
|---|---|
| Route | la route d'interface d'un module. Un conteneur n'en a pas |
| Label | le nom sous lequel l'entrée est proposée. Obligatoire pour un conteneur ; pour un module, laissé vide, c'est le nom de la route qui sert |
| Description | l'infobulle de l'entrée |
| Disabled | désactive l'entrée pour tout le monde, sans la retirer de la liste |

En mode **Portal**, une entrée porte en plus une icône :

| Champ | Ce qu'il fait |
|---|---|
| Icon | un pictogramme choisi dans la bibliothèque de la console, stocké en SVG et dessiné comme un masque CSS - aucune police d'icônes n'est jamais chargée. Laissé vide, c'est l'initiale du libellé qui sert |

En mode **Links**, les modules d'un conteneur sont listés à sa place, dans l'ordre.

## Le catalogue dit ce qui existe, la route dit qui le voit

Une entrée hérite des **règles d'accès de la route à laquelle elle est liée** :
un visiteur ne se voit donc proposer que ce que ses droits autorisent. Aucune
règle d'accès ne se décide ici : ce serait loger une décision de sécurité dans
un réglage d'affichage.

Les données envoyées au navigateur ne contiennent aucune règle non plus : le
filtrage a lieu avant leur écriture, et c'est pourquoi la page ne contient rien
à lire ni à falsifier.

C'est aussi la raison pour laquelle le catalogue est global plutôt que propre à
chaque organisation : il est déjà personnalisé, par les routes.

> [!NOTE] Une route ne s'inscrit plus toute seule dans la liste
> Auparavant, le nom d'une application se décidait sur **chaque route
> d'interface**, dans un champ `Link`. Trois endroits pouvaient ainsi nommer la
> même application et, comme la liste était déduite des routes, il fallait bien
> deviner lesquelles désignaient la même chose - il est courant qu'une
> installation expose un seul produit par plusieurs routes, une par
> organisation ou par version, qui diffèrent par ce qu'elles relaient et jamais
> par l'endroit où elles mènent.
>
> Une nouvelle route d'interface n'apparaît donc plus d'elle-même : vous
> l'ajoutez ici. Le nombre de décisions est le même qu'avant, mais elles se
> prennent en un seul endroit.

## En-tête ou rail

En mode **Portal** uniquement. Un seul axe, qui s'inverse :

| Disposition | Les entrées | Leurs enfants |
|---|---|---|
| `header` | un bandeau d'onglets en haut | un rail |
| `rail` | un rail | un bandeau d'onglets en haut |

Le rail se place du côté que vous choisissez, et ce côté a toujours un sens
puisque l'une des deux surfaces est toujours un rail.

Les entrées s'affichent avec l'icône seule, le libellé seul, ou les deux - un
seul réglage pour toute la barre. Le nom de l'application défini dans la marque
peut figurer à côté du logo ; il est désactivé par défaut, car le logo seul
suffit à faire la marque.

## Ce que la barre remplace

En mode **Portal**, elle prend la place du bouton utilisateur propre à chaque
route, sur **toutes** les routes d'interface. Le bouton ne disparaît pas : il
s'installe **dans** la barre, où il perd son sous-menu Applications, puisque ce
rôle revient désormais à la barre elle-même. La navigation et le menu du compte
occupent une seule surface, et non deux coins de l'écran.

La barre n'est jamais affichée dans une iframe : une page incorporée ailleurs
n'est pas un endroit où naviguer.

Techniquement, c'est un simple élément personnalisé doté d'un shadow DOM, servi
comme le bouton utilisateur, qui prend le thème du plan de données et le mode
clair ou sombre du visiteur.

## Modifier le catalogue

En mode **Links**, la console montre la liste pour ce qu'elle est : des entrées
numérotées et deux flèches pour l'ordre. En mode **Portal**, elle montre une
maquette interactive de la barre pendant que vous la construisez : la
disposition se juge ainsi là où elle sera lue, plutôt que dans une liste de
champs.

## Ce qui n'est pas réalisé

- **Le canal des badges.** Une entrée possède une clé de badge et l'emplacement
 est réservé, mais rien n'y envoie encore de compteur - c'est prévu sur le
 canal temps réel.
- **L'arrivée sur la première application accessible.** Un visiteur qui ne peut
 pas ouvrir l'application sur laquelle il arrive n'est pas redirigé vers une
 application qu'il peut ouvrir.
- **Le glisser-déposer** : réordonner à la main, ou faire glisser une entrée
 pour la changer de niveau. Les flèches font l'affaire.
- **Un agencement par organisation.** Ce serait la première surcharge visuelle
 par organisation du produit, alors que le thème et la marque sont aujourd'hui
 globaux.
