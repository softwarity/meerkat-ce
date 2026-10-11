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
| `iss` | l'émetteur : `meerkat` tant que vous n'en réglez pas un autre |
| `sub` | l'identifiant du compte de l'appelant |
| `aud` | le nom de la route |
| `iat`, `exp` | quand il a été signé, et quand il cesse d'être valide : deux minutes plus tard par défaut, réglable par route |
| vos claims | les attributs que la route a choisi d'envoyer - nom d'utilisateur, e-mail, rôles, organisation - sous les noms que vous leur avez donnés |

Il se choisit dans l'éditeur de route, sous **Auth forward** : le mécanisme
`signed-jwt`, l'algorithme (**ES256** recommandé, EdDSA ou RS256 pour une
bibliothèque qui ne connaît rien d'autre), les attributs et la forme des rôles.
L'aperçu y montre le jeton exact que recevrait un service.

## Comment un service le vérifie

Tout ce dont un service a besoin est servi sur le **plan interne**, port
**7070** : le port qu'appellent les services placés derrière la gateway,
jamais placé derrière un ingress. Rien hors du cluster n'a de raison de le
lire, donc rien hors du cluster ne l'atteint. Il ne demande aucun jeton : il ne
contient que des clés publiques, qui ne donnent aucun pouvoir, et un service
doit pouvoir les lire avant de détenir quoi que ce soit. `meerkat` ci-dessous
est le nom de la gateway dans nos fichiers de déploiement ; le tiroir JWT écrit le
vrai, demandé au runtime - sur Kubernetes le nom complet du Service, par exemple
`http://meerkat-internal.shop.svc.cluster.local:7070`, sur Swarm ou compose celui
du service.

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
  `openid-client` en Node. N'importe quel émetteur convient, `meerkat`
  compris. En HTTP simple, dites-le là où la bibliothèque le demande
  (`RequireHttpsMetadata = false` en .NET).
- **L'émetteur seul** - `issuer-uri` chez Spring, le provider de go-oidc.
  Elles lisent `{issuer}/.well-known/openid-configuration` et vérifient que le
  document nomme le même émetteur : l'émetteur doit donc être l'URL du plan
  interne. Réglez-le sur `http://meerkat:7070` dans le tiroir JWT (voir
  plus bas), et toute la configuration du service tient en cette ligne.

### Le JWKS, pour une bibliothèque qui ne prend que des clés

```
http://meerkat:7070/.well-known/jwks.json
```

La partie publique des clés, sous forme de JWKS standard - pour une
bibliothèque qui ne connaît pas la découverte, comme le jeu de clés distant de
`jose`. Le `kid` du jeton désigne la clé, et la clé porte son algorithme : le
service prend donc l'algorithme dans la clé, jamais dans l'en-tête du jeton. Il
vérifie alors lui-même l'émetteur, avec la valeur réglée dans le tiroir.

### Un PEM, pour un service qui ne peut rien lire

Le tiroir **JWT**, à côté de **Global** sur l'écran Routes, montre la clé
publique de chaque algorithme au format PEM. Cette clé doit être remplacée à la
main à chaque rotation, là où les deux adresses ci-dessus la suivent toutes
seules.

## L'émetteur

Chaque jeton dit qui l'a émis dans `iss`, et un service le vérifie. Il vaut
`meerkat` tant que vous n'en réglez pas un autre, dans le tiroir **JWT** :
**Issuer**. Changez-le, et le jeton suivant porte le nouveau, sur chaque nœud -
changez donc en même temps ce qu'attendent les services. Un émetteur qui est une
URL est une URL http ou https avec un hôte, sans requête, sans fragment et sans
barre finale.

> [!NOTE]
> Le document de découverte décrit comment vérifier un jeton, pas comment en
> obtenir un. Il n'y a ni endpoint d'autorisation, ni de jeton, ni userinfo : la
> gateway n'est pas un serveur OAuth, elle signe un jeton court pour chaque
> requête qu'elle transmet. Une bibliothèque qui exige ces endpoints doit plutôt
> être pointée vers le JWKS.

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
> le tiroir JWT donne les deux. Rien d'autre ne change : mêmes clés, mêmes
> jetons.

## Pour aller plus loin

- [Architecture](/docs/concepts/architecture) - les trois plans et ce qui les
  sépare.
- [Cluster Kubernetes](/docs/deploy/kubernetes) - le Service interne, et une
  NetworkPolicy qui tient la console à l'écart des backends.
- [Appels planifiés](/docs/operations/scheduler) - l'autre API du plan interne.
