---
title: Which shape to deploy
section: Deploy
order: 233
navTitle: The shapes
summary: Three shapes and the files for each: a community gateway, an Enterprise gateway with the tunnel, three Enterprise gateways in front of one PostgreSQL.
---

# Which shape to deploy

Three shapes, and the choice is not made on the number of users: it is made on
what you accept to lose when a machine goes, and on how many organisations the
gateway serves.

::: figure shapes
What each shape holds, and the file that deploys it.
:::

## One gateway, community

One container, one volume, nothing beside it. The embedded store holds the
routes, the accounts, the roles, the sealed vault and the certificates. That is
the whole gateway for one organisation, free, production included.

What it does not do is survive the loss of its machine: a volume does not
follow a container from one host to another. Back up the volume and you have
backed up the gateway.

```bash
# Docker
curl -fsSLO https://www.softwarity.io/deploy/docker-compose.yml
MEERKAT_ADMIN_PASSWORD='choose-one' docker compose up -d

# Kubernetes
helm install meerkat ./meerkat-chart.tgz -f values-ce-one-node.yaml \
  --set admin.password='choose-one'
```

## One gateway, Enterprise

Same shape, Enterprise image: several organisations, the corporate
directories, and the **developer tunnel**. That one listens on **`22222`**, and
it is one more port to publish where your developers reach it and nowhere
else. It does not speak HTTP: it is SSH, and what the gateway checks there is
the public key each developer deposited on their account. See
[Development mode](/product/dev-mode).

The Enterprise image lives on a private registry, hence a pull secret:

```bash
kubectl create secret docker-registry ghcr \
  --docker-server=ghcr.io --docker-username=YOU --docker-password=TOKEN

helm install meerkat ./meerkat-chart.tgz -f values-ee-one-node.yaml \
  --set admin.password='choose-one'
```

## Three gateways, Enterprise

What makes three pods ONE gateway is the database they share, and nothing
else: the nodes never address each other. Sessions live there too, so there is
no affinity to ask of the load balancer, and a rolling update takes nobody's
session with it.

There is no volume any more: what the gateway knows is rows. But the **vault's
master key** has to be the same on every pod, or a secret sealed by one is
unreadable by the others.

```bash
kubectl create secret generic meerkat-state \
  --from-literal=database-url='postgres://meerkat:PASSWORD@postgres:5432/meerkat?sslmode=require' \
  --from-literal=vault-key="$(openssl rand -hex 32)"

helm install meerkat ./meerkat-chart.tgz -f values-ee-cluster.yaml \
  --set admin.password='choose-one'
```

On Swarm, the same shape deploys with the stack below. The three values are
interpolated by Swarm at deploy time, from the shell that deploys: they are
never in the file.

```bash
export MEERKAT_DATABASE_URL='postgres://meerkat:PASSWORD@postgres:5432/meerkat?sslmode=require'
export MEERKAT_VAULT_KEY="$(openssl rand -hex 32)"   # once, then keep it
export MEERKAT_ADMIN_PASSWORD='choose-one'
docker stack deploy -c stack.swarm.yml meerkat
```

> [!WARNING]
> Three pods on the embedded store are three gateways: three sets of routes,
> three sets of accounts, and a console showing whichever one your request
> reached. The chart refuses to render that shape and says what to set instead.

## The files

These are the repository's own, copied here on every publication of the site:
what you download is what the released version deploys.

| File | For |
|---|---|
| [meerkat-chart.tgz](/deploy/meerkat-chart.tgz) | The Helm chart, installed as it is |
| [values-ce-one-node.yaml](/deploy/values-ce-one-node.yaml) | One community gateway on its volume |
| [values-ee-one-node.yaml](/deploy/values-ee-one-node.yaml) | One Enterprise gateway, developer tunnel open |
| [values-ee-cluster.yaml](/deploy/values-ee-cluster.yaml) | Three Enterprise gateways on a shared PostgreSQL |
| [docker-compose.yml](/deploy/docker-compose.yml) | One community gateway, one command |
| [docker-compose.ee.yml](/deploy/docker-compose.ee.yml) | One Enterprise gateway with the tunnel |
| [stack.swarm.yml](/deploy/stack.swarm.yml) | Three gateways on Docker Swarm |

Every file is commented line by line: what a variable does, and what it costs
to forget it. The detail of each variable is on
[One gateway](/docs/deploy/one-gateway), and the Kubernetes manifests written
out in full are on [Kubernetes cluster](/docs/deploy/kubernetes).
