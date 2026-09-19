---
title: Filtres
section: Filtres
order: 60
summary: Ce qu'est un filtre, les quatre phases dans lesquelles il peut tourner, et les trente-trois du catalogue.
---

# Filtres

Un **filtre** est une brique qu'une route pose sur le trafic qu'elle a accepté. Là
où un [prédicat](/docs/predicates/overview) décide si une requête est pour cette
route, un filtre décide ce qu'il lui arrive : la refuser, la modifier à l'aller,
modifier la réponse au retour, ou y répondre lui-même.

Ils s'écrivent tous de la même façon, un `type` et ses `args` :

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
type est refusé à l'enregistrement de la route, en nommant ce qui est permis.

## Les quatre phases

La phase d'un filtre n'est pas un choix : elle vient avec le type, et elle dit
**quand** la brique tourne.

| Phase | Ce qu'elle fait |
| --- | --- |
| **gate** | Accepte ou refuse, avant que quoi que ce soit ne soit transformé. Elle répond elle-même à l'appelant. |
| **request** | Modifie la requête sur le chemin du service. |
| **response** | Modifie la réponse sur le chemin du retour. |
| **terminal** | La route répond elle-même, et rien n'est proxifié. |

Pour une requête, dans l'ordre :

1. les **gates**, dans l'ordre où elles sont écrites. Celle qui refuse répond sur le champ, avec un statut sur lequel l'appelant peut agir, et rien d'autre ne tourne.
2. les filtres **request**, dans l'ordre où ils sont écrits. Puis l'amont est appelé : le chemin de base de l'amont est ajouté après que les filtres ont eu leur mot à dire, et le transfert d'identité passe en dernier, donc il gagne sur les en-têtes qu'il écrit lui-même.
3. les filtres **response**, dans l'ordre où ils sont écrits, au retour.

Une gate n'est pas un prédicat. Un prédicat qui ne matche pas laisse essayer la
**route suivante** ; une gate qui refuse arrête la requête là. Trop gros ne veut
pas dire 'pas pour cette route', ça veut dire non.

## Ce que veut dire terminal

Un filtre terminal répond à la place de la route, donc l'amont n'est jamais
appelé : [redirect](/docs/filters/redirect),
[respond](/docs/filters/respond) et
[maintenance](/docs/filters/maintenance) sont les trois.

- **Un seul par route.** Un deuxième filtre terminal sur la même route est refusé.
- **Les filtres request sont abandonnés** et la gateway journalise combien : il n'y a plus de requête proxifiée à modifier.
- **Les filtres response s'appliquent toujours.** Ce qu'un terminal répond est une réponse comme une autre, et un `Cache-Control` ou un en-tête de sécurité dessus est aussi légitime que sur une réponse proxifiée.
- **Les gates s'appliquent toujours.** Une route qui répond elle-même a autant de raisons de refuser un corps trop gros qu'une route qui proxifie.

> [!TIP]
> N'importe quel argument peut porter une référence au coffre-fort, écrite `$nom`,
> résolue au chargement de la route - c'est ainsi qu'un secret partagé reste hors
> de la configuration exportée. Deux arguments sont pris au mot et jamais
> interprétés : le gabarit de [respond](/docs/filters/respond) et le motif de
> [version](/docs/predicates/version), parce que tous les deux écrivent un `$`
> qui leur appartient.

## Les gates

| Type | Ce qu'il fait |
| --- | --- |
| [max-request-body](/docs/filters/max-request-body) | Refuse un corps de requête plus gros que cette taille, avec un `413`. |
| [max-request-headers](/docs/filters/max-request-headers) | Refuse une requête dont les en-têtes pèsent plus que cette taille, avec un `431`. |

## Les filtres de requête

| Type | Ce qu'il fait |
| --- | --- |
| [add-query-param](/docs/filters/add-query-param) | Ajoute un paramètre de requête. |
| [add-request-header](/docs/filters/add-request-header) | Ajoute une valeur d'en-tête de requête, éventuellement seulement si l'appelant n'en a envoyé aucune. |
| [copy-request-header](/docs/filters/copy-request-header) | Copie un en-tête de requête sous un second nom, en laissant l'original en place. |
| [prefix-path](/docs/filters/prefix-path) | Préfixe le chemin avant de proxifier. |
| [preserve-host](/docs/filters/preserve-host) | Envoie à l'amont l'hôte de l'appelant plutôt que celui de l'amont. |
| [remove-query-param](/docs/filters/remove-query-param) | Retire un paramètre de la requête proxifiée. |
| [remove-request-cookie](/docs/filters/remove-request-cookie) | Retire un cookie de la requête. |
| [remove-request-header](/docs/filters/remove-request-header) | Retire un en-tête de requête avant de proxifier. |
| [rename-request-header](/docs/filters/rename-request-header) | Déplace un en-tête de requête sous un autre nom, valeurs comprises. |
| [rewrite-path](/docs/filters/rewrite-path) | Réécrit le chemin avec un remplacement par regexp. |
| [rewrite-query-param](/docs/filters/rewrite-query-param) | Réécrit la valeur d'un paramètre avec un remplacement par regexp. |
| [set-host](/docs/filters/set-host) | Fixe l'hôte envoyé à l'amont. |
| [set-path](/docs/filters/set-path) | Remplace tout le chemin envoyé à l'amont. |
| [set-query-param](/docs/filters/set-query-param) | Fixe un paramètre de requête, en remplaçant ce que l'appelant a envoyé. |
| [set-request-header](/docs/filters/set-request-header) | Fixe un en-tête de requête, en remplaçant la valeur du client. |
| [strip-prefix](/docs/filters/strip-prefix) | Retire les premiers segments du chemin avant de proxifier. |

## Les filtres de réponse

| Type | Ce qu'il fait |
| --- | --- |
| [add-response-header](/docs/filters/add-response-header) | Ajoute une valeur d'en-tête de réponse. |
| [cache-control](/docs/filters/cache-control) | Pose le `Cache-Control` de la réponse. |
| [cookie-attributes](/docs/filters/cookie-attributes) | Force des attributs sur les cookies que pose un amont. |
| [dedupe-response-header](/docs/filters/dedupe-response-header) | Supprime les valeurs répétées d'un en-tête de réponse. |
| [remove-json-fields](/docs/filters/remove-json-fields) | Retire des champs d'une réponse JSON, par nom ou par chemin pointé. |
| [remove-response-header](/docs/filters/remove-response-header) | Retire un en-tête de réponse avant qu'il n'atteigne le client. |
| [rename-response-header](/docs/filters/rename-response-header) | Déplace un en-tête de réponse sous un autre nom, valeurs comprises. |
| [rewrite-location](/docs/filters/rewrite-location) | Ramène le `Location` d'une redirection d'amont dans l'espace public. |
| [rewrite-response-header](/docs/filters/rewrite-response-header) | Réécrit la valeur d'un en-tête de réponse avec un remplacement par regexp. |
| [security-headers](/docs/filters/security-headers) | Pose les en-têtes de réponse sur lesquels un navigateur se durcit. |
| [set-response-header](/docs/filters/set-response-header) | Fixe un en-tête de réponse, en remplaçant la valeur de l'amont. |
| [set-status](/docs/filters/set-status) | Remplace le code de statut de la réponse de l'amont. |

## Les filtres terminaux

| Type | Ce qu'il fait |
| --- | --- |
| [maintenance](/docs/filters/maintenance) | Répond `503` avec la page d'indisponibilité de la gateway au lieu de proxifier. |
| [redirect](/docs/filters/redirect) | Répond une redirection au lieu de proxifier. |
| [respond](/docs/filters/respond) | Répond depuis un gabarit, avec l'appelant connecté à disposition. |
