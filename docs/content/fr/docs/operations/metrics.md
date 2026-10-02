---
title: Métriques
section: Exploitation
order: 209
summary: Ce que la passerelle compte, où la console le montre, et comment en garder l'historique en le poussant en OTLP.
---

# Métriques

La passerelle compte ce qu'elle sert, et la console le dessine : l'écran **Metrics** (`/traffic`)
montre la dernière heure, sans rien installer - c'est la promesse zéro dépendance, dans les deux
éditions. Voir [l'écran de trafic](/docs/operations/traffic) et
[l'écran Metrics](/docs/console/traffic).

Cette fenêtre vit en mémoire et repart de zéro au redémarrage de la passerelle. Pour garder un
historique, alerter dessus ou le dessiner à côté du reste de votre plateforme, les compteurs sont
**poussés en OTLP** vers un collecteur OpenTelemetry, qui les écrit dans Prometheus. C'est la seule
façon dont ils sortent de la passerelle : rien ne vient la scraper.

## Ce qui est compté

Par route, sous les noms qu'elles prennent dans Prometheus une fois traduites par un collecteur :

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
| `meerkat_gateway_requests_total` | les requêtes répondues par toute la passerelle, toutes routes comprises, par classe de statut |
| `meerkat_gateway_request_duration_seconds` | le temps d'une réponse, toutes routes comprises |
| `meerkat_requests_in_flight` | une jauge - le seul chiffre qui dit *saturé* plutôt que *occupé* |
| `meerkat_logins_total` | les tentatives de connexion, par `outcome` |
| `meerkat_unmatched_total` | les requêtes qui n'ont matché aucune route |

Une route dont **Include this route in OpenTelemetry** est coupé n'a aucune
série à elle. Ses requêtes comptent quand même dans les deux totaux
`meerkat_gateway_`.

Un appel gRPC répond toujours `200` et met son verdict dans `grpc-status`. Il est
compté selon ce verdict, traduit comme gRPC traduit ses codes en HTTP :
`UNAVAILABLE` est un `5xx`, `NOT_FOUND` ou `PERMISSION_DENIED` un `4xx`. L'appelant
reçoit toujours son `200` et son trailer.

Les étiquettes sont bornées par construction : un identifiant et un nom de route, une classe de
statut, un seau, un gabarit d'opération, et un `source` qui dit si ce gabarit était `declared` ou
`deduced`. Jamais un utilisateur, jamais une adresse, jamais un chemin brut.

## Garder un historique : la poussée OTLP

Elle s'allume dans **Infra, OpenTelemetry**, onglet **Metrics**. Les compteurs partent vers le
collecteur des traces, avec la même crédentiale : toutes les 30 secondes, des totaux cumulés depuis
le démarrage de la passerelle, une ressource par noeud (`service.instance.id`).

Les noms sont ceux d'OpenTelemetry (`meerkat.requests`, `meerkat.request.duration` en secondes...),
choisis pour qu'un collecteur qui les écrit dans Prometheus retombe sur les séries ci-dessus : une
somme monotone prend `_total`, une unité en secondes prend `_seconds`. Les fichiers de collecteur que
donne la console (Infra, OpenTelemetry, *No collector yet?*) envoient déjà les métriques vers
Prometheus.

Il faut un collecteur qui reçoit des métriques. Un backend qui ne prend que des traces ne suffit
pas : mettez un OpenTelemetry Collector devant. Le bouton Test le dit.

> [!NOTE] Enterprise edition
> La poussée des compteurs est Enterprise. Les compteurs et l'écran Metrics sont dans les deux
> éditions ; ce qui se vend, c'est de les externaliser vers la stack que vous faites déjà tourner.

## Le tableau de bord Grafana

Parmi les fichiers du collecteur, deux compagnons Grafana. `grafana-dashboard.json` est un tableau
de bord prêt sur ces séries : trafic par route, taux d'échec, p95, endpoints les plus lents et les
plus coûteux, services muets, connexions refusées. `grafana-datasources.yaml` déclare les sources
Prometheus, Tempo et Loki (le tableau de bord lit Prometheus, uid `meerkat-prometheus`).

## Ce qui manque

- Rien sur une requête en particulier : c'est l'autre moitié, et elle a sa page
 ([les traces](/docs/operations/tracing)). Un compteur détecte et délimite, une trace explique un
 cas.
- Rien sur QUI a appelé : aucune étiquette n'est jamais un utilisateur, et c'est ce qui borne la
 cardinalité. Cette question-là se répond dans le [journal d'accès](/docs/operations/logs).
