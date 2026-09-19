---
title: Contrôle d'accès
section: Concepts
order: 23
summary: Rôles, groupes et organisations, et comment la règle d'une route les lit pour décider si une requête passe.
---

# Contrôle d'accès

Savoir qui appelle est une question ; savoir s'il a le droit de passer en est une
autre. Meerkat répond à la seconde avec quatre pièces : un catalogue de rôles, des
groupes qui distribuent les rôles, des organisations qui en délimitent la portée,
et une règle sur chaque route qui lit le résultat.

## Les rôles

Un rôle est un nom dans un **catalogue unique, à l'échelle de la gateway**. Les
rôles forment une hiérarchie : un rôle parent implique ses enfants, donc accorder
`finance-manager` accorde tout ce qui est en dessous sans avoir à l'énumérer.

Les rôles portent des étiquettes, qui servent au classement et non à la
permission. Les rôles marqués système ne peuvent pas être supprimés ; supprimer un
rôle ordinaire rattache ses enfants à son propre parent plutôt que de les laisser
orphelins.

Le catalogue est global à dessein. Ce qui diffère d'une organisation à l'autre,
c'est qui détient quoi - pas le sens des mots.

## Les groupes

Un groupe appartient à une **organisation** et regroupe des rôles du catalogue. Les
personnes reçoivent des groupes par organisation, et c'est ce qui permet au même
compte d'être administrateur dans l'une et lecteur dans l'autre.

Chaque organisation choisit comment ses groupes s'additionnent :

| Mode | Sens |
|---|---|
| `MULTIPLE` | tous les groupes affectés au membre comptent, et leurs rôles se cumulent |
| `SINGLE` | un groupe à la fois, choisi à la connexion ; les autres ne s'appliquent pas avant une nouvelle connexion |

`MULTIPLE` est le défaut. En mode `SINGLE`, une session sans groupe choisi n'a
aucun rôle.

## Les organisations

Une organisation - un tenant - regroupe des personnes et les groupes dans lesquels
elles sont. Une appartenance est typée `ADMIN` ou `USER`. La propriété est un champ
distinct sur l'organisation, toujours renseigné, transmissible, et indépendant de
l'appartenance : un propriétaire n'a pas besoin d'être membre.

Un membre porte ses propres surcharges par-dessus l'organisation : s'il est actif,
sa fenêtre d'accès, sa durée de session.

> [!NOTE]
> Édition Enterprise, pour plus d'une organisation. Une installation
> communautaire en sert exactement une, que la console ne nomme jamais - les
> écrans agissent simplement dessus.

## La chaîne d'héritage

Deux réglages se résolvent **membre, puis organisation, puis installation**, le
plus spécifique gagne :

- la durée de session ;
- la fenêtre d'accès métier (jours, heures, fuseau).

> [!WARNING]
> Cette chaîne fait exactement deux réglages de long. Le second facteur n'y est
> **pas** - il se résout depuis le compte, puis l'installation, sans niveau
> organisation, parce que l'étape MFA s'exécute avant qu'une organisation soit
> choisie. Une règle par organisation n'aurait encore rien à lire.

Une troisième chaîne, distincte, existe par autorité d'authentification, pour le
MFA, les passkeys et la création automatique de compte.

Les heures ouvrées sont une capacité Enterprise, et partiellement écrite : il n'y a
pas de revérification en cours de session, et la fenêtre par membre n'a pas
d'écran dans la console.

## Les capacités du compte

Certains droits ne sont pas des rôles : ce sont des drapeaux sur le compte, parce
qu'ils portent sur l'administration de la gateway et non sur l'usage d'une
application.

| Drapeau | Ce qu'il ouvre |
|---|---|
| `root` | l'administration globale ; implique les deux suivants, et c'est le seul drapeau qui peut émettre des jetons du plan de contrôle |
| `infraAdmin` | le côté routage : routes, TLS, autorités, pages intégrées |
| `appAdmin` | le côté application : comptes, rôles, réglages globaux |
| `dev` | l'outillage développeur, là où le mode développeur est ouvert |
| `tenantCreator` | le droit de créer des organisations |

Administrer une organisation n'est délibérément **pas** un drapeau : c'est en être
membre `ADMIN`, ou en être propriétaire.

## La règle posée sur une route

Chaque route porte une règle, et elle se lit sur **deux axes combinés en ET**.

Le premier axe est l'appartenance exigée :

| Niveau | Passe |
|---|---|
| *(vide)* | tout le monde - la route ne demande rien et l'amont décide |
| `auth` | tout compte connecté |
| `tenant` | un compte connecté avec une organisation active |
| `tenants` | ...et cette organisation est l'une de celles nommées |
| `deny` | personne |

Le second axe est l'ensemble des rôles que l'appelant détient **dans son
organisation active**.

Ainsi `roles: [admin]` seul veut dire administrateur de n'importe quelle
organisation, alors que `tenants: [acme]` plus `roles: [admin]` veut dire
administrateur d'Acme. Une liste de comptes nommés peut s'ajouter, et elle est lue
d'abord : une personne nommée passe quoi que demande le niveau, `deny` compris.

## À quoi ressemble un refus

La gateway répond dans l'ordre de ce que la personne peut vraiment y faire :

| Situation | Réponse |
|---|---|
| Pas de session, et c'est une navigation | la page de connexion, avec la destination visée |
| Pas de session, et c'est un appel d'API | 401 avec `WWW-Authenticate: Session` |
| Une étape de connexion encore due | cette étape |
| Changer d'organisation aiderait | le sélecteur d'organisation, en disant pourquoi |
| Aucune appartenance du tout | la salle d'attente |
| Tout autre cas, sur une route UI | `/refused`, qui nomme la règle qui a écarté et offre ce que cette session peut ouvrir |
| Tout autre cas, sur une route de service | un 403 sec - personne ne lit un 403 dans un navigateur |

Chaque refus d'un appelant connecté écrit une ligne d'avertissement portant le
même code de motif que la page affiche : ce qu'une personne rapporte et ce que dit
le journal sont donc la même chose.

## Les règles par endpoint

Une route qui expose une spec OpenAPI peut porter des règles par **opération** -
méthode plus chemin, avec les gabarits `{var}` et `*` pour n'importe quelle
méthode - posées sur l'inventaire d'endpoints que la gateway a lu dans cette spec.
La première surcharge qui correspond gagne.

> [!NOTE]
> Une opération sans surcharge retombe sur la règle de la **route**. Il n'y a pas
> encore d'option deny-by-default : une spec qui a gagné une opération pour
> laquelle personne n'a écrit de règle est couverte par la route, pas refusée.

## Ce qui n'est pas écrit

- **L'impersonation** - se connecter en tant que quelqu'un d'autre pour reproduire ce qu'il voit. Rien n'existe : aucun bandeau, aucune double identité à l'audit, aucun endpoint.
- **Un interrupteur RBAC global** - un mode où tout compte connecté passe. L'équivalent s'écrit route par route, ce qui est un confort et non un mécanisme.
