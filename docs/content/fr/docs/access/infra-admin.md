---
title: Administrateur infra
section: Contrôle d'accès
order: 141
summary: Ce que la capacité infra admin ouvre dans la console - le plan de routage, ses certificats et ses autorités - et ce qui reste hors de portée.
---

# Administrateur infra

Un compte qui porte la capacité **infra admin** tient le **plan de routage** : ce
que la gateway répond, pour quelles adresses, derrière quels certificats, avec
quelles autorités de connexion. Il ne décide ni de qui sont les gens ni de l'allure
des pages : c'est le rôle de l'[administrateur applicatif](/docs/access/app-admin).

La capacité est un indicateur du compte, posé par un administrateur applicatif ou
par root dans **Application > Users**. Voir
[Capacités d'administration](/docs/access/capabilities).

![La console d'un administrateur infra : Infra, Vault, Data plane et Meerkat dans le rail, le menu Infra ouvert sur Routes](img/console/capability-infra-admin.webp)

## Ce que vous voyez

La console s'ouvre sur **Infra > Routes**. Le rail compte quatre entrées :
**Infra**, **Vault**, **Data plane** et **Meerkat**.

| Section | Écrans |
|---|---|
| Infra | Topology, Routes, Endpoint security, Endpoint rate limits, Endpoint audit, Authentication, Mail relay, Domains & TLS, Access tokens, MCP, Model, OpenTelemetry, Plug |
| Vault | les entrées du plan de routage |
| Data plane | Metrics, Issues |
| Meerkat | Audit, Logs, API reference, Release notes, License |

## Ce que vous y faites

- **[Topology](/docs/console/topology)** : les instances qui servent l'installation,
  la base qu'elles partagent, les services derrière les routes et s'ils répondent.
- **[Routes](/docs/console/routes)** : la table de routage et l'éditeur de route -
  où va une requête, qui peut l'atteindre, les filtres en chemin, le testeur.
- **[Endpoint security, rate limits et audit](/docs/console/endpoints)** : qui peut
  appeler chaque opération d'une API, combien elle peut porter, et ce qui a été appelé.
- **[Authentication](/docs/console/authentication)** : les autorités par lesquelles
  on se connecte - les comptes locaux, OIDC, LDAP et Active Directory, GitHub.
- **[Mail relay](/docs/console/mail-relay)** : le serveur SMTP par lequel partent les
  mails des comptes.
- **[Domains & TLS](/docs/console/tls)** : les noms publics et leurs certificats, sur
  chaque plan.
- **[Access tokens et MCP](/docs/console/access-and-agents)** : un jeton pour un
  script, une connexion pour un assistant - dans les limites de votre capacité.
- **Model** : les champs supplémentaires d'un tenant. **OpenTelemetry** : où partent
  les traces. **Plug** : l'ouverture du tunnel des développeurs.
- **[Vault](/docs/console/vault)** : les secrets et valeurs auxquels le plan de
  routage fait référence.
- **[Metrics](/docs/console/traffic)** et **[Issues](/docs/console/audit-and-issues)** :
  ce qui est passé par la gateway, et ce que les utilisateurs ont signalé.
- **Logs** et **Audit** : ce que la gateway dit d'elle-même, et ce qui a été changé
  sur votre plan.

## Ce qui n'est pas de votre ressort

| Vous ne voyez pas | C'est l'affaire de |
|---|---|
| Les comptes, les rôles, les pages servies, le portail, les politiques de sécurité | l'[administrateur applicatif](/docs/access/app-admin) |
| Les groupes, membres et règles d'un tenant | les [administrateurs du tenant](/docs/access/tenant-admin) |
| Les sessions, le scheduler, les notifications | l'administrateur applicatif |
| La configuration (export, import, sauvegarde) et les sessions de la console | root |

Un écran que vous ne pouvez pas ouvrir vous renvoie sur Routes, même tapé dans la
barre d'adresse, et l'API d'administration répond `403` à ses appels : masquer un
lien est un confort, c'est l'API qui fait la règle.
