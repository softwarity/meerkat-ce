---
title: Routes
section: Concepts
order: 21
summary: Une route répond à deux questions - cette requête est-elle pour moi, et qu'en fais-je - et tout dépend de l'ordre dans lequel les routes sont essayées.
---

# Routes

La route est l'unité de configuration de Meerkat. Elle porte tout : les
requêtes qu'elle veut, ce qu'elle leur fait, où elles vont, qui peut passer, le
temps accordé à l'upstream, le volume qu'il peut transporter. Il n'y a pas d'objet
service à créer au préalable : une route dotée d'une URL d'upstream est une route
complète.

Les routes vivent dans la base de données et se modifient depuis la console.
Une route enregistrée s'applique aussitôt.

![L'écran Routes](img/console/routes-list.webp)

## Les quatre parties

**Les prédicats** répondent à la question *cette requête est-elle pour moi ?*.
C'est la moitié qui reconnaît les requêtes, et ils se combinent par un **ET** :
une route qui porte un prédicat de chemin et un prédicat d'hôte exige les deux.

**Les filtres** transforment. Un filtre de requête réécrit ce qui part vers
l'upstream, un filtre de réponse réécrit ce qui en revient, un filtre de garde
(gate) refuse purement et simplement une requête, et un filtre terminal répond
à la place de l'upstream.

**L'upstream** est la destination de la requête : `http://`, `https://` ou
`h2c://` pour HTTP/2 en clair, le protocole que parle un service gRPC à
l'intérieur d'un cluster.

**L'accès** est la règle de la route : qui peut passer. Une règle vide signifie
que la route ne demande rien et que l'upstream décide seul.

## Ce qu'une route peut reconnaître

| Prédicat | Porte sur |
|---|---|
| `path` | le chemin de la requête, avec les motifs `*` et `**` |
| `host` | l'en-tête Host |
| `method` | le verbe HTTP |
| `header`, `cookie`, `query` | une valeur nommée, présente ou égale à une valeur donnée |
| `remote-addr` | l'adresse du pair, par CIDR |
| `x-forwarded-remote-addr` | la même chose, lue dans l'en-tête de transfert |
| `time-window` | une période, pour une route qui n'existe qu'à certaines heures |
| `version` | une plage de versions d'API, lue dans un en-tête, un paramètre de requête ou le chemin |
| `weight` | une part du trafic, tirée au sort à chaque requête, pour un déploiement canari |

> [!NOTE]
> Un prédicat de langue, l'exclusion par expression régulière et une option de
> barre oblique finale sont décrits mais pas encore réalisés. La liste
> ci-dessus correspond à ce qui existe.

## Ce qu'une route peut faire à une requête

Les filtres s'exécutent par phases, et la console les regroupe dans l'ordre où
ils s'exécutent :

- **Gates** refuse avant toute autre chose : `max-request-body`, `max-request-headers`.
- **Incoming** réécrit la requête : `strip-prefix`, `prefix-path`, `rewrite-path`, `set-path`, `set-host`, `preserve-host`, et toute la famille des opérations sur les en-têtes, les paramètres de requête et les cookies - définir, ajouter, supprimer, renommer, copier, réécrire.
- **Outgoing** réécrit la réponse : la même famille pour les en-têtes, plus `set-status`, `cache-control`, `security-headers`, `cookie-attributes`, `dedupe-response-header`, `rewrite-location`, `remove-json-fields`.
- Avec un filtre **Terminal**, la route répond d'elle-même : `redirect`, `maintenance`, `respond`.

Une route qui porte un filtre terminal ne relaie rien : ses filtres entrants et
la transmission de l'identité sont donc abandonnés. La console désactive ces
sections plutôt que de vous laisser saisir des réglages qui ne serviraient à
rien.

## L'ordre, et pourquoi tout en dépend

Les routes sont essayées par **ordre croissant**, et c'est **la première qui
reconnaît la requête qui répond**. À ordre égal, le nom départage : une même
configuration se compile donc toujours en une même table.

Deux aspects de cet ordre surprennent, et tous deux sont voulus.

**La sécurité fait partie de la sélection.** Deux routes peuvent couvrir les
mêmes chemins et ne différer que par leur public. Un appelant que la règle
écarte **retombe sur la route suivante qui reconnaît la requête**. C'est ce qui
permet de placer une route `/reports/**` réservée aux rôles de la finance
au-dessus d'une route `/reports/**` ouverte à tous les autres.

**Une règle `deny` ne laisse jamais retomber.** La seule règle écrite pour
fermer un chemin le ferme. Tout autre comportement en ferait une redirection.

Si toutes les routes qui reconnaissent la requête ont écarté l'appelant, c'est
la **première** d'entre elles qui produit le refus - avec le motif qu'elle
connaît, et le changement d'organisation qu'elle peut proposer. Un 404 sec
serait ici la seule réponse dont personne ne peut rien faire.

Si aucune route n'a reconnu la requête, la gateway répond 404 et le compte à
l'échelle de la gateway : un nombre croissant de requêtes non reconnues
trahit une erreur de configuration que quelqu'un doit voir.

## La route attrape-tout

Une route attrape-tout n'est pas un réglage : c'est une route ordinaire dont le
prédicat de chemin vaut `/**` et qui est placée en dernier. Elle n'a aucun
privilège : elle se trouve simplement essayée après toutes les autres, et
récupère donc tout ce qui n'a pas été reconnu, `/` compris.

Une installation neuve n'en a pas : un chemin non reconnu répond 404. Ajoutez-en
une quand vous voulez que ces chemins mènent quelque part - à votre propre page
d'accueil, par exemple.

> [!TIP]
> Une nouvelle route est créée avec l'ordre `0`, ce qui la place **en tête** de
> la table : elle est essayée avant toutes celles qui existent déjà. Pour
> changer cela, faites glisser les lignes sur l'écran Routes.

## Deux types de route

Toute route est une route de service. **UI** est un indicateur qui s'y ajoute et
qui ouvre les options n'ayant de sens que pour ce qu'un navigateur affiche : le
bouton utilisateur injecté, le mode clair ou sombre, l'identité inscrite dans
la page. Sur une route UI, un refus mène à une page ; sur une route de service,
il reste un 403, parce que personne ne lit un 403 dans un navigateur.

Voir [Ce que la gateway injecte](/docs/concepts/data-plane-chrome).

## Ce qu'une route porte encore

| | Quoi, et d'où vient la valeur à défaut |
|---|---|
| Timeouts | connexion et première réponse, par route, sinon ceux de l'installation, sinon 5 s / 15 s. Le corps n'est jamais borné : un téléchargement ou un websocket qui a commencé dure aussi longtemps que nécessaire |
| Disjoncteur | désactivé par défaut. Après N échecs consécutifs, la route cesse d'appeler l'upstream et sert la page d'indisponibilité, puis laisse passer une requête une fois le temps de repos écoulé. Un 500 ne compte pas : un service qui répond 500 fonctionne, et il a un bug |
| Rate limits | plusieurs à la fois, chacun indexé sur un critère différent - la route, un utilisateur, un jeton, une organisation, une adresse. Le premier dépassé répond |
| Transmission de l'identité | ce que l'upstream apprend de l'appelant - par des en-têtes, ou par un JWT signé |
| Langues | les langues que propose la route, et la manière dont la langue est transmise à l'upstream |
| Spec OpenAPI | publiée par le service ou déposée ici ; c'est sur elle que se posent les règles par endpoint |

## Tester avant le trafic

Le **Routing test** de l'écran Routes compose une requête qui n'est jamais
envoyée - méthode, chemin, hôte, en-têtes, cookies, adresse du client, heure,
et l'identité sous laquelle elle sera jugée - et indique, route par route,
laquelle la prend. Il donne les quatre réponses qui comptent : la route prend
cette requête, les prédicats la refusent, la requête est reconnue mais la règle
écarte cet appelant, ou la route n'est pas évaluée parce qu'une autre, placée
plus haut, a répondu avant elle.
