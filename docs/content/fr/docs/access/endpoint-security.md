---
title: Sécurité par endpoint
section: Contrôle d'accès
order: 136
summary: Des règles par opération d'une route d'API, posées sur l'inventaire que fournit sa spec OpenAPI.
---

# Sécurité par endpoint

La règle d'une route couvre toute la route. Quand une route d'API expose quarante
opérations et que trois doivent être fermées, la règle à écrire est par opération.

**Infra > Endpoint security.** Choisissez une route qui expose une spec OpenAPI :
Meerkat la récupère et l'analyse côté serveur, liste ses opérations - méthode,
chemin, résumé, étiquettes - et vous laisse poser une règle sur n'importe laquelle.
Swagger 2.0 et OpenAPI 3.x sont tous deux compris, en JSON comme en YAML.

La spec est soit **publiée par votre service**, et c'est alors une référence vivante
relue à chaque ouverture de l'écran, soit un **fichier déposé ici**, et c'est alors
un instantané qui ne change que si vous en déposez un autre. L'écran le dit.

## Les deux situations pour lesquelles il existe

**Vous ne pouvez pas modifier l'application.** Un produit acheté, un service
qu'une autre équipe possède, un binaire dont vous n'avez pas les sources. Son
autorisation est celle que son auteur a décidée, et vous n'avez aucun moyen d'y
ajouter une règle. La règle se pose devant : Meerkat refuse l'appel avant même
que le service soit atteint, donc l'application n'a pas besoin de savoir
qu'elle est protégée.

**Vous découvrez un trou en production.** Une opération qui n'aurait jamais dû
être ouverte - une réindexation, une purge, un export - répond à quiconque
connaît son chemin, parce que le backend n'a rien vérifié. Vous ne déployez
pas. Vous ouvrez cet écran, vous posez la règle, et elle tient à la requête
suivante.

![Les opérations d'une route, chacune avec la règle qui la gouverne](img/console/endpoint-security.webp)

La liste vient directement de la description OpenAPI de la route : méthode,
chemin, résumé et étiquettes, exactement comme le service les déclare. Le badge
à gauche est la règle en vigueur, et les icônes à côté disent si des rôles, des
utilisateurs nommés ou des limites de débit y sont attachés.

Ici, deux opérations portent leur propre règle. `DELETE /orders/{id}` est
restreinte à un rôle dont le backend ne sait rien : ce sont des rôles
prototypés devant un service plutôt que câblés dedans. Et
`POST /admin/reindex` est fermée à tout le monde, à une exception près.

![Fermer une opération, avec une exception nommée](img/console/endpoint-rule.webp)

C'est le trou de production, bouché : **Personne** ne peut l'appeler, refusée
avant que le service soit appelé, sauf le seul exploitant qui doit encore la
lancer. Aucun déploiement, aucun ticket à l'équipe qui possède le backend,
aucun changement dans l'application.

## Ce qu'est une règle

La même règle que celle d'une route - niveau, organisations, rôles, comptes nommés -
plus une méthode et un chemin :

| Partie | Valeurs |
|---|---|
| Méthode | `GET`, `PUT`, `POST`, `DELETE`, `OPTIONS`, `HEAD`, `PATCH`, `TRACE`, ou `*` pour n'importe laquelle |
| Chemin | le chemin **tel que la spec l'écrit** : segments exacts, `{id}` pour un segment, `**` en dernier segment pour tout ce qui est en dessous |
| La règle | voir [La règle d'accès d'une route](/docs/access/route-access) |

Une règle peut aussi porter ses propres limites de débit, et celles-là sont évaluées
**avant** la règle d'accès : un flot est refusé sans qu'une session soit cherchée.

> [!NOTE]
> Le chemin est celui de la spec, pas celui qui arrive sur la gateway. Si la route
> retire un préfixe, Meerkat le remet avant d'apparier : `/orders/{id}` dans le
> document est ce que vous écrivez, quel que soit le chemin public.

## La priorité : l'ordre, pas la précision

**La première règle de la liste qui matche gagne.** Il n'y a pas de classement par
précision. Une règle sur `*` et `/**` placée en premier avalerait tout ce qui est en
dessous : mettez les règles générales en dernier.

Une opération qu'**aucune** règle ne matche retombe sur la **règle de la route**.

> [!WARNING]
> Il n'y a **aucun interrupteur de refus par défaut**. Une opération que vous avez
> oubliée n'est pas fermée : elle est régie par la règle de la route, qui peut être
> déléguée. Laisser une liste incomplète de règles d'endpoint sur une route déléguée
> ne protège rien.

Si vous voulez un inventaire où seul ce que vous avez listé est atteignable, écrivez
le refus vous-même, en **dernière** règle : méthode `*`, chemin `/**`, niveau
*Nobody*. Tout ce qui n'a pas été apparié plus haut y atterrit. C'est la réponse
d'aujourd'hui au refus par défaut, et FEATURES.md liste toujours l'interrupteur comme
manquant.

## Une règle peut ouvrir autant que fermer

Une règle d'endpoint remplace la règle de la route pour cette opération, dans les
deux sens. Une règle vide est *déléguée* : elle **réouvre** donc une opération sur
une route qui exige une session - c'est ainsi qu'un contrôle de santé public vit sur
une API par ailleurs fermée.

À cause de cela, une route qui porte des règles d'endpoint n'applique pas sa propre
règle pendant le choix des routes : elle écarterait un appelant qu'une règle
d'endpoint était sur le point de laisser entrer. La règle de la route n'est pas
perdue - elle est le recours à l'intérieur, pour chaque opération qu'aucune règle n'a
appariée.

## La spec elle-même n'est pas cachée

Quand la spec est un fichier que vous avez déposé, la gateway la sert sur la route,
et elle la sert **en dehors** des règles d'endpoint : elle porte la règle de la route
à la place. Les règles par opération décorent un contrat, elles ne le dissimulent
pas : un `curl`, Postman et un générateur de client la lisent tous à la même URL.
