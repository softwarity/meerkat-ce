---
title: Identité
section: Concepts
order: 22
summary: Les trois façons dont la gateway sait qui appelle - une session, un jeton d'API, ou rien du tout - et d'où viennent les comptes.
---

# Identité

Avant de décider si une requête a le droit de passer, la gateway établit qui la
fait. Il y a exactement deux réponses qu'elle peut trouver, et un troisième cas
qu'elle traite comme tel.

| | Ce que c'est | Qui s'en sert |
|---|---|---|
| Session | un jeton opaque dans un cookie httpOnly | une personne dans un navigateur |
| Jeton d'API | `Authorization: Bearer mk_...` | un script, un service, un agent |
| Anonyme | ni l'un ni l'autre | tout ce qu'une route laisse passer sans rien demander |

## Les sessions

Une session est une ligne en base ; le navigateur détient un jeton dont c'est
l'empreinte qui est stockée. Détruire une session détruit la ligne : un cookie de
déconnexion n'est donc pas seulement oublié, il ne correspond plus à rien.

Chaque plan a son cookie, `MEERKAT_SESSION` sur le plan de données et
`MEERKAT_ADMIN_SESSION` sur le plan de contrôle, et chaque session porte
l'estampille de son plan. Un jeton de l'un est refusé sur l'autre.

La durée mesure l'**inactivité**, pas le temps depuis la connexion : toute requête
portant une session vivante repousse son échéance. Le défaut est de 30 minutes, et
il se résout depuis le membre, puis l'organisation, puis l'installation.

> [!NOTE]
> Cette résolution existe, et seule la valeur globale est éditable dans la console
> aujourd'hui. Les surcharges par organisation et par membre sont stockées mais
> n'ont pas encore d'écran.

Une session porte aussi ce que le flux de connexion a tranché : l'organisation
active, le groupe actif, et l'étape qui reste due. Une session bloquée sur une
étape y est renvoyée à chaque navigation jusqu'à ce qu'elle soit franchie.

> [!WARNING]
> Le cookie de session ne porte aucun attribut `Domain` : il n'est donc pas
> partagé entre sous-domaines. Des applications servies sous des noms d'hôte
> différents ne partagent pas de session aujourd'hui.

## Les jetons d'API

Un jeton est émis une fois et sa valeur en clair est affichée une fois. Il est
préfixé `mk_`, et se présente à l'endroit standard :

```bash
curl -H 'Authorization: Bearer mk_...' https://apps.exemple.fr/billing/invoices
```

Un jeton appartient à un **plan**, et les deux sont étanches : un jeton du plan de
données n'ouvre jamais le port d'administration, et un jeton d'administration ne
passe jamais sur le plan de données. Les jetons d'administration ne sont émis que
par un administrateur global, depuis la console ; une personne émet son propre
jeton du plan de données sur `/profile/tokens`.

Un jeton ne peut jamais porter **plus** que son propriétaire. Trois axes le
rétrécissent : une portée (lecture seule, complète, ou métriques seulement), un
domaine (le côté routage, le côté application, ou les deux), et une liste de
réseaux clients - jugée sur l'adresse réelle du pair, jamais sur un en-tête de
transmission.

> [!NOTE]
> Les jetons du plan de données ne portent pas encore de périmètre : ils sont émis
> en portée complète, et ils capturent silencieusement le groupe qu'avait la
> session. Ils n'ont pas non plus d'écran de console - la page de self-service est
> le seul endroit.

## D'où viennent les comptes

Un compte est reconnu par une **autorité**, et la table des comptes locaux est une
autorité parmi les autres. Un seul écran, **Infra > Authentication**, les liste
toutes et répond à la question *par quoi peut-on se connecter ici*.

| Autorité | État |
|---|---|
| Comptes locaux (mot de passe) | livré |
| OIDC - n'importe quel fournisseur conforme : Keycloak, Entra ID, Okta | livré |
| GitHub | livré |
| LDAP / Active Directory | livré, édition Enterprise |
| SAML 2.0 | pas écrit - le genre peut être enregistré, et il est refusé à l'usage |
| Kerberos / SPNEGO | pas écrit |

Une autorité externe répond du **premier facteur seulement**. Elle dit que c'est
bien cette personne ; elle ne décide jamais de ce qu'elle a le droit de faire ici -
cela appartient à la gateway, et cela vient des rôles, des groupes et des
appartenances.

Plusieurs réglages sont par autorité, avec un troisième état qui hérite du réglage
global : si un second facteur est exigé, si les passkeys sont permises, et si une
personne inconnue qui se connecte obtient un compte créé pour elle.

## Le flux de connexion

Les pages sont servies par la gateway, sur le plan de données, dans la langue du
visiteur et le thème de l'installation. L'ordre est fixe :

1. **Mise à jour du mot de passe**, s'il est temporaire ou expiré.
2. **Second facteur** : un code TOTP si le compte est enrôlé, sauf si ce navigateur a été marqué de confiance ; enrôlement forcé si le MFA est exigé et qu'il n'y a pas encore de facteur.
3. **Organisation**, puis **groupe** quand l'organisation travaille en mode groupe unique.

Sans aucune organisation, la personne atterrit dans une salle d'attente qui
explique comment demander un accès, plutôt que sur un refus.

Le plan de contrôle saute entièrement l'étape 3 : la console n'a pas
d'organisation à choisir.

## Pour aller plus loin

La section **Authentification** détaille chacun de ces points - la politique de
mot de passe et l'étranglement des tentatives dans [Comptes
locaux](/#/docs/auth/local-accounts), puis le second facteur, les passkeys et
chaque autorité externe.

Qui a le droit de passer, une fois qu'on sait qui c'est : [Contrôle
d'accès](/#/docs/concepts/access-control).
