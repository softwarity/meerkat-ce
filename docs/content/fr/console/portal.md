---
title: Portail
section: La console
order: 176
summary: La barre de navigation que portent les applications proxifiées, construite face à un aperçu vivant.
---

# Portail

**Application > Portal.** Une barre pour circuler entre les applications que cette
passerelle sert. Elle est injectée dans les pages proxifiées, et chaque visiteur ne voit
que les entrées que son accès autorise.

C'est un réglage **global**, comme le thème : une barre pour l'installation, éditée ici.

Quand le portail est actif, il **remplace le bouton utilisateur par route** sur toutes les
routes UI : le bouton de compte monte dans la barre.

![L'écran Portal : les contrôles d'arrangement, et la vraie barre en mode édition dessous](img/console/portal.webp)

Mode en-tête, icône et libellé, le nom d'application à côté du logo. Le canevas en dessous
est la barre elle-même : *Acme Corp* à gauche, trois modules, et les sous-modules de celui
qui est sélectionné montrés en rail.

## Ce qu'on y fait

1. **L'allumer.** Rien en dessous n'apparaît avant.
2. **Choisir l'arrangement** : *header mode* ou *rail mode*, le côté que prend le rail, si
   les entrées d'en-tête montrent l'icône, le libellé ou les deux, et si le nom
   d'application se pose à côté du logo.
3. **Ajouter un module** par application que la barre doit atteindre. Les modules se
   construisent sur le canevas, qui est **la vraie barre en mode édition** : cliquer une
   entrée l'ouvre dans le tiroir.
4. **Vérifier en étroit.** Les trois boutons de largeur mettent l'aperçu en tablette et en
   téléphone, pour voir apparaître les chevrons de débordement et le lanceur en gaufre
   quand les onglets ne tiennent plus. Seule la pleine largeur est éditable, et la largeur
   n'est jamais enregistrée.

## Un module

- **Application (route)** - la route UI vers laquelle cette entrée mène. Seules les
  **routes UI activées** peuvent porter la barre, et seules celles-là sont proposées. Le
  module hérite de l'accès de la route, et c'est ce qui rend la barre différente par
  visiteur : le payload ne porte aucune règle d'accès.
- **Label** - vide reprend le nom de la route.
- **Home label** (parents seulement) - la ligne *retour ici* d'un sous-menu. Vide reprend
  le libellé.
- **Description** - l'infobulle.
- **Icon** - chercher dans le jeu d'icônes embarqué, ou coller un SVG.

Les actions rapides en haut du tiroir déplacent un module vers le haut ou le bas, ajoutent
un sous-module à un parent, le désactivent sans le supprimer, ou le retirent.

## Pièges

- **Pas de route UI, pas de portail.** Un module a besoin d'une route activée et marquée
  UI dans [Routes](/#/docs/console/routes).
- **Un visiteur voit moins d'entrées que vous**, et c'est voulu : la barre est filtrée par
  l'accès de la route de chaque module.
- **Elle ne s'affiche jamais dans une iframe.** Une page incluse dans une autre n'est pas
  l'endroit d'une barre de navigation.
- **Allumer le portail change toutes les routes UI d'un coup.** Les boutons utilisateur par
  route perdent leur sous-menu Applications au profit de la barre.
