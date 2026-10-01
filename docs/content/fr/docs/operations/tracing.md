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
> Le contexte circule dans les deux éditions ; l'export est Enterprise, comme
> pour Prometheus.

On parle **OTLP**, pas un produit : l'adresse pointe vers ce que vous faites
déjà tourner - OpenTelemetry Collector, Tempo, Jaeger, ou l'endpoint d'un
éditeur.

Le réglage est dans la console, **Infra, OpenTelemetry**, avec les autres
systèmes auxquels l'installation est branchée. Il s'applique à chaud, sur
chaque noeud.

Un interrupteur envoie au collecteur, et deux disent **quoi** : les
**traces**, dont parle surtout cette page, et les **métriques** - les mêmes
compteurs que ceux de l'[endpoint de métriques](/docs/operations/metrics),
poussés en OTLP toutes les 30 secondes. L'un, l'autre, ou les deux.

| champ | | défaut |
|---|---|---|
| Adresse du collecteur | l'adresse **de base** : `/v1/traces` (et `/v1/metrics` quand les métriques partent) est ajouté pour vous, et un chemin collé est retiré | - |
| En-tête d'authentification | une **référence de coffre** (`$otlp-token`), jamais la clef elle-même | - |
| Journeys recorded | la part des voyages **ouverts ici** : à la porte d'entrée pour un appel qui arrive sans décision, dans la page quand la moitié navigateur est allumée. Une décision déjà prise par l'appelant est respectée plutôt que rejouée | **10 %** |
| Ceiling | le plafond de voyages enregistrés par seconde, quel qu'en soit le décideur - votre budget | **200/s** |

Le bouton **Test**, à côté d'Enregistrer, pose la question au collecteur avant
qu'on enregistre quoi que ce soit : la passerelle lui envoie un lot **vide** sur
`/v1/traces` et rend ce qu'il a répondu, avec le temps que ça a pris. Tout le
chemin est exercé - résolution du nom, réseau, TLS, la crédentiale - et aucune
trace n'est écrite nulle part. Les erreurs qui arrivent vraiment sont nommées,
et la première est celle qu'un code de retour seul ne voit pas : l'adresse de l'**interface** d'un collecteur au
lieu de son port OTLP. Une interface web répond 200 à tout - elle sert sa page
pour n'importe quel chemin -, donc le test regarde AUSSI ce qui a répondu, et
refuse une page (Jaeger écoute les traces en 4318 et sert son UI en 16686).
L'autre est une crédentiale refusée. Avec les métriques cochées, il interroge
aussi `/v1/metrics`, et le dit quand un backend ne prend que des traces - ce
que fait Jaeger.

**Une trace n'est pas un compteur, et n'est pas censée être exhaustive.** A
100 %, une journée à 400 req/s fait trente-cinq millions de traces, que votre
backend facture et indexe. A 10 %, le profil de latence est le même - c'est une
distribution, pas un inventaire - et la requête précise que vous cherchez, vous
la retrouvez par son `trace_id` quand elle a été tirée. Ce que vous voulez
exhaustif, ce sont les [métriques](/docs/operations/metrics) et le
[journal d'accès](/docs/operations/logs) : ils sont à 100 % et le restent parce
qu'ils ne coûtent pas par requête.

Meerkat émet deux spans : un span `SERVER` qui couvre la traversée complète,
de l'arrivée de la requête jusqu'à la réponse **écrite** - filtres de sortie et
injections comprises - et, à l'intérieur, un span `CLIENT` autour de l'appel à
votre service. **L'écart entre les deux est son temps propre.** La route
choisie et le verdict d'accès y sont des attributs, pas des spans - un span par
étape interne se compterait en milliers par requête.

Un collecteur en panne ne ralentit jamais le trafic : les spans partent d'une
file bornée, et ce qui ne passe pas est jeté.

::: details Un collecteur, si vous n'en avez pas
La page OpenTelemetry vous donne le fichier, pour Docker Swarm et
pour Kubernetes, avec l'adresse à recopier dans le champ ci-dessus.
:::

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
**OpenTelemetry**, avec deux interrupteurs.

**Push traces of this route to OpenTelemetry** (allumé par défaut) : la
passerelle enregistre ses propres spans pour ce que cette route répond. Éteint,
cette route ne produit **rien du tout** - ni traversée, ni appel amont, ni
paquet injecté - et le contexte que l'appelant a envoyé repart intact, sans que
la passerelle se déclare son parent.

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

## Ce qui manque

- `tracestate` et `baggage` traversent, mais la passerelle ne s'y déclare pas.
- Aucun lien cliquable entre une ligne du [journal d'audit](/docs/operations/audit)
  et sa trace.
- Une réponse qui ne finit pas - SSE, WebSocket - ferme son span à
  l'établissement : le reste est compté, pas tracé.
