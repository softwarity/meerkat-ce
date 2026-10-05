---
title: OpenID Connect
section: Authentification
order: 104
summary: Envoyer les personnes vers le fournisseur d'identité de votre entreprise - Keycloak, Entra ID, Okta, Auth0, Google - et le laisser prouver qui elles sont.
---

# OpenID Connect

Une autorité OIDC délègue le **premier facteur** à un fournisseur d'identité que votre
organisation exploite déjà. Le navigateur part chez le fournisseur, revient avec un code,
et Meerkat l'échange contre un jeton d'identité qu'il vérifie avec les clés publiées par le
fournisseur. Rien n'est cru sur parole.

Elle est présente dans les deux images, édition Community comprise.

Un bouton OIDC apparaît sur la page de connexion dès que l'autorité est activée. Plusieurs
autorités OIDC peuvent cohabiter : deux fournisseurs, deux boutons.

## Déclarer le fournisseur dans Meerkat

**Infra > Authentication > New authority > OpenID Connect.**

| Champ | Ce qu'il faut y mettre |
|---|---|
| Name | le libellé du bouton sur la page de connexion, par exemple `Acme SSO` |
| Identifier | le segment d'URL par lequel passe cette connexion, déduit du nom ; **figé après la création** |
| Issuer | l'URL de base du fournisseur, à partir de laquelle son document de découverte est lu |
| Client ID | le client que le fournisseur vous a attribué |
| Client secret | son secret, si le client est confidentiel |
| Allowed e-mail domains | séparés par des virgules. Vide, toutes les adresses connues du fournisseur sont acceptées |

Les émetteurs ont cette forme :

| Fournisseur | Émetteur |
|---|---|
| Keycloak | `https://sso.acme.io/realms/main` |
| Microsoft Entra ID | `https://login.microsoftonline.com/<directory-id>/v2.0` |
| Okta | `https://acme.okta.com/oauth2/default` |
| Auth0 | `https://acme.eu.auth0.com` |
| Google | `https://accounts.google.com` |

La barre oblique finale de l'émetteur est retirée des deux côtés : `https://acme.eu.auth0.com/`
et `https://acme.eu.auth0.com` désignent donc la même autorité.

La liste des domaines autorisés est comparée, sans tenir compte de la casse, à ce qui suit
le dernier `@`, et le `@` initial est facultatif dans la liste : `acme.io` et `@acme.io`
fonctionnent tous les deux. Quand la liste n'est pas vide et que le fournisseur ne renvoie
aucune adresse, la connexion est refusée plutôt qu'acceptée.

Le secret client peut être une référence au coffre (`$acme-sso-secret`) plutôt que le
secret lui-même ; les références sont résolues dans la portée infra. Un secret saisi en
clair bloque l'enregistrement tant qu'il n'a pas été rangé dans le coffre.

## Déclarer Meerkat chez le fournisseur

Le tiroir affiche une **Redirect URI** avec un bouton pour la copier. C'est la
seule valeur dont le fournisseur a besoin :

```
https://apps.acme.io/login/acme-sso/callback
```

Sa forme est `/login/<identifier>/callback`, sur l'hôte du **plan de données**.

> [!WARNING]
> Cette adresse est construite à partir de l'hôte par lequel vous avez atteint la
> gateway. Si vous faites cette configuration en passant par `localhost`, déclarez
> plutôt le nom de domaine public : un fournisseur à qui l'on donne un localhost renvoie
> tout le monde vers une machine qui n'est pas la leur.

Les boutons de connexion apparaissent aussi sur la page de connexion de la console, sur le
port d'administration. Une connexion lancée depuis cette page revient sur **cet** hôte, ce
qui donne une URI de redirection différente. Si vous voulez aussi le SSO sur la console,
déclarez les deux adresses chez le fournisseur.

## Ce qui est vérifié au retour

Chaque connexion fait l'objet d'une vérification complète, sans rien présumer :

- le document de découverte est lu à `<issuer>/.well-known/openid-configuration` et mis en cache pendant une heure ; les clés de signature viennent de son `jwks_uri` et sont mises en cache pendant quinze minutes ;
- `state` doit correspondre à celui que Meerkat a généré, et **PKCE** (`S256`) est toujours envoyé, avec ou sans secret ;
- la signature du jeton d'identité est vérifiée avec les clés publiées - un `alg` valant `none` est refusé d'office ;
- `iss` doit être l'émetteur configuré, l'audience doit contenir le Client ID, `exp` doit être dans le futur et `nonce` doit correspondre.

Si le fournisseur ne renvoie pas d'`id_token`, le message le dit clairement : vérifiez que
la portée `openid` est accordée.

## Les claims

| Identité | Claim | Modifiable |
|---|---|---|
| Sujet, le lien stable avec le compte | `sub` | non |
| E-mail | `email` | non |
| Adresse garantie par le fournisseur | `email_verified` | non |
| Nom complet | `name` | non |
| Identifiant | `preferred_username`, à défaut `email` | oui, `usernameClaim` |
| Groupes | `groups` | oui, `groupsClaim` |

Les portées demandées sont par défaut `openid profile email`.

> [!NOTE]
> `usernameClaim`, `groupsClaim`, `scopes` et `useUserinfo` - ce dernier complète les
> claims absents du jeton avec ce que renvoie l'endpoint *userinfo* du fournisseur -
> existent dans la configuration de l'autorité, mais n'ont pas de champ dans la console.
> Ils se règlent par l'API d'administration ou par un fichier de configuration importé.

## Qui la personne devient ici

Une identité venue d'un fournisseur est rapprochée d'un compte local dans cet ordre :

1. un **lien existant** pour cette autorité et ce `sub` - la réponse stable ;
2. un compte portant la **même adresse**, mais seulement si le fournisseur déclare cette adresse vérifiée. Une adresse non vérifiée n'est jamais rapprochée : ce serait permettre une prise de contrôle de compte ;
3. un nouveau compte, si cette autorité est autorisée à en créer.

Le lien est enregistré par autorité : une personne qui s'est inscrite avec un mot de passe
local et qui passe ensuite au SSO conserve donc son compte, ses organisations et ses rôles.
Un compte n'a pas de colonne *source* : il peut être connu de plusieurs autorités à la
fois, et la personne peut se connecter par n'importe laquelle.

Un compte tout juste créé n'a accès à **rien**. Il arrive dans la salle d'attente et y
reste jusqu'à ce qu'un administrateur le rattache à une organisation et lui attribue des
rôles.

Les groupes que rapporte le fournisseur sont enregistrés sur le lien. Ils ne deviennent des
appartenances et des rôles que par l'intermédiaire d'une **règle de groupe** définie sur
une organisation.

> [!NOTE] **Édition Enterprise.**
> L'écriture des règles de groupe - ce qui transforme un claim en appartenance et en
> rôles - fait partie de l'édition Enterprise. Les lire et les supprimer ne sont pas
> soumis à cette restriction.

## Les deux politiques

En bas du tiroir :

**Self-registration** - *Allowed*, *Refused*, ou hérité de l'interrupteur de l'application.
*Refused* signifie que seules les personnes déjà liées peuvent entrer : une personne qui
se présente pour la première fois est refusée avec le message "non invité", au lieu de
recevoir un compte.

**Two-factor** - *Left to the authority* dispense du second facteur propre à Meerkat les
personnes arrivées par cette autorité : le fournisseur les a déjà vérifiées, et demander
deux fois est une contrainte, pas une protection.

> [!WARNING]
> *Always required* sur une autorité se comporte exactement comme *Inherited from the
> application* : il n'impose pas de second facteur. Ce qui décide, c'est le réglage du
> compte et, à défaut, le réglage global dans **Application > Security**. Seul *Left to
> the authority* change quelque chose ici.

Et cette dispense est une **déclaration**, pas une preuve. Meerkat ne lit pas les claims
`acr` et `amr` : il ne peut donc pas savoir si le fournisseur a réellement demandé un
second facteur. Ne choisissez *Left to the authority* que si vous en êtes certain.

## Avant de quitter l'écran

**Test the connection** récupère le document de découverte et les clés de signature. Un
test réussi signifie que l'émetteur est joignable et qu'il publie ce qu'il doit ; il ne dit
rien du secret client, que seule une vraie connexion met à l'épreuve.
