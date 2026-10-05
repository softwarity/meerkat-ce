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

## Une connexion, une identité

Le thème et la marque se décident **une seule fois**, pour toute la gateway : un même
parcours de connexion sert toutes les applications placées derrière elle, il n'a donc qu'une
apparence et qu'un nom. Plusieurs thèmes peuvent coexister, mais un seul est actif - les
thèmes sont des essais de couleurs, et l'identité ne se dédouble pas avec eux.

La console d'administration garde son apparence propre et ne suit pas le thème : c'est un
outil d'exploitant, elle ne fait pas partie de ce que votre produit donne à voir.

## Ce qui n'est pas personnalisable

- **Par organisation.** Il n'existe ni thème, ni marque, ni portail propres à une
 organisation. Ce serait la première surcharge visuelle par organisation du produit : elle
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
