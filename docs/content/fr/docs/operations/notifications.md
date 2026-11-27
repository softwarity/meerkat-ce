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

- **Un compteur** sur la cloche de la barre de portail, ou, sans la barre, sur
  le bouton de compte : les notifications du tenant de leur session.
- **Un tiroir**, par-dessus la page, quand ils appuient sur la cloche ou sur
  l'entrée **Notifications** du bouton de compte : contre la barre, ou le long
  du bord de la page opposé au bouton de compte, qui reste visible. Chaque
  notification montre son niveau, son titre (en gras tant qu'elle n'est pas
  lue), son texte coupé à deux lignes (un chevron le déplie), qui l'a envoyée
  et quand. Un clic sur le texte la marque comme lue, la croix la **ferme**.
  Dessous, à droite, chacun à sa place : votre action, **Ouvrir** et
  **Acquitter**. Son en-tête mène à la page des notifications, qui ramène
  ensuite là où ils étaient.
- **Un snackbar** sur la page qu'ils ont devant eux à l'arrivée d'une
  notification : l'icône du niveau, le titre, **Ouvrir**, et **Acquitter** quand
  elle le demande. Il disparaît au bout de 6 secondes, attend tant que le
  pointeur ou le focus est dessus, et trois s'empilent au plus.
- **Une notification du bureau** quand l'onglet n'est pas devant eux, une fois
  qu'ils l'ont activée avec **Afficher sur mon bureau** (la cloche de la barre
  du haut de leur page de notifications), et seulement sur une adresse sûre (https, ou localhost). Ses
  boutons sont **Ouvrir**, puis votre action, exécutée en arrière-plan avec son
  résultat affiché sur la notification, et **Acquitter** quand c'est demandé.
  Un clic sur elle ouvre la page des notifications.
- **La page des notifications**, `/meerkat/notifications`, ou la page de
  l'application quand l'administrateur en a choisi une. Une liste, les plus
  récentes d'abord, chaque jour sur une carte, les non lues sur un ton à part,
  chaque rangée dessinée comme celles du tiroir : l'icône de son niveau, son
  titre (en gras tant qu'elle n'est pas lue), deux lignes de son texte (un
  chevron déplie la suite), qui l'a envoyée et quand ; au bout du titre
  **Supprimer** (la corbeille) et **Fermer** (la croix) ; dessous, toujours
  visibles, votre action avec son résultat, **Ouvrir** et **Acquitter**. Un
  clic sur le texte la marque comme lue. Seule la liste défile : la barre du
  haut, la recherche et les filtres restent en haut de la fenêtre, le pied de
  page en bas. Sans la barre de portail, la page porte le bouton de compte de
  l'application d'où vient le lecteur, à sa place, et sa flèche retour mène à
  la page qu'il a quittée. L'émetteur
  est nommé pour le lecteur : un service par son nom court (`station-registry`
  pour le workload `ns/Deployment/station-registry`), un jeton par son nom, un
  administrateur qui l'a envoyée depuis la console par le nom qu'affiche son
  compte.
- **La recherche et les filtres**, sous la barre du haut de la page, qui reste
  visible avec eux pendant que la liste défile : une barre de recherche, puis
  **Non lues seulement** et les tenants quand il y en a plusieurs. **Tout
  marquer comme lu**, **Afficher sur mon bureau** et **Tout supprimer** sont
  des icônes de la barre du haut, nommées quand le pointeur s'y arrête ou que
  le clavier les atteint ; sur un téléphone, Tout supprimer et le bureau
  passent dans le menu de la barre (les trois points). **Fermées** liste ce
  que le lecteur a fermé. Ils se combinent, et l'adresse les garde, si bien qu'un
  rechargement ou un lien gardent la même liste. La recherche cherche chaque
  mot tapé, sans casse ni accents, dans le titre, le texte et les métadonnées
  que vous avez jointes ; elle se fait sur la gateway, donc vos métadonnées
  n'atteignent jamais le navigateur du lecteur, et une notification retirée ne
  correspond à rien. Un mot écrit `clé=valeur` demande cette métadonnée
  exactement : `ticket=ZK-4471` trouve les notifications dont le `ticket` vaut
  `ZK-4471`, toujours sans casse ni accents, et pas `ZK-44710`.
- **Un mail**, quand vous le demandez et qu'un relais de messagerie est
  configuré : le texte, et un bouton vers la page des notifications. Jamais
  d'action.
- **L'étiquette EXERCICE** devant le titre, partout où elle paraît, sur une
  notification envoyée [pour s'exercer](#les-exercices).

![Le tiroir ouvert depuis la cloche de la barre de portail : chaque notification avec son niveau, son titre, deux lignes de texte, qui l'a envoyée et quand, la croix qui la ferme, et dessous l'action, Ouvrir et Acquitter](img/console/notifications-drawer.webp)

![La page des notifications : la barre du haut et ses actions, la barre de recherche, Non lues seulement et Fermées ; chaque rangée avec son niveau, son titre, son texte, l'émetteur et l'heure, la corbeille et la croix au bout du titre, et dessous l'action, Ouvrir et Acquitter](img/console/notifications-page.webp)

**Lue, fermée, supprimée.** Un lecteur a trois façons d'en finir avec une
notification, pour lui seul - l'émetteur et les autres lecteurs gardent la
leur :

| | Comment | Où elle va |
|---|---|---|
| **Lue** | un clic sur son texte, Ouvrir, ou Tout marquer comme lu | hors du compte, toujours listée |
| **Fermée** | la croix, dans le tiroir ou sur la page | hors du tiroir et de la liste, lue ; listée sous **Fermées** sur la page, où **Rouvrir** la ramène, jusqu'à son expiration |
| **Supprimée** | la corbeille de la page, ouverte ou fermée | partie pour de bon |

**Tout marquer comme lu** et **Tout supprimer** agissent sur ce que montre la
liste : réduite aux non lues, aux fermées ou par une recherche, ils prennent
cela et rien d'autre, et Tout supprimer demande d'abord, en le disant. Tout
suit en direct, dans chaque onglet et sur chaque appareil du lecteur. Une
notification qui attend l'acquittement du lecteur ne se ferme ni ne se
supprime avant qu'il le donne.

Une notification que son émetteur a **retirée** ou **remplacée** quitte la
liste et le compte de chaque lecteur, sans rien laisser à sa place : une
remplaçante tient la place de celle qu'elle remplace. Votre service peut aussi
dire, pour tous les lecteurs ou pour un seul, qu'une notification est lue,
fermée ou supprimée : voir [Quand le travail est fait](#quand-le-travail-est-fait).

**Plusieurs tenants.** Un lecteur voit les notifications du tenant de sa
session ; celles de ses autres tenants l'attendent là-bas, et paraissent quand
il y bascule. Un message envoyé à tous les tenants n'est **qu'une**
notification pour lui, lue, fermée, supprimée et acquittée en une fois pour
tous ses exemplaires.

## Qui la reçoit

À la publication, la gateway calcule les lecteurs et garde **une ligne par
lecteur**. Elle garde aussi l'audience que vous avez nommée, si bien que
quelqu'un qui y entre plus tard la reçoit encore (voir
[Les droits acquis ensuite](#les-droits-acquis-ensuite)). L'audience part des
membres actifs du tenant, réduits aux `roles` et aux
`users` que vous nommez (l'un ou l'autre suffit ; aucun des deux veut dire tous
les membres), et ne garde que les personnes qui peuvent **ouvrir ce vers quoi
elle mène** :

| La notification a | Elle va à |
|---|---|
| un détail sur une route | ceux que la route laisse entrer |
| une action sur une route, et pas de détail | ceux qui peuvent utiliser l'action |
| ni l'un ni l'autre, ou seulement des liens hors de la gateway | tous ceux qui sont nommés |

Les droits sont revérifiés à chaque affichage : qui a quitté le tenant, ou perdu
le rôle, ne la voit plus, et une action qu'un lecteur ne peut pas utiliser ne
lui est pas proposée. Une publication qui toucherait plus de personnes que le
plafond fixé par un administrateur (5000 sauf réglage) est refusée.

### Les droits acquis ensuite

Deux moments donnent une notification active à quelqu'un qui ne l'avait pas :
il **rejoint ou quitte le tenant** (le quitter ne donne rien), ou un **rôle lui
est attribué** dans ce tenant. Un rôle se tient par un groupe de rôles : il est
attribué quand on le met dans un groupe de rôles qui le porte, ou quand un de
ses groupes de rôles le reçoit. À ce moment, la gateway regarde les notifications
actives du tenant (ni expirées, ni retirées, ni remplacées) dont l'audience
gardée le comprend désormais, avec les mêmes règles que ci-dessus, et lui donne
celles qu'il n'a pas.

- Il est prévenu **une fois**, quel que soit le nombre : un snackbar, ou une
  notification du bureau quand l'onglet n'est pas devant lui, qui dit
  *Notifications manquées : 3* et mène à sa liste. Jamais une par notification.
- Quand au moins l'une d'elles demande un mail, il reçoit **un seul mail
  récapitulatif** qui les liste, par la même [file des mails](#le-mail), jamais
  un mail chacune.
- Ouvrir sa liste refait le même travail, sans bruit : ce qu'un moment a manqué
  est donné alors.
- Le travail se fait en arrière-plan : l'enregistrement d'une appartenance, ou
  une connexion qui attribue un tenant ou un rôle, n'est jamais retardé par
  lui.

Un droit perdu ne change rien aux lignes : la page revérifie l'accès à chaque
affichage et cache ce que le lecteur ne peut plus ouvrir. Une notification
publiée avant cette version n'a pas gardé d'audience, et n'est donnée à
personne ensuite.

## Publier

Sur le **plan interne**, port 7070, comme les
[appels planifiés](/docs/operations/scheduler) : appelé depuis l'intérieur du
cluster par le nom interne de la gateway, jamais par un navigateur.

| | |
|---|---|
| `POST /api/notifications` | publier |
| `GET /api/notifications` | les vôtres encore actives, les plus récentes d'abord, filtrées par `meta.<clé>` |
| `GET /api/notifications/{id}/acks` | qui a acquitté l'une des vôtres, et qui pas encore |
| `POST /api/notifications/{id}/read` | marquer l'une des vôtres comme lue, pour tous les lecteurs ou pour `?user=` |
| `POST /api/notifications/{id}/close` | fermer l'une des vôtres, pour tous les lecteurs ou pour `?user=` |
| `DELETE /api/notifications/{id}` | retirer l'une des vôtres par son id : tous les exemplaires d'une envoyée à tous les tenants ; avec `?user=`, la supprimer pour ce lecteur seul |
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
| `exercise` | `true` pour un message envoyé pour s'exercer. Voir [Les exercices](#les-exercices) |
| `detail` | où elle mène : un [lien](#les-liens-un-service-et-un-chemin), sur une route UI |
| `actions` | un bouton au plus : un `label` (60 caractères), un lien, et [la façon de l'appeler](#les-actions-method-et-body) |
| `mail` | `true` pour l'envoyer aussi par mail. Refusé quand aucun relais de messagerie n'est configuré. Voir [Le mail](#le-mail) |
| `expires` | une durée (`P7D`, `PT12H`) ou une date (`2026-11-01T08:00:00Z`) : 30 jours par défaut, 90 au plus. Ensuite elle quitte toutes les listes |
| `id` | obligatoire, un UUID de votre choix : il permet de renvoyer sans doublon, de remplacer ou de retirer la notification, et de suivre ses acquittements et ses mails. 200 caractères au plus, lettres, chiffres et `-_.:`. Voir [Son id](#son-id-remplacer-et-retirer) |
| `replaces` | l'id d'une de vos notifications antérieures, qui se termine comme remplacée. Voir [Remplacer](#son-id-remplacer-et-retirer) |
| `meta` | vos étiquettes, comme `{"invoice": "INV-2026-0142"}`, pour retrouver et retirer les vôtres. Jamais montrées aux lecteurs |
| `ack` | `none` (par défaut), `single` ou `all` : voir [L'acquittement](#lacquittement) |
| `onAck` | votre service, appelé à chaque acquittement. Exige `ack` |

Les textes sont dans **une seule langue** pour tous les lecteurs : écrivez-les
dans la langue que partagent vos utilisateurs. Un objet par langue est refusé.

### Les exercices

Un exercice doit ressembler à la réalité, et ne jamais être pris pour elle.
Envoyez-le avec `"exercise": true` et un `level` réaliste : il porte l'étiquette
**EXERCICE**, dans la langue du lecteur, devant son titre sur la page, sur le
snackbar et sur la notification du bureau, et dans l'objet, le titre et la
première ligne de son mail. La console le montre avec la même étiquette. Rien
d'autre ne change : son audience est calculée comme pour toute autre
notification, si bien que choisir qui participe à l'exercice revient à votre
service, par `roles` et `users`, et il se compte et se lit comme les autres.

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
`/billing/invoices/INV-2026-0142`. Derrière une route qui garde son chemin, le
service voit lui-même `/billing/invoices/...` ; un chemin donné depuis
l'intérieur de l'application, `/invoices/INV-2026-0142`, est placé sous l'entrée
de la route de la même façon.

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
notifications sont activées, chacun avec ses lecteurs, tous sous l'id unique de
l'envoi. Un tenant où
personne ne peut la recevoir est écarté, et la réponse le dit. Les utilisateurs
nommés dans `users` sont pris dans les tenants dont ils sont membres. Un lecteur
membre de plusieurs de ces tenants n'en voit **qu'une**, qui les nomme, avec un
seul snackbar, une seule notification du bureau et un seul mail.

Renvoyé avec le même id, le message répond les exemplaires déjà publiés et
publie ceux qui manquent : une publication coupée en route se termine en la
renvoyant. Remplacer ou retirer par cet id agit sur chaque exemplaire.

### Son id, remplacer et retirer

**Un seul id, le vôtre, obligatoire.** Chaque message porte un `id`, un UUID de
votre choix : il permet de renvoyer sans doublon, de remplacer ou de retirer la
notification, et de suivre ses acquittements et ses mails. Un message sans id
est refusé (`422` sur `id`). Un service qui ne reviendra jamais sur une
notification envoie un UUID neuf et ne le garde pas. La gateway n'invente
jamais d'id. Sa source est gardée sur chaque notification, et la console la
montre. Un id vaut pour
sa source, l'appelant que reconnaît le plan interne : le nom d'un jeton Meerkat, le
ServiceAccount du pod, ou le workload derrière l'adresse (une adresse seule
seulement quand rien de mieux n'est connu). Un autre service peut utiliser le
même id. Republiée depuis la même source sous un id déjà utilisé, avant
l'expiration de cette notification, la réponse est la notification déjà là,
avec `"existing": true` : rien n'est créé, personne n'est prévenu deux fois, et
le contenu n'est pas comparé. C'est vrai même après l'avoir retirée ou
remplacée : un id que vous avez clos ne revient pas en le renvoyant.

Vos actions sont appelées avec `notification=<id>`, et `onAck` reçoit le même
id : celui que vous connaissez.

**Remplacer.** Publiez la nouvelle notification avec `"replaces"` qui nomme
l'id de l'ancienne. Donnez à la remplaçante un id à elle : sous l'id qu'elle
remplace, elle ne retrouverait que celle-ci, et c'est refusé.

```json
{ "id": "job-7-80", "title": "Export 7 is at 80%", "replaces": "job-7-40" }
```

L'ancienne se termine comme **remplacée** : elle quitte la liste et le compte de
chaque lecteur, la remplaçante tenant sa place, ses mails pas encore partis
sont annulés, et elle ne demande plus d'acquittement. La réponse dit ce qu'elle
a clos, dans `replaced`. Renvoyer la même remplaçante la répond avec
`"existing": true` et ne clôt rien de plus.

**Retirer.** `DELETE /api/notifications/{id}` la termine comme **retirée** :
elle quitte la liste et le compte de chaque lecteur sans rien laisser à sa
place, et ses mails pas encore partis sont annulés. Un id envoyé à tous les tenants retire chaque exemplaire. La réponse
dit si elle a été close maintenant, ou l'était déjà :

```json
{ "withdrawn": ["job-7-80"] }
{ "withdrawn": [], "already": ["job-7-80"] }
```

**Quand vos appels arrivent dans le désordre.** Rien ne garantit qu'un retrait
ou un remplacement atteigne la gateway après la notification qu'il nomme. S'il
nomme un id qu'aucune de vos notifications ne porte encore, la gateway le
garde : un retrait répond `202` avec `{"withdrawn": [], "pending": "<id>"}`, un
remplacement est publié et sa liste `replaced` est vide. Quand cet id arrive,
la notification est enregistrée **déjà close**, retirée ou remplacée, et
personne n'est prévenu ni écrit ; la réponse porte `"state"`. Ce qui est gardé
vit autant qu'une notification : la durée de la remplaçante, ou pour un retrait
30 jours sauf si `?expires=` dit autre chose (90 au plus).

| Une notification est | Sur la page du lecteur | Dans le compte |
|---|---|---|
| active | telle qu'envoyée | tant qu'elle n'est pas lue |
| retirée | absente | non |
| remplacée | absente : la remplaçante tient sa place | non |
| expirée | disparue, avec l'état de chaque lecteur | non |

La console les montre toutes avec leur état, leur émetteur et leur id.

### Quand le travail est fait

Une notification demande souvent un travail que votre service voit faire : un
document à valider, validé dans vos propres écrans, peut-être plusieurs d'un
coup, peut-être par un autre lecteur. Votre service le dit avec les mots du
lecteur, pour tous les lecteurs ou pour celui que `?user=` nomme (un id ou un
nom d'utilisateur : celui qu'il a vu faire le travail) :

| | |
|---|---|
| `POST /api/notifications/{id}/read` | lue : hors du compte, toujours listée |
| `POST /api/notifications/{id}/close` | fermée : hors des tiroirs et des listes, gardée sous Fermées sur la page des notifications |
| `DELETE /api/notifications/{id}?user=<qui>` | supprimée pour ce lecteur ; sans `user`, la notification est retirée pour tout le monde |

```bash
# Une seule validation suffit : fermée pour tous les lecteurs.
curl -X POST http://meerkat:7070/api/notifications/fd-101/close -H "$AUTH"
# {"id":"fd-101","readers":3}

# Chaque lecteur valide la sienne : fermée pour celui qui l'a fait.
curl -X POST "http://meerkat:7070/api/notifications/fd-102/close?user=alice" -H "$AUTH"
```

La réponse dit combien de lecteurs ont changé (0 quand il ne restait rien à
changer). Un acquittement encore dû ne la retient pas : c'est votre service qui
sait qu'elle n'a plus lieu d'être. Un message envoyé à tous les tenants change
dans tous. Les pages des lecteurs suivent aussitôt. Un utilisateur qui n'en lit
aucun exemplaire, ou un id qui n'est pas l'un des vôtres, répond `404`.

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
    "id": "overdue-INV-2026-0142",
    "meta": { "invoice": "INV-2026-0142" },
    "ack": "single",
    "onAck": { "service": "billing", "path": "/notifications/acked" }
  }'
```

```http
HTTP/1.1 201 Created
Content-Type: application/json

{"id": "overdue-INV-2026-0142", "recipients": 4}
```

### Les réponses

| Statut | Corps | Sens |
|---|---|---|
| `201` | `{"id", "recipients"}` | publiée sous votre `id`. Avec `replaces`, `replaced` dit ce qu'elle a clos ; `"state"` quand un retrait ou un remplacement avait nommé l'id avant son arrivée |
| `200` | le même, avec `"existing": true` | l'id est déjà connu : personne n'est prévenu de nouveau. `"state"` vaut `withdrawn` ou `replaced` quand elle est close |
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
  "id": "maintenance-2026-10-25",
  "recipients": 37,
  "tenants": [
    { "tenant": "7c1e2a90", "name": "Acme Logistics", "recipients": 21 },
    { "tenant": "b2f4c8d1", "name": "Acme Retail", "recipients": 16 }
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

Tant qu'un lecteur la doit, la rangée porte une barre sur son bord, porte **Acquitter**, et le
lecteur ne peut ni la fermer ni la supprimer ; **Tout supprimer** la garde et dit combien il en a gardé, et
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
Meerkat-Notification: overdue-INV-2026-0142
Meerkat-Attempt: 1

{"notification": "overdue-INV-2026-0142", "tenant": "7c1e2a90",
 "tenantName": "Acme Logistics",
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
une requête. La réponse liste ce qui a été retiré, `{"withdrawn": ["<id>", ...], "count": 2}`,
et ces notifications quittent aussitôt la page de chaque lecteur. La console les garde, marquées Withdrawn, avec qui les a lues.

## Le mail

Avec `"mail": true`, chaque lecteur qui a une adresse reçoit un mail dans sa
langue, au gabarit des mails de la gateway : le texte, et un bouton vers sa page
de notifications quand la gateway connaît son adresse publique. Un mail ne
porte jamais d'action. Un message envoyé à tous les tenants n'est écrit qu'une
fois au lecteur de plusieurs d'entre eux.

**La file est dans la base**, un mail par lecteur, chacun avec son état sur la
ligne du lecteur. Les nœuds les envoient un par un, un toutes les 200 ms, et un
seul nœud envoie à la fois, si bien qu'une audience de milliers de personnes
n'arrive pas d'un coup au relais. Deux nœuds n'envoient jamais le même mail :
un nœud réserve un mail avant de l'envoyer. Une gateway arrêtée en route, ou
redémarrée, ne perd rien : ce qui reste à envoyer part par le prochain nœud qui
tourne, ou par le même à son redémarrage.

| Le mail d'un lecteur est | Sens |
|---|---|
| `pending` | à envoyer, maintenant ou à sa prochaine tentative |
| `sending` | un nœud est en train de l'envoyer |
| `accepted` | le relais l'a accepté. Le plus que la gateway sache : jamais « remis » |
| `failed` | le relais l'a refusé pour de bon (une réponse `5yz`), ou un refus temporaire (`4yz`) a duré plus de 24 heures, ou plus que la vie de la notification. Le code de réponse est gardé |
| `unknown` | la connexion est tombée après l'envoi du message entier et avant la réponse du relais, ou le nœud qui l'envoyait s'est arrêté : le relais l'a peut-être pris, donc il n'est jamais renvoyé de lui-même |
| `cancelled` | la notification a été retirée, remplacée ou a expiré avant son départ |
| `noaddress` | le lecteur n'a pas d'adresse, ou son compte est désactivé |

Un refus temporaire, ou un relais injoignable, est retenté après 1, 5, 15 et
30 minutes, puis toutes les heures, pendant 24 heures au plus. Chaque tentative
d'un même mail porte le même `Message-ID`, pour qu'une boîte qui le reçoit deux
fois puisse le reconnaître.

**Où ils en sont.** Pour une notification envoyée avec mail, votre liste porte
`mails`, le compte des mails de ses lecteurs par état :

```json
{ "id": "overdue-INV-2026-0142", "mail": true,
  "mails": { "accepted": 38, "failed": 1, "noaddress": 2 } }
```

Une fois qu'il ne reste rien à envoyer, le résultat tient en une ligne de la
piste d'audit pour la notification, `notification.mail`, jamais une par
lecteur. La console montre le mail de chaque lecteur à côté de l'heure de sa
lecture.

## Ce que garde la piste d'audit

Ce qu'un service fait sur le plan interne est dans la
[piste d'audit](/docs/operations/audit), l'émetteur pour acteur, nommé comme
l'accès l'a reconnu (le nom d'un jeton, un ServiceAccount, une adresse), dans
le tenant de la notification :

| Action | Écrite quand |
|---|---|
| `notification.publish` | une notification est publiée : son émetteur, son tenant, combien de destinataires, si c'est un exercice, et si elle est arrivée déjà retirée ou remplacée. Envoyée à tous les tenants, une ligne par exemplaire |
| `notification.replace` | une notification est remplacée, en nommant celle qui la remplace |
| `notification.withdraw` | une notification est retirée, par son émetteur ou un administrateur |
| `notification.close` | son émetteur ferme une notification, pour tous les lecteurs ou pour un seul, nommé dans la ligne |
| `notification.delete` | son émetteur supprime une notification pour un lecteur, nommé dans la ligne |
| `notification.mail` | l'envoi des mails d'une notification est terminé : une ligne pour la notification, jamais une par destinataire, avec combien de mails le relais a acceptés, combien ont échoué, combien de lecteurs n'avaient pas d'adresse, et combien ont été annulés |
| `notification.given` | des notifications ont été données à quelqu'un après leur publication, parce qu'il a rejoint le tenant ou reçu un rôle, ou à l'ouverture de sa liste : une ligne qui nomme la personne et les notifications, la gateway pour acteur |
| `notification.ack` | un lecteur en acquitte une : une ligne par acquittement, le lecteur pour acteur, gardée après l'expiration de la notification |

Lire, fermer et supprimer appartiennent au lecteur, gardés sur sa ligne de la
notification, et ne sont pas dans la piste ; une lecture marquée par
l'émetteur non plus. L'écran **Data plane,
Notifications** de la console et la section **Notifications** du tenant suivent
chaque publication en direct, celles du plan interne comme celles de la console.

## Votre propre page de notifications

La page intégrée est un choix. Une application peut montrer les notifications
dans ses propres écrans : l'administrateur indique alors cette page à la gateway
(**Application, Portal, Notifications page** : une route UI et un chemin
dessous), et le tiroir, le snackbar et les mails y mènent. Qui
ne peut pas ouvrir cette route reçoit quand même la page intégrée, qui reste
ouverte à tous.

Votre page lit les notifications du lecteur sur le plan de données, avec la
session :

| | |
|---|---|
| `GET /meerkat/notifications.json` | une page des notifications du lecteur, dans le tenant de sa session. `?unread=1`, `?closed=1` (ce qu'il a fermé) et `?q=` la réduisent ; `?q=` cherche dans le titre, le texte et les métadonnées sur le serveur et répond tout ce qui correspond d'un coup, jamais les métadonnées elles-mêmes |
| `GET /meerkat/notifications/counts` | le nombre de non lues |
| `POST /meerkat/notifications/{id}/read` | en marquer une comme lue ; `POST /meerkat/notifications/read` les marque toutes, ou ce que montrent `?tenant=`, `?unread=1` et `?q=` |
| `POST /meerkat/notifications/{id}/ack` | en acquitter une |
| `POST /meerkat/notifications/{id}/close` | en fermer une pour ce lecteur ; `POST /meerkat/notifications/{id}/reopen` la ramène |
| `DELETE /meerkat/notifications/{id}` | en supprimer une pour ce lecteur, pour de bon ; `DELETE /meerkat/notifications` les supprime toutes, ou ce que montrent `?unread=1`, `?closed=1` et `?q=`, sauf ce qui attend un acquittement |

Chaque action de la liste arrive avec sa méthode et son corps, prête à envoyer.
Le contrat complet est dans la référence d'API du plan de données (**Meerkat,
API reference** dans la console).
