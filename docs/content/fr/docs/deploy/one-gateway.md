---
title: Une gateway
section: Déployer
order: 234
summary: Docker Compose et Helm pour une gateway unique sur son propre volume, et ce que décide chaque variable d'environnement.
---

# Une gateway

Un binaire, deux ports. **8080 est le plan applicatif** - celui qu'atteignent
vos utilisateurs. **9090 est le plan de contrôle** - la console
d'administration. Seul le premier a sa place sur Internet, et c'est exactement
pour cette raison que le chart en fait deux Services : un Ingress placé devant
vos applications ne doit pas pouvoir atteindre la console par accident.

Tout ce que sait la gateway - les routes, les comptes, le coffre scellé, les
certificats - se trouve dans un seul répertoire. Donnez-lui un volume, et la
gateway peut redémarrer sans rien perdre ; sauvegardez ce volume, et vous
avez sauvegardé la gateway.

## Docker Compose

L'image Community, en entier :

```yaml
services:
  meerkat:
    image: docker.io/softwarity/meerkat:latest
    ports:
      - "8080:8080"   # plan applicatif
      - "9090:9090"   # plan de contrôle - gardez celui-ci en interne
    environment:
      # Lu UNE SEULE FOIS, au premier démarrage, pour créer le compte admin.
      MEERKAT_ADMIN_PASSWORD: your-first-password
    volumes:
      - meerkat-data:/data
    restart: unless-stopped

volumes:
  meerkat-data:
```

## Ouvrir le tunnel développeur

Le tunnel branche la machine d'un développeur sur votre stack : un service
qu'il exécute sur son portable répond, pour lui, sous son nom dans le cluster,
et tout le reste continue de s'adresser à ce service comme s'il était toujours
déployé. C'est [plug](https://github.com/softwarity/plug), compilé dans la
gateway - avec l'image Community, plug tourne à côté de la gateway, ce
qui est son mode par défaut et ne demande rien à Meerkat. La page
[Mode développement](/product/dev-mode) explique à quoi sert l'ensemble.

Le déployer n'ouvre encore rien. Le tunnel a aussi son propre interrupteur dans
la console, **Infra, Plug**, livré désactivé, à côté de celui du mode
développeur ; cette page enregistre l'adresse qu'utilisent les développeurs et
leur fournit les commandes d'installation de plug sous macOS, Linux et Windows.
Voir [Plug](/docs/operations/plug).

Le fichier Enterprise est le précédent, plus trois choses : l'image Enterprise,
un port, et le socket qui permet à l'agent de donner un nom à une machine.
L'image est hébergée dans un registre privé, que seul un accord commercial
ouvre - voir [les éditions](/product/editions).

```yaml
services:
  meerkat:
    image: ghcr.io/softwarity/meerkat:latest
    ports:
      - "8080:8080"
      - "9090:9090"
      - "22222:22222"   # le tunnel développeur
    environment:
      MEERKAT_ADMIN_PASSWORD: your-first-password
      # MEERKAT_PRODUCTION: "1"   # voir plus bas
    volumes:
      - meerkat-data:/data
      - /var/run/docker.sock:/var/run/docker.sock
```

> [!WARNING]
> Monter le socket, c'est accorder un vrai pouvoir. Quiconque peut agir au nom
> de ce conteneur peut agir sur ce démon Docker. Montez-le sur les clusters
> auxquels vos développeurs ont déjà accès en confiance, et nulle part
> ailleurs. Sans lui, la gateway fonctionne exactement comme avant : le
> tunnel écrit une ligne indiquant la ressource qui lui manque, et la
> gateway sert comme d'habitude.

### L'image d'évaluation

Pour essayer Enterprise avant de parler à qui que ce soit, l'**image d'évaluation**
est l'image Enterprise sans rien en moins, publique sur Docker Hub, avec une
mention d'évaluation sur ses pages et dans sa console - sans licence pour la
production. Même fichier, une ligne changée, aucun registre où se connecter :

```yaml
    image: docker.io/softwarity/meerkat:eval   # ou X.Y.Z-eval, pour épingler une version
```

Le fichier publié [docker-compose.ee.yml](/deploy/docker-compose.ee.yml) prend son
image dans `MEERKAT_IMAGE` : il lance l'évaluation sans être modifié.

```bash
MEERKAT_IMAGE=docker.io/softwarity/meerkat:eval docker compose -f docker-compose.ee.yml up -d
```

## Kubernetes, avec Helm

Une gateway sur son propre volume : c'est l'architecture décrite ci-dessous.
Plusieurs gateways au service d'une même installation demandent un autre
jeu d'objets (pas de volume, une base de données partagée, trois répliques),
et font l'objet d'une page à part :
[Cluster Kubernetes](/docs/deploy/kubernetes), avec les manifestes et avec ce
que personne ne met par écrit - comment éviter que le point d'entrée ne
devienne un point de défaillance unique.

```bash
helm repo add meerkat https://www.softwarity.io/deploy

helm install meerkat meerkat/meerkat \
  --set admin.password='your-first-password'

# Enterprise, tunnel ouvert (`plug.enabled` vaut déjà true) :
helm install meerkat meerkat/meerkat \
  --set image.repository=ghcr.io/softwarity/meerkat --set 'image.pullSecrets[0]=ghcr' \
  --set admin.password='your-first-password'

# Évaluation : tout ce que fait Enterprise, publique, pas pour la production.
helm install meerkat meerkat/meerkat \
  -f https://www.softwarity.io/deploy/values-eval-one-node.yaml \
  --set admin.password='your-first-password'

# Enterprise en production : la surface développeur est fermée, et le chart
# n'accorde alors plus aucun droit sur le namespace.
helm install meerkat meerkat/meerkat \
  --set image.repository=ghcr.io/softwarity/meerkat --set 'image.pullSecrets[0]=ghcr' \
  --set admin.password='your-first-password' \
  --set production=true
```

Ce que le chart accorde au ServiceAccount de cette gateway, **dans son
propre namespace uniquement**, correspond à ce dont l'agent a besoin, et à
rien de plus :

| sur | pour |
|---|---|
| `services`, `endpoints` | faire pointer un nom du cluster vers la machine d'un développeur, puis le remettre en place |
| `endpointslices` (list, deletecollection) | supprimer la slice que Kubernetes conserve pour le pod déployé derrière un Service à port nommé - sans cela, une requête sur deux atteint encore ce pod |
| `deployments` (get, patch) | son propre redémarrage, et le balayage qui rétablit un Service mis de côté |
| `pods` (get, create, delete) | hériter de l'environnement du service remplacé, et exécuter - le temps d'une session - le pod qui sert ses volumes à la machine du développeur |
| `pods/exec` | lire cet environnement à l'intérieur du pod mis de côté, comme le fait mirrord |
| `persistentvolumeclaims` (get) | refuser un volume `ReadWriteOncePod` en donnant la raison, plutôt que par un pod qui ne démarre jamais |

`pods/exec` est le plus large des sept : dans ce namespace, il revient à
pouvoir exécuter du code dans les pods, et lire l'environnement qu'un pod a
**déjà résolu**, c'est lire les secrets qu'il a résolus. C'est aussi pour cela
que l'agent ne demande jamais `get secrets`. Il ne voit aucun autre namespace,
et la liste est celle de plug lui-même (`deploy/plug-k8s.yaml` dans son
dépôt) : un droit que l'agent aurait acquis depuis ne se découvrirait qu'au
moment où quelqu'un utilise la fonction, c'est-à-dire au pire moment.

Ces droits **suivent le tunnel**, et non un interrupteur qui leur serait
propre. `production: true` ferme la surface développeur : la gateway
n'ouvre aucun tunnel, et le chart n'accorde alors strictement rien - un droit
que personne n'exerce est une surface d'attaque gratuite. Dans le cas
contraire, le tunnel peut s'ouvrir - il le fait dès qu'il est activé dans
**Infra, Plug** - et le déploiement accorde donc ce qu'on pourra lui demander
d'utiliser. C'est pourquoi `plug.enabled` vaut **true** par défaut : la
situation inverse - le tunnel ouvert, les droits absents - est la seule
combinaison qui ne peut pas fonctionner, avec un agent qui répète chaque
minute qu'il ne peut pas faire son travail. `plug.enabled: false` refuse
quand même purement et simplement ces droits, pour une installation qui veut
la surface développeur sans le tunnel.

Sur OpenShift ou OKD, le chart s'installe tel quel, sous la SCC par défaut - voir [OpenShift et OKD](/docs/deploy/kubernetes#openshift-et-okd) avant de surcharger son contexte de sécurité.

## Déclarer une gateway de production

`MEERKAT_PRODUCTION` (`production: true` dans le chart) ferme ici toute la
surface développeur - l'outillage, la documentation des API, le mode de test
des interfaces, le tunnel - quoi que dise la base de données.

Ce réglage mérite d'être posé même là où personne n'attend de tunnel, et la
raison en est intéressante : **l'interrupteur stocké en base voyage**. Un
export de configuration, une sauvegarde restaurée, une base copiée depuis la
préproduction peuvent chacun amener en production un mode développeur ACTIVÉ,
et personne ne le remarque avant qu'un port de tunnel se retrouve face aux
clients. L'environnement, lui, ne voyage pas : il est une propriété de
l'endroit où s'exécute le processus.

Il n'agit que dans un sens : il ferme une surface, il n'en rouvre jamais une
qu'un exploitant a coupée. La console affiche l'interrupteur désactivé *en en
donnant la raison* plutôt que de le masquer, car celui qui cherche un outil
absent doit pouvoir comprendre pourquoi.

## Ce que fait chaque variable

| Variable | Valeur par défaut | Ce qu'elle décide |
| --- | --- | --- |
| `MEERKAT_ADMIN_PASSWORD` | - | Le premier compte admin, au PREMIER démarrage uniquement. Elle n'est plus jamais lue ensuite. |
| `MEERKAT_DATA` | `data` | L'endroit où la gateway conserve tout ce qu'elle sait. |
| `MEERKAT_ADDR` / `MEERKAT_ADMIN_ADDR` | `:8080` / `:9090` | Les deux plans. |
| `MEERKAT_TLS_ADDR` / `MEERKAT_ADMIN_TLS_ADDR` | `:8443` / `:9443` | Les accès HTTPS, ouverts dès qu'un certificat existe pour un nom. En posséder un, c'est cela l'activation - aucun interrupteur ne peut dire "activé" alors que rien n'est servi. |
| `MEERKAT_DATABASE_URL` | - | Un PostgreSQL externe à la place du stockage embarqué. Édition Enterprise, et condition préalable à un [cluster](/docs/deploy/kubernetes). |
| `MEERKAT_VAULT_KEY` | un fichier dans le répertoire de données | La clé maîtresse du coffre. Elle doit être fournie, et identique, sur chaque nœud d'un cluster. |
| `MEERKAT_PLUG_ADDR` | `:22222` | Change le port du tunnel développeur. Elle ne le désactive pas : c'est le rôle de l'interrupteur **Infra, Plug**, ou de la fermeture de la surface développeur. |
| `MEERKAT_PRODUCTION` | non définie | Déclare cette gateway comme étant de production et ferme définitivement la surface développeur. |
| `MEERKAT_TENANCY` | `single` | Une seule organisation implicite, ou plusieurs (Enterprise). Se choisit une fois, au premier démarrage ; c'est ensuite la console qui en décide. |
