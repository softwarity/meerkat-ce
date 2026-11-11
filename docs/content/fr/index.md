---
title: Proxy d'authentification et API gateway pour vos applications internes
summary: Meerkat place la connexion, le contrôle d'accès, les tenants, le routage, les quotas et l'audit devant vos applications internes, à la place d'un proxy d'authentification, d'un serveur d'identité et d'une API gateway. Un seul binaire, aucune dépendance.
layout: wide
---

::: hero
# La sentinelle à la porte de votre application

Meerkat est un **proxy d'authentification et une API gateway** qui place le
contrôle d'accès devant les applications internes que construisent vos équipes.
Il remplace les oauth2-proxy, Keycloak et Kong que l'on assemble d'ordinaire
autour d'elles : connexion, règles d'accès, tenants, routage, quotas,
audit. Une seule image, aucune dépendance.

- [Démarrer](/docs/start/quick-start)
- [La voir](/showcase/index)
- [Ce qu'elle fait](/product/features)

::: figure meerkat
:::
:::

::: stats
### 1 image
à déployer
### 70 Mo
sur disque
### {{memory.idle}} Mo
en mémoire, au repos
### 0
dépendance
:::

Et elle est rapide : mesurée à côté de Kong, APISIX et Traefik, chacun sur le
même CPU, avec [des chiffres](/product/performance) tenus à jour par la CI.

::: lead
Vos services reçoivent des requêtes déjà authentifiées, accompagnées d'un jeton
signé qui porte une identité, des rôles et un tenant. Ils n'ont plus à
fournir une page de connexion, un modèle de rôles ni une table d'utilisateurs,
et redeviennent ce que vous vendez réellement.

Vous la comparez à ce qu'il vous faudrait assembler sans elle ?
[Le dossier Meerkat](/product/the-case) fait le calcul : huit produits ou un
seul, trente-huit pods ou un seul, et ce que coûte le même socle en licences et
en jours d'ingénierie.
:::

![Les utilisateurs atteignent les services à travers Meerkat, qui les connecte, route leurs requêtes et garde la trace de ce qui s'est passé](img/readme/app-gateway.gif)

::: cards
### Tout le parcours du compte, déjà écrit

La connexion, l'inscription, le mot de passe oublié, la double authentification,
les passkeys, les pages du compte, le choix du tenant, des refus qui disent
pourquoi : une quarantaine de pages qu'une gateway d'API ne sert pas, servies
ici en 22 langues et à vos couleurs. Une application derrière Meerkat n'en écrit
[aucune](/docs/customise/built-in-pages).

### Une porte, pas une stack

Installer la gateway, puis Prometheus, puis Grafana, puis écrire du YAML
pour tout : Meerkat existe pour rompre avec ce schéma. Les chiffres de trafic,
le journal d'audit, les règles de quotas et la santé des services sont des
écrans de la console, et toute la gateway tient dans un binaire au stockage embarqué.

### L'identité fait partie du produit

Les comptes, les rôles, les groupes, les tenants et les pages de
connexion sont dans la gateway, pas à côté. L'annuaire de votre entreprise -
OpenID Connect, GitHub, et avec Enterprise SAML, LDAP ou Active Directory -
connecte les utilisateurs, et en Enterprise ses groupes peuvent accorder des
rôles.

### Une connexion solide, intégrée

Les passkeys comme premier facteur, une application d'authentification qui se
souvient d'un navigateur de confiance, et une politique de mots de passe qui se
règle au lieu de se réécrire.

### Elle habille vos pages

Le bouton de compte et son menu des applications, le mode clair ou sombre et le
CSS par rôle sont ajoutés aux pages que sert la gateway - et avec Enterprise,
une barre de navigation entre vos applications. Votre
application en bénéficie quel que soit le langage dans lequel elle est écrite,
et n'embarque aucune bibliothèque pour cela.

### L'authentification unique de vos autres outils

La console d'un broker, une interface de base de données, un tableau de bord :
toute application qui parle OpenID Connect connecte ses utilisateurs par la
gateway, qui devient leur fournisseur OpenID - sans Keycloak à côté. La route
décide qui entre et les rôles que portent les jetons, et une connexion s'essaie
pour n'importe qui avant qu'il ne la tente.
[Enterprise](/docs/auth/oidc-provider).

### Modifiée à chaud, jamais redémarrée

Onze prédicats décident qu'une requête relève d'une route, trente-quatre filtres
la transforment, et un changement s'applique dès la requête suivante. Il n'y a
pas de fichier de configuration à redéployer.

### Pilotée par un agent

Le plan de contrôle expose un endpoint MCP. Un assistant lit l'installation et
la modifie en suivant exactement les règles imposées à un humain : la lecture
seule reste la lecture seule. Et il pilote vos propres API : il lit l'OpenAPI
d'une route, puis appelle le service à travers la gateway, sous n'importe quel
rôle, pour faire le travail ou prouver une règle.
:::

## À quoi elle ressemble

::: gallery
![L'écran des routes](img/console/routes-list.webp) Chaque route, ce qu'elle reconnaît et où elle mène.
![L'éditeur de thème](img/console/built-in-pages-theme.webp) Les pages de connexion prennent vos couleurs, en clair comme en sombre.
![Le trafic](img/console/traffic.webp) Ce qui est passé, sans outillage de métriques à installer.
:::

[Ouvrir la galerie](/showcase/index)

## L'essayer

L'édition Community, gratuite et utilisable en production :

```bash
docker run -p 8080:8080 -p 9090:9090 \
  -e MEERKAT_ADMIN_PASSWORD=choose-one softwarity/meerkat-ce
```

Tout ce que fait Meerkat, en évaluation jusqu'à l'ajout d'une licence :

```bash
docker run -p 8080:8080 -p 9090:9090 \
  -e MEERKAT_ADMIN_PASSWORD=choose-one softwarity/meerkat
```

Le port 8080 est celui qu'atteignent vos utilisateurs, le 9090 celui de la
console d'administration. L'évaluation porte une mention sur ses pages ; une
licence Team ou Enterprise la retire, sans redémarrage. À partir de là, le
[démarrage rapide](/docs/start/quick-start) place l'un de vos services derrière
la gateway en cinq minutes environ.

::: cta
### Gratuite, et ce n'est pas une version d'essai

L'édition Community est toute la gateway pour un tenant sur une
instance - en production, en entreprise, pour un usage commercial, sans rien
payer. Elle est source-available, sous Functional Source License : lisez le
code, modifiez-le, livrez-le dans votre propre produit ; deux
ans après sa publication, chaque version passe sous licence Apache 2.0.

Enterprise est ce qu'il vous faut dès que l'installation grandit : plusieurs
tenants, SAML, LDAP et Active Directory, le fournisseur OpenID de vos autres
outils, plusieurs instances de la gateway derrière un même point d'entrée. Jamais une brique de sécurité : TLS, le second facteur, les passkeys,
le coffre et le journal d'audit sont gratuits et le resteront. Team, c'est
Enterprise pour un tenant sur une instance, et l'édition d'évaluation, c'est
Enterprise avec une mention affichée, gratuite, pour tout essayer d'abord.

[plug](https://softwarity.github.io/plug/), le tunnel développeur de
Softwarity, est publié à part et tourne à côté de n'importe quelle édition ;
l'image Enterprise embarque son agent, et la gateway sait qui est branché.

[Comparer les éditions](/product/editions) . [Tarifs](/pricing/index)
:::
