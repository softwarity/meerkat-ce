---
title: CSS et JavaScript par route
section: Personnalisation
order: 258
summary: Ce que la passerelle injecte déjà dans une page proxifiée, et où atterrissent votre CSS et votre JavaScript.
---

# CSS et JavaScript par route

Meerkat réécrit le HTML des routes UI qu'il proxifie, pour y ajouter le peu de lui-même dont le visiteur a
besoin - et pour y ajouter ce que vous écrivez sur la route (UIF-02, UIF-06, SAUTH-03).

Seules les réponses HTML sont touchées, et seulement sur les routes **UI**. Le JSON d'une route API n'est
jamais réécrit.

## Ce que la passerelle injecte déjà

| Quoi | Où ça atterrit | Quand |
|---|---|---|
| L'agent de page, `/meerkat/page.js` | en haut du `<body>` | sur chaque route UI |
| Le bouton utilisateur, ou la barre de [portail](/#/docs/customise/portal) | en haut du `<body>`, avec l'agent | selon la route et le portail |
| Votre CSS en plus | après `<head>` | quand la route en porte |
| Votre JavaScript en plus | après `<head>` | quand la route en porte |
| Le crochet de langue | après `<head>` | quand le mécanisme de langue de la route est un script |
| Le liseré de maintenance | après `<head>` | seulement pour l'administrateur qui a pris la porte |
| La surcharge de schéma dans le stockage de l'application | avant l'agent | quand la route en déclare une |

L'agent porte la langue du visiteur, son choix de clair ou sombre, et la surveillance de la session.
**Aucune route ne s'en dispense** : une page sans agent est une page qui continue d'avoir l'air connectée
des heures après la fin de la session.

L'agent et le bouton sont **une seule** injection, dans cet ordre, et délibérément : les deux scripts sont
en `defer`, donc ils tournent dans l'ordre du document, et l'élément du bouton ne doit pas se mettre à jour
avant que l'agent ait défini ce qu'il lit. Deux insertions séparées atterriraient chacune juste après
`<body>` et mettraient la seconde en premier.

Tout ce que la passerelle ajoute va en haut du **body**, jamais dans le head. Un élément personnalisé dans
`<head>` le ferme là où il se trouve, et tout ce qui suit - charset, titre, et `<base href>` - atterrit dans
le body, où un `base` est ignoré. Une application servie sous un préfixe résoudrait alors chaque URL depuis
la racine.

## Votre propre CSS et votre propre JavaScript

Deux éditeurs de code sur la route, dans ses options UI. Le CSS voyage dans une balise `<style>`, le
JavaScript dans une balise `<script>`, les deux tels quels.

| Règle | Pourquoi |
|---|---|
| Au plus `64 KiB` chacun | largement assez pour des retouches ; au-delà c'est un bundle, et un bundle appartient à l'application |
| `</style` et `</script` sont refusés à l'enregistrement | l'un ou l'autre sortirait de la balise dans laquelle il voyage |

Ils sont injectés après `<head>`, donc ils viennent **avant** les feuilles de style et les scripts de
l'application dans l'ordre du document. Pour le CSS, cela veut dire que les règles de l'application gagnent
à spécificité égale - écrivez avec ça en tête plutôt que d'attraper `!important` en premier.

## Styler selon les rôles du visiteur

Les **rôles effectifs** de la session peuvent être estampillés sur le HTML servi, **côté serveur** : en
classes, en un attribut sur une balise de votre choix, ou en `<meta>`. Aucun JavaScript côté client, aucun
appel qui rentre à la maison. Votre CSS peut alors cacher ou montrer des éléments par rôle avec un simple
sélecteur.

Le même mécanisme peut estampiller les faits de l'utilisateur - nom d'utilisateur, identifiant, nom complet,
e-mail, organisation, fuseau, locale - chacun sous un nom d'attribut ou de meta que vous choisissez.

> [!WARNING]
> Une page ainsi estampillée appartient à **une personne**, donc la passerelle la rend non stockable :
> `no-store`, et les en-têtes qui pourraient dire l'inverse - `Expires`, `Pragma`, `Last-Modified`, `ETag` -
> sont retirés, quoi qu'en dise l'application derrière. Sans quoi un `public, max-age=300` - le réglage par
> défaut de tout serveur de fichiers statiques - ferait servir la page d'Alice à Bob par un CDN, un proxy
> d'entreprise ou un navigateur partagé.

Une requête anonyme n'est pas estampillée et garde la mise en cache de l'application.

## Le coût

Réécrire un corps veut dire le tenir en mémoire. Le plafond est dans **Routes > Global**, entre un et deux
cent cinquante-six MiB, et vingt par défaut. Au-delà, la réponse passe **intacte et entière** : rien ne
casse, l'injection ne s'applique simplement pas. Le coût est par requête servie à cet instant, donc un grand
plafond se multiplie par le nombre de requêtes qui arrivent ensemble.

L'estampillage des rôles est conditionné à une vérification de session peu coûteuse, donc les requêtes
anonymes ne mettent jamais rien en tampon.

## Ce qui manque

- **La réécriture de `<base href>`** en cohérence avec un préfixe retiré (UIF-01) : rien ne le réécrit, ce
  qui est pourquoi l'injection prend soin de ne jamais fermer le head.
- **Un crochet post-authentification** et des presets pour les outils courants (SAUTH-03) : le bloc
  JavaScript est là, le crochet qui tournerait à la connexion ne l'est pas.
- **La réécriture des specs OpenAPI dans les réponses proxifiées** (UIF-07) : seul le portail de
  documentation pour développeurs réécrit la spec qu'il sert.
