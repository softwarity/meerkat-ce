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

Le déployer n'ouvre encore rien. Le tunnel a aussi son propre interrupteur dans
la console, **Infra, Plug**, livré éteint, à côté du mode développeur ; cette
page enregistre l'adresse que les développeurs utilisent et leur donne les
commandes pour installer plug sous macOS, Linux et Windows. Voir
[Plug](/docs/operations/plug).

Le fichier Enterprise est celui du dessus plus trois choses : l'image
Enterprise, un port, et le socket qui permet à l'agent de donner un nom à une
machine. L'image est dans un registre privé, et c'est un accord commercial qui
l'ouvre - voir [les éditions](/product/editions).

```yaml
services:
  meerkat:
    image: ghcr.io/softwarity/meerkat:latest
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
helm repo add meerkat https://www.softwarity.io/deploy

helm install meerkat meerkat/meerkat \
  --set admin.password='votre-premier-mot-de-passe'

# Enterprise, tunnel ouvert (`plug.enabled` vaut déjà true) :
helm install meerkat meerkat/meerkat \
  --set image.repository=ghcr.io/softwarity/meerkat --set 'image.pullSecrets[0]=ghcr' \
  --set admin.password='votre-premier-mot-de-passe'

# Enterprise de production : la surface de développement est fermée, et le
# chart n'accorde alors aucun droit sur le namespace.
helm install meerkat meerkat/meerkat \
  --set image.repository=ghcr.io/softwarity/meerkat --set 'image.pullSecrets[0]=ghcr' \
  --set admin.password='votre-premier-mot-de-passe' \
  --set production=true
```

Ce que le chart accorde au ServiceAccount de cette passerelle, **dans son seul
namespace**, est ce dont l'agent a besoin et rien de plus :

| sur | pour |
|---|---|
| `services`, `endpoints` | pointer un nom de cluster vers la machine d'un développeur, puis le remettre en place |
| `endpointslices` (list, deletecollection) | supprimer la slice que Kubernetes garde pour le pod déployé derrière un Service à port nommé - sans elle, une requête sur deux atteint encore ce pod |
| `deployments` (lire, patcher) | son propre redémarrage, et la reprise périodique d'un Service garé |
| `pods` (lire, créer, supprimer) | hériter de l'environnement du service remplacé, et lancer le temps d'une session le pod qui sert ses volumes au poste du développeur |
| `pods/exec` | lire cet environnement dans le pod garé, comme le fait mirrord |
| `persistentvolumeclaims` (lire) | refuser avec une raison un volume `ReadWriteOncePod`, plutôt qu'un pod qui ne démarre jamais |

`pods/exec` est le plus large des sept : dans ce namespace, cela revient à
exécuter du code dans les pods, et lire l'environnement qu'un pod a **déjà
résolu** est lire les secrets qu'il a résolus. C'est aussi pourquoi l'agent ne
demande jamais `get secrets`. Il ne voit aucun autre namespace, et la liste est
celle de plug lui-même (`deploy/plug-k8s.yaml` dans son dépôt) : un droit que
l'agent a acquis depuis se découvrirait au moment où quelqu'un utilise la
fonction, ce qui est le pire endroit pour l'apprendre.

Le droit **suit le tunnel** plutôt qu'un interrupteur à lui. `production: true`
ferme la surface de développement : la passerelle n'ouvre aucun tunnel, et le
chart n'accorde alors strictement rien - un droit que personne n'exerce est de
la surface pour rien. Sinon le tunnel peut s'ouvrir - il le fait une fois allumé
sous **Infra, Plug** -, donc le déploiement accorde ce qu'on peut lui demander
d'utiliser : c'est pourquoi `plug.enabled` vaut **true par
défaut**, la combinaison inverse - le tunnel ouvert, les droits absents - étant
la seule qui ne peut pas marcher, avec un agent qui dit chaque minute qu'il ne
peut pas faire son travail. `plug.enabled: false` refuse tout de même le grant,
pour une installation qui veut la surface de développement sans le tunnel.

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
| `MEERKAT_PLUG_ADDR` | `:22222` | Déplace le port du tunnel de développement. Cela ne le coupe pas : l'interrupteur **Infra, Plug** le fait, et fermer la surface de développement aussi. |
| `MEERKAT_PRODUCTION` | non posée | Déclare cette passerelle de production et ferme définitivement la surface de développement. |
| `MEERKAT_TENANCY` | `single` | Une organisation implicite, ou plusieurs (Enterprise). Choisi une fois, au premier démarrage ; la console en est maîtresse ensuite. |
