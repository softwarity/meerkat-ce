---
title: Traces
section: Exploitation
order: 210
summary: Où passent les secondes d'une requête, et le temps que seule la passerelle peut mesurer.
---

# Traces

Vous tracez déjà vos services, et il reste un trou : le temps passé **avant**
eux.

```
[ navigateur ............................. ] 4,2 s
     [ orders-api ...................... ]   3,9 s
         [ payment-api ................ ]    3,8 s
     ^^^^
     300 ms que personne n'explique
```

Ces 300 ms sont celles de la passerelle : choisir la route, contrôler l'accès,
appliquer les filtres, joindre l'amont. Personne d'autre dans le cluster ne
peut les mesurer, et un trou dans une trace est là où commence une discussion
entre deux équipes. Meerkat se déclare dessus :

```
[ navigateur ............................. ] 4,2 s
  [ meerkat .............................. ] 4,1 s
      route=orders   acces=session, role orders_write
      [ orders-api ...................... ]   3,9 s
          [ payment-api ................ ]    3,8 s
```

## Traces et métriques ne se remplacent pas

| | répond à | exhaustif ? | gardé |
|---|---|---|---|
| [Métriques](/docs/operations/metrics) | combien, en ce moment | 100 % | des mois |
| Traces | où ça casse, et de quel côté | échantillonné | quelques jours |

On n'alerte jamais sur une trace, et on ne diagnostique jamais un cas précis
sur un compteur.

## Sans rien activer

`traceparent`, `tracestate` et `baggage` traversent la passerelle intacts : une
installation qui trace déjà ses services garde sa trace entière.

Et **chaque requête reçoit un identifiant**, que rien n'en apporte ou non. Il
vous revient de trois façons :

```
en-tete de reponse    Meerkat-Trace-Id: 4bf92f3577b34da6a3ce929d0e0e4736
dans le journal       {"type":"access","trace_id":"4bf92f35...", ...}
au pied des pages     discret, sous la marque, selectionnable
```

C'est ce qui permet à votre support de partir d'un identifiant lu à l'écran, et
c'est la clef qui joint une ligne du [journal d'accès](/docs/operations/logs) à
l'audit métier de votre service. Elle existe sur 100 % des requêtes et vit
aussi longtemps que vos journaux - une trace, elle, est échantillonnée et
gardée quelques jours.

Un identifiant ouvert ici part avec le bit d'échantillonnage à `00`, c'est-à-
dire « voici le nom de ce voyage, personne n'est censé le rapporter » : sans
export activé, vous verrez donc cet identifiant partout et aucune trace dans
votre backend, ce qui est le comportement attendu.

## Exporter vers votre collecteur

> [!NOTE] Enterprise edition
> Le contexte circule dans les deux éditions. L'export est Enterprise.

L'export parle **OTLP**, pas un produit : l'adresse peut être n'importe quel
endpoint OTLP. Nous recommandons un **OpenTelemetry Collector**. Il reçoit tous
les signaux sur une seule adresse et envoie chacun à sa place : les traces à
Tempo, les métriques à Prometheus, les logs et l'audit à Loki. Il échantillonne
aussi les traces complètes (tail sampling), donc la passerelle peut enregistrer
**100 %** et lui laisser le choix.

Le réglage est dans **Infra, OpenTelemetry**. Il s'applique à chaud, sur chaque
noeud.

![L'écran OpenTelemetry : l'interrupteur d'export, l'adresse du collecteur, et l'onglet Traces avec son échantillonnage et son plafond](img/console/opentelemetry.webp)

Un interrupteur principal, **Export to an OpenTelemetry collector**, active
l'export. Quatre onglets disent ensuite ce qui part :

| onglet | quoi |
|---|---|
| **Traces** | un interrupteur : les traces dont parle cette page |
| **Metrics** | un interrupteur : les compteurs de la passerelle, poussés toutes les 30 secondes. C'est leur seule façon de sortir (voir [les métriques](/docs/operations/metrics)) |
| **Audit** | deux interrupteurs : **Send the audit logs** (connexions, refus, changements d'identifiants, opérations cochées dans [Endpoint audit](/docs/operations/audit#auditer-les-oprations-dune-route)) et **Send Meerkat's console audit too** (voir [l'audit](/docs/operations/audit)) |
| **Logs** | un mode parmi trois, ci-dessous |

L'onglet **Logs** a trois modes (voir [les journaux](/docs/operations/logs)) :

- **Not sent** : les journaux gardent leur format habituel.
- **Written for an agent** : les journaux sont écrits en JSON OpenTelemetry sur
  stdout. Un Collector sur chaque noeud (un DaemonSet) les lit. Marche export
  coupé, dans les deux éditions.
- **Pushed to the collector** : les journaux partent en OTLP vers l'adresse du
  collecteur, pour les noeuds sans agent. Demande l'export (Enterprise).

| champ | | défaut |
|---|---|---|
| Collector address | l'adresse **de base** : `/v1/traces` et `/v1/metrics` sont ajoutés, et un chemin collé est retiré | - |
| Auth header | le nom de l'en-tête, par exemple `Authorization`. Utile surtout pour un collecteur hébergé | - |
| Its value | rangée dans le coffre : la configuration ne garde que la référence (`Bearer $otlp-token`) | - |
| Journeys recorded (%) | la part des traces **ouvertes ici**. La décision d'échantillonnage de l'appelant est gardée | **10 %** |
| Ceiling (per second) | au plus ce nombre de traces enregistrées par seconde, quel qu'en soit le décideur. 0 pour aucune limite | **200/s** |

Le bouton **Test** envoie un lot **vide** sur `/v1/traces` avant
l'enregistrement. Il exerce tout le chemin : résolution du nom, réseau, TLS,
crédentiale. Aucune trace n'est écrite. Il refuse une page web : l'adresse de
l'interface d'un collecteur répond 200 à tout, mais ce n'est pas le port OTLP.
Avec les métriques cochées, il interroge aussi `/v1/metrics`, et dit quand un
backend ne prend que des traces.

**Une trace n'est pas un compteur.** A 100 %, une journée à 400 req/s fait
trente-cinq millions de traces à facturer et indexer. A 10 %, le profil de
latence est le même, et une requête précise se retrouve par son `trace_id`
quand elle a été tirée. Ce que vous voulez exhaustif, ce sont les
[métriques](/docs/operations/metrics) et le
[journal d'accès](/docs/operations/logs) : ils restent à 100 %.

Meerkat émet deux spans : un span `SERVER` pour toute la traversée, jusqu'à la
réponse **écrite**, et dedans un span `CLIENT` autour de l'appel à votre
service. **L'écart entre les deux est le temps propre de la passerelle.** La
route et le verdict d'accès sont des attributs, pas des spans.

Un collecteur en panne ne ralentit jamais le trafic : les spans partent d'une
file bornée, et ce qui ne passe pas est jeté.

### Pas encore de collecteur ?

**No collector yet?** sur la page OpenTelemetry ouvre les fichiers pour
déployer un OpenTelemetry Collector, sur Docker Swarm ou Kubernetes :

- la configuration du Collector ;
- son agent de logs sur chaque noeud (un DaemonSet sur Kubernetes), qui lit les
  sorties des conteneurs ;
- sur Kubernetes, un Service `opentelemetry` : chaque producteur envoie à
  `opentelemetry:4318` ;
- deux fichiers Grafana compagnons : les sources de données (Prometheus, Tempo,
  Loki) et un tableau de bord pour les [métriques](/docs/operations/metrics).

![No collector yet? ouvre un tiroir avec les fichiers pour en déployer un, sur Docker Swarm ou sur Kubernetes](img/console/opentelemetry-collector.webp)

## Commencer la trace au clic

Le second interrupteur d'une route UI, **Start the trace from the UI**, injecte
le paquet OpenTelemetry dans les pages de cette application.

Ce qu'il vous montre et que la passerelle ne verra jamais : le réseau avant
elle, la file d'attente du navigateur, le rendu - et surtout **les appels qui
n'arrivent jamais jusqu'à vous**. Un DNS qui échoue, un CORS refusé, un
appareil hors ligne : vos tableaux de bord sont verts et le support reçoit des
appels.

Trois choses à savoir :

- **Le script vient de Meerkat, jamais d'un CDN** : une installation coupée
  d'Internet fonctionne, et aucun tiers ne voit vos utilisateurs.
- **Un tiers ne reçoit jamais de `traceparent`** : la page ne le porte que vers
  les adresses que la passerelle sert.
- **Votre collecteur reste privé** : les spans de la page remontent par la
  passerelle, qui les relaie avec une crédentiale qu'aucune page ne voit.

## Choisir les routes tracées

Le tracing se décide **route par route**, dans l'éditeur de route, section
**OpenTelemetry**, avec deux interrupteurs. Une requête à laquelle **aucune
route ne répond** - un 404 - n'est jamais tracée : aucune route n'a dit oui, et
une telle requête n'est le parcours de personne. Elle reste comptée dans les
métriques et écrite dans le journal d'accès.

**Include this route in OpenTelemetry** (allumé par défaut) : la passerelle
enregistre ses propres spans pour ce que cette route répond, et la route a ses
propres séries parmi les métriques poussées au collecteur. Éteint, cette route
ne produit **rien du tout** - ni traversée, ni appel amont, ni paquet injecté, ni
série à elle - même si elle compte toujours dans les totaux de la passerelle
(`meerkat.gateway.requests`), et le contexte que l'appelant a envoyé repart
intact, sans que la passerelle se déclare son parent.

**Start the trace from the UI** (éteint par défaut, et disponible seulement si
le premier est allumé et que la route est une UI) : le paquet OpenTelemetry est
injecté dans les pages de cette application, et le voyage commence au **clic**
plutôt qu'à la passerelle.

- Premier seul : le span `00` est celui de Meerkat, votre service est son
  enfant.
- Les deux : le span `00` est celui de la page, et la traversée de Meerkat
  devient l'enfant du clic.

C'est ce qui rend le backend lisible. Une passerelle voit tout passer, et la
plupart de ce qu'elle voit n'est le voyage de personne : l'administration de
RabbitMQ qu'un exploitant garde ouverte dans un onglet, Jaeger qu'un collègue
consulte, une sonde de santé toutes les dix secondes. On coche les applications
que l'on veut suivre, et les interfaces d'exploitation servies à côté ne coûtent
rien.

L'autre raison d'éteindre une route est une application qui porte **déjà** son
propre agent : deux agents sur une page donnent deux traces pour un clic, et
aucune n'est l'histoire complète.

Ce que cela ne change pas : le `traceparent` est toujours généré et toujours
repris dans le journal d'accès - c'est la clef qui joint une ligne de Meerkat à
l'audit de votre service, et elle n'est pas une trace.

## Voir le travail de la passerelle

Par défaut, une traversée, ce sont deux spans : l'entrée dans Meerkat et l'appel à votre service. L'écart entre les deux est
le temps propre de la passerelle, et c'est un seul chiffre.

L'interrupteur **Detail the gateway's own work** (Infra, OpenTelemetry) le découpe : chaque traversée enregistrée porte
alors ses étapes - la transmission de l'identité à votre service, avec qui est l'appelant et la signature du jeton, et
chaque requête au store, nommée `SELECT users` avec son texte (les marqueurs, jamais les valeurs).

Il est **éteint par défaut**, et c'est voulu. Il n'ajoute aucune trace : il approfondit celles qui sont déjà
échantillonnées, donc le taux et le plafond gardent leur sens - mais chacune pèse plus lourd dans votre collecteur.
Allumez-le le temps de regarder où passe le temps de la passerelle, puis éteignez-le.

## Qui a fait l'appel

Une trace répond à « qu'est-ce qui a été lent » ; un ticket de support demande « pour qui ». L'interrupteur **Name the
caller on the spans** (Infra, OpenTelemetry) pose l'appelant connecté sur la trace :

| Attribut | Quoi |
|---|---|
| `user.id`, `user.name` | le compte (les noms d'OpenTelemetry lui-même, qu'un backend qui les connaît affiche comme un utilisateur) |
| `meerkat.tenant.id`, `meerkat.tenant.name` | l'organisation active |
| `meerkat.group` | le groupe choisi par la session, dans une organisation en mode exclusif |
| `meerkat.roles` | les rôles sur lesquels les règles d'accès ont été jugées - une liste, et ce qui explique un refus |

Un appel anonyme ne porte rien.

**Une fois par parcours, jamais deux.** Un parcours qui commence dans une page porte la personne sur les spans du
navigateur ; le span de la passerelle sur ce parcours reste muet. Tout autre parcours - un backend, un script, une page
sans le bundle - la porte sur le span de la passerelle. Le bundle marque les parcours qu'il ouvre avec `meerkat=b` dans
`tracestate`, et c'est ainsi que la passerelle les distingue. La recherche d'un backend de traces retrouve quand même la trace entière
à partir de n'importe lequel de ces attributs : une trace correspond dès qu'un de ses spans correspond.

Il est **éteint par défaut** : une personne dans une trace est une donnée personnelle qui part vers un collecteur que
quelqu'un d'autre fait peut-être tourner.

Les spans du navigateur sont estampillés **par la passerelle**, au relais qui les transmet à votre collecteur : la page
n'apprend jamais qui la lit, et un attribut qu'une page poserait sous l'un de ces noms est remplacé par la réponse de
la passerelle plutôt que cru.

## Ce qui manque

- `tracestate` et `baggage` traversent, mais la passerelle ne s'y déclare pas.
- Aucun lien cliquable entre une ligne du [journal d'audit](/docs/operations/audit)
  et sa trace.
- Une réponse qui ne finit pas - SSE, WebSocket - ferme son span à
  l'établissement : le reste est compté, pas tracé.
