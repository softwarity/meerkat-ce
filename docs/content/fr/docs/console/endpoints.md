---
title: Sécurité et rate limits par endpoint
section: La console
order: 154
summary: Définir qui peut appeler chaque opération d'une API, et quel volume elle accepte, à partir de la spécification OpenAPI de la route.
---

# Sécurité et rate limits par endpoint

**Infra > Endpoint security**, **Infra > Endpoint rate limits** et **Infra >
Endpoint audit** sont trois entrées vers un même inventaire : les opérations que la
gateway lit dans la spécification OpenAPI d'une route. La sécurité répond à la
question *qui peut appeler cette opération*, les rate limits à *combien de fois*,
l'audit à *quels appels consigner* (Enterprise, voir
[auditer les opérations d'une route](/docs/operations/audit#auditer-les-oprations-dune-route)).
Cette page traite des deux premières.

## Avant de commencer

L'écran ne liste que les routes qui exposent une spécification OpenAPI. Si aucune n'en
expose, il le signale et vous renvoie vers Routes : déclarez la spécification dans la
section **Target** de la route, qu'elle soit publiée par le service ou déposée sous
forme de fichier, puis revenez.

## Ce que vous y faites

1. **Choisissez la route** dans le sélecteur en haut de l'écran. Son titre, son format
   et sa version s'affichent à côté. Si vous arrivez depuis l'éditeur d'une route,
   celle-ci est présélectionnée.
2. **Trouvez l'opération.** Le tableau donne la méthode, le chemin, les tags et la
   description. Les en-têtes des colonnes méthode et tags sont des filtres ; la
   colonne du chemin se trie. Le pied du tableau compte ce qui est couvert :
   *N operations, M secured* (ou *M bounded*).
3. **Cliquez sur la ligne.** L'opération s'ouvre dans un tiroir, avec son
   identifiant d'opération et la section dont traite cette page.
4. **Écrivez la règle.** Il n'y a pas de bouton Save : le pied du panneau affiche
   *Saving...* puis *All changes saved*.

## Sur la page de sécurité

Le tiroir comporte un seul interrupteur, **Override the route config**.

- Si vous n'y touchez pas, l'opération affiche **Inherits the route config**, et cette
  phrase renvoie vers la section Security de la route. Une opération à laquelle vous ne
  touchez jamais reste sécurisée par le backend lui-même.
- Si vous l'activez, vous retrouvez l'éditeur d'accès qu'utilise la route. Il comporte
  deux axes, qui doivent être satisfaits tous les deux : le **niveau d'appartenance**,
  et un **filtre de rôles** évalué dans l'organisation active de l'appelant.

| Niveau | Ce qu'il exige |
|---|---|
| **Delegated** | Rien. Tout le monde passe, connecté ou non ; c'est le service qui décide |
| **Signed in** | N'importe quel compte, y compris un compte qui n'appartient encore à aucune organisation |
| **In an organisation** | Une organisation doit être active dans la session |
| **In one of these organisations** | L'organisation active doit faire partie de celles qui sont nommées |
| **Nobody** | Refusé avant même que le service soit appelé |

![Le même éditeur d'accès, dans la section Security d'une route : un niveau, une liste de rôles et une exception pour des utilisateurs nommés](img/console/route-editor-security.webp)

Le même éditeur, ici sur la route *Orders API* : le niveau, les rôles (un seul suffit
pour obtenir l'accès, et il doit être détenu dans l'organisation active) et l'exception
pour les utilisateurs nommés. Le panneau d'une opération affiche exactement cela une
fois la dérogation activée.

**Les utilisateurs nommés sont une exception, pas un niveau** : toute personne listée
passe, quoi qu'exige le niveau. C'est ainsi qu'un compte de service ou un compte de
support franchit une règle écrite pour tous les autres.

> [!NOTE]
> Quel que soit votre choix, le service continue d'appliquer ses propres règles. Cet
> écran ajoute des conditions, il n'en retire jamais - c'est pourquoi le niveau le plus
> ouvert s'appelle *delegated* et non *public*.

## Sur la page des rate limits

![La page des rate limits : les limites que porte une opération, dans la première colonne](img/console/endpoint-limits.webp)

Le tiroir contient les limites propres à l'opération, qui **s'ajoutent** à
celles de la route, toujours applicables. Une limite est un compteur, et vous
choisissez la clé sur laquelle il porte :

| Clé | Un budget par |
|---|---|
| **The whole operation** | opération, pour tout ce qu'elle reçoit, quel que soit l'appelant |
| **Each user** | compte connecté (les appelants anonymes ne sont pas couverts) |
| **Each API token** | jeton, ce qui limite une intégration sans limiter son propriétaire |
| **Each organisation** | organisation - un quota vendu à un client |
| **Each address** | adresse du client, la seule clé dont dispose un appelant anonyme |

La seconde moitié d'une règle précise **à qui** elle s'applique, sous la même forme que
l'accès décrit plus haut : un rôle devient ainsi un palier tarifaire, sans concept
supplémentaire. Plusieurs limites s'appliquent en même temps, et la première dépassée
répond 429.

Réservez ces limites aux quelques opérations qui en ont besoin. Une limite posée sur
chaque opération d'un vaste inventaire, c'est un compteur par opération et par
appelant.

## Pièges

- **Une spécification publiée par le service évolue avec le service.** Quand le service
  renomme un chemin, la règle écrite pour l'ancien chemin ne se rattache plus à rien.
  Un fichier déposé est un snapshot : il ne change pas à votre insu.
- **Une opération non modifiée n'est pas protégée par Meerkat**, ce qui ne veut pas
  dire qu'elle n'est pas protégée : le backend reste aux commandes. Décider de
  centraliser le contrôle d'accès, c'est activer la dérogation délibérément, opération
  par opération.
- **La règle propre à la route ne figure pas sur cet écran.** Elle intervient dans le
  choix même de la route, elle est donc rangée avec la route ; la phrase *Inherits*
  permet d'y accéder.
- **Avec une clé par adresse, c'est l'attaquant qui choisit son budget.** L'adresse est
  lue sur la connexion, jamais dans un en-tête transmis, mais cela reste un budget par
  adresse.

Voir aussi [Routes](/docs/console/routes) pour la règle qui vaut pour toute la route,
et [Roles](/docs/console/roles) pour le catalogue auquel ces règles font référence.
