---
title: Meerkat fournisseur OpenID
section: Authentification
order: 118
summary: Laisser les applications servies derrière vos routes connecter leurs utilisateurs par Meerkat, comme avec n'importe quel fournisseur OpenID Connect - le client, la route, les claims, deux niveaux d'accès, les adresses et la déconnexion.
---

# Meerkat fournisseur OpenID

::: lead
Certaines applications ne connaissent qu'une façon d'apprendre qui les utilise :
envoyer la personne chez un fournisseur OpenID Connect et lire les jetons qu'il
leur rend. Meerkat peut être ce fournisseur pour les applications servies derrière
ses routes. La personne est déjà connectée à la gateway, elle passe donc sans voir
de page, et l'application reçoit des jetons standard qui disent qui elle est.
Édition Enterprise.
:::

## Quand s'en servir

Une route a trois autres façons de dire à son service qui appelle, et elles sont
plus simples : rien à régler dans l'application, sinon faire confiance à la gateway.

| Mécanisme | Ce que reçoit le service | À choisir quand |
|---|---|---|
| **Headers** | un en-tête par fait, à chaque requête | l'application a un mode « faire confiance au proxy » (un en-tête d'authentification) |
| **Signed JWT** | un jeton signé court, à chaque requête | le service vérifie un jeton bearer avec un JWKS ([Le JWT d'identité](/docs/auth/service-jwt)) |
| **OIDC** | rien sur la requête : l'application connecte elle-même, par Meerkat | l'application n'accepte qu'un fournisseur OpenID Connect, ou son code navigateur appelle son API avec un access token |

Des recettes pour des applications courantes, une par mécanisme, sont dans
[Brancher une application](/docs/auth/connect-an-application).

## Le déroulé d'une connexion

1. La personne ouvre l'application par sa route. La gateway la connecte comme
   toujours : l'application n'y est pour rien.
2. L'application envoie le navigateur vers l'endpoint **authorize** de Meerkat,
   avec son identifiant de client.
3. Meerkat prend les routes liées à ce client, dans l'ordre du routage, et la
   première dont la règle d'accès laisse entrer la personne décide. Il renvoie un
   code à l'application, à l'adresse qu'elle a demandée, pourvu que ses routes la
   servent. Pas de page, pas d'écran de consentement.
4. L'application échange le code contre un **ID token**, un **access token** et un
   **refresh token** : depuis son serveur avec son secret, ou depuis son code
   navigateur avec PKCE.
5. À chaque **refresh**, la route est choisie de nouveau, avec ce qu'est la
   personne à ce moment-là. Un rôle retiré, une organisation quittée, une route
   désactivée : chacun s'applique au refresh suivant, pas à la prochaine
   déconnexion.

Il n'y a pas d'écran de consentement parce que les clients sont déclarés par un
administrateur : celui qui a déclaré l'application a déjà décidé qu'elle peut
savoir qui s'y connecte.

## Avant tout : une adresse publique

Une application compare l'émetteur de ses jetons à celui que lui a donné la
découverte de son fournisseur : les deux doivent donc être une seule adresse, celle
qu'atteignent ses utilisateurs. L'émetteur ne se tape pas : c'est le **nom public**
du plan Application, déclaré dans **Domains & TLS**, onglet **Application**
([Domaines et TLS](/docs/operations/tls)), écrit `https://<nom>` - qui que ce soit
qui le serve, la gateway ou un proxy devant. Un nom de cette machine (`localhost`)
donne `http://localhost:<port du plan de données>`, pour la gateway d'un
développeur.

- **Un nom public** : c'est l'émetteur.
- **Plusieurs** : le tiroir **OIDC / JWT** les propose dans **Public address** ;
  tant qu'aucun n'est choisi, le premier. Un nom retiré emporte le choix, et
  l'émetteur passe au premier qui reste.
- **Aucun** : les jetons disent `meerkat`, le fournisseur reste éteint - chaque
  endpoint répond par la phrase qui dit où déclarer un nom - et le tiroir mène à
  l'onglet Application.

> [!WARNING]
> L'émetteur est aussi le `iss` du JWT d'identité que les routes aux mécanismes
> **JWT** et **Signed JWT** envoient à leurs services. Déclarer, retirer ou choisir
> un nom public change aussi leur prochain jeton : mettez à jour en même temps ce
> qu'attendent ces services.

## Déclarer le client

**Infra > Routes**, le bouton **OIDC / JWT** à côté de **Global**. Sous les clés,
**OIDC clients**, puis **New client** :

| Champ | Ce qu'il décide |
|---|---|
| **Client id** | ce que l'application appelle son `client_id` : lettres minuscules, chiffres, `.`, `_` et `-`, 63 caractères au plus. Figé une fois le client créé |
| **Client secret** | d'où vient le secret, un choix parmi trois (ci-dessous) |
| **Audience** | le `aud` des access tokens : le nom que l'application s'attend à y trouver. Vide, l'identifiant du client. Un texte de 255 caractères au plus, sans caractère de contrôle ; les espaces aux deux bouts sont retirés. Le `aud` de l'ID token est toujours l'identifiant du client |
| **Token lifetime** | la durée de vie des ID tokens et des access tokens : 5 minutes par défaut, de 1 minute à 24 heures. Un refresh les renouvelle |

Un client ne déclare aucune adresse de retour : les routes qui lui sont liées
disent où vit l'application ([L'adresse de retour](#ladresse-de-retour)).

![Un nouveau client, son secret tiré par Meerkat dans le formulaire](img/console/oidc-client.webp)

### Le secret du client

Un seul champ sous les trois choix, quel que soit le choix :

| Choix | Le champ | Quand |
|---|---|---|
| **No secret** | désactivé | du code qui tourne dans un navigateur ou sur un téléphone, qui ne peut rien cacher : il se prouve par **PKCE** (S256), qui devient obligatoire |
| **Drawn by Meerkat** | un secret qui commence par `mko_`, tiré dans le formulaire, avec un bouton de copie et un bouton pour en tirer un autre | l'application prend le secret qu'on lui donne : copiez-le dans sa configuration avant d'enregistrer |
| **Given by the application** | vide, à remplir | l'application impose le sien : son chart le génère, ou son éditeur l'a figé. De 16 à 256 caractères, sans espace |

Dans tous les cas, la gateway n'en garde que le sha256 et ne le montre plus jamais :
la liste en affiche le début, `mko_...` pour un secret tiré, *application's secret*
et quatre caractères pour l'autre. La longueur minimale est là parce que le sha256
voyage dans une configuration exportée, et qu'un secret court se retrouve à partir
de lui.

Modifier un client affiche le début de son secret et le garde tel quel. Tirez-en un
autre ou collez-en un autre et l'indication le dit : **le secret actuel cesse de
fonctionner à l'enregistrement**, et l'application ne connecte plus personne tant
qu'elle n'a pas le nouveau. Annuler le laisse fonctionner.

![Les clients sous les clés : un secret tiré par Meerkat, celui de l'application, et aucun](img/console/oidc-clients.webp)

Chaque ligne de la liste nomme les routes qui utilisent le client ; un clic en
ouvre une sur sa section **Auth forward**. Une marque d'avertissement signale
quand l'une d'elles couvre tout un hôte. L'icône de clé ouvre le client sur un **nouveau
secret** tiré par Meerkat, en vigueur une fois enregistré. Un client qu'une route utilise
encore ne se supprime pas, et le refus nomme les routes.

Les clients voyagent dans une [configuration](/docs/console/configuration), le secret
sous forme de son sha256 seulement (`secretHash`) : importé sur une autre gateway, un
client garde le même secret. Un client confidentiel importé sans lui n'a **pas encore
de secret**, et sa ligne le dit : l'icône de clé tire le premier.

## Y lier une route

Dans l'éditeur de route, **Auth forward**, mécanisme **OIDC (the service signs in
through Meerkat)**. Choisissez le **Client** - ou créez-le sur place avec **New
client** - et remplissez **Claims sent** : ce que les jetons disent de la personne.
Chaque claim est un nom et une source :

- **un fait** que déclare la section **Identity** de la route : le nom
  d'utilisateur, l'e-mail, le nom complet, l'organisation, les rôles... rendu
  comme le mécanisme JWT le rend, les rôles par l'expression que leur donne
  Identity ;
- **une valeur fixe**, la même pour tous ceux qui se connectent par cette route.

![Une route liée à un client, les claims que portent ses jetons et ce que chacun dit pour la personne qui l'édite](img/console/route-editor-oidc.webp)

Une nouvelle configuration part des noms que toute bibliothèque OpenID lit
d'abord, chacun gardé seulement si Identity déclare son fait : `sub` (l'identifiant
du compte), `preferred_username`, `email`, `name`. Un fait que la personne n'a pas
est omis plutôt qu'envoyé vide.

Le fournisseur écrit certains claims lui-même, et ils sont refusés par leur nom :
`iss`, `aud`, `exp`, `iat`, `nbf`, `nonce`, `azp`, `auth_time`, `at_hash`, `jti`,
`client_id`, `scope`. `sub` fait exception : il vaut par défaut l'identifiant du
compte et peut être fixé, par exemple à une valeur constante quand une application
doit voir un seul compte partagé.

La route porte toujours le trafic de l'application, comme toute route. En OIDC,
elle n'ajoute rien aux requêtes et laisse `Authorization` intact : c'est là que le
code navigateur de l'application envoie son access token.

## L'adresse de retour

À chaque connexion, l'application donne l'adresse où elle veut retrouver la
personne (sa `redirect_uri`), et le code n'y part que si cette adresse est bien la
sienne. L'application est servie par les routes liées à son client : ces routes
disent où elle vit, et rien d'autre n'est accepté. Une adresse est acceptée
quand, pour **une même** route active liée au client,

- son schéma et son hôte sont l'**adresse publique** de la gateway (celle de
  l'émetteur), ou un nom du prédicat **host** de la route (https, ou le schéma de
  l'émetteur), et
- son chemin est sous le début littéral d'un des motifs **path** de la route,
  segment par segment : `/rabbitmq/**` accepte `/rabbitmq/` et tout ce qui est
  dessous, jamais `/rabbitmqx`. Un motif s'arrête à sa première variable :
  `/api/{tenant}/**` accepte ce qui est sous `/api`.

L'hôte d'une route ne se marie jamais au chemin d'une autre, et une route
désactivée n'accepte rien. Une query string est conservée ; un fragment, ou un
segment `..` qui sortirait du chemin, est refusé. Un agent lit ces endroits dans
`list_oidc_clients` (`acceptedUnder`), par exemple
`https://apps.example.com/rabbitmq/`.

> [!WARNING]
> Une route sur `/**` couvre **tout son hôte** : n'importe quelle adresse de cet
> hôte peut alors recevoir un code pour le client, et la liste des clients le
> signale. Donnez à une telle route un hôte à elle, ou un chemin plus étroit.

Une application servie hors de la gateway, ou sous un nom qu'aucune route de son
client ne donne (un prédicat host à joker, `*.example.com`, ne nomme aucune
adresse), ne peut pas utiliser le fournisseur : mettez-la derrière une route.

Le code navigateur de l'application peut appeler les endpoints token et userinfo
depuis les mêmes origines : l'adresse publique de la gateway et les hôtes des
routes.

## Qui peut se connecter : la règle d'accès de la route

Le client ne dit rien des personnes. Qui peut se connecter à l'application, c'est
la **règle d'accès** de la route qui lui est liée, posée dans **Security** comme
toute autre. Une personne qu'elle refuse reçoit la réponse de la gateway elle-même :
le choix d'organisation quand une autre organisation la laisserait entrer, sinon la
page de refus qui nomme la règle.

### Deux niveaux d'accès

Deux niveaux pour une application - par exemple les administrateurs et les
lecteurs - sont **deux routes sur les mêmes chemins avec le même client**, chacune
avec sa règle d'accès et ses claims :

| Ordre | Route | Règle d'accès | Claims envoyés |
|---|---|---|---|
| 1 | *Supervision - admins* | rôle `MONITORING_ADMIN` | `roles` = `admin` |
| 2 | *Supervision - lecteurs* | connecté | `roles` = `viewer` |

La **première** route qui accorde la personne décide, exactement comme le routage
le fait pour les requêtes : un administrateur correspond à la première, tous les
autres retombent sur la seconde. Le choix étant refait à chaque refresh, perdre le
rôle fait des jetons suivants ceux d'un lecteur.

## Les adresses

On ne donne qu'une chose à l'application : l'**émetteur**. Tout le reste est dans
le document de découverte. Le haut du tiroir **OIDC / JWT** donne les trois
adresses bien connues à copier : **Discovery (internal)**, **OIDC discovery
(public)** - affichée tant que le fournisseur répond - et **JWKS**.

| | Adresse publique (plan de données) | Plan interne |
|---|---|---|
| Découverte | `https://apps.example.com/.well-known/openid-configuration` | `http://meerkat:7070/.well-known/openid-configuration` |
| Authorize | `https://apps.example.com/meerkat/oidc/authorize` | - (la porte d'un navigateur, jamais sur le plan interne) |
| Token | `https://apps.example.com/meerkat/oidc/token` | `http://meerkat:7070/meerkat/oidc/token` |
| Userinfo | `https://apps.example.com/meerkat/oidc/userinfo` | `http://meerkat:7070/meerkat/oidc/userinfo` |
| Clés (JWKS) | `https://apps.example.com/meerkat/oidc/jwks` | `http://meerkat:7070/meerkat/oidc/jwks` |
| Déconnexion | `https://apps.example.com/logout` | - |

- Le document de découverte **public** ne répond que tant qu'au moins un client
  est déclaré. Avant, le chemin appartient aux routes comme n'importe quel autre :
  une application derrière une route peut servir le sien.
- Le **plan interne** est pour le **serveur** de l'application, dans le cluster :
  il y échange ses codes et y lit les clés sans ressortir par le nom public. Son
  document de découverte nomme l'endpoint authorize public - c'est le navigateur de
  la personne qui y va - et les autres à l'adresse qu'a utilisée le serveur. Dès
  qu'un certificat est placé sur le plan interne, la même chose est servie en HTTPS
  sur **7443** ([TLS](/docs/operations/tls#un-certificat-depuis-le-systme-de-fichiers)).
  `meerkat` désigne le nom de la gateway dans le cluster ; le tiroir OIDC / JWT
  écrit le vrai.
- Une bibliothèque qui part du seul émetteur lit la découverte **publique** : le
  serveur doit alors atteindre l'adresse publique. Une bibliothèque qui prend
  l'adresse du document peut recevoir celle du plan interne.

Pris en charge : `response_type=code`, PKCE `S256`, `prompt=none`, les grants
`authorization_code` et `refresh_token`, l'authentification du client par
`client_secret_basic`, `client_secret_post` ou aucune, les scopes `openid`
(obligatoire), `profile` et `email`, des ID tokens signés en **RS256** avec les clés
de la gateway. Les autres scopes sont ignorés plutôt que refusés ; les claims
viennent de la route, quels que soient les scopes. Userinfo répond les claims de
l'access token qui concernent la personne.

## La déconnexion

Une seule session compte, celle de la gateway, et une seule façon d'y mettre fin :
le **`/logout`** du plan de données, que le document de découverte donne comme
`end_session_endpoint`. Faites pointer la déconnexion de l'application vers
`https://apps.example.com/logout`.

- Depuis une page du même site - l'application servie par la gateway -, un
  `GET /logout` déconnecte aussitôt et mène à la page de connexion.
- Depuis n'importe où ailleurs, ou quand un navigateur la précharge avant un clic,
  une page demande d'abord, avec un bouton **Sign out**.
- Aucune adresse de retour n'est lue dans la requête : la personne arrive toujours
  sur la page de connexion.

Mettre fin à la session de la gateway révoque les codes et les refresh tokens nés
dans cette session. Un access token déjà émis reste valable jusqu'à son expiration,
d'où la durée courte par défaut. Une application qui ne termine que sa propre
session laisse la personne connectée à la gateway, et sa prochaine connexion passe
sans page.

## Essayer une connexion

L'icône de flèche sur la ligne d'un client essaie une connexion pour n'importe qui,
sans lui et sans rien émettre. Choisissez la personne, et l'organisation dans laquelle
sa session serait ouverte (la seule qu'elle ait, par défaut) :

- la route qui la laisse entrer - la première liée au client, dans l'ordre de
  routage, dont la règle le fait - ou un refus quand aucune ne le fait ;
- les routes dont la règle l'a écartée ;
- les claims de l'**ID token**, de l'**access token** et de **userinfo**, non signés.

Les claims sont construits par le code qui construit les vrais jetons, avec les rôles
et les faits de la personne et le rattachement des rôles de la route : ce que vous
lisez est ce que l'application reçoit. Cela répond à « pourquoi cette personne ne voit
rien dans l'application » avant de lui demander de se reconnecter.

![Une connexion essayée pour une personne : la route qui la laisse entrer, et les claims de l'ID token](img/console/oidc-try.webp)

## Quand ça ne marche pas

Ce que voient la personne ou l'application, et la ligne que journalise la gateway
(`OIDC sign-in refused`, avec le client et la raison, jamais un secret ni un jeton) :

| Symptôme | Cause |
|---|---|
| une page : *asked to sign in as client ..., which this gateway does not know* | l'identifiant du client n'est pas déclaré, ou mal écrit |
| une page : *demande à vous ramener à une adresse qu'elle n'a pas le droit d'utiliser* | l'adresse n'est sous aucune route active liée au client : liez la route de l'application au client. La ligne de journal nomme ce qui est arrivé (`redirect_uri`) et ce qui aurait été accepté (`accepted_under`) |
| `unauthorized_client` | aucune route active n'est liée au client par le mécanisme OIDC |
| la page de refus, ou `access_denied` avec `prompt=none` | aucune route liée au client ne laisse entrer cette personne |
| `temporarily_unavailable` avec une phrase sur l'émetteur | le plan Application n'a pas de nom public : déclarez-en un dans Domains & TLS, onglet Application |
| `invalid_client` | un secret faux, ou un secret envoyé par un client déclaré sans |
| `invalid_grant` sur un refresh | la session de la gateway est finie (déconnexion, expiration), ou plus aucune route n'accorde la personne |
| l'application refuse l'audience du jeton | l'audience qu'elle attend n'est pas l'**Audience** du client |

## Communauté et Enterprise

| | Image communautaire | Image Enterprise |
|---|---|---|
| Le tiroir OIDC / JWT : émetteur, clés, découverte interne et JWKS | oui | oui |
| La ligne de découverte OIDC publique | verrouillée avec la marque Enterprise, sans adresse | tant que le fournisseur répond |
| Les clients OIDC, le mécanisme OIDC sur une route | visibles, verrouillés avec la marque Enterprise ; l'API refuse de déclarer un client | oui |
| Les endpoints du fournisseur | absents du binaire : les chemins retombent sur les routes | oui |

L'API du plan de contrôle est `/api/oidc/clients`, chaque changement audité avec le
secret masqué, et un agent a les outils correspondants (`list_oidc_clients`,
`save_oidc_client`, `regenerate_oidc_client_secret`, `delete_oidc_client`,
`try_oidc_sign_in`), ainsi que pour l'émetteur et les clés (`get_signing_keys`,
`set_issuer`, `renew_signing_keys`) : un assistant peut mettre une application
derrière la gateway de bout en bout, et le prouver par un essai. Par l'API
ou un outil, un client confidentiel sans `secret` en reçoit un tiré par le serveur et
montré une fois dans la réponse ; avec le sien, celui de l'application, il le garde
et ne renvoie rien.

## Pour aller plus loin

- [Brancher une application](/docs/auth/connect-an-application) - des recettes, par
  mécanisme.
- [La règle d'accès d'une route](/docs/access/route-access) - les règles qui décident qui se
  connecte.
- [Sessions](/docs/auth/sessions) - ce qui met fin à la session de la gateway.
- [Le JWT d'identité](/docs/auth/service-jwt) - les clés et l'émetteur partagés avec
  cette page.
