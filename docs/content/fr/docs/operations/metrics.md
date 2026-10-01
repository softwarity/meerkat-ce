---
title: Métriques
section: Exploitation
order: 209
summary: Ce que la passerelle compte, et comment l'aspirer dans la stack de supervision que vous faites déjà tourner.
---

# Métriques

Les compteurs sont dans les deux éditions, et les courbes de la console aussi : c'est la promesse
zéro dépendance, une passerelle qu'on voit sans rien installer. Ce qui se vend est
l'**externalisation**, vers la stack qui porte déjà votre rétention, vos alertes et vos tableaux de
bord.

> [!NOTE] Enterprise edition
> L'exposition `/metrics` est Enterprise. L'image communautaire ne se contente pas de refuser : le
> code qui connaît le format n'y est pas lié, donc elle n'a aucun moyen de répondre. Elle le dit,
> plutôt que de rendre un corps vide.

## Ce qui est compté

Par route :

| Série | Ce qu'elle porte |
|---|---|
| `meerkat_requests_total` | les requêtes répondues, par classe de statut, de `1xx` à `5xx` |
| `meerkat_request_duration_seconds` | un histogramme à douze bornes, avec sa somme et son compte |
| `meerkat_upstream_failures_total` | les échecs entre la passerelle et le service, par `kind` : `connect`, `timeout`, `refused`, `upstream` |

Par endpoint, où l'unité est le gabarit et jamais un chemin brut :

| Série | Ce qu'elle porte |
|---|---|
| `meerkat_endpoint_requests_total` | les requêtes répondues par cette opération |
| `meerkat_endpoint_errors_total` | les `4xx` et `5xx` parmi elles |
| `meerkat_endpoint_duration_seconds_total` | les secondes passées à y répondre |

Il n'y a pas d'histogramme par endpoint, et c'est délibéré : une spec qui déclare deux cents
opérations transformerait douze seaux en deux mille quatre cents séries pour une seule route. La
somme et le compte donnent une moyenne, qui est ce sur quoi une vue par endpoint se lit ; la
distribution reste au niveau de la route.

Et la passerelle elle-même :

| Série | Ce qu'elle porte |
|---|---|
| `meerkat_requests_in_flight` | une jauge - le seul chiffre qui dit *saturé* plutôt que *occupé* |
| `meerkat_logins_total` | les tentatives de connexion, par `outcome` |
| `meerkat_unmatched_total` | les requêtes qui n'ont matché aucune route |

Les étiquettes sont bornées par construction : un identifiant et un nom de route, une classe de
statut, un seau, un gabarit d'opération, et un `source` qui dit si ce gabarit était `declared` ou
`deduced`. Jamais un utilisateur, jamais une adresse, jamais un chemin brut.

## L'endpoint

`/metrics` a son **propre port**, comme les exporteurs de PostgreSQL (9187) ou de RabbitMQ (15692).
On le choisit dans la console, **Infra, Metrics endpoint**, au moment d'allumer l'exposition :
**9091** par défaut, et un autre si la plateforme l'utilise déjà. La passerelle ouvre ce port sur
chaque noeud tant que l'interrupteur est allumé, le déplace quand on en change, et le referme quand
on l'éteint. Il sert `/metrics` et rien d'autre : ni la console, ni l'API.

Un port que ce noeud ne peut pas ouvrir, déjà pris ou réservé, est refusé avec la raison, et rien
n'est enregistré. Les ports sur lesquels écoutent les deux plans de cette gateway (8080 et 9090 par défaut) sont refusés d'office.

Trois choses le gardent, et le refus dit laquelle manque :

1. l'image Enterprise ;
2. un interrupteur livré **éteint** ;
3. le réseau, et au choix un jeton.

Par défaut, ce port **ne demande pas de jeton**. C'est le réseau qui ferme : on ne le publie
jamais, et on ne met ni route ni ingress devant. Un scrape sans crédentiale, c'est une configuration
de supervision sans secret à faire tourner.

Un second interrupteur, **Require a token**, sert quand d'autres charges du cluster peuvent joindre ce
port et ne doivent pas le lire. Les compteurs nomment chaque route et chaque gabarit d'endpoint :
c'est une carte de l'installation, pas une page publique. Le jeton est alors vérifié par le même
entonnoir que le plan de contrôle. Il se frappe avec la portée `metrics`, qui n'ouvre que ce chemin.

`/metrics` répond aussi sur le plan de contrôle, **toujours** avec un jeton. C'est la porte d'une
installation dont le Prometheus ne joint la passerelle que par l'adresse de la console.

En Kubernetes, le Service doit déclarer le port pour qu'un `ServiceMonitor` le trouve. Le chart le
fait avec la valeur `metrics.port`, sur un Service `-metrics` toujours en ClusterIP, et cette valeur
doit reprendre le port choisi dans la console.

![Les jetons d'accès, là où se frappe la crédentiale du scraper](img/console/access-tokens.webp)

Le format est le texte Prometheus, `version=0.0.4`, annoncé dans le type de contenu. Les compteurs ne
font que monter et Prometheus fait sa propre différence ; la fenêtre de la console est dérivée des
mêmes compteurs, et non l'inverse.

## Poussées en OTLP

L'endpoint est une sortie, celle où un scraper vient chercher. L'autre est d'**envoyer** les mêmes
compteurs à un collecteur OpenTelemetry, ce que veut une stack qui reçoit plutôt qu'elle ne scrape.
Cela s'allume dans **Infra, OpenTelemetry**, à côté des traces, et part vers le même collecteur avec
la même crédentiale : toutes les 30 secondes, des totaux cumulés depuis le démarrage de la
passerelle, une ressource par noeud (`service.instance.id`).

Les noms sont ceux d'OpenTelemetry (`meerkat.requests`, `meerkat.request.duration` en secondes...),
choisis pour qu'un collecteur qui les retraduit en Prometheus retombe sur **les mêmes séries** que
l'endpoint. Un tableau de bord écrit sur une porte lit l'autre.

Il faut un collecteur qui reçoit des métriques. Jaeger ne prend que des traces : mettez un
OpenTelemetry Collector devant. Le bouton Test de cette page le dit.

## Les fichiers à écrire

La page Metrics endpoint porte de vraies ressources, servies par la passerelle sous `/monitoring/` sur le
plan de contrôle : un `prometheus.yml`, un fichier compose pour Swarm, un `ServiceMonitor` pour
Kubernetes, et pour Grafana sa source de données, son provisionnement de tableaux de bord et un
tableau de bord prêt. Ils sont copiables et téléchargeables, et ils portent le port de métriques de
**cette** installation. Le bloc du jeton n'y apparaît que si le port en exige un.

Il y a un seul `prometheus.yml` et non un par plateforme : le scrape ne change pas de Swarm à
Kubernetes, seule la découverte de la cible change. Le fichier porte les deux, et un bouton par
plateforme rend la version voulue.

Le compose Grafana éteint aussi le formulaire de connexion de Grafana et la place derrière la
passerelle : la route décide qui entre, et l'identité transmise dit qui c'est. Ce qui fait de la non
publication du port de Grafana une règle et non plus un confort.

## En cluster

Les compteurs sont **par noeud**. Les deux fichiers de plateforme visent donc chaque noeud -
`tasks.meerkat` en Swarm, `role: pod` en Kubernetes - et jamais le service devant eux : scraper une
adresse virtuelle tirerait un noeud différent à chaque passage et dessinerait une courbe qui n'est
celle de personne.

Prometheus **tire**, donc la vue intégrée de la console est plus temps réel que lui, pas une version
dégradée.

## Ce qui manque

- Aucune courbe de p95 dans la console. L'histogramme est collecté et exposé ; Grafana la trace, et la
 requête est sur la page Metrics endpoint.
- Rien sur une requête en particulier : c'est l'autre moitié, et elle a sa page
 ([les traces](/docs/operations/tracing)). Un compteur détecte et délimite, une trace explique un
 cas.
- `grpc-status` n'est pas lu, donc tout appel gRPC compte en `2xx` et le taux d'échec d'une route gRPC
 lit zéro.
- Rien sur QUI a appelé : aucune étiquette n'est jamais un utilisateur, et c'est ce qui borne la
 cardinalité. Cette question-là se répond dans le [journal d'accès](/docs/operations/logs).
