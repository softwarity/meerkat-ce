---
title: Les pages intégrées
section: Personnalisation
order: 249
summary: Les pages que la gateway sert elle-même, leur disposition, et les langues dans lesquelles elles existent.
---

# Les pages intégrées

Ce sont les pages que Meerkat sert lui-même, devant vos applications. Il s'agit de simple
HTML produit par le serveur - pas de framework, pas de bundle, rien qui vienne d'un CDN - et
elles lisent les jetons du thème : elles suivent donc votre palette sans recompilation.

| Page | Où elle apparaît |
|---|---|
| `/login` | le formulaire de connexion et les boutons des autorités externes |
| `/totp` et `/totp-enroll` | la vérification et l'enregistrement d'un second facteur |
| `/update-password` | un mot de passe que la gateway vous impose de changer |
| `/forgot-password`, `/reset-password` | la réinitialisation du mot de passe |
| `/register`, `/confirm`, `/account-pending` | l'auto-inscription et sa confirmation |
| `/select-tenant`, `/select-group` | le choix de l'organisation ou du groupe au nom duquel agir |
| `/refused` | la raison pour laquelle un appelant ne passe pas |
| `/profile` et ses sous-pages | les pages du compte lui-même : mot de passe, second facteur, passkeys, autorités, historique, jetons |
| la page d'indisponibilité | une route dont le service est arrêté, ou l'interrupteur global |

## Un écran, quatre onglets, un aperçu

Les couleurs, la disposition et l'identité formaient trois tâches distinctes, avec pour
chacune ses options d'un côté et le **même** aperçu de l'autre, chacune montrant une page
dont les deux autres décident aussi. Il s'agit d'un seul sujet - ce que voit le visiteur -
et donc d'une seule entrée à trois onglets, les onglets n'occupant que la partie gauche.
L'aperçu ne bouge jamais, et le carrousel des thèmes reste dessous dans les trois onglets :
essayer une couleur tout en jugeant une disposition est la façon normale de travailler, pas
un cas particulier.

Un quatrième onglet, **Locale**, porte les mots des pages : chaque texte de chaque page,
langue par langue, chacun modifiable là où il s'affiche - voir [Les langues](#les-langues).

Les onglets sont de vraies URL : un marque-page posé sur la galerie des dispositions ramène
à la galerie des dispositions.

## La disposition

Un catalogue fermé de dispositions, chacune étant un bloc de CSS livré avec le produit :

| Disposition | Ce que cela donne |
|---|---|
| `centered` | la marque en haut, la carte au milieu, le fond derrière l'ensemble. C'est la valeur par défaut |
| `split` | l'image occupe une moitié de l'écran sur toute la hauteur et porte la marque, le formulaire occupe l'autre |
| `drawer` | l'image reste entière et un panneau opaque, plaqué contre un bord, porte la marque et le formulaire |
| `banner` | la marque dans un bandeau en haut de l'écran, la carte en dessous |
| `bare` | aucune carte : les champs reposent directement sur le fond |

`split` et `drawer` sont faites de deux moitiés et demandent donc un **côté** : gauche ou
droite. Les autres l'ignorent, et le champ est vidé plutôt que conservé sans servir à rien.

> [!NOTE] Édition Enterprise
> Changer la disposition fait partie de la marque blanche, le même achat que le retrait de la
> mention : il s'agit de donner à ces pages l'apparence de votre produit plutôt que du nôtre.

Trois conséquences en découlent, toutes voulues. Un enregistrement des réglages transporte
la totalité des données, si bien que tous les autres écrans renvoient la disposition en
cours sans y toucher - le contrôle porte sur le **changement**, pas sur la valeur, faute de
quoi enregistrer une langue sur une instance Community serait refusé. Une disposition déjà en
place **continue d'être servie** si une licence arrive à échéance : le modèle est perpétuel,
et un fichier expiré qui redessinerait sans prévenir la page de connexion de tous vos clients
serait exactement le "ça marchait hier" dont ce produit ne veut pas. Enfin, revenir à
`centered` est **toujours** permis, sans quoi une instance pourrait rester bloquée sur une
disposition qu'elle ne peut plus quitter.

## Clair, sombre, ou le choix du visiteur

L'intégrateur décide en premier, en décochant un mode sur l'aperçu :

| Réglage | Ce que font les pages |
|---|---|
| le visiteur décide | elles suivent d'abord son système, puis un bouton de bascule dont le choix est retenu |
| clair | clair uniquement, et le bouton de bascule disparaît |
| sombre | sombre uniquement, et le bouton de bascule disparaît |

Imposer un mode n'a rien d'un caprice. Ces pages sont placées **devant** une application qui
ne connaît peut-être qu'une seule apparence : une page de connexion qui suit le système en
mode sombre du visiteur, puis passe la main à un portail qui n'existe qu'en clair, donne
l'impression de deux produits différents - et vous ne pouvez pas le corriger de votre côté.

Quand le visiteur choisit, son choix est conservé un an dans un cookie **et** sur son compte.
Il est donc rétabli sur un navigateur qui ne l'a jamais vu - exactement comme sa langue. Voir
[clair et sombre](/docs/customise/color-scheme) pour ce qu'en fait une application placée
derrière la gateway.

## Les langues

![L'onglet Locale : les textes de la page de connexion en anglais, chacun modifiable, à côté de l'aperçu](img/console/built-in-pages-locale.webp)

Vingt catalogues sont embarqués dans le binaire, à raison d'un fichier JSON par langue :
arabe, allemand, anglais, espagnol, français, hébreu, hindi, indonésien, italien, japonais,
coréen, néerlandais, polonais, portugais, russe, thaï, turc, ukrainien, vietnamien et chinois
simplifié.

L'anglais sert de référence : chaque autre catalogue lui est comparé au démarrage, et une clé
absente d'un catalogue est remplacée par son texte anglais plutôt que par un blanc. Les
messages d'erreur du serveur sont traduits de la même manière.

La langue vient du choix du visiteur - un cookie et son compte - et, à défaut, de ce que
demande son navigateur. Les langues *proposées* sont la réunion de celles dans lesquelles les
routes d'interface déclarent être écrites (**Routes > une route > Locales**) : il n'y a pas
de réserve à élargir, chaque route apporte les siennes.

> [!WARNING]
> La console, elle, n'existe **qu'en anglais**, et c'est une décision, pas une lacune : c'est
> un outil d'exploitant. Les pages décrites ci-dessus sont celles que voient vos
> utilisateurs, et celles-là sont traduites.

## Ce qui manque

- **Votre propre HTML.** La disposition se choisit dans un catalogue ; remplacer le balisage
 d'une page par votre propre modèle n'est pas possible.
- La page où le développeur choisit sa variante, qui n'aura de sens que lorsque les
 surcharges pourront être limitées à un développeur.
