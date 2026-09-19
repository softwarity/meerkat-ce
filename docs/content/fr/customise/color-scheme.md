---
title: Clair et sombre
section: Personnalisation
order: 255
summary: Comment dire à la passerelle ce que votre application sait faire d'un schéma de couleur, et comment travailler avec une application qui garde le sien.
---

# Clair et sombre

Un visiteur choisit clair ou sombre une fois, dans le bouton utilisateur, et il s'attend à ce que tout
suive : les pages de Meerkat **et** l'application derrière. Les pages sont à nous de peindre.
L'application est à vous, et aucune ne lit un schéma de la même façon - c'est pourquoi le mécanisme se
déclare **sur la route** au lieu d'être deviné (UIF-03).

Cette page est écrite pour la personne qui intègre une application. Pour les pages que Meerkat sert
lui-même, voir [les pages intégrées](/#/docs/customise/built-in-pages).

## Où vit le choix

Le choix du visiteur est un cookie, `MEERKAT_SCHEME`, qui porte `light`, `dark` ou `auto`, gardé un an et
en `SameSite=Lax`. Il est aussi stocké sur son compte, donc il est reposé sur un navigateur qui ne l'a
jamais vu.

`auto` veut dire « suivre le système », et c'est le défaut : rien n'est imposé jusqu'à ce que quelqu'un
choisisse.

## Ce que la passerelle fait toujours

Sur une route UI dont le schéma n'est pas `none`, la passerelle injecte un petit agent. Quand un schéma
est choisi, il pose sur `<html>` :

- la propriété CSS `color-scheme`, pour que les contrôles de formulaire, les barres de défilement et le
  fond par défaut suivent ;
- `data-meerkat-scheme="light"` ou `"dark"`, pour une application qui préfère lire un attribut qu'un
  style calculé.

Sur `auto` les deux sont retirés, et le navigateur revient à suivre le système.

Cela suffit à une application dont le CSS est écrit sur `prefers-color-scheme` ou sur `light-dark()`.
Tout ce qui suit est pour les applications qui basculent autrement.

## Déclarer le mécanisme

Quatre réponses, et l'une d'elles est « il n'y a rien à basculer ».

![Déclarer le schéma de couleur sur une route](img/console/route-editor-color-scheme.webp)

| Mécanisme | Ce que la passerelle écrit | Balisage typique |
|---|---|---|
| *(aucun choisi)* | la propriété CSS `color-scheme` seule | votre CSS lit `prefers-color-scheme` |
| `attribute` | **un** attribut, nommé par vous, toujours écrit, à la valeur claire ou à la sombre | `<html data-theme="dark">` |
| `add-attribute` | les deux valeurs **sont** des noms d'attribut, posés nus et retirés comme des classes | `<body dark-theme>` |
| `class` | les deux valeurs sont des noms de classe, les deux retirées et la courante ajoutée | `<body class="dark">` |
| `none` | rien du tout | clair et sombre ne veulent rien dire pour cette UI |

Deux champs de plus le façonnent :

- **la balise**, `html` sauf indication contraire. Une application qui lit son thème sur `<body>` ne l'a
  jamais vu sur `<html>`, et c'est de loin la première raison pour laquelle une bascule a l'air de ne
  rien faire.
- **la valeur claire et la valeur sombre.** Pour `attribute` ce sont les deux valeurs de l'attribut ;
  pour les deux autres ce sont les noms eux-mêmes.

D'où le fait qu'une **valeur vide veut dire quelque chose** dans les deux derniers : rien sur la balise
dans cet état. C'est la forme la plus répandue qui existe - rien en clair, `dark` en sombre :

| Mécanisme | clair | sombre | Résultat |
|---|---|---|---|
| `class` | *(vide)* | `dark` | `<body>` en clair, `<body class="dark">` en sombre |
| `add-attribute` | *(vide)* | `dark-theme` | `<body>` en clair, `<body dark-theme>` en sombre |
| `attribute` sur `data-theme` | `light` | `dark` | toujours écrit, l'un ou l'autre |

La balise n'est peut-être pas encore analysée - l'agent tourne depuis le head, exprès, pour que `<html>`
soit habillé avant le premier rendu. Un `<body>` qui n'existe pas encore est rattrapé une fois le document
analysé, et jamais écrit sur `<html>` à la place : une classe laissée sur le mauvais élément est un thème
que rien ne retire.

## Ce que "none" veut dire, et ce que ce n'est pas

`none` dit que cette UI n'a **aucun schéma de couleur à elle**. Ce n'est pas « elle prend la propriété CSS
`color-scheme` et rien de plus » - ça, c'est laisser le mécanisme vide. C'est : clair et sombre ne veulent
rien dire ici.

La bascule n'est alors pas offerte dans le bouton utilisateur, et l'agent laisse le document tranquille.

## Offrir la bascule, et habiller le bouton

Deux choses qui voyageaient ensemble et qui sont maintenant séparées :

- **offrir la bascule** est de l'habillage. Ça appartient au bouton utilisateur, et une route peut
  l'offrir ou non.
- **comment l'application consomme un schéma** appartient à la route, et reste vrai quel que soit celui
  qui offre la bascule - y compris une barre de [portail](/#/docs/customise/portal), où il n'y a pas de
  bouton par route auquel l'accrocher.

Quand une route n'offre **pas** la bascule, elle dit à la place ce que le bouton injecté porte lui-même :
clair, sombre, ou le choix de la personne. Ça existe pour l'application qui n'a qu'un seul aspect et pas
de bascule, où le bouton suit sinon le système du visiteur et flotte en clair sur une page sombre. Ça
habille l'habillage seul ; la page n'est jamais touchée.

## Les applications qui gardent leur propre thème

Voici le cas qui mord, et la raison d'être de cette page.

Une application qui mémorise son clair ou sombre dans `localStorage` **le restaure au chargement**,
par-dessus ce que la passerelle vient d'appliquer. Le choix du visiteur tient jusqu'à ce que le script de
l'application tourne, puis il revient en arrière. Le même portail a alors l'air juste dans un navigateur
et faux dans un autre, et ce qui diffère n'est que ce que ce navigateur avait en réserve.

Lutter sur le document ne marche pas. La façon de travailler **avec** est de parler son propre stockage.

| Champ | Ce que c'est |
|---|---|
| Surcharger son stockage | l'interrupteur. Sans lui la passerelle ne touche jamais au stockage de l'application |
| Clé | la clé `localStorage` sous laquelle l'application garde son choix : `theme`, `color-mode`, `vuetify:theme`... |
| Valeur claire | ce qu'il faut écrire pour le clair. `light` par défaut |
| Valeur sombre | ce qu'il faut écrire pour le sombre. `dark` par défaut |
| Valeur auto | comment « suivre le système » s'appelle là-bas. **Vide retire l'entrée** à la place |

Les valeurs sont le **vocabulaire propre** de l'application - `dark`, `night`, `1` - et la passerelle ne
les devine pas : deviner serait écrire un dialecte que nous ne parlons pas.

Une valeur auto vide est un choix à part entière : sans rien en réserve, l'application retombe sur son
propre défaut, qui pour la plupart d'entre elles *est* le système.

L'écriture se fait dans un script en ligne placé **avant** le script de démarrage de l'application, pour
que la valeur soit déjà en place quand l'application la lit pour la première fois - et l'habillage hérite
alors simplement du document.

> [!TIP]
> `ng-m3-theme` de l'écosystème Softwarity est le cas qui a enseigné ça : son service garde `system`,
> `light` ou `dark` sous une clé, et en mode `system` il **efface** la propriété `color-scheme` du document
> à chaque passage. Tout ce que la passerelle posait était balayé un tick plus tard. Écrire le choix dans
> cette clé laisse au contraire l'application l'appliquer comme elle sait déjà le faire, avant son premier
> rendu - donc ici la valeur auto est `system`, et non vide.

## Le déboguer

Ce que la route a déclaré voyage sur la balise de script de l'agent, donc un coup d'oeil au HTML servi dit
ce que la passerelle croit avoir reçu :

```html
<script defer src="/meerkat/page.js"
        data-scheme="select"
        data-scheme-mechanism="class"
        data-scheme-tag="body"
        data-scheme-light=""
        data-scheme-dark="dark"
        data-scheme-storage="theme"
        data-scheme-storage-light="light"
        data-scheme-storage-dark="dark"
        data-scheme-storage-auto="system"></script>
```

Si un champ que vous avez rempli est absent là, la route ne l'a pas enregistré. S'il est présent et que
rien ne se passe, c'est la balise qui est en général la réponse.
