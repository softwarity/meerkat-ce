---
title: Ce que la gateway injecte
section: Concepts
order: 24
summary: Le bouton utilisateur, la barre de navigation, l'agent de page et le mode clair ou sombre sont ajoutés par la gateway aux pages qu'elle relaie ; l'application n'a besoin d'aucune bibliothèque.
---

# Ce que la gateway injecte

Une route marquée **UI** sert des pages qu'un navigateur affiche, et la
gateway y ajoute quelques éléments au retour : un bouton utilisateur, une
barre de navigation, un petit script, et la préférence du visiteur entre clair
et sombre.

Elle les **ajoute**. On ne demande pas à l'application d'embarquer une
bibliothèque, d'appeler un endpoint, de parler un protocole ou d'être
reconstruite. C'est tout l'intérêt : une application interne qui tourne depuis
six ans reçoit un menu de déconnexion et un sélecteur de langue sans que
personne n'ouvre ses sources.

## Ce qui est ajouté

| | Ce que c'est | Activé par |
|---|---|---|
| Agent de page | `/meerkat/page.js`, qui installe `window.meerkatPage` | le fait que la route soit une route UI |
| Bouton utilisateur | un Web Component `meerkat-user-button` : qui vous êtes, l'organisation, la langue, la déconnexion | un interrupteur sur la route |
| Portail de navigation | une barre `meerkat-portal-nav` qui liste les applications que cette personne peut ouvrir | le catalogue global en mode `portal` ; elle remplace le bouton seul |
| Marquage d'identité | les rôles et les champs de l'utilisateur écrits dans le balisage même de la page, côté serveur | un interrupteur sur la route, champ par champ |
| CSS et JS personnalisés | ce que vous avez écrit dans la section Injections de la route | un contenu non vide |
| Point d'accroche de langue | une fonction que la gateway appelle quand le visiteur change de langue | une route dont la langue est transmise par script |

Le bouton utilisateur donne aussi accès à ce que la gateway détient pour
cette personne : ses applications, son profil, et **Signaler un problème**
lorsque le signalement de problèmes est activé.

## L'agent de page

`window.meerkatPage` est la petite API sur laquelle s'appuient les éléments
injectés, et la page peut s'en servir elle aussi :

- `data()` - ce que la gateway sait de ce visiteur, les mêmes données que celles dont le bouton tire son affichage.
- `applyScheme`, `pickScheme` - lire et changer le mode clair ou sombre.
- `applyLanguage`, `pickLanguage`, `resolvedLanguage` - la même chose pour la langue.
- `onEvent`, `onLanguage` - s'abonner à ce qu'annonce la gateway.
- `signedOut` - ce qu'il faut faire quand la session a disparu.

L'agent surveille aussi l'échéance de la session, grâce à un cookie compagnon
lisible, et renvoie la page vers le parcours de connexion quand elle est
dépassée - plutôt que de laisser la personne remplir un formulaire qui sera
refusé.

> [!NOTE]
> L'agent est un script global, pas un Web Component. Le bouton utilisateur, lui,
> en est un.

## Clair et sombre

Le choix du visiteur est conservé dans un cookie pendant un an **et** sur son
compte : un navigateur qui ne l'a jamais vu applique donc le bon mode dès la
connexion. Sur la page elle-même, la gateway fait trois choses :

1. elle pose `color-scheme` et `data-meerkat-scheme` sur l'élément racine - cela suffit à toute application qui respecte la propriété CSS ;
2. elle pilote le mécanisme **propre** à l'application, si elle en a un : un attribut nommé, deux attributs sans valeur, ou une classe, sur la balise que vous désignez ;
3. elle écrit la clé `localStorage` de l'application, dans le vocabulaire de l'application - et elle l'écrit **avant** que l'application ne démarre, depuis un petit script en ligne placé avant tout le reste.

C'est ce troisième point qui permet à un framework lisant son thème dans le
stockage au démarrage de s'afficher directement dans le bon mode, au lieu de
faire apparaître le mauvais un instant avant de se corriger.

> [!TIP]
> La palette du thème n'est **pas** injectée dans votre page. Elle voyage dans
> le JSON que récupèrent le bouton et la barre, et s'applique à l'intérieur de
> leur shadow root : rien de ce qu'ajoute la gateway ne peut modifier par
> accident le style de votre application.

## Ce que la page apprend de l'appelant

Il existe deux voies, qui répondent à des besoins différents.

**Le marquage** est écrit côté serveur dans le HTML, sans script et sans
aller-retour. Les rôles arrivent sous forme de classes, d'un attribut unique ou
d'une balise `<meta>`. Les champs de l'utilisateur arrivent de la même façon -
`username`, `email`, `tenant`, `tenantid`, `locale`, et les champs que votre
installation a définis. Tout est échappé à l'écriture.

C'est ce qui permet au CSS piloté par les rôles de fonctionner dès le tout
premier affichage : une page peut masquer un bouton à tous ceux qui ne
détiennent pas un rôle, avec une feuille de style et rien d'autre.

**Le JSON**, sur `/meerkat/user-button.json`, est ce que lit un script qui veut
la vue d'ensemble : l'identité, les organisations vers lesquelles cette
personne peut basculer, les applications qu'elle peut ouvrir, ses rôles et ses
groupes. Il est servi en `no-store`.

Aucune de ces deux voies n'est celle par laquelle un **service upstream** apprend
qui appelle. Il s'agit d'un mécanisme distinct - des en-têtes ou un JWT signé,
configurés dans la section Identity de la route - et la gateway commence par
purger toute valeur entrante de ces en-têtes : un appelant ne peut donc pas se
faire passer pour quelqu'un d'autre.

## D'où viennent les ressources

Tout ce qui se trouve sous `/meerkat/...` est servi par le moteur de la
gateway lui-même, et non par une route : `page.js`, `user-button.js`,
`portal.js` et les fichiers JSON qui les accompagnent. Les scripts sont mis en
cache cinq minutes ; tout ce qui porte une identité est en `no-store`.

> [!WARNING]
> `/meerkat/` est réservé. Une route dont les prédicats le couvrent ne recevra
> jamais les requêtes vers ces chemins.

## Quand rien n'est injecté

Réécrire une réponse impose de la mettre en mémoire tampon ; la gateway
limite donc volontairement les cas où elle le fait. Rien n'est injecté quand :

- la réponse n'est pas du `text/html` ;
- le corps n'est pas un **document complet** - un fragment HTML renvoyé à une requête XHR n'a pas de `<head>` et repart intact, car ajouter quoi que ce soit en tête d'un gabarit casse le framework qui l'a demandé ;
- le statut est 204, 304 ou 206 ;
- la connexion est un upgrade, ou le corps est un flux server-sent events ;
- la réponse indique `Cache-Control: no-transform` ;
- le corps dépasse le plafond de réécriture - 20 Mio par défaut, réglable de 1 à 256 ;
- le corps est compressé dans un format que la gateway ne sait pas réencoder. Les encodages identity, gzip et brotli font l'aller-retour ; **zstd, non**, et une réponse en zstd est transmise telle quelle.

Une injection abandonnée ne laisse aucune trace : ni erreur, ni en-tête, ni
ligne de journal. Si le bouton manque sur une page, commencez par vérifier les
conditions ci-dessus.

Une réponse réécrite perd l'`ETag` de l'upstream, puisque les octets ne sont plus
ceux dont l'upstream a calculé l'empreinte, et part en `no-cache` : le navigateur
interroge la gateway avant de la réutiliser, et un changement de
configuration se voit donc au chargement suivant. Cette question ne coûte rien
quand rien n'a changé : la page reçoit un `ETag` qui lui est propre, une
empreinte des octets réellement envoyés, et la gateway répond à un
`If-None-Match` concordant par un `304` vide. Modifiez quelque chose dans la
console, et les octets changent, donc l'empreinte aussi. Une réponse qui porte
une identité est marquée `no-store, private` à la place, sans validateur :
aucun cache partagé ne peut ainsi servir la page d'une personne à une autre.
