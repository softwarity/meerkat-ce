---
title: SAML 2.0
section: Authentification
order: 105
summary: Envoyer les personnes vers un fournisseur d'identité qui parle SAML - ADFS, Entra ID, Okta, Shibboleth - et vérifier sa réponse signée.
---

# SAML 2.0

Une autorité SAML fait ce que fait une autorité [OIDC](/docs/auth/oidc), pour les
entreprises dont le fournisseur d'identité parle SAML : le navigateur part chez le
fournisseur, la personne s'y connecte, et le fournisseur renvoie une **assertion
signée** qui dit qui elle est. Meerkat ne voit jamais son mot de passe. Édition
Enterprise.

La connexion est initiée par la gateway : elle commence sur sa propre page, avec un
bouton par fournisseur. Une assertion que le fournisseur enverrait de lui-même, sans
demande de cette gateway, est refusée : rien ne la lierait au navigateur qui la présente.

## La déclarer dans Meerkat

**Infra > Authentication**, une nouvelle autorité, de type **SAML** :

- **Identity provider metadata URL** - l'adresse où le fournisseur publie ses
  métadonnées (ADFS : `/FederationMetadata/2007-06/FederationMetadata.xml`). Elles sont
  relues toutes les heures : un nouveau certificat de signature publié avant un
  renouvellement est pris en compte tout seul. Si le fournisseur ne publie pas d'URL,
  collez ses métadonnées à la place.
- **NameID format** - la façon dont le fournisseur désigne la personne. Laissez-le
  choisir, ou demandez `persistent` ou `emailAddress`.
- **Attributes** - les attributs qui portent l'adresse, le nom et les groupes. Vides,
  les noms usuels sont essayés : les URI de claims d'ADFS et d'Entra ID, les noms
  courts d'Okta et de Shibboleth.
- **Allowed e-mail domains**, comme pour OIDC.

**Test** lit les métadonnées du fournisseur et dit ce qui manque : aucun certificat de
signature, ou aucun point de connexion pour le binding HTTP-Redirect par lequel la
gateway envoie ses demandes.

## Déclarer Meerkat chez le fournisseur

L'écran donne trois valeurs, chacune avec son bouton de copie :

| Ce que le fournisseur appelle | Ce que c'est |
|---|---|
| Entity ID, identifiant, audience | `https://<gateway>/login/<id>/metadata` |
| Assertion consumer service, reply URL | `https://<gateway>/login/<id>/callback` |
| Métadonnées du fournisseur de service | la même adresse `/metadata` : la plupart des fournisseurs l'importent plutôt que de faire saisir les deux précédentes |

Les métadonnées de la gateway disent ce qu'elle attend : des assertions signées,
renvoyées par POST (HTTP-POST).

## Ce que la réponse doit respecter

Tout, avec un refus au premier écart :

- **la signature**, vérifiée avec le certificat du fournisseur tiré de ses
  métadonnées - par une bibliothèque qu'exercent des milliers de déploiements, jamais à
  la main : c'est sur le « signature wrapping » que cassent les implémentations SAML ;
- **l'émetteur**, l'entity ID du fournisseur ;
- **l'audience**, l'entity ID de cette gateway ;
- **la destination et le destinataire**, l'ACS de cette gateway ;
- **la période de validité** (`NotBefore`, `NotOnOrAfter`), et un certificat expiré ;
- **la demande à laquelle elle répond** (`InResponseTo`) : celle que ce même navigateur a
  faite, gardée dans son propre cookie ;
- **un rejeu** : une assertion n'est acceptée qu'une fois. Celles déjà utilisées sont
  gardées en base jusqu'à leur expiration : un second envoi de la même est refusé sur
  toutes les gateways d'un cluster.

La réponse est renvoyée depuis le site du fournisseur, et un navigateur ne joint pas le
cookie de la gateway à ce genre d'envoi. La gateway y répond donc par une page qui
renvoie la même réponse à elle-même, depuis sa propre adresse : le cookie part, et rien
de la connexion en cours n'est gardé sur le serveur.

## Qui devient la personne ici

Le compte est lié au NameID, qui doit donc être **stable**. Un NameID **transitoire**
change à chaque connexion et créerait un nouveau compte à chaque fois : il est refusé,
avec une issue - nommez un attribut stable (l'adresse, un `objectGUID`) sous **Subject
attribute**, et le compte est lié par lui.

La suite est celle de toute autorité : un premier venu crée un compte en attente, ou est
refusé, selon la politique de l'autorité ; le second facteur et les passkeys suivent ses
politiques ; les groupes qu'elle envoie sont ce qu'une règle de groupe d'une organisation
fait correspondre à des rôles. Voir [OpenID Connect](/docs/auth/oidc).

## Demandes signées, assertions chiffrées

La plupart des fournisseurs signent leurs réponses et n'attendent rien de la gateway.
Deux cas demandent sa propre paire de clés :

- **Sign the requests**, pour un fournisseur qui l'exige ;
- **les assertions chiffrées**, que la gateway lit avec la même clé.

Collez le certificat de la gateway et sa clé privée - un champ secret, rangé dans le
[coffre](/docs/console/vault). Le certificat apparaît alors dans les métadonnées de la
gateway, que le fournisseur peut reprendre.
