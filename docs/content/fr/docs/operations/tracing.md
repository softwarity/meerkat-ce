---
title: Traces
section: Exploitation
order: 210
summary: Où passent les secondes d'une requête, et le temps que seule la gateway peut mesurer.
---

# Traces

Vous tracez déjà vos services, et il reste un angle mort : le temps passé **avant**
eux.

```
[ navigateur ............................. ] 4,2 s
     [ orders-api ...................... ]   3,9 s
         [ payment-api ................ ]    3,8 s
     ^^^^
     300 ms que personne ne sait expliquer
```

Ces 300 ms sont celles de la gateway : choisir la route, vérifier l'accès,
appliquer les filtres, joindre l'upstream. Personne d'autre dans le cluster ne peut
les mesurer, et c'est sur un trou dans une trace que démarre une discussion
entre deux équipes. Meerkat s'y déclare :

```
[ navigateur ............................. ] 4,2 s
  [ meerkat .............................. ] 4,1 s
      route=orders   access=session, role orders_write
      [ orders-api ...................... ]   3,9 s
          [ payment-api ................ ]    3,8 s
```

## Traces et métriques ne se remplacent pas

| | répond à | exhaustif ? | conservation |
|---|---|---|---|
| [Métriques](/docs/operations/metrics) | combien, en ce moment | 100 % | des mois |
| Traces | où ça casse, et de quel côté | échantillonné | quelques jours |

On ne déclenche jamais d'alerte sur une trace, et on ne diagnostique jamais un cas
particulier à partir d'un compteur.

## Sans rien activer

`traceparent`, `tracestate` et `baggage` traversent la gateway intacts : une
installation qui trace déjà ses services conserve sa trace complète.

Et **chaque requête reçoit un identifiant**, qu'elle en ait apporté un ou non. Il
vous revient par trois chemins :

```
en-tête de réponse   Meerkat-Trace-Id: 4bf92f3577b34da6a3ce929d0e0e4736
dans le journal      {"type":"access","trace_id":"4bf92f35...", ...}
en pied de page      discret, sous la marque, sélectionnable
```

C'est ce qui permet à votre support de partir d'un identifiant lu sur un écran, et
c'est la clé qui relie une ligne du [journal d'accès](/docs/operations/logs) à
l'audit métier de votre service. Cet identifiant existe sur 100 % des requêtes et
vit aussi longtemps que vos journaux - une trace, à l'inverse, est échantillonnée
et conservée quelques jours.

Un identifiant créé ici voyage avec le bit d'échantillonnage à `00`, ce qui
signifie "voici le nom de ce parcours, personne n'est tenu de le remonter" : sans
export activé, vous verrez l'identifiant partout et aucune trace dans votre
backend. C'est le comportement attendu.

## Exporter vers votre collecteur

> [!NOTE] Édition Enterprise
> Le contexte circule dans les deux éditions. L'export relève de l'édition
> Enterprise.

L'export parle **OTLP**, pas le langage d'un produit : l'adresse peut être
n'importe quel endpoint OTLP. Nous recommandons un **OpenTelemetry Collector**. Il
reçoit tous les signaux à une seule adresse et envoie chacun là où il doit aller :
les traces vers Tempo, les métriques vers Prometheus, les journaux et l'audit vers
Loki. Il échantillonne aussi sur des traces complètes (tail sampling) : la
gateway peut donc enregistrer **100 %** des traces et le laisser choisir.

Le réglage se trouve dans **Infra, OpenTelemetry**. Il s'applique à chaud, sur tous
les nœuds.

![L'écran OpenTelemetry : l'interrupteur d'export, l'adresse du collecteur, et l'onglet Traces avec son échantillonnage et son plafond](img/console/opentelemetry.webp)

Un interrupteur général, **Export to an OpenTelemetry collector**, active l'export.
Quatre onglets précisent ensuite ce qui est envoyé :

| onglet | contenu |
|---|---|
| **Traces** | un interrupteur : les traces dont parle cette page |
| **Metrics** | un interrupteur : les compteurs de la gateway, poussés toutes les 30 secondes. C'est leur seule façon de sortir de la gateway (voir les [métriques](/docs/operations/metrics)) |
| **Audit** | deux interrupteurs : **Send the audit logs** (connexions, refus, changements d'identifiants, opérations d'[Endpoint audit](/docs/operations/audit#auditer-les-oprations-dune-route)) et **Send Meerkat's console audit too** (voir l'[audit](/docs/operations/audit)) |
| **Logs** | un mode parmi trois, décrits ci-dessous |

L'onglet **Logs** propose trois modes (voir les [journaux](/docs/operations/logs)) :

- **Not sent** : les journaux gardent leur format habituel.
- **Written for an agent** : les journaux sont écrits en JSON OpenTelemetry sur la
  sortie standard. Un Collector présent sur chaque nœud (un DaemonSet) les lit. Ce
  mode fonctionne sans l'export, dans les deux éditions.
- **Pushed to the collector** : les journaux sont envoyés en OTLP à l'adresse du
  collecteur, pour les nœuds dépourvus d'agent. Ce mode nécessite l'export
  (Enterprise).

| champ | | valeur par défaut |
|---|---|---|
| Collector address | l'adresse **de base** : `/v1/traces` et `/v1/metrics` y sont ajoutés, et un chemin collé avec l'adresse est retiré | - |
| Auth header | le nom de l'en-tête, par exemple `Authorization`. En général, seul un collecteur hébergé en a besoin | - |
| Its value | stockée dans le coffre : la configuration ne garde que la référence (`Bearer $otlp-token`) | - |
| Journeys recorded (%) | la part des traces **démarrées ici**. La décision d'échantillonnage prise par un appelant est respectée | **10 %** |
| Ceiling (per second) | le nombre maximal de traces enregistrées par seconde, quel que soit celui qui a décidé. 0 pour ne pas limiter | **200/s** |

Le bouton **Test** envoie un lot **vide** à `/v1/traces` avant que vous
n'enregistriez. Il éprouve tout le chemin : résolution du nom, réseau, TLS,
authentification. Aucune trace n'est écrite. Il refuse une page web : l'adresse de
l'interface d'un collecteur répond 200 à n'importe quoi, mais ce n'est pas le port
OTLP. Si les métriques sont activées, il interroge aussi `/v1/metrics`, et signale
un backend qui n'accepte que les traces.

**Une trace n'est pas un compteur.** À 100 %, une journée à 400 req/s représente
trente-cinq millions de traces à facturer et à indexer. À 10 %, le profil de latence
est le même, et vous retrouvez une requête par son `trace_id` lorsqu'elle a été
tirée au sort. Ce que vous voulez exhaustif, ce sont les
[métriques](/docs/operations/metrics) et le
[journal d'accès](/docs/operations/logs) : eux restent à 100 %.

Meerkat émet deux spans : un span `SERVER` pour toute la traversée, jusqu'à
l'**écriture** de la réponse, et à l'intérieur un span `CLIENT` autour de l'appel à
votre service. **L'écart entre les deux est le temps propre de la gateway.** La
route et le verdict d'accès sont des attributs, pas des spans.

Un collecteur en panne ne ralentit jamais le trafic : les spans partent d'une file
d'attente bornée, et ce qui n'y tient pas est abandonné.

### Pas encore de collecteur ?

Sur la page OpenTelemetry, **No collector yet?** ouvre les fichiers nécessaires
pour déployer un OpenTelemetry Collector, sous Docker Swarm ou sous Kubernetes :

- la configuration du Collector ;
- son agent de journaux sur chaque nœud (un DaemonSet sous Kubernetes), qui lit
  les sorties des conteneurs ;
- sous Kubernetes, un Service `opentelemetry` : chaque producteur envoie ses
  données à `opentelemetry:4318` ;
- deux fichiers d'accompagnement pour Grafana : les sources de données
  (Prometheus, Tempo, Loki) et un tableau de bord pour les
  [métriques](/docs/operations/metrics).

![No collector yet? ouvre un tiroir contenant les fichiers pour en déployer un, sous Docker Swarm ou sous Kubernetes](img/console/opentelemetry-collector.webp)

## Démarrer la trace au clic

Le second interrupteur d'une route d'interface, **Start the trace from the UI**,
injecte le bundle OpenTelemetry dans les pages de cette application.

Il vous montre ce que la gateway ne verra jamais : le réseau situé devant elle,
la file d'attente du navigateur, le rendu - et surtout **les appels qui ne vous
parviennent jamais**. Une résolution DNS qui échoue, un refus CORS, un appareil
hors ligne : vos tableaux de bord sont au vert et le support croule sous les
appels.

Trois choses à savoir :

- **Le script est servi par Meerkat, jamais par un CDN** : une installation coupée
  d'Internet fonctionne, et aucun tiers ne voit vos utilisateurs.
- **Un tiers ne reçoit jamais de `traceparent`** : la page ne le transmet qu'aux
  adresses servies par la gateway.
- **Votre collecteur reste privé** : les spans de la page reviennent par la
  gateway, qui les relaie avec des identifiants qu'aucune page ne voit jamais.

## Choisir les routes tracées

Le traçage se décide **route par route**, dans l'éditeur de route, section
**OpenTelemetry**, à l'aide de deux interrupteurs. Une requête à laquelle **aucune
route ne répond** - un 404 - n'est jamais tracée : aucune route n'a dit oui, et une
telle requête n'est le parcours de personne. Elle reste comptée dans les métriques
et inscrite dans le journal d'accès.

**Include this route in OpenTelemetry** (activé par défaut) : la gateway
enregistre ses propres spans pour ce que cette route sert, et la route a ses
propres séries parmi les métriques poussées vers le collecteur. Désactivé, la
route ne produit **rien du tout** - ni traversée, ni appel sortant, ni bundle
injecté, ni série propre - mais elle compte toujours dans les totaux de la
gateway (`meerkat.gateway.requests`), et le contexte envoyé par l'appelant
poursuit sa route intact, sans que la gateway s'en déclare le parent.

**Start the trace from the UI** (désactivé par défaut, et disponible uniquement
quand le premier est activé et que la route est une route d'interface) : le bundle
OpenTelemetry est injecté dans les pages de cette application, et le parcours
commence au **clic** plutôt qu'à la gateway.

- Le premier seul : le span `00` est celui de Meerkat, et votre service est son
  enfant.
- Les deux : le span `00` est celui de la page, et la traversée de Meerkat
  devient l'enfant du clic.

C'est ce qui garde le backend lisible. Une gateway voit tout passer, et
l'essentiel de ce qu'elle voit n'est le parcours de personne : l'administration de
RabbitMQ qu'un exploitant garde ouverte dans un onglet, le Jaeger que consulte un
collègue, un health check toutes les dix secondes. Cochez les applications qui
méritent d'être suivies, et les interfaces d'exploitation servies à côté ne
coûtent rien.

L'autre raison de désactiver une route, c'est une application qui embarque
**déjà** son propre agent : deux agents sur une même page produisent deux traces
pour un seul clic, et aucune ne raconte toute l'histoire.

Ce que cela ne change pas : le `traceparent` est toujours généré et figure
toujours dans le journal d'accès - c'est la clé qui relie une de nos lignes à
l'audit de votre service, et ce n'est pas une trace.

## Voir le travail de la gateway

Par défaut, une traversée se compose de deux spans : l'entrée dans Meerkat et
l'appel à votre service. L'écart entre les deux est le temps propre de la
gateway, et il tient en un seul chiffre.

L'interrupteur **Detail the gateway's own work** (Infra, OpenTelemetry) le
décompose : chaque traversée enregistrée porte alors ses étapes - l'identité
transmise à votre service, avec l'identité de l'appelant et la signature du jeton,
et chaque requête adressée à la base de données, nommée `SELECT users` et
accompagnée de son texte (les paramètres substituables, jamais les valeurs).

Il est **désactivé par défaut**, à dessein. Il n'ajoute aucune trace : il
approfondit celles qui sont déjà échantillonnées, de sorte que le taux et le
plafond gardent leur sens - mais chacune pèse plus lourd dans votre collecteur.
Activez-le le temps de regarder où passe le temps de la gateway, puis
désactivez-le.

## Qui a fait l'appel

Une trace répond à la question "qu'est-ce qui a été lent ?" ; un ticket de support
demande "pour qui ?". L'interrupteur **Name the caller on the spans** (Infra,
OpenTelemetry) inscrit l'appelant connecté sur la trace :

| Attribut | Contenu |
|---|---|
| `user.id`, `user.name` | le compte (les noms définis par OpenTelemetry, qu'un backend qui les connaît affiche comme un utilisateur) |
| `meerkat.tenant.id`, `meerkat.tenant.name` | l'organisation active |
| `meerkat.group` | le groupe choisi pour la session, dans une organisation qui fonctionne en mode exclusif |
| `meerkat.roles` | les rôles sur lesquels les règles d'accès ont été évaluées - une liste, et ce qui explique un refus |

Un appel anonyme ne porte rien.

**Une fois par parcours, jamais deux.** Un parcours qui démarre dans une page porte
la personne sur les spans du navigateur ; le span de la gateway sur ce parcours
reste muet. Tout autre parcours - un backend, un script, une page sans le bundle -
la porte sur le span de la gateway. Le bundle marque les parcours qu'il ouvre
avec `meerkat=b` dans `tracestate`, et c'est ainsi que la gateway les distingue.
La recherche d'un backend de traces retrouve tout de même la trace entière à partir
de n'importe lequel de ces attributs : une trace correspond dès que l'un de ses
spans correspond.

Il est **désactivé par défaut** : une personne dans une trace, c'est une donnée
personnelle envoyée vers un collecteur que quelqu'un d'autre exploite peut-être.

Les spans du navigateur sont marqués **par la gateway**, au niveau du relais qui
les transmet à votre collecteur : la page ne sait jamais qui la lit, et un attribut
qu'une page définit sous l'un de ces noms est remplacé par la réponse de la
gateway au lieu d'être cru sur parole.

## Ce qui manque

- `tracestate` et `baggage` traversent la gateway, mais elle ne s'y déclare
  pas.
- Aucun lien cliquable entre une ligne du [journal d'audit](/docs/operations/audit)
  et sa trace.
- Une réponse qui ne se termine jamais - SSE, WebSocket - ferme son span à
  l'établissement : le reste est compté, pas tracé.
