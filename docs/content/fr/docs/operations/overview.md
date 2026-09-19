---
title: Exploitation
section: Exploitation
order: 200
summary: Où regarder quand ça va mal, ce que la passerelle enregistre, et ce qu'une sauvegarde contient vraiment.
---

# Exploitation

Meerkat est sur le chemin de toutes les requêtes, donc les questions qu'un exploitant lui pose sont
toujours les mêmes : est-ce que le trafic arrive, est-ce qu'il passe, et qui a changé quelque chose.
Chacune a un écran, et cette page dit lequel.

## Où regarder

| La question | Où elle est répondue |
|---|---|
| Le trafic arrive-t-il, et à quel rythme | [L'écran de trafic](/docs/operations/traffic), l'entrée **Metrics** du rail |
| Ce service répond-il encore | [Santé des amonts](/docs/operations/upstream-health), et la liste des routes marque ce qui échoue |
| Pourquoi cet appel a-t-il été refusé en `429` | [Limites de débit](/docs/operations/rate-limits) |
| Qui a changé ça, et quand | [Le journal d'audit](/docs/operations/audit) |
| Ce noeud est-il prêt à prendre du trafic | [Les sondes de santé](/docs/operations/health) |
| Quand ce certificat expire-t-il | [TLS et certificats](/docs/operations/tls) |
| Ce qu'il y a vraiment dans une sauvegarde | [Sauvegarde et restauration](/docs/operations/backup-restore) |
| Où vit ce mot de passe | [Le coffre](/docs/operations/vault) |

## Ce que la passerelle enregistre, et pour combien de temps

| Quoi | Où ça vit | Combien de temps |
|---|---|---|
| Compteurs de trafic | en mémoire, sur chaque noeud | une heure, perdue au redémarrage |
| Historique par endpoint | en mémoire, sur chaque noeud | soixante-dix minutes, un point par minute |
| Journal d'audit | la base | un an, puis purge |
| Points de reprise | la base | gardés, jamais élagués |
| Journaux | sortie d'erreur, structurés | ce qui les collecte |

Rien du trafic n'est écrit sur disque, et c'est voulu : une app-gateway n'est pas une base de séries
temporelles, et une installation qui veut un an de courbes les fait aspirer dans celle qu'elle fait
déjà tourner ([métriques](/docs/operations/metrics)).

Les journaux sont structurés mais nus : ni niveau réglable, ni journal des requêtes pour l'instant
(OBS-03).

## Une sauvegarde et un export de configuration ne sont pas la même chose

Un **instantané** est toute la base : les comptes, les sessions, le coffre, le journal d'audit, les
certificats. C'est ce qui restaure *cette* installation.

Un **export de configuration** est fait des routes, des rôles, des autorités, des thèmes et des
réglages, dans un fichier qu'une personne peut lire et comparer. Il reproduit une passerelle
*ailleurs*. Il ne porte aucun compte, aucun certificat et aucune valeur secrète : ce n'est donc pas
une sauvegarde.

## Ce qui est par noeud et ce qui est partagé

| Gardé par chaque noeud | Partagé par la base |
|---|---|
| les compteurs de trafic (sommés pour l'écran) | le journal d'audit et les points de reprise |
| le verdict du disjoncteur sur un amont | les routes, les certificats, les réglages, le coffre |
| les compteurs de limite de débit | le compteur anti-force brute des connexions |
| les abonnés du canal live (les messages, eux, sont relayés) | les sessions et les jetons d'API |

> [!NOTE] Enterprise edition
> Faire tourner plusieurs passerelles contre une seule base PostgreSQL est le fait de l'image
> Enterprise : le pilote qui ouvre la connexion et attend les notifications vit là (STORE-03,
> PERF-03). L'image communautaire dit ce qu'elle sait faire plutôt que d'échouer sur un pilote.

> [!WARNING]
> La clé maîtresse du coffre est un **fichier** à côté du répertoire de données, et elle le reste
> même avec une base externe. Chaque noeud doit porter la même clé, sinon un noeud ne peut pas
> ouvrir ce qu'un autre a scellé - les certificats compris. Voir
> [le coffre](/docs/operations/vault).
