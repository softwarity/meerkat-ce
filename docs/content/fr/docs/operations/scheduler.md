---
title: Appels planifiés
section: Exploitation
order: 212
summary: La passerelle appelle vos services à l'heure dite, avec l'identité du compte à qui la tâche appartient, et sans courtier à installer.
---

# Appels planifiés

Un service demande qu'on l'appelle plus tard : « POST `/jobs/close` sur moi,
tous les jours ». À l'heure dite, la passerelle fait cet appel **par sa propre
porte d'entrée**, avec une identité, et note ce qui a répondu.

Votre service expose un endpoint - il en expose déjà. Pas de bibliothèque à
embarquer, pas de protocole à nous, pas de courtier à installer.

## Elle s'exécute comme `meerkat`, avec les rôles que la tâche demande

C'est tout le modèle de sécurité, et il tient en une phrase : **une tâche
atteint exactement ce que ses rôles atteignent**.

Il n'y a **aucun compte** derrière un appel planifié. La tâche dit les rôles
que son appel doit porter (`roles`, ou `role` s'il n'y en a qu'un), la
passerelle fait l'appel sous le nom `meerkat` muni de ces rôles, et tout ce qui
est posé devant le service les lit comme ceux de n'importe qui : prédicats,
filtres, règle de la route, règles par endpoint, horaires de l'organisation.
Une tâche qui demande le mauvais rôle n'est pas un trou : c'est une tâche qui
échoue tous les soirs, et la console garde la phrase qui le dit.

```json
{ "role": "station_fetch", "routeId": "stations-api", "path": "/fetch/station" }
```

Les rôles demandés passent par la **hiérarchie du catalogue**, exactement comme
ceux d'une session : demander un rôle parent donne ce qu'il implique.

Aucun compte de service à créer, donc aucun droit à tenir à jour à vie. Ce qui
borne une tâche, c'est le **jeton qui l'a écrite** : un jeton de périmètre
`schedules`, frappé par root, tracé à l'audit, révocable en un clic. Deux
services qui ne doivent pas atteindre les mêmes choses reçoivent deux jetons.

Si vous voulez quand même savoir à qui une tâche appartient, rangez-le dans les
**métadonnées** : `"metadata": { "owner": "stations-svc" }` est une clé comme
une autre, sur laquelle vous cherchez.

## Quand l'endpoint appelé est protégé

**Fermé par la règle de la route, ou par une règle d'endpoint** : rien à mettre
dans la tâche, à part `roles`. Le service reçoit l'identité transmise comme
pour un humain - en en-têtes, en `REMOTE_USER` ou en JWT signé, selon ce que la
route déclare. Un mauvais rôle donne un 403 que la console affiche en toutes
lettres, pas un trou.

**Fermé par un secret que le service attend lui-même** - une clé maison, un
tiers : c'est là que `headers` sert, avec une **référence de coffre** pour
valeur. Le secret n'est lu qu'au moment de l'appel : ni la base, ni l'API, ni
la console ne le voient.

## Au moins une fois

Un appel peut réussir chez vous et échouer au retour, et une passerelle peut
s'arrêter au milieu d'un appel. Chaque appel porte donc un identifiant
d'exécution, le même à chaque tentative :

```http
POST /jobs/close
Meerkat-Job: sch_83b4f3a9...
Meerkat-Job-Run: GvbDqbNG__YTOY00
Meerkat-Job-Attempt: 1
```

**Votre service doit ignorer un identifiant qu'il a déjà vu.** L'« exactement
une fois » demanderait une transaction partagée entre la passerelle et votre
service, donc une base commune : c'est précisément la dépendance dont ce
produit se passe.

Quand la même exécution part-elle deux fois ? Seulement quand personne ne peut
savoir si vous l'avez reçue :

| Ce qui s'est passé | Ce que fait la passerelle |
|---|---|
| Vous avez répondu, quel que soit le statut | L'exécution se ferme sur cette réponse. Ce même appel n'est **jamais** renvoyé. Quelques échecs donnent une nouvelle TENTATIVE, sous la forme d'une nouvelle exécution avec un nouvel identifiant - voir « Quels échecs sont réessayés » |
| Vous avez répondu `202` | Le travail est à vous. Jamais renvoyé, quoi qu'il arrive à la passerelle |
| La passerelle qui faisait l'appel s'est arrêtée avant votre réponse | Une autre passerelle le renvoie, **même identifiant d'exécution**, `Meerkat-Job-Attempt` augmenté de un |

La dernière ligne a trois bornes. **Trois tentatives** au plus, pour qu'un
appel qui fait tomber une passerelle ne les fasse pas tomber toutes à tour de
rôle. **Jamais plus tard que `catchUp`**, la même règle que pour un tour manqué
pendant que la passerelle était arrêtée. Et **dans quel délai** : tout de suite
quand la passerelle s'est arrêtée proprement - un déploiement progressif rend
ses appels en cours en partant - et à l'expiration du bail, cinq minutes, quand
elle est morte.

## Les travaux longs répondent 202

L'appel **déclenche** le travail, il ne l'attend pas. Un service qui répond
`202 Accepted` garde l'exécution ouverte et rapporte lui-même :

```http
PATCH /api/schedules/{id}/run
{"run": "GvbDqbNG__YTOY00", "state": "running", "progress": 60}
```

puis, à la fin, `{"state": "done", "detail": "12000 lignes"}`. C'est ce qui
rend un travail de trois heures exprimable sans requête HTTP de trois heures.

**Avec quel jeton ?** Le vôtre, celui qui a créé la tâche, sur la même API que
tout le reste. L'appel planifié, lui, n'apportait aucun jeton : son identité a
été posée à l'intérieur de la passerelle, et n'a jamais circulé sur le réseau.

Chaque rapport renouvelle le **bail**. Sans nouvelles pendant cinq minutes,
l'exécution est close comme perdue plutôt que de rester figée pour toujours -
et elle n'est **pas** renvoyée : le 202 a dit que le travail était à vous.

Toute autre réponse ferme l'exécution sur place : 2xx réussie, le reste
échouée, avec le statut et ce que la réponse dit - une page d'erreur est
réduite à la phrase qu'un humain y lit, jamais à son doctype.

## Quand le service est éteint

Rien de particulier, et c'est voulu : l'appel entre par la porte d'entrée, donc
il reçoit ce qu'un navigateur aurait reçu. La passerelle répond `502` pour un
service qui ne répond pas, l'exécution se ferme en échec, et la console garde
la phrase : « 502 Bad Gateway: Unavailable This application is not responding ».

- **Trois tentatives, puis le tour suivant.** Un échec qui mérite une nouvelle
  tentative - voir plus bas - repart 30 secondes plus tard, puis 2 minutes plus
  tard, et c'est tout. Un service éteint toute la nuit coûte trois appels puis
  un par cadence, jamais un par seconde.
- **Pas d'exécution figée.** L'échec ferme le tour sur place ; le bail n'a rien
  à récupérer, et la tâche n'est pas bloquée par un tour qui ne reviendra pas.
- **Pas de contagion.** Une tâche qui échoue n'en suspend aucune autre, et ne
  se suspend pas elle-même : suspendre est une décision d'exploitant. Chaque
  appel est aussi fait à part : un service qui prend son temps retient son
  propre appel, et celui de personne d'autre.
- **Un appel qui n'aboutit pas** dans le `timeout` de la tâche est fermé par
  nous, et le dit ainsi - « no answer within the 3s this schedule allows » -
  plutôt que de laisser croire à une panne du service alors que c'est nous qui
  avons cessé d'attendre.

Autrement dit, un service absent se lit comme un service absent, et se répare
en le rallumant : rien à reprendre, rien à débloquer.

### Quels échecs sont réessayés

Derrière un appel planifié il y a un service interne, donc une poignée de
réponses sont le plus souvent un **moment** et non un verdict - une passerelle
qui vient de redémarrer, une version qui monte encore :

| La réponse | Ce qui se passe |
|---|---|
| `401`, `403`, `404` | réessayée : le compte n'est pas encore connu, les rôles pas encore chargés, la route d'une version qui monte pas encore là |
| `502`, `503`, `504`, ou aucune réponse dans le `timeout` | réessayée : personne n'a répondu |
| tout le reste, et le `500` en premier | **pas** réessayée. Là, le service DIT quelque chose, et un bug à l'autre bout se traite à l'autre bout |
| `lost` - il a pris le travail avec un `202` puis s'est tu | pas réessayée : il a le travail |

Une tentative qui repart est une **nouvelle exécution**, avec son propre
identifiant : un service qui déduplique aurait jeté un appel sur lequel il n'a
jamais rien fait. C'est le même **tour**, donc les tentatives se comptent
ensemble - trois au maximum - et l'historique les relie.

Deux bornes en plus des trois tentatives : le `catchUp` de la tâche, parce
qu'un appel qui ne vaut que s'il part à l'heure ne vaut plus vingt minutes plus
tard, et l'attente croissante entre deux tentatives, parce que ce qu'on attend
est un service qui remonte.

### Quand le service ne peut pas encore le faire

Rien n'est cassé, le travail ne peut simplement pas être fait maintenant :
l'extraction n'est pas publiée, une dépendance n'a pas répondu. Le service
sait pourquoi, et il sait quand revenir - alors il le dit, au lieu de laisser
la passerelle deviner :

```http
HTTP/1.1 424 Failed Dependency
Retry-After: 600

l'extraction du jour n'est pas encore publiée
```

Le tour revient à ce moment-là. `424` et non `503` : un `503` dit « je suis
éteint », ce que la reprise automatique couvre déjà, là où `424` dit « moi je
vais bien, c'est ce dont j'avais besoin qui manque » - et ça, seul le service
peut le savoir.

Pour un travail accepté avec un `202`, la même réponse voyage sur le rapport :

```json
{"run": "GvbDqbNG__YTOY00", "state": "failed",
 "detail": "l'extraction du jour n'est pas encore publiée", "retryIn": "PT10M"}
```

Dans les deux cas, la **raison reste sur l'exécution qui l'a donnée**, et le
tour qui suit pointe vers elle : l'historique se lit « pas encore publiée,
reporté à 10 h 15, réussi » comme une seule chaîne.

Trois bornes, et elles ramènent au lieu de refuser - une réponse raisonnable ne
doit jamais coûter un tour : **une minute** au plus tôt, **un jour** au plus
tard, **cinq fois de suite** au maximum. Un service qui a demandé cinq fois
n'attend pas un moment, il est cassé : le tour est lâché, la cadence de la
tâche reprend, et l'écran dit pourquoi.

## L'API, et où elle vit

Sur le **plan de contrôle**, pas sur celui de vos applications. Une tâche
planifiée est un service que rend la passerelle, au même titre que le point
d'entrée des agents : aucun navigateur ne l'appelle. C'est un **backend** qui
l'appelle, depuis l'intérieur du cluster, par le nom interne de la passerelle.

```
http://meerkat:9090/api/schedules
```

| | |
|---|---|
| `POST /api/schedules` | créer - l'identifiant revient dans la réponse |
| `GET /api/schedules` | la liste, filtrable |
| `GET /api/schedules/{id}` | l'une d'elles |
| `PUT /api/schedules/{id}` | modifier |
| `DELETE /api/schedules/{id}` | supprimer |
| `POST /api/schedules/{id}/pause` et `/resume` | suspendre, reprendre |
| `POST /api/schedules/{id}/run` | avancer le prochain tour |
| `GET /api/schedules/{id}/runs` | ce qu'a fait chaque tour, du plus récent au plus ancien |
| `PATCH /api/schedules/{id}/run` | rapporter sur l'exécution en cours |

### Le jeton

C'est un jeton du plan de contrôle, de **périmètre `schedules`** : il ouvre
cette API et rien d'autre sur ce port - ni la configuration, ni les comptes, ni
les routes. Étroit exprès : il vit dans un manifeste de déploiement, souvent dans le dépôt
d'une autre équipe, et c'est celui que personne ne pense à faire tourner.

**Ce que les appels atteignent** ne se décide pas là : c'est le champ `roles`
de la tâche elle-même. Un backend garde donc **un seul jeton** et planifie pour
autant d'endpoints différents qu'il en sert - un jeton par endpoint serait un
secret de plus à faire tourner, pour une question à laquelle la tâche répond
déjà.

### Les identifiants sont les nôtres

`POST` crée et renvoie l'identifiant ; tout le reste le prend. Vous ne voulez
pas le conserver ? Classez la tâche sous vos propres **métadonnées** et
redemandez-la comme ça - c'est ce que fait une boucle de réconciliation.

Une tâche nomme une **route**, jamais une URL : un service qui déménage garde
ses tâches.

```json
{
  "name": "station 42, relève horaire",
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

**`method`** est le verbe de l'appel **sortant**, celui que la passerelle
enverra au service : `POST` par défaut, et `POST`, `PUT`, `PATCH` ou `DELETE`
acceptés. `GET` est refusé, et le message le dit : un appel planifié est une
action, une tâche qui ne fait que lire est une tâche dont personne ne regarde
la réponse.

**`headers`** porte ce que le service demande à s'entendre dire, en plus de ce
que la passerelle écrit elle-même. Dix au maximum. Une valeur peut être une
**référence de coffre** - `${stations-key}` ou `$stations-key` - résolue au
moment de l'appel, dans la portée de l'application ou dans celle de
l'organisation quand la tâche en a une : la ligne en base garde la référence,
donc le secret n'apparaît ni dans la réponse de l'API ni dans la console. Un
nom que le coffre ne tient pas part **tel quel**, `$typo`, plutôt que de
devenir un en-tête vide qui échouerait de façon bien plus déroutante.

Cinq noms sont refusés à l'écriture, chacun avec sa raison :
`Authorization`, parce que la passerelle l'écrit et que c'est l'identité du
compte sous lequel la tâche tourne ; `Host`, parce que c'est la route qui
décide ; `Meerkat-Job`, `Meerkat-Job-Run` et `Meerkat-Job-Attempt`, parce que
la passerelle les écrit et que c'est à eux que votre service reconnaît un tour
qu'il a déjà vu.

**`body`** est du JSON. Un objet ou un tableau part tel quel, avec
`Content-Type: application/json`. Une **chaîne** JSON part comme son contenu,
en `text/plain; charset=utf-8` par défaut : c'est ainsi qu'on envoie un
formulaire, un document XML ou une ligne de texte depuis un champ qui est par
ailleurs du JSON. Un `contentType` explicite gagne toujours, puisque c'est
celui que le service a demandé.

### Les métadonnées : le classement du service

`metadata` est un objet libre, à vous : les clés que vous voulez, des valeurs
texte, que Meerkat stocke et compare sans jamais chercher à savoir ce qu'elles
veulent dire. C'est ce qui fait qu'**un seul jeton suffit** - ce sont elles qui
distinguent une tâche parmi mille.

On les interroge en retour :

```bash
# une station
curl "http://meerkat:9090/api/schedules?meta.station=42" -H "$AUTH"

# deux conditions, toutes les deux exigées
curl "http://meerkat:9090/api/schedules?meta.kind=poll&meta.region=west" -H "$AUTH"

# une expression regulière, pour une flotte
curl "http://meerkat:9090/api/schedules?meta.station=~^[0-9]{1,2}$" -H "$AUTH"
```

`meta.<clé>=<valeur>` compare à l'identique, `meta.<clé>=~<expression>` compare
par expression régulière. Deux filtres de plus, sur ce que la passerelle
connaît elle-même : `tenant` et `route`. Une expression qui ne compile pas est un refus qui le
dit, jamais une liste vide qui se lirait « il n'y a rien ».

Vingt clés au maximum par tâche : c'est un classement, pas un endroit où ranger
ses données.

> [!NOTE]
> Vos utilisateurs ne parlent jamais à Meerkat. Le service de gestion des
> stations demande ce qui appartient à la station qu'il a devant lui, et rend à
> son client ce qu'il décide de lui montrer.

## Une cadence, un calendrier, ou une date

Une tâche dit **quand** d'une des trois façons, jamais de deux.

**`every`, une cadence** : une durée ISO 8601 - `PT30M`, `PT6H`, `P1D` -
comptée depuis la **fin** du tour précédent. La plus courte est la minute.
C'est ce qu'on veut pour « toutes les six heures », et c'est ce qui empêche un
travail lent de repartir à l'instant où il revient.

**`cron`, un calendrier** : les cinq champs standard, avec les listes, les
plages, les pas, les noms `MON`-`SUN` et `JAN`-`DEC`, et les raccourcis
`@hourly`, `@daily`, `@weekly`, `@monthly`, `@yearly`.

```json
{
  "name": "rapport du lundi",
  "routeId": "orders-api",
  "method": "POST",
  "path": "/jobs/report",
  "cron": "0 3 * * MON",
  "timezone": "Europe/Paris"
}
```

**`at`, une date unique** : un instant RFC 3339, avec son décalage -
`2026-10-03T04:00:00Z`, `2026-10-03T06:00:00+02:00`. L'appel part à ce
moment-là, **une fois**, et la tâche est **terminée** : rien n'est armé après.

```json
{
  "name": "panier 4471, relance",
  "routeId": "orders-api",
  "path": "/jobs/remind",
  "at": "2026-10-03T04:00:00Z",
  "metadata": { "cart": "4471" }
}
```

C'est l'**action différée** : le déclencheur est quelque chose qui est arrivé
- un panier abandonné, une réservation prise, un document déposé - et non une
position dans un calendrier. Votre service traite ce qui est arrivé, pose la
date à laquelle il veut qu'on le rappelle, et n'y pense plus.

Ce que « une fois » entraîne :

- **La date porte son décalage**, donc `timezone` n'a rien à dire à côté
  d'elle et est refusé. `startAt` l'est aussi : il place le premier tour de
  quelque chose qui se répète, et une date unique est son propre premier tour.
- **Une date déjà passée n'est pas une erreur** : l'appel part tout de suite
  si `catchUp` le permet encore, et est abandonné sinon - exactement comme un
  tour manqué pendant que la passerelle était arrêtée.
- **La ligne reste** une fois l'appel parti, avec son résultat, pour qu'on
  puisse voir qu'il est bien parti. Elle est balayée après une rétention que
  root choisit sur l'écran Scheduler, un mois par défaut. Une tâche qui se
  répète n'est jamais balayée.
- **Déplacer sa date** avec un `PUT` déplace le tour : c'est ainsi qu'on
  repousse une action différée, et qu'on redonne une date à une terminée.

Prenez le calendrier pour « tous les lundis à 3 h » ou « le 1er du mois » :
une cadence ne sait pas le dire, et elle **dérive** un peu à chaque tour.

Prenez la cadence pour une période. Un cron n'en exprime pas : ses champs sont
des positions dans un calendrier, pas des pas dans une suite.

```
0 */7 * * *    0 h, 7 h, 14 h, 21 h, puis minuit   -> un trou de 3 heures
*/45 * * * *   0, 45, puis 60, puis 105            -> un trou de 15 minutes
*/20 * * * *   0, 20, 40, 60                       -> celui-la tombe juste
```

`*/n` n'est une période que si `n` divise 60 ou 24. « Toutes les 90 minutes »
s'écrit `PT90M`, et pas autrement. Une cadence n'a par ailleurs ni fuseau à
choisir ni changement d'heure à traverser, et cinquante tâches en `PT1H` se
répartissent d'elles-mêmes là où cinquante `0 * * * *` partent à la même
seconde.

Comme dans tous les crontab, quand les **deux** champs de jour sont restreints,
un jour qui satisfait l'un **ou** l'autre est un jour retenu : `0 0 1 * MON`
est le 1er du mois et tous les lundis.

### Un calendrier se lit quelque part

`timezone` est un nom IANA - `Europe/Paris` - et **UTC** si vous ne dites rien :
« 3 h du matin » veut dire 3 h UTC tant que vous n'avez pas écrit où. Un fuseau
inconnu est refusé à l'écriture plutôt que de retomber en silence sur UTC.

Les deux nuits qui comptent sont gérées : l'heure qui **n'existe pas** au
printemps décale le tour, celle qui **existe deux fois** à l'automne ne
déclenche qu'une fois. Une cadence, elle, ignore ce champ.

## Trois réglages qui évitent trois surprises

| Réglage | Ce qu'il empêche |
|---|---|
| `overlap` (`skip` par défaut) | Un travail lent qui n'est pas revenu : le tour suivant ne part pas. Rien de plus n'est dû tant qu'une exécution est en cours, et sa clôture **arme le tour suivant**, compté à partir de là - un travail lent ne trouve donc jamais un tour qui l'attend dans le passé à l'instant où il revient |
| `catchUp`, en secondes | Une passerelle arrêtée toute la nuit qui tirerait douze tours d'un coup au réveil. Zéro veut dire « lance-le quand tu peux ». Il borne aussi le retard avec lequel un appel coupé par une passerelle qui s'arrête est renvoyé, et décide si une action différée que personne n'a vue passer part quand même |
| `timeout` | Un appel qui ne répond jamais. Il borne la **réponse**, pas le travail : c'est à ça que sert le 202. Deux minutes au plus (`PT2M`) |

## Ce que chaque tour laisse derrière lui

La tâche elle-même garde le **dernier** résultat, celui qu'une liste montre.
Chaque tour terminé est en plus gardé à part, et c'est ce qui répond à
« depuis quand ça échoue », « est-ce que la clôture de cette nuit est passée »
et « qu'est-ce que ça a répondu » :

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

Quatre façons pour un tour de se terminer, et la différence compte :

| | |
|---|---|
| `done` | le service a répondu 2xx |
| `failed` | il a répondu autre chose, ou personne n'a répondu dans le `timeout` |
| `lost` | il a pris le travail avec un `202` puis s'est tu au-delà du bail |
| `dropped` | le tour **n'est jamais parti** : il est arrivé plus tard que ce que `catchUp` autorise |

`attempt` dit quelle tentative a terminé le tour : `2` veut dire qu'une
passerelle s'est arrêtée avant la réponse et qu'une autre a renvoyé le même
appel. `cause` et `ofRun` sont la chaîne - un tour ordinaire ne dit rien, un
rejeu nomme l'exécution qu'il continue.

### Rejouer un tour

Un tour abandonné, ou qui a échoué pour une raison corrigée depuis, se
redemande par son nom :

```bash
curl -X POST "http://meerkat:9090/api/schedules/sch_9f2c/run" -H "$AUTH" \
  -H "Content-Type: application/json" \
  -d '{"replayOf":"run-17906f3c9d2a4e10-9f2c11"}'
```

Il repart comme une **nouvelle exécution**, avec un nouvel identifiant et le
**payload d'aujourd'hui** - la tâche telle qu'elle est maintenant, pas telle
qu'elle était cette nuit-là. Ce qu'il rejoue est écrit dans l'historique, donc
les deux se lisent comme une seule chaîne. Sans corps, le même appel est le
simple « lancer maintenant ».

L'historique est balayé sur la même rétention qu'une action différée terminée
- un mois par défaut, que root peut changer - et part avec la tâche quand on
la supprime.

## Quand elle regarde, et en cluster

**Il n'y a pas de tic.** La passerelle dort jusqu'à la prochaine chose due et
se réveille à la seconde. Écrire une tâche, la suspendre, avancer son tour ou
clore une exécution la réveille tout de suite ; un filet de sécurité couvre le
reste une fois par minute.

Plusieurs passerelles partagent la même table et **aucune ne prend de verrou** :
la prise de tour est un `UPDATE` conditionnel, donc deux passerelles au même
instant donnent un gagnant. Les appels se **répartissent entre elles**,
plusieurs à la fois sur chacune - un service lent retient son propre appel et
celui de personne d'autre.

Ce que le cluster ajoute vraiment, c'est le **bail** : une exécution dont
personne ne dit rien pendant cinq minutes est réglée par la première passerelle
qui la voit, renvoyée ou close comme perdue.

Tant qu'une exécution est en cours, rien de plus n'est dû : « lancer
maintenant » répond `409`, et la clôture arme le tour suivant.


## Ce que ce n'est pas

**Une file de messages.** Pas de diffusion à plusieurs consommateurs, pas
d'accusé de réception, pas de file de rebut : une tâche, un appel, au moins une
fois. Si vous avez besoin de l'autre chose, il vous faut un courtier.

> [!TIP]
> Vous avez déjà des CronJobs Kubernetes et vous voulez qu'ils restent l'horloge ?
> Faites-leur appeler `POST /api/schedules/{id}/run` avec un jeton d'API - il
> répond `409` tant que l'exécution précédente est en cours. La passerelle
> garde l'identité, le confinement et la trace ; votre cluster garde le
> calendrier. Meerkat n'a besoin d'aucun droit sur votre cluster pour ça.

## Ce que voit l'exploitant

**Scheduler**, dans le rail, liste tout ce qui est planifié, filtrable par
compte, organisation et service, mis à jour en direct : une exécution qui
démarre, avance et se termine apparaît sans rien rafraîchir. Trois actions y
vivent, et ce sont celles qu'on veut à deux heures du matin : suspendre,
avancer le prochain tour, supprimer.

![L'écran Scheduler : une cadence, une ligne cron et une date unique, chacune avec son prochain tour et le dernier](img/console/scheduler.webp)

Une exécution en cours dit où elle en est - **calling**, en attente de la
réponse, ou **accepted**, le service a pris le travail - et à quelle tentative
elle est quand une passerelle s'est arrêtée avant la réponse. Son prochain tour
se lit "after this run" : c'est la clôture qui l'arme, et "lancer maintenant"
l'attend.

Le tiroir d'une tâche garde **tous les tours qu'elle a faits** : quand il
s'est terminé, comment, ce qui a répondu, quel noeud a fait l'appel, quelle
tentative c'était - et un bouton pour rejouer un tour abandonné ou en échec.

Une **action différée** déjà partie se lit "finished" et garde son résultat à
l'écran jusqu'au balayage. Un interrupteur les masque quand un service en pose
assez pour noyer le reste, et root choisit à côté combien de temps on les
garde.

Les heures s'écrivent dans le **fuseau de l'exploitant** - celui de son profil,
posé une fois et lu par les deux plans - ou en **UTC**, sur un bouton qui dit
lequel est affiché : « 2 h 24 » ne veut rien dire tant qu'on ne sait pas de qui
c'est 2 h 24.

Un bouton **API** ouvre, dans le même tiroir, le mécanisme et l'API : les
commandes et le payload déjà remplis avec l'adresse de **cette** installation,
puisque l'écran ne crée rien et que la marche suivante est un appel que le
service fait lui-même.

Le tiroir d'une tâche garde, replié, le **payload qui la recrée** : les champs
qu'une ligne ne peut pas montrer - le corps envoyé, les trois réglages - et un
exemple tout fait pour qui doit en écrire une autre.

Le filtre **Metadata** y accepte la même chose que l'API - `station=42`, ou
`station=~^st-` - parce qu'un exploitant qui cherche une tâche dans une flotte
la cherche par ce que le service, lui, a écrit dessus.

Créer une tâche ne s'y fait pas, et c'est délibéré : une tâche s'exécute avec
l'identité d'un compte, et les administrateurs qui lisent cet écran ne sont pas
les comptes sous lesquels elle devrait tourner. L'API le refuse aussi, avec la
phrase qui le dit : la création appartient au service, avec son jeton.
