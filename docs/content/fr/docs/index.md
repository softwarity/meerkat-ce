---
title: Documentation
section: Démarrer
order: 0
summary: Tout ce que fait Meerkat, comment l'installer, et à quoi servent chaque écran, chaque prédicat et chaque filtre.
---

# Documentation

Meerkat est un proxy d'authentification et une API gateway : une porte unique
devant vos applications internes. Elle se charge de l'authentification, des règles d'accès, du routage
et de l'observation, pour que vos services n'aient plus à s'en occuper.

## Par où commencer

- **[Ce que fait Meerkat](/docs/capabilities)** - chaque capacité, ce qu'elle demande sans Meerkat, et ce qui vient ensuite.
- **[Ce qu'est Meerkat](/product/what-is-meerkat)** - ce que fait le produit, en deux minutes.
- **[Démarrage rapide](/docs/start/quick-start)** - une gateway en marche et une première application derrière elle.
- **[Concepts](/docs/concepts/architecture)** - les trois plans, les routes, l'identité, les accès.

## Qui administre quoi

Trois administrateurs se partagent la console, chacun sur une partie, et chacun ne voit que la sienne :

- **[Administrateur infra](/docs/access/infra-admin)** - le plan de routage : routes, certificats, autorités, relais mail.
- **[Administrateur applicatif](/docs/access/app-admin)** - les personnes : comptes, rôles, réglages, pages que sert la gateway.
- **[Administrateur de tenant](/docs/access/tenant-admin)** - un tenant : ses membres, ses groupes, ses réglages.

[Capacités d'administration](/docs/access/capabilities) dit comment on les donne, et la
[galerie](/showcase/index) montre la console de chacun.

## Référence

- **[Prédicats](/docs/predicates/overview)** - les onze façons dont une route décide qu'une requête lui est destinée.
- **[Filtres](/docs/filters/overview)** - les trente-quatre façons dont elle la transforme.
- **[La console](/docs/console/overview)** - chaque écran, et ce à quoi il sert.

## Au quotidien

- **[Exploitation](/docs/operations/overview)** - trafic, audit, sauvegardes, montées de version.
- **[Déployer](/docs/deploy/one-gateway)** - Docker, Helm, et ce que règle chaque variable.
- **[Cluster Kubernetes](/docs/deploy/kubernetes)** - plusieurs instances de la gateway derrière un seul point d'entrée.

> [!TIP]
> La recherche de la barre supérieure parcourt toutes les pages, dans la langue
> de lecture. Appuyez sur / pour l'ouvrir.
