---
title: Applications
section: La console
order: 176
summary: Le catalogue des applications que propose la gateway, et le sélecteur à trois états qui décide de la manière dont elle les propose.
---

# Applications

**Application > Portal.** La liste des applications que propose cette gateway,
dans l'ordre de votre choix, et la manière dont elle est présentée. Chaque visiteur
ne voit que les entrées auxquelles il a accès.

C'est un réglage **global**, comme le thème : un seul catalogue pour toute
l'installation, que vous modifiez ici.

![L'écran Portal : le sélecteur à trois états, les réglages de disposition et, en dessous, la vraie barre en mode édition](img/console/portal.webp)

## Le sélecteur à trois états

| | Ce qui s'affiche |
|---|---|
| **None** | rien. Le bouton utilisateur et les pages intégrées affichent le nom de votre marque, sans rien de cliquable |
| **Links** | la liste, dans le sous-menu *Applications* du bouton utilisateur et sur les pages du plan de données |
| **Portal** | une barre de navigation sur chaque page de chaque application ; les pages intégrées proposent alors un seul lien de retour |

**Changer de mode ne fait pas perdre la liste.** Vous pouvez composer un menu, le
transformer en barre, puis revenir en arrière : les entrées sont conservées.

## Ce que vous faites sur cet écran

1. **Choisissez le mode.** En mode *None*, rien de ce qui suit n'apparaît.
2. **Ajoutez une application** pour chaque interface que la liste doit proposer. En
   mode *Links*, elles s'empilent dans une liste numérotée, avec deux flèches pour
   régler l'ordre. En mode *Portal*, vous les composez sur le canevas, qui est
   **la vraie barre en mode édition** : un clic sur une entrée l'ouvre dans le
   tiroir.
3. **Choisissez la disposition** (mode *Portal*) : *header mode* ou *rail mode*, le
   côté où se place le rail, ce qu'affichent les entrées de l'en-tête (l'icône, le
   libellé ou les deux) et la présence du nom de l'application à côté du logo.
4. **Vérifiez le rendu sur un écran étroit** (mode *Portal*). Les trois boutons de
   largeur font passer l'aperçu au format tablette ou téléphone : vous voyez ainsi
   apparaître les chevrons de débordement et le lanceur d'applications lorsque les
   onglets ne tiennent plus. En disposition rail, les modules d'un conteneur se
   replient dans un seul bouton, qui nomme le module où l'on se trouve et les
   ouvre en menu : sur téléphone, ou dès qu'ils ne tiennent plus sur une ligne.
   L'édition n'est possible qu'en pleine largeur, et la largeur choisie n'est
   jamais enregistrée.

## Une entrée

**Add a module** ajoute une application, **Add a container** un groupe de modules.

- **Application (route)** - la route UI d'un module. Seules les **routes UI
  activées** sont proposées. Le module hérite de l'accès de la route : c'est ce
  qui fait que la liste varie d'un visiteur à l'autre, car les données envoyées
  ne contiennent aucune règle d'accès. Un conteneur n'a pas de route.
- **Label** - obligatoire pour un conteneur ; pour un module, laissé vide, il
  reprend le nom de la route.
- **Description** - le texte de l'infobulle.
- **Icon** (mode *Portal*) - cherchez dans le jeu d'icônes embarqué, ou fournissez un SVG : collez-le, déposez son fichier ou choisissez-le.

Un conteneur ouvre son premier sous-module que le visiteur a le droit d'ouvrir,
et il n'est pas affiché pour un visiteur qui ne peut en ouvrir aucun.

Les actions rapides, en haut du tiroir, permettent de monter ou de descendre une
entrée, de la désactiver sans la supprimer, ou de la retirer. Pour un module, *Move under*
le met **dans un conteneur** ou le ramène **au premier niveau** : sa liste propose
d'abord le premier niveau, puis les conteneurs. Sur un conteneur, **+** ajoute un module à l'intérieur.

## Pièges

- **Une nouvelle route UI n'apparaît pas toute seule.** Elle n'est proposée que si
  elle figure dans cette liste : c'est ce qui remplace l'ancien champ *Link* de
  l'éditeur de route.
- **Sans route UI, pas de catalogue.** Une entrée a besoin d'une route activée et
  déclarée UI dans [Routes](/docs/console/routes).
- **Un visiteur voit moins d'entrées que vous**, et c'est voulu : la liste est
  filtrée selon l'accès de la route de chaque entrée.
- **La barre ne s'affiche jamais dans une iframe.** Une page intégrée dans une
  autre n'a pas à porter de barre de navigation.
- **Passer en mode *Portal* modifie toutes les routes UI d'un coup.** Les boutons
  utilisateur de chaque route perdent leur sous-menu Applications, que la barre
  remplace.
