---
title: CSS et JavaScript par route
section: Personnalisation
order: 258
summary: Ce que la gateway injecte déjà dans une page qu'elle relaie, et où arrivent votre CSS et votre JavaScript.
---

# CSS et JavaScript par route

Meerkat réécrit le HTML des routes d'interface qu'il relaie, pour y ajouter le peu de
lui-même dont le visiteur a besoin - et pour y ajouter ce que vous écrivez sur la route.

Seules les réponses HTML sont modifiées, et uniquement sur les routes **UI**. Le JSON d'une
route d'API n'est jamais réécrit.

## Ce que la gateway injecte déjà

| Quoi | Où cela arrive | Quand |
|---|---|---|
| L'agent de page, `/meerkat/page.js` | au début de `<body>` | sur toutes les routes UI |
| Le bouton utilisateur, ou la barre du [portail](/docs/customise/portal) | au début de `<body>`, avec l'agent | selon la route et le portail |
| Votre CSS supplémentaire | après `<head>` | quand la route en contient |
| Votre JavaScript supplémentaire | après `<head>` | quand la route en contient |
| Le point d'accroche de la langue | après `<head>` | quand le mécanisme de langue de la route est un script |
| Le bandeau de maintenance | après `<head>` | uniquement pour l'administrateur entré par l'accès de maintenance |
| L'écriture du mode clair ou sombre dans le stockage de l'application | avant l'agent | quand la route en déclare une |

L'agent transporte la langue du visiteur, son choix du clair ou du sombre, et la surveillance
de la session. **Aucune route ne peut s'en dispenser** : une page sans agent continue de
paraître connectée des heures après la fin de la session.

L'agent et le bouton forment **une seule** injection, dans cet ordre, et c'est voulu : les
deux scripts sont différés, ils s'exécutent donc dans l'ordre du document, et l'élément du
bouton ne doit pas s'initialiser avant que l'agent ait défini ce qu'il lit. Deux insertions
distinctes arriveraient chacune juste après `<body>`, ce qui placerait la seconde en premier.

L'agent et le bouton sont placés au début du **body**, jamais dans le head. Un élément
personnalisé placé dans `<head>` le referme à l'endroit où il se trouve, et tout ce qui le
suit - le jeu de caractères, le titre et `<base href>` - se retrouve dans le body, où une telle
balise est ignorée. Une application servie sous un préfixe résoudrait alors toutes ses
URL depuis la racine.

## Votre propre CSS et votre propre JavaScript

Deux éditeurs de code sur la route, parmi ses options d'interface. Le CSS est transporté
dans une balise `<style>`, le JavaScript dans une balise `<script>`, l'un et l'autre tels
quels.

| Règle | Pourquoi |
|---|---|
| Au plus `64 KiB` chacun | c'est largement assez pour retoucher une page ; au-delà, il s'agit d'un bundle, et un bundle a sa place dans l'application |
| `</style` et `</script` sont refusés à l'enregistrement | l'un comme l'autre ferait sortir le contenu de la balise qui le transporte |

Ils sont injectés après `<head>` et passent donc **avant** les feuilles de style et les
scripts de l'application dans l'ordre du document. Pour le CSS, cela signifie que les règles
de l'application l'emportent à spécificité égale - écrivez en en tenant compte, plutôt que
de recourir d'emblée à `!important`.

## Adapter le style aux rôles du visiteur

Les **rôles effectifs** de la session peuvent être inscrits dans le HTML servi, **côté
serveur** : sous forme de classes, d'un attribut sur la balise de votre choix, ou d'une
balise `<meta>`. Aucun JavaScript côté client, aucun appel de retour vers la gateway.
Votre CSS peut alors masquer ou afficher des éléments selon le rôle, avec un simple
sélecteur.

Le même mécanisme peut inscrire les informations propres à l'utilisateur - nom d'utilisateur,
identifiant, nom complet, adresse e-mail, organisation, fuseau horaire, langue - chacune sous
le nom d'attribut ou de meta que vous choisissez.

> [!WARNING]
> Une page marquée de cette façon appartient à **une seule personne**. La gateway interdit
> donc de la conserver : elle pose `no-store` et retire les en-têtes qui pourraient dire le
> contraire - `Expires`, `Pragma`, `Last-Modified`, `ETag` - quoi qu'en dise l'application
> derrière elle. Sans cela, un `public, max-age=300` - le réglage par défaut de tous les
> serveurs de fichiers statiques - conduirait un CDN, un proxy d'entreprise ou un navigateur
> partagé à servir la page d'Alice à Bob.

Une requête anonyme n'est pas marquée et conserve la politique de cache de l'application.

## Le coût

Réécrire un corps de réponse oblige à le garder en mémoire. Le plafond se règle dans
**Routes > Global**, entre un et deux cent cinquante-six MiB ; il vaut vingt par défaut.
Au-delà, la réponse est transmise **intacte et entière** : rien ne casse, l'injection ne
s'applique tout simplement pas. Ce coût se paie pour chaque requête en cours de traitement à
un instant donné : un plafond élevé se multiplie donc par le nombre de requêtes qui arrivent
en même temps.

L'inscription des rôles est conditionnée par une vérification de session peu coûteuse, si
bien que les requêtes anonymes ne mettent jamais rien en mémoire tampon.

## Ce qui manque

- **La réécriture de `<base href>`** pour tenir compte d'un préfixe retiré : rien ne la
 réécrit, et c'est pourquoi l'injection veille à ne jamais refermer le head.
- **Un point d'accroche après l'authentification** et des préréglages pour les outils
 courants : le bloc JavaScript existe, pas le code qui s'exécuterait à la connexion.
- **La réécriture des spécifications OpenAPI dans les réponses relayées** : seul le portail
 de documentation destiné aux développeurs réécrit la spécification qu'il sert.
