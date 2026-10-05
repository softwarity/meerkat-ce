---
title: set-query-param
section: Filtres
order: 89
summary: Fixe un paramètre de requête sur la requête envoyée à l'upstream.
---

# set-query-param

Fixe un paramètre de requête au niveau de la route, en remplaçant ce que
l'appelant a envoyé. Servez-vous-en quand la valeur relève de la route et non de
l'appelant - une organisation, une clé d'API, un format imposé.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `name` | chaîne | oui | Le paramètre fixé. |
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

Le filtre remplace toute valeur envoyée par l'appelant, et ajoute le paramètre
s'il était absent. La valeur est encodée en sortie : inutile de l'échapper
vous-même.

Pour conserver ce que l'appelant a envoyé et ajouter une autre valeur, utilisez
[add-query-param](/docs/filters/add-query-param).
