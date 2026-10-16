---
title: Brancher une application
section: Authentification
order: 119
summary: Des recettes pour qu'une application derrière Meerkat sache qui l'utilise - Grafana et GeoServer par en-têtes, RabbitMQ et DbGate par le fournisseur OpenID de Meerkat - avec les réglages des deux côtés et les pièges.
---

# Brancher une application

::: lead
Mettre une application derrière une route la protège. Lui faire **savoir** qui
l'utilise - son nom, ses droits - est la seconde moitié, et chaque application la
prend à sa façon. Ces recettes en couvrent quelques-unes, courantes, une par
famille : lisez celle qui ressemble le plus à la vôtre.
:::

Deux familles couvrent presque tout :

- **Les en-têtes.** L'application fait confiance à un en-tête d'authentification
  posé par son proxy. Rien à déclarer d'autre que la route : dans **Identity**,
  choisissez les faits et donnez à chacun le nom d'en-tête que lit l'application ;
  dans **Auth forward**, mécanisme **Headers**. Meerkat retire toute copie de ces
  en-têtes envoyée par l'appelant : à travers la gateway, ils ne se forgent pas.
- **OpenID Connect.** L'application connecte elle-même ses utilisateurs, auprès d'un
  fournisseur. Meerkat est ce fournisseur (édition Enterprise) : déclarez un client,
  liez-y la route. Voir [Meerkat fournisseur OpenID](/docs/auth/oidc-provider).

Les exemples ci-dessous prennent `https://apps.example.com` pour l'adresse publique
de la gateway, une route par application sous son propre chemin, et `meerkat` pour
le nom de la gateway dans le cluster (le tiroir OIDC / JWT écrit le vrai).

> [!WARNING]
> Les deux familles reposent sur une promesse : l'application n'est joignable
> **que par la gateway**. Un en-tête est cru parce que personne d'autre ne peut le
> poser ; une application joignable aussi en direct - un NodePort, un ingress à
> elle, un port publié - croit quiconque l'atteint par là. Gardez son Service
> interne (ClusterIP), ne mettez rien d'autre devant, et quand la plateforme le
> permet, ajoutez une politique réseau qui n'admet que la gateway.

## Grafana, par en-têtes

**Mécanisme** : en-têtes, lus par l'*auth proxy* de Grafana.

**Côté Grafana** (variables d'environnement ; les mêmes clés existent dans
`grafana.ini` sous `[auth.proxy]` et `[server]`) :

```ini
GF_SERVER_ROOT_URL=https://apps.example.com/grafana/
GF_SERVER_SERVE_FROM_SUB_PATH=true
GF_AUTH_PROXY_ENABLED=true
GF_AUTH_PROXY_HEADER_NAME=X-WEBAUTH-USER
GF_AUTH_PROXY_HEADER_PROPERTY=username
GF_AUTH_PROXY_AUTO_SIGN_UP=true
GF_AUTH_PROXY_HEADERS=Email:X-WEBAUTH-EMAIL Name:X-WEBAUTH-NAME Role:X-WEBAUTH-ROLE
GF_AUTH_PROXY_WHITELIST=<les adresses depuis lesquelles appelle la gateway>
GF_AUTH_SIGNOUT_REDIRECT_URL=https://apps.example.com/logout
```

**Côté Meerkat** :

1. Une route sur `/grafana/**` vers le service Grafana, chemin gardé tel quel
   (Grafana sert depuis le sous-chemin).
2. **Identity** : `username` sous `X-WEBAUTH-USER`, `email` sous `X-WEBAUTH-EMAIL`,
   `fullname` sous `X-WEBAUTH-NAME`.
3. **Auth forward** : mécanisme **Headers**.
4. Le rôle. Grafana attend `Viewer`, `Editor` ou `Admin`. Le plus simple : deux
   routes sur le même chemin, la première avec une règle d'accès sur le rôle de vos
   administrateurs Grafana et un filtre Incoming
   [set-request-header](/docs/filters/set-request-header)
   `X-WEBAUTH-ROLE: Admin`, la seconde pour tous les autres avec `Viewer`. La
   première route qui laisse entrer une personne la sert.

**Pièges** :

- Sans `GF_AUTH_PROXY_WHITELIST`, Grafana prend l'en-tête de quiconque l'atteint.
  Réglez-le sur les adresses de la gateway, et gardez quand même Grafana hors de
  tout autre chemin.
- `GF_SERVER_ROOT_URL` doit être l'adresse publique avec le chemin de la route,
  sinon Grafana écrit des liens et des redirections qui sortent de la route.
- Le rôle s'applique quand Grafana synchronise l'utilisateur (toutes les `sync_ttl`
  minutes, 15 par défaut) : un changement de rôle à la gateway atteint Grafana dans
  ce délai.

## GeoServer, par en-têtes

**Mécanisme** : en-têtes, lus par le filtre d'authentification *HTTP Header* de
GeoServer.

**Côté GeoServer** (administration web, **Security > Authentication**) :

1. **Authentication Filters**, **Add new**, **HTTP Header**. Nommez-le (`gateway`)
   et réglez **Request header attribute** sur un nom que personne ne devinerait,
   comme `X-Gw-Auth-7f3c9a` - la documentation de GeoServer en parle comme d'un
   secret partagé entre le proxy et GeoServer.
2. **Role source** : soit *User group service* (les utilisateurs et rôles propres à
   GeoServer, retrouvés par le nom), soit *Request header* avec un second en-tête
   qui porte les rôles, séparés par `;`.
3. **Filter Chains** : dans chaque chaîne qu'utilisent le navigateur et les clients
   (`web`, `rest`, `gwc`, `default`), ajoutez le nouveau filtre avant `anonymous`,
   et retirez `basic` et `form` là où l'on ne doit arriver que par la gateway.
4. **Settings > Global**, **Proxy Base URL** : `https://apps.example.com/geoserver`,
   pour que les documents de capacités nomment l'adresse publique.

**Côté Meerkat** :

1. Une route sur `/geoserver/**` vers le service GeoServer.
2. **Identity** : `username` sous `X-Gw-Auth-7f3c9a` (le même nom). Avec les rôles
   par en-tête, `roles` sous l'en-tête des rôles, mis à la forme de GeoServer, par
   exemple `{{.Roles | .Keep "tag:GEOSERVER" | join ";"}}` pour les rôles que vous
   avez étiquetés pour lui, nommés comme GeoServer les nomme
   (`ROLE_ADMINISTRATOR`...).
3. **Auth forward** : mécanisme **Headers**.

**Pièges** :

- **Ne laissez jamais de chemin qui contourne la gateway.** GeoServer croit
  l'en-tête, et son Service, un vieil ingress ou un port publié répondent à
  quiconque connaît le nom. Le nom obscur est un second verrou, pas le premier.
- Le nom de l'en-tête est un secret : gardez-le hors des tickets et des captures,
  et changez-le des deux côtés s'il fuit.
- Retirez le mot de passe du compte `admin` par défaut, ou désactivez-le, une fois
  l'en-tête en place : la connexion par formulaire est le contournement de la
  gateway à l'intérieur de l'application.

## L'administration de RabbitMQ, par le fournisseur OpenID

**Mécanisme** : OIDC (Enterprise). L'interface d'administration se connecte par
Meerkat avec PKCE, puis appelle l'API d'administration avec son access token, par la
même route.

**Côté Meerkat** :

1. L'**émetteur** : le nom public du plan Application, déclaré dans Domains &
   TLS, onglet Application - ici `apps.example.com`, donc
   `https://apps.example.com`.
2. Un client `rabbitmq-ui`, **sans secret** (l'interface est du code navigateur),
   et **Audience** `rabbitmq` (le `resource_server_id` de RabbitMQ). Un client ne
   déclare pas de redirect URI : la page de retour de l'interface est sous le
   chemin de la route, c'est ce que le fournisseur accepte.
3. Une route sur `/rabbitmq/**` vers le port d'administration (15672), mécanisme
   **OIDC**, client `rabbitmq-ui`, claims `preferred_username` depuis `username` et
   un claim `roles`. Deux niveaux sont deux routes sur le même chemin et le même
   client : la première pour vos administrateurs RabbitMQ avec `roles` fixé à
   `admin`, la seconde pour les autres avec `roles` fixé à `viewer`.

**Côté RabbitMQ** (`rabbitmq.conf`) :

```ini
auth_backends.1 = rabbit_auth_backend_oauth2
# pendant la bascule seulement, pour que les comptes existants marchent encore :
auth_backends.2 = rabbit_auth_backend_internal

auth_oauth2.resource_server_id = rabbitmq
auth_oauth2.issuer = https://apps.example.com
auth_oauth2.preferred_username_claims.1 = preferred_username
auth_oauth2.additional_scopes_key = roles
auth_oauth2.scope_aliases.admin = rabbitmq.tag:administrator rabbitmq.read:*/* rabbitmq.write:*/* rabbitmq.configure:*/*
auth_oauth2.scope_aliases.viewer = rabbitmq.tag:monitoring rabbitmq.read:*/*

management.path_prefix = /rabbitmq
management.oauth_enabled = true
management.oauth_client_id = rabbitmq-ui
management.oauth_scopes = openid profile
# pendant la bascule seulement : garde le formulaire nom et mot de passe
management.oauth_disable_basic_auth = false
```

**Pièges** :

- **Le chemin de retour.** L'interface revient sur sa propre page de retour,
  `/rabbitmq/js/oidc-oauth/login-callback.html`, sous le chemin de la route :
  acceptée parce que la route est liée au client. Si Meerkat la refuse, le journal de la gateway nomme
  l'adresse envoyée par l'interface (`redirect_uri`) à côté des endroits qu'il
  accepte : une adresse hors de `/rabbitmq` veut dire que l'interface ne sait pas où
  elle est publiée - vérifiez `management.path_prefix`.
- **Les clés se lisent en https.** RabbitMQ refuse une adresse http pour les clés.
  Il les trouve à partir de l'émetteur : le broker doit donc atteindre l'adresse
  publique et faire confiance à son certificat. Pour rester dans le cluster,
  donnez-lui le plan interne, mais **seulement par son port TLS** (un certificat
  placé sur le plan interne) :
  `auth_oauth2.jwks_uri = https://meerkat:7443/meerkat/oidc/jwks` et
  `auth_oauth2.https.cacertfile` = l'autorité qui a signé ce certificat.
- **Pas de secret.** Déclarez le client sans : l'interface est du code public et
  n'en envoie aucun, et un secret envoyé par un client déclaré sans est refusé.
- **Les droits viennent de `roles`, pas de `scope`.** Les jetons de Meerkat ne
  portent que les scopes qu'il connaît (`openid profile email`) ; les permissions
  de RabbitMQ arrivent par `additional_scopes_key` et ses alias. L'audience doit
  être le `resource_server_id`, sinon tous les jetons sont refusés.
- Une fois tout le monde passé par Meerkat, retirez `auth_backends.2` et la ligne
  `oauth_disable_basic_auth = false` : jusque-là, les anciens mots de passe ouvrent
  encore le broker.
- La déconnexion : le bouton de RabbitMQ termine sa propre session ; envoyez les
  gens vers `https://apps.example.com/logout` pour terminer celle de la gateway.

## DbGate, par le fournisseur OpenID

**Mécanisme** : OIDC (Enterprise). C'est le serveur de DbGate qui mène la connexion
et qui garde un secret.

**Côté Meerkat** :

1. L'**émetteur** : le nom public `apps.example.com`, donc
   `https://apps.example.com`.
2. Un client `dbgate`, **avec un secret** (copiez-le quand il s'affiche). DbGate
   revient sous `/dbgate/`, le chemin de la route : rien à déclarer.
3. Une route sur `/dbgate/**` vers DbGate, mécanisme **OIDC**, client `dbgate`,
   claim `preferred_username` depuis `username` (et `email`, `name` si vous le
   souhaitez).

**Côté DbGate** (variables d'environnement) :

```ini
WEB_ROOT=/dbgate
OAUTH_AUTH=https://apps.example.com/meerkat/oidc/authorize
OAUTH_TOKEN=http://meerkat:7070/meerkat/oidc/token
OAUTH_LOGOUT=https://apps.example.com/logout
OAUTH_CLIENT_ID=dbgate
OAUTH_CLIENT_SECRET=mko_...
OAUTH_SCOPE=openid profile
OAUTH_LOGIN_FIELD=preferred_username
LOGIN_PERMISSIONS_alice=*
LOGIN_PERMISSIONS_bob=~*, widgets/database
```

**Pièges** :

- `OAUTH_AUTH` est l'adresse où va le **navigateur** : toujours l'adresse publique.
  `OAUTH_TOKEN` est appelé par le **serveur** de DbGate : le plan interne est le
  chemin court (ou l'endpoint token public, si DbGate l'atteint).
- DbGate revient sous `WEB_ROOT`. Si Meerkat refuse la connexion, le journal de la
  gateway nomme l'adresse envoyée par DbGate : une adresse hors de `/dbgate` veut
  dire que `WEB_ROOT` n'est pas le chemin de la route.
- `BASIC_AUTH` ne fonctionne pas avec OAuth : ne le réglez pas.
- Les permissions sont par login, pris dans le claim que nomme `OAUTH_LOGIN_FIELD` :
  `LOGIN_PERMISSIONS_<login>` pour chaque personne, `PERMISSIONS` pour tout le
  monde. Un claim `preferred_username` à **valeur fixe** sur la route fait de tout le
  monde un seul compte DbGate partagé ; deux routes avec deux valeurs fixes donnent
  deux comptes partagés, un par niveau.
- Sans `OAUTH_LOGIN_FIELD`, DbGate donne le même login à tous les utilisateurs.

## Pour aller plus loin

- [Meerkat fournisseur OpenID](/docs/auth/oidc-provider) - clients, claims,
  adresses, déconnexion.
- [Le JWT d'identité](/docs/auth/service-jwt) - la troisième voie : un jeton signé
  à chaque requête.
- [La règle d'accès d'une route](/docs/access/route-access) - les règles qui
  décident qui entre.
