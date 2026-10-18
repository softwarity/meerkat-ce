---
title: Remplacer un proxy d'authentification
section: Le produit
order: 6
summary: Passer une application interne d'oauth2-proxy, de Pomerium ou d'un forward-auth dans nginx ou Traefik à Meerkat - ce qui correspond à quoi, et les étapes.
---

# Remplacer un proxy d'authentification

Une application interne sans connexion à elle est d'ordinaire protégée par un
proxy d'authentification placé devant : oauth2-proxy, Pomerium, ou une règle de
forward-auth dans nginx (`auth_request`) ou Traefik (`ForwardAuth`) qui en
appelle un. Le proxy connecte la personne auprès d'un fournisseur d'identité et
passe son nom à l'application dans un en-tête.

Meerkat fait le même travail, et garde au même endroit ce qui entoure
d'habitude le proxy : les comptes et leurs rôles, la règle d'accès de chaque
application, le routage, les limites de débit et le journal d'audit.

## Ce qui correspond à quoi

| Devant l'application aujourd'hui | Dans Meerkat |
| --- | --- |
| Un proxy, ou une règle de forward-auth, par application | Une [route](/docs/concepts/routes) par application, toutes sur une même gateway |
| Le fournisseur d'identité auquel le proxy délègue | Le même fournisseur, par [OpenID Connect](/docs/auth/oidc) ; ou des comptes locaux, GitHub, et en Enterprise [LDAP](/docs/auth/ldap) et [SAML](/docs/auth/saml) |
| Domaines d'e-mail autorisés, groupes autorisés | La [règle d'accès](/docs/access/route-access) de la route : connecté, un rôle, une liste de personnes |
| L'en-tête qui porte le nom de l'utilisateur | **Identity** sur la route : chaque information sous le nom d'en-tête que l'application lit déjà |
| Un jeton que l'application vérifie | Un [JWT signé](/docs/auth/service-jwt), vérifié avec les clés publiées par la gateway |
| Des règles par chemin | Des [règles par endpoint](/docs/access/endpoint-security), lues dans la description OpenAPI de l'application |

Les en-têtes que pose Meerkat sont d'abord retirés de la requête entrante : un
appelant ne peut pas les falsifier à travers la gateway.

## Les étapes

1. **Lancer Meerkat à côté du proxy actuel.** Le
   [démarrage rapide](/docs/start/quick-start) le fait tourner en quelques
   minutes, avec Docker, Compose, Swarm ou Helm.
2. **Brancher les comptes.** Déclarer le fournisseur d'identité qu'utilise le
   proxy aujourd'hui, ou créer des comptes locaux. Les gens se connectent sur
   les pages de Meerkat, à vos couleurs.
3. **Ajouter une route pour l'application.** Un prédicat de chemin ou d'hôte,
   et l'adresse interne de l'application comme cible. Voir
   [votre première route](/docs/start/first-route).
4. **Dire qui passe.** Connecté, ou un rôle, dans la règle d'accès de la route.
5. **Transmettre l'identité.** Dans **Identity**, donner à chaque information
   le nom d'en-tête que l'application lit aujourd'hui, avec le mécanisme
   **Headers** ; ou envoyer un JWT signé.
   [Brancher une application](/docs/auth/connect-an-application) donne les
   recettes pour Grafana, GeoServer et d'autres.
6. **Basculer le trafic.** Faire pointer l'adresse publique de l'application
   vers le plan de données de Meerkat, et ne laisser l'application joignable
   que par la gateway.
7. **Retirer l'ancien proxy.**

> [!WARNING]
> Un en-tête est cru parce que personne d'autre ne peut le poser. Gardez
> l'application hors de tout autre chemin - pas de port publié, pas d'ingress à
> elle - sinon quiconque l'atteint directement peut envoyer l'en-tête lui-même.

## Ce qui change pour l'application

Rien, quand elle lit les mêmes en-têtes qu'avant. Ce qui change est autour
d'elle : sa règle d'accès, ses rôles et son trafic sont dans la console avec
ceux des autres applications, et chaque modification est dans le
[journal d'audit](/docs/operations/audit).

Voir les [éditions](/product/editions) pour ce que comprend l'image gratuite.
