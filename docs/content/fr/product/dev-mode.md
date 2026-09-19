---
title: Mode développement
section: Le produit
order: 7
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

## Retirer un accès

La clé vit sur le compte du développeur, et la passerelle la vérifie à chaque
connexion. La retirer, ou retirer le mode dev, ferme le tunnel en quelques
secondes : rien n'a été émis qui survivrait au retrait, donc il n'y a ni
expiration à attendre ni révocation à propager ailleurs. La page de profil
montre l'empreinte SHA256, celle qu'OpenSSH affiche.

## Tout le monde voit qui sert quoi

Une substitution change ce que l'application devant vous *est*. Ce n'est donc
pas une préférence de développeur, c'est une information pour tous ceux qui la
regardent, et elle est nommée : "checkout, par Alice" plutôt que "checkout, par
quelqu'un".

- **À la connexion**, une page nomme ce qui est substitué et par qui, avant de
  rendre la main sur l'application.
- **Pendant le travail**, un bandeau escamotable le redit sur chaque page, et
  suit en direct les substitutions qui apparaissent et disparaissent.

> [!NOTE]
> Le tunnel embarqué dans la passerelle est Enterprise, et c'est lui qui apporte
> cette attribution : chaque développeur dépose sa propre clé, donc une
> connexion dit qui. L'image communautaire fait tourner plug en agent autonome à
> côté de la passerelle, avec une clé partagée : la connexion prouve alors qu'on
> *a* plug, pas qui on est. Voir [Une passerelle](/docs/deploy/one-gateway) pour
> le fichier compose et les valeurs Helm qui l'ouvrent.

## Ce qui reste à venir

La **portée** d'une substitution : elle vaut aujourd'hui pour tout le trafic, et
rien ne permet encore de la limiter à ceux qui la demandent. Côté exploitation,
il manque l'écran de console listant les sessions en cours, et l'entrée au
journal d'audit pour chaque clé déposée et chaque substitution posée.
