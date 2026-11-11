---
title: Personnaliser Meerkat
section: Personnalisation
order: 240
summary: Ce qui peut prendre les couleurs de votre client, où se règle chaque élément, et ce qui reste global.
---

# Personnaliser Meerkat

Meerkat sert ses propres pages - le parcours de connexion, la page d'indisponibilité - et il
injecte un peu de lui-même dans les applications placées derrière lui. Les unes comme les
autres peuvent prendre l'apparence de votre produit plutôt que celle du nôtre.

## Ce qui est personnalisable, et où

| Quoi | Où cela se règle | Portée |
|---|---|---|
| [La palette](/docs/customise/theme) | Application > Built-in pages, onglet **Theme** | globale |
| [La disposition des pages](/docs/customise/built-in-pages) | même écran, onglet **Layout** | globale, Enterprise |
| [Nom, logo, favicon, fond](/docs/customise/branding) | même écran, onglet **Branding** | globale |
| Le mode clair ou sombre imposé sur ces pages | même écran, sur l'aperçu | globale |
| [Le catalogue des applications](/docs/customise/portal) | Application > Portal | globale |
| Les langues proposées | Routes > une route > Locales | déclarées route par route, l'offre est leur réunion |
| [La façon dont une application reçoit le clair et le sombre](/docs/customise/color-scheme) | la route | par route |
| [Du CSS et du JavaScript supplémentaires](/docs/customise/injections) | la route | par route |
| Le bouton utilisateur, son coin et son menu | la route | par route |

## Six marques, les mêmes pages

La même page de connexion, habillée pour six marques inventées pour l'occasion. Chacune est
un thème (couleurs et polices), une marque (nom, slogan, logo, fond) et une disposition, tous
réglés depuis les écrans listés plus haut : rien ici n'est du HTML ou du CSS sur mesure.

::: gallery compact
![Glimmo, disposition centered, en clair](img/login-examples/glimmo-centered-light.webp) **Glimmo**, une application de partage de photos : la disposition centered, celle par défaut, en clair.
![Glimmo, disposition centered, en sombre](img/login-examples/glimmo-centered-dark.webp) Le même Glimmo en sombre, avec son propre [fond sombre](/docs/customise/branding).
![Halvern Bank, disposition split, en clair](img/login-examples/halvern-split-light.webp) **Halvern Bank**, une banque privée : la disposition split, un titre à empattements et un texte sans.
![Halvern Bank, disposition split, en sombre](img/login-examples/halvern-split-dark.webp) La même banque en sombre : la [palette](/docs/customise/theme) a son côté sombre, le fond son image sombre.
![Maison Velune, disposition split, en sombre](img/login-examples/velune-split-dark.webp) **Maison Velune**, une maison de couture : la disposition split, cette fois l'image à gauche, en sombre.
![Maison Velune, disposition drawer, en clair](img/login-examples/velune-drawer-light.webp) La même maison en clair, en disposition drawer avec son panneau à gauche.
![Nightreel, disposition drawer, en sombre](img/login-examples/nightreel-drawer-dark.webp) **Nightreel**, un service de streaming : la disposition [drawer](/docs/customise/built-in-pages), son panneau contre le bord droit, en sombre.
![Monoforge, disposition bare, en sombre](img/login-examples/monoforge-bare-dark.webp) **Monoforge**, une plateforme pour développeurs : la disposition bare, sans carte, et la case [Glow](/docs/customise/theme) décochée, pour un rendu plat.
![Basketa, disposition banner, en clair](img/login-examples/basketa-banner-light.webp) **Basketa**, la livraison de courses : la disposition banner, la marque dans un bandeau en haut.
:::

Un fond se lit avec la page qu'il porte : une image faite pour le mode sombre est souvent
trop sombre derrière un texte clair, et inversement. Quand les pages proposent les deux
modes, donnez au fond [une image par mode](/docs/customise/branding).

Toutes les dispositions sauf `centered` relèvent d'Enterprise. Le script qui construit ces
marques et prend les captures est `e2e/scripts/capture-themes.mjs` : il tourne contre
n'importe quelle gateway, ce qui en fait un point de départ pour les vôtres.

## Une connexion, une identité

Le thème et la marque se décident **une seule fois**, pour toute la gateway : un même
parcours de connexion sert toutes les applications placées derrière elle, il n'a donc qu'une
apparence et qu'un nom. Plusieurs thèmes peuvent coexister, mais un seul est actif - les
thèmes sont des essais de couleurs, et l'identité ne se dédouble pas avec eux.

La console d'administration garde son apparence propre et ne suit pas le thème : c'est un
outil d'exploitant, elle ne fait pas partie de ce que votre produit donne à voir.

## Ce qui n'est pas personnalisable

- **Par tenant.** Il n'existe ni thème, ni marque, ni portail propres à un
 tenant. Ce serait la première surcharge visuelle par tenant du produit : elle
 est prévue, et nous préférons l'écrire ici plutôt que la livrer à moitié faite.
- **Par application.** Un groupe de routes partageant sa propre marque et ses propres langues
 n'existe pas : la configuration est globale et une route n'appartient à aucun groupe.
- **Votre propre HTML pour les pages du parcours.** La disposition se choisit dans un
 catalogue fermé de modèles livrés avec le produit ; remplacer le balisage d'une page n'est
 pas possible.

## Ce qui voyage

Tout ce que décrit cette page est de la configuration, et voyage donc dans un export de
configuration - à l'exception des images. Un export YAML simple n'emporte jamais d'image et
signale ce qu'il a laissé de côté ; un paquet `.zip` emporte les logos et le fond. Voir
[sauvegarde et restauration](/docs/operations/backup-restore).
