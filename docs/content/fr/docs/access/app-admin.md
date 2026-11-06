---
title: Administrateur applicatif
section: Contrôle d'accès
order: 142
summary: Ce que la capacité app admin ouvre dans la console - les gens, leurs rôles, les pages servies - et ce qui reste hors de portée.
---

# Administrateur applicatif

Un compte qui porte la capacité **app admin** tient l'**identité de
l'application** : qui sont les comptes, les rôles qu'ils peuvent tenir, les
politiques sous lesquelles ils se connectent, et l'allure des pages de la gateway.
Il ne décide pas où vont les requêtes : c'est le rôle de
l'[administrateur infra](/docs/access/infra-admin).

La capacité est un indicateur du compte, posé dans **Application > Users** par un
autre administrateur applicatif ou par root. Voir
[Capacités d'administration](/docs/access/capabilities).

![La console d'un administrateur applicatif : Application, Vault, Data plane et Meerkat dans le rail, le menu Application ouvert sur Users](img/console/capability-app-admin.webp)

## Ce que vous voyez

La console s'ouvre sur **Application > General**. Le rail compte quatre entrées :
**Application**, **Vault**, **Data plane** et **Meerkat**.

| Section | Écrans |
|---|---|
| Application | General, Roles, Users, Built-in pages, Portal, Security, Access tokens - et Plug tant que le tunnel des développeurs est ouvert |
| Vault | les entrées de l'application |
| Data plane | Audit, Sessions, Scheduler, Notifications, Issues |
| Meerkat | Audit, API reference, Release notes, License |

Sur une installation à un seul tenant, ses **Groups**, **Members** et **Group
rules** sont aussi dans le menu Application : il n'y a qu'un tenant, et personne
n'a à le choisir. Avec plusieurs, ils appartiennent à chaque tenant, sous
**Tenants**.

## Ce que vous y faites

- **[General et Security](/docs/console/application)** : ce qu'est l'installation, les
  langues qu'elle parle, la politique de mots de passe, la double authentification,
  les passkeys, la durée des sessions.
- **[Users](/docs/console/users)** : les comptes et les capacités qu'ils portent - sauf
  root, que seul root accorde.
- **[Roles](/docs/console/roles)** : le catalogue global des rôles que demandent les routes.
- **[Built-in pages](/docs/console/built-in-pages)** et **[Portal](/docs/console/portal)** :
  l'allure des pages de connexion et de compte, et la barre qui fait passer d'une
  application à l'autre.
- **[Access tokens](/docs/console/access-and-agents)** : un jeton pour un script, dans
  les limites de votre capacité.
- **[Vault](/docs/console/vault)** : les valeurs et secrets auxquels l'application fait
  référence.
- **[Audit et Issues](/docs/console/audit-and-issues)** : qui s'est connecté, qui a changé
  quoi dans l'application, ce que les utilisateurs ont signalé. **Sessions** : qui est
  connecté, et fermer une session. **[Scheduler](/docs/operations/scheduler)** et
  **[Notifications](/docs/console/notifications)** : les appels que les services
  programment, et ce qu'ils ont dit à leurs utilisateurs.

## Ce qui n'est pas de votre ressort

| Vous ne voyez pas | C'est l'affaire de |
|---|---|
| Les routes, les autorités, TLS, le relais de mail, le modèle, les métriques, les logs | l'[administrateur infra](/docs/access/infra-admin) |
| Les écrans propres à un tenant, sur une installation qui en a plusieurs | les [administrateurs du tenant](/docs/access/tenant-admin) |
| La configuration (export, import, sauvegarde) et les sessions de la console | root |

Un écran que vous ne pouvez pas ouvrir vous renvoie sur General, même tapé dans la
barre d'adresse, et l'API d'administration répond `403` à ses appels.
