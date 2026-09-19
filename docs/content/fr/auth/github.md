---
title: GitHub
section: Authentification
order: 108
summary: Se connecter avec un compte GitHub, restreint aux organisations que vous nommez.
---

# GitHub

GitHub, c'est de l'OAuth2 simple, pas de l'OpenID Connect. Il prouve le compte,
mais il ne signe rien : il n'y a pas de jeton d'identité à vérifier contre des
clés publiées, donc la confiance repose sur l'échange du code lui-même et sur
TLS. C'est un cran en dessous d'un fournisseur OIDC, et cela mérite d'être su
avant de le mettre devant une application interne.

C'est dans les deux images, communautaire comprise. Cela convient à une gateway
dont les utilisateurs sont des développeurs que vous connaissez déjà par leur
organisation GitHub.

## Déclarer GitHub dans Meerkat

**Infra > Authentication > New authority > GitHub.**

| Champ | Ce qu'on y met |
|---|---|
| Client ID | celui de l'application OAuth, `Iv1.`... |
| Client secret | du même endroit. **Obligatoire**, contrairement à OIDC |
| Allowed organisations | séparées par des virgules, par exemple `acme-io, acme-labs` |

> [!WARNING]
> Laisser **Allowed organisations** vide laisse entrer **n'importe quel compte
> GitHub au monde**. Avec l'auto-inscription, cela veut dire que quiconque a un
> compte GitHub obtient un compte ici - un compte qui n'atteint rien jusqu'à ce
> qu'un administrateur le place quelque part, mais un compte quand même.

## Déclarer Meerkat chez GitHub

Le tiroir vous donne les deux valeurs que réclame le formulaire de GitHub, avec
des boutons de copie et un lien direct vers la page de création :

| Libellé chez GitHub | Ce qu'on y colle |
|---|---|
| Homepage URL | l'origine de votre plan de données, `https://apps.acme.io` |
| Authorization callback URL | `https://apps.acme.io/login/github/callback` |

Puis vous recollez le Client ID et le secret dans Meerkat.

## L'échange

- Portées demandées : `user:email`, plus `read:org` **seulement quand vous avez nommé des organisations**. Ajouter cette seconde portée fait redemander son consentement à la personne, une fois.
- Le `state` est émis et vérifié. Il n'y a **ni PKCE ni nonce** ici : les applications OAuth de GitHub ne connaissent ni l'un ni l'autre, donc le `state` est la seule garde contre un retour rejoué.
- L'habitude de GitHub de répondre une erreur dans une réponse `200` est prise en compte.

## Ce que Meerkat lit

| Identité | D'où ça vient |
|---|---|
| Sujet, le lien stable | l'identifiant **numérique** du compte, jamais le login - un login peut être renommé puis repris par quelqu'un d'autre |
| Identifiant | `login` |
| Nom complet | `name` |
| Adresse | l'adresse du compte, puis la liste des adresses vérifiées, en préférant la principale vérifiée |
| Groupes | `<org>` pour chaque organisation, `<org>/<team>` pour chaque équipe |

Une adresse n'est marquée **vérifiée** que si elle a été trouvée dans cette liste
des adresses vérifiées. Si la liste ne peut pas être lue, l'adresse reste non
vérifiée - donc elle ne sera pas appariée à un compte local existant. C'est
délibéré : apparier sur une adresse non vérifiée est une prise de contrôle de
compte.

Les groupes ne sont récupérés que si vous avez nommé des organisations. Ils
deviennent des appartenances et des rôles par une règle de groupe posée sur une
organisation.

## Comment se comporte la restriction par organisation

Le contrôle a lieu une fois l'identité assemblée, et avant que quoi que ce soit de
local soit touché. Une entrée correspond si c'est l'organisation elle-même, ou
l'une de ses équipes : `acme-io` et `acme-io/platform` satisfont tous deux
`acme-io`. La casse est ignorée. Sinon la personne est refusée, et on lui dit
quelles organisations sont permises.

Si Meerkat n'arrive pas du tout à lire les organisations du compte, la connexion
**échoue** plutôt que d'être traitée comme "n'appartient à rien". La cause
habituelle est nommée dans l'erreur : une organisation qui restreint les
applications tierces doit approuver cette application OAuth.

## Test the connection

Pour GitHub, le bouton vérifie seulement que l'URL d'autorisation répond. Il ne
peut pas valider le secret client - seule une vraie connexion le fait.
