---
title: Le JWT d'identité
section: Authentification
order: 117
summary: Le jeton signé qu'un service reçoit avec chaque requête, comment il le vérifie à partir du document de découverte du plan interne, et ce qu'une rotation des clés lui fait.
---

# Le JWT d'identité

::: lead
Une route peut dire à son service qui appelle au moyen d'un **JWT signé** : la
gateway signe un jeton court pour chaque requête qu'elle transmet, et le service
le vérifie avec les clés publiques que la gateway publie. Le service n'a besoin
d'aucun secret, d'aucune table de comptes ni d'aucun rappel vers qui que ce soit.
:::

## Ce que reçoit le service

Le jeton arrive dans `Authorization: Bearer ...`, et tout `Authorization` envoyé
par le client est d'abord supprimé : un appelant ne peut pas en fabriquer un. Il
porte :

| Claim | Ce qu'il contient |
|---|---|
| `iss` | l'émetteur : l'adresse publique du plan Application (`https://<nom public>`), `meerkat` tant qu'aucun n'est déclaré |
| `sub` | l'identifiant du compte de l'appelant |
| `aud` | l'**Audience** de la route, ou son nom quand le champ est vide : en poser une quand le service attend une valeur à lui, ou pour qu'un renommage de la route ne change pas ses jetons. La même règle que l'audience d'un client OIDC : un texte de 255 caractères au plus, espaces à l'intérieur compris, sans caractère de contrôle |
| `iat`, `exp` | quand il a été signé, et quand il cesse d'être valide : deux minutes plus tard par défaut, réglable par route |
| vos claims | les attributs que la route a choisi d'envoyer - nom d'utilisateur, e-mail, rôles, organisation - sous les noms que vous leur avez donnés |

Il se choisit dans l'éditeur de route, sous **Auth forward** : le mécanisme
`signed-jwt`, l'algorithme (**ES256** recommandé, EdDSA ou RS256 pour une
bibliothèque qui ne connaît rien d'autre), la durée du jeton et son audience,
les attributs et la forme des rôles.
L'aperçu y montre le jeton exact que recevrait un service.

### La forme des rôles

L'attribut roles porte une expression, lue comme un pipeline de gauche à
droite. La même expression façonne l'en-tête des rôles, un claim JWT et un claim
OIDC ; un résultat qui est un tableau JSON arrive dans un claim comme un tableau.

| Verbe | Ce qu'il fait |
|---|---|
| `.Keep "tag:BILLING" "ROLE_USER"` | garde les rôles qui portent une étiquette du catalogue, ou nommés |
| `.Drop ...` | l'inverse : tout sauf ceux-là |
| `.Declared` | les rôles que l'application de la route déclare dans sa spec, rattachés sur Endpoint security : ceux que la personne détient, sous les noms de l'application |
| `cutHead "ROLE_"`, `cutTail`, `addHead`, `addTail` | retouchent chaque nom |
| `lower`, `upper` | changent la casse |
| `join ","`, `json` | le résultat : une liste séparée, ou un tableau JSON |

Sans expression, une route envoie tous les rôles séparés par des virgules
(`{{join "," .Roles}}`), ou, dès qu'elle rattache les rôles que son application
déclare, `{{.Roles | .Declared | join ","}}` (voir
[Sécurité par endpoint](/docs/access/endpoint-security#les-rles-que-lapplication-dclare)).
`.Keep`, `.Drop` et `.Declared` lisent les noms du catalogue : ils passent avant
tout verbe qui les retouche.

## Comment un service le vérifie

Tout ce dont un service a besoin est servi sur le **plan interne**, port
**7070** : le port qu'appellent les services placés derrière la gateway,
jamais placé derrière un ingress. Rien hors du cluster n'a de raison de le
lire, donc rien hors du cluster ne l'atteint. Il ne demande aucun jeton : il ne
contient que des clés publiques, qui ne donnent aucun pouvoir, et un service
doit pouvoir les lire avant de détenir quoi que ce soit. `meerkat` ci-dessous
est le nom de la gateway dans nos fichiers de déploiement ; le tiroir OIDC / JWT écrit le
vrai, demandé au runtime - sur Kubernetes le nom complet du Service, par exemple
`http://meerkat-internal.shop.svc.cluster.local:7070`, sur Swarm ou compose celui
du service. Dès qu'un certificat est placé sur le plan interne, le même contenu est
servi en HTTPS sur **7443**, et le tiroir donne l'adresse en `https://` ; les
services font alors confiance à l'autorité qui l'a signé
([TLS](/docs/operations/tls#un-certificat-depuis-le-systme-de-fichiers)).

### Recommandé : le document de découverte

```
http://meerkat:7070/.well-known/openid-configuration
```

```json
{
  "issuer": "meerkat",
  "jwks_uri": "http://meerkat:7070/.well-known/jwks.json",
  "id_token_signing_alg_values_supported": ["ES256", "EdDSA", "RS256"],
  "subject_types_supported": ["public"]
}
```

Une seule adresse, et la bibliothèque y apprend les deux choses qu'elle doit
vérifier : l'**émetteur** que portent les jetons, et les **clés** qui les
signent. Rien à recopier à la main dans la configuration du service, et une
rotation des clés lui parvient toute seule. `jwks_uri` est construit à partir
de l'adresse qu'a utilisée l'appelant : il nomme donc la gateway comme le
service l'atteint.

Les bibliothèques le prennent de l'une de deux façons :

- **L'adresse du document**, comme ci-dessus - `MetadataAddress` en .NET,
  `openid-client` en Node. Quel que soit l'émetteur, c'est la voie que doit
  prendre un service dans le cluster. En HTTP simple, dites-le là où la
  bibliothèque le demande (`RequireHttpsMetadata = false` en .NET).
- **L'émetteur seul** - `issuer-uri` chez Spring, le provider de go-oidc.
  Elles lisent `{issuer}/.well-known/openid-configuration` sur l'adresse
  publique de la gateway, qui ne répond que tant que le
  [fournisseur OpenID](/docs/auth/oidc-provider) (édition Enterprise) a un
  client déclaré. Sinon, donnez à une telle bibliothèque le JWKS ci-dessous et
  l'émetteur à vérifier (Spring : `jwk-set-uri` à côté de l'émetteur attendu)
  plutôt que l'émetteur seul.

### Le JWKS, pour une bibliothèque qui ne prend que des clés

```
http://meerkat:7070/.well-known/jwks.json
```

La partie publique des clés, sous forme de JWKS standard - pour une
bibliothèque qui ne connaît pas la découverte, comme le jeu de clés distant de
`jose`. Le `kid` du jeton désigne la clé, et la clé porte son algorithme : le
service prend donc l'algorithme dans la clé, jamais dans l'en-tête du jeton. Il
vérifie alors lui-même l'émetteur, celui que montre le tiroir.

### Un PEM, pour un service qui ne peut rien lire

Le tiroir **OIDC / JWT**, à côté de **Global** sur l'écran Routes, montre la clé
publique de chaque algorithme au format PEM. Cette clé doit être remplacée à la
main à chaque rotation, là où les deux adresses ci-dessus la suivent toutes
seules.

## L'émetteur

Chaque jeton dit qui l'a émis dans `iss`, et un service le vérifie. Il ne se
tape pas : c'est le **nom public** du plan Application, déclaré dans **Domains &
TLS**, onglet **Application**, écrit `https://<nom>` (un nom de cette machine,
`localhost`, donne `http://localhost:<port du plan de données>`). Avec plusieurs
noms, le tiroir **OIDC / JWT** en choisit un dans **Public address**, le premier
jusque-là ; sans aucun, les jetons disent `meerkat` et le tiroir mène à cet
onglet. Le même émetteur est celui du [fournisseur OpenID](/docs/auth/oidc-provider).

Déclarez, retirez ou choisissez un nom public, et le jeton suivant porte le
nouvel émetteur, sur chaque nœud - changez donc en même temps ce qu'attendent
les services.

> [!NOTE]
> Le document de découverte décrit comment vérifier un jeton, pas comment en
> obtenir un : pour ce mécanisme, la gateway signe un jeton court pour chaque
> requête qu'elle transmet, et il n'y a rien à échanger. Une bibliothèque qui
> exige des endpoints d'autorisation et de jeton doit plutôt être pointée vers le
> JWKS. Une application qui veut connecter elle-même ses utilisateurs est un autre
> cas : avec le [fournisseur OpenID](/docs/auth/oidc-provider) (édition
> Enterprise), le même document nomme alors aussi ces endpoints.

## La rotation

**Renew keys**, dans le même tiroir, crée une nouvelle paire pour chaque
algorithme et signe aussitôt avec elle. Les anciennes clés publiques restent dix
minutes dans le JWKS, pour que les jetons déjà signés restent vérifiables jusqu'à
leur expiration.

Le JWKS est servi avec un cache de cinq minutes. Une bibliothèque qui relit le
JWKS quand elle rencontre un `kid` inconnu - c'est le cas de la plupart - suit
une rotation aussitôt. Une bibliothèque qui ne se fie qu'à son cache peut refuser
les nouveaux jetons jusqu'à cinq minutes après une rotation : vérifiez le
comportement de la vôtre avant de faire une rotation en production.

## Depuis le port applicatif

Avant le plan interne, le JWKS était servi sur le **port applicatif** (8080), et
les services pointaient vers lui. Ce n'est plus le cas : le port applicatif sert
vos utilisateurs, et ces clés ne les regardent pas. Sur ce port, le chemin est
désormais un chemin ordinaire, qu'une route peut prendre.

> [!WARNING]
> À la mise à jour, un service qui lit encore
> `http://...:8080/.well-known/jwks.json` ne peut plus vérifier ses jetons, et
> refuse les requêtes qu'il reçoit. Faites-le pointer vers le plan interne
> avant la mise à jour ou en même temps - le document de découverte,
> `http://<gateway>:7070/.well-known/openid-configuration`, ou le JWKS à côté ;
> le tiroir OIDC / JWT donne les deux. Rien d'autre ne change : mêmes clés, mêmes
> jetons.

## Pour aller plus loin

- [Architecture](/docs/concepts/architecture) - les trois plans et ce qui les
  sépare.
- [Cluster Kubernetes](/docs/deploy/kubernetes) - le Service interne, et une
  NetworkPolicy qui tient la console à l'écart des backends.
- [Appels planifiés](/docs/operations/scheduler) - l'autre API du plan interne.
- [Meerkat fournisseur OpenID](/docs/auth/oidc-provider) - quand l'application
  connecte elle-même ses utilisateurs, avec les mêmes clés.
