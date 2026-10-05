---
title: respond
section: Filtres
order: 81
summary: Répond à partir d'un gabarit au lieu de transmettre à l'upstream, avec l'appelant connecté à disposition.
---

# respond

Permet à la route de répondre elle-même à partir d'un gabarit, qui a accès à
l'appelant connecté. Deux usages : exposer l'endpoint d'identité qu'une application
hébergée attend **sous sa propre forme**, et servir un petit document fixe - un
`robots.txt`, une configuration publique - sans aucun service derrière.

C'est un filtre **terminal** : rien n'est transmis et aucun upstream n'est appelé.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `body` | chaîne | oui | Un gabarit Go `text/template`. Ce qu'il peut lire est décrit plus bas. |
| `contentType` | chaîne | non | Le Content-Type de la réponse. Par défaut : `application/json; charset=utf-8`. |
| `status` | entier | non | Le statut HTTP de la réponse. Par défaut : `200`. |

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
`{{.Email}}`, `{{.Tenant}}`, `{{.TenantID}}`, `{{.Timezone}}`, `{{.Roles}}`, ainsi
que `{{.SignedIn}}`, qui vaut `false` quand personne n'est connecté. Rien d'autre
n'est accessible : ni base de données, ni système de fichiers, ni autre compte.

Trois fonctions l'accompagnent :

- `json` rend une valeur en JSON, guillemets et échappement compris.
- `join` aplatit une liste, par exemple `{{join "," .Roles}}`.
- `wrap` transforme une liste en objets à une seule clé : `{{json (wrap "authority" .Roles)}}` donne `[{"authority":"A"}]`, la forme que la moitié des applications attendent pour les rôles.

> [!WARNING]
> Écrivez `"name": {{json .Username}}` et **non** `"name": "{{.Username}}"`. La
> seconde forme a l'air correcte, et casse le jour où un nom contient un
> guillemet - or les noms viennent des annuaires, pas de vous.

Le gabarit est analysé **et exécuté une fois** à l'enregistrement de la route, avec
un appelant témoin : un champ qui n'existe pas (`{{.Usernme}}`) est détecté à ce
moment-là plutôt qu'en production. La console affiche un aperçu de la réponse
rendue pendant la saisie, en passant par le même code.

La réponse porte `Cache-Control: no-store`, puisque son contenu dépend de celui
qui la demande.

Le gabarit est pris tel quel : un `$` qui s'y trouve n'est jamais lu comme une
référence au coffre.
