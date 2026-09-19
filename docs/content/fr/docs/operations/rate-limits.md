---
title: Limites de débit
section: Exploitation
order: 221
summary: Comment une borne s'écrit, ce qu'un refus porte, et pourquoi le chiffre à l'écran veut dire autre chose en cluster.
---

# Limites de débit

Le mot est **par**, pas **pour**. Personne n'écrit une limite *pour* alice : une liste de noms est une
liste que personne ne maintient, et elle répond à la mauvaise question. Une règle dit sur quoi le
compteur est **clé**, et les budgets se fabriquent tout seuls - un par appelant (ROUTE-08).

## Les deux moitiés d'une règle

| Moitié | Ce qu'elle dit |
|---|---|
| **par** | sur quoi le compteur est clé : la route entière, un utilisateur, un jeton d'API, une organisation, ou une adresse client |
| **s'applique à** | à qui la règle s'adresse, écrit comme un `Access` - le même vocabulaire que les règles d'accès. Vide veut dire tout le monde. |

C'est ce qui transforme un rôle en palier tarifaire sans inventer de concept : *mille par minute par
utilisateur, pour qui porte `partner`* et *soixante par minute par utilisateur, pour qui porte `trial`*
sont deux lignes, et aucune ne nomme une personne.

Une fenêtre est une durée ISO-8601 (`PT1M`, `PT1H`), entre une seconde et un jour. En dessous d'une
seconde, une fenêtre glissante mesure de la gigue ; au-delà d'un jour, c'est un quota qui doit survivre
à un redémarrage, et ces compteurs ne le font pas.

## Plusieurs s'appliquent en même temps

C'est là qu'une borne diffère d'une règle d'accès, où la première qui matche gagne. Les limites sont
des bornes, et on les veut toutes :

- cinq mille par minute pour la route entière protègent le service ;
- cent par minute par utilisateur empêchent un appelant de tout prendre ;
- soixante par minute par adresse couvrent celui qui n'a pas de compte.

La **première borne dépassée** refuse.

> [!WARNING]
> Une règle restreinte par *only for* borne les appelants qu'elle décrit et **personne d'autre**. Trois
> règles étroites font une route qui a l'air bornée et qui est grande ouverte à tous ceux que les trois
> ne décrivent pas - exactement le cas qu'on croit avoir couvert. La console le dit quand aucune borne
> n'est pour tout le monde.

## Où une borne est vérifiée

Les bornes qui n'ont besoin d'aucune identité - une pour la route entière, une par adresse - sont
vérifiées **le plus à l'extérieur** : avant qu'une session soit cherchée, avant qu'un corps soit lu,
avant qu'un amont soit joint. Refuser tôt est tout l'intérêt d'une borne.

Les bornes clées sur un utilisateur, un jeton ou une organisation ne peuvent pas l'être, donc elles
coûtent une résolution de session - payée seulement par les routes qui en demandent une. Une règle
*restreinte* à certains appelants a besoin d'une identité elle aussi, quoi qu'elle compte.

## Ce qu'un refus porte

Un `429`, avec `RateLimit-Limit`, `RateLimit-Remaining`, `RateLimit-Reset` et `Retry-After`. Un refus
sans eux est une porte sans panneau : un client qui ne peut pas lire quand revenir abandonne ou tape en
boucle, et les deux sont pires pour le service que la borne protège. `Retry-After` n'est jamais zéro -
ce serait une invitation.

Une requête refusée n'est **pas comptée**. Un refus qui incrémente quand même transforme une rafale en
blocage qui lui survit : l'appelant lève le pied, et le compteur devant lequel il lève le pied est
encore nourri par ses propres refus.

## La fenêtre glisse

Un seau qui se remet à zéro répond faux à « cent sur la dernière minute » à chaque bascule : deux cents
requêtes passent en deux secondes, cent de chaque côté de la remise à zéro. Ce n'est pas une
approximation, c'est une faute. La forme est donc l'estimation classique à deux fenêtres - le compte de
la fenêtre courante plus celui de la précédente, pondéré par ce qui s'est écoulé de la courante - qui
coûte deux entiers par clé et se trompe de moins d'un pour cent.

## Les clés sont plafonnées

Une limite clée par adresse fait un compteur par adresse, et l'ensemble des adresses est choisi par
celui qui envoie les requêtes : un déluge ferait grossir la mémoire de la passerelle par le mécanisme
même installé pour survivre à un déluge.

Donc chaque règle suit au plus dix mille appelants distincts, les clés expirées sont balayées d'abord,
et la longue traîne **partage un compteur**. Refuser les non suivis laisserait n'importe qui couper le
service en tournant d'adresse ; les laisser passer sans borne laisserait n'importe qui contourner la
borne de la même façon.

## Ce qu'un rechargement fait

Les compteurs sont fabriqués quand une route est compilée, donc **enregistrer une route les remet à
zéro** et un appelant en cours de fenêtre repart. C'est l'échange honnête d'une conception sans
persistance : porter les compteurs à travers un rechargement voudrait dire les clés par identité de
règle, et une règle n'a pas d'identité - passer *cent par minute* à *deux cents par minute* hériterait
du compte d'une borne qui n'existe plus.

## Par endpoint

Une opération de l'inventaire OpenAPI d'une route peut porter ses propres bornes, en plus de celles de
la route (QUOTA-05). Choisie opération par opération et jamais en défaut sur tout l'inventaire : une
borne est un compteur par opération **et** par appelant, donc une spec de deux cents opérations avec dix
mille clés ferait deux millions de compteurs - la leçon de cardinalité, un étage plus bas.

L'écran est **Infra > Endpoint rate limits**.

## En cluster : relire le chiffre

> [!WARNING]
> Ces compteurs vivent **en mémoire, sur chaque noeud**. Une borne de cent par minute sur quatre noeuds
> laisse passer jusqu'à quatre cents par minute pour l'installation. La console le dit là où le nombre
> se tape, parce qu'un chiffre qui veut dire autre chose en cluster que seul doit le dire à l'endroit
> où on l'écrit.

C'est une décision plutôt qu'un oubli : un compteur exact et partagé coûte un aller-retour vers la base
à chaque requête, ce qui n'est pas un prix qu'une passerelle peut payer sur le chemin de tout le trafic.
Ceci est la moitié **protectrice** - approximative, gratuite, et suffisante contre l'abus.

Un compteur *est* en base et il est exact : le **compteur anti-force brute** des connexions (AUTH-11).
Cinq essais veut dire cinq pour l'installation, pas cinq par noeud, et un redémarrage ne laisse plus
rentrer un attaquant qu'on venait de freiner.

## Ce qui manque

- **Le ralentissement.** Dépasser une borne bloque ; ça ne ralentit pas (QUOTA-02).
- **Les compteurs facturables.** La moitié qui se facture ou se montre à un client est un autre
  mécanisme, écrit par lots, et il n'existe pas encore (QUOTA-03) - avec lui, l'écran de consommation
  et ses seuils d'alerte.
- **Les compteurs partagés.** Un comptage correct en cluster attend les compteurs facturables
  (QUOTA-04). Le chemin est tracé : le compteur de connexions est exactement cette forme.
