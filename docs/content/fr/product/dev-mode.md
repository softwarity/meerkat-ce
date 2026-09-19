---
title: Mode développement
section: Le produit
order: 5
summary: Le poste d'un développeur rejoint le cluster et remplace un service déployé, et tous ceux qui regardent l'application savent lequel, et par qui.
---

# Mode développement

Le problème que connaît toute équipe d'application interne : le cluster
contient tous les services **et toutes les données**, et cet environnement est
difficile, parfois interdit, à reproduire sur la machine d'un développeur. La
réponse de Meerkat n'est pas de dupliquer l'environnement mais d'amener le
poste **dans le maillage de routage**, comme un locataire limité au routage,
grâce à [plug](https://github.com/softwarity/plug).

## Comment cela marche

Un administrateur marque un utilisateur comme `dev`. Le développeur dépose sa
**clé SSH publique** sur son compte Meerkat (Profil, Développeur, clé), puis
lance son service localement à travers plug :

```bash
plug -p cluster --service user-mng-service npm run start
```

- **Du poste vers le cluster** : le processus local résout et atteint les
  services du cluster comme s'il tournait dedans. C'est le comportement
  d'origine de plug.
- **Du cluster vers le poste** : `--service` déclare une **substitution**. Le
  trafic vers `user-mng-service` passe par le tunnel inverse jusqu'au processus
  local, qui est vu, en entrée comme en sortie, comme un membre à part entière
  du cluster. Toutes les routes qui référencent le service suivent
  automatiquement.
- **Cela vit le temps de la session** : la substitution disparaît quand le
  processus s'arrête, et un redémarrage de la passerelle emporte les deux bouts.

## Une clé, pas un certificat

Une autorité de certification a été étudiée puis écartée. Un certificat porte sa
propre validité : une fois émis, il est accepté jusqu'à son expiration, donc le
reprendre demande d'attendre ou de monter une machinerie de révocation. Une
passerelle est debout par définition : elle peut répondre *cette clé est-elle
toujours autorisée* à chaque connexion. Donc **révoquer, c'est supprimer la
ligne**, et quelqu'un qui part cesse de pouvoir tunneler en quelques secondes.
La page de profil montre l'empreinte SHA256 qu'OpenSSH affiche lui-même, et
aucune date d'expiration : l'absence est le choix.

## Personne ne choisit une variante, tout le monde est prévenu

Il n'y a pas de menu de développeurs parmi lesquels choisir, et pas de rôle
testeur. Une substitution change ce que l'application devant vous *est*, donc
c'est une nouvelle pour tous ceux qui la regardent : le travail de Meerkat est
de rendre l'état courant **visible et attribué** - "checkout, par Alice" plutôt
que "checkout, par quelqu'un" - et non d'offrir un choix que personne ne peut
faire correctement.

Cette attribution est ce qu'ajoute l'édition Enterprise. plug seul embarque une
clé unique dans le binaire publié : une connexion prouve alors que l'appelant
*a* plug, pas qui il est, ce qui est honnête et suffisant pour les clusters de
confiance qu'il vise.

> [!NOTE]
> Le tunnel embarqué dans la passerelle est Enterprise. plug reste un produit à
> part entière : l'image communautaire le fait tourner en agent autonome à côté
> de la passerelle, ce qui est son mode par défaut et ne demande rien à Meerkat.
> Voir [Une passerelle](/docs/deploy/one-gateway) pour le fichier compose et les
> valeurs Helm qui l'ouvrent.

## Ce qui reste à venir

L'état est servi mais pas encore montré : une page après connexion nommant ce
qui est substitué, un bandeau qui reste vrai pendant qu'on travaille, un écran
de console listant les sessions en cours, et le journal d'audit de chaque clé
déposée et de chaque substitution posée.
