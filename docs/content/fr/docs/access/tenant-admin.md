---
title: Administrateur de tenant
section: Contrôle d'accès
order: 143
summary: Ce qu'administrer un tenant ouvre dans la console - ses groupes, ses membres et ses règles, son audit et ses sessions - et rien des autres.
---

# Administrateur de tenant

Administrer un tenant n'est pas une capacité du compte : on administre un tenant
quand on en est le **propriétaire**, ou qu'on y tient une adhésion **ADMIN**
active - voir [Tenants](/docs/access/tenants). La console le déduit de là, et ne
montre que les tenants concernés.

C'est une fonction de l'édition Enterprise : l'image communautaire sert un seul
tenant, dont les groupes et les membres relèvent de
l'[administrateur applicatif](/docs/access/app-admin).

![La console d'un administrateur de tenant : Tenants, Data plane et Meerkat dans le rail, le menu du tenant ouvert sur Members](img/console/capability-tenant-admin.webp)

## Ce que vous voyez

La console s'ouvre sur votre tenant. Le rail compte trois entrées : **Tenants**,
**Data plane** et **Meerkat**.

| Section | Écrans |
|---|---|
| Tenants | pour chaque tenant que vous administrez : General, Groups, Members, Group rules, Notifications, Danger zone |
| Data plane | Audit, Sessions, Issues - de vos tenants |
| Meerkat | Audit, Release notes, License |

## Ce que vous y faites

- **[General](/docs/console/tenants)** : le nom du tenant, ses horaires d'accès, la
  durée de session de ses membres.
- **[Groups, Members et Group rules](/docs/console/organisation)** : qui appartient au
  tenant, les groupes qui portent leurs rôles, et les règles qui remplissent ces
  groupes à partir d'un annuaire.
- **[Notifications](/docs/console/notifications)** : si les gens du tenant les reçoivent,
  et ce qui leur a été envoyé.
- **Danger zone** : les actes sur lesquels on ne revient pas.
- **[Audit et Issues](/docs/console/audit-and-issues)** et **Sessions** : qui s'est
  connecté à votre tenant, ce qui y a changé, ce que ses gens ont signalé, qui est
  connecté maintenant.

## Ce qui n'est pas de votre ressort

| Vous ne voyez pas | C'est l'affaire de |
|---|---|
| Un autre tenant, même par son adresse | ses propres administrateurs |
| Les rôles qu'un groupe peut tenir : le catalogue global | l'[administrateur applicatif](/docs/access/app-admin) |
| Les comptes en tant que tels, les pages servies, les politiques de sécurité | l'administrateur applicatif |
| Les routes, les autorités, TLS, les métriques, les logs | l'[administrateur infra](/docs/access/infra-admin) |
| Le Vault | root et les deux administrateurs ci-dessus |

Les écrans d'un autre tenant vous renvoient vers le vôtre, même tapés dans la barre
d'adresse, et l'API d'administration répond `403` à leurs appels.

## Un compte qui n'administre rien

Connecté à la console sans capacité ni tenant à administrer, un compte ne voit que
**Meerkat** : les notes de version et la licence.

![La console d'un compte qui n'administre rien : Meerkat seul dans le rail, l'écran License](img/console/capability-none.webp)
