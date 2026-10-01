---
title: Metrics
section: La console
order: 182
summary: Ce que la passerelle a réellement servi - cinq chiffres, deux courbes, et un classement des routes jusqu'à leurs endpoints.
---

# Metrics

L'entrée **Metrics** du rail, c'est ce que la passerelle a vraiment servi. Tous les autres
écrans montrent ce qui est **configuré**, et dans une configuration une route qui échoue et
une route que personne n'appelle se ressemblent.

Elle vit sur `/traffic`, parce que `/metrics` sur ce port est l'exposition Prometheus. Root
et infra admins seulement : les chiffres nomment chaque route et chaque service, ce qui est
une carte de l'installation.

![L'écran Metrics : quatre chiffres sur la dernière minute, les courbes de trafic et de latence, et le classement des routes](img/console/traffic.webp)

Requêtes par seconde, part de refusé ou échoué, réponse moyenne, p95 et vol en cours ; en
dessous les deux courbes, et le classement avec ses trois onglets - celui des échecs
portant son compteur.

## Ce qu'il y a sur la page

- **Over the last minute** : requêtes par seconde, part refusée ou échouée, réponse
  moyenne, le **p95** - le temps sous lequel sont passées 95 % des réponses, que la moyenne
  cache : quatre-vingt-dix réponses rapides et dix de trois secondes donnent une moyenne que
  personne n'a attendue - et combien de requêtes sont en vol.
- **Traffic** et **How long an answer takes** : deux courbes, la seconde traçant la moyenne et
  le p95 côte à côte, alimentées par la passerelle
  qui pousse un intervalle toutes les cinq secondes. Rien n'est interrogé en boucle.
- **Routes**, classées sur l'un de trois axes - **slowest**, **failing**, **costliest** -
  sur la fenêtre que couvrent les échantillons. L'onglet des échecs porte son compte, pour
  qu'un oeil soit attiré sans avoir à basculer pour savoir s'il y a de quoi basculer.

![Le classement des routes, plus bas : cinq routes avec leurs requêtes, leurs échecs, leur réponse moyenne et le temps passé](img/console/metrics.webp)

Plus bas sur le même écran : les cinq routes sur deux minutes. *Inventory (maintenance)*
répond à chaque requête en 0,4 ms et les compte toutes comme refusées - ce que fait
exactement une route en maintenance.

Une route **s'ouvre sur ses endpoints**, dans le même tableau et sur la même période, donc
les lignes s'additionnent. L'unité est le gabarit (`/orders/{id}`), jamais le chemin brut :
un chemin brut serait une série par commande.

Une ligne d'endpoint marquée **deduced** a été déduite de la forme des chemins que cette
passerelle a vus, pas d'une spec - les segments qui ressemblaient à des identifiants ont été
repliés. Cela peut se tromper, puisqu'une année ressemble à un id. Déclarez une spec OpenAPI
sur la [route](/docs/console/routes) pour des noms exacts.

Les courbes sont dessinées même vides : *rien de mesuré* et *pas connecté* sont deux
réponses que le lecteur doit pouvoir distinguer.

> [!NOTE]
> La fenêtre est tenue en mémoire et repart au redémarrage de la passerelle. En cluster, les
> échantillons sont sommés sur tous les noeuds, et un endpoint est compté sur celui qui a
> répondu - l'écran le dit là où cela compte.

## Metrics endpoint

Le bouton dans l'en-tête porte l'état de l'endpoint de scrape sur son visage - exposé ou non - et
mène là où il se configure : **Infra, Metrics endpoint**, à côté d'OpenTelemetry. L'interrupteur,
le port, le jeton optionnel et les fichiers à écrire y sont, et les mêmes compteurs peuvent aussi
être poussés en OTLP depuis **Infra, OpenTelemetry**. Voir [les métriques](/docs/operations/metrics).

> [!NOTE]
> Edition Enterprise : sortir les compteurs, scrapés ou poussés. Les compteurs et cet écran sont
> dans les deux éditions - des courbes sans rien installer est ce que promet l'image
> communautaire ; ce qui se vend, c'est de les externaliser dans une stack que vous exploitez
> déjà.

## Pièges

- **Prometheus scrape chaque noeud, pas le service devant eux.** Les compteurs sont par
  noeud, donc un répartiteur donnerait un noeud différent à chaque passage et dessinerait une
  courbe qui n'est celle de personne.
- **Un redémarrage remet les courbes à zéro**, pas les compteurs que lit Prometheus.
- **Une route que personne n'a appelée n'a rien à dire**, ni ses endpoints. Ce n'est pas une
  panne.
- **Les gabarits déduits peuvent se tromper.** Traitez-les comme une indication tant qu'aucune
  spec n'est déclarée.
