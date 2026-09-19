---
title: Open source
section: Softwarity
order: 4
summary: Les composants et les outils que nous publions, parce que nous en avions besoin et que trimballer une copie de projet en projet est la façon dont un code vieillit.
---

# Open source

Tout ce qui suit existe parce que nous en avions besoin sur un vrai projet. Le
publier coûte un README et une chaîne de publication ; ne pas le publier coûte
le même composant, légèrement différent, dans chaque dépôt que nous touchons.
Nous avons choisi le premier.

## Composants Angular

Construits sur Angular Material, sur la version majeure courante, en
signal-first. C'est le mobilier d'une console d'administration : les pièces que
personne ne veut réécrire et que tout le monde réécrit.

| Paquet | Ce que c'est |
| --- | --- |
| [`@softwarity/rail-nav`](https://github.com/softwarity/rail-nav) | Le rail de navigation Material Design 3, avec son tiroir contextuel. Le rail à gauche de ce site, c'est lui. |
| [`@softwarity/row-actions`](https://github.com/softwarity/row-actions) | Les actions de ligne d'un tableau Material, dans une barre qui se replie au lieu d'une colonne d'icônes. |
| [`@softwarity/loading-indicator`](https://github.com/softwarity/loading-indicator) | L'indicateur de chargement expressif de Material 3, avec son animation de morphing. |
| [`@softwarity/split-button`](https://github.com/softwarity/split-button) | Une directive de bouton scindé : l'action par défaut, et le menu à côté. |
| [`@softwarity/timezone-select`](https://github.com/softwarity/timezone-select) | Un sélecteur de fuseau horaire qui navigue par décalage UTC plutôt que par une liste alphabétique de quatre cents noms. |
| [`@softwarity/livewire`](https://github.com/softwarity/livewire) | La synchronisation de requêtes en direct : on s'abonne à une requête sur un seul WebSocket, on reçoit sa réponse et toutes les suivantes. Source de données à défilement virtuel comprise. |
| [`@softwarity/projects`](https://github.com/softwarity/projects) | Le menu de la barre du haut de ce site : un web component construit à partir de Markdown. |

## Outils

- **[plug](https://github.com/softwarity/plug)** - le poste d'un développeur
  rejoint un cluster, et un service qui tourne en local répond sous son nom de
  cluster. C'est un produit à part entière, et le moteur du
  [mode développement](/product/dev-mode).

## Produits

- **[Meerkat](https://github.com/softwarity/meerkat-ce)** - l'app-gateway dont
  parle ce site. Son cœur est sous Functional Source License et devient de
  l'Apache 2.0 deux ans après chaque version ; voir
  [Éditions](/product/editions).
