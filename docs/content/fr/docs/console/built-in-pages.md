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

![Built-in pages sur l'onglet Theme : le tableau des jetons de thème à gauche, les deux aperçus à droite avec le carrousel de thèmes entre eux](img/console/built-in-pages-theme.webp)

La page de connexion telle qu'elle est servie, en mode sombre en haut et en mode clair
en bas, avec le carrousel de thèmes entre les deux. À gauche, une ligne par jeton de
thème et une colonne par mode.

## Le sélecteur de thème

Le carrousel au milieu de l'aperçu est la liste des thèmes. Cliquez sur une palette
pour l'examiner ; cliquez sur **la palette affichée sur le bloc de navigation** pour
mettre ce thème **en service**. Sélectionné et actif sont deux états distincts : c'est
ce qui permet d'essayer un thème sans le servir.

Les commandes situées entre les flèches ajoutent un thème (en dupliquant celui-ci ou en
partant d'un préréglage) et en suppriment un. Le thème actif ne peut pas être supprimé.

## Theme

Les deux palettes du thème, **sombre et claire côte à côte**, avec une ligne par jeton.
Survoler le nom d'un jeton met en évidence la partie de l'aperçu qu'il colore : c'est
le moyen le plus rapide de comprendre à quoi correspond un nom.

- **Les cases Dark et Light** de l'en-tête déterminent quels modes proposent les pages
  servies. Décochez-en une et les pages cessent de proposer ce mode ; il est impossible
  de décocher les deux.
- **Glow** regroupe sous un seul interrupteur les effets décoratifs des pages : le halo
  d'ambiance derrière la page, les lueurs du logo et des boutons, le dégradé du nom de
  l'application. Décoché, le rendu est plat, et la couleur qu'utilisent ces effets ne
  sert plus.
- **Export** et **Import** transportent une palette sous forme de fichier, pour la
  déplacer d'une installation à une autre.

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
