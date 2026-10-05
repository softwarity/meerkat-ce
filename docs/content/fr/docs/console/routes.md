---
title: Routes
section: La console
order: 152
summary: La table de routage, dans l'ordre où elle est lue, et le fonctionnement de l'éditeur de route.
---

# Routes

**Infra > Routes.** Toute la table de routage, dans l'ordre où la gateway la
lit, et l'éditeur dans lequel s'ouvre chaque route. C'est l'écran sur lequel vous
passerez le plus de temps.

![L'écran Routes : cinq routes dans l'ordre, avec leurs badges d'accès, ce qu'elles reconnaissent et leur upstream](img/console/routes-list.webp)

Cinq routes, dans l'ordre où elles sont lues. *Billing*, *Docs portal* et
*Inventory* portent la marque UI, *Billing* et *Orders API* exigent une session
(`AUTH`), *Inventory* répond d'elle-même, en maintenance, et *Catch-all*, sur
`path: /**`, vient en dernier : placée ailleurs, une route attrape-tout
répondrait à la place de toutes celles qui la suivent.

## La liste

Une ligne par route : son nom, ce qu'elle décide en matière d'accès, et une
colonne **Matching and target** sur deux lignes, avec ce que la route reconnaît,
puis l'endroit où elle envoie la requête. L'ordre a un sens - **la première route
qui correspond l'emporte** - et le tableau est donc ordonné, pas trié.

- **La poignée de déplacement** monte ou descend une route, et le changement est
  enregistré aussitôt. Elle ne fonctionne que sur la liste complète : dès qu'une
  recherche ou un filtre par type est actif, la poignée se désactive et explique
  pourquoi. Dans une vue filtrée, faire passer la troisième ligne au-dessus de la
  première la placerait en réalité au-dessus de la route qui occupe vraiment la
  première place.
- **Le cœur** placé devant le nom indique si la cible de la route est joignable :
  vert quand elle l'est, brisé et rouge quand elle ne l'est pas, avec la raison
  dans l'infobulle. Il n'y a pas de cœur tant que rien n'est connu, ni pour une
  route qui répond d'elle-même, ni pour une route désactivée. Une seconde marque
  signale une **route UI** (une route qui sert des pages dans un navigateur).
- **Les badges d'accès** indiquent ce que Meerkat exige lui-même : `AUTH` pour un
  utilisateur connecté, `ORG` pour un membre d'une organisation, `ORG-2` pour un
  membre de l'une des deux organisations désignées, `DENY` pour personne, un tiret
  quand l'accès est délégué. Un badge qui compte des endpoints apparaît lorsque la
  route affine l'accès opération par opération ; un clic dessus ouvre
  [Endpoint security](/docs/console/endpoints).
- **Les actions de ligne** : ouvrir la route dans le plan de données (routes UI
  uniquement), l'activer ou la désactiver, la dupliquer, la supprimer.
- **La recherche** porte sur le nom de la route, son upstream et ses motifs de
  chemin, c'est-à-dire ce dont vous vous souvenez en général. Le sélecteur voisin
  restreint la liste aux routes UI ou aux routes de service.

**Dupliquer** crée une copie dotée d'une nouvelle identité, **désactivée**, et
placée juste après l'originale. C'est la manière prévue d'essayer une variante :
un autre upstream, l'accès d'une autre organisation, à comparer côte à côte sans
rien ressaisir.

### La cible est-elle joignable ?

Le cœur répond à cette question pour chaque route qui envoie vers un service.

| Cœur | Quand |
|---|---|
| vert | le service est trouvé dans le cluster avec au moins une réplique prête, ou une cible externe accepte la connexion |
| brisé, rouge | aucune réplique n'est prête, la cible refuse la connexion ou son nom ne se résout pas, ou le disjoncteur est ouvert |
| aucun | rien n'est encore connu, la route répond d'elle-même, ou elle est désactivée |

La gateway vérifie toutes les 30 secondes, en arrière-plan : elle lit le
nombre de répliques que déclare Docker ou Swarm, et ouvre une simple connexion
TCP vers tout le reste (un service Kubernetes, un hôte externe). Aucune requête
HTTP n'est envoyée : l'upstream ne voit donc ni appel, ni authentification, ni ligne
dans ses journaux. Chaque nœud vérifie depuis l'endroit où il se trouve. Quand un
cœur change d'état, la liste se met à jour sans rechargement.

## Trois boutons dans le bandeau

- **Routing test** compose une requête fictive et vous dit quelle route la prend.
  Servez-vous-en avant de déplacer quoi que ce soit : c'est la réponse à la seule
  question que pose l'ordre.
- **Global** rassemble ce qui vaut pour toutes les routes à la fois :
  l'interrupteur **Unavailable**, qui les ferme toutes (les pages de connexion
  restent en service, et quiconque administre ou développe ici continue de
  passer), le temps qu'une route attend un service avant de répondre 502, et le
  plafond de ce qu'un filtre de réécriture du corps peut garder en mémoire. Tant
  que l'interrupteur est actif, le bouton lui-même vire à l'ambre et affiche
  *Unavailable* : personne n'a besoin de l'ouvrir pour le savoir.
- **JWT** rassemble les clés qui signent le JWT d'identité que vérifient vos
  services : le JWKS à fournir à un backend, la partie publique de chaque
  algorithme, la rotation, et quelles routes signent avec quelle clé.
  Un service qui pointe sur le JWKS n'a besoin de rien d'autre : le `kid` du jeton
  désigne la clé, et la clé porte son algorithme. L'algorithme se choisit donc une
  seule fois, dans Auth forward. Le service le lit sur la clé, jamais dans
  l'en-tête du jeton.

## L'éditeur

Un clic sur une ligne ouvre la route dans le tiroir de droite. Le tiroir figure
dans l'URL - `/infra/routes/:id/:section` - si bien qu'une section peut être mise
en favori et qu'un rechargement y ramène.

![L'éditeur de route ouvert sur Target, avec la liste des sections à gauche](img/console/route-editor-target.webp)

L'éditeur sur *Orders API* : à gauche, la liste des sections avec ses étoiles et
ses compteurs ; à droite, le panneau Target, avec le mode, l'upstream, les deux
timeouts, le disjoncteur et le contrat d'API.

Le nom se trouve dans l'en-tête. La colonne de gauche liste les sections,
regroupées ainsi :

| Groupe | Sections |
|---|---|
| - | **Target**, **Identity**, **OpenTelemetry** |
| Filters | Security, Predicates, Gates, Rate limits |
| Modifiers | Incoming, Outgoing, Auth forward |
| UI | Locales, Color scheme, User button, User info, Custom |

**Identity** contient ce que la route sait de l'appelant. Deux sections
l'exploitent : **Auth forward** le transmet au service, **User info** l'affiche
sur la page. **OpenTelemetry** décide si la route est tracée (voir
[les traces](/docs/operations/tracing#choisir-les-routes-traces)).

### Lire les marques

- **Une étoile** signale une section toujours obligatoire : Target et Predicates.
  C'est une propriété des routes, et elle ne disparaît donc jamais.
- **Le rouge** signale une section où il manque quelque chose en ce moment. Il
  disparaît une fois le manque comblé.
- **Un nombre entre parenthèses** donne le nombre d'éléments que contient la
  section (trois prédicats, deux gates).
- **La section Security** affiche le niveau qu'elle impose (`AUTH`, `ORG`, `DENY`,
  ou un point quand elle n'impose rien mais que des utilisateurs font exception) :
  une route ouverte et une route fermée ne se présentent ainsi pas de la même
  façon dans la liste.

### Ce qui manque, et Save

Save ne s'active que lorsque la route est valide **et** que quelque chose a
changé. À côté, un bouton rouge **N to fix** dresse la liste des manques : chaque
ligne nomme la section concernée et vous y conduit. Inutile de parcourir quinze
sections à la recherche du champ à compléter.

L'enregistrement laisse le tiroir ouvert et applique la route aussitôt. Si vous
fermez le tiroir avec des modifications non enregistrées, une confirmation vous
est demandée ; tant qu'il en reste, un clic en dehors du tiroir ne le ferme pas.

### Les sections qui se désactivent

- Les sections **UI** restent visibles mais désactivées tant que la case UI de
  leur groupe n'est pas cochée. Une route est toujours un service ; l'UI vient
  s'y ajouter.

![La section Color scheme d'une route UI, avec son mécanisme, son nom de balise et le remplacement du thème mémorisé](img/console/route-editor-color-scheme.webp)

Une section UI une fois la case cochée : ici Color scheme, qui indique comment
l'application servie reçoit le choix d'un thème clair ou sombre.

- **Incoming**, **Identity** et **Auth forward** sont désactivées quand la route
  répond d'elle-même (redirect, maintenance, respond). Ce n'est pas une question
  de présentation : sur une telle route, la gateway écarte tous les filtres de
  requête, et les modifier reviendrait à écrire des réglages qu'elle ignore.

## La section Target

Le mode détermine tout le reste du panneau.

| Mode | Ce que fait la route |
|---|---|
| **Proxy** | Interroge un service et renvoie sa réponse |
| **Redirect** | Envoie le navigateur ailleurs |
| **Maintenance** | Sert la page d'indisponibilité intégrée |
| **Respond** | Construit une réponse à partir d'un gabarit, sans rien appeler |

Sur une route en mode proxy, vous réglez aussi :

- **Upstream** - le schéma se choisit, il ne se saisit jamais : `http` en
  premier, parce qu'à l'intérieur d'un cluster TLS se termine à la gateway,
  `https` pour un tiers, `h2c` pour un service gRPC. Le champ propose les services
  que la gateway a découverts.
- **When the service is slow or down** - les timeouts de connexion et de première
  réponse, qui reprennent les valeurs de Global tant qu'ils ne sont pas réglés
  ici. Passé l'un ou l'autre, l'appelant reçoit une réponse 502. Ce qui suit la
  première ligne n'est jamais borné : un téléchargement ou un websocket dure
  aussi longtemps que nécessaire.
- **Stop calling this service when it stops answering** - le disjoncteur : après
  N échecs consécutifs, les appelants reçoivent immédiatement la page
  d'indisponibilité ; une fois le délai écoulé, le service reçoit une seule
  requête d'essai, et non tout ce qui s'était accumulé. Une réponse 500 n'entre
  pas dans le décompte.
- **The API contract** - aucune spécification, une spécification publiée par le
  service (une URL relative à l'upstream), ou une spécification déposée ici sous
  forme de fichier. C'est elle qui donne accès aux
  [écrans des endpoints](/docs/console/endpoints) et au swagger développeur.

![La section Predicates : un prédicat de chemin, un prédicat de méthode et un prédicat d'en-tête qui accepte deux valeurs](img/console/route-editor-predicates.webp)

Les briques s'empilent dans un panneau, chacune avec sa propre explication et son
propre bouton de suppression. Ici : deux motifs de chemin, quatre méthodes, et un
en-tête qui accepte une courte liste de valeurs.

## Prédicats, gates, filtres

Les sections qui contiennent des briques forment le vocabulaire du routage, et
elles ont leur propre référence :

- **[Prédicats](/docs/predicates/overview)** - les façons dont une route décide qu'une requête lui est destinée.
- **[Filtres](/docs/filters/overview)** - les façons dont elle la transforme, et les gates qui la refusent.

![La section Incoming : un filtre strip-prefix et deux filtres set-request-header, dont l'un lit une entrée du coffre, chacun avec des flèches pour le réordonner](img/console/route-editor-filters.webp)

Les filtres s'appliquent dans l'ordre de la liste, et les flèches de chaque carte
la font monter ou descendre.

## Erreurs fréquentes

- **Réordonner alors qu'une recherche est active.** La poignée refuse et le dit ;
  effacez d'abord la recherche.
- **S'attendre à ce qu'une nouvelle route soit joignable.** Une copie est créée
  désactivée, et c'est voulu.
- **Deux routes qui reconnaissent les mêmes chemins.** C'est parfaitement permis,
  et c'est la raison d'être de l'ordre. Utilisez Routing test plutôt que de
  raisonner de tête.
- **Modifier Incoming, Identity ou Auth forward sur une redirection.** Ces
  sections sont désactivées ; ce que vous cherchez est probablement Outgoing, qui
  s'applique à tous les modes.
- **Chercher des règles par endpoint sans spécification.** Déclarez d'abord la
  spécification OpenAPI dans la section Target de la route.
