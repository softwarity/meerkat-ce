---
title: respond
section: Filtres
order: 81
summary: Répond depuis un gabarit au lieu de proxifier, avec l'appelant connecté à disposition.
---

# respond

Laisse la route répondre elle-même depuis un gabarit, avec l'appelant connecté à
disposition. Deux usages : exposer l'endpoint d'identité qu'une application
hébergée attend **dans sa propre forme**, et servir un petit document fixe - un
`robots.txt`, une configuration publique - sans service derrière.

C'est un filtre **terminal** : rien n'est proxifié et aucun amont n'est appelé.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `body` | chaîne | oui | Un `text/template` Go. Voir ci-dessous ce qu'il peut atteindre. |
| `contentType` | chaîne | non | Content-Type de la réponse. Défaut : `application/json; charset=utf-8`. |
| `status` | entier | non | Statut HTTP de la réponse. Défaut : `200`. |

## Exemple

```yaml
filters:
  - type: respond
    args:
      body: '{"name": {{json .Username}}, "authorities": {{json (wrap "authority" .Roles)}}}'
      contentType: application/json; charset=utf-8
      status: 200
```

Une application qui interroge `/user` reçoit `{"name":"jsmith","authorities":[{"authority":"BILLING"}]}`.

## Notes

L'appelant est disponible sous `{{.Username}}`, `{{.UserID}}`, `{{.Fullname}}`,
`{{.Email}}`, `{{.Tenant}}`, `{{.TenantID}}`, `{{.Timezone}}`, `{{.Roles}}`, plus
`{{.SignedIn}}`, qui vaut `false` quand personne n'est connecté. Rien d'autre n'est
atteignable : ni base, ni système de fichiers, ni autre compte.

Trois fonctions l'accompagnent :

- `json` rend une valeur en JSON, quotes et échappement compris.
- `join` aplatit une liste, par exemple `{{join "," .Roles}}`.
- `wrap` transforme une liste en objets à une clé : `{{json (wrap "authority" .Roles)}}` donne `[{"authority":"A"}]`, la forme qu'attendent la moitié des applications pour des rôles.

> [!WARNING]
> Écrivez `"name": {{json .Username}}` et **non** `"name": "{{.Username}}"`. La
> seconde forme a l'air juste et casse le jour où un nom contient une quote - et
> les noms viennent des annuaires, pas de vous.

Le gabarit est analysé **et exécuté une fois** à l'enregistrement de la route,
contre un appelant témoin : un champ qui n'existe pas (`{{.Usernme}}`) est attrapé
là plutôt qu'en production. La console prévisualise la réponse rendue pendant la
frappe, par le même code.

La réponse porte `Cache-Control: no-store`, puisque ce qu'elle dit dépend de qui
demande.

Le gabarit est pris au mot : un `$` dedans n'est jamais lu comme une référence au
coffre-fort.
