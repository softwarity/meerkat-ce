---
title: Appels planifiés
section: Exploitation
order: 212
summary: La gateway appelle vos services à l'heure que vous indiquez, avec les rôles que la planification demande, et sans broker à installer.
---

# Appels planifiés

Un service demande à être appelé plus tard : "POST `/jobs/close` chez moi, tous les
jours". À l'heure dite, la gateway effectue cet appel **par sa propre porte
d'entrée**, avec une identité, et enregistre ce qui a répondu.

Votre service expose un endpoint - il en expose déjà. Aucune bibliothèque à
embarquer, aucun protocole maison, aucun broker à installer.

## L'appel s'exécute sous le nom `meerkat`, avec les rôles que la planification demande

Tout le modèle de sécurité est là, et il tient en une phrase : **une planification
atteint exactement ce que ses rôles permettent d'atteindre**.

Il n'y a **aucun compte** derrière un appel planifié. La planification indique les
rôles que son appel doit porter (`roles`, ou `role` s'il n'y en a qu'un), la
gateway effectue l'appel sous le nom `meerkat`, muni de ces rôles, et tout ce qui
se trouve devant le service les lit comme il lit ceux de n'importe qui : les
prédicats, les filtres, la règle de la route, les règles par endpoint, les horaires
de l'organisation. Une planification qui demande le mauvais rôle n'est pas une
faille : c'est une planification qui échoue chaque nuit, et la console conserve la
phrase qui l'explique.

```json
{ "role": "station_fetch", "routeId": "stations-api", "path": "/fetch/station" }
```

Les rôles demandés passent par la **hiérarchie du catalogue**, exactement comme ceux
d'une session : demander un rôle parent accorde ce qu'il implique.

Pas de compte de service à créer, donc pas de droits à tenir à jour indéfiniment. Ce
qui borne une planification, c'est **le jeton qui l'a écrite** : un jeton dont le
périmètre est `schedules`, émis par root, tracé dans le journal d'audit, révocable
en un clic. Deux services qui ne doivent pas atteindre les mêmes choses reçoivent
deux jetons.

Si vous tenez à savoir à qui appartient une planification, notez-le dans les
**métadonnées** : `"metadata": { "owner": "stations-svc" }` est une clé comme une
autre, et vous pouvez chercher dessus.

## Quand l'endpoint appelé est protégé

**Fermé par la règle de la route, ou par une règle d'endpoint** : rien d'autre à
mettre dans la planification que `roles`. Le service reçoit l'identité, transmise
comme elle l'est pour un humain - par des en-têtes, par `REMOTE_USER` ou par un JWT
signé, selon ce que déclare la route. Un mauvais rôle produit un 403 que la console
affiche en toutes lettres, pas une faille.

**Fermé par un secret que le service attend lui-même** - une clé qui lui est propre,
un tiers : c'est le rôle de `headers`, avec une **référence au coffre** en guise de
valeur. Le secret n'est lu qu'au moment de l'appel : ni la base de données, ni
l'API, ni la console ne le voient.

## Au moins une fois

Un appel peut réussir de votre côté et échouer sur le chemin du retour, et une
gateway peut s'arrêter au beau milieu d'un appel. Chaque appel porte donc un
identifiant d'exécution, identique à chaque tentative :

```http
POST /jobs/close
Meerkat-Job: sch_83b4f3a9...
Meerkat-Job-Run: GvbDqbNG__YTOY00
Meerkat-Job-Attempt: 1
```

**Votre service doit ignorer un identifiant qu'il a déjà vu.** Le "exactement une
fois" exigerait une transaction partagée entre la gateway et votre service, donc
une base de données commune : précisément la dépendance dont ce produit se passe.

Quand la même exécution est-elle envoyée deux fois ? Uniquement quand personne ne
peut savoir si vous l'avez reçue :

| Ce qui s'est passé | Ce que fait la gateway |
|---|---|
| Vous avez répondu, quel que soit le statut | L'exécution se clôt sur cette réponse. Ce même appel n'est **jamais** renvoyé. Quelques échecs donnent droit à une autre TENTATIVE, sous la forme d'une nouvelle exécution dotée d'un nouvel identifiant - voir "Quels échecs sont retentés" |
| Vous avez répondu `202` | Le travail est à vous. Il n'est jamais renvoyé, quoi qu'il arrive à la gateway |
| La gateway qui effectuait l'appel s'est arrêtée avant votre réponse | Une autre gateway le renvoie, avec le **même identifiant d'exécution** et un `Meerkat-Job-Attempt` augmenté de un |

La dernière ligne connaît trois bornes. **Trois tentatives** au plus, pour qu'un
appel qui fait tomber une gateway ne les fasse pas toutes tomber l'une après
l'autre. **Jamais plus tard que `catchUp`**, la même règle que pour un tour manqué
pendant que la gateway était arrêtée. Et **le délai de reprise** : immédiat quand
la gateway s'est arrêtée proprement - un déploiement progressif transmet ses
appels en cours en partant - et à l'expiration du bail, cinq minutes, quand elle est
morte brutalement.

## Un travail long répond 202

L'appel **démarre** le travail ; il n'en attend pas la fin. Un service qui répond
`202 Accepted` garde l'exécution ouverte et rend compte lui-même :

```http
PATCH /api/schedules/{id}/run
{"run": "GvbDqbNG__YTOY00", "state": "running", "progress": 60}
```

puis, à la fin, `{"state": "done", "detail": "12000 rows"}`. C'est ce qui permet
d'exprimer un traitement de trois heures sans requête HTTP de trois heures.

**Avec quel jeton ?** Le vôtre, celui qui a créé la planification, sur la même API
que tout le reste. L'appel planifié n'a apporté aucun jeton : son identité a été
établie à l'intérieur de la gateway, et n'a jamais circulé sur le réseau.

Chaque compte rendu renouvelle le **bail**. Restez silencieux cinq minutes, et
l'exécution est close comme perdue au lieu de rester en cours indéfiniment - et elle
n'est **pas** renvoyée : le 202 disait que le travail était à vous.

Toute autre réponse clôt l'exécution sur-le-champ : 2xx, c'est un succès ; le reste,
un échec, avec le statut et ce que disait la réponse - une page d'erreur est réduite
à la phrase qu'un humain y lit, jamais à son doctype.

## Quand le service est arrêté

Il ne se passe rien de particulier, et c'est tout l'intérêt : l'appel entre par la
porte d'entrée, il obtient donc ce qu'un navigateur aurait obtenu. La gateway
répond `502` pour un service qui ne répond pas, l'exécution se clôt en échec, et la
console conserve la phrase : "502 Bad Gateway: Unavailable This application is not
responding".

- **Trois tentatives, puis le tour suivant.** Un échec qui mérite une nouvelle
  tentative - voir plus bas - est retenté 30 secondes plus tard, puis 2 minutes
  plus tard, et c'est tout. Un service arrêté toute la nuit coûte trois appels,
  puis un par cadence, jamais un par seconde.
- **Aucune exécution bloquée.** L'échec clôt le tour sur-le-champ ; le bail n'a
  rien à récupérer, et la planification n'est pas retenue par un tour qui ne
  reviendra jamais.
- **Aucune contagion.** Une planification qui échoue n'en suspend aucune autre, et
  ne se suspend pas elle-même : la mise en pause est une décision d'exploitant.
  Chaque appel est d'ailleurs effectué indépendamment : un service qui prend son
  temps retient son propre appel, et celui de personne d'autre.
- **Un appel qui ne répond pas** dans le `timeout` de la planification est clos
  par nous, et le dit - "no answer within the 3s this schedule allows" - au lieu
  de laisser croire à une panne du service alors que c'est nous qui avons cessé
  d'attendre.

Un service arrêté se lit donc comme un service arrêté, et se répare en le
redémarrant : rien à rejouer, rien à débloquer.

### Quels échecs sont retentés

Derrière un appel planifié se trouve un service interne : une poignée de réponses
traduisent donc le plus souvent un **état passager** plutôt qu'un verdict - une
gateway qui vient de redémarrer, une version encore en cours de déploiement :

| La réponse | Ce qui se passe |
|---|---|
| `401`, `403`, `404` | nouvelle tentative : le compte n'est pas encore connu, les rôles ne sont pas chargés, la route d'une version en cours de démarrage n'existe pas encore |
| `502`, `503`, `504`, ou aucune réponse dans le `timeout` | nouvelle tentative : personne n'a répondu |
| tout le reste, `500` en tête | **pas** de nouvelle tentative. Là, le service DIT quelque chose, et un bogue à l'autre bout se traite à l'autre bout |
| `lost` - il a pris le travail avec un `202` puis s'est tu | pas de nouvelle tentative : le travail est chez lui |

Une nouvelle tentative est une **nouvelle exécution**, avec son propre
identifiant : un service qui déduplique aurait sinon écarté un appel qu'il n'a
jamais traité. C'est en revanche le même **tour** : les tentatives se comptent donc
ensemble - trois au plus - et l'historique les relie.

Deux bornes s'ajoutent aux trois tentatives : le `catchUp` de la planification,
parce qu'un appel qui ne vaut que s'il part à l'heure ne vaut plus rien vingt
minutes plus tard, et l'attente croissante entre les tentatives, parce que ce que
l'on attend, c'est le redémarrage d'un service.

### Quand le service ne peut pas encore l'exécuter

Rien n'est cassé, le travail ne peut simplement pas être fait maintenant :
l'extraction n'est pas publiée, une dépendance n'a pas répondu. Le service sait
pourquoi, et il sait quand revenir - il le dit donc, au lieu de laisser la
gateway deviner :

```http
HTTP/1.1 424 Failed Dependency
Retry-After: 600

the daily extract is not published yet
```

Le tour revient à ce moment-là. `424` et non `503` : un `503` dit "je suis arrêté",
ce que les nouvelles tentatives couvrent déjà, alors qu'un `424` dit "je vais bien,
c'est ce dont j'ai besoin qui manque" - ce que seul le service peut savoir.

Pour un travail accepté avec un `202`, la même réponse passe par le compte rendu :

```json
{"run": "GvbDqbNG__YTOY00", "state": "failed",
 "detail": "the daily extract is not published yet", "retryIn": "PT10M"}
```

Dans les deux cas, la **raison reste attachée à l'exécution qui l'a donnée**, et le
tour suivant y renvoie : l'historique se lit "pas encore publié, reporté à 10 h 15,
terminé" comme une seule chaîne.

Trois bornes, vers lesquelles la valeur est ramenée plutôt que refusée - une
réponse raisonnable ne doit jamais coûter un tour : **une minute** au plus tôt, **un
jour** au plus tard, et **cinq reports d'affilée** au plus. Un service qui a demandé
cinq fois un report n'attend pas un état passager, il est en panne : le tour est
abandonné, la cadence propre à la planification reprend la main, et l'écran dit
pourquoi.

## L'API, et où elle se trouve

Sur le **plan de contrôle**, pas sur celui où répondent vos applications. Un appel
planifié est un service rendu par la gateway, comme l'endpoint destiné aux
agents : aucun navigateur ne l'appelle. C'est un **backend** qui le fait, depuis
l'intérieur du cluster, par le nom interne de la gateway.

```
http://meerkat:9090/api/schedules
```

| | |
|---|---|
| `POST /api/schedules` | créer - l'identifiant revient dans la réponse |
| `GET /api/schedules` | la liste, filtrée |
| `GET /api/schedules/{id}` | une planification |
| `PUT /api/schedules/{id}` | la modifier |
| `DELETE /api/schedules/{id}` | la supprimer |
| `POST /api/schedules/{id}/pause` et `/resume` | l'empêcher de se déclencher, l'y autoriser de nouveau |
| `POST /api/schedules/{id}/run` | avancer le prochain tour |
| `GET /api/schedules/{id}/runs` | ce qu'a fait chaque tour, du plus récent au plus ancien |
| `PATCH /api/schedules/{id}/run` | rendre compte de l'exécution en cours |

### Le jeton

Un jeton du plan de contrôle dont le **périmètre est `schedules`** : il ouvre cette
API et rien d'autre sur ce port - ni la configuration, ni les comptes, ni les
routes. Il est étroit à dessein : il vit dans un manifeste de déploiement, souvent
dans le dépôt d'une autre équipe, et c'est celui que personne ne pense à
renouveler.

**Ce que les appels peuvent atteindre** ne se décide pas ici : c'est le champ
`roles` de la planification elle-même. Un backend garde donc **un seul jeton** et
planifie des appels vers autant d'endpoints différents qu'il en sert - un jeton par
endpoint ferait un secret de plus à renouveler, pour une question à laquelle la
planification répond déjà.

### Les identifiants sont les nôtres

`POST` crée la planification et renvoie son identifiant ; tout le reste le prend en
paramètre. Vous préférez ne pas le conserver ? Classez la planification sous vos
propres **métadonnées** et retrouvez-la par ce biais - c'est ce que fait une boucle
de réconciliation.

Une planification désigne une **route**, jamais une URL : un service qui déménage
garde ses planifications.

```json
{
  "name": "station 42, hourly poll",
  "roles": ["station_fetch"],
  "method": "POST",
  "routeId": "stations-api",
  "path": "/fetch/station",
  "every": "PT1H",
  "headers": { "X-Action": "fetch", "X-Api-Key": "${stations-key}" },
  "body": { "kind": "poll", "region": "west", "station": "42" },
  "metadata": { "svc": "stations", "station": "42", "env": "prod" }
}
```

### Le verbe, les en-têtes, le corps

**`method`** est le verbe de l'appel **sortant**, celui que la gateway enverra au
service : `POST` par défaut ; `POST`, `PUT`, `PATCH` et `DELETE` sont acceptés.
`GET` est refusé, et le message explique pourquoi : un appel planifié est une
action, et une planification qui ne fait que lire est une planification dont
personne ne regarde la réponse.

**`headers`** porte ce que le service a demandé à recevoir, en plus de ce que la
gateway écrit d'elle-même. Dix au maximum. Une valeur peut être une **référence
au coffre** - `${stations-key}` ou `$stations-key` - résolue au moment de l'appel,
dans la portée de l'application, ou dans celle de l'organisation quand la
planification en a une : la ligne conserve la référence, si bien que le secret
n'apparaît ni dans les réponses de cette API ni dans la console. Un nom absent du
coffre part **tel quel**, `$typo`, au lieu de devenir un en-tête vide, dont l'échec
serait bien plus déroutant.

Cinq noms sont refusés dès l'écriture de la planification, chacun avec sa raison :
`Authorization`, parce que la gateway l'écrit et qu'il s'agit de l'identité du
compte sous lequel la planification s'exécute ; `Host`, parce que c'est la route
qui en décide ; `Meerkat-Job`, `Meerkat-Job-Run` et `Meerkat-Job-Attempt`, parce que
la gateway les écrit et que c'est grâce à eux que votre service reconnaît un tour
qu'il a déjà vu.

**`body`** est du JSON. Un objet ou un tableau part tel quel, avec
`Content-Type: application/json`. Une **chaîne** JSON part sous la forme de son
contenu, en `text/plain; charset=utf-8` par défaut : c'est ainsi que l'on envoie un
formulaire, un document XML ou une ligne de texte depuis un champ qui, par
ailleurs, est du JSON. Un `contentType` explicite l'emporte toujours, puisque c'est
celui que le service a demandé.

### Les métadonnées : le système de classement du service

`metadata` est un objet libre, le vôtre : les clés que vous voulez, des valeurs
textuelles, que Meerkat stocke et sur lesquelles il filtre sans jamais chercher à
savoir ce qu'elles signifient. C'est ce qui fait qu'**un seul jeton suffit** - ce
sont les métadonnées qui distinguent une planification parmi mille.

Vous les retrouvez de la même manière :

```bash
# une station
curl "http://meerkat:9090/api/schedules?meta.station=42" -H "$AUTH"

# deux conditions, toutes deux requises
curl "http://meerkat:9090/api/schedules?meta.kind=poll&meta.region=west" -H "$AUTH"

# une expression, pour toute une flotte
curl "http://meerkat:9090/api/schedules?meta.station=~^[0-9]{1,2}$" -H "$AUTH"
```

`meta.<key>=<value>` correspond à une égalité stricte, `meta.<key>=~<expression>` à
une expression régulière. Deux autres filtres portent sur ce que la gateway sait
d'elle-même : `tenant` et `route`. Une expression qui ne compile pas produit un
refus qui le dit, jamais une liste vide qui se lirait comme "il n'y a rien".

Vingt clés au plus par planification : c'est un système de classement, pas un
endroit où stocker vos données.

> [!NOTE]
> Vos utilisateurs ne parlent jamais à Meerkat. Le service des stations demande ce
> qui appartient à la station qu'il a devant lui, et remet à son client ce qu'il
> décide de lui montrer.

## Une cadence, un calendrier ou une date unique

Une planification dit **quand** de trois manières possibles, jamais deux à la fois.

**`every`, une cadence** : une durée ISO 8601 - `PT30M`, `PT6H`, `P1D` - comptée à
partir de la **fin** de la dernière exécution. La plus courte est d'une minute.
C'est ce qu'attend un "toutes les six heures", et c'est ce qui évite à un
traitement lent de se redéclencher à l'instant même où il se termine.

**`cron`, un calendrier** : les cinq champs standard, avec les listes, les
intervalles, les pas, les noms `MON`-`SUN` et `JAN`-`DEC`, et les raccourcis
`@hourly`, `@daily`, `@weekly`, `@monthly`, `@yearly`.

```json
{
  "name": "monday report",
  "routeId": "orders-api",
  "method": "POST",
  "path": "/jobs/report",
  "cron": "0 3 * * MON",
  "timezone": "Europe/Paris"
}
```

**`at`, une date unique** : un instant RFC 3339, avec son décalage -
`2026-10-03T04:00:00Z`, `2026-10-03T06:00:00+02:00`. L'appel part à ce moment-là,
**une seule fois**, et la planification est **terminée** : rien n'est armé ensuite.

```json
{
  "name": "cart 4471, reminder",
  "routeId": "orders-api",
  "path": "/jobs/remind",
  "at": "2026-10-03T04:00:00Z",
  "metadata": { "cart": "4471" }
}
```

C'est l'**action différée** : le déclencheur est un événement qui s'est produit -
un panier abandonné, une réservation effectuée, un document déposé - et non une
échéance de calendrier. Votre service traite l'événement, envoie la date à
laquelle il veut être rappelé, et n'y pense plus.

Ce "une seule fois" a quelques conséquences :

- **La date porte son propre décalage** : `timezone` n'a donc aucun sens à côté
  d'elle, et il est refusé. `startAt` est refusé lui aussi : il fixe le premier
  tour de quelque chose qui se répète, et une date unique est son propre premier
  tour.
- **Une date déjà passée n'est pas une erreur** : l'appel part immédiatement si
  `catchUp` l'autorise encore, et il est abandonné sinon - exactement comme un
  tour manqué pendant que la gateway était arrêtée.
- **La ligne reste** une fois l'appel parti, avec son résultat, pour que l'on
  puisse constater qu'il est bien parti. Elle est purgée à l'issue d'une durée de
  rétention que root choisit sur l'écran Scheduler, un mois par défaut. Une
  planification qui se répète n'est jamais purgée.
- **Déplacer sa date** avec un `PUT` déplace le tour : c'est ainsi que l'on
  repousse une action différée, et que l'on donne une nouvelle date à une action
  terminée.

Choisissez le calendrier pour "tous les lundis à trois heures" ou "le premier du
mois" : une cadence ne sait pas le dire, et elle **dérive** un peu à chaque tour.

Choisissez la cadence pour une période. Un cron n'en exprime aucune : ses champs
sont des positions dans un calendrier, pas des pas dans une suite.

```
0 */7 * * *    00:00, 07:00, 14:00, 21:00, puis minuit    -> un trou de 3 heures
*/45 * * * *   0, 45, puis 60, puis 105                   -> un trou de 15 minutes
*/20 * * * *   0, 20, 40, 60                              -> celui-là tombe juste
```

`*/n` n'est une période que si `n` divise 60 ou 24. "Toutes les 90 minutes"
s'écrit `PT90M`, et pas autrement. Une cadence n'a en outre ni fuseau à choisir ni
nuit de changement d'heure à traverser, et cinquante planifications en `PT1H` se
répartissent d'elles-mêmes, là où cinquante planifications en `0 * * * *` partent
toutes à la même seconde.

Comme dans toute crontab, quand les **deux** champs de jour sont restreints, un
jour qui correspond à l'un **ou** à l'autre est retenu : `0 0 1 * MON` désigne le
premier du mois et tous les lundis.

### Un calendrier se lit quelque part

`timezone` est un nom IANA - `Europe/Paris` - et vaut **UTC** si vous ne précisez
rien : "trois heures du matin" signifie trois heures UTC tant que vous n'avez pas
écrit où. Un fuseau que cette gateway ne connaît pas est refusé dès l'écriture,
plutôt que remplacé discrètement par UTC.

Les deux nuits qui comptent sont prises en charge : au printemps, l'heure qui
**n'existe pas** décale le tour ; à l'automne, celle qui **se produit deux fois**
ne déclenche qu'un seul appel. Une cadence ignore ce champ.

## Trois réglages qui évitent trois surprises

| Réglage | Ce qu'il évite |
|---|---|
| `overlap` (`skip` par défaut) | Un traitement lent qui n'est pas terminé : le tour suivant ne part pas. Rien d'autre n'est dû tant qu'une exécution est en cours, et c'est sa clôture qui **arme le tour suivant**, compté à partir de là - un traitement lent ne trouve donc jamais, à l'instant où il se termine, un tour qui l'attend dans le passé |
| `catchUp`, en secondes | Une gateway arrêtée toute la nuit qui déclenche douze tours à son réveil. Zéro signifie "exécutez-le dès que possible". Ce réglage borne aussi le retard avec lequel est renvoyé un appel interrompu par l'arrêt d'une gateway, et décide si une action différée dont l'heure est passée pendant un arrêt part encore |
| `timeout` | Un appel qui ne répond jamais. Il borne la **réponse**, pas le travail : c'est à cela que sert le 202. Deux minutes au plus (`PT2M`) |

## Ce que chaque tour laisse derrière lui

La planification elle-même conserve le **dernier** résultat, celui qu'affiche une
liste. Chaque tour terminé est également conservé séparément, et c'est ce qui
répond à "depuis quand est-ce en échec ?", "la clôture de cette nuit a-t-elle
tourné ?" et "qu'a-t-il répondu ?" :

```bash
curl "http://meerkat:9090/api/schedules/sch_9f2c/runs" -H "$AUTH"
```

```json
[
  {
    "id": "run-17906f3c9d2a4e10-9f2c11",
    "runId": "GvbDqbNG__YTOY00",
    "attempt": 1,
    "node": "meerkat-7d9c",
    "startedAt": 1790770190,
    "endedAt": 1790770191,
    "state": "done",
    "status": 200,
    "detail": "200 OK"
  }
]
```

Un tour se termine de quatre façons, et la différence compte :

| | |
|---|---|
| `done` | le service a répondu 2xx |
| `failed` | il a répondu autre chose, ou personne n'a répondu avant le timeout |
| `lost` | il a pris le travail avec un `202`, puis s'est tu au-delà de la durée du bail |
| `dropped` | le tour **n'est jamais parti** : son heure est arrivée plus tard que `catchUp` ne l'autorise |

`attempt` indique quelle tentative a terminé le tour : `2` signifie qu'une
gateway s'est arrêtée avant la réponse et qu'une autre a renvoyé le même appel.
`cause` et `ofRun` forment la chaîne - un tour ordinaire ne dit rien, une
réexécution nomme l'exécution qu'elle prolonge.

### Rejouer un tour

Un tour abandonné, ou en échec pour une raison corrigée depuis, se redemande par
son nom :

```bash
curl -X POST "http://meerkat:9090/api/schedules/sch_9f2c/run" -H "$AUTH" \
  -H "Content-Type: application/json" \
  -d '{"replayOf":"run-17906f3c9d2a4e10-9f2c11"}'
```

Il part comme une **nouvelle exécution**, avec un nouvel identifiant et **le
contenu d'aujourd'hui** - la planification telle qu'elle est maintenant, pas telle
qu'elle était cette nuit-là. Ce qu'il rejoue est inscrit dans l'historique, si bien
que les deux se lisent comme une seule chaîne. Sans corps, le même appel est le
simple "run now".

L'historique est purgé selon la même rétention qu'une action différée terminée -
un mois par défaut, modifiable par root - et disparaît avec la planification
quand elle est supprimée.

## Quand la gateway regarde, et en cluster

**Il n'y a pas de tic d'horloge.** La gateway dort jusqu'à la prochaine échéance
et se réveille à la seconde près. Écrire une planification, la mettre en pause,
l'avancer ou clore une exécution la réveille aussitôt ; un filet de sécurité couvre
le reste, une fois par minute.

Plusieurs gateways partagent une même table et **rien ne pose de verrou** :
prendre un tour est un `UPDATE` conditionnel, de sorte que deux gateways au même
instant ne produisent qu'un seul gagnant. Les appels **se répartissent entre les
gateways**, plusieurs à la fois sur chacune - un service lent retient son propre
appel, et celui de personne d'autre.

Ce qu'un cluster apporte réellement, c'est le **bail** : une exécution dont
personne ne dit rien pendant cinq minutes est réglée par la première gateway qui
la voit, renvoyée ou close comme perdue.

Tant qu'une exécution est en cours, rien d'autre n'est dû : "run now" répond `409`,
et c'est la clôture de l'exécution qui arme le tour suivant.

## Ce que ce n'est pas

**Une file de messages.** Pas de diffusion vers plusieurs consommateurs, pas
d'acquittement, pas de file de messages rejetés : une planification, un appel, au
moins une fois. Si vous avez besoin du reste, il vous faut un broker.

> [!TIP]
> Vous utilisez déjà des CronJobs Kubernetes et vous voulez qu'ils restent
> l'horloge ? Faites appeler `POST /api/schedules/{id}/run` par l'un d'eux, avec un
> jeton d'API - l'appel répond `409` tant que l'exécution précédente est en cours.
> La gateway garde l'identité, le cloisonnement et l'historique ; votre cluster
> garde le calendrier. Meerkat n'a besoin d'aucun droit sur votre cluster pour
> cela.

## Ce que voit l'exploitant

**Scheduler**, dans le rail, liste tout ce qui est planifié, avec un filtre par
organisation et par service, et se met à jour en direct : une exécution qui
démarre, progresse et se termine apparaît sans rien rafraîchir. Trois actions s'y
trouvent, celles dont on a besoin à deux heures du matin : mettre en pause, avancer
le prochain tour, supprimer. Il n'y a volontairement aucun bouton de création : une
planification appartient au service qui en a besoin, et c'est lui qui la crée par
l'API, depuis ses propres écrans.

![L'écran Scheduler : une cadence, une ligne cron et une date unique, chacune avec sa prochaine exécution et sa dernière](img/console/scheduler.webp)

Une exécution en cours indique où elle en est - **calling**, en attente de la
réponse, ou **accepted**, le service a pris le travail - et de quelle tentative il
s'agit quand une gateway s'est arrêtée avant la réponse. Son prochain tour
affiche "after this run" : c'est la clôture qui l'arme, et "Run now" attend ce
moment.

Le tiroir d'une planification conserve **tous les tours qu'elle a exécutés** :
quand chacun s'est terminé, comment, ce qui a répondu, quel nœud a effectué
l'appel, de quelle tentative il s'agissait - et un bouton pour rejouer un tour
abandonné ou en échec.

Une **action différée** déjà partie affiche "finished" et garde son résultat à
l'écran jusqu'à la purge. Un interrupteur les masque quand un service en envoie
assez pour noyer le reste, et root choisit juste à côté leur durée de conservation.

Les heures sont affichées dans le **fuseau de l'exploitant** - celui de son profil,
défini une fois et lu par les deux plans - ou en **UTC**, grâce à un bouton qui
indique lequel est affiché : "2:24 AM" ne veut rien dire tant que l'on ne sait pas
de quel 2:24 il s'agit.

Un bouton **API** ouvre, dans le même tiroir, le mécanisme et l'API : les commandes
et le contenu de la requête sont déjà remplis avec l'adresse de **cette**
installation, puisque l'écran ne crée rien et que l'étape suivante est un appel que
le service effectue lui-même.

Le tiroir d'une planification conserve, replié, le **contenu qui permet de la
recréer** : les champs qu'une ligne ne peut pas montrer - le corps envoyé, les trois
réglages - et un exemple complet pour celui qui devra écrire la suivante.

Le filtre **Metadata** de cet écran accepte ce qu'accepte l'API - `station=42`, ou
`station=~^st-` - parce qu'un exploitant qui cherche une planification dans une
flotte la cherche d'après ce que le service a écrit dessus.

La création d'une planification n'en fait pas partie, à dessein : une planification
s'exécute sous l'identité d'un compte, et les administrateurs qui lisent cet écran
ne sont pas les comptes sous lesquels elle doit s'exécuter. L'API la refuse
également, en toutes lettres : la création revient au service, avec son propre
jeton.
