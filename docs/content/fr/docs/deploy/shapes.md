---
title: Quelle forme déployer
section: Déployer
order: 233
navTitle: Les formes
summary: Trois formes et les fichiers pour chacune : une passerelle communautaire, une passerelle Enterprise avec le tunnel, trois passerelles Enterprise devant un PostgreSQL.
---

# Quelle forme déployer

Trois formes, et le choix ne se fait pas sur le nombre d'utilisateurs : il se
fait sur ce que vous acceptez de perdre quand une machine tombe, et sur le
nombre d'organisations que la passerelle sert.

::: figure shapes
Ce que chaque forme contient, et le fichier qui la déploie.
:::

## Une passerelle, communautaire

Un conteneur, un volume, rien à côté. Le stockage embarqué tient les routes,
les comptes, les rôles, le coffre scellé et les certificats. C'est toute la
passerelle pour une organisation, gratuitement, y compris en production.

Ce qu'elle ne fait pas, c'est survivre à la perte de sa machine : un volume ne
suit pas un conteneur d'un hôte à l'autre. Sauvegardez le volume et vous avez
sauvegardé la passerelle.

```bash
# Docker
curl -fsSLO https://www.softwarity.io/deploy/docker-compose.yml
MEERKAT_ADMIN_PASSWORD='choisissez-en-un' docker compose up -d

# Kubernetes
helm install meerkat ./meerkat-chart.tgz -f values-ce-one-node.yaml \
  --set admin.password='choisissez-en-un'
```

## Une passerelle, Enterprise

Même forme, image Enterprise : plusieurs organisations, les annuaires
d'entreprise, et le **tunnel de développement**. Celui-ci écoute sur
**`22222`**, et c'est un port de plus à publier là où vos développeurs
l'atteignent, jamais ailleurs. Il ne parle pas HTTP : c'est du SSH, et ce que
la passerelle y vérifie est la clé publique déposée par chaque développeur sur
son compte. Voir [Mode développement](/product/dev-mode).

L'image Enterprise vit sur un registre privé, donc un secret de tirage :

```bash
kubectl create secret docker-registry ghcr \
  --docker-server=ghcr.io --docker-username=VOUS --docker-password=JETON

helm install meerkat ./meerkat-chart.tgz -f values-ee-one-node.yaml \
  --set admin.password='choisissez-en-un'
```

## Trois passerelles, Enterprise

Ce qui fait de trois pods UNE passerelle est la base qu'ils partagent, et rien
d'autre : les nœuds ne s'adressent jamais entre eux. Les sessions y vivent
aussi, donc aucune affinité à demander au répartiteur, et une mise à jour
progressive n'emporte la session de personne.

Il n'y a plus de volume : ce que la passerelle sait est en base. Mais la **clé
maître du coffre** doit être la même sur tous les pods, sinon un secret scellé
par l'un est illisible par les autres.

```bash
kubectl create secret generic meerkat-state \
  --from-literal=database-url='postgres://meerkat:MOTDEPASSE@postgres:5432/meerkat?sslmode=require' \
  --from-literal=vault-key="$(openssl rand -hex 32)"

helm install meerkat ./meerkat-chart.tgz -f values-ee-cluster.yaml \
  --set admin.password='choisissez-en-un'
```

Sur Swarm, la même forme se déploie avec la pile ci-dessous. Les trois valeurs
sont interpolées par Swarm au moment du déploiement, depuis le shell qui
déploie : elles ne sont jamais dans le fichier.

```bash
export MEERKAT_DATABASE_URL='postgres://meerkat:MOTDEPASSE@postgres:5432/meerkat?sslmode=require'
export MEERKAT_VAULT_KEY="$(openssl rand -hex 32)"   # une fois, puis gardez-la
export MEERKAT_ADMIN_PASSWORD='choisissez-en-un'
docker stack deploy -c stack.swarm.yml meerkat
```

> [!WARNING]
> Trois pods sur le stockage embarqué, ce sont trois passerelles : trois jeux
> de routes, trois jeux de comptes, et une console qui montre celle que votre
> requête a touchée. Le chart refuse de rendre cette forme-là et dit quoi
> poser à la place.

## Les fichiers

Ce sont ceux du dépôt, copiés ici à chaque publication du site : ce que vous
téléchargez est ce que la version en ligne déploie.

| Fichier | Pour |
|---|---|
| [meerkat-chart.tgz](/deploy/meerkat-chart.tgz) | Le chart Helm, à installer tel quel |
| [values-ce-one-node.yaml](/deploy/values-ce-one-node.yaml) | Une passerelle communautaire sur son volume |
| [values-ee-one-node.yaml](/deploy/values-ee-one-node.yaml) | Une passerelle Enterprise, tunnel de développement ouvert |
| [values-ee-cluster.yaml](/deploy/values-ee-cluster.yaml) | Trois passerelles Enterprise sur un PostgreSQL partagé |
| [docker-compose.yml](/deploy/docker-compose.yml) | Une passerelle communautaire, une commande |
| [docker-compose.ee.yml](/deploy/docker-compose.ee.yml) | Une passerelle Enterprise avec le tunnel |
| [stack.swarm.yml](/deploy/stack.swarm.yml) | Trois passerelles sur Docker Swarm |

Chaque fichier est commenté ligne à ligne : ce que fait une variable, et ce
qu'elle coûte si on l'oublie. Le détail de chaque variable est sur
[Une passerelle](/docs/deploy/one-gateway), et les manifestes Kubernetes écrits
en entier sont sur [Cluster Kubernetes](/docs/deploy/kubernetes).
