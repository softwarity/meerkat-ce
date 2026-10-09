---
title: Identité
section: Concepts
order: 22
summary: Les trois manières dont la gateway sait qui appelle - une session, un jeton d'API, ou rien du tout - et d'où viennent les comptes.
---

# Identité

Avant de décider si une requête peut passer, la gateway détermine qui en est
l'auteur. Elle peut trouver exactement deux réponses, et il existe un troisième
cas qu'elle traite comme tel.

| | Ce que c'est | Qui s'en sert |
|---|---|---|
| Session | un jeton opaque dans un cookie httpOnly | une personne dans un navigateur |
| Jeton d'API | `Authorization: Bearer mk_...` | un script, un service, un agent |
| Anonyme | ni l'un ni l'autre | tout ce qu'une route laisse passer sans rien demander |

## Les sessions

Une session est une ligne dans la base de données ; le navigateur détient un
jeton, et c'est son empreinte qui est stockée. Détruire une session détruit la
ligne : après une déconnexion, le cookie n'est pas simplement oublié, il ne
correspond plus à rien.

Chaque plan a son propre cookie, `MEERKAT_SESSION` sur le plan de données et
`MEERKAT_ADMIN_SESSION` sur le plan de contrôle, et chaque session porte la
marque de son plan. Un jeton de l'un est refusé sur l'autre.

La durée de vie mesure l'**inactivité**, et non le temps écoulé depuis la
connexion : toute requête qui porte une session valide repousse son échéance.
La valeur par défaut est de 30 minutes ; elle se résout d'abord au niveau du
membre, puis de l'organisation, puis de l'installation.

> [!NOTE]
> Cette résolution existe, mais seule la valeur globale se modifie aujourd'hui
> dans la console. Les surcharges par organisation et par membre sont stockées,
> sans écran pour les modifier à ce jour.

Une session porte aussi ce que le parcours de connexion a établi :
l'organisation active, le groupe actif, et l'étape qui reste à franchir. Une
session bloquée sur une étape y est renvoyée à chaque navigation, jusqu'à ce
que l'étape soit franchie.

> [!WARNING]
> Le cookie de session n'a pas d'attribut `Domain` : il n'est donc pas partagé
> entre sous-domaines. À ce jour, des applications servies sous des noms d'hôte
> différents ne partagent pas une même session.

## Les jetons d'API

Un jeton est émis une seule fois, et sa valeur en clair n'est affichée qu'une
fois. Il porte le préfixe `mk_` et se présente à l'endroit habituel :

```bash
curl -H 'Authorization: Bearer mk_...' https://apps.example.com/billing/invoices
```

Un jeton appartient à un seul **plan**, et les deux sont étanches : un jeton du
plan de données n'ouvre jamais le port d'administration, et un jeton
d'administration ne passe jamais sur le plan de données. Seul un administrateur
global émet des jetons d'administration, depuis la console ; chacun émet son
propre jeton du plan de données sur `/profile/tokens`.

Un jeton ne peut jamais détenir que **moins** de droits que son propriétaire.
Trois axes le restreignent : une portée (lecture seule, complète, ou appels
planifiés uniquement), un domaine (le côté routage, le côté application, ou les
deux) et une liste de réseaux clients - évaluée sur l'adresse réelle du pair,
jamais sur un en-tête de transfert.

> [!NOTE]
> Les jetons du plan de données n'ont pas encore de périmètre : ils sont émis
> avec la portée complète et reprennent, sans le dire, le groupe que portait la
> session. La console les liste et peut les révoquer (**Access tokens**), mais
> n'en crée jamais : un jeton n'est émis que par son propriétaire, depuis la
> page en libre-service.

## D'où viennent les comptes

Un compte est reconnu par une **autorité**, et la table des comptes locaux est
une autorité parmi les autres. Un seul écran, **Infra > Authentication**, les
liste toutes et répond à la question *par quels moyens peut-on se connecter
ici ?*

| Autorité | État |
|---|---|
| Comptes locaux (mot de passe) | livré |
| OIDC - tout fournisseur conforme : Keycloak, Entra ID, Okta | livré |
| GitHub | livré |
| LDAP / Active Directory | livré, édition Enterprise |
| SAML 2.0 | livré, édition Enterprise |
| Kerberos / SPNEGO | non réalisé |

Une autorité externe ne répond que du **premier facteur**. Elle dit que c'est
bien cette personne ; ce qu'elle peut faire ici relève de la gateway, et découle
des rôles, des groupes et des appartenances - qu'une règle de groupe Enterprise
peut déduire, à chaque connexion, des groupes de l'annuaire.

Plusieurs réglages se définissent par autorité, avec un troisième état qui
hérite du réglage global : l'obligation d'un second facteur, l'autorisation des
passkeys, et la création d'un compte pour une personne inconnue qui se
connecte.

## Le parcours de connexion

Les pages sont servies par la gateway, sur le plan de données, dans la
langue du visiteur et avec le thème de l'installation. L'ordre est fixe :

1. **Changement du mot de passe**, s'il est temporaire ou expiré.
2. **Second facteur** : un code TOTP si le compte est enrôlé, sauf si ce navigateur a été déclaré de confiance ; un enrôlement forcé si le MFA est obligatoire et qu'aucun facteur n'existe encore.
3. **Organisation**, puis **groupe** quand l'organisation fonctionne en mode à groupe unique.

Sans aucune organisation, la personne arrive dans une salle d'attente qui lui
explique comment demander un accès, et non sur un refus.

Le plan de contrôle saute entièrement l'étape 3 : la console n'a pas
d'organisation à faire choisir.

## Pour aller plus loin

La section **Authentification** détaille chacun de ces points : la politique de
mot de passe et la limitation des tentatives de connexion dans [Comptes
locaux](/docs/auth/local-accounts), puis le second facteur, les passkeys et
chaque autorité externe.

Qui peut passer, une fois que l'on sait qui appelle : [Contrôle
d'accès](/docs/concepts/access-control).
