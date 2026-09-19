---
title: Open source
section: Softwarity
order: 4
summary: Les composants, bibliothèques et outils que nous publions, parce que nous en avions besoin et que trimballer une copie de projet en projet est la façon dont un code vieillit.
---

# Open source

Tout ce qui suit existe parce que nous en avions besoin sur un vrai projet. Le
publier coûte un README et une chaîne de publication ; ne pas le publier coûte
le même composant, légèrement différent, dans chaque dépôt que nous touchons.
Nous avons choisi le premier.

## Angular

Le mobilier d'une console d'administration : les pièces que personne ne veut
réécrire et que tout le monde réécrit. Construits sur Angular Material, sur la
version majeure courante, en signal-first.

| Paquet | Ce que c'est |
| --- | --- |
| [`rail-nav`](https://github.com/softwarity/rail-nav) | Le rail de navigation Material Design 3, avec son tiroir contextuel. Le rail à gauche de ce site, c'est lui. |
| [`row-actions`](https://github.com/softwarity/row-actions) | Les actions de ligne d'un tableau Material, dans une barre qui se replie au lieu d'une colonne d'icônes. |
| [`loading-indicator`](https://github.com/softwarity/loading-indicator) | L'indicateur de chargement expressif de Material 3, avec son animation de morphing. |
| [`split-button`](https://github.com/softwarity/split-button) | Une directive de bouton scindé : l'action par défaut, et le menu à côté. |
| [`timezone-select`](https://github.com/softwarity/timezone-select) | Un sélecteur de fuseau horaire qui navigue par décalage UTC plutôt que par une liste alphabétique de quatre cents noms. |
| [`store`](https://github.com/softwarity/store) | Garder ce que le lecteur a choisi - colonnes visibles, tri, taille de page, filtres - dans le navigateur, avec un décorateur et aucun serveur. |

## Angular et i18n

Traduire une application Angular est un endroit où passe beaucoup de temps :
cela a son étagère.

| Paquet | Ce que c'est |
| --- | --- |
| [`polyglot`](https://github.com/softwarity/polyglot) | Sert toutes les locales d'une application i18n derrière un seul port de dev, lues dans `angular.json`. Il supprime la taxe « une locale par `ng serve` ». |
| [`angular-i18n-cli`](https://github.com/softwarity/angular-i18n-cli) | Met en place et entretient la configuration i18n d'un projet, au lieu de vous la faire retenir. |
| [Xliff translator](https://xliff.softwarity.io/fr/) | Un éditeur hébergé de catalogues XLIFF : l'étape de traduction, sans tableur. |

## NestJS

| Paquet | Ce que c'est |
| --- | --- |
| [`nestjs-granted`](https://github.com/softwarity/nestjs-granted) | Le RBAC des endpoints NestJS : autorisation par décorateurs, avec des expressions booléennes composables. |
| [`nestjs-amqp`](https://github.com/softwarity/nestjs-amqp) | AMQP 1.0 pour NestJS, sur rhea, avec publieurs et consommateurs par décorateurs. |

## Données en direct, et web components

| Paquet | Ce que c'est |
| --- | --- |
| [`livewire`](https://github.com/softwarity/livewire) | La synchronisation de requêtes en direct entre NestJS, Go et Angular : on s'abonne à une requête sur un seul WebSocket, on reçoit sa réponse et toutes les suivantes. Source de données à défilement virtuel comprise. |
| [`interactive-code`](https://github.com/softwarity/interactive-code) | Du code coloré et éditable au clic, avec sections repliables, copie et téléchargement. Agnostique du framework, zéro dépendance. |

## Météorologie aéronautique

Un domaine dans lequel nous travaillons depuis des années, et les outils qui en
sont sortis. Tous sont des web components ou des bibliothèques agnostiques du
framework : ils portent les règles de l'OACI et de l'OMM, pas un parti pris
d'interface.

| Projet | Ce que c'est |
| --- | --- |
| [Sigmet Draw](https://github.com/softwarity/sigmet-draw) | Dessiner et éditer les zones de danger des SIGMET OACI sur une carte, sur MapLibre, OpenLayers ou Leaflet. |
| [Sigwx Draw](https://github.com/softwarity/sigwx-draw) | Dessiner les cartes de temps significatif SIGWX et WAFS de l'OACI, sur les mêmes trois. |
| [TAC editor](https://github.com/softwarity/tac-editor) | Éditer les codes alphanumériques traditionnels de la météorologie aéronautique, avec coloration et validation. |
| [TTAAii provider](https://github.com/softwarity/ttaaii-provider) | Les données et la logique des en-têtes de bulletins TTAAii de l'OMM - complétion, validation, décodage - sans aucune interface attachée. |
| [GeoJSON editor](https://github.com/softwarity/geojson-editor) | Éditer des entités GeoJSON avec coloration, nœuds repliables et sélecteur de couleur. |

## Outils et images

| Projet | Ce que c'est |
| --- | --- |
| [plug](https://github.com/softwarity/plug) | Faire tourner un processus local comme s'il était dans votre cluster : DNS et services du cluster joignables, aucune configuration applicative. C'est le moteur du [mode développement](/product/dev-mode). |
| [Service PDFBox](https://github.com/softwarity/pdfbox) | Un petit service Spring Boot autonome qui transforme du HTML en PDF, PDF/A compris. On pousse du HTML, on récupère le binaire. |

## Produits

- **[Meerkat](https://github.com/softwarity/meerkat-ce)** - l'app-gateway dont
  parle ce site. Son cœur est sous Functional Source License et devient de
  l'Apache 2.0 deux ans après chaque version ; voir
  [Éditions](/product/editions).
