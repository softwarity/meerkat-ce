---
title: Sécurité par endpoint
section: Contrôle d'accès
order: 136
summary: Des règles par opération d'une route d'API, posées sur l'inventaire que fournit sa spec OpenAPI.
---

# Sécurité par endpoint

La règle d'une route couvre la route entière. Quand une route d'API expose
quarante opérations et que trois d'entre elles doivent être fermées, la règle
à écrire porte sur l'opération.

**Infra > Endpoint security.** Choisissez une route qui expose une spec
OpenAPI : Meerkat récupère et analyse la spec côté serveur, liste ses
opérations - méthode, chemin, résumé, tags - et vous laisse poser une règle sur
n'importe laquelle. Swagger 2.0 et OpenAPI 3.x sont tous deux compris, en JSON
comme en YAML.

La spec est soit **publiée par votre service**, auquel cas c'est une référence
vivante, relue à chaque ouverture de l'écran, soit **un fichier déposé ici**,
auquel cas c'est un snapshot qui ne change que lorsque vous en déposez un
autre. L'écran indique dans quel cas vous êtes.

## Les deux situations pour lesquelles elle existe

**Vous ne pouvez pas modifier l'application.** Un produit acheté, un service
qui appartient à une autre équipe, un binaire dont vous n'avez pas les sources.
Ses autorisations sont celles que son auteur a décidées, et vous n'avez aucun
moyen d'y ajouter une règle. La règle se place donc devant l'application :
Meerkat refuse l'appel avant même que le service soit atteint, et l'application
n'a pas besoin de savoir qu'elle est protégée.

**Vous avez découvert une faille en production.** Une opération qui n'aurait
jamais dû être ouverte - une réindexation, une purge, un export - répond à
quiconque connaît son chemin, parce que le backend n'a jamais rien vérifié.
Vous ne déployez rien. Vous ouvrez cet écran, vous posez la règle, et elle
s'applique dès la requête suivante.

![Les opérations d'une route, chacune avec la règle qui la gouverne](img/console/endpoint-security.webp)

La liste vient directement de la description OpenAPI de la route : méthode,
chemin, résumé et tags, tels que le service les déclare. Le badge de gauche
indique la règle en vigueur, et les icônes voisines signalent si des rôles, des
utilisateurs nommés ou des rate limits y sont attachés.

Ici, deux opérations portent leur propre règle. `DELETE /orders/{id}` est
réservée à un rôle dont le backend ignore tout : ce sont des rôles que l'on
prototype devant un service au lieu de les câbler dedans. Et
`POST /admin/reindex` est fermée à tout le monde, à une exception près.

![Fermer une opération, avec une exception nommée](img/console/endpoint-rule.webp)

Voilà la faille de production refermée : le niveau **Nobody** interdit
l'opération à tout le monde, avant tout appel au service, sauf au seul
exploitant qui doit encore l'exécuter. Aucun déploiement, aucun ticket à l'équipe responsable du backend,
aucune modification de l'application.

## Ce qu'est une règle

La même règle que celle d'une route - niveau, organisations, rôles,
utilisateurs nommés - à laquelle s'ajoutent une méthode et un chemin :

| Partie | Valeurs |
|---|---|
| Méthode | `GET`, `PUT`, `POST`, `DELETE`, `OPTIONS`, `HEAD`, `PATCH`, `TRACE`, ou `*` pour n'importe laquelle |
| Chemin | le chemin **tel que l'écrit la spec** : des segments exacts, `{id}` pour un segment, `**` en dernier segment pour tout ce qui se trouve en dessous |
| La règle | voir [La règle d'accès d'une route](/docs/access/route-access) |

Une règle peut aussi porter ses propres rate limits, et ceux-ci sont
évalués **avant** la règle d'accès : un afflux de requêtes est refusé sans
qu'aucune session soit recherchée.

> [!NOTE]
> Le chemin est celui de la spec, pas celui qui arrive à la gateway. Si la
> route retire un préfixe, Meerkat le remet avant de comparer : vous écrivez
> donc `/orders/{id}`, comme dans le document, quel que soit le chemin public.

## Priorité : l'ordre, pas la précision

**La première règle de la liste qui correspond l'emporte.** Il n'y a pas de
classement donnant l'avantage à la règle la plus précise. Une règle `*` sur
`/**` placée en premier absorberait tout ce qui se trouve en dessous : mettez
donc les règles générales en dernier.

Une opération à laquelle **aucune** règle ne correspond retombe sur **la règle
de la route elle-même** - sauf si **Only listed operations are reachable** est
activé, au bas de l'écran. Dans ce cas, elle est refusée à tout le monde, et le
tableau affiche *Nobody* en face d'elle.

> [!WARNING]
> Sans cet interrupteur, une opération que vous avez oubliée n'est pas fermée :
> elle obéit à la règle de la route, qui peut être déléguée - et il en va de
> même pour un endpoint que le service livrera la semaine prochaine. Laisser
> une liste incomplète de règles d'endpoint sur une route déléguée ne protège
> rien. Activez l'interrupteur quand la liste est censée constituer tout le
> contrat.

## Une règle peut ouvrir aussi bien que fermer

Une règle d'endpoint remplace la règle de la route pour cette opération, dans
les deux sens. Une règle où rien n'est défini est *déléguée* : elle **rouvre**
donc une opération sur une route qui exige une session - c'est ainsi qu'un
health check public cohabite avec une API par ailleurs fermée.

Pour cette raison, une route qui porte des règles d'endpoint n'applique pas sa
propre règle au moment où les routes sont choisies : elle refuserait un
appelant qu'une règle d'endpoint s'apprêtait à laisser entrer. La règle de la
route n'est pas perdue pour autant : elle sert de repli, à l'intérieur, pour
toute opération à laquelle aucune règle ne correspond.

## La spec elle-même n'est pas cachée

Quand la spec est un fichier que vous avez déposé, la gateway la sert sur la
route, et elle la sert **en dehors** des règles d'endpoint : c'est la règle de
la route qui s'y applique. Les règles par opération habillent un contrat, elles
ne le dissimulent pas - et `curl`, Postman ou un générateur de client le lisent
tous à la même URL.

## Ce que l'application exige déjà

Beaucoup de services disent dans leur propre spec quels rôles une opération
demande : ce sont les scopes de l'exigence de sécurité de l'opération, tels que
des bibliothèques comme nestjs-granted ou springdoc les écrivent à partir des
contrôles du code :

```json
"security": [{ "bearer": ["ROLE_ADMIN"] }]
```

L'écran l'affiche dans une colonne **Application requires** : les rôles, *Signed
in* pour une exigence sans rôle, ou *Public* pour une opération que la spec ouvre
explicitement (la colonne de la première image de cette page). C'est une
information en lecture seule. **C'est l'application qui
l'applique, pas Meerkat** : rien ici ne change ce que la gateway laisse passer,
et avec **Only listed operations are reachable** activé, une opération que
l'application protège reste refusée par Meerkat tant qu'aucune règle ne la liste.

La documentation d'API des développeurs (`/meerkat/apidocs/`) le dit aussi, en
tête de la description de chaque opération, parce que swagger-ui affiche un
cadenas pour une exigence bearer mais jamais ses rôles.

## Les rôles que l'application déclare

**Declared roles**, dans l'en-tête de l'écran, liste tous les rôles que la spec
nomme et rattache chacun au catalogue :

- **New role** : créé dans le catalogue avec un préfixe (le nom de la route et un
  tiret par défaut, `orders-ROLE_ADMIN`) et une étiquette (le nom de la route par
  défaut) ;
- **un rôle existant** : qui le détient détient le rôle de l'application ;
- **Not mapped** : jamais transmis.

![Les rôles que l'application déclare, chacun rattaché à un rôle du catalogue](img/console/endpoint-declared-roles.webp)

Créer des rôles du catalogue demande le droit d'écrire le catalogue (root ou
app admin) ; se rattacher à des rôles existants ne demande que celui de la route.

Une fois qu'une route rattache ses rôles, son **Auth forward** envoie par défaut
les rôles déclarés que la personne détient, **sous les noms de l'application** :
quelqu'un qui détient `orders-ROLE_ADMIN` arrive au service comme `ROLE_ADMIN`, et
les autres rôles du catalogue restent à la maison. L'expression des rôles
derrière est `{{.Roles | .Declared | join ","}}` ; `.Declared` se compose avec les
autres verbes, donc `{{.Roles | .Declared | json}}` envoie les mêmes rôles en
tableau JSON.
