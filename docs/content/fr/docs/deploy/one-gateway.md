---
title: Une passerelle
section: Déployer
order: 234
summary: Docker Compose et Helm pour une passerelle unique sur son volume, et ce que décide chaque variable d'environnement.
---

# Une passerelle

Un binaire, deux ports. **8080 est le plan applicatif**, celui que vos
utilisateurs atteignent. **9090 est le plan de contrôle**, la console
d'administration. Seul le premier a sa place sur internet, et ce sont deux
Services dans le chart exactement pour cette raison : un Ingress placé devant
vos applications ne doit pas pouvoir atteindre la console par accident.

Tout ce que la passerelle sait - routes, comptes, coffre scellé, certificats -
vit dans un seul répertoire. Donnez-lui un volume et vous avez rendu la
passerelle redémarrable ; sauvegardez ce volume et vous avez sauvegardé la
passerelle.

## Docker Compose

L'image communautaire, en entier :

```yaml
services:
  meerkat:
    image: docker.io/softwarity/meerkat:latest
    ports:
      - "8080:8080"   # plan applicatif
      - "9090:9090"   # plan de contrôle - gardez celui-ci interne
    environment:
      # Lu UNE FOIS, au premier démarrage, pour créer le compte admin.
      MEERKAT_ADMIN_PASSWORD: votre-premier-mot-de-passe
    volumes:
      - meerkat-data:/data
    restart: unless-stopped

volumes:
  meerkat-data:
```

## Ouvrir le tunnel de développement

Le tunnel branche la machine d'un développeur sur votre stack : un service
qu'il fait tourner sur son portable répond, pour lui, sous son nom de cluster,
et tout le reste continue de lui parler comme s'il était encore déployé. C'est
[plug](https://github.com/softwarity/plug) compilé dans la passerelle - l'image
communautaire fait tourner plug à côté de la passerelle, ce qui est le mode par
défaut de plug et ne demande rien à Meerkat. À quoi tout cela sert est sur la
page [Mode développement](/product/dev-mode).

Le fichier Enterprise est celui du dessus plus trois choses : l'image
Enterprise, un port, et le socket qui permet à l'agent de donner un nom à une
machine. L'image est dans un registre privé, et c'est un accord commercial qui
l'ouvre - voir [les éditions](/product/editions).

```yaml
services:
  meerkat:
    image: ghcr.io/softwarity/meerkat-ee:latest
    ports:
      - "8080:8080"
      - "9090:9090"
      - "22222:22222"   # le tunnel de développement
    environment:
      MEERKAT_ADMIN_PASSWORD: votre-premier-mot-de-passe
      # MEERKAT_PRODUCTION: "1"   # voir plus bas
    volumes:
      - meerkat-data:/data
      - /var/run/docker.sock:/var/run/docker.sock
```

> [!WARNING]
> Le socket est un vrai droit. Qui peut agir comme ce conteneur peut agir sur ce
> démon Docker. Montez-le sur les clusters auxquels vos développeurs ont déjà
> accès, et nulle part ailleurs. Sans lui la passerelle tourne exactement comme
> avant : le tunnel écrit une ligne disant quelle ressource lui manque, et sert
> comme d'habitude.

## Kubernetes, avec Helm

Une passerelle sur son volume, la forme ci-dessous. Plusieurs passerelles pour
une seule installation, ce sont d'autres objets (plus de volume, une base
partagée, trois répliques), et cela a sa page :
[cluster Kubernetes](/docs/deploy/kubernetes), avec les manifestes et avec la
partie que personne n'écrit - comment ne pas faire du point d'entrée le point
unique de défaillance.

```bash
helm install meerkat ./deploy/helm/meerkat \
  --set admin.password='votre-premier-mot-de-passe'

# Enterprise, tunnel ouvert :
helm install meerkat ./deploy/helm/meerkat \
  --set image.repository=ghcr.io/softwarity/meerkat-ee \
  --set admin.password='votre-premier-mot-de-passe' \
  --set plug.enabled=true
```

`plug.enabled` est ce qui accorde au ServiceAccount de cette passerelle le droit
de **gérer les Services de son propre namespace**, et rien d'autre. C'est ce qui
permet à une session de pointer un nom de cluster vers la machine d'un
développeur, puis de le remettre en place. L'agent ne peut pas lire un secret,
toucher un pod, ni voir un autre namespace. Désactivé, le chart n'accorde
strictement rien.

## Déclarer une passerelle de production

`MEERKAT_PRODUCTION` (`production: true` dans le chart) ferme ici toute la
surface de développement - l'outillage, la documentation d'API, le mode test de
l'UI, le tunnel - quoi que dise la base.

Cela vaut la peine d'être posé même là où personne n'attend de tunnel, et la
raison est la bonne : **l'interrupteur stocké voyage**. Un export de
configuration, une sauvegarde restaurée, une base copiée depuis la préproduction
peuvent chacun emporter le mode développement ACTIVÉ en production, et personne
ne s'en aperçoit avant qu'il y ait un port de tunnel devant les clients.
L'environnement, lui, ne voyage pas : c'est une propriété de l'endroit où le
processus tourne.

Cela ne va que dans un sens : cela ferme une surface, cela n'en ouvre jamais une
qu'un opérateur a coupée. La console montre l'interrupteur désactivé *avec la
raison* plutôt que de le cacher, parce que quelqu'un qui cherche un outillage
absent doit apprendre pourquoi.

## Ce que décide chaque variable

| Variable | Défaut | Ce qu'elle décide |
| --- | --- | --- |
| `MEERKAT_ADMIN_PASSWORD` | - | Le premier compte admin, au PREMIER démarrage seulement. Jamais relue ensuite. |
| `MEERKAT_DATA` | `data` | Où la passerelle range tout ce qu'elle sait. |
| `MEERKAT_ADDR` / `MEERKAT_ADMIN_ADDR` | `:8080` / `:9090` | Les deux plans. |
| `MEERKAT_TLS_ADDR` / `MEERKAT_ADMIN_TLS_ADDR` | `:8443` / `:9443` | Les portes HTTPS, ouvertes dès qu'un certificat existe pour un nom. En avoir un EST l'activation : aucun interrupteur ne peut dire "actif" alors que rien n'est servi. |
| `MEERKAT_DATABASE_URL` | - | Un PostgreSQL externe à la place du stockage embarqué. Enterprise, et le prérequis d'un [cluster](/docs/deploy/kubernetes). |
| `MEERKAT_VAULT_KEY` | un fichier sous le répertoire de données | La clé maître du coffre. Elle doit être fournie, et identique, sur tous les nœuds d'un cluster. |
| `MEERKAT_PLUG_ADDR` | `:22222` | Déplace le port du tunnel de développement. Cela ne le coupe pas : fermer la surface de développement, si. |
| `MEERKAT_PRODUCTION` | non posée | Déclare cette passerelle de production et ferme définitivement la surface de développement. |
| `MEERKAT_TENANCY` | `single` | Une organisation implicite, ou plusieurs (Enterprise). Choisi une fois, au premier démarrage ; la console en est maîtresse ensuite. |
