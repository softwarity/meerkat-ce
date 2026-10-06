---
title: Métriques
section: Exploitation
order: 209
summary: Ce que la gateway compte, où la console l'affiche, et comment en conserver l'historique en le poussant en OTLP.
---

# Métriques

La gateway compte ce qu'elle sert, et la console en trace les courbes : l'écran
**Metrics** (`/data-plane/metrics`) affiche la dernière heure, sans rien à installer - c'est la promesse
du zéro dépendance, dans les deux éditions. Voir [l'écran de trafic](/docs/operations/traffic)
et [l'écran Metrics](/docs/console/traffic).

Cette fenêtre est conservée en mémoire et repart de zéro quand la gateway redémarre. Pour
garder un historique, déclencher des alertes ou tracer ces courbes à côté de celles du reste
de votre plateforme, les compteurs sont **poussés en OTLP** vers un collecteur OpenTelemetry,
qui les écrit dans Prometheus. C'est leur seule façon de sortir de la gateway : rien ne
vient les y collecter.

## Ce qui est compté

Par route, sous les noms que les séries prennent dans Prometheus une fois traduites par un
collecteur :

| Série | Ce qu'elle contient |
|---|---|
| `meerkat_requests_total` | les requêtes servies, par classe de statut, de `1xx` à `5xx` |
| `meerkat_request_duration_seconds` | un histogramme à douze bornes, avec sa somme et son nombre d'observations |
| `meerkat_upstream_failures_total` | les échecs entre la gateway et le service, par `kind` : `connect`, `timeout`, `refused`, `upstream` |

Par endpoint, l'unité étant le gabarit et jamais un chemin brut :

| Série | Ce qu'elle contient |
|---|---|
| `meerkat_endpoint_requests_total` | les requêtes servies par cette opération |
| `meerkat_endpoint_errors_total` | parmi elles, les `4xx` et les `5xx` |
| `meerkat_endpoint_duration_seconds_total` | les secondes passées à y répondre |

Il n'y a pas d'histogramme par endpoint, et c'est voulu : avec une spécification qui déclare
deux cents opérations, douze intervalles deviendraient deux mille quatre cents séries pour
une seule route. La somme et le nombre de requêtes donnent une moyenne, et c'est elle qu'on
lit dans une vue par endpoint ; la distribution reste au niveau de la route.

Et pour la gateway elle-même :

| Série | Ce qu'elle contient |
|---|---|
| `meerkat_gateway_requests_total` | les requêtes servies par la gateway entière, toutes routes confondues, par classe de statut |
| `meerkat_gateway_request_duration_seconds` | le temps mis à répondre, toutes routes confondues |
| `meerkat_requests_in_flight` | une jauge - le seul chiffre qui dit *saturée* plutôt que *occupée* |
| `meerkat_logins_total` | les tentatives de connexion, par `outcome` |
| `meerkat_unmatched_total` | les requêtes qui ne correspondaient à aucune route |

Une route dont l'option **Include this route in OpenTelemetry** est désactivée n'a aucune
série propre. Ses requêtes comptent tout de même dans les deux totaux `meerkat_gateway_`.

Un appel gRPC répond toujours `200` et place son verdict dans `grpc-status`. Il est compté
d'après ce verdict, traduit selon la correspondance que gRPC établit entre ses codes et ceux
de HTTP : `UNAVAILABLE` est un `5xx`, `NOT_FOUND` ou `PERMISSION_DENIED` un `4xx`. L'appelant,
lui, reçoit toujours son `200` et son trailer.

Les étiquettes sont bornées par construction : l'identifiant et le nom d'une route, une
classe de statut, un intervalle d'histogramme, un gabarit d'opération, et une étiquette
`source` qui dit si ce gabarit est `declared` ou `deduced`. Jamais un utilisateur, jamais une
adresse, jamais un chemin brut.

## Garder un historique : l'envoi en OTLP

Activez-le dans **Infra, OpenTelemetry**, onglet **Metrics**. Les compteurs partent vers le
même collecteur que les traces, avec les mêmes identifiants : toutes les 30 secondes, les
totaux cumulés depuis le démarrage de la gateway, à raison d'une ressource par nœud
(`service.instance.id`).

Les noms sont ceux d'OpenTelemetry (`meerkat.requests`, `meerkat.request.duration` en
secondes...). Ils sont choisis pour qu'un collecteur qui les écrit dans Prometheus aboutisse
aux séries ci-dessus : une somme monotone reçoit le suffixe `_total`, une unité en secondes
le suffixe `_seconds`. Les fichiers de collecteur fournis par la console (Infra,
OpenTelemetry, *No collector yet?*) acheminent déjà les métriques vers Prometheus.

Il faut un collecteur qui accepte les métriques. Un backend qui ne prend que des traces ne
convient pas : placez un OpenTelemetry Collector devant lui. Le bouton Test vous le signale.

> [!NOTE] Édition Enterprise
> L'envoi des compteurs relève de l'édition Enterprise. Les compteurs et l'écran Metrics
> existent dans les deux éditions ; ce qui est vendu, c'est leur externalisation vers une
> pile d'outils que vous exploitez déjà.

## Le tableau de bord Grafana

Les fichiers du collecteur comprennent deux fichiers d'accompagnement pour Grafana.
`grafana-dashboard.json` est un tableau de bord prêt à l'emploi, construit sur ces séries :
trafic par route, taux d'échec, p95, endpoints les plus lents et les plus coûteux, services
silencieux, connexions refusées. `grafana-datasources.yaml` déclare les sources de données
Prometheus, Tempo et Loki (le tableau de bord lit Prometheus, uid `meerkat-prometheus`).

## Ce qui manque

- Rien sur une requête en particulier : c'est l'autre moitié du sujet, et elle a sa propre
  page ([les traces](/docs/operations/tracing)). Un compteur détecte et délimite ; une trace
  explique un cas précis.
- Rien sur QUI a appelé : aucune étiquette n'est jamais un utilisateur, et c'est ce qui borne
  la cardinalité. La réponse à cette question se trouve dans le
  [journal d'accès](/docs/operations/logs).
