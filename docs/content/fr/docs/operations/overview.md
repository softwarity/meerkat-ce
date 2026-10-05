---
title: Exploitation
section: Exploitation
order: 200
summary: Où regarder quand quelque chose ne va pas, ce que la gateway enregistre, et ce que contient réellement une sauvegarde.
---

# Exploitation

Meerkat se trouve sur le chemin de chaque requête. Les questions qu'un exploitant lui pose
sont donc toujours les mêmes : le trafic arrive-t-il, passe-t-il, et qui a modifié quelque
chose. À chacune correspond un écran, et cette page vous dit lequel.

## Où regarder

| La question | Où trouver la réponse |
|---|---|
| Le trafic arrive-t-il, et à quel rythme | [L'écran de trafic](/docs/operations/traffic), l'entrée **Metrics** du rail |
| Ce service répond-il encore | [La santé des upstreams](/docs/operations/upstream-health), et la liste Routes signale ce qui échoue |
| Pourquoi cet appel a-t-il été refusé avec un `429` | [Les rate limits](/docs/operations/rate-limits) |
| Qui a modifié cela, et quand | [Le journal d'audit](/docs/operations/audit), qui peut aussi être envoyé à un collecteur |
| Qui a appelé quoi, et avec quel résultat | [Les journaux](/docs/operations/logs) |
| Où sont passées les secondes de CETTE requête | [Les traces](/docs/operations/tracing) |
| L'appel planifié a-t-il eu lieu, et comment s'est-il passé | [Les appels planifiés](/docs/operations/scheduler), l'entrée **Scheduler** du rail |
| Comment les traces, les métriques, l'audit et les journaux parviennent à mon collecteur | [L'export OpenTelemetry](/docs/operations/tracing#exporter-vers-votre-collecteur), dans **Infra, OpenTelemetry** |
| Ce nœud est-il prêt à recevoir du trafic | [Les health checks](/docs/operations/health) |
| Quand ce certificat expire-t-il | [TLS et certificats](/docs/operations/tls) |
| Que contient réellement une sauvegarde | [Sauvegarde et restauration](/docs/operations/backup-restore) |
| Où ce mot de passe est-il rangé | [Le coffre](/docs/operations/vault) |

## Ce que la gateway enregistre, et pour combien de temps

| Quoi | Où | Combien de temps |
|---|---|---|
| Les compteurs de trafic | en mémoire, sur chaque nœud | une heure, perdue au redémarrage |
| L'historique par endpoint | en mémoire, sur chaque nœud | soixante-dix minutes, un point par minute |
| Le journal d'audit | dans la base de données | un an par défaut, durée choisie par root (de trois mois à cinq ans), puis purge |
| Les points de reprise | dans la base de données | conservés, jamais élagués |
| Le journal opérationnel | sur la sortie d'erreur, structuré | selon l'outil qui le collecte |
| Le journal d'accès | sur la sortie standard, structuré, désactivé par défaut | selon l'outil qui le collecte |
| Les traces | rien ici : elles sont exportées en OTLP | dans le collecteur, quelques jours |

Rien de ce qui concerne le trafic n'est écrit sur disque, et c'est voulu : une app-gateway
n'est pas une base de séries temporelles. Une installation qui veut un an de courbes pousse
les compteurs en OTLP vers un collecteur, qui les écrit dans la base qu'elle exploite déjà
([métriques](/docs/operations/metrics)).

Les [journaux](/docs/operations/logs) sont structurés. Le niveau se règle au démarrage, et la
console peut le changer à chaud. Un journal d'accès - une ligne par requête qui franchit la
porte d'entrée - s'active sur demande. Il porte le même identifiant que
[les traces](/docs/operations/tracing) : c'est ce qui relie une ligne de la gateway à
l'audit métier d'un service.

## Une sauvegarde et un export de configuration ne sont pas la même chose

Un **snapshot**, c'est la base de données entière : les comptes, les sessions, le coffre,
le journal d'audit, les certificats. C'est lui qui restaure *cette* installation.

Un **export de configuration**, ce sont les routes, les rôles, les autorités, les thèmes et
les réglages, dans un fichier qu'une personne peut lire et comparer. Il reproduit une
gateway *ailleurs*. Il ne contient aucun compte, aucun certificat et aucune valeur
secrète : ce n'est donc pas une sauvegarde.

## Ce qui est propre à chaque nœud et ce qui est partagé

| Conservé par chaque nœud | Partagé par la base de données |
|---|---|
| les compteurs de trafic (additionnés pour l'écran) | le journal d'audit et les points de reprise |
| le verdict du disjoncteur sur un upstream | les routes, les certificats, les réglages, le coffre |
| les compteurs des rate limits | le compteur de tentatives de connexion contre la force brute |
| les abonnés du canal temps réel (les messages, eux, sont relayés) | les sessions et les jetons d'API |

> [!NOTE] Édition Enterprise
> Faire tourner plusieurs gateways sur une même base PostgreSQL est réservé à l'image
> Enterprise : c'est elle qui contient le pilote qui ouvre la connexion et attend les
> notifications. L'image Community vous dit ce qu'elle sait faire, au lieu d'échouer sur un
> pilote.

> [!WARNING]
> La clé maîtresse du coffre est un **fichier** du répertoire de données, et elle le reste
> même avec une base de données externe. Tous les nœuds doivent avoir la même clé, sinon un
> nœud ne peut pas ouvrir ce qu'un autre a scellé - certificats compris. Voir
> [le coffre](/docs/operations/vault).
