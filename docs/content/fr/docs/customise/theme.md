---
title: Le thème
section: Personnalisation
order: 243
summary: Dix jetons de couleur, une palette claire et une palette sombre, et la façon d'emporter une palette d'une installation à une autre.
---

# Le thème

Un thème est une palette, et rien d'autre. Il colore les pages que la gateway sert
elle-même - le parcours de connexion, le choix de l'organisation, les pages de mot de passe,
la page d'indisponibilité - ainsi que les petits éléments qu'elle injecte dans les
applications placées derrière elle.

Il se modifie dans **Application > Built-in pages**, onglet **Theme**, avec à côté l'aperçu
de la page réelle.

![L'éditeur de palette et son aperçu](img/console/built-in-pages-theme.webp)

## Les jetons

Dix couleurs modifiables. Chacune est émise sous la forme d'une propriété CSS personnalisée
que lisent les pages ; aucune page ne contient de couleur écrite en dur.

| Jeton | Propriété CSS | Ce qu'il colore |
|---|---|---|
| primary | `--mk-primary` | la couleur d'accent : boutons, liens, focus |
| onPrimary | `--mk-on-primary` | le texte et les icônes posés sur cette couleur d'accent |
| night | `--mk-night` | la teinte du halo d'ambiance |
| surface | `--mk-surface` | la page elle-même |
| onSurface | `--mk-on-surface` | le texte principal |
| surfaceContainer | `--mk-surface-container` | la carte |
| surfaceContainerHigh | `--mk-surface-container-high` | les champs et les zones en relief qu'elle contient |
| onSurfaceVariant | `--mk-on-surface-variant` | le texte secondaire, les indications |
| outline | `--mk-outline` | les bordures et les séparateurs |
| error | `--mk-error` | les refus et les champs invalides |

Une valeur doit être une couleur hexadécimale : `#rgb`, `#rrggbb` ou `#rrggbbaa`. Toute autre
valeur est refusée, en nommant le jeton en cause - le bloc est émis dans une balise
`<style>`, et rien d'autre ne doit pouvoir s'y glisser.

## Le clair et le sombre sont indépendants

Il existe deux palettes, et aucune n'est déduite de l'autre. Les pages émettent un seul bloc
de jetons au moyen de la fonction CSS `light-dark()` : le mode du visiteur sélectionne donc
la palette sans seconde feuille de style.

Un thème sort toujours **complet** de la base : un jeton auquel vous n'avez jamais touché
reçoit explicitement sa valeur par défaut, de sorte que l'éditeur, l'aperçu en direct et la
page servie voient tous la même palette. Sans cela, un thème partiellement défini
s'afficherait différemment dans chacun des trois.

Quatre autres jetons relèvent de la structure et non de la couleur, et ne sont pas
modifiables : les deux rayons d'arrondi, la pile de polices du texte et celle à chasse fixe.

## L'interrupteur des effets

Un seul interrupteur coupe les effets décoratifs des pages intégrées : les halos derrière le
logo, les boutons et la ligne d'état, le dégradé radial d'ambiance, et le dégradé du nom de
l'application. Il les commande tous par un unique jeton `--mk-glow`, il n'y a donc rien à
cocher un par un.

Il est rangé avec la palette parce qu'il répond à la même question : à quoi ressemble cette
page.

## Les palettes de départ

Huit palettes de départ sont livrées avec le produit. Elles partagent le même système de
surfaces et de textes, si bien que seule la couleur d'**accent** change de l'une à l'autre :
vous choisissez une teinte et tout le reste demeure cohérent.

Sentinel's Watch, Midnight, Lavender, Orchid, Rose, Crimson, Ember, Forest. Une installation
neuve les reçoit toutes, la première étant active. Le bouton `+` les propose à nouveau : une
palette que vous avez abîmée se recrée en un clic.

## Plusieurs thèmes, un seul actif

Dupliquer, retoucher, prévisualiser, activer, revenir en arrière - c'est la même logique que
pour une configuration enregistrée. Le thème actif ne peut pas être supprimé : activez-en
d'abord un autre.

La liste suit un ordre stable, qui **ne dépend pas** du thème actif : en activer un ne
redistribue pas les onglets sous votre curseur.

## Emporter une palette ailleurs

L'éditeur exporte la palette dans son état du moment, modifications non enregistrées
comprises, sous la forme d'un petit fichier JSON : les deux palettes, l'interrupteur des
effets et le mode imposé. Importer un tel fichier **remplit l'éditeur** sans rien
enregistrer - vous vérifiez les couleurs dans l'aperçu, puis vous enregistrez.

Seuls les jetons connus dotés d'une valeur hexadécimale survivent à un import : un fichier
modifié à la main ou venu d'ailleurs ne peut donc rien introduire en fraude.

## Ce qui manque

- **La génération d'une palette à partir de couleurs sources**, à la manière de Material
 Design, une palette secondaire et les niveaux d'élévation.
- **La console n'utilise pas cette palette.** Elle repose sur ses propres jetons Material et
 garde donc l'apparence Softwarity, quel que soit votre choix ici.
