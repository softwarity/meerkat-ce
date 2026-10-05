---
title: GitHub
section: Authentification
order: 109
summary: Se connecter avec un compte GitHub, en limitant l'accès aux organisations que vous indiquez.
---

# GitHub

GitHub parle OAuth2 et non OpenID Connect. Il prouve l'existence du compte, mais il ne
signe rien : il n'y a pas de jeton d'identité à vérifier avec des clés publiées. La
confiance repose donc sur l'échange du code lui-même et sur TLS. C'est un cran en dessous
d'un fournisseur OIDC, et mieux vaut le savoir avant de placer GitHub devant une
application interne.

Il est présent dans les deux images, édition Community comprise. Il convient à une
gateway dont les utilisateurs sont des développeurs que vous connaissez déjà par leur
organisation GitHub.

## Déclarer GitHub dans Meerkat

**Infra > Authentication > New authority > GitHub.**

| Champ | Ce qu'il faut y mettre |
|---|---|
| Client ID | celui de l'application OAuth, `Iv1.`... |
| Client secret | fourni au même endroit. **Obligatoire**, contrairement à OIDC |
| Allowed organisations | séparées par des virgules, par exemple `acme-io, acme-labs` |

> [!WARNING]
> Laisser **Allowed organisations** vide laisse entrer **n'importe quel compte GitHub au
> monde**. Associé à l'auto-inscription, cela signifie que toute personne possédant un
> compte GitHub obtient un compte ici - un compte qui n'a accès à rien tant qu'un
> administrateur ne l'a pas rattaché quelque part, mais un compte tout de même.

## Déclarer Meerkat chez GitHub

Le tiroir vous donne les deux valeurs que demande le formulaire de GitHub, avec
des boutons pour les copier et un lien direct vers la page de création :

| Libellé chez GitHub | Ce qu'il faut coller |
|---|---|
| Homepage URL | l'origine de votre plan de données, `https://apps.acme.io` |
| Authorization callback URL | `https://apps.acme.io/login/github/callback` |

Reportez ensuite le Client ID et le secret dans Meerkat.

## L'échange

- Portées demandées : `user:email`, plus `read:org` **uniquement si vous avez indiqué des organisations**. L'ajout de cette seconde portée amène GitHub à redemander son consentement à la personne, une fois.
- `state` est généré puis vérifié. Il n'y a **ni PKCE ni nonce** ici : les applications OAuth de GitHub ne prennent en charge ni l'un ni l'autre, et `state` est donc la seule protection contre le rejeu d'un retour.
- L'habitude qu'a GitHub de renvoyer une erreur dans une réponse `200` est prise en compte.

## Ce que lit Meerkat

| Identité | Origine |
|---|---|
| Sujet, le lien stable | l'identifiant **numérique** du compte, jamais le login - un login peut être renommé puis repris par quelqu'un d'autre |
| Identifiant | `login` |
| Nom complet | `name` |
| E-mail | l'adresse du compte, puis la liste des adresses vérifiées, en préférant l'adresse principale vérifiée |
| Groupes | `<org>` pour chaque organisation, `<org>/<team>` pour chaque équipe |

Une adresse n'est marquée **vérifiée** que si elle figure dans cette liste d'adresses
vérifiées. Si la liste ne peut pas être lue, l'adresse reste non vérifiée, ce qui signifie
qu'elle ne sera pas rapprochée d'un compte local existant. C'est voulu : rapprocher des
comptes sur la foi d'une adresse non vérifiée, c'est permettre une prise de contrôle de
compte.

Les groupes ne sont récupérés que si vous avez indiqué des organisations. Ils deviennent
des appartenances et des rôles par l'intermédiaire d'une règle de groupe définie sur une
organisation.

## Comment se comporte la restriction par organisation

Le contrôle a lieu une fois l'identité reconstituée, et avant de toucher à quoi que ce
soit en local. Une entrée correspond s'il s'agit de l'organisation elle-même ou de l'une de
ses équipes : `acme-io` et `acme-io/platform` satisfont tous deux `acme-io`. La casse est
ignorée. Dans le cas contraire, la personne est refusée, et le message lui indique quelles
organisations sont autorisées.

Si Meerkat ne parvient pas du tout à lire les organisations du compte, la connexion
**échoue** au lieu d'être traitée comme "n'appartient à aucune organisation". L'erreur
nomme la cause habituelle : une organisation qui restreint les applications tierces doit
approuver cette application OAuth.

## Test the connection

Pour GitHub, ce bouton vérifie seulement que l'URL d'autorisation répond. Il ne peut pas
valider le secret client : seule une vraie connexion le fait.
