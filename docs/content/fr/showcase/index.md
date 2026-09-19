---
title: Galerie
section: Galerie
order: 1
layout: wide
summary: À quoi ressemble la passerelle, de la page de connexion que rencontrent vos utilisateurs à chaque écran où travaille un exploitant.
---

::: hero
# Voir

Deux surfaces, et toutes les deux sont livrées avec le produit. D'un côté, les
pages que rencontrent vos utilisateurs - connexion, barre de navigation, menu
de compte - qui portent déjà vos couleurs. De l'autre, la console dans laquelle
vit un exploitant.

- [Démarrer](/docs/start/quick-start)
- [Ce qu'elle fait](/product/features)

::: figure meerkat
:::
:::

## Ce que rencontrent vos utilisateurs

Ces pages sont servies par la passerelle elle-même. L'application derrière
n'embarque pas une ligne de code pour cela : les couleurs, le logo et la
navigation viennent de la console.

::: gallery
![La page de connexion](img/app/signin-light.webp) La page de connexion, avec votre nom, votre logo et votre palette. Les passkeys sont à côté du mot de passe, pas derrière une option.
![La page de connexion en sombre](img/app/signin-dark.webp) La même page en sombre, avec sa propre image de fond si vous en voulez une.
![Le portail de navigation](img/app/portal-light.webp) Une barre unique au travers de toutes les applications servies, filtrée par ce que le visiteur a le droit d'ouvrir.
![Le portail de navigation en sombre](img/app/portal-dark.webp) Le thème clair ou sombre du visiteur est porté dans l'application proxifiée, même quand celle-ci a le sien.
![Le menu de compte](img/app/user-button.webp) Le bouton de compte : qui vous êtes, la langue, la bascule clair et sombre, et la sortie.
![Une route en maintenance](img/app/maintenance.webp) Une route fermée exprès répond une page, avec une raison, plutôt qu'un 503 que personne ne peut lire.
:::

## Le routage

::: gallery
![L'écran des routes](img/console/routes-list.webp) Chaque route dans l'ordre, ce qu'elle reconnaît, où elle va, et si elle est active.
![La cible d'une route](img/console/route-editor-target.webp) L'amont, et ce que la passerelle propose pour le remplir.
![Les prédicats](img/console/route-editor-predicates.webp) Les conditions qu'une requête doit remplir pour que cette route la prenne.
![Les filtres](img/console/route-editor-filters.webp) Ce qui arrive à la requête, et à la réponse, au passage.
![La sécurité par endpoint](img/console/route-editor-security.webp) Une règle par opération, lue dans la description OpenAPI du service.
:::

## L'identité et les accès

::: gallery
![Les utilisateurs](img/console/users.webp) Les comptes, ce qu'ils peuvent administrer, et les champs que vous avez décidé de demander.
![Les rôles](img/console/roles.webp) Un catalogue hiérarchique : un rôle qui en hérite d'un autre obtient tout ce que celui-ci ouvre.
![Les groupes](img/console/groups.webp) Des groupes par organisation, pour qu'un rôle s'accorde par appartenance plutôt qu'un par un.
![Les membres](img/console/members.webp) Qui appartient à une organisation, et à quel titre.
![Les autorités](img/console/auth-providers.webp) OpenID Connect, LDAP, Active Directory, GitHub. Elles authentifient ; elles ne décident jamais des rôles.
![Les jetons d'API](img/console/access-tokens.webp) Des jetons avec un plan et un périmètre, et un secret montré exactement une fois.
:::

## L'exploiter

::: gallery
![Le trafic](img/console/traffic.webp) Ce qui est passé, par route, avec les échecs distingués du silence.
![Les métriques](img/console/metrics.webp) Latence et classes de statut sur la dernière heure, sans pile de métriques à installer.
![Le journal d'audit](img/console/audit.webp) Chaque changement d'administration, avec son auteur et un diff champ par champ.
![Le coffre](img/console/vault.webp) Des secrets scellés au repos et des valeurs en clair, les deux référencés par leur nom.
![TLS](img/console/tls.webp) Les certificats, leurs noms et leur expiration, émis ou déposés.
![La configuration](img/console/configuration.webp) Toute l'installation en un document, à exporter et à rejouer ailleurs.
:::

## La faire vôtre

::: gallery
![Le thème](img/console/built-in-pages-theme.webp) Une palette construite à partir d'une couleur source, en clair et en sombre, appliquée à toutes les pages servies.
![La marque](img/console/built-in-pages-branding.webp) Le nom, le logo, la signature et l'image de fond.
![L'éditeur de portail](img/console/portal.webp) La barre de navigation, arrangée : modules, sous-modules, icônes, et un aperçu en direct.
![Les réglages généraux](img/console/general.webp) Ce qu'est cette installation, et les politiques sous lesquelles vit chaque compte.
![Le point d'entrée agent](img/console/mcp.webp) MCP sur le plan de contrôle, pour qu'un assistant travaille sous les mêmes règles.
:::
