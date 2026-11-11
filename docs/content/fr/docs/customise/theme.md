---
title: Le thème
section: Personnalisation
order: 243
summary: Six couleurs sources, un niveau de contraste et trois polices, d'où les deux modes sont générés comme le fait Material Theme Builder.
---

# Le thème

Le thème décide de l'apparence des pages que sert la gateway : le parcours de connexion, le
choix du tenant, les pages de mot de passe et du compte, les pages d'indisponibilité et
introuvable, les e-mails qu'elle envoie, ainsi que le bouton utilisateur et la barre de
portail qu'elle injecte dans les applications placées derrière elle.

Il se construit comme dans [Material Theme Builder](https://material-foundation.github.io/material-theme-builder/),
et c'est le même thème : six couleurs sources, un niveau de contraste et un « color match »,
d'où sont générés tous les rôles de Material 3, en clair et en sombre. Trois polices et un
interrupteur pour les effets décoratifs le complètent.

Il se modifie dans **Application > Built-in pages**, onglet **Theme**, avec à côté l'aperçu
des vraies pages.

![L'éditeur de thème : les couleurs sources, le contraste et la typographie à gauche, les deux aperçus à droite](img/console/built-in-pages-theme.webp)

Le nom, le logo et l'image de fond ne font pas partie d'un thème : ils forment la
[marque](/docs/customise/branding), qui reste la même quel que soit le thème actif. La
disposition de la page est la [disposition](/docs/customise/built-in-pages). Pour voir ce que
les trois produisent ensemble, voir [six marques, les mêmes pages](/docs/customise/overview).

## Six couleurs sources

Seule la **primaire** est obligatoire. Les autres sont dérivées d'elle tant que vous ne les
fixez pas ; l'éditeur les marque **derived** et affiche la valeur qu'elles prennent, et un
bouton remet une couleur fixée à sa valeur dérivée.

| Source | Ce qu'elle peint sur les pages |
|---|---|
| primaire | la marque et l'action : le bouton principal, le focus, les liens, le halo du logo |
| secondaire | ce qu'on choisit ou parcourt : l'option sélectionnée, la langue et le mode actifs, l'onglet courant de la barre de portail |
| tertiaire | l'état et l'information : les badges connecté et courant, les règles de mot de passe satisfaites, les explications, les codes à recopier |
| erreur | les refus et les champs invalides |
| neutre | la page, la carte et le texte |
| neutre variante | le texte secondaire, les indications, les contours |

## Les deux modes sont générés

À l'enregistrement, la gateway génère les **49 rôles** de Material 3, pour le mode clair et pour
le mode sombre, à partir des couleurs sources. Elle utilise le même algorithme que Material
Theme Builder (material-color-utilities) : les mêmes six couleurs donnent les mêmes modes là-bas
et ici, rôle pour rôle. Les deux modes sont générés ensemble et ne peuvent donc pas se
contredire ; l'éditeur liste chaque rôle, sombre et clair côte à côte, sous **Generated roles**,
en lecture seule.

- **Contrast** : standard, moyen ou élevé, appliqué aux deux modes.
- **Color match** est le « rester fidèle à mes couleurs » du builder : les conteneurs gardent le
  ton des couleurs données au lieu des tons que dérive la spécification. Sans lui, une couleur
  de marque très claire ou très sombre revient retouchée.

Les modes que les pages **proposent** se décident dans le même en-tête : décochez **Dark** ou
**Light** et les pages cessent de le proposer. L'un des deux reste toujours. Voir
[clair, sombre, ou le choix du visiteur](/docs/customise/built-in-pages).

## Glow

Une case pilote les effets décoratifs des pages intégrées : le halo d'ambiance derrière la
page, les lueurs du logo, des boutons et de la ligne d'état, et le dégradé sur le nom de
l'application. Décochez **Glow** pour un rendu plat. Dans l'API, c'est le champ `flat` du thème.

Un nom en dégradé est une affaire de goût, et de lisibilité : sur un fond clair, une primaire
pâle donne un nom pâle. Le rendu plat le peint d'une seule couleur.

## La typographie

Trois polices, chacune une des familles livrées avec la gateway, ou aucune pour celle du
système :

| Emplacement | Ce qu'il compose | Familles |
|---|---|---|
| Display | les titres et le nom de l'application | Inter, Roboto, Open Sans, Montserrat, Nunito, Raleway, Noto Sans, Source Serif 4, Lora, Merriweather, Playfair Display, Noto Serif |
| Body | le texte et les boutons | les mêmes douze |
| Code | les codes, les clés, les champs et les libellés | JetBrains Mono, Roboto Mono |

Les fichiers sont dans le binaire et la gateway les sert elle-même : un visiteur ne contacte
jamais un CDN de polices, une installation sans accès à Internet les a, et la
Content-Security-Policy d'une application (`'self'`) les laisse passer. Un navigateur ne
télécharge un fichier que pour un caractère de sa plage : une page en français coûte un seul
fichier latin.

La famille choisie dessine le latin et le cyrillique. Derrière n'importe quel choix, Noto
dessine l'arabe, l'hébreu, le devanagari et le thaï ; le chinois, le japonais et le coréen
utilisent les polices du système. Un e-mail ne porte que la pile de polices, parce que la
plupart des clients de messagerie ne chargent pas de police web.

## L'aperçu

La droite de l'écran montre la page dans les deux modes, sombre et clair, et suit **à chaque
choix**, avant tout enregistrement. Un carrousel au-dessus fait parcourir tout ce que peint un
thème : le spécimen de connexion, chaque page intégrée, les e-mails et la barre de portail.
Survoler une couleur source ou un rôle fait clignoter ce qu'il peint dans l'aperçu : c'est le
moyen le plus rapide de comprendre ce que désigne un nom de rôle.

## Plusieurs thèmes, un seul actif

Les thèmes sont des essais de couleurs ; un seul est actif, celui que voient les visiteurs. Le
carrousel entre les deux aperçus est la liste des thèmes :

- les **flèches** sélectionnent un thème, pour le regarder et le modifier ; sélectionner ne le
  met pas en ligne ;
- la **pastille du milieu** active le thème sélectionné, et avertit tant que le thème qu'elle
  porte n'est pas celui que voient les visiteurs ;
- le menu **more** à côté du nom le duplique ou le supprime. Le thème actif ne peut pas être
  supprimé : activez-en un autre d'abord.

Huit thèmes intégrés figurent dans le carrousel, en lecture seule, tous sur la même base avec
une primaire différente : Sentinel's Watch (celui par défaut), Midnight, Lavender, Orchid, Rose,
Crimson, Ember et Forest. Un cadenas remplace le crayon de renommage ; pour partir de l'un
d'eux, dupliquez-le.

## Emporter un thème ailleurs

**Export** écrit le format JSON de Material Theme Builder : les couleurs sources, les six
modes (clair et sombre aux trois niveaux de contraste) et les palettes tonales. Ce que seul
Meerkat connaît voyage sous une clé à part, `meerkat`, que les lecteurs du builder ignorent :
le nom, le contraste, le color match, les polices, Glow et les modes proposés.

**Import** lit un export du builder, venu de Meerkat ou du builder lui-même, et **remplit
l'éditeur** sans enregistrer : vous regardez le résultat dans l'aperçu, puis vous enregistrez.
L'état du Color match se lit dans les modes que porte le fichier. Un export antérieur aux
règles de contraste 2025 du builder s'importe par ses couleurs, et l'écran signale que
certains rôles diffèrent du fichier.

Un thème fait aussi partie d'un [export de configuration](/docs/operations/backup-restore),
avec le reste de l'installation.

## Les propriétés CSS

Les pages lisent le thème sous forme de propriétés personnalisées, émises une fois par page avec
la valeur claire et la valeur sombre côte à côte (`light-dark()` en CSS) : le mode du visiteur
choisit la palette, sans clignotement ni script.

- une par rôle : `--mk-` suivi du nom du rôle en kebab case, comme `--mk-primary`,
  `--mk-on-surface-variant` ou `--mk-surface-container-high` ;
- `--mk-night`, la teinte du halo d'ambiance, et `--mk-glow`, 1 ou 0 selon que Glow est activé
  ou non ;
- `--mk-font`, `--mk-mono` et `--mk-display`, les piles de polices ;
- `--mk-radius` et `--mk-radius-small`, les rayons des coins, qui ne se modifient pas.

Chaque valeur qu'émet le thème est une couleur générée par la gateway : rien de ce qui est
saisi dans l'éditeur n'atteint le bloc de style de la page tel quel.

## Ce qui manque

- **Les élévations.** Les ombres de Material 3 ne sont pas générées.
- **Les faces italiques**, et l'ajout d'une famille depuis l'écran plutôt qu'en la livrant.
- **La console ne porte pas le thème.** Elle garde l'apparence de Meerkat, par choix : c'est
  l'outil de l'exploitant, et un thème mal réglé ne doit pas rendre illisible l'écran où on le
  corrige.
- **Un thème par tenant.** Le thème est global, comme la connexion qu'il habille.
