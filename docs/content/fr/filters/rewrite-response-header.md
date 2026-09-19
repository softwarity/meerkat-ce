---
title: rewrite-response-header
section: Filtres
order: 85
summary: Réécrit la valeur d'un en-tête de réponse avec un remplacement par regexp.
---

# rewrite-response-header

Sort un nom interne de ce que le service dit de lui-même :
`billing-3.internal:8080` qui repart en `service:8080`.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `name` | chaîne | oui | L'en-tête réécrit. |
| `pattern` | chaîne | oui | La regexp confrontée à la valeur. |
| `replacement` | chaîne | non | Ce qui la remplace. Vide retire ce qui a matché. Les captures s'écrivent `$1`, `$2`. |

Un motif qui ne compile pas est refusé à l'enregistrement de la route.

## Exemple

```yaml
filters:
  - type: rewrite-response-header
    args:
      name: Server
      pattern: ^[^.]+\.internal
      replacement: service
```

## Notes

Toutes les valeurs d'un en-tête répété sont réécrites. Rien ne se passe si
l'en-tête est absent.

Pour un `Location`, utilisez
[rewrite-location](/#/docs/filters/rewrite-location) : il connaît l'origine que
l'appelant a réellement utilisée, ce qu'un remplacement fixe ne sait pas.

La syntaxe est celle de RE2, la bibliothèque de Go : pas de rétroréférence, pas de
lookaround.
