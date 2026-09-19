---
title: OpenID Connect
section: Authentification
order: 104
summary: Envoyer les gens vers le fournisseur d'identité de l'entreprise - Keycloak, Entra ID, Okta, Auth0, Google - et le laisser prouver qui ils sont.
---

# OpenID Connect

Une autorité OIDC délègue le **premier facteur** à un fournisseur d'identité que
votre organisation exploite déjà. Le navigateur part chez le fournisseur, revient
avec un code, et Meerkat l'échange contre un jeton d'identité qu'il vérifie
contre les clés publiées par le fournisseur. Rien n'est cru sur parole.

C'est dans les deux images, communautaire comprise.

Un bouton OIDC apparaît sur la page de connexion dès que l'autorité est active.
Plusieurs autorités OIDC vivent côte à côte : deux fournisseurs, deux boutons.

## Déclarer le fournisseur dans Meerkat

**Infra > Authentication > New authority > OpenID Connect.**

| Champ | Ce qu'on y met |
|---|---|
| Name | ce que dira le bouton de la page de connexion, par exemple `Acme SSO` |
| Identifier | le segment par lequel passe cette connexion, dérivé du nom ; **figé après création** |
| Issuer | l'URL de base du fournisseur, d'où son document de découverte est lu |
| Client ID | le client que le fournisseur vous a donné |
| Client secret | son secret, si le client est confidentiel |
| Allowed e-mail domains | séparés par des virgules. Vide accepte toute adresse que le fournisseur connaît |

Les issuers ressemblent à ceci :

| Fournisseur | Issuer |
|---|---|
| Keycloak | `https://sso.acme.io/realms/main` |
| Microsoft Entra ID | `https://login.microsoftonline.com/<directory-id>/v2.0` |
| Okta | `https://acme.okta.com/oauth2/default` |
| Auth0 | `https://acme.eu.auth0.com` |
| Google | `https://accounts.google.com` |

Une barre oblique finale est retirée des deux côtés : `https://acme.eu.auth0.com/`
et `https://acme.eu.auth0.com` sont la même autorité.

La liste de domaines est comparée sans tenir compte de la casse à ce qui suit le
dernier `@`, et un `@` en tête de la liste est facultatif : `acme.io` et
`@acme.io` marchent tous les deux. Quand la liste n'est pas vide et que le
fournisseur ne rend aucune adresse, la connexion est refusée plutôt que laissée
passer.

Le secret client peut être une référence au coffre (`$acme-sso-secret`) plutôt
que le secret lui-même ; les références se résolvent dans la portée infra. Taper
un secret en clair bloque l'enregistrement jusqu'à ce qu'il soit rangé.

## Déclarer Meerkat chez le fournisseur

Le tiroir affiche une **Redirect URI** avec un bouton de copie, et c'est la seule
valeur dont le fournisseur a besoin :

```
https://apps.acme.io/login/acme-sso/callback
```

La forme est `/login/<identifier>/callback`, sur l'hôte du **plan de données**.

> [!WARNING]
> Cette adresse est construite depuis l'hôte par lequel vous avez atteint la
> gateway. Si vous faites le réglage via `localhost`, enregistrez plutôt le nom
> de domaine public : un fournisseur à qui l'on donne un localhost renvoie tout
> le monde vers une machine qui n'est pas la sienne.

Les boutons de connexion apparaissent aussi sur la page de connexion de la
console, sur le port d'administration. Une connexion lancée là revient sur **cet**
hôte, donc sur une autre Redirect URI. Pour avoir le SSO sur la console aussi,
enregistrez les deux adresses chez le fournisseur.

## Ce qui est vérifié au retour

Chaque connexion est une vérification complète, pas un acte de confiance :

- le document de découverte est lu sur `<issuer>/.well-known/openid-configuration` et gardé une heure ; les clés de signature viennent de son `jwks_uri` et sont gardées quinze minutes ;
- le `state` doit correspondre à celui que Meerkat a émis, et **PKCE** (`S256`) est toujours envoyé, avec ou sans secret ;
- la signature du jeton d'identité est vérifiée contre les clés publiées - un `alg` valant `none` est refusé net ;
- `iss` doit être l'issuer configuré, l'audience doit contenir le Client ID, `exp` doit être dans le futur, et le `nonce` doit correspondre.

Un fournisseur qui ne rend pas d'`id_token` se le fait dire clairement :
vérifiez que la portée `openid` est accordée.

## Les claims

| Identité | Claim | Modifiable |
|---|---|---|
| Sujet, le lien stable avec le compte | `sub` | non |
| Adresse | `email` | non |
| Adresse attestée | `email_verified` | non |
| Nom complet | `name` | non |
| Identifiant | `preferred_username`, à défaut `email` | oui, `usernameClaim` |
| Groupes | `groups` | oui, `groupsClaim` |

Les portées demandées valent par défaut `openid profile email`.

> [!NOTE]
> `usernameClaim`, `groupsClaim`, `scopes` et `useUserinfo` - ce dernier fusionne
> ce que dit le point *userinfo* du fournisseur dans les claims que le jeton a
> laissés vides - existent dans la configuration de l'autorité mais n'ont pas de
> case dans la console. Ils se posent par l'API d'administration ou par un
> fichier de configuration importé.

## Qui la personne devient ici

Une identité venue d'un fournisseur est résolue vers un compte local dans cet
ordre :

1. un **lien existant** pour cette autorité et ce `sub` : la réponse stable ;
2. un compte portant la **même adresse**, mais seulement si le fournisseur dit que cette adresse est vérifiée. Une adresse non vérifiée n'est jamais appariée : ce serait une prise de contrôle de compte ;
3. un nouveau compte, si cette autorité est autorisée à en créer.

Le lien est enregistré par autorité : qui s'est inscrit avec un mot de passe
local puis passe au SSO garde son compte, ses organisations et ses rôles. Il n'y
a pas de colonne "source" sur un compte : un compte peut être connu de
plusieurs autorités à la fois, et la personne se connecte par n'importe laquelle.

Un compte fraîchement créé n'atteint **rien**. Il attend dans la salle d'attente
jusqu'à ce qu'un administrateur le place dans une organisation et lui accorde des
rôles.

Les groupes que le fournisseur rapporte sont enregistrés sur le lien. Ils ne
deviennent des appartenances et des rôles que par une **règle de groupe** posée
sur une organisation.

> [!NOTE] **Enterprise edition.**
> Écrire des règles de groupe - ce qui transforme un claim en appartenance et en
> rôles - fait partie de l'édition Enterprise. Les lire, et en supprimer une, ne
> sont pas bridés.

## Les deux politiques

En bas du tiroir :

**Self-registration** - *Allowed*, *Refused*, ou hérité de l'interrupteur de
l'application. *Refused* veut dire que seules les personnes déjà liées entrent :
un premier arrivant est écarté d'un "non invité" plutôt que doté d'un compte.

**Two-factor** - *Left to the authority* saute le second facteur propre à Meerkat
pour les gens entrés par là : le fournisseur les a déjà mis au défi, et demander
deux fois est un impôt, pas une défense.

> [!WARNING]
> *Always required* sur une autorité se comporte exactement comme *Inherited from
> the application* : cela ne force pas le second facteur. Ce qui décide, c'est le
> réglage du compte, puis celui de l'installation dans **Application > Security**.
> Seul *Left to the authority* change quelque chose ici.

Et le saut est une **déclaration**, pas une preuve. Meerkat ne lit pas les claims
`acr` ni `amr` : il ne peut pas savoir si le fournisseur a réellement demandé un
second facteur. Ne posez *Left to the authority* que là où vous le savez.

## Avant de quitter l'écran

**Test the connection** récupère le document de découverte et les clés de
signature. Réussir prouve que l'issuer est joignable et publie ce qu'il doit ;
cela ne dit rien du secret client, que seule une vraie connexion exerce.
