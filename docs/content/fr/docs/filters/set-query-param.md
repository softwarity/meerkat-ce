---
title: set-query-param
section: Filtres
order: 89
summary: Fixe un paramètre de requête sur la requête proxifiée.
---

# set-query-param

Décide un paramètre de requête au niveau de la route, en remplaçant ce que
l'appelant a envoyé. À utiliser quand la valeur est l'affaire de la route et non
de l'appelant : un tenant, une clé d'API, un format imposé.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `name` | chaîne | oui | Le paramètre posé. |
| `value` | chaîne | oui | La valeur écrite. |

## Exemple

```yaml
filters:
  - type: set-query-param
    args:
      name: format
      value: json
```

## Notes

Remplace toute valeur envoyée par l'appelant, et ajoute le paramètre s'il était
absent. La valeur est encodée à la sortie : elle n'a pas besoin d'être échappée.

Pour conserver ce que l'appelant a envoyé et ajouter une valeur, utilisez
[add-query-param](/docs/filters/add-query-param).
