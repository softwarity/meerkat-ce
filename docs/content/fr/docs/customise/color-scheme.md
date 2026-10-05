---
title: Clair et sombre
section: Personnalisation
order: 255
summary: Comment indiquer à la gateway ce que votre application sait faire d'un mode clair ou sombre, et comment composer avec une application qui garde le sien.
---

# Clair et sombre

Un visiteur choisit le clair ou le sombre une seule fois, dans le bouton utilisateur, et
s'attend à ce que tout suive : les pages de Meerkat **et** l'application qui se trouve
derrière. Les pages, c'est à nous de les peindre. L'application est la vôtre, et il n'y en a
pas deux qui lisent ce mode de la même façon - c'est pourquoi le mécanisme se déclare **sur
la route** au lieu d'être deviné.

Cette page s'adresse à la personne qui intègre une application. Pour les pages que Meerkat
sert lui-même, voir [les pages intégrées](/docs/customise/built-in-pages).

## Où est conservé le choix

Le choix du visiteur est un cookie, `MEERKAT_SCHEME`, qui contient `light`, `dark` ou `auto`,
conservé un an et marqué `SameSite=Lax`. Il est aussi enregistré sur son compte, ce qui
permet de le rétablir sur un navigateur qui ne l'a jamais vu.

`auto` signifie "suivre le système", et c'est la valeur par défaut : rien n'est imposé tant
que personne n'a choisi.

## Ce que la gateway fait toujours

Sur une route d'interface dont le mécanisme n'est pas `none`, la gateway injecte un petit
agent. Quand un mode est choisi, il place sur `<html>` :

- la propriété CSS `color-scheme`, pour que les contrôles de formulaire, les barres de
 défilement et le fond par défaut suivent ;
- `data-meerkat-scheme="light"` ou `"dark"`, pour une application qui préfère lire un
 attribut plutôt qu'un style calculé.

En `auto`, les deux sont retirés et le navigateur recommence à suivre le système.

Cela suffit à une application dont le CSS repose sur `prefers-color-scheme` ou sur
`light-dark()`. Tout ce qui suit concerne les applications qui basculent autrement.

## Déclarer le mécanisme

Quatre réponses possibles, dont l'une est "il n'y a rien à basculer".

![Déclarer le mode de couleur sur une route](img/console/route-editor-color-scheme.webp)

| Mécanisme | Ce qu'écrit la gateway | Balisage type |
|---|---|---|
| *(aucun choisi)* | uniquement la propriété CSS `color-scheme` | votre CSS lit `prefers-color-scheme` |
| `attribute` | **un seul** attribut, que vous nommez, toujours écrit, avec la valeur du clair ou celle du sombre | `<html data-theme="dark">` |
| `add-attribute` | les deux valeurs **sont** des noms d'attributs, ajoutés sans valeur et retirés comme des classes | `<body dark-theme>` |
| `class` | les deux valeurs sont des noms de classes ; les deux sont retirées, puis celle du mode en cours est ajoutée | `<body class="dark">` |
| `none` | rien du tout | le clair et le sombre n'ont aucun sens pour cette interface |

Deux autres champs le précisent :

- **la balise**, `html` sauf indication contraire. Une application qui lit son thème sur
 `<body>` ne le verra jamais sur `<html>`, et c'est de loin la raison la plus fréquente pour
 laquelle une bascule semble ne rien faire.
- **la valeur du clair et la valeur du sombre.** Pour `attribute`, ce sont les deux valeurs
 de l'attribut ; pour les deux autres, ce sont les noms eux-mêmes.

C'est ce qui donne un **sens à une valeur vide** dans ces deux derniers mécanismes : rien
n'est posé sur la balise dans cet état. C'est le cas de figure le plus répandu - rien en
clair, `dark` en sombre :

| Mécanisme | clair | sombre | Résultat |
|---|---|---|---|
| `class` | *(vide)* | `dark` | `<body>` en clair, `<body class="dark">` en sombre |
| `add-attribute` | *(vide)* | `dark-theme` | `<body>` en clair, `<body dark-theme>` en sombre |
| `attribute` sur `data-theme` | `light` | `dark` | toujours écrit, avec l'une ou l'autre valeur |

Il se peut que la balise n'ait pas encore été analysée - l'agent s'exécute dès l'en-tête du
document, à dessein, pour que `<html>` soit habillé avant le premier affichage. Un `<body>`
qui n'existe pas encore est traité dès que le document est analysé, et l'agent n'écrit jamais
sur `<html>` à sa place : une classe laissée sur le mauvais élément est un thème que plus
rien ne retire.

## Ce que "none" veut dire, et ce qu'il ne veut pas dire

`none` déclare que cette interface n'a **aucun mode de couleur qui lui soit propre**. Cela ne
veut pas dire "elle prend la propriété CSS `color-scheme` et rien de plus" - cela, c'est
laisser le mécanisme vide. Cela veut dire : le clair et le sombre n'ont aucun sens ici.

La bascule n'est alors pas proposée dans le bouton utilisateur, et l'agent ne touche pas au
document.

## Proposer la bascule, et habiller le bouton

Deux choses qui allaient autrefois de pair et sont désormais séparées :

- **proposer la bascule** relève de l'habillage. Cela appartient au bouton utilisateur, et
 une route peut la proposer ou non.
- **la façon dont l'application reçoit un mode** appartient à la route, et vaut quel que soit
 l'élément qui propose la bascule - y compris une barre de [portail](/docs/customise/portal),
 où il n'y a pas de bouton propre à la route auquel la rattacher.

Quand une route ne propose **pas** la bascule, elle indique à la place ce que porte le bouton
injecté lui-même : le clair, le sombre ou le choix du visiteur. Ce réglage existe pour
l'application qui n'a qu'une seule apparence et pas de bascule : sans lui, le bouton suit le
système du visiteur et se retrouve en clair sur une page sombre. Il n'habille que l'élément
injecté ; la page n'est jamais modifiée.

## Les applications qui gardent leur propre thème

Voici le cas qui fait mal, et la raison d'être de cette page.

Une application qui mémorise son propre mode clair ou sombre dans `localStorage` **le
rétablit au chargement**, par-dessus ce que la gateway vient d'appliquer. Le choix du
visiteur tient jusqu'à l'exécution du script de l'application, puis l'ancien mode revient
brutalement. Le même portail paraît alors correct dans un navigateur et faux dans un autre,
et la seule différence tient à ce que ce navigateur avait en mémoire.

Lutter contre cela au niveau du document ne fonctionne pas. La solution consiste à composer
**avec** l'application, en écrivant dans son propre stockage.

| Champ | Ce que c'est |
|---|---|
| Override the application's stored theme | l'interrupteur. Sans lui, la gateway ne touche jamais au stockage de l'application |
| Key | la clé `localStorage` sous laquelle l'application range son choix : `theme`, `color-mode`, `vuetify:theme`... |
| Light | ce qu'il faut écrire pour le clair. Par défaut `light` |
| Dark | ce qu'il faut écrire pour le sombre. Par défaut `dark` |
| System | le nom que porte "suivre le système" dans cette application. **Laissé vide, l'entrée est supprimée** |

Ces valeurs appartiennent au **vocabulaire de l'application** - `dark`, `night`, `1` - et la
gateway ne cherche pas à les deviner : deviner reviendrait à écrire dans un dialecte que
nous ne parlons pas.

Une valeur System vide est un choix à part entière : quand rien n'est stocké, l'application
retombe sur son propre comportement par défaut qui, pour la plupart d'entre elles, *est* de
suivre le système.

L'écriture est faite par un script en ligne placé **avant** le script de démarrage de
l'application : la valeur est donc déjà en place quand l'application la lit pour la première
fois - et l'habillage injecté n'a plus qu'à hériter du document.

> [!TIP]
> `ng-m3-theme`, de l'écosystème Softwarity, est le cas qui nous l'a appris : son service
> conserve `system`, `light` ou `dark` sous une clé et, en mode `system`, il **efface** la
> propriété `color-scheme` du document à chaque exécution. Tout ce que posait la gateway
> disparaissait l'instant d'après. Écrire le choix dans cette clé laisse au contraire
> l'application l'appliquer comme elle sait déjà le faire, avant son premier affichage - ici,
> la valeur System est donc `system`, et non vide.

## Déboguer

Ce que la route a déclaré est transporté par la balise de script de l'agent lui-même : un
coup d'œil au HTML servi suffit pour savoir ce que la gateway pense avoir reçu comme
consigne :

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

Si un champ que vous avez rempli n'y figure pas, la route ne l'a pas enregistré. S'il y
figure et que rien ne se passe, la cause est le plus souvent la balise.
