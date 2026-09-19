---
title: Le journal d'audit
section: Exploitation
order: 206
summary: Chaque changement d'administration, avec son auteur et un diff champ par champ, et qui a le droit de le lire.
---

# Le journal d'audit

Toute mutation faite par le plan de contrôle est enregistrée : qui l'a faite, ce qu'elle a
touché, et - pour une modification - les champs exacts qui ont bougé, avant et après
(AUD-01).

Le journal est **append-only**. Il ne connaît que l'insertion et la purge : aucun endpoint ne peut
modifier un événement.

![Le journal d'audit](img/console/audit.webp)

## Ce qu'un événement porte

| Champ | Ce qu'il dit |
|---|---|
| quand | la seconde où c'est arrivé |
| acteur | le compte, plus le **jeton** quand un jeton a servi |
| action | `route.update`, `theme.branding`, `settings.update`, `maintenance`, `backup.snapshot`... |
| cible | le genre d'objet, son identifiant et son nom |
| organisation | quand le changement appartient à l'une d'elles |
| changements | une entrée par champ qui a bougé, avec son `from` et son `to` |
| détail | une note, pour les créations, les suppressions et les actes sans diff |

Une modification dont le diff ressort vide n'écrit **rien** : enregistrer un formulaire sans changer
une valeur n'est pas un événement.

Le diff est calculé de façon générique, en comparant l'ancien et le nouvel objet champ par champ,
donc il marche pour une route, un compte, une organisation ou la charge utile des réglages sans code
par type. Un objet imbriqué ou une liste se comparent par leur encodage entier : un changement
n'importe où dedans fait remonter le champ en entier.

## Le jeton qui a agi est nommé

Un changement fait par un agent ou par un script se lit `admin, via claude-desktop`, et non `admin`.
Le tampon est posé dans l'écriture d'audit elle-même plutôt que chez ses appelants, pour qu'aucun
endpoint ajouté plus tard ne l'oublie (MCP-03). Un journal qui nomme l'agent quatre fois sur cinq est
pire qu'un journal qui ne le nomme jamais : la cinquième se lit comme si une personne l'avait fait.

## Les secrets n'atterrissent jamais dans le journal

Un champ dont le nom contient `password`, `secret`, `token` ou `hash` est enregistré comme `***`, à
n'importe quelle profondeur d'un objet imbriqué. Une image envoyée en data URI est résumée - son genre
et sa taille - plutôt que stockée deux fois en base64 : un enregistrement de marque en porte deux,
l'avant et l'après.

Les champs qui ne disent rien d'un changement sont sautés : les identifiants, les horodatages du
serveur, et les noms d'affichage qui font l'aller-retour sans être modifiables.

## Qui voit quoi

Le journal est un écran transverse à lui, et chaque appelant lit la tranche que couvrent ses capacités
(RBAC-05) :

| Appelant | Ce qu'il lit |
|---|---|
| root | tout |
| gateway-admin | les routes, les thèmes, les jetons |
| app-admin | les comptes, les rôles, les réglages, les signalements |
| un administrateur d'organisation | les événements des organisations qu'il administre |
| n'importe qui d'autre | rien, et l'endpoint refuse plutôt que de servir une page vide |

L'outil `read_audit` de l'agent partage exactement cette fonction : un agent qui verrait un journal
plus large que la console serait un contournement du modèle de capacités, et ce ne serait la faute de
personne en particulier.

## Filtres, rétention, et ce qui manque

L'écran filtre sur l'acteur, la cible, l'identifiant de cible et un intervalle de temps.

La rétention est d'**un an**, appliquée par l'entretien périodique, et elle n'est pas encore réglable.

Pas encore là (AUD-01, AUD-02) : la pagination côté serveur, les activités de développement et de
tunnel, la rétention configurable, et l'export analytique. Les connexions ne sont pas dans ce journal
non plus - elles ont leur propre historique, sur le compte.
