---
title: Quelle architecture déployer
section: Déployer
order: 233
navTitle: Les architectures
summary: Trois architectures et les fichiers de chacune : une gateway Community, une gateway Enterprise avec le tunnel, trois gateways Enterprise devant un même PostgreSQL.
---

# Quelle architecture déployer

Trois architectures possibles, et le choix ne dépend pas du nombre
d'utilisateurs : il dépend de ce que vous acceptez de perdre quand une machine
tombe, et du nombre d'organisations que sert la gateway.

::: figure shapes
Ce que contient chaque architecture, et le fichier qui la déploie.
:::

## Une gateway, Community

Un conteneur, un volume, rien d'autre à côté. Le stockage embarqué contient les
routes, les comptes, les rôles, le coffre scellé et les certificats. C'est la
gateway complète pour une organisation, gratuite, production comprise.

Ce qu'elle ne sait pas faire, c'est survivre à la perte de sa machine : un
volume ne suit pas un conteneur d'un hôte à un autre. Sauvegardez le volume, et
vous avez sauvegardé la gateway.

```bash
# Docker
curl -fsSLO https://www.softwarity.io/deploy/docker-compose.yml
MEERKAT_ADMIN_PASSWORD='choose-one' docker compose up -d

# Kubernetes
helm repo add meerkat https://www.softwarity.io/deploy
helm install meerkat meerkat/meerkat -f https://www.softwarity.io/deploy/values-ce-one-node.yaml \
  --set admin.password='choose-one'
```

## Une gateway, Enterprise

La même architecture avec l'image Enterprise : plusieurs organisations, les
annuaires d'entreprise et le **tunnel développeur**. Ce dernier écoute sur le
port **`22222`** : c'est un port de plus à publier, là où vos développeurs
peuvent l'atteindre et nulle part ailleurs. Il ne parle pas HTTP mais SSH, et
la gateway y vérifie la clé publique que chaque développeur a déposée sur
son compte. Voir [Mode développement](/product/dev-mode).

L'image Enterprise est hébergée sur un registre privé, d'où un secret d'accès
au registre :

```bash
kubectl create secret docker-registry ghcr \
  --docker-server=ghcr.io --docker-username=YOU --docker-password=TOKEN

helm install meerkat meerkat/meerkat -f https://www.softwarity.io/deploy/values-ee-one-node.yaml \
  --set admin.password='choose-one'
```

## Trois gateways, Enterprise

Ce qui fait de trois pods UNE seule gateway, c'est la base de données qu'ils
partagent, et rien d'autre : les nœuds ne s'adressent jamais les uns aux
autres. Les sessions s'y trouvent aussi : il n'y a donc aucune affinité à
demander au load balancer, et une mise à jour progressive ne fait
perdre sa session à personne.

Il n'y a plus de volume : tout ce que sait la gateway est fait de lignes en
base. En revanche, la **clé maîtresse du coffre** doit être la même sur tous
les pods, faute de quoi un secret scellé par l'un est illisible pour les
autres.

```bash
kubectl create secret generic meerkat-state \
  --from-literal=database-url='postgres://meerkat:PASSWORD@postgres:5432/meerkat?sslmode=require' \
  --from-literal=vault-key="$(openssl rand -hex 32)"

helm install meerkat meerkat/meerkat -f https://www.softwarity.io/deploy/values-ee-cluster.yaml \
  --set admin.password='choose-one'
```

Sur Swarm, la même architecture se déploie avec la stack ci-dessous. Swarm
interpole les trois valeurs au moment du déploiement, à partir du shell qui
déploie : elles ne figurent jamais dans le fichier.

```bash
export MEERKAT_DATABASE_URL='postgres://meerkat:PASSWORD@postgres:5432/meerkat?sslmode=require'
export MEERKAT_VAULT_KEY="$(openssl rand -hex 32)"   # une seule fois, puis conservez-la
export MEERKAT_ADMIN_PASSWORD='choose-one'
docker stack deploy -c stack.swarm.yml meerkat
```

> [!WARNING]
> Trois pods sur le stockage embarqué sont trois gateways : trois jeux de
> routes, trois jeux de comptes, et une console qui montre celle des trois
> qu'a atteinte votre requête. Le chart refuse de générer cette architecture
> et indique quoi régler à la place.

## Les fichiers

Ce sont les fichiers du dépôt lui-même, copiés ici à chaque publication du
site : ce que vous téléchargez est ce que déploie la version publiée.

| Fichier | Usage |
|---|---|
| [index.yaml](/deploy/index.yaml) | Le dépôt Helm : `helm repo add meerkat https://www.softwarity.io/deploy` |
| [meerkat-chart.tgz](/deploy/meerkat-chart.tgz) | Le chart Helm, à installer tel quel |
| [values-ce-one-node.yaml](/deploy/values-ce-one-node.yaml) | Une gateway Community sur son volume |
| [values-ee-one-node.yaml](/deploy/values-ee-one-node.yaml) | Une gateway Enterprise, tunnel développeur ouvert |
| [values-ee-cluster.yaml](/deploy/values-ee-cluster.yaml) | Trois gateways Enterprise sur un PostgreSQL partagé |
| [docker-compose.yml](/deploy/docker-compose.yml) | Une gateway Community, en une commande |
| [docker-compose.ee.yml](/deploy/docker-compose.ee.yml) | Une gateway Enterprise avec le tunnel |
| [stack.swarm.yml](/deploy/stack.swarm.yml) | Trois gateways sur Docker Swarm |

Le dépôt distribue le **chart**, pas les images : le cluster les télécharge
lui-même, depuis le registre que désignent les valeurs. Il n'y a qu'un chart
pour les deux éditions, et l'édition, c'est l'image :

- **Community** : `docker.io/softwarity/meerkat`, publique - c'est la valeur
  par défaut du chart, il n'y a rien à ajouter.
- **Enterprise** : `ghcr.io/softwarity/meerkat`, privée - un secret d'accès au
  registre, qui contient l'accès qui vous a été remis, est nommé dans
  `image.pullSecrets`. Les fichiers `values-ee-*` définissent les deux ; c'est
  toute la différence au moment de l'installation.

Chaque fichier est commenté ligne par ligne : ce que fait une variable, et ce
qu'il en coûte de l'oublier. Le détail de chaque variable se trouve sur la page
[Une gateway](/docs/deploy/one-gateway), et les manifestes Kubernetes
écrits en entier sur la page [Cluster Kubernetes](/docs/deploy/kubernetes).
