---
title: Rate limits
section: Exploitation
order: 221
summary: Comment s'écrit une borne, ce qu'un refus contient, et pourquoi le chiffre affiché à l'écran change de sens en cluster.
---

# Rate limits

Le mot juste est **par**, pas **pour**. Personne n'écrit une limite *pour* alice : une
liste de noms, personne ne la tient à jour, et elle répond à la mauvaise question. Une
règle dit sur quoi le compteur est **indexé**, et les budgets se créent tout seuls - un
par appelant.

## Les deux moitiés d'une règle

| Moitié | Ce qu'elle dit |
|---|---|
| **par** | sur quoi le compteur est indexé : la route entière, un utilisateur, un jeton d'API, une organisation ou une adresse cliente |
| **s'applique à** | qui la règle concerne, exprimé par un `Access` - le vocabulaire que les règles d'accès emploient déjà. Vide signifie tout le monde. |

C'est ce qui fait d'un rôle un palier tarifaire sans inventer de nouveau concept :
*mille par minute et par utilisateur, pour quiconque détient `partner`* et *soixante par
minute et par utilisateur, pour quiconque détient `trial`* sont deux lignes, et aucune ne
nomme qui que ce soit.

Une fenêtre est une durée ISO-8601 (`PT1M`, `PT1H`), comprise entre une seconde et un
jour. En dessous d'une seconde, une fenêtre glissante ne mesure plus que de la gigue ;
au-delà d'un jour, il s'agit d'un quota, qui doit survivre à un redémarrage, et ces
compteurs n'y survivent pas.

## Plusieurs bornes s'appliquent à la fois

C'est ici qu'une borne se distingue d'une règle d'accès, où la première règle qui
correspond l'emporte. Les limites sont des bornes, et vous les voulez toutes :

- cinq mille par minute pour la route entière protègent le service ;
- cent par minute et par utilisateur empêchent un appelant de tout accaparer ;
- soixante par minute et par adresse couvrent ceux qui n'ont pas de compte.

La **première borne dépassée** refuse la requête.

> [!WARNING]
> Une règle restreinte avec *only for* borne les appelants qu'elle décrit et **personne
> d'autre**. Trois règles étroites donnent une route qui paraît bornée et qui reste grande
> ouverte à tous ceux qu'aucune des trois ne décrit - précisément le cas que l'on croit
> couvert. La console le signale quand aucune borne ne s'applique à tout le monde.

## Où une borne est vérifiée

Les bornes qui se passent d'identité - une pour la route entière, une par adresse - sont
vérifiées **en tout premier** : avant de chercher une session, avant de lire un corps,
avant de contacter un upstream. Refuser tôt, c'est toute la raison d'être d'une borne.

Les bornes indexées sur un utilisateur, un jeton ou une organisation ne peuvent pas
l'être : elles coûtent une résolution de session, que seules paient les routes qui en
demandent une. Une règle *restreinte* à certains appelants a elle aussi besoin d'une
identité, quel que soit son critère de comptage.

## Ce que contient un refus

Un `429`, accompagné de `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset` et
`Retry-After`. Sans ces en-têtes, un refus est une porte sans écriteau : un client qui ne
peut pas lire quand revenir abandonne ou s'acharne, et dans les deux cas le service que
la borne devait protéger y perd. `Retry-After` ne vaut jamais zéro - ce serait une
invitation.

Une requête refusée n'est **pas comptée**. Un refus qui incrémenterait quand même le
compteur transformerait une rafale en un blocage qui lui survit : l'appelant temporise,
et le compteur qui l'y oblige continue de se remplir de ses propres refus.

## La fenêtre glisse

Un compteur qui repart de zéro à intervalle fixe répond mal à la question "cent sur la
dernière minute" à chaque changement de fenêtre : deux cents requêtes passent en deux
secondes, cent de chaque côté de la remise à zéro. Ce n'est pas une approximation, c'est
une erreur. Le calcul retenu est donc l'estimation classique à deux fenêtres - le compte
de la fenêtre courante, plus celui de la précédente pondéré par l'avancement dans la
fenêtre courante. Elle coûte deux entiers par clé et l'écart reste inférieur à un pour
cent.

## Le nombre de clés est plafonné

Une limite indexée par adresse crée un compteur par adresse, et l'ensemble des adresses
est choisi par celui qui envoie les requêtes : une inondation ferait grossir la mémoire
de la gateway par le mécanisme même qui doit lui permettre d'y résister.

Chaque règle suit donc au plus dix mille appelants distincts, les clés expirées sont
purgées en premier, et tous les appelants restants **partagent un seul compteur**.
Refuser d'emblée ceux qui ne sont pas suivis permettrait à n'importe qui de provoquer un
déni de service en changeant d'adresse ; les laisser passer sans borne permettrait à
n'importe qui de contourner la borne de la même manière.

## L'effet d'un rechargement

Les compteurs sont créés à la compilation d'une route : **enregistrer une route les
remet donc à zéro**, et un appelant en milieu de fenêtre repart de zéro. C'est le
compromis assumé d'une conception sans persistance : conserver les compteurs d'un
rechargement à l'autre obligerait à les indexer sur l'identité de la règle, et une règle
n'a pas d'identité - en passant de *cent par minute* à *deux cents par minute*, elle
hériterait du compte d'une borne qui n'existe plus.

## Par endpoint

Une opération de l'inventaire OpenAPI d'une route peut porter ses propres bornes, qui
s'ajoutent à celles de la route (QUOTA-05). Elles se choisissent opération par
opération, jamais comme valeur par défaut pour tout l'inventaire : une borne, c'est un
compteur par opération **et** par appelant, si bien qu'une spécification de deux cents
opérations avec dix mille clés représenterait deux millions de compteurs - la leçon sur
la cardinalité, un niveau plus bas.

L'écran se trouve dans **Infra > Endpoint rate limits**.

![Endpoint rate limits : trois opérations dotées de leurs propres bornes, par utilisateur ou pour l'opération entière](img/console/endpoint-limits.webp)

## En cluster : lisez le chiffre deux fois

> [!WARNING]
> Ces compteurs vivent **en mémoire, sur chaque nœud**. Avec une borne de cent par minute
> sur quatre nœuds, l'installation laisse passer jusqu'à quatre cents requêtes par
> minute. La console le rappelle à l'endroit où vous saisissez le nombre : un chiffre qui
> n'a pas le même sens en cluster que sur une instance seule doit le dire là où il
> s'écrit.

C'est un choix, pas un oubli : un compteur partagé exact coûte un aller-retour vers la
base de données à chaque requête, un prix qu'une gateway ne peut pas payer sur le
chemin de tout le trafic. Cette moitié est la moitié **protectrice** - approximative,
sans coût, et suffisante contre les abus.

Un compteur se trouve *bel et bien* en base, et il est exact : le **compteur
anti-force brute** des connexions. Cinq tentatives, c'est cinq pour toute l'installation
et non cinq par nœud, et un redémarrage ne rouvre plus la porte à un attaquant en cours
de limitation.

## Ce qui manque

- **Le ralentissement.** Dépasser une borne bloque la requête ; cela ne la ralentit pas
 (QUOTA-02).
- **Les compteurs facturables.** La moitié que l'on facture ou que l'on montre à un
 client relève d'un autre mécanisme, écrit par lots, qui n'existe pas encore (QUOTA-03) -
 et avec lui l'écran de consommation et ses seuils d'alerte.
- **Les compteurs partagés.** Le comptage exact à l'échelle d'un cluster attend les
 compteurs facturables (QUOTA-04). La voie est tracée : le compteur des connexions a
 exactement cette forme.
