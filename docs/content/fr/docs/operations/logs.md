---
title: Journaux
section: Exploitation
order: 211
summary: Une ligne par requête qui franchit la porte d'entrée, avec le compte tel que la gateway l'a authentifié.
---

# Journaux

Il y a deux journaux, et ils ne se lisent pas au même rythme.

```
journal opérationnel    démarrage, rechargements, upstream qui tombe
                        sur la sortie d'erreur, quelques milliers de lignes/jour

journal d'accès         une ligne par requête, refus compris
                        sur la sortie standard, 35 millions/jour à 400 req/s
```

Un seul flux transporte les deux, et chaque ligne est un objet typé : un collecteur les
aiguille d'après le champ `type`, vers deux destinations et avec deux durées de conservation.

## Le journal opérationnel

```
MEERKAT_LOG_LEVEL    debug | info | warn | error      info par défaut
MEERKAT_LOG_FORMAT   json | text                      json en production, text ailleurs
```

Une faute de frappe dans le niveau ramène à `info` au lieu d'empêcher le démarrage.

### Dans la console

![L'écran Logs : le niveau en haut à droite, les filtres par niveau et la recherche, puis les lignes, dont une ouverte](img/console/logs.webp)

**Logs**, dans le rail sous Audit (root et infra admin), affiche en direct les 5 000
dernières lignes. La gateway les conserve sous forme structurée : l'écran les présente
donc de la même façon quel que soit le format de sortie - heure, niveau, message, attributs.
Cliquez sur une ligne pour l'ouvrir. Vous pouvez filtrer par niveau, chercher dans le texte,
et exporter ce qui est affiché au format `.jsonl`. Dès que vous remontez dans la liste,
l'écran cesse de suivre les nouvelles lignes ; la flèche relance le suivi.

**Le niveau**, en haut à droite, s'applique immédiatement, sur tous les nœuds. Un niveau plus
bavard que celui du démarrage revient de lui-même à sa valeur initiale au bout de 30 minutes,
et l'écran affiche le compte à rebours. Ce niveau n'est pas enregistré : après un
redémarrage, c'est de nouveau `MEERKAT_LOG_LEVEL` qui s'applique.

> [!NOTE] Un seul nœud
> En cluster, les lignes sont celles du nœud qui a répondu, et l'écran indique lequel. Le
> journal du cluster entier se lit dans le collecteur.

Le journal d'accès ne figure pas sur cet écran : il en chasserait tout le reste en quelques
secondes. Pour cela, il y a [Metrics](/docs/console/traffic) et les traces.

## Le journal d'accès

Il est désactivé par défaut : à quatre cents requêtes par seconde, il représente trente-cinq
millions de lignes par jour.

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
| `type` | `access` ou `app` : le discriminant qui permet à un collecteur de séparer les deux journaux |
| `trace_id` | la **clé de jointure** avec l'audit métier de votre service |
| `path` | le chemin tel qu'il a été demandé, identifiants compris |
| `endpoint` | le gabarit, quand la route déclare une spécification - c'est ce qui permet de compter ces lignes |
| `user` | le compte **tel que cette gateway l'a authentifié**, jamais tel qu'un en-tête le prétend |
| `token` | le nom du jeton d'API quand l'appel vient d'une machine : un même compte peut en avoir plusieurs |
| `ip` | l'adresse **résolue par la gateway**, jamais un `X-Forwarded-For` écrit par l'appelant |
| `outcome` | `ok`, `refused`, `rejected`, `failed`, `upstream-down` |

`outcome` distingue `refused` de `failed` : un 403, c'est la gateway qui fait son travail ;
un 502, c'est un service qui est tombé. Si les deux se lisaient sous le même mot, le premier
serait noyé dans le second le matin où un upstream lâche.

Baisser le niveau du journal opérationnel ne fait pas taire celui-ci : réduire le bruit ne
doit pas faire disparaître la piste d'audit.

> [!NOTE] La ligne qu'aucun service ne peut écrire à votre place
> Un service dont l'accès a été refusé n'a **jamais vu l'appel**, et il ne sait de
> l'appelant que ce que la gateway lui en a dit. C'est la gateway qui a authentifié :
> le registre qui fait foi est donc celui qu'elle tient - y compris pour une application que
> plus personne ne peut modifier.

Ce qui n'y figure pas : le corps de la requête. `POST /orders/12/validate` est enregistré ;
"remise exceptionnelle de 40 %", c'est à votre service de l'écrire, sous le même `trace_id`,
dans son propre audit.

```
MEERKAT    qui, quand, d'où, avec quelle identité, sur quel endpoint,
           avec quel résultat
  |
  |  trace_id: 4bf92f35...
  v
SERVICE    ce que l'opération signifiait, sur quelles données, avec quelles valeurs
```

Rien n'est écrit dans un fichier : l'environnement d'exécution assure déjà la rotation de ce
qu'un conteneur écrit sur sa sortie standard.

> [!WARNING] Pensez à limiter les journaux de Docker
> Le pilote `json-file` n'a aucune limite par défaut, et trente-cinq millions de lignes par
> jour remplissent un disque.
>
> ```
> docker run --log-opt max-size=50m --log-opt max-file=5
> ```
>
> Kubernetes s'en charge tout seul : 10 Mi, 5 fichiers.

## Au format d'OpenTelemetry

**Infra, OpenTelemetry**, onglet **Logs**, mode **Written for an agent**. Les deux journaux
conservent leurs sorties et changent de forme, à chaud, sur tous les nœuds. Chaque ligne est
un objet, avec les noms de champs du modèle de données des logs d'OpenTelemetry :

```json
{"timestamp": "2026-10-01T16:35:37.296Z", "severity_text": "WARN", "severity_number": 13,
 "body": "the upstream is down", "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
 "attributes": {"route": "billing", "upstream.host": "billing:8080"},
 "resource": {"service.name": "meerkat", "service.version": "..."}}
```

Rien n'est poussé. Un Collector déployé en **DaemonSet** lit la sortie de chaque conteneur et
décode ces champs : une ligne arrive ainsi dans Loki avec sa sévérité, sa trace et ses
attributs. Sur la page OpenTelemetry, **No collector yet?** fournit la configuration de cet
agent. Ce mode existe dans les deux éditions : il écrit, il n'exporte pas.

## Ou poussés vers le collecteur

Le troisième mode, **Pushed to the collector**, envoie les deux journaux en OTLP à l'adresse
du collecteur (Enterprise). Il s'adresse aux nœuds sans agent : un simple hôte Docker, une
VM. Les sorties conservent le format choisi au démarrage. Un seul mode est actif à la fois :
si un agent lisait aussi les sorties, chaque ligne arriverait en double.

Cet envoi a sa propre file d'attente, distincte de celle de l'audit : un déluge de logs ne
peut pas faire perdre un événement au journal d'audit. Un lot que le collecteur refuse trois
fois est abandonné et compté, et l'onglet indique combien l'ont été.
