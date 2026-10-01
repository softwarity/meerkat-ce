---
title: Applications
section: La console
order: 176
summary: Le catalogue des applications que la passerelle offre, et le bouton à trois états qui décide de la façon dont elle les offre.
---

# Applications

**Application > Portal.** La liste des applications que cette passerelle offre,
dans l'ordre que vous choisissez, et la façon dont elle est offerte. Chaque
visiteur ne voit que les entrées que son accès autorise.

C'est un réglage **global**, comme le thème : un catalogue pour l'installation,
édité ici.

![L'écran Portal : le bouton à trois états, les contrôles d'arrangement, et la vraie barre en mode édition dessous](img/console/portal.webp)

## Le bouton à trois états

| | Ce qui est dessiné |
|---|---|
| **None** | rien. Le bouton utilisateur et les pages intégrées portent le nom de votre marque, sans rien à cliquer |
| **Links** | la liste, dans le sous-menu *Applications* du bouton utilisateur et sur les pages du plan de données |
| **Portal** | une barre de navigation sur chaque page de chaque application ; les pages intégrées n'offrent plus qu'un lien de retour |

**Changer de mode ne coûte pas la liste.** Vous pouvez construire un menu, le
promouvoir en barre, revenir : les entrées restent.

## Ce qu'on y fait

1. **Choisir le mode.** Rien en dessous n'apparaît en *None*.
2. **Ajouter une application** par UI que la liste doit offrir. En *Links* elles
   s'empilent dans une liste numérotée, avec deux flèches pour l'ordre. En
   *Portal* elles se construisent sur le canevas, qui est **la vraie barre en
   mode édition** : cliquer une entrée l'ouvre dans le tiroir.
3. **Choisir l'arrangement** (mode *Portal*) : *header mode* ou *rail mode*, le
   côté que prend le rail, si les entrées d'en-tête montrent l'icône, le libellé
   ou les deux, et si le nom d'application se pose à côté du logo.
4. **Vérifier en étroit** (mode *Portal*). Les trois boutons de largeur mettent
   l'aperçu en tablette et en téléphone, pour voir apparaître les chevrons de
   débordement et le lanceur en gaufre quand les onglets ne tiennent plus. Seule
   la pleine largeur est éditable, et la largeur n'est jamais enregistrée.

## Une entrée

- **Application (route)** - la route UI vers laquelle cette entrée mène. Seules
  les **routes UI activées** sont proposées. L'entrée hérite de l'accès de la
  route, et c'est ce qui rend la liste différente par visiteur : le payload ne
  porte aucune règle d'accès.
- **Label** - vide reprend le nom de la route.
- **Description** - l'infobulle.

En mode *Portal* s'y ajoutent de quoi dessiner une barre :

- **Icon** - chercher dans le jeu d'icônes embarqué, ou coller un SVG.
- **Home label** - la ligne *retour ici* d'un sous-menu. Vide reprend le libellé.
- **Sous-applications** - l'action rapide du tiroir en ajoute une.

Les actions rapides en haut du tiroir déplacent une entrée vers le haut ou le
bas, la désactivent sans la supprimer, ou la retirent.

## Pièges

- **Une nouvelle route UI n'apparaît pas toute seule.** Elle n'est offerte que
  si elle est dans cette liste : c'est ce qui remplace l'ancien champ *Link* de
  l'éditeur de route.
- **Pas de route UI, pas de catalogue.** Une entrée a besoin d'une route activée
  et marquée UI dans [Routes](/docs/console/routes).
- **Un visiteur voit moins d'entrées que vous**, et c'est voulu : la liste est
  filtrée par l'accès de la route de chaque entrée.
- **La barre ne s'affiche jamais dans une iframe.** Une page incluse dans une
  autre n'est pas l'endroit d'une navigation.
- **Passer en *Portal* change toutes les routes UI d'un coup.** Les boutons
  utilisateur par route perdent leur sous-menu Applications au profit de la barre.
