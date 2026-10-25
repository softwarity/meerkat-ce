---
title: Notifier vos utilisateurs
section: Exploitation
order: 214
summary: Un service placé derrière la gateway prévient ses utilisateurs, qui l'apprennent dans l'application qu'ils ont ouverte - avec un compteur, un snackbar, une notification du bureau, et un acquittement quand vous le demandez.
---

# Notifier vos utilisateurs

Un service placé derrière la gateway a quelque chose à dire à ses utilisateurs :
une facture est en retard, une chambre froide chauffe, un export est prêt. Il
envoie **une requête** à la gateway, et les personnes concernées l'apprennent
dans l'application qu'elles ont ouverte. Aucun service de push à faire tourner,
aucune liste d'appareils à tenir, aucune page à écrire. Édition Enterprise.

Cette page s'adresse au développeur de ce service, et dit ce que voient ses
utilisateurs. Les écrans de l'administrateur sont décrits dans
[Notifications dans la console](/docs/console/notifications).

## Ce que voient vos utilisateurs

- **Un compteur** sur le bouton de compte, ou sur l'entrée **Notifications** de
  la barre de portail quand le catalogue en a une. Un seul compteur pour tous
  leurs tenants.
- **Un snackbar** sur la page qu'ils ont devant eux à l'arrivée d'une
  notification : l'icône du niveau, le titre, **Ouvrir**, et **Acquitter** quand
  elle le demande. Il disparaît au bout de 6 secondes, attend tant que le
  pointeur ou le focus est dessus, et trois s'empilent au plus.
- **Une notification du bureau** quand l'onglet n'est pas devant eux, une fois
  qu'ils l'ont activée avec **Afficher sur mon bureau** sur leur page de
  notifications, et seulement sur une adresse sûre (https, ou localhost). Ses
  boutons sont **Ouvrir**, puis vos actions, exécutées en arrière-plan avec leur
  résultat affiché sur la notification, et **Acquitter** quand c'est demandé.
  Un clic sur elle ouvre la page des notifications.
- **La page des notifications**, `/meerkat/notifications`, ou la page de
  l'application quand l'administrateur en a choisi une. Les plus récentes
  d'abord. Un clic marque une notification comme lue et la déplie quand il y a
  plus à voir (un chevron l'annonce) : le texte entier, **Ouvrir** pour le
  détail, et vos actions, chacune avec son résultat.
- **Un mail**, quand vous le demandez et qu'un relais de messagerie est
  configuré : le texte, et un bouton vers la page des notifications. Jamais
  d'action.

![La page des notifications d'une lectrice de deux tenants : le filtre par tenant, un message envoyé aux deux sur une seule rangée, une notification qui attend son acquittement, et une autre dépliée avec Ouvrir et son action](img/console/notifications-page.webp)

Sur la page, **Supprimer** retire une notification de la liste du lecteur, pour
de bon ; l'émetteur et les autres lecteurs gardent la leur. **Tout marquer comme
lu** et **Tout supprimer** agissent sur ce que montre la liste. Tout suit en
direct, dans chaque onglet et sur chaque appareil du lecteur.

**Plusieurs tenants.** Un lecteur reçoit les notifications de tous ses tenants.
Chaque rangée nomme alors son tenant, et un filtre borne la liste à l'un d'eux.
Une notification d'un autre tenant que celui où il travaille se lit et se
supprime ; son détail et ses actions attendent qu'il passe sur ce tenant, et le
bouton le dit. Un message envoyé à tous les tenants n'est **qu'une** rangée qui
les nomme, comptée une fois, lue et supprimée en une fois.

**Retirer et Supprimer ne sont pas la même chose.** Retirer appartient à
l'émetteur, ou à un administrateur : la notification quitte aussitôt la page de
chaque lecteur. Supprimer appartient au lecteur : elle quitte sa propre liste,
et celle de personne d'autre.

## Qui la reçoit

L'audience est arrêtée **une fois, à la publication**, et conservée : une ligne
par lecteur. Elle part des membres actifs du tenant, réduits aux `roles` et aux
`users` que vous nommez (l'un ou l'autre suffit ; aucun des deux veut dire tous
les membres), et ne garde que les personnes qui peuvent **ouvrir ce vers quoi
elle mène** :

| La notification a | Elle va à |
|---|---|
| un détail sur une route | ceux que la route laisse entrer |
| des actions sur des routes, et pas de détail | ceux qui peuvent utiliser au moins une action |
| ni l'un ni l'autre, ou seulement des liens hors de la gateway | tous ceux qui sont nommés |

Les droits sont revérifiés à chaque affichage : qui a quitté le tenant, ou perdu
le rôle, ne la voit plus, et une action qu'un lecteur ne peut pas utiliser ne
lui est pas proposée. Une publication qui toucherait plus de personnes que le
plafond fixé par un administrateur (5000 sauf réglage) est refusée.

## Publier

Sur le **plan interne**, port 7070, comme les
[appels planifiés](/docs/operations/scheduler) : appelé depuis l'intérieur du
cluster par le nom interne de la gateway, jamais par un navigateur.

| | |
|---|---|
| `POST /api/notifications` | publier |
| `GET /api/notifications` | les vôtres encore actives, les plus récentes d'abord, filtrées par `meta.<clé>` |
| `GET /api/notifications/{id}/acks` | qui a acquitté l'une des vôtres, et qui pas encore |
| `DELETE /api/notifications/{id}` | retirer l'une des vôtres |
| `DELETE /api/notifications?meta.<clé>=<valeur>` | retirer toutes les vôtres qui correspondent |

**Qui peut l'appeler** se règle dans la console, **Data plane, Notifications,
Access** : les trois modes du scheduler (aucun justificatif, un jeton Meerkat,
le jeton de ServiceAccount du pod sur Kubernetes), puis tout le monde ou
seulement les appelants listés. Une nouvelle installation demande un **jeton
Meerkat** : un jeton interne de périmètre `notifications`, émis sous **Infra,
Access tokens, Internal**. Il publie des notifications et n'ouvre rien d'autre.
Le bouton **API**, à côté d'Access, résume cette page pour le développeur d'un
service.

Le nom que l'accès donne à votre service est l'**émetteur**. "Les vôtres", sur
cette page, veut dire envoyées sous ce nom.

### Le message

64 Kio au plus ; un champ inconnu est refusé.

| Champ | |
|---|---|
| `tenant` | le tenant, par id ou par nom ; facultatif quand il n'y en a qu'un. `"*"` l'envoie à [tous les tenants](#tous-les-tenants-dun-coup) |
| `roles` | seulement les membres qui portent l'un de ces rôles |
| `users` | seulement ces membres, par nom d'utilisateur. Avec `roles`, l'un ou l'autre suffit |
| `title` | obligatoire. Texte brut, 200 caractères au plus |
| `body` | texte brut, 4000 caractères au plus |
| `level` | `info` (par défaut), `success`, `warning` ou `error` |
| `detail` | où elle mène : un [lien](#les-liens-un-service-et-un-chemin), sur une route UI |
| `actions` | des boutons, 5 au plus : un `label` (60 caractères), un lien, et [la façon de l'appeler](#les-actions-method-et-body) |
| `mail` | `true` pour l'envoyer aussi par mail. Refusé quand aucun relais de messagerie n'est configuré |
| `expires` | une durée (`P7D`, `PT12H`) ou une date (`2026-11-01T08:00:00Z`) : 30 jours par défaut, 90 au plus. Ensuite elle quitte toutes les listes |
| `key` | votre clé de déduplication, 200 caractères. Renvoyée au même tenant tant que la première est active, elle répond celle-ci et ne prévient personne de nouveau |
| `meta` | vos étiquettes, comme `{"invoice": "INV-2026-0142"}`, pour retrouver et retirer les vôtres. Jamais montrées aux lecteurs |
| `ack` | `none` (par défaut), `single` ou `all` : voir [L'acquittement](#lacquittement) |
| `onAck` | votre service, appelé à chaque acquittement. Exige `ack` |

Les textes sont dans **une seule langue** pour tous les lecteurs : écrivez-les
dans la langue que partagent vos utilisateurs. Un objet par langue est refusé.

### Les liens : un service et un chemin

Vous ne connaissez pas les routes de la gateway, et vous n'avez pas à les
connaître. Nommez un lien comme vous l'appelez dans le cluster : le
**service**, et le **chemin tel que le service le voit**. La gateway trouve la
route placée devant ce service, défait ses réécritures, et donne au lecteur
l'adresse publique.

```json
{ "service": "billing", "path": "/invoices/INV-2026-0142" }
```

`billing` est l'hôte de l'upstream de la route, ou son premier segment :
`billing.svc.acme.internal` se trouve par `billing`. Derrière une route qui
répond à `/billing/**` et retire son premier segment, le lecteur reçoit
`/billing/invoices/INV-2026-0142`.

Un lien est refusé, avec la raison, quand aucune route active ne sert ce
service, quand plusieurs le servent (la gateway n'en choisit jamais une au
hasard), ou quand la route réécrit ses chemins d'une façon qui ne se défait pas
(`rewrite-path`, `set-path`).

Hors de la gateway, donnez `{"url": "https://..."}` : https seulement.

### Les actions : method et body

Une action est une requête que fait **le navigateur du lecteur**, à travers la
gateway, avec la session du lecteur. Votre service voit le lecteur, avec
l'identité que transmet la route, et décide. L'adresse d'une action sur une
route porte `notification=<id>`, pour que vous sachiez de quelle notification
vient le clic.

| Champ | |
|---|---|
| `method` | `GET` (par défaut), `POST`, `PUT`, `PATCH` ou `DELETE` |
| `body` | n'importe quelle valeur JSON, envoyée telle quelle avec `Content-Type: application/json`. Pas avec `GET` |

```json
{
  "label": "Send a reminder",
  "service": "billing",
  "path": "/invoices/INV-2026-0142/remind",
  "method": "POST",
  "body": { "channel": "email" }
}
```

> [!WARNING]
> Le navigateur du lecteur reçoit le corps pour l'envoyer : **n'y mettez jamais
> de secret**. Ce qui prouve l'appel, c'est la session du lecteur, pas le corps.

Le corps compte dans les 64 Kio du message. Une action sur une `{"url"}` hors de
la gateway part sans la session ; avec une autre méthode que `GET` et un corps
JSON, le navigateur demande d'abord la permission à ce serveur (un preflight
CORS), qu'il doit accorder.

### Tous les tenants d'un coup

`"tenant": "*"` envoie un exemplaire à chaque tenant actif dont les
notifications sont activées, chacun avec son id et ses lecteurs. Un tenant où
personne ne peut la recevoir est écarté, et la réponse le dit. Les utilisateurs
nommés dans `users` sont pris dans les tenants dont ils sont membres. Un lecteur
membre de plusieurs de ces tenants n'en voit **qu'une**, qui les nomme, avec un
seul snackbar, une seule notification du bureau et un seul mail.

La clé vaut par tenant : renvoyé avec la même clé, le message répond les
exemplaires encore actifs et publie ceux qui manquent. Une publication coupée en
route se termine en la renvoyant.

### Un exemple complet

```bash
curl -X POST http://meerkat:7070/api/notifications \
  -H "Authorization: Bearer $MEERKAT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "tenant": "Acme Logistics",
    "roles": ["BILLING"],
    "level": "warning",
    "title": "Invoice INV-2026-0142 is overdue",
    "body": "Northwind Traders has not paid the 4820.00 EUR due on 2026-10-20.",
    "detail": { "service": "billing", "path": "/invoices/INV-2026-0142" },
    "actions": [
      { "label": "Send a reminder", "service": "billing",
        "path": "/invoices/INV-2026-0142/remind",
        "method": "POST", "body": { "channel": "email" } }
    ],
    "key": "overdue-INV-2026-0142",
    "meta": { "invoice": "INV-2026-0142" },
    "ack": "single",
    "onAck": { "service": "billing", "path": "/notifications/acked" }
  }'
```

```http
HTTP/1.1 201 Created
Content-Type: application/json

{"id": "ntf_5c0e9a1f2b7d4e6a8c3b1d0f", "recipients": 4}
```

### Les réponses

| Statut | Corps | Sens |
|---|---|---|
| `201` | `{"id", "recipients"}` | publiée |
| `200` | le même, avec `"existing": true` | la clé est déjà active : personne n'est prévenu de nouveau |
| `400` | `{"error"}` | pas du JSON, un champ inconnu, ou plus de 64 Kio |
| `422` | `{"error"}` | refusée : la phrase nomme le champ et ce qui est permis |
| `401`, `403` | `{"error"}` | l'accès a refusé l'appelant : ce qui manque, et qui est admis |

Quelques refus, tels qu'ils sont écrits :

```text
level "urgent": allowed are info, success, warning, error
tenant "Acme": no such tenant; there are Acme Logistics, Acme Retail
notifications are off for the tenant "Acme Retail", turned off by the tenant admin: only that level turns them back on
users: zoe not members of this tenant
action 1 method "FETCH": allowed are GET, POST, PUT, PATCH, DELETE
action 1: a GET sends no body; choose POST, PUT, PATCH or DELETE to send this one, or remove the body
nobody in this audience may open what the notification leads to: check the roles and accounts named, and the access of the routes behind its links
```

Envoyé à tous les tenants, le `201` liste les exemplaires et les tenants
écartés :

```json
{
  "recipients": 37,
  "tenants": [
    { "tenant": "7c1e2a90", "name": "Acme Logistics", "id": "ntf_5c0e9a1f2b7d4e6a8c3b1d0f", "recipients": 21 },
    { "tenant": "b2f4c8d1", "name": "Acme Retail", "id": "ntf_e81d07c3a9b24f5e6d1c8a72", "recipients": 16 }
  ],
  "skipped": [
    { "tenant": "40aa9e3b", "name": "Acme Labs", "reason": "notifications are off, turned off by the tenant admin" }
  ]
}
```

Quand aucun tenant ne l'a reçu, la réponse est un `422` qui nomme chaque tenant
et pourquoi.

## L'acquittement

`ack` demande aux lecteurs plus que de lire :

- `single` : le premier lecteur qui acquitte la clôt pour tous, comme pour une
  alerte d'astreinte. Les autres voient qui l'a fait.
- `all` : chaque lecteur acquitte la sienne, et elle est complète quand le
  dernier l'a fait, comme pour une consigne de sécurité.

Tant qu'un lecteur la doit, la rangée est encadrée, porte **Acquitter**, et ne se
supprime pas ; **Tout supprimer** la garde et dit combien il en a gardé, et
**Tout marquer comme lu** n'acquitte jamais. Le snackbar et la notification du
bureau portent **Acquitter** aussi. Une fois fait, la rangée dit quand, et en
`single` les autres lecteurs voient qui l'a fait, en direct. Un message envoyé à
tous les tenants s'acquitte en entier, depuis une session de n'importe lequel
des tenants du lecteur ; en `single`, un acquittement clôt tous ses exemplaires.
Une notification retirée ne demande plus rien.

Chaque acquittement est une ligne de la [piste d'audit](/docs/operations/audit),
`notification.ack`, avec le lecteur pour acteur. Elle reste après l'expiration
de la notification.

**`onAck`** nomme votre service, `{service, path}` seulement. À chaque
acquittement, la gateway lui envoie un POST par sa route, sous l'utilisateur
`meerkat` dans le tenant de la notification, sans rôle, comme un appel planifié :
si la route ou une règle d'endpoint exige un rôle, laissez passer ce chemin.
Tout ce qui n'est pas un 2xx est retenté deux fois.

```http
POST /notifications/acked
Content-Type: application/json
Meerkat-Notification: ntf_5c0e9a1f2b7d4e6a8c3b1d0f
Meerkat-Attempt: 1

{"notification": "ntf_5c0e9a1f2b7d4e6a8c3b1d0f", "tenant": "7c1e2a90",
 "tenantName": "Acme Logistics", "key": "overdue-INV-2026-0142",
 "meta": {"invoice": "INV-2026-0142"}, "ack": "single",
 "by": {"id": "u_41f2", "username": "alice", "name": "Alice Martin"},
 "at": "2026-10-25T09:12:44Z", "acknowledged": 1, "recipients": 4, "complete": true}
```

`complete` vaut `true` sur l'acquittement qui l'a close : le premier en
`single`, le dernier en `all`.

**Où elle en est.** Votre liste porte `ackState` (`pending` ou `done`),
`acknowledged` (combien de lecteurs l'ont fait) et, en `single`, `ackBy`.
`GET /api/notifications/{id}/acks` le donne personne par personne : `acks` (qui
et quand, dans l'ordre) et `missing` (qui pas encore). Une notification qui ne
demande pas d'acquittement répond `409`.

## Retrouver et retirer les vôtres

`meta` retrouve vos notifications sans garder leurs ids :

```bash
curl "http://meerkat:7070/api/notifications?meta.invoice=INV-2026-0142" -H "$AUTH"
curl -X DELETE "http://meerkat:7070/api/notifications?meta.invoice=INV-2026-0142" -H "$AUTH"
```

`meta.<clé>=<valeur>` compare une valeur, `meta.<clé>=~<regex>` une expression
régulière (RE2), et toutes les conditions doivent correspondre. Retirer par
filtre en exige au moins une : tout ce qu'un émetteur a jamais envoyé n'est pas
une requête. La réponse liste ce qui a été retiré, `{"withdrawn": ["ntf_..."]}`,
et ces notifications quittent aussitôt la page de chaque lecteur. La console les
garde, marquées Withdrawn, avec qui les a lues.

## Votre propre page de notifications

La page intégrée est un choix. Une application peut montrer les notifications
dans ses propres écrans : l'administrateur indique alors cette page à la gateway
(**Application, Portal, Notifications page** : une route UI et un chemin
dessous), et la barre, le menu du compte, le snackbar et les mails y mènent. Qui
ne peut pas ouvrir cette route reçoit quand même la page intégrée, qui reste
ouverte à tous.

Votre page lit les notifications du lecteur sur le plan de données, avec la
session :

| | |
|---|---|
| `GET /meerkat/notifications.json` | une page des notifications du lecteur, de tous ses tenants |
| `GET /meerkat/notifications/counts` | le nombre de non lues |
| `POST /meerkat/notifications/{id}/read` | en marquer une comme lue ; `POST /meerkat/notifications/read` les marque toutes |
| `POST /meerkat/notifications/{id}/ack` | en acquitter une |
| `DELETE /meerkat/notifications/{id}` | en supprimer une pour ce lecteur ; `DELETE /meerkat/notifications` les supprime toutes, sauf ce qui attend un acquittement |

Chaque action de la liste arrive avec sa méthode et son corps, prête à envoyer.
Le contrat complet est dans la référence d'API du plan de données (**Meerkat,
API reference** dans la console).
