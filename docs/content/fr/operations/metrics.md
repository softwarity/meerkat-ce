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
bord (OBS-05).

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
| `meerkat_endpoint_duration_seconds_sum` | les secondes passées à y répondre |

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

`/metrics` est sur le **plan de contrôle** : aucun port à ouvrir, c'est là où est déjà la console.
Trois choses le gardent, et le refus dit laquelle manque :

1. l'image Enterprise ;
2. un interrupteur livré **éteint**, dans le tiroir Prometheus de l'écran Metrics ;
3. la capacité gateway-admin, ou un jeton dont la portée est `metrics`.

La garde n'est pas une formalité. Les compteurs nomment chaque route et chaque gabarit d'endpoint :
c'est une carte de l'installation, pas une page publique. Un jeton `metrics` n'ouvre que ce chemin et
rien d'autre - une crédentiale de scraper vit dans la configuration d'une stack de supervision,
souvent le dépôt d'une autre équipe, l'endroit où un jeton a le plus de chances de fuiter et le moins
d'être tourné.

![Les jetons d'accès, là où se frappe la crédentiale du scraper](img/console/access-tokens.webp)

Le format est le texte Prometheus, `version=0.0.4`, annoncé dans le type de contenu. Les compteurs ne
font que monter et Prometheus fait sa propre différence ; la fenêtre de la console est dérivée des
mêmes compteurs, et non l'inverse.

## Les fichiers à écrire

Le tiroir Prometheus porte de vraies ressources, servies par la passerelle sous `/monitoring/` sur le
plan de contrôle : un `prometheus.yml`, un fichier compose pour Swarm, un `ServiceMonitor` pour
Kubernetes, et pour Grafana sa source de données, son provisionnement de tableaux de bord et un
tableau de bord prêt. Ils sont copiables et téléchargeables, et ils portent le port d'écoute de
**cette** installation - pas le port par lequel la console a été joint, puisqu'une publication de port
ou un ingress s'intercale et que le scrape, lui, adresse le conteneur.

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
  requête est dans le tiroir.
- Aucun `traceparent` propagé vers les amonts (OBS-04).
- `grpc-status` n'est pas lu, donc tout appel gRPC compte en `2xx` et le taux d'échec d'une route gRPC
  lit zéro (ROUTE-20).
- Les journaux n'ont pas de niveau réglable et il n'y a pas de journal des requêtes (OBS-03).
