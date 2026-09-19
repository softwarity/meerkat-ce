---
title: La marque
section: Personnalisation
order: 246
summary: Nom, logo, favicon et le fond de page - y compris une image différente en clair et en sombre.
---

# La marque

La marque, c'est l'identité de l'application sur les pages que la passerelle sert : le nom qu'un
visiteur lit au-dessus de la carte de connexion, le symbole dans l'onglet du navigateur, l'image
derrière tout ça (THEME-02, THEME-06).

Elle est **globale** - une identité par passerelle, quel que soit le thème actif. Les thèmes sont des
essais de couleur ; l'identité ne se dédouble pas avec eux. Elle s'édite sur l'onglet **Branding** de
**Application > Built-in pages**.

![L'onglet de la marque](img/console/built-in-pages-branding.webp)

## Nom et description

Le nom de l'application et une description d'une ligne. Une installation neuve arrive avec des
placeholders évidents - `MY APP` et `My application description` - pour qu'il soit clair que les deux
sont à vous plutôt que quelque chose à contourner.

## Le logo

Une image téléversée, stockée en data URI. Formats acceptés : PNG, JPEG, WebP, SVG et ICO. Au-delà
d'environ `195 KiB` l'enregistrement est refusé en le disant, plutôt que tronqué en silence.

Sa **taille** est choisie, pas devinée : normale, grande ou très grande. Un logo n'est pas une forme
fixe - une sentinelle carrée et un logotype large ne remplissent pas la même boîte, et le logotype posé
dans la boîte carrée en sort au tiers de sa hauteur. L'autre solution était de la déduire du rapport
d'aspect de l'image, ce qui décide à votre place sur une page qui est la vôtre. Trois tailles, choisies
une fois, à côté de l'image.

## Le favicon

Optionnel, et délibérément : laissé vide, **le logo sert d'icône d'onglet**. Un logo est presque
toujours utilisable comme icône, et demander une seconde image pour voir sa propre marque dans l'onglet
est une étape que la plupart des gens sautent - après quoi la page de connexion de leur application
porte la sentinelle de Meerkat, qui est le seul endroit où elle ne doit pas être.

Un favicon est un petit carré : au-delà d'environ `41 KiB` c'est une image entière déposée par erreur,
et elle voyagerait sur chaque page.

## Le fond

L'image derrière les pages intégrées appartient à la **marque** et non à un thème, exprès : une
photographie de bâtiment ou un visuel produit est l'identité de l'application, et elle doit survivre aux
essais de couleur qu'un thème est.

| Champ | Ce qu'il fait |
|---|---|
| Image | l'image, jusqu'à environ `911 KiB` |
| Cadrage | `cover` remplit l'écran et recadre, `contain` montre tout, `tile` répète |
| Voile | la quantité de couleur de surface posée par-dessus l'image, de rien à opaque |

**Le voile mérite sa place.** Sans lui, n'importe quelle image avec un coin clair rend la carte de
connexion illisible dans l'un ou l'autre schéma, et le seul recours serait de retoucher l'image.

Le fond est désigné par une URL et jamais intégré dans la page : c'est le seul actif ici qui peut peser
un mégaoctet, et un data URI le mettrait dans chaque page du flux au lieu d'une fois dans le cache du
navigateur.

## Un fond différent en clair et en sombre

Une photographie passe souvent dans un schéma et pas dans l'autre. Le schéma sombre reçoit donc sa
**propre** image, son propre cadrage et son propre voile.

Un interrupteur décide dans quel sens ça marche :

- **une image pour les deux** - l'image claire sert aussi en sombre, et les champs sombres sont ignorés
  (la console les désactive) ;
- **une image chacun** - deux images, deux cadrages, deux voiles.

Téléverser une image claire et aucune sombre veut dire « utilise-la dans les deux », ce qui est la
lecture intuitive d'une image unique. Retirer les deux remet le fond sur *éteint*, une seule forme
plutôt que « éteint avec des réglages encore dedans ».

Sous le capot, CSS ne peut pas commuter une `url()` par `light-dark()`, donc l'image suit le schéma de
deux façons : la préférence système, et une classe que le serveur pose quand un schéma est imposé - qui
l'emporte sur la préférence, pour que le choix du visiteur tienne même contre celui de son système.

## La mention « powered by »

Les pages servies portent une ligne discrète `powered by softwarity/meerkat`. La retirer est un
interrupteur sur cet écran.

> [!NOTE] Enterprise edition
> Cacher la mention est ce que la fonctionnalité white-label accorde. C'est un **choix** et non un
> effet de bord de la détention d'une licence : une installation qui ne l'a jamais demandé garde la
> mention. Sans l'image Enterprise l'interrupteur est refusé à l'enregistrement, en disant pourquoi -
> un interrupteur qui enregistre et ne fait rien est pire qu'un interrupteur qui dit qu'il ne peut pas.

Disposer les pages se vend avec la même clé - voir
[les pages intégrées](/docs/customise/built-in-pages).

## Ce qui manque

La couleur de fond du logo a été abandonnée plutôt que construite : l'image de fond a remplacé de fait
le besoin (THEME-02).
