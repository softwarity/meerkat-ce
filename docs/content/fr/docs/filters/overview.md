---
title: Filtres
section: Filtres
order: 60
summary: Ce qu'est un filtre, les quatre phases où il peut s'exécuter, et les trente-quatre filtres du catalogue.
---

# Filtres

Un **filtre** est une brique qu'une route applique au trafic qu'elle a accepté. Là
où un [prédicat](/docs/predicates/overview) décide si une requête concerne cette
route, un filtre décide de ce qu'elle devient : la refuser, la modifier à l'aller,
modifier la réponse au retour, ou y répondre directement.

Tous s'écrivent de la même façon, un `type` et ses `args` :

```yaml
filters:
  - type: strip-prefix
    args:
      parts: 1
  - type: set-request-header
    args:
      name: X-Tenant
      value: northwind
  - type: security-headers
    args:
      frameOptions: DENY
```

Un argument inconnu, un argument obligatoire manquant ou une valeur du mauvais
type sont refusés à l'enregistrement de la route, et le message indique ce qui est
autorisé.

## Les quatre phases

La phase d'un filtre ne se choisit pas : elle découle de son type, et elle dit
**quand** la brique s'exécute.

| Phase | Ce qu'elle fait |
| --- | --- |
| **gate** | Accepte ou refuse, avant toute transformation. Elle répond elle-même à l'appelant. |
| **request** | Modifie la requête sur le chemin du service. |
| **response** | Modifie la réponse sur le chemin du retour vers l'appelant. |
| **terminal** | La route répond elle-même, et rien n'est transmis à l'upstream. |

Pour une requête, dans l'ordre :

1. les **gates**, dans l'ordre où elles sont écrites. Celle qui refuse répond sur-le-champ, avec un statut dont l'appelant peut tirer quelque chose, et plus rien ne s'exécute.
2. les filtres de **requête**, dans l'ordre où ils sont écrits. L'upstream est ensuite appelé : son propre chemin de base est ajouté une fois que les filtres ont fait leur travail, et le transfert d'identité passe en dernier, si bien qu'il a le dernier mot sur les en-têtes qu'il écrit lui-même.
3. les filtres de **réponse**, dans l'ordre où ils sont écrits, sur le chemin du retour.

Une gate n'est pas un prédicat. Un prédicat qui ne correspond pas laisse sa chance
à la **route suivante** ; une gate qui refuse arrête la requête sur place. "Trop
volumineux" ne veut pas dire "pas pour cette route" : cela veut dire non.

## Ce que veut dire terminal

Un filtre terminal répond lui-même pour la route, et l'upstream n'est donc jamais
appelé. Ils sont quatre : [redirect](/docs/filters/redirect),
[respond](/docs/filters/respond), [files](/docs/filters/files) et
[maintenance](/docs/filters/maintenance).

- **Un seul par route.** Un second filtre terminal sur la même route est refusé.
- **Les filtres de requête sont ignorés**, et la gateway journalise leur nombre : il n'y a plus de requête transmise à modifier.
- **Les filtres de réponse s'appliquent toujours.** Ce que renvoie un filtre terminal est une réponse comme une autre, et un `Cache-Control` ou un en-tête de sécurité y est aussi légitime que sur une réponse venue de l'upstream.
- **Les gates s'appliquent toujours.** Une route qui répond elle-même a autant de raisons de refuser un corps trop volumineux qu'une route qui transmet à un upstream.

> [!TIP]
> Tout argument peut contenir une référence au coffre, écrite `$name` et résolue
> au chargement de la route - c'est ainsi qu'un secret partagé reste hors de la
> configuration exportée. Deux arguments sont pris tels quels et ne sont jamais
> développés : le gabarit de [respond](/docs/filters/respond) et le motif de
> [version](/docs/predicates/version), parce que tous deux écrivent leurs propres
> `$`.

## Les gates

| Type | Ce que ça fait |
| --- | --- |
| [max-request-body](/docs/filters/max-request-body) | Refuse, avec un `413`, une requête dont le corps dépasse cette taille. |
| [max-request-headers](/docs/filters/max-request-headers) | Refuse, avec un `431`, une requête dont les en-têtes pèsent plus que cette taille. |

## Les filtres de requête

| Type | Ce que ça fait |
| --- | --- |
| [add-query-param](/docs/filters/add-query-param) | Ajoute un paramètre de requête. |
| [add-request-header](/docs/filters/add-request-header) | Ajoute une valeur à un en-tête de requête, au besoin seulement si l'appelant n'en a envoyé aucune. |
| [copy-request-header](/docs/filters/copy-request-header) | Copie un en-tête de requête sous un second nom, sans toucher à l'original. |
| [prefix-path](/docs/filters/prefix-path) | Ajoute un préfixe au début du chemin avant l'envoi à l'upstream. |
| [preserve-host](/docs/filters/preserve-host) | Envoie à l'upstream le Host de l'appelant, et non celui de l'upstream. |
| [remove-query-param](/docs/filters/remove-query-param) | Retire un paramètre de la requête envoyée à l'upstream. |
| [remove-request-cookie](/docs/filters/remove-request-cookie) | Retire un cookie de la requête. |
| [remove-request-header](/docs/filters/remove-request-header) | Retire un en-tête de la requête avant l'envoi à l'upstream. |
| [rename-request-header](/docs/filters/rename-request-header) | Déplace un en-tête de requête sous un autre nom, avec toutes ses valeurs. |
| [rewrite-path](/docs/filters/rewrite-path) | Réécrit le chemin par un remplacement à expression régulière. |
| [rewrite-query-param](/docs/filters/rewrite-query-param) | Réécrit la valeur d'un paramètre de requête par un remplacement à expression régulière. |
| [set-host](/docs/filters/set-host) | Fixe le Host envoyé à l'upstream. |
| [set-path](/docs/filters/set-path) | Remplace en entier le chemin envoyé à l'upstream. |
| [set-query-param](/docs/filters/set-query-param) | Fixe un paramètre de requête, en remplaçant toute valeur envoyée par l'appelant. |
| [set-request-header](/docs/filters/set-request-header) | Fixe un en-tête de requête, en remplaçant toute valeur envoyée par le client. |
| [strip-prefix](/docs/filters/strip-prefix) | Retire les premiers segments du chemin avant l'envoi à l'upstream. |

## Les filtres de réponse

| Type | Ce que ça fait |
| --- | --- |
| [add-response-header](/docs/filters/add-response-header) | Ajoute une valeur à un en-tête de réponse. |
| [cache-control](/docs/filters/cache-control) | Fixe l'en-tête `Cache-Control` de la réponse. |
| [cookie-attributes](/docs/filters/cookie-attributes) | Impose des attributs aux cookies écrits par un upstream. |
| [dedupe-response-header](/docs/filters/dedupe-response-header) | Supprime les valeurs en double d'un en-tête de réponse. |
| [remove-json-fields](/docs/filters/remove-json-fields) | Retire des champs d'une réponse JSON, par leur nom ou par un chemin à points. |
| [remove-response-header](/docs/filters/remove-response-header) | Retire un en-tête de la réponse avant qu'elle n'atteigne le client. |
| [rename-response-header](/docs/filters/rename-response-header) | Déplace un en-tête de réponse sous un autre nom, avec toutes ses valeurs. |
| [rewrite-location](/docs/filters/rewrite-location) | Ramène dans l'espace public le `Location` d'une redirection émise par l'upstream. |
| [rewrite-response-header](/docs/filters/rewrite-response-header) | Réécrit la valeur d'un en-tête de réponse par un remplacement à expression régulière. |
| [security-headers](/docs/filters/security-headers) | Ajoute les en-têtes de réponse sur lesquels un navigateur s'appuie pour se durcir. |
| [set-response-header](/docs/filters/set-response-header) | Fixe un en-tête de réponse, en remplaçant toute valeur envoyée par l'upstream. |
| [set-status](/docs/filters/set-status) | Remplace le code de statut de la réponse de l'upstream. |

## Les filtres terminaux

| Type | Ce que ça fait |
| --- | --- |
| [files](/docs/filters/files) | Répond avec les fichiers téléversés sur la route, sans rien transmettre. |
| [maintenance](/docs/filters/maintenance) | Répond `503` avec la page d'indisponibilité de la gateway, sans rien transmettre à l'upstream. |
| [redirect](/docs/filters/redirect) | Répond par une redirection, sans rien transmettre à l'upstream. |
| [respond](/docs/filters/respond) | Répond à partir d'un gabarit, avec l'appelant connecté à disposition. |
