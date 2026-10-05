---
title: Health checks
section: Exploitation
order: 213
summary: Ce à quoi répondent /healthz et /readyz, et pourquoi la liveness probe n'interroge jamais la base de données.
---

# Health checks

Deux health checks, servis sur les **deux** plans, répondent à deux questions différentes.
Brancher un orchestrateur sur le mauvais est l'erreur la plus coûteuse que l'on puisse faire
ici : ils sont donc volontairement séparés.

## /healthz est la vivacité

Elle répond `UP` sans condition, avec la version.

C'est la bonne réponse, et non une réponse de facilité. La vivacité sert à décider s'il faut
**tuer** le processus. Une liveness probe qui échouerait parce que la base de données est
injoignable transformerait un incident passager de la base en redémarrage de tous les nœuds à
la fois - chacun tué pour une panne qui n'est pas la sienne, et que sa mort ne répare pas. Ce
qui relève d'une dépendance relève de la disponibilité.

```json
{"status":"UP","version":"dev"}
```

C'est aussi la **startup probe**. La base est ouverte et migrée avant l'ouverture des
ports : une gateway en cours de migration ne répond donc pas encore. La startup probe
lui accorde jusqu'à deux minutes avant que la vivacité commence à compter. Sans elle, une
longue migration après une montée de version serait tuée à mi-chemin et recommencerait, en
boucle.

## /readyz est la disponibilité

Elle sert à décider s'il faut **envoyer du trafic**. Elle pose donc les deux questions dont
dépend l'utilité d'un nœud :

- **la base répond-elle ?** Un ping, limité à deux secondes. Une readiness probe qui
  reste bloquée est une probe qui ne dit rien, et c'est alors le timeout de
  l'orchestrateur qui tranche, sans aucune information.
- **le routeur a-t-il compilé sa table au moins une fois ?** Un nœud qui accepte des
  connexions sans avoir jamais compilé sa table répond `404` à tout. Une mise à jour
  progressive y voit sans hésiter une instance saine et lui envoie du trafic réel.

En cas d'échec, la réponse est un `503` **accompagné du motif**, car l'exploitant qui lit un
échec de health check doit savoir laquelle des deux conditions a manqué.

```json
{"status":"DOWN","reason":"the store is not answering","version":"dev"}
```

```json
{"status":"DOWN","reason":"the routing table has not been compiled yet","version":"dev"}
```

Un nœud qui a perdu sa base de données conserve sa table compilée et répond encore aux
requêtes, alors qu'il ne peut plus résoudre une session, lire un réglage ni recharger une
route. C'est exactement l'état qui, auparavant, se déclarait prêt.

## Les deux échappent à la redirection HTTPS

Lorsque le port en clair redirige vers HTTPS, ces deux chemins en sont exemptés : la plupart
des health checks interprètent un `308` comme "pas prêt", et une gateway serait retirée de la
rotation précisément parce qu'elle est bien configurée.

## En cluster

Le load balancer sonde `/readyz` et **jamais** `/healthz`. Il n'a aucune affinité de
session à assurer - les sessions sont en base de données, et le cache de cinq secondes est
invalidé par le bus de changement - ni aucun nœud primaire à viser, puisque le verrou
consultatif protège tout ce qui ne doit se produire qu'une fois.
