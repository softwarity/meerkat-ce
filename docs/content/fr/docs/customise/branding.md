---
title: La marque
section: Personnalisation
order: 246
summary: Le nom, le logo, le favicon et le fond de page - avec, si besoin, une image différente en clair et en sombre.
---

# La marque

La marque est l'identité de l'application sur les pages que sert la gateway : le nom que
le visiteur lit au-dessus de la carte de connexion, l'icône dans l'onglet du navigateur,
l'image en arrière-plan.

Elle est **globale** - une identité par gateway, quel que soit le thème actif. Les thèmes
sont des essais de couleurs ; l'identité ne se dédouble pas avec eux. Elle se modifie dans
l'onglet **Branding** de **Application > Built-in pages**.

![L'onglet de la marque](img/console/built-in-pages-branding.webp)

## Nom et description

Le nom de l'application et une description d'une ligne. Une installation neuve est livrée
avec des valeurs manifestement provisoires - `MY APP` et `My application description` - pour
qu'il soit évident que ces deux champs sont à remplir par vous, et non à contourner.

## Le logo

Une image que vous envoyez, stockée sous forme de data URI. Formats acceptés : PNG, JPEG,
WebP, SVG et ICO. Au-delà d'environ `195 KiB`, l'enregistrement est refusé, en nommant le
champ en cause, plutôt que de tronquer l'image sans rien dire.

Sa **taille** se choisit, elle ne se devine pas : normale, grande ou très grande. Un logo n'a
pas de forme fixe - une sentinelle carrée et un logotype tout en largeur ne remplissent pas
le même cadre, et le logotype placé dans le cadre carré n'atteint qu'un tiers de sa hauteur.
L'autre solution consistait à déduire la taille des proportions de l'image, c'est-à-dire à
décider à votre place sur une page qui est la vôtre. Trois tailles, donc, choisies une fois
pour toutes, à côté de l'image.

## Le favicon

Il est facultatif, et c'est voulu : si vous le laissez vide, **le logo sert d'icône
d'onglet**. Un logo fait presque toujours une icône acceptable, et réclamer une seconde image
pour voir sa propre marque dans l'onglet est une étape que la plupart des gens sautent - à la
suite de quoi la page de connexion de leur application affiche la sentinelle de Meerkat, au
seul endroit où elle n'a rien à faire.

Un favicon est un petit carré : au-delà d'environ `41 KiB`, il s'agit d'une image complète
déposée là par erreur, et elle serait transportée avec chaque page.

## Le fond

L'image affichée derrière les pages intégrées appartient à la **marque** et non à un thème,
et c'est délibéré : la photographie d'un bâtiment ou le visuel d'un produit font partie de
l'identité de l'application, et doivent survivre aux essais de couleurs que sont les thèmes.

| Champ | Ce qu'il fait |
|---|---|
| Image | l'image, jusqu'à environ `911 KiB` |
| Fit | `cover` remplit l'écran quitte à rogner l'image, `contain` la montre en entier, `tile` la répète |
| Dim | la quantité de couleur de surface superposée à l'image, de rien du tout à l'opacité complète |

**Le voile (Dim) n'est pas là par hasard.** Sans lui, la moindre image dont un coin est
lumineux rend la carte de connexion illisible dans l'un des deux modes, et il ne resterait
plus qu'à retoucher l'image.

Le fond est référencé par une URL et jamais incorporé à la page : c'est le seul élément, ici,
qui puisse peser un mégaoctet, et une data URI le placerait dans chaque page du parcours au
lieu de le ranger une seule fois dans le cache du navigateur.

## Un fond différent en clair et en sombre

Une photographie passe souvent bien dans un mode et mal dans l'autre. Le mode sombre dispose
donc de sa **propre** image, de son propre cadrage et de son propre voile.

Un seul interrupteur décide du fonctionnement :

- **une image pour les deux modes** - l'image du mode clair sert aussi en sombre, et les
 champs du mode sombre sont ignorés (la console les désactive) ;
- **une image par mode** - deux images, deux cadrages, deux voiles.

Envoyer une image pour le clair et aucune pour le sombre signifie "utilisez-la dans les deux
modes" : c'est ce que l'on attend naturellement d'une image unique. Retirer les deux images
remet le fond à l'état *désactivé*, un état unique plutôt qu'un fond "désactivé, mais qui
garde encore des réglages".

En coulisses, CSS ne sait pas faire varier une `url()` avec `light-dark()`. L'image suit donc
le mode de deux manières : par la préférence du système, et par une classe que le serveur
appose quand un mode est imposé - celle-ci l'emporte sur la préférence, si bien que le choix
du visiteur tient même contre celui de son système.

## La mention "powered by"

Les pages servies portent une discrète ligne `powered by softwarity/meerkat`. Un interrupteur
de cet écran permet de la retirer.

> [!NOTE] Édition Enterprise
> Masquer la mention est précisément ce qu'apporte la fonction de marque blanche. C'est un
> **choix**, et non un effet secondaire de la possession d'une licence : une installation qui
> ne l'a jamais demandé garde la mention. Sans l'image Enterprise, l'enregistrement refuse
> l'interrupteur et dit pourquoi - un interrupteur qui s'enregistre sans rien faire est pire
> qu'un interrupteur qui annonce qu'il ne peut rien.

Le choix de la disposition des pages est vendu avec la même clé - voir
[les pages intégrées](/docs/customise/built-in-pages).

## Ce qui manque

La couleur de fond propre au logo a été abandonnée plutôt que réalisée : l'image de fond en a
supprimé le besoin.
