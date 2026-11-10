---
title: Topology
section: La console
order: 151
summary: Ce qui sert l'installation et ce qu'elle sert - les instances synchronisées, la base qu'elles partagent, les services derrière les routes et s'ils répondent.
---

# Topology

**Infra, Topology** dessine l'installation : les instances Meerkat qui la servent,
la base qu'elles partagent, et les services vers lesquels pointent les routes.
C'est la première entrée de la section Infra, avant les routes : ce qui sert,
avant ce qui est servi.

L'écran est à `/infra/topology`. Réservé à root et aux administrateurs infra.

![L'écran Topology : trois instances synchronisées sur PostgreSQL à gauche, les services derrière les routes à droite](img/console/topology.webp)

## Les instances

À gauche, dans le cadre **Meerkat**, chaque instance qui tourne, de la plus
ancienne à la plus récente :

- son **nom d'hôte** - le pod ou le conteneur, ce par quoi on la retrouve - avec
  un bouton qui le copie ;
- sa **version** dans le coin, et l'heure de son **démarrage** dans son infobulle,
  dans le fuseau de votre profil ;
- **Not ready**, avec la raison dans son infobulle (ce que répond son `/readyz`),
  seulement quand elle ne l'est pas ; **Past the license** pour une instance au-delà
  de ce que couvre la licence.

L'instance qui a répondu à l'écran prend la couleur du cadre.

Entre deux instances, **in sync** dit que le bus de changement les tient
synchronisées : une route, un certificat, une session ou la licence enregistrés
sur l'une atteignent les autres en moins d'une seconde. Chaque instance descend tout
droit vers la **base** dessous, celle qu'elles partagent. Sur la base embarquée, il n'y a
qu'une instance et rien à synchroniser.

Chaque instance dit qui elle est toutes les cinq secondes, qu'elle porte du trafic
ou non. Celle qui s'arrête le dit et quitte le dessin aussitôt ; celle qui se tait
le quitte au bout de vingt secondes.

## Les services

À droite, dans le cadre **Services** : une carte par adresse vers laquelle pointent
les routes actives, nommée par les routes qui y mènent, l'adresse dessous avec un
bouton qui la copie. L'icône est `web`
quand l'une de ces routes sert des pages à des personnes, `api` sinon. Seul ce
qu'une route déclare est montré : rien n'est découvert ici.

Un badge dans son coin dit ce que la gateway en voit - combien de ses répliques
répondent, sur combien - et son infobulle le dit en mots :

- **3/3**, dans la couleur d'accent : il répond ;
- **1/2**, en orange : il manque des répliques ;
- **0/2**, en rouge : il ne répond pas, avec la raison et depuis quand ;
- **?** pour un service qu'aucun trafic ni aucune vérification n'a encore atteint.

Un service dont le runtime ne compte pas les répliques - un hôte externe, vérifié
par une connexion - est une seule adresse : **1/1** ou **0/1**.

Ceux qui demandent un regard passent en tête, et la liste défile seule quand il y
en a beaucoup. C'est une vue : un service se modifie par ses routes, sur l'écran
[Routes](/docs/console/routes).

## Ce qu'il signale

En clair en haut de l'écran, et par une pastille sur **Topology** et sur **Infra**
dans le rail :

- **plus d'instances que la licence n'en couvre**, et l'heure à laquelle la mention
  d'évaluation reviendra si elles sont toujours aussi nombreuses - une mise à jour
  progressive qui se termine avant n'est pas un dépassement ;
- **deux versions en même temps** : une mise à jour en cours, ou arrêtée à
  mi-chemin ;
- **une instance pas prête**, et pourquoi ;
- **un service qui ne répond pas**.

L'écran suit la gateway en direct : rien à rafraîchir.
