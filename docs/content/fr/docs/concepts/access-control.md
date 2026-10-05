---
title: Contrôle d'accès
section: Concepts
order: 23
summary: Les rôles, les groupes et les organisations, et la manière dont la règle d'une route les lit pour décider si une requête passe.
---

# Contrôle d'accès

Savoir qui appelle est une question ; savoir s'il peut passer en est une autre.
Meerkat répond à la seconde avec quatre éléments : un catalogue de rôles, des
groupes qui distribuent ces rôles, des organisations qui délimitent les
groupes, et sur chaque route une règle qui lit le résultat.

## Les rôles

Un rôle est un nom dans un **catalogue unique, commun à toute la gateway**.
Les rôles forment une hiérarchie : un rôle parent implique ses enfants, de
sorte qu'accorder `finance-manager` accorde tout ce qui se trouve en dessous
sans avoir à l'énumérer.

Les rôles portent des étiquettes (tags), qui servent à classer et non à
autoriser. Les rôles marqués comme rôles système ne peuvent pas être
supprimés ; supprimer un rôle ordinaire fait remonter ses enfants au premier
niveau au lieu de les supprimer.

Le catalogue est global à dessein. Ce qui change d'une organisation à l'autre,
c'est qui détient quoi - pas le sens des mots.

## Les groupes

Un groupe appartient à une **organisation** et réunit des rôles du catalogue.
Les groupes sont attribués aux personnes organisation par organisation : c'est
ce qui permet à un même compte d'être administrateur dans l'une et simple
lecteur dans une autre.

Chaque organisation choisit la manière dont ses groupes s'additionnent :

| Mode | Signification |
|---|---|
| `MULTIPLE` | tous les groupes attribués au membre comptent, et leurs rôles se cumulent |
| `SINGLE` | un seul groupe à la fois, choisi à la connexion ; les autres ne s'appliquent pas tant que la personne ne se reconnecte pas |

`MULTIPLE` est la valeur par défaut. En mode `SINGLE`, une session pour
laquelle aucun groupe n'a été choisi n'a aucun rôle.

## Les organisations

Une organisation - le tenant des URL et de l'API - rassemble des personnes et
les groupes auxquels elles appartiennent. Une appartenance est de type `ADMIN` ou `USER`. La
propriété est un champ distinct de l'organisation, toujours renseigné,
transférable et indépendant de l'appartenance : le propriétaire n'est pas
nécessairement membre.

Un membre porte ses propres surcharges, qui s'ajoutent à celles de
l'organisation : son activation, sa fenêtre d'accès, la durée de vie de sa
session.

> [!NOTE]
> Édition Enterprise, au-delà d'une organisation. Une installation Community
> en sert exactement une, que la console ne nomme jamais : les écrans agissent
> simplement sur elle.

## La chaîne d'héritage

Deux réglages se résolvent dans l'ordre **membre, puis organisation, puis
installation**, le plus précis l'emportant :

- la durée de vie de la session ;
- la fenêtre d'accès métier (jours, heures, fuseau horaire).

> [!WARNING]
> Cette chaîne compte exactement deux réglages. Le second facteur n'en fait
> **pas** partie : il se résout au niveau du compte, puis de l'installation,
> sans niveau organisation, parce que l'étape MFA s'exécute avant qu'une
> organisation ait été choisie. Une règle par organisation n'aurait encore rien
> à lire.

Une troisième chaîne, distincte, existe par autorité d'authentification, pour
le MFA, les passkeys et la création automatique des comptes.

Les heures d'accès métier sont une fonctionnalité Enterprise, réalisée en
partie : il n'y a pas de nouveau contrôle en cours de session, et la console
n'a pas d'écran pour modifier la fenêtre d'un membre.

## Les capacités du compte

Certains droits ne sont pas des rôles : ce sont des indicateurs posés sur le
compte, parce qu'ils concernent l'administration de la gateway et non
l'usage d'une application.

| Indicateur | Ce qu'il ouvre |
|---|---|
| `root` | l'administration globale ; implique les deux suivants, et c'est le seul indicateur qui permet d'émettre des jetons du plan de contrôle |
| `infraAdmin` | le côté routage : routes, TLS, autorités, relais de messagerie |
| `appAdmin` | le côté application : comptes, rôles, réglages globaux, pages intégrées |
| `dev` | l'outillage développeur, là où le mode développeur est activé |
| `tenantCreator` | peut créer des organisations |

Administrer une organisation n'est volontairement **pas** un indicateur : il
faut en être membre `ADMIN`, ou en être le propriétaire.

## La règle d'une route

Chaque route porte une règle, qui se lit selon **deux axes combinés par un
ET**.

Le premier axe est le type d'appartenance exigé :

| Niveau | Qui passe |
|---|---|
| *(vide)* | tout le monde - la route ne demande rien et l'upstream décide |
| `auth` | tout compte connecté |
| `tenant` | un compte connecté qui a une organisation active |
| `tenants` | ...et cette organisation fait partie de celles qui sont nommées |
| `deny` | personne |

Le second axe, ce sont les rôles que l'appelant détient **dans son organisation
active**.

Ainsi, `roles: [admin]` seul désigne un administrateur de n'importe quelle
organisation, tandis que `tenants: [acme]` avec `roles: [admin]` désigne un
administrateur d'Acme. On peut y ajouter une liste de comptes nommés, et elle
est lue en premier : une personne nommée passe quel que soit le niveau exigé,
`deny` compris.

## À quoi ressemble un refus

La gateway répond en fonction de ce que la personne peut réellement faire,
dans cet ordre :

| Situation | Réponse |
|---|---|
| Pas de session, et il s'agit d'une navigation | la page de connexion, qui retient la destination |
| Pas de session, et il s'agit d'un appel d'API | 401 avec `WWW-Authenticate: Session` |
| Une étape de connexion reste à franchir | cette étape |
| Changer d'organisation débloquerait la situation | le choix de l'organisation, avec l'explication |
| Aucune appartenance | la salle d'attente |
| Tout autre cas, sur une route UI | `/refused`, qui nomme la règle à l'origine du refus et propose ce que cette session peut ouvrir |
| Tout autre cas, sur une route de service | un simple 403 - personne ne lit un 403 dans un navigateur |

Chaque refus opposé à un appelant connecté écrit une ligne d'avertissement
portant le même code de motif que celui affiché sur la page : ce que la
personne rapporte et ce que dit le journal sont une seule et même chose.

## Les règles par endpoint

Une route qui expose une spec OpenAPI peut porter des règles par **opération** -
une méthode et un chemin, avec des gabarits `{var}` et `*` pour toute méthode -
posées sur l'inventaire des endpoints que la gateway a lu dans cette spec.
La première règle qui correspond l'emporte.

> [!NOTE]
> Une opération sans règle propre retombe sur la règle de la **route**, sauf si
> **Only listed operations are reachable** est activé : elle est alors refusée
> à tout le monde. Sans cet interrupteur, une spec qui s'est enrichie d'une
> opération pour laquelle personne n'a écrit de règle est couverte par la
> route, et non refusée.

## Ce qui n'existe pas

- **L'emprunt d'identité** - se connecter à la place de quelqu'un pour reproduire ce qu'il voit. Rien n'existe : ni bandeau, ni double identité dans le journal d'audit, ni endpoint.
- **Un interrupteur global pour couper le RBAC** - un mode où tout compte connecté passe. L'équivalent s'écrit route par route : c'est une commodité, pas un mécanisme.
