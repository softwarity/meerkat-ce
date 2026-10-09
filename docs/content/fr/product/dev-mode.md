---
title: Mode développement
section: Le produit
order: 7
summary: Le poste d'un développeur rejoint le cluster et prend la place d'un service déployé, et tous ceux qui regardent l'application savent lequel, et qui le remplace.
---

# Mode développement

Toutes les équipes connaissent le problème : le cluster contient tous les
services **et toutes les données**, et reconstruire cela sur un portable est
difficile, parfois interdit. Meerkat ne copie pas l'environnement. Il permet à
un développeur de faire tourner **un service sur sa propre machine, à la place
du service déployé**, le temps de travailler dessus - tout le reste reste en
place. Le tunnel qui le permet s'appelle [plug](https://github.com/softwarity/plug).

> [!NOTE] Édition Enterprise
> plug lui-même est gratuit, et tourne à côté de n'importe quelle stack. Ce que
> décrit cette page - plug intégré à Meerkat, avec une clé par développeur, et
> la substitution nommée et annoncée à tous - relève d'Enterprise. Le Swagger
> des développeurs, plus bas, est dans toutes les éditions.

## Comment cela marche

Un administrateur marque un utilisateur comme développeur (`dev`). Le
développeur ajoute sa **clé SSH publique** à son compte Meerkat (Profil,
Développeur), puis lance son service en local à travers plug :

```bash
plug -p <gateway-host> -s user-mng-service:8080:3000 npm run start
```

- **Le poste atteint le cluster** : le service local appelle les autres
  services par leur nom, comme s'il tournait dans le cluster.
- **Le cluster atteint le poste** : `-s` déclare une **substitution**. Le
  trafic destiné à `user-mng-service` part désormais vers la machine du
  développeur, et toutes les routes qui utilisent ce service suivent d'elles-mêmes.
- **Elle dure le temps de la session** : arrêtez le processus, et le service
  déployé revient. Un redémarrage de la gateway y met fin aussi.

Pour l'installer selon votre système et ouvrir le tunnel :
[Plug](/docs/operations/plug).

## Retirer un accès

La clé est attachée au compte du développeur, et la gateway la vérifie à chaque
connexion. Retirez la clé, ou le rôle de développeur, et le tunnel se ferme en
quelques secondes : aucun certificat ni jeton n'a été émis qui pourrait y
survivre, il n'y a donc rien à faire expirer ni à révoquer ailleurs. La page de
profil affiche l'empreinte de la clé exactement comme OpenSSH l'affiche.

## Tout le monde voit qui sert quoi

Une substitution change la nature même de l'application que vous avez sous les
yeux. Tous ceux qui l'utilisent en sont donc avertis, avec un nom : "checkout,
par Alice" plutôt que "checkout, par quelqu'un".

- **À la connexion**, une page indique ce qui est substitué et par qui, avant
  de donner accès à l'application.
- **Pendant que vous travaillez**, un petit bandeau le rappelle sur chaque
  page, et se met à jour en direct quand une substitution commence ou s'arrête.

> [!NOTE]
> C'est l'intégration qui permet de nommer : chaque développeur a sa propre
> clé, donc chaque connexion dit qui se connecte. Avec Community, plug tourne
> seul à côté de la gateway, avec une seule clé partagée : une connexion prouve
> alors que l'appelant *possède* plug, pas qui il est, et Meerkat ne sait rien
> de la substitution. Voir [Une gateway](/docs/deploy/one-gateway) pour le
> fichier compose et les valeurs Helm qui ouvrent le tunnel intégré.

## Le Swagger des développeurs, servi par la gateway

La gateway voit toutes les API de l'installation, et connaît la description
OpenAPI que déclare chaque route. Meerkat en fait donc une seule page, à
l'adresse `/meerkat/apidocs`, ouverte aux développeurs et à personne d'autre -
dans toutes les éditions.

Ce n'est pas juste un Swagger UI de plus, à côté d'un service :

- **Toutes les API au même endroit**, routes désactivées comprises : un
  développeur voit ce qui existe et ce qui est en construction.
- **Rien à installer, rien qui vienne d'un CDN.** Swagger UI est dans la
  gateway : une installation sans internet a la même page, et aucune requête ne
  part vers un tiers.
- **Les appels passent par la gateway**, avec son authentification, ses règles
  d'accès et ses réécritures. Ce que montre la page est ce que le service répond
  en production, pas ce qu'il répondrait si on l'appelait directement.
- **Et surtout, vous choisissez qui appelle.** *Try it out* peut agir sous un
  **utilisateur** donné, des **groupes** ou des **rôles**, et affiche l'identité
  qui en résulte. C'est la vraie question que se pose un développeur devant une
  API protégée : "que répond-elle à quelqu'un qui n'a que ce rôle ?". Sans cela,
  il faut un compte de test par cas, et chacun finit par tester avec le sien.

La console a sa propre page, à l'adresse `/apidocs`, pour l'API
d'administration de Meerkat : celle qu'utilisent la console et
l'[agent IA](/docs/agent/overview).

## Ce qui reste à venir

- **Limiter une substitution à certains utilisateurs** : aujourd'hui, elle vaut
  pour tout le trafic.
- **Un écran de la console qui liste les sessions en cours**, et une entrée
  d'audit pour chaque substitution. L'ajout d'une clé figure déjà dans le
  [journal d'audit](/docs/operations/audit), avec son empreinte.
