---
title: Open source
section: Softwarity
order: 4
summary: Les composants, les bibliothèques et les outils que nous publions, parce que nous en avions besoin et qu'une base de code vieillit à force de traîner une copie d'un projet à l'autre.
---

# Open source

Tout ce qui suit existe parce que nous en avons eu besoin sur un vrai projet.
Le publier coûte un README et une chaîne de publication ; ne pas le publier
coûte le même composant, légèrement différent, dans chaque dépôt auquel nous
touchons. Nous avons choisi la première option.

## Angular

Le mobilier d'une console d'administration : les pièces que personne ne veut
réécrire, et que tout le monde réécrit. Elles reposent sur Angular Material,
sur la version majeure en cours, et donnent la priorité aux signaux.

| Paquet | Ce que c'est |
| --- | --- |
| [`rail-nav`](https://github.com/softwarity/rail-nav) | Le rail de navigation de Material Design 3, avec son tiroir contextuel. Le rail à gauche de ce site, c'est lui. |
| [`row-actions`](https://github.com/softwarity/row-actions) | Les actions de ligne d'un tableau Material, dans une barre d'outils qui se replie, à la place d'une colonne d'icônes. |
| [`loading-indicator`](https://github.com/softwarity/loading-indicator) | L'indicateur de chargement expressif de Material 3, avec son animation de métamorphose. |
| [`split-button`](https://github.com/softwarity/split-button) | Une directive de bouton partagé : l'action par défaut, et le menu à côté. |
| [`timezone-select`](https://github.com/softwarity/timezone-select) | Un sélecteur de fuseau horaire où l'on navigue par décalage UTC plutôt que dans une liste alphabétique de quatre cents noms. |
| [`store`](https://github.com/softwarity/store) | Conserve dans le navigateur ce que l'utilisateur a choisi - colonnes visibles, tri, taille de page, filtres - avec un seul décorateur et sans serveur. |

## Angular et i18n

Traduire une application Angular prend beaucoup de temps : le sujet a donc sa
propre étagère.

| Paquet | Ce que c'est |
| --- | --- |
| [`polyglot`](https://github.com/softwarity/polyglot) | Sert toutes les langues d'une application i18n en même temps, derrière un seul port de développement, d'après `angular.json`. Il supprime la contrainte "une langue par `ng serve`". |
| [`angular-i18n-cli`](https://github.com/softwarity/angular-i18n-cli) | Met en place et entretient la configuration i18n d'un projet, pour que vous n'ayez pas à la retenir. |
| [Xliff translator](https://xliff.softwarity.io/fr/) | Un éditeur en ligne pour les catalogues XLIFF : l'étape de traduction, sans tableur. |

## NestJS

| Paquet | Ce que c'est |
| --- | --- |
| [`nestjs-granted`](https://github.com/softwarity/nestjs-granted) | Le contrôle d'accès par rôles (RBAC) pour les endpoints NestJS : des autorisations posées par décorateur, avec des expressions booléennes composables. |
| [`nestjs-amqp`](https://github.com/softwarity/nestjs-amqp) | AMQP 1.0 pour NestJS, bâti sur rhea, avec des éditeurs et des consommateurs déclarés par décorateur. |

## Données en direct et composants web

| Paquet | Ce que c'est |
| --- | --- |
| [`livewire`](https://github.com/softwarity/livewire) | La synchronisation de requêtes en direct entre NestJS, Go et Angular : abonnez-vous à une requête sur un seul WebSocket, recevez sa réponse et toutes celles qui suivent. Une source de données pour le défilement virtuel est fournie. |
| [`interactive-code`](https://github.com/softwarity/interactive-code) | Du code avec coloration syntaxique, modifiable d'un clic, avec des sections repliables, la copie et le téléchargement. Indépendant de tout framework, sans aucune dépendance. |

## Météorologie aéronautique

Un domaine dans lequel nous travaillons depuis des années, et les outils qui en
sont issus. Ce sont tous des composants web ou des bibliothèques indépendants
de tout framework : ils portent les règles de l'OACI et de l'OMM, pas un parti
pris d'interface.

| Projet | Ce que c'est |
| --- | --- |
| [Sigmet Draw](https://github.com/softwarity/sigmet-draw) | Dessiner et modifier sur une carte les zones de danger des SIGMET de l'OACI, avec MapLibre, OpenLayers ou Leaflet. |
| [Sigwx Draw](https://github.com/softwarity/sigwx-draw) | Dessiner les cartes de temps significatif SIGWX et WAFS de l'OACI, avec les trois mêmes bibliothèques. |
| [TAC editor](https://github.com/softwarity/tac-editor) | Modifier les codes alphanumériques traditionnels (TAC) de la météorologie aéronautique, avec coloration et validation. |
| [TTAAii provider](https://github.com/softwarity/ttaaii-provider) | Les données et la logique des en-têtes de bulletins TTAAii de l'OMM - complétion, validation, décodage - sans aucune interface. |
| [GeoJSON editor](https://github.com/softwarity/geojson-editor) | Modifier des entités GeoJSON, avec coloration, nœuds repliables et sélecteur de couleur. |

## Outils et images

| Projet | Ce que c'est |
| --- | --- |
| [plug](https://github.com/softwarity/plug) | Faire tourner un processus local comme s'il était dans votre cluster : le DNS et les services du cluster sont joignables, sans configuration de l'application. C'est le moteur du [mode développement](/product/dev-mode). |
| [PDFBox service](https://github.com/softwarity/pdfbox) | Un petit service Spring Boot autonome qui transforme du HTML en PDF, PDF/A compris. Envoyez du HTML, récupérez le binaire. |

## Produits

- **[Meerkat](https://github.com/softwarity/meerkat-ce)** - l'app-gateway à
  laquelle ce site est consacré. Son cœur est sous Functional Source License et
  passe sous licence Apache 2.0 deux ans après la publication de chaque
  version ; voir [Éditions](/product/editions).
