---
title: Routes
section: La console
order: 152
summary: La table de routage dans l'ordre où elle est lue, et le fonctionnement de l'éditeur de route.
---

# Routes

**Infra > Routes.** Toute la table de routage, dans l'ordre où la passerelle la
lit, et l'éditeur dans lequel chaque route s'ouvre. C'est l'écran sur lequel on
passe le plus de temps.

![L'écran Routes : cinq routes dans l'ordre, avec leurs pastilles d'accès, ce qu'elles filtrent et leur amont](img/console/routes-list.webp)

Cinq routes dans l'ordre où elles sont lues. *Billing* et *Docs portal* portent la
marque UI, *Orders API* montre un amont qui échoue, *Inventory* est en
maintenance, et *Catch-all* sur `path: /**` est en dernier - un attrape-tout
ailleurs répondrait pour tout ce qui est en dessous.

## La liste

Une ligne par route : son nom, ce qu'elle décide de l'accès, ce qu'elle filtre, et
où elle envoie. L'ordre compte - **la première route qui matche gagne** - donc le
tableau est ordonné, pas trié.

- **La poignée de glissement** déplace une route vers le haut ou le bas et
  enregistre aussitôt. Elle ne marche que sur la liste entière : avec une recherche
  ou un filtre actif, elle se tait et dit pourquoi, car faire passer la ligne trois
  au-dessus de la ligne une d'une vue filtrée la placerait au-dessus de ce qui est
  réellement premier.
- **Le point** devant le nom dit activée ou désactivée. Une deuxième marque
  apparaît pour une **route UI** (celle qui sert des pages dans un navigateur), et
  une troisième quand la passerelle a réellement vu l'amont échouer.
- **Les pastilles d'accès** disent ce que Meerkat exige lui-même : `AUTH` connecté,
  `ORG` dans une organisation, `ORG-2` dans l'une de deux organisations nommées,
  `DENY` personne, un tiret pour délégué. Une pastille comptant les endpoints
  apparaît quand la route affine l'accès par opération, et son clic ouvre
  [Endpoint security](/#/docs/console/endpoints).
- **Les actions de ligne** : ouvrir la route dans le plan de données (routes UI
  seulement), activer ou désactiver, dupliquer, supprimer.
- **La recherche** matche le nom de la route, son amont et ses motifs de chemin -
  c'est-à-dire ce dont on se souvient d'elle. Le sélecteur à côté restreint aux
  routes UI ou aux routes de service.

**Dupliquer** fabrique une copie à l'identité neuve, **désactivée**, posée juste
après l'originale. C'est la façon prévue d'essayer une variante : un autre amont,
l'accès d'une autre organisation, comparés côte à côte sans tout retaper.

## Trois boutons dans la bannière

- **Routing test** compose une requête fictive et dit quelle route la prend. A
  utiliser avant de déplacer quoi que ce soit : il répond à la seule question dont
  l'ordre est le sujet.
- **Global** porte ce qui est vrai de toutes les routes à la fois : l'interrupteur
  **Unavailable** qui les ferme toutes (les pages de connexion continuent, et
  quiconque administre ou développe ici passe encore), combien de temps une route
  attend un service avant de répondre 502, et le plafond de ce qu'un filtre de
  réécriture de corps peut tenir en mémoire. Le bouton lui-même passe à
  *Unavailable* tant que l'interrupteur est actif, pour que personne n'ait à
  l'ouvrir pour le savoir.
- **JWT** porte les clés qui signent le JWT d'identité que vos services vérifient :
  le JWKS à donner à un backend, la moitié publique de chaque algorithme, la
  rotation, et quelles routes signent avec quoi.

## L'éditeur

Cliquer une ligne ouvre la route dans le tiroir de droite. Le tiroir est dans
l'URL - `/infra/routes/:id/:section` - donc une section se met en marque-page et un
rafraîchissement y revient.

![L'éditeur de route ouvert sur Target, avec la liste des sections à gauche](img/console/route-editor-target.webp)

L'éditeur sur *Orders API* : la liste des sections à gauche avec ses étoiles et
ses compteurs, et le panneau Target à droite - mode, amont, les deux attentes, le
disjoncteur et le contrat d'API.

Le nom est dans l'en-tête. La colonne de gauche liste les sections, groupées :

| Groupe | Sections |
|---|---|
| - | **Target** |
| Filters | Security, Predicates, Gates, Rate limits |
| Modifiers | Incoming, Outgoing |
| Forwarders | Identity, Locales |
| UI | Color scheme, User button, User info, Injections |

### Lire les marques

- **Une étoile** marque une section toujours obligatoire : Target et Predicates.
  C'est un fait sur les routes, donc elle ne disparaît jamais.
- **Le rouge** marque une section à laquelle il manque quelque chose maintenant. Il
  s'en va quand le manque est comblé.
- **Un nombre entre parenthèses** dit combien d'éléments la section porte (trois
  prédicats, deux gates).
- **La section Security** porte le niveau qu'elle pose (`AUTH`, `ORG`, `DENY`, ou un
  point quand rien n'est posé mais que des utilisateurs sont exceptés) : une route
  ouverte et une route fermée ne se lisent pas pareil dans la liste.

### Ce qui manque, et Save

Save reste désactivé jusqu'à ce que la route soit valide **et** que quelque chose
ait changé. A côté, un bouton rouge **N to fix** liste chaque manque : chaque ligne
nomme la section et y saute. Personne ne fouille onze sections à la recherche du
champ voulu.

Enregistrer garde le tiroir ouvert et applique la route aussitôt. Fermer avec des
modifications non enregistrées demande d'abord, et tant qu'il y en a, un clic à
l'extérieur ne ferme pas le tiroir.

### Les sections qui se taisent

- Les sections **UI** restent visibles mais désactivées jusqu'à ce que la case UI de
  leur groupe soit cochée. Une route est toujours un service ; l'UI vient par-dessus.
![La section Color scheme d'une route UI, avec son mécanisme, son nom de balise et la surcharge du thème stocké](img/console/route-editor-color-scheme.webp)

Une section UI une fois la case cochée : ici Color scheme, qui dit comment
l'application servie prend un choix clair ou sombre.

- **Incoming** et **Identity** sont désactivées quand la route répond d'elle-même
  (redirect, maintenance, respond). Ce n'est pas du rangement : la passerelle jette
  tous les filtres de requête sur une telle route, donc les éditer écrirait des
  réglages qu'elle jette.

## La section Target

Le mode décide de tout le reste du panneau.

| Mode | Ce que fait la route |
|---|---|
| **Proxy** | Va chercher chez un service et rend ce qu'il dit |
| **Redirect** | Envoie le navigateur ailleurs |
| **Maintenance** | Sert la page d'indisponibilité intégrée |
| **Respond** | Construit une réponse depuis un gabarit, sans rien appeler |

Sur une route proxy, on règle aussi :

- **Upstream** - le schéma se choisit, il ne se tape pas : `http` d'abord, parce que
  dans un cluster TLS s'arrête à la passerelle, `https` pour un tiers, `h2c` pour un
  service gRPC. Les services découverts par la passerelle sont proposés dans le
  champ.
- **When the service is slow or down** - les délais de connexion et de première
  réponse, qui héritent des valeurs Global sauf réglage ici. Passé l'un des deux,
  l'appelant reçoit un 502. Ce qui suit la première ligne n'est jamais borné : un
  téléchargement ou un websocket dure ce qu'il faut.
- **Stop calling this service when it stops answering** - le disjoncteur : après N
  échecs d'affilée, les appelants reçoivent tout de suite la page d'indisponibilité,
  et après le délai le service est rencontré par une seule requête plutôt que par
  tout ce qui s'est accumulé. Un 500 ne compte pas.
- **The API contract** - pas de spec, une spec publiée par le service (une URL
  relative à l'amont), ou une spec déposée ici en fichier. Une spec est ce qui
  débloque les [écrans d'endpoints](/#/docs/console/endpoints) et le swagger
  développeur.

![La section Predicates : un prédicat de chemin à deux motifs, un prédicat de méthode et un prédicat d'en-tête](img/console/route-editor-predicates.webp)

Les briques s'empilent dans le panneau, chacune avec son explication et son bouton
de retrait. Ici : deux motifs de chemin, quatre méthodes, et un en-tête qui
accepte une courte liste de valeurs.

## Prédicats, gates, filtres

Les sections qui portent des briques sont le vocabulaire du routage, et elles ont
leur propre référence :

- **[Prédicats](/#/docs/predicates/overview)** - les façons dont une route décide qu'une requête est pour elle.
- **[Filtres](/#/docs/filters/overview)** - les façons dont elle la transforme, et les gates qui la refusent.

![La section Incoming : un filtre strip-prefix et un filtre set-request-header, chacun avec ses flèches pour le déplacer](img/console/route-editor-filters.webp)

Les filtres s'appliquent dans l'ordre où ils sont listés, et les flèches de
chaque carte le déplacent vers le haut ou le bas.

## Erreurs fréquentes

- **Réordonner avec une recherche active.** La poignée refuse et le dit ; effacez la
  recherche d'abord.
- **Attendre qu'une route neuve soit joignable.** Une duplication naît désactivée,
  volontairement.
- **Deux routes qui matchent les mêmes chemins.** Parfaitement légal, et la raison
  d'être de l'ordre. Utilisez Routing test plutôt que de raisonner dessus.
- **Editer Incoming ou Identity sur une redirection.** Les sections sont
  désactivées ; ce que vous cherchez est probablement Outgoing, qui s'applique à
  tous les modes.
- **Chercher des règles par endpoint sans spec.** Déclarez d'abord la spec OpenAPI
  dans la section Target de la route.
