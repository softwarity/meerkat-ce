---
title: Pages intégrées
section: La console
order: 174
summary: L'allure des pages que Meerkat sert lui-même - couleurs, disposition et identité, avec un seul aperçu vivant.
---

# Pages intégrées

**Application > Built-in pages.** Les pages que la passerelle sert en son nom propre :
connexion, inscription, challenge du second facteur, page d'indisponibilité, pages
d'erreur.

Un écran, un aperçu, trois onglets à gauche : **Theme**, **Layout**, **Branding**. Ils
décrivent la même page, donc l'aperçu ne bouge pas, et le sélecteur de thèmes reste
dessous sur les trois - essayer une couleur en jugeant une disposition est le sens
normal, pas un cas particulier.

Les onglets sont de vraies routes : un marque-page sur la galerie des dispositions revient
sur la galerie des dispositions.

![Built-in pages sur l'onglet Theme : la table des jetons à gauche, les deux aperçus à droite avec le carrousel de thèmes entre eux](img/console/built-in-pages-theme.webp)

La page de connexion telle qu'elle est servie, sombre en haut et claire en bas, avec le
carrousel de thèmes dans l'intervalle. A gauche, une ligne par jeton, une colonne par
schéma.

## Le sélecteur de thèmes

Le carrousel au milieu de l'aperçu est la liste des thèmes. Cliquez une palette pour la
regarder ; cliquez **la palette posée sur le bloc de navigation** pour rendre ce thème
**vivant**. Sélectionné et actif sont deux choses différentes, et c'est ce qui permet
d'essayer un thème sans le servir.

Les commandes entre les flèches ajoutent un thème (une copie de celui-ci, ou un départ
depuis un préréglage) et en suppriment un. Le thème actif ne peut pas être supprimé.

## Theme

Les deux palettes du thème, **sombre et claire côte à côte**, une ligne par jeton.
Survoler le nom d'un jeton met en évidence la partie de l'aperçu qu'il peint, ce qui est
le moyen le plus rapide de découvrir ce qu'un nom veut dire.

- **Les cases Dark et Light** de l'en-tête décident quels schémas les pages servies
  proposent. Décochez-en une et les pages cessent de la proposer ; les deux ne peuvent pas
  être décochées.
- **Glow** rassemble en un interrupteur les effets décoratifs des pages : le halo derrière
  la page, les lueurs du logo et des boutons, le dégradé du nom d'application. Décoché
  donne un design plat, et la couleur que ces effets utilisent devient inutilisée.
- **Export** et **Import** emportent une palette en fichier, pour la déplacer d'une
  installation à l'autre.

Le bouton Save est sur cet onglet : un thème est un objet à lui, et l'enregistrer est ce
qui change ce qu'un thème vivant sert.

## Layout

**Arrangement** est une galerie de maquettes : où se posent la marque, l'image et le
formulaire. En choisir une met l'aperçu à jour aussitôt.

> [!NOTE]
> Edition Enterprise : **garder** un arrangement autre que le centré. La galerie reste
> cliquable et l'aperçu suit, parce que voir un arrangement est ce à quoi sert cet écran ;
> ce que la licence achète, c'est de le garder.

- **Logo size** - la boîte dans laquelle le logo est dessiné. Un logotype large posé dans
  la boîte normale sort au quart de sa hauteur ; *banner* dessine la marque à sa propre
  taille. Il n'y a rien à dimensionner tant qu'aucun logo n'est posé dans l'onglet
  Branding.
- **Which side** - le bord que prend la marque. Seuls les arrangements faits de moitiés
  ont un côté.

Une page ouverte dans une iframe laisse tomber la marque et remplit le cadre d'elle-même,
quel que soit l'arrangement choisi.

## Branding

![Built-in pages sur l'onglet Branding : nom d'application, accroche, zones de dépôt du logo et du fond, et la carte de la marque Meerkat](img/console/built-in-pages-branding.webp)

Le nom et l'accroche tapés ici apparaissent aussitôt dans l'aperçu. En dessous, la carte
de la marque Meerkat, avec sa pastille Enterprise.

L'identité de l'application : **nom**, **accroche**, **logo**, **icône d'onglet**
(favicon), et **image de fond** avec son ajustement (couvrir, contenir, répéter) et son
assombrissement. Le schéma sombre peut porter sa propre image, ou partager celle du clair.

La **marque Meerkat** a sa propre carte, parce que c'est une question de licence et non
d'identité : les pages servies portent une ligne *powered by softwarity/meerkat* en pied,
et la retirer est Enterprise.

> [!NOTE]
> Edition Enterprise : retirer la marque Meerkat.

## Pièges

- **Sélectionné n'est pas actif.** Editer une palette change le thème que vous regardez ;
  le servir, c'est le clic sur la palette du bloc de navigation.
- **La console ne porte pas ce thème.** Ce sont les pages que le **plan de données** sert
  à vos utilisateurs. La console a sa propre allure.
- **La taille du logo ne fait rien sans logo**, et le côté ne fait rien sur un arrangement
  sans moitiés. Les contrôles sont désactivés plutôt que muets.
- **Une image de fond voyage dans un export en paquet**, pas dans un export YAML simple.
  Voir [Configuration](/#/docs/console/configuration).
