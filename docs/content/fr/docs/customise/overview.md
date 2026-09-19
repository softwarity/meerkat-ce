---
title: Personnaliser Meerkat
section: Personnalisation
order: 240
summary: Ce qui peut porter les couleurs de votre client, où chaque pièce se règle, et ce qui reste global.
---

# Personnaliser Meerkat

Meerkat sert des pages à lui - le flux de connexion, la page d'indisponibilité - et il injecte un peu
de lui-même dans les applications qu'il proxifie. Les deux peuvent ressembler à votre produit plutôt
qu'au nôtre.

## Ce qui est personnalisable, et où

| Quoi | Où ça se règle | Portée |
|---|---|---|
| [La palette](/docs/customise/theme) | Application > Built-in pages, onglet **Theme** | globale |
| [La disposition des pages](/docs/customise/built-in-pages) | même écran, onglet **Layout** | globale, Enterprise |
| [Nom, logo, favicon, fond](/docs/customise/branding) | même écran, onglet **Branding** | globale |
| Clair ou sombre imposé sur ces pages | même écran, sur l'aperçu | globale |
| [Le portail de navigation](/docs/customise/portal) | Application > Portal | globale |
| Les langues offertes | Application > Locales | réserve globale, offre par route |
| [Comment une application consomme clair et sombre](/docs/customise/color-scheme) | la route | par route |
| [CSS et JavaScript en plus](/docs/customise/injections) | la route | par route |
| Le bouton utilisateur, son coin et son menu | la route | par route |

## Une connexion, une identité

Le thème et la marque se décident **une fois**, globalement (THEME-01) : une seule procédure de
connexion sert toutes les applications derrière la passerelle, donc elle a un seul aspect et un seul
nom. Plusieurs thèmes peuvent coexister, mais exactement un est actif - les thèmes sont des essais de
couleur, et l'identité ne se dédouble pas avec eux.

La console d'administration garde son propre aspect et n'est pas thémée : c'est un outil d'exploitant,
pas une partie de la surface de votre produit.

## Ce qui n'est pas personnalisable

- **Par organisation.** Il n'y a ni thème, ni marque, ni portail par tenant. Ce serait la première
  surcharge visuelle par tenant du produit, et c'est écrit comme PORTAL-02 plutôt que construit à
  moitié.
- **Par application.** Un groupe de routes portant sa propre marque et ses propres locales est SVC-05,
  et ça n'existe pas : la configuration est globale et une route ne se rattache à aucun groupe.
- **Votre propre HTML pour les pages du flux.** La disposition est un catalogue fermé de gabarits
  livrés avec le produit ; remplacer le balisage d'une page est l'autre moitié de PAGE-02 et n'est pas
  construit.
- **Les catalogues de langues.** Vingt langues sont embarquées ; en ajouter une reste une
  recompilation (PAGE-03).

## Ce qui voyage

Tout ce qui est sur cette page est de la configuration, donc ça voyage dans un export de
configuration - sauf les images. Un export YAML nu ne porte jamais d'image et dit ce qu'il a laissé
derrière lui ; un paquet `.zip` porte les logos et le fond. Voir
[sauvegarde et restauration](/docs/operations/backup-restore).
