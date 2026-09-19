---
title: L'app-gateway de vos applications internes
section: Le produit
order: 0
layout: wide
hideNav: true
summary: Une porte devant vos applications internes. Elle prend en charge l'authentification, les règles d'accès, le routage, les quotas et l'audit, pour que vos services restent légers.
---

::: hero
# La sentinelle devant votre application

Meerkat est une **app-gateway** : un point d'entrée unique devant l'application
interne que vos équipes construisent, qui prend en charge tout ce qui n'est pas
leur métier. Authentification, règles d'accès, organisations, routage, quotas,
audit. Un binaire, zéro dépendance.

- [Démarrer](/docs/start/quick-start)
- [Voir](/showcase/index)
- [Ce qu'elle fait](/product/features)
:::

::: lead
Vos services reçoivent des requêtes déjà authentifiées, porteuses d'un jeton
signé avec une identité, des rôles et une organisation. Ils arrêtent d'embarquer
une page de connexion, un modèle de rôles et une table d'utilisateurs, et
redeviennent ce que vous vendez vraiment.
:::

::: cards
### Une porte, pas une pile

Installer la gateway, puis Prometheus, puis Grafana, puis écrire du YAML pour
tout : c'est le schéma que Meerkat existe pour casser. Les chiffres de trafic,
le journal d'audit, les quotas et la santé sont des écrans de la console, et
toute la passerelle est un binaire avec son stockage embarqué.

### L'identité fait partie du produit

Comptes, rôles, groupes, organisations et pages de connexion sont dans la
passerelle, pas à côté. Les annuaires d'entreprise - OpenID Connect, LDAP,
Active Directory, GitHub - authentifient ; ils ne décident jamais des rôles.

### Sans mot de passe d'abord

Les passkeys sont un facteur de plein droit, le TOTP retient un navigateur de
confiance, et une politique de mot de passe est un réglage plutôt qu'une
réécriture.

### Elle habille vos pages

Le bouton de compte, le portail de navigation, le thème clair et sombre et le
CSS par rôle sont injectés dans les pages que la passerelle proxifie. Votre
application les reçoit quel que soit son framework, et n'embarque aucune
bibliothèque pour cela.

### Éditée à chaud, jamais redémarrée

Onze prédicats décident qu'une requête est pour une route, trente-trois filtres
la transforment, et un changement s'applique à la requête suivante. Il n'y a
aucun fichier de configuration à redéployer.

### Pilotée par un agent

Le plan de contrôle expose un endpoint MCP. Un assistant lit l'installation et
la modifie selon exactement les règles qu'un humain reçoit : lecture seule reste
lecture seule.
:::

## À quoi cela ressemble

::: gallery
![L'écran des routes](img/console/routes-list.webp) Chaque route, ce qu'elle reconnaît et où elle va.
![L'éditeur de thème](img/console/built-in-pages-theme.webp) Les pages de connexion portent vos couleurs, en clair et en sombre.
![Le trafic](img/console/traffic.webp) Ce qui est passé, sans pile de métriques.
:::

[Ouvrir la galerie](/showcase/index)

## L'essayer

```bash
docker run -p 8080:8080 -p 9090:9090 \
  -e MEERKAT_ADMIN_PASSWORD=choisissez-en-un softwarity/meerkat
```

8080 est ce que vos utilisateurs atteignent, 9090 est la console
d'administration. Ensuite, [Démarrage rapide](/docs/start/quick-start) met un de
vos propres services derrière elle en cinq minutes.

::: cta
### Gratuite, et pas un essai

L'édition communautaire est toute la passerelle pour une organisation sur une
instance - en production, en entreprise, commercialement, sans rien payer.
Lisez le code, modifiez-le, embarquez-le dans votre propre produit ; deux ans
après chaque version, celle-ci devient de l'Apache 2.0 sans condition.

Enterprise est ce qu'il vous faut quand l'installation grandit : plusieurs
organisations, Active Directory, plusieurs passerelles derrière une seule
entrée. Jamais une primitive de sécurité : TLS, le second facteur, les
passkeys, le coffre et le journal d'audit sont gratuits et le restent.

[Comparer les éditions](/product/editions) . [Tarifs](/product/pricing)
:::
