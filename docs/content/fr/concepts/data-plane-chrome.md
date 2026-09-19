---
title: Ce que la gateway injecte
section: Concepts
order: 24
summary: Le bouton utilisateur, la barre de navigation, l'agent de page et le schéma de couleurs sont ajoutés aux pages proxifiées par la gateway, pour que l'application n'ait aucune bibliothèque à embarquer.
---

# Ce que la gateway injecte

Une route marquée **UI** sert des pages qu'un navigateur affiche, et la gateway y
ajoute quelques choses au retour : un bouton utilisateur, une barre de navigation,
un petit script, et la préférence clair ou sombre du visiteur.

Elle les **ajoute**. On ne demande pas à l'application d'embarquer une
bibliothèque, d'appeler un endpoint, de parler un protocole ni d'être reconstruite.
C'est bien le but : une application interne qui tourne depuis six ans gagne un menu
de déconnexion et un sélecteur de langue sans que personne n'ouvre ses sources.

## Ce qui est ajouté

| | Ce que c'est | Déclenché par |
|---|---|---|
| Agent de page | `/meerkat/page.js`, qui installe `window.meerkatPage` | le fait que la route soit une route UI |
| Bouton utilisateur | un Web Component `meerkat-user-button` : qui vous êtes, l'organisation, la langue, la déconnexion | un interrupteur sur la route |
| Portail de navigation | une barre `meerkat-portal-nav` listant les applications que cette personne peut ouvrir | un interrupteur global ; elle remplace le bouton isolé |
| Estampille d'identité | les rôles et les champs du compte écrits dans le balisage de la page, côté serveur | un interrupteur sur la route, champ par champ |
| CSS et JS maison | ce que vous avez écrit dans la section Injections de la route | leur simple présence |
| Rappel de langue | une fonction que la gateway appelle quand le visiteur change de langue | une route dont les langues voyagent par script |

Le bouton utilisateur porte aussi les portes d'entrée de ce que la gateway détient
pour cette personne : ses applications, son profil, et **Signaler un problème**
quand l'interrupteur des anomalies est ouvert.

## L'agent de page

`window.meerkatPage` est la petite API sur laquelle tourne le décor injecté, et la
page peut s'en servir aussi :

- `data()` - ce que la gateway sait de ce visiteur, la charge même dont le bouton se sert pour se dessiner.
- `applyScheme`, `pickScheme` - lire et changer le clair ou le sombre.
- `applyLanguage`, `pickLanguage`, `resolvedLanguage` - la même chose pour la langue.
- `onEvent`, `onLanguage` - s'abonner à ce que la gateway annonce.
- `signedOut` - quoi faire quand la session n'est plus là.

L'agent surveille aussi l'échéance de la session, à travers un cookie compagnon
lisible, et envoie la page vers le flux de connexion quand elle expire - plutôt que
de laisser la personne saisir un formulaire qui sera refusé.

> [!NOTE]
> L'agent est un script global, pas un Web Component. Le bouton utilisateur, lui,
> en est un.

## Clair et sombre

Le choix du visiteur vit dans un cookie d'un an **et** sur son compte, pour qu'un
navigateur qui ne l'a jamais vu soit quand même juste après la connexion. Sur la
page elle-même, la gateway fait trois choses :

1. elle pose `color-scheme` et `data-meerkat-scheme` sur l'élément racine - ce qui suffit à toute application qui respecte la propriété CSS ;
2. elle pilote le mécanisme **propre** à l'application, si elle en a un : un attribut nommé, deux attributs nus, ou une classe, sur la balise que vous désignez ;
3. elle écrit la clé `localStorage` de l'application, dans le vocabulaire de l'application - et elle l'écrit **avant** que l'application démarre, depuis un petit script en ligne placé devant tout le reste.

Le troisième point est ce qui fait qu'un framework qui lit son thème dans le
stockage au démarrage se lève dans le bon, au lieu d'afficher le mauvais une
fraction de seconde puis de se corriger.

> [!TIP]
> La palette du thème n'est **pas** injectée dans votre page. Elle voyage dans le
> JSON que le bouton et la barre vont chercher, et ne s'applique qu'à l'intérieur
> de leurs shadow roots : rien de ce que la gateway ajoute ne peut restyler votre
> application par accident.

## Ce que la page apprend de l'appelant

Deux chemins, et ils répondent à des besoins différents.

**L'estampille** est écrite côté serveur dans le HTML, sans script et sans
aller-retour. Les rôles arrivent en jetons de classe, ou en un attribut, ou en une
balise `<meta>`. Les champs du compte arrivent de la même façon - `username`,
`email`, `tenant`, `tenantid`, `locale`, et les champs que votre installation a
définis. Tout est échappé au passage.

C'est ce qui fait marcher une visibilité pilotée par les rôles dès le premier
rendu : une page peut cacher un bouton pour tous ceux qui ne détiennent pas un
rôle, avec une feuille de style et rien d'autre.

**Le JSON**, sur `/meerkat/user-button.json`, est ce qu'un script lit quand il veut
le tableau complet : l'identité, les organisations vers lesquelles cette personne
peut basculer, les applications qu'elle peut ouvrir, ses rôles et ses groupes. Il
est servi en `no-store`.

Aucun des deux n'est la façon dont un **service amont** apprend qui appelle. C'est
un mécanisme distinct - en-têtes ou JWT signé, configuré dans la section Identity de
la route - et la gateway purge d'abord toute valeur entrante de ces en-têtes, pour
qu'un appelant ne puisse pas se prétendre quelqu'un.

## D'où viennent les ressources

Tout ce qui est sous `/meerkat/...` est servi par le moteur de la gateway et non
par une route : `page.js`, `user-button.js`, `portal.js` et leurs compagnons JSON.
Les scripts sont mis en cache cinq minutes ; tout ce qui porte une identité est en
`no-store`.

> [!WARNING]
> `/meerkat/` est réservé. Une route dont les prédicats le couvrent ne sera jamais
> atteinte pour ces chemins.

## Quand rien n'est injecté

Réécrire une réponse veut dire la mettre en mémoire tampon, donc la gateway est
volontairement étroite sur les cas où elle le fait. Rien n'est injecté quand :

- la réponse n'est pas du `text/html` ;
- le corps n'est pas un **document entier** - un fragment HTML rendu à un XHR n'a pas de `<head>` et est rendu intact, parce que préfixer quoi que ce soit à un gabarit casse le framework qui l'a demandé ;
- le statut est 204, 304 ou 206 ;
- la connexion est une bascule de protocole, ou le corps est un flux server-sent-events ;
- la réponse dit `Cache-Control: no-transform` ;
- le corps dépasse le plafond de réécriture - 20 Mio par défaut, réglable entre 1 et 256 ;
- le corps est compressé avec quelque chose que la gateway ne sait pas réencoder. Identity, gzip et brotli font l'aller-retour ; **zstd non**, et une réponse zstd passe telle quelle.

Une injection sautée est silencieuse : pas d'erreur, pas d'en-tête, pas de ligne de
journal. Si le bouton manque sur une page, une des conditions ci-dessus est le
premier endroit à regarder.

Une réponse réécrite perd aussi son `ETag`, puisque les octets ne sont plus ceux
que l'amont a hachés, et une réponse qui porte une identité est marquée `no-store,
private` pour qu'aucun cache partagé ne rende à quelqu'un la page d'un autre.
