---
title: Galerie
section: Galerie
order: 1
layout: wide
summary: À quoi ressemble la gateway, de la page de connexion que voient vos utilisateurs à chacun des écrans où travaille un exploitant.
---

::: hero
# La voir

Deux faces, toutes deux livrées avec le produit. D'un côté, les pages que
voient vos utilisateurs - la connexion, la barre de navigation, le menu du
compte - déjà à vos couleurs. De l'autre, la console dans laquelle un
exploitant passe ses journées.

- [Démarrer](/docs/start/quick-start)
- [Ce qu'elle fait](/product/features)

::: figure meerkat
:::
:::

## Ce que voient vos utilisateurs

Ces pages sont servies par la gateway elle-même. L'application qui se
trouve derrière ne contient pas une ligne de code pour cela : les couleurs, le
logo et la navigation viennent de la console.

::: gallery
![La page de connexion](img/showcase/signin-light.webp) La page de connexion, avec votre nom, votre logo et votre palette. Les passkeys sont à côté du mot de passe, pas cachées derrière une option.
![La page de connexion en mode sombre](img/showcase/signin-dark.webp) La même page en mode sombre, avec sa propre image de fond si vous le souhaitez.
![Le portail de navigation](img/showcase/portal-light.webp) Une seule barre pour toutes les applications que sert la gateway, filtrée selon ce que le visiteur a le droit d'ouvrir.
![Le portail de navigation en mode sombre](img/showcase/portal-dark.webp) Le mode clair ou sombre choisi par le visiteur est appliqué à l'application relayée, même si elle a le sien.
![Le menu du compte](img/showcase/user-button.webp) Le bouton de compte : qui vous êtes, la langue, le passage du clair au sombre, et la déconnexion.
![Une route en maintenance](img/showcase/maintenance.webp) Une route fermée volontairement répond par une page qui donne une raison, plutôt que par une erreur 503 que personne ne sait lire.
:::

## Le routage

::: gallery
![L'écran des routes](img/showcase/routes-list.webp) Toutes les routes dans l'ordre, ce que chacune reconnaît, où elle mène, et si elle est active.
![La cible d'une route](img/showcase/route-editor-target.webp) L'upstream, et ce que la gateway propose pour le renseigner.
![Les prédicats](img/showcase/route-editor-predicates.webp) Les conditions qu'une requête doit remplir pour que cette route la prenne.
![Les filtres](img/showcase/route-editor-filters.webp) Ce qui arrive à la requête, puis à la réponse, pendant leur passage.
![La sécurité par endpoint](img/showcase/route-editor-security.webp) Une règle par opération, lue dans la description OpenAPI du service lui-même.
:::

## L'identité et les accès

::: gallery
![Les utilisateurs](img/showcase/users.webp) Les comptes, ce qu'ils peuvent administrer, et les champs que vous avez choisi de demander.
![Les rôles](img/showcase/roles.webp) Un catalogue hiérarchique : un rôle qui hérite d'un autre obtient tout ce que cet autre ouvre.
![Les groupes](img/showcase/groups.webp) Des groupes par organisation, pour accorder un rôle par appartenance plutôt que compte par compte.
![Les membres](img/showcase/members.webp) Qui appartient à une organisation, et à quel titre.
![Les autorités](img/showcase/auth-providers.webp) OpenID Connect, LDAP, Active Directory, GitHub. Elles authentifient ; elles ne décident jamais des rôles.
![Les jetons d'API](img/showcase/access-tokens.webp) Des jetons dotés d'un plan et d'un périmètre, et un secret affiché une fois, pas deux.
:::

## L'exploiter

::: gallery
![Le trafic](img/showcase/traffic.webp) Le trafic en direct, sans rien installer : requêtes par seconde, latence moyenne et p95, refus distingués des échecs, et les routes classées selon la plus lente, celle qui échoue le plus ou la plus coûteuse.
![Le journal d'audit](img/showcase/audit.webp) Chaque changement d'administration, avec son auteur et les différences champ par champ.
![Le coffre](img/showcase/vault.webp) Des secrets scellés au repos et des valeurs en clair, tous référencés par leur nom.
![Domains & TLS](img/showcase/tls.webp) Les certificats, leurs noms et leur date d'expiration, qu'ils soient émis ou importés.
![La configuration](img/showcase/configuration.webp) Toute l'installation en un seul document, à exporter et à rejouer ailleurs.
:::

## Vous l'approprier

::: gallery
![Le thème](img/showcase/built-in-pages-theme.webp) Une palette construite à partir d'une seule couleur de base, en clair et en sombre, appliquée à toutes les pages que sert la gateway.
![La marque](img/showcase/built-in-pages-branding.webp) Le nom, le logo, le slogan et l'image de fond.
![L'éditeur du portail](img/showcase/portal.webp) La barre de navigation, organisée : modules, sous-modules, icônes, avec un aperçu en direct.
![Les réglages généraux](img/showcase/general.webp) Ce qu'est cette installation, et les politiques auxquelles chaque compte est soumis.
![Le point d'entrée des agents](img/showcase/mcp.webp) MCP sur le plan de contrôle, pour qu'un assistant travaille en suivant les mêmes règles.
:::
