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
![La page de connexion](img/app/signin-light.webp) La page de connexion, avec votre nom, votre logo et votre palette. Les passkeys sont à côté du mot de passe, pas cachées derrière une option.
![La page de connexion en mode sombre](img/app/signin-dark.webp) La même page en mode sombre, avec sa propre image de fond si vous le souhaitez.
![Le portail de navigation](img/app/portal-light.webp) Une seule barre pour toutes les applications que sert la gateway, filtrée selon ce que le visiteur a le droit d'ouvrir.
![Le portail de navigation en mode sombre](img/app/portal-dark.webp) Le mode clair ou sombre choisi par le visiteur est appliqué à l'application relayée, même si elle a le sien.
![Le menu du compte](img/app/user-button.webp) Le bouton de compte : qui vous êtes, la langue, le passage du clair au sombre, et la déconnexion.
![Une route en maintenance](img/app/maintenance.webp) Une route fermée volontairement répond par une page qui donne une raison, plutôt que par une erreur 503 que personne ne sait lire.
:::

## Le routage

::: gallery
![L'écran des routes](img/console/routes-list.webp) Toutes les routes dans l'ordre, ce que chacune reconnaît, où elle mène, et si elle est active.
![La cible d'une route](img/console/route-editor-target.webp) L'upstream, et ce que la gateway propose pour le renseigner.
![Les prédicats](img/console/route-editor-predicates.webp) Les conditions qu'une requête doit remplir pour que cette route la prenne.
![Les filtres](img/console/route-editor-filters.webp) Ce qui arrive à la requête, puis à la réponse, pendant leur passage.
![La sécurité par endpoint](img/console/route-editor-security.webp) Une règle par opération, lue dans la description OpenAPI du service lui-même.
:::

## L'identité et les accès

::: gallery
![Les utilisateurs](img/console/users.webp) Les comptes, ce qu'ils peuvent administrer, et les champs que vous avez choisi de demander.
![Les rôles](img/console/roles.webp) Un catalogue hiérarchique : un rôle qui hérite d'un autre obtient tout ce que cet autre ouvre.
![Les groupes](img/console/groups.webp) Des groupes par organisation, pour accorder un rôle par appartenance plutôt que compte par compte.
![Les membres](img/console/members.webp) Qui appartient à une organisation, et à quel titre.
![Les autorités](img/console/auth-providers.webp) OpenID Connect, LDAP, Active Directory, GitHub. Elles authentifient ; elles ne décident jamais des rôles.
![Les jetons d'API](img/console/access-tokens.webp) Des jetons dotés d'un plan et d'un périmètre, et un secret affiché une fois, pas deux.
:::

## L'exploiter

::: gallery
![Le trafic](img/console/traffic.webp) Le trafic en direct, sans rien installer : requêtes par seconde, latence moyenne et p95, refus distingués des échecs, et les routes classées selon la plus lente, celle qui échoue le plus ou la plus coûteuse.
![Le journal d'audit](img/console/audit.webp) Chaque changement d'administration, avec son auteur et les différences champ par champ.
![Le coffre](img/console/vault.webp) Des secrets scellés au repos et des valeurs en clair, tous référencés par leur nom.
![TLS](img/console/tls.webp) Les certificats, leurs noms et leur date d'expiration, qu'ils soient émis ou importés.
![La configuration](img/console/configuration.webp) Toute l'installation en un seul document, à exporter et à rejouer ailleurs.
:::

## Vous l'approprier

::: gallery
![Le thème](img/console/built-in-pages-theme.webp) Une palette construite à partir d'une seule couleur de base, en clair et en sombre, appliquée à toutes les pages que sert la gateway.
![La marque](img/console/built-in-pages-branding.webp) Le nom, le logo, le slogan et l'image de fond.
![L'éditeur du portail](img/console/portal.webp) La barre de navigation, organisée : modules, sous-modules, icônes, avec un aperçu en direct.
![Les réglages généraux](img/console/general.webp) Ce qu'est cette installation, et les politiques auxquelles chaque compte est soumis.
![Le point d'entrée des agents](img/console/mcp.webp) MCP sur le plan de contrôle, pour qu'un assistant travaille en suivant les mêmes règles.
:::
