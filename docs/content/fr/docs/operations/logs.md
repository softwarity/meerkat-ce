---
title: Journaux
section: Exploitation
order: 211
summary: Une ligne par requête franchissant la porte d'entrée, avec le compte tel que la passerelle l'a authentifié.
---

# Journaux

Deux journaux, et ils ne se lisent pas au même rythme.

```
journal operationnel    demarrage, rechargements, amont qui tombe
                        sur la sortie d'erreur, quelques milliers de lignes/jour

journal d'acces         une ligne par requete, refus compris
                        sur la sortie standard, 35 millions/jour a 400 req/s
```

Un seul flux transporte les deux, et chaque ligne est un objet typé : un
collecteur les route sur le champ `type`, vers deux destinations et deux
rétentions.

## Le journal opérationnel

```
MEERKAT_LOG_LEVEL    debug | info | warn | error      info par defaut
MEERKAT_LOG_FORMAT   json | text                      json en production, texte ailleurs
```

Une faute de frappe dans le niveau retombe sur `info` plutôt que d'empêcher un
démarrage.

## Le journal d'accès

Livré éteint : à quatre cents requêtes par seconde, c'est trente-cinq millions
de lignes par jour.

```
MEERKAT_ACCESS_LOG=1
```

Une ligne, telle qu'elle sort :

```json
{"time":"2026-09-22T09:14:07Z","type":"access","trace_id":"4bf92f3577b34da6a3ce929d0e0e4736",
 "method":"GET","path":"/patients/42","endpoint":"/patients/{id}","route":"dmp-api",
 "status":200,"ms":45,"outcome":"ok","user":"dr.martin","ip":"10.0.0.7"}
```

| | |
|---|---|
| `type` | `access` ou `app` : le discriminant sur lequel un collecteur sépare les deux journaux |
| `trace_id` | la **clef de jointure** avec l'audit métier de votre service |
| `path` | le chemin tel qu'il a été demandé, identifiants compris |
| `endpoint` | le gabarit, quand la route déclare une spec - ce qui rend ces lignes comptables |
| `user` | le compte **tel que cette passerelle l'a authentifié**, jamais tel qu'un en-tête l'a prétendu |
| `ip` | l'adresse **résolue par la passerelle**, jamais un `X-Forwarded-For` écrit par l'appelant |
| `outcome` | `ok`, `refused`, `rejected`, `failed`, `upstream-down` |

`outcome` sépare `refused` de `failed` : un 403 est la passerelle qui fait son
travail, un 502 est un service tombé. Les lire sous le même mot enterre le
premier dans le second le matin où un amont lâche.

Baisser le niveau opérationnel ne fait pas taire ce journal : réduire le bruit
ne doit pas emporter la piste d'audit.

> [!NOTE] La ligne qu'aucun service ne peut écrire à votre place
> Un service refusé n'a **jamais vu l'appel**, et de l'appelant il ne sait que
> ce que la passerelle lui en a dit. C'est elle qui a authentifié, donc c'est
> chez elle que se trouve le registre opposable - y compris pour une
> application qu'on ne peut plus modifier.

Ce qui n'y est pas : le corps. `POST /orders/12/validate` est enregistré ;
« remise exceptionnelle de 40 % » est à votre service de l'écrire, sous le même
`trace_id`, dans son propre audit.

```
MEERKAT    qui, quand, depuis ou, avec quelle identite, sur quel endpoint,
           avec quel resultat
  |
  |  trace_id : 4bf92f35...
  v
SERVICE    ce que l operation voulait dire, sur quelle donnee, avec quelles valeurs
```

Rien n'est écrit dans un fichier : le runtime fait déjà tourner ce qu'un
conteneur écrit sur sa sortie standard.

> [!WARNING] Pensez à borner les journaux de Docker
> Le pilote `json-file` n'a aucune limite par défaut, et trente-cinq millions
> de lignes par jour remplissent un disque.
>
> ```
> docker --log-opt max-size=50m --log-opt max-file=5
> ```
>
> Kubernetes le fait tout seul : 10 Mi, 5 fichiers.

## Au format d'OpenTelemetry

**Infra, OpenTelemetry**, onglet **Logs**, mode **Written for an agent**. Les
deux journaux gardent leurs sorties et changent de forme, à chaud, sur chaque
noeud. Un objet par ligne, avec les noms de champs du modèle de log
d'OpenTelemetry :

```json
{"timestamp": "2026-10-01T16:35:37.296Z", "severity_text": "WARN", "severity_number": 13,
 "body": "the upstream is down", "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
 "attributes": {"route": "billing", "upstream.host": "billing:8080"},
 "resource": {"service.name": "meerkat", "service.version": "..."}}
```

Rien n'est poussé. Un Collector en **DaemonSet** lit la sortie de chaque
conteneur et décode ces champs : une ligne arrive dans Loki avec sa sévérité, sa
trace et ses attributs. **No collector yet?** sur la page OpenTelemetry donne la
configuration de cet agent. Ce mode existe dans les deux éditions : il écrit, il
n'exporte pas.

## Ou poussés au collecteur

Le troisième mode, **Pushed to the collector**, envoie les deux journaux en
OTLP à l'adresse du collecteur (Enterprise), pour les noeuds sans agent : un
hôte Docker nu, une VM. Les sorties gardent le format choisi au démarrage. Un
seul mode à la fois : avec un agent qui lit aussi les sorties, chaque ligne
arriverait deux fois.

La poussée a sa propre file, jamais celle de l'audit : un flot de logs ne peut
pas coûter un événement au journal. Un lot refusé trois fois par le collecteur
est abandonné et compté, et l'onglet dit combien.

## Ce qui manque

- Le niveau réglable depuis la console plutôt qu'au démarrage seulement.
- Le nom du jeton à côté du compte quand une machine appelle.
