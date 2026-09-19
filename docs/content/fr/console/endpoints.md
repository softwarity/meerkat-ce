---
title: Sécurité et quotas par endpoint
section: La console
order: 154
summary: Décider qui peut appeler chaque opération d'une API, et combien elle peut porter, depuis la spec OpenAPI de la route.
---

# Sécurité et quotas par endpoint

**Infra > Endpoint security** et **Infra > Endpoint rate limits** sont deux entrées
sur un seul inventaire : les opérations que la passerelle lit dans la spec OpenAPI
d'une route. La sécurité demande *qui peut appeler ceci*, les quotas demandent
*combien*. Deux entrées de menu, parce que ce sont deux questions avec lesquelles on
arrive, et un menu qui n'en nomme aucune est un menu où aucune ne se trouve.

## Avant de commencer

L'écran ne liste que les routes qui exposent une spec OpenAPI. Si aucune ne le
fait, il le dit et renvoie vers Routes : déclarez la spec dans la section **Target**
de la route, publiée par le service ou déposée en fichier, puis revenez.

## Ce qu'on y fait

1. **Choisir la route** dans le sélecteur en haut. Son titre, son format et sa
   version s'affichent à côté. Arriver depuis l'éditeur d'une route la
   présélectionne.
2. **Trouver l'opération.** Le tableau liste méthode, chemin, tags et description.
   Les en-têtes des colonnes méthode et tags sont des filtres ; la colonne chemin
   trie. Le pied de page compte ce qui est couvert : *N operations, M secured* (ou
   *M bounded*).
3. **Cliquer la ligne.** L'opération s'ouvre dans un tiroir, avec son identifiant
   d'opération et la section dont cette page parle.
4. **Ecrire la règle.** Il n'y a pas de bouton Save : le pied de page dit
   *Saving...* puis *All changes saved*.

## Sur la page sécurité

Le tiroir porte un interrupteur, **Override the route config**.

- Laissé tranquille, l'opération affiche **Inherits the route config**, et la phrase
  renvoie vers la section Security de la route. Une opération jamais touchée reste
  sécurisée par le backend lui-même.
- Activé, vous obtenez l'éditeur d'accès que la route utilise. Deux axes, tous deux
  exigés : le **niveau d'appartenance**, et un **filtre de rôles** évalué dans
  l'organisation active de l'appelant.

| Niveau | Ce qu'il exige |
|---|---|
| **Delegated** | Rien. Tout le monde passe, connecté ou non ; le service décide |
| **Signed in** | N'importe quel compte, y compris un qui n'appartient à aucune organisation |
| **In an organisation** | Une organisation doit être active sur la session |
| **In one of these organisations** | L'organisation active doit être l'une de celles nommées |
| **Nobody** | Refusé avant que le service soit appelé |

![Le même éditeur d'accès, sur la section Security d'une route : un niveau, une liste de rôles, et une exception pour des utilisateurs nommés](img/console/route-editor-security.webp)

Le même éditeur, ici sur la route *Billing* : le niveau, les rôles (un seul
suffit, jugés dans l'organisation active), et l'exception des utilisateurs
nommés. Le tiroir d'une opération montre exactement cela une fois la surcharge
activée.

**Les utilisateurs nommés sont une exception, pas un niveau** : qui est listé passe
quoi qu'exige le niveau. C'est ainsi qu'un compte de service ou un accès de support
traverse une règle écrite pour tous les autres.

> [!NOTE]
> Quel que soit votre choix, le service applique encore ses propres règles. Cet
> écran ajoute des conditions, il n'en enlève jamais - et c'est pourquoi le bout
> ouvert s'appelle *delegated* et non *public*.

## Sur la page quotas

Le tiroir porte les bornes propres à l'opération, **en plus** de celles de la route,
qui s'appliquent toujours. Une borne est un compteur, et vous choisissez sur quoi il
est clé :

| Clé | Un budget par |
|---|---|
| **The whole operation** | tout ce qu'elle porte, qui que soit l'appelant |
| **Each user** | compte connecté (les appelants anonymes ne sont pas couverts) |
| **Each API token** | jeton, pour borner une intégration sans borner son propriétaire |
| **Each organisation** | organisation - un quota vendu à un client |
| **Each address** | adresse cliente, la seule clé qu'un anonyme possède |

La seconde moitié d'une règle dit **à qui** elle s'applique, et c'est la même forme
d'accès que ci-dessus : un rôle devient un palier tarifaire sans nouveau concept.
Plusieurs bornes sont vraies à la fois, et la première dépassée répond 429.

Ecrivez-en pour les quelques opérations qui en ont besoin. Une borne sur chaque
opération d'un gros inventaire, c'est un compteur par opération et par appelant.

## Pièges

- **Une spec publiée par le service suit le service.** Quand le service renomme un
  chemin, la règle écrite contre l'ancien n'a plus rien à quoi s'accrocher. Un
  fichier déposé est un instantané et ne bouge pas sous vous.
- **Non touchée veut dire non gardée par Meerkat**, pas non gardée : le backend est
  encore aux commandes. Centraliser le contrôle d'accès veut dire surcharger
  délibérément, opération par opération.
- **La règle de la route n'est pas sur cet écran.** Elle participe au choix de la
  route elle-même, donc elle vit avec la route ; la phrase *Inherits* est le chemin
  vers elle.
- **Une clé par adresse laisse l'attaquant choisir son budget.** Elle est lue sur la
  connexion, jamais sur un en-tête transmis, mais elle reste un budget par adresse.

Voir aussi [Routes](/#/docs/console/routes) pour la règle de toute la route, et
[Roles](/#/docs/console/roles) pour le catalogue que ces règles nomment.
