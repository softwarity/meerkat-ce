---
title: Devant vos microservices
section: Le produit
order: 2
navTitle: Microservices
summary: Où se place la passerelle dans votre cluster : un seul point publié devant des services qui ne changent pas, pour les interfaces comme pour les API.
---

# Devant vos microservices

::: lead
Une application interne n'est plus un programme : c'est une dizaine de services,
écrits dans plusieurs langages, livrés par plusieurs équipes. La question n'est
alors pas seulement ce qu'une gateway sait faire, mais où elle se met.
:::

## Une porte, pas une bibliothèque par service

Chacun de ces services a besoin de savoir qui appelle et ce que cette personne
a le droit de faire. Répondu service par service, cela donne autant de pages de
connexion que de services, autant de tables d'utilisateurs, et des sessions qui
ne veulent pas dire la même chose d'un service à l'autre. Répondu une fois,
devant, cela donne une porte.

::: figure mesh
Le chemin d'une requête : elle arrive sur la passerelle, qui tranche, puis elle
repart vers le service concerné, une interface ou une API.
:::

Une **route UI** est une page qu'une personne regarde, alors la passerelle
l'habille au passage : le portail de navigation, le bouton de compte, le thème,
le mode clair et sombre, injectés dans le HTML. L'application n'embarque rien
pour cela et ne sait même pas qu'on l'habille.

Une **route API** est appelée par un programme, il n'y a donc rien à habiller.
Elle reçoit un jeton signé, qui dit qui appelle, avec ses rôles et son
organisation, et elle n'authentifie personne. Un service est souvent les deux :
ses pages derrière une route UI, sa propre API derrière une route API.

## Dans le cluster

::: figure cluster
Un seul point publié, plusieurs répliques interchangeables derrière, et vos
services qui restent à l'intérieur.
:::

Votre ingress ne publie qu'une chose : le plan de données de la passerelle. Vos
services restent joignables depuis l'intérieur du cluster seulement, ce qui veut
dire qu'aucun d'eux n'a de porte à garder. La console d'administration, elle,
n'est pas sur ce port du tout : c'est le second plan, sur un réseau interne.

Derrière l'ingress, plusieurs répliques servent les mêmes routes. Elles ne se
parlent jamais : ce qu'elles ont en commun est en base, et une route modifiée
atteint les autres en une seconde. Les sessions y vivent aussi, donc aucune
affinité à demander au répartiteur : une requête atterrit sur n'importe quelle
réplique, et une mise à jour progressive n'emporte la session de personne.

> [!NOTE]
> Cet état partagé demande un PostgreSQL, et c'est une capacité Enterprise. Une
> passerelle seule sur son stockage embarqué ne partage rien avec personne et
> n'en a pas besoin : c'est l'édition communautaire, et elle tient une
> application entière.

## Ce que vos services cessent de porter

::: cards
### La page de connexion

Elle est servie par la passerelle, à vos couleurs, avec le mot de passe, le
second facteur, les passkeys et votre annuaire d'entreprise derrière. Aucun de
vos services n'en a plus une.

### La table des utilisateurs

Comptes, rôles, groupes et organisations sont dans la passerelle. Vos services
ne stockent plus d'identités et n'ont plus à les tenir d'accord entre eux.

### Les règles d'accès

Qui passe se décide à la porte, par route et jusqu'au niveau d'un endpoint de
votre spec OpenAPI. La règle change sans redéployer le service qu'elle protège.

### L'habillage

Le portail de navigation, le bouton de compte et le thème arrivent dans le HTML
au passage. Une interface de plus les reçoit sans qu'on y touche.
:::

## Là où la passerelle ne se met pas

Le trafic entre vos services reste entre vos services. Meerkat tient la porte,
c'est-à-dire ce qui entre et qui a le droit, pas la circulation intérieure du
cluster. Si vous avez déjà un maillage de services pour le trafic est-ouest,
les deux ne se recouvrent pas : l'un s'occupe de vos services entre eux, l'autre
du monde qui frappe.

## Ajouter un service

::: steps
### Déclarez la route

Un chemin, une cible, et la passerelle route. Depuis la console, l'API
d'administration, ou en laissant un agent le faire.

### Dites si c'est une interface ou une API

C'est ce qui décide si la réponse est habillée ou si la requête part avec un
jeton signé.

### Posez la règle d'accès

Un niveau, des rôles, des comptes nommés, et si besoin une règle par endpoint
tirée de la spec du service.
:::

Le service, lui, ne bouge pas : il n'embarque aucune bibliothèque, ne parle
aucun protocole à nous, et ne sait pas qu'il est derrière une passerelle.

::: cta
### Le détail

La forme Kubernetes complète, avec le Deployment, les deux Services, l'Ingress
et les sondes, est sur [Cluster Kubernetes](/docs/deploy/kubernetes). Les deux
plans et ce qui les sépare sont sur [Architecture](/docs/concepts/architecture).
Ce que le même assemblage coûte autrement est sur
[le dossier Meerkat](/product/the-case).
:::
