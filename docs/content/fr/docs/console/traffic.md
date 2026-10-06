---
title: Metrics
section: La console
order: 182
summary: Ce que la gateway a réellement servi - cinq chiffres, deux courbes et un classement des routes, détaillé jusqu'à leurs endpoints.
---

# Metrics

**Data plane, Metrics** montre ce que la gateway a réellement servi. Tous les
autres écrans montrent ce qui est **configuré** ; or, dans une configuration, rien ne
distingue une route en échec d'une route que personne n'appelle.

L'écran se trouve à l'adresse `/data-plane/metrics`. Il est réservé au compte root et aux administrateurs infra :
ces chiffres nomment chaque route et chaque service, et dessinent ainsi une carte de
l'installation.

![L'écran Metrics : cinq chiffres sur la dernière minute, les courbes de trafic et de latence, et le classement des routes](img/console/traffic.webp)

Les requêtes par seconde, la part des requêtes refusées ou en échec, le temps de réponse
moyen, le p95 et les requêtes en cours ; en dessous, les deux courbes, puis le
classement et ses trois onglets, celui des échecs affichant son compteur.

## Ce que contient la page

- **Over the last minute** : les requêtes par seconde, la part des requêtes refusées ou
  en échec, le temps de réponse moyen, le **p95** et le nombre de requêtes en cours. Le
  p95 est le temps en dessous duquel 95 % des réponses sont arrivées, ce que masque la
  moyenne : quatre-vingt-dix réponses rapides et dix réponses de trois secondes donnent
  une moyenne que personne n'a réellement attendue.
- **Traffic** et **How long an answer takes** : deux courbes, la seconde traçant côte à
  côte la moyenne et le p95. La gateway les alimente en poussant un intervalle toutes
  les cinq secondes. L'écran n'interroge rien périodiquement.
- **Routes**, classées selon l'un des trois axes - **Slowest**, **Failing**,
  **Costliest** - sur la fenêtre que couvrent les échantillons. L'onglet Failing affiche
  son compteur : il attire l'œil sans que vous ayez à changer d'onglet pour savoir s'il
  y a quelque chose à y voir.

![Le classement des routes, plus bas dans la page : cinq routes avec leurs requêtes, leurs échecs, leur temps de réponse moyen et le temps consommé](img/console/metrics.webp)

Plus bas sur le même écran : les cinq routes, sur les minutes que couvrent les
échantillons. *Inventory (maintenance)* répond à chaque requête en moins d'une
milliseconde et les compte toutes comme refusées, ce qui est exactement le rôle d'une
route en maintenance.

Une route **se déplie sur ses endpoints**, dans le même tableau et sur la même période :
les lignes s'additionnent donc. L'unité est le gabarit (`/orders/{id}`), jamais le chemin
brut, qui donnerait une série par commande.

Une ligne d'endpoint marquée **deduced** a été déduite de la forme des chemins que cette
gateway a vus passer, et non d'une spécification : les segments qui ressemblaient à
des identifiants ont été regroupés. La déduction peut être fausse, car une année
ressemble à un identifiant. Pour obtenir des noms exacts, déclarez une spécification
OpenAPI sur la [route](/docs/console/routes).

Les courbes sont tracées même lorsqu'elles sont vides : *rien n'a encore été mesuré* et
*l'écran n'est pas connecté* sont deux réponses que vous devez pouvoir distinguer.

> [!NOTE]
> La fenêtre est conservée en mémoire et repart de zéro au redémarrage de la gateway.
> Dans un cluster, les échantillons sont additionnés sur tous les nœuds, et un endpoint
> est compté sur le nœud qui a répondu - l'écran le précise là où c'est utile.

## Conserver un historique

Les compteurs ne quittent la gateway que d'une seule manière : ils sont poussés en
OTLP vers un collecteur, depuis **Infra, OpenTelemetry**, onglet **Metrics**, et c'est
le collecteur qui les écrit dans Prometheus. Voir
[les métriques](/docs/operations/metrics).

> [!NOTE]
> Édition Enterprise : l'export des compteurs. Les compteurs et cet écran existent dans
> les deux éditions - des courbes sans rien installer, c'est la promesse de l'image
> Community ; ce qui est vendu, c'est leur envoi vers une pile d'outils que vous
> exploitez déjà.

## Pièges

- **Les compteurs sont propres à chaque nœud.** Chaque nœud pousse les siens, sous son
  propre `service.instance.id`, et c'est un tableau de bord qui les additionne.
- **Un redémarrage remet les courbes à zéro.** L'historique se trouve dans Prometheus,
  à condition que les compteurs y soient poussés.
- **Une route que personne n'a appelée n'a rien à afficher**, et ses endpoints non
  plus. Ce n'est pas une panne.
- **Les gabarits déduits peuvent être faux.** Considérez-les comme une indication tant
  qu'aucune spécification n'est déclarée.
