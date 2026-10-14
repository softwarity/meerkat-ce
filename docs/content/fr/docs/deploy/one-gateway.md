---
title: Une gateway
section: Déployer
order: 234
summary: Docker Compose et Helm pour une gateway unique sur son propre volume, et ce que décide chaque variable d'environnement.
---

# Une gateway

Un binaire, trois ports. **8080 est le plan applicatif** - celui qu'atteignent
vos utilisateurs. **9090 est le plan de contrôle** - la console
d'administration. **7070 est le plan interne** - ce qu'appellent vos services :
le JWKS, l'API du scheduler. Seul le premier a sa place sur Internet, et c'est
exactement pour cette raison que le chart en fait trois Services : un Ingress
placé devant vos applications ne doit pas pouvoir atteindre la console par
accident, et publier la console ne doit pas publier ce qu'appellent les
backends. Le plan interne n'est publié nulle part : les services du même réseau
l'atteignent par son nom.

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
      # Le runtime, à travers le proxy en lecture seule ci-dessous.
      DOCKER_HOST: tcp://docker-proxy:2375
    networks: [default, runtime]
    volumes:
      - meerkat-data:/data
    restart: unless-stopped

  # Le socket Docker derrière un filtre qui refuse toute écriture.
  docker-proxy:
    image: tecnativa/docker-socket-proxy:v0.4.1
    environment:
      { SERVICES: 1, TASKS: 1, CONTAINERS: 1, NETWORKS: 1, IMAGES: 1, NODES: 1, EVENTS: 1, INFO: 1 }
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    networks: [runtime]
    restart: unless-stopped

networks:
  runtime:
    internal: true

volumes:
  meerkat-data:
```

Le proxy permet à la gateway de lire le runtime sans pouvoir agir dessus : l'éditeur
de routes propose les conteneurs qu'il trouve, et la liste des routes montre si chaque
cible tourne et sur quelle image, d'après les événements de Docker (voir
[Santé des upstreams](/docs/operations/upstream-health)). Monter le socket dans la
gateway elle-même donnerait l'hôte entier à quiconque contrôle la gateway. Sans le
proxy, la gateway fonctionne comme avant, sans l'un ni l'autre.

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

## Derrière un proxy ou un load balancer

La plupart des gateways ne sont pas jointes directement : un contrôleur Ingress, nginx,
Traefik, un load balancer de cloud ou un CDN se tient devant. La connexion que la gateway
accepte vient alors de ce proxy, et l'identité réelle de l'appelant voyage dans deux
en-têtes que le proxy écrit : `X-Forwarded-For` (l'adresse de l'appelant) et
`X-Forwarded-Proto` (le navigateur était-il en https).

Ce sont des en-têtes, et n'importe qui peut écrire un en-tête. La gateway ne les croit donc
**que venant d'un proxy de confiance**, et cela décide de beaucoup de choses : chaque
compteur par adresse (échecs de connexion, réinitialisations de mot de passe, limite de
débit par IP d'une route), l'adresse que montrent l'historique des connexions et l'audit,
l'attribut `Secure` d'un cookie de session, et la redirection du port en clair vers https.

**Ce qu'elle déduit par défaut.** Sans réglage, la gateway regarde l'adresse de l'autre bout
de la connexion - le pair TCP :

- un pair **de boucle locale ou privé** (`127.0.0.0/8`, `::1`, `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `fc00::/7`) est un proxy : ses en-têtes sont crus. C'est l'Ingress d'un cluster Kubernetes, le pont Docker, un réseau compose, un nginx sur la même machine ;
- tout **autre** pair est un inconnu : ses en-têtes `X-Forwarded-*` sont ignorés, et sa propre adresse est celle de l'appelant.

Derrière plusieurs proxys, `X-Forwarded-For` se lit de droite à gauche : chaque saut de
confiance est sauté, et la première adresse qui n'en est pas un est l'appelant. Ce que
l'appelant a écrit plus à gauche n'est jamais lu. La plage du NAT des opérateurs
(`100.64.0.0/10`) n'est pas de confiance - c'est là que vivent les appareils des abonnés,
un réseau Tailscale par exemple, pas des proxys -, pas plus que le lien local.

**Ce que reçoit le service derrière** est ce que la gateway croit, jamais plus :
`X-Forwarded-Proto` vaut `https` quand la porte de la gateway était en TLS ou qu'un proxy
de confiance a dit que celle du navigateur l'était - un service derrière un Ingress qui
termine le TLS construit donc des liens `https://` ; `X-Forwarded-For` est la chaîne de
confiance suivie de l'adresse du proxy, ou, venant d'un inconnu, sa seule adresse ;
`X-Forwarded-Host` est l'hôte par lequel la gateway a été jointe ; et `Forwarded`
(RFC 7239) dit les trois mêmes choses en un seul élément,
`for=<appelant>;host=<hôte>;proto=<schéma>`.

**Quand régler `MEERKAT_TRUSTED_PROXIES`.** Quand ce qui se tient devant joint la gateway
depuis des adresses **publiques** : un CDN, ou un load balancer de cloud qui ne passe pas
par un réseau privé (ceux de Google Cloud joignent les backends depuis `130.211.0.0/22` et
`35.191.0.0/16`, par exemple). Nommez ses plages, telles que le fournisseur les publie :

```yaml
environment:
  MEERKAT_TRUSTED_PROXIES: "130.211.0.0/22, 35.191.0.0/16"
```

La valeur est une liste de plages CIDR ou d'adresses séparées par des virgules, et elle
**remplace** le défaut au lieu de s'y ajouter : listez aussi le proxy privé s'il y en a un
entre le load balancer et la gateway. Dans le chart, c'est
`trustedProxies: ["130.211.0.0/22", "35.191.0.0/16"]`.

`none` ne fait confiance à personne : chaque adresse est celle du pair TCP, et aucun en-tête
transmis n'est jamais lu. C'est la valeur d'une gateway jointe **directement** depuis
internet, et d'un réseau local où l'on joint la gateway depuis des adresses privées sans
proxy - le défaut ferait confiance à chacun comme à un proxy.

Une valeur illisible arrête la gateway au démarrage en donnant la raison, plutôt que de
retomber sur un défaut que personne n'a choisi. Le journal de démarrage dit quel ensemble
est en vigueur (`forwarded headers ... trusted_proxies=`).

**Ce qui se passe quand c'est faux.**

- **Trop étroit** (le load balancer n'est pas de confiance) : chaque appelant est compté comme le load balancer. Les échecs de connexion d'une personne freinent tout le monde, l'historique et l'audit montrent l'adresse du load balancer pour tous, et derrière un load balancer qui termine TLS la gateway croit le navigateur en http - les cookies perdent `Secure` et la redirection vers https boucle.
- **Trop large** (l'adresse d'un appelant est de confiance) : cet appelant peut écrire l'adresse de son choix dans `X-Forwarded-For`, s'offrir un budget neuf à chaque requête et traverser tous les compteurs, et choisir l'adresse que l'audit enregistre.

L'audit ne montre jamais seulement ce qu'un en-tête prétendait : quand l'adresse vient d'un
proxy, la ligne garde les deux - `203.0.113.5 via 10.42.0.15`.

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
| `MEERKAT_ADDR` / `MEERKAT_ADMIN_ADDR` | `:8080` / `:9090` | Le plan applicatif et le plan de contrôle. |
| `MEERKAT_INTERNAL_ADDR` / `MEERKAT_INTERNAL_TLS_ADDR` | `:7070` / `:7443` | Le plan interne : le JWKS et l'API du scheduler, pour les services placés derrière la gateway. Jamais publié ; sa porte HTTPS s'ouvre quand un certificat y est placé. |
| `MEERKAT_TLS_ADDR` / `MEERKAT_ADMIN_TLS_ADDR` | `:8443` / `:9443` | Les accès HTTPS, ouverts dès qu'un certificat existe pour un nom. En posséder un, c'est cela l'activation - aucun interrupteur ne peut dire "activé" alors que rien n'est servi. |
| `MEERKAT_DATABASE_URL` | - | Un PostgreSQL externe à la place du stockage embarqué. Édition Enterprise, et condition préalable à un [cluster](/docs/deploy/kubernetes). |
| `MEERKAT_VAULT_KEY` | un fichier dans le répertoire de données | La clé maîtresse du coffre. Elle doit être fournie, et identique, sur chaque nœud d'un cluster. |
| `MEERKAT_PLUG_ADDR` | `:22222` | Change le port du tunnel développeur. Elle ne le désactive pas : c'est le rôle de l'interrupteur **Infra, Plug**, ou de la fermeture de la surface développeur. |
| `MEERKAT_PRODUCTION` | non définie | Déclare cette gateway comme étant de production et ferme définitivement la surface développeur. |
| `MEERKAT_TRUSTED_PROXIES` | boucle locale et plages privées | Les proxys dont `X-Forwarded-For` et `X-Forwarded-Proto` sont crus : plages CIDR ou adresses séparées par des virgules, qui remplacent le défaut ; `none` ne fait confiance à personne. Voir [Derrière un proxy ou un load balancer](#derrire-un-proxy-ou-un-load-balancer). |
| `MEERKAT_TENANCY` | `single` | Une seule organisation implicite, ou plusieurs (Enterprise). Se choisit une fois, au premier démarrage ; c'est ensuite la console qui en décide. |
