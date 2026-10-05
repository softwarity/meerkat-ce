---
title: Devant vos microservices
section: Le produit
order: 2
navTitle: Microservices
summary: La place de la gateway dans votre cluster : un seul point publié devant des services qui ne changent pas, pour les interfaces comme pour les API.
---

# Devant vos microservices

::: lead
Une application interne n'est plus un programme : c'est une dizaine de
services, écrits dans plusieurs langages et livrés par plusieurs équipes. La
question n'est donc plus seulement de savoir ce que fait une gateway, mais
où elle se place.
:::

## Une porte, pas une bibliothèque par service

Chacun de ces services doit savoir qui appelle et ce que cette personne a le
droit de faire. Si chaque service y répond de son côté, cela donne autant de
pages de connexion que de services, autant de tables d'utilisateurs, et des
sessions qui n'ont pas le même sens d'un service à l'autre. Si l'on y répond
une seule fois, à l'entrée, cela donne une porte.

::: figure mesh
Le trajet d'une requête : elle arrive à la gateway, qui décide, puis elle
repart vers le service concerné - une interface ou une API.
:::

Une **route UI** mène à une page qu'une personne regarde : la gateway
l'habille donc au passage. Le portail de navigation, le bouton de compte, le
thème et le mode clair ou sombre sont injectés dans le HTML. L'application
n'embarque rien pour cela, et ne sait même pas qu'on l'habille.

Une **route API** est appelée par un programme : il n'y a donc rien à
habiller. Elle reçoit un jeton signé qui dit qui appelle, avec ses rôles et son
organisation, et elle n'authentifie personne. Un service est souvent les deux à
la fois : ses pages derrière une route UI, son API derrière une route API.

## Dans le cluster

::: figure cluster
Un seul point publié, plusieurs répliques interchangeables derrière lui, et vos
services qui restent à l'intérieur.
:::

Votre ingress ne publie qu'une chose : le plan de données de la gateway. Vos
services ne restent joignables que depuis l'intérieur du cluster : aucun d'eux
n'a donc de porte à garder. La console d'administration, elle, n'est pas du
tout sur ce port : c'est le second plan, sur un réseau interne.

Derrière l'ingress, plusieurs répliques servent les mêmes routes. Elles ne se
parlent jamais : ce qu'elles ont en commun est dans la base de données, et une
route modifiée parvient aux autres répliques en une seconde. Les sessions s'y
trouvent aussi : il n'y a donc aucune affinité à demander au répartiteur de
charge. Une requête arrive sur n'importe quelle réplique, et une mise à jour
progressive ne fait perdre sa session à personne.

> [!NOTE]
> Cet état partagé demande un PostgreSQL, et c'est une capacité Enterprise. Une
> gateway seule, sur son stockage embarqué, ne partage rien avec personne et
> n'en a pas besoin : c'est l'édition Community, et elle suffit à porter une
> application entière.

## Ce que vos services n'ont plus à porter

::: cards
### La page de connexion

La gateway la sert, à vos couleurs, avec le mot de passe, le second
facteur, les passkeys et votre annuaire d'entreprise derrière. Plus aucun de
vos services n'en a une.

### La table des utilisateurs

Les comptes, les rôles, les groupes et les organisations sont dans la
gateway. Vos services ne stockent plus d'identités, et n'ont plus à les
maintenir cohérentes entre eux.

### Les règles d'accès

Qui passe se décide à la porte, route par route, et jusqu'à une opération
précise de votre spécification OpenAPI. La règle change sans redéployer le
service qu'elle protège.

### L'habillage

Le portail de navigation, le bouton de compte et le thème arrivent dans le
HTML au passage. Une interface de plus en bénéficie sans que l'on y touche.
:::

## Là où la gateway ne va pas

Le trafic entre vos services reste entre vos services. Meerkat tient la porte,
c'est-à-dire ce qui entre et qui y est autorisé, pas la circulation interne du
cluster. Si vous utilisez déjà un maillage de services (service mesh) pour le
trafic est-ouest, les deux ne se recouvrent pas : l'un s'occupe de vos services
entre eux, l'autre du monde qui frappe à la porte.

## Ajouter un service

::: steps
### Déclarez la route

Un chemin, une cible, et la gateway route. Depuis la console, depuis l'API
d'administration, ou en laissant un agent s'en charger.

### Dites si c'est une interface ou une API

C'est ce qui détermine si la réponse est habillée ou si la requête part avec
un jeton signé.

### Définissez la règle d'accès

Un niveau, des rôles, des comptes nommés et, si nécessaire, une règle par
opération tirée de la spécification du service.
:::

Le service, lui, ne bouge pas : il n'embarque aucune de nos bibliothèques, ne
parle aucun de nos protocoles, et ignore qu'il se trouve derrière une
gateway.

::: cta
### Le détail

La configuration Kubernetes complète, avec le Deployment, les deux Services,
l'Ingress et les health checks, est décrite dans
[Cluster Kubernetes](/docs/deploy/kubernetes). Les deux plans, et ce qui les
sépare, sont présentés dans [Architecture](/docs/concepts/architecture). Ce que
coûte le même assemblage sans Meerkat est détaillé dans
[le dossier Meerkat](/product/the-case).
:::
