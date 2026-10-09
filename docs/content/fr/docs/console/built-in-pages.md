---
title: Pages intégrées
section: La console
order: 174
summary: L'apparence des pages que Meerkat sert lui-même - couleurs, disposition et identité, avec un seul aperçu en direct.
---

# Pages intégrées

**Application > Built-in pages.** Les pages que la gateway sert en son nom propre :
la connexion, l'inscription, la demande du second facteur, la page d'indisponibilité,
les pages d'erreur.

Un seul écran, un seul aperçu, et trois onglets à gauche : **Theme**, **Layout**,
**Branding**. Tous trois décrivent la même page : l'aperçu ne bouge donc jamais, et le
sélecteur de thème reste en dessous dans les trois onglets - essayer une couleur
pendant qu'on évalue une disposition est la façon normale de procéder.

Les onglets sont de véritables routes : un favori posé sur la galerie des dispositions
ramène à la galerie des dispositions.

![Built-in pages sur l'onglet Theme : les six couleurs sources, le contraste et la typographie à gauche, les deux aperçus à droite avec le carrousel de thèmes entre eux](img/console/built-in-pages-theme.webp)

La page de connexion telle qu'elle est servie, en mode sombre en haut et en mode clair
en bas, avec le carrousel de thèmes entre les deux. À gauche, les couleurs dont le thème
est fait.

## Le sélecteur de thème

Le carrousel au milieu de l'aperçu est la liste des thèmes. Cliquez sur une palette
pour l'examiner ; cliquez sur **la palette affichée sur le bloc de navigation** pour
mettre ce thème **en service**. Sélectionné et actif sont deux états distincts : c'est
ce qui permet d'essayer un thème sans le servir.

Les commandes situées entre les flèches ajoutent un thème (en dupliquant celui-ci ou en
partant d'un préréglage) et en suppriment un. Le thème actif ne peut pas être supprimé.

## Theme

Un thème se fait comme dans le [Material Theme Builder](https://material-foundation.github.io/material-theme-builder/),
et c'est le même thème : les mêmes six couleurs donnent les mêmes schémas, rôle pour
rôle.

- **Core colours.** La **primaire** est la source. **Secondaire**, **tertiaire**,
  **erreur**, **neutre** (fonds et surfaces) et **neutre variante** (emphase moyenne et
  contours) en sont dérivées tant qu'on ne les fixe pas - la valeur affichée en grisé
  est celle dont elles sont dérivées. La croix remet une couleur en dérivée.
- **Contrast** : standard, moyen ou élevé, pour les deux schémas.
- **Color match** est le "rester fidèle à mes couleurs" du builder : les conteneurs
  gardent le ton des couleurs données plutôt que celui de la spécification.
- **Typography** : une police **display** pour les titres et le nom de l'application,
  une police **body** pour le texte et les boutons, une police **code** pour les codes,
  les clés, les champs et les libellés - chacune parmi quatorze familles que la gateway
  livre et sert elle-même, ou celle du système. Derrière n'importe quel choix, Noto
  dessine l'arabe, l'hébreu, le devanagari et le thaï ; le chinois, le japonais et le
  coréen utilisent les polices du système. Chaque famille s'affiche dans son propre dessin.
- **Generated roles** liste chaque rôle Material 3 que font les couleurs, sombre et
  clair côte à côte - en lecture seule, ils suivent.

Survoler une couleur ou un rôle fait clignoter dans l'aperçu ce qu'il colore : c'est le
moyen le plus rapide de comprendre à quoi correspond un nom. L'aperçu suit **pendant le
choix**, sur la page qu'il montre, quelle qu'elle soit : un e-mail et la barre de
portail sont redessinés avec les couleurs à l'écran eux aussi.

- **Les cases Dark et Light** de l'en-tête déterminent quels modes proposent les pages
  servies. Décochez-en une et les pages cessent de proposer ce mode ; il est impossible
  de décocher les deux.
- **Glow** regroupe sous un seul interrupteur les effets décoratifs des pages : le halo
  d'ambiance derrière la page, les lueurs du logo et des boutons, le dégradé du nom de
  l'application. Décoché, le rendu est plat.
- **Export** écrit au format JSON du builder (couleurs sources, les six schémas, les
  palettes), les réglages propres à Meerkat sous une clé à part. **Import** lit un
  export du builder - on retrouve si Color match était actif en regardant les schémas
  qu'il porte. Un export plus ancien que les règles de contraste de 2025 du builder
  s'importe par ses couleurs, et l'écran signale que certains rôles diffèrent du fichier.

Un thème saisi jeton par jeton dans une version antérieure est converti à la mise à
jour en les six couleurs qui s'en approchent le plus : sa primaire reste, ses surfaces et
ses contours prennent les tons de Material 3.

Le bouton Save se trouve dans cet onglet : un thème est un objet à part entière, et
c'est son enregistrement qui modifie ce que sert un thème en service.

## Layout

![L'onglet Layout : cinq dispositions, la taille du logo et le côté, avec les aperçus à côté](img/console/built-in-pages-layout.webp)

**Arrangement** est une galerie de maquettes : l'emplacement de la marque, de l'image
et du formulaire. En choisir une met aussitôt l'aperçu à jour.

> [!NOTE]
> Édition Enterprise : **conserver** une disposition autre que la disposition centrée.
> La galerie reste cliquable et l'aperçu suit, car cet écran sert justement à voir une
> disposition ; ce que la licence apporte, c'est le droit de la conserver.

- **Logo size** - le cadre dans lequel le logo est dessiné. Un logotype tout en largeur
  placé dans le cadre normal ressort au quart de sa hauteur ; *banner* dessine le logo
  à sa taille réelle. Il n'y a rien à dimensionner tant qu'aucun logo n'a été défini
  dans l'onglet Branding.
- **Which side** - le bord qu'occupe la marque. Seules les dispositions faites de deux
  moitiés ont un côté.

Une page ouverte dans une iframe abandonne d'elle-même la marque et remplit le cadre,
quelle que soit la disposition choisie.

## Branding

![Built-in pages sur l'onglet Branding : le nom de l'application, le slogan, les zones de dépôt du logo, de l'icône d'onglet et de l'image de fond, et la carte de la mention Meerkat](img/console/built-in-pages-branding.webp)

Le nom et le slogan saisis ici apparaissent aussitôt dans l'aperçu. En dessous, la
carte de la mention Meerkat, réservée à l'édition Enterprise.

L'identité de l'application : le **nom**, le **slogan**, le **logo**, l'**icône
d'onglet** (favicon) et l'**image de fond**, avec son ajustement (couvrir, contenir,
mosaïque) et son assombrissement. Le mode sombre peut avoir sa propre image ou partager
celle du mode clair.

La **mention Meerkat** a sa propre carte, parce que c'est une question de licence et
non d'identité : les pages servies affichent en pied de page une ligne *powered by
softwarity/meerkat*, et la retirer est réservé à l'édition Enterprise.

> [!NOTE]
> Édition Enterprise : retirer la mention Meerkat.

## Pièges

- **Sélectionné ne veut pas dire actif.** Modifier une palette change le thème que vous
  avez sous les yeux ; pour le servir, il faut cliquer sur la palette du bloc de
  navigation.
- **La console ne porte pas ce thème.** Il s'agit des pages que le **plan de données**
  sert à vos utilisateurs. La console a sa propre apparence.
- **La taille du logo n'a aucun effet sans logo**, et le côté n'a aucun effet sur une
  disposition sans moitiés. Dans ces cas, les commandes sont désactivées plutôt que
  d'agir dans le vide.
- **Une image de fond voyage dans un export sous forme de paquet**, pas dans un simple
  export YAML. Voir [Configuration](/docs/console/configuration).
