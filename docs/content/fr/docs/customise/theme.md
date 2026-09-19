---
title: Le thème
section: Personnalisation
order: 243
summary: Dix jetons de couleur, une palette claire et une sombre, et comment porter une palette d'une installation à une autre.
---

# Le thème

Un thème est une palette, et rien d'autre. Il colore les pages que la passerelle sert elle-même - le
flux de connexion, le choix d'organisation, les pages de mot de passe, la page d'indisponibilité - et
les petites pièces qu'elle injecte dans les applications proxifiées (THEME-03, THEME-04).

Il s'édite sous **Application > Built-in pages**, onglet **Theme**, avec la vraie page en aperçu à
côté.

![L'éditeur de palette et son aperçu](img/console/built-in-pages-theme.webp)

## Les jetons

Dix couleurs éditables. Chacune est émise comme une propriété CSS que les pages lisent ; aucune page
ne code une couleur en dur.

| Jeton | Propriété CSS | Ce qu'il peint |
|---|---|---|
| primary | `--mk-primary` | l'accent : boutons, liens, focus |
| onPrimary | `--mk-on-primary` | le texte et les icônes posés sur cet accent |
| night | `--mk-night` | la teinte de la lueur d'ambiance |
| surface | `--mk-surface` | la page elle-même |
| onSurface | `--mk-on-surface` | le texte principal |
| surfaceContainer | `--mk-surface-container` | la carte |
| surfaceContainerHigh | `--mk-surface-container-high` | les champs et les zones en relief dedans |
| onSurfaceVariant | `--mk-on-surface-variant` | le texte secondaire, les aides |
| outline | `--mk-outline` | les bordures et les séparateurs |
| error | `--mk-error` | les refus et les champs invalides |

Une valeur doit être une couleur hexadécimale : `#rgb`, `#rrggbb` ou `#rrggbbaa`. Tout le reste est
refusé en nommant ce qui est permis - le bloc est émis dans un `<style>`, et rien d'autre ne peut y
passer.

## Clair et sombre sont indépendants

Il y a deux palettes, et aucune n'est dérivée de l'autre. Les pages émettent un seul bloc de jetons à
travers la fonction CSS `light-dark()`, donc le schéma du visiteur choisit la palette sans seconde
feuille de style.

Un thème quitte toujours la base **complet** : un jeton que vous n'avez jamais touché est matérialisé à
sa valeur par défaut, pour que l'éditeur, l'aperçu live et la page servie voient la même palette. Un
thème défini à moitié se rendrait sinon différemment dans chacun des trois.

Quatre jetons de plus sont structurels et non des couleurs, et ne sont pas éditables : les deux rayons
d'angle, la pile de polices de texte et celle à espacement fixe.

## L'interrupteur plat

Un seul interrupteur éteint les effets décoratifs des pages intégrées : les lueurs derrière le logo,
les boutons et la ligne d'état, le radial d'ambiance, et le dégradé sur le nom de l'application. Il les
pilote tous par un unique jeton `--mk-glow`, donc il n'y a rien à cocher un par un.

Il vit sur la palette parce que c'est la même question : à quoi cette page ressemble.

## Les palettes de départ

Huit palettes sont livrées avec le produit, et elles partagent un même système de surfaces et de textes
pour que seul l'**accent** change entre elles : on choisit une teinte, tout le reste reste cohérent.

Sentinel's Watch, Midnight, Lavender, Orchid, Rose, Crimson, Ember, Forest. Une installation neuve les
reçoit toutes, la première active. Elles sont re-proposées depuis le bouton `+`, donc une palette que
vous avez saccagée est à un clic d'être recréée.

## Plusieurs thèmes, un actif

Dupliquer, retoucher, prévisualiser, activer, revenir en arrière - la même philosophie qu'une
configuration enregistrée. Le thème actif ne peut pas être supprimé : activez-en un autre d'abord.

La liste garde un ordre stable qui **ne dépend pas** du thème actif, pour qu'activer l'un ne rebatte pas
les onglets sous votre curseur.

## Porter une palette ailleurs

L'éditeur exporte la palette telle qu'elle est, retouches non enregistrées comprises, dans un petit
fichier JSON : les deux palettes, l'interrupteur plat et le schéma imposé. Importer un fichier
**remplit l'éditeur** et n'enregistre pas - vous relisez les couleurs dans l'aperçu, puis vous
enregistrez.

Seuls les jetons connus portant une valeur hexadécimale survivent à un import, donc un fichier édité à
la main ou venu d'ailleurs ne peut rien faire passer en fraude.

## Ce qui manque

- **Générer une palette depuis des couleurs sources** à la façon de Material Design, une palette
  secondaire, et les élévations (THEME-04).
- **La console ne consomme pas cette palette.** Elle vit sur ses propres jetons Material, donc la
  console garde l'aspect Softwarity quoi que vous choisissiez ici (THEME-03).
