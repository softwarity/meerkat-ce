---
title: add-query-param
section: Filtres
order: 61
summary: Ajoute un paramètre de requête.
---

# add-query-param

Ajoute une valeur à la chaîne de requête envoyée à l'upstream, sans toucher à ce que
l'appelant a déjà envoyé sous ce nom. Servez-vous-en quand un service lit un
indicateur dans la chaîne de requête et que c'est à la route, et non à l'appelant,
d'en décider.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `name` | chaîne | oui | Le paramètre à ajouter. |
| `value` | chaîne | oui | La valeur ajoutée. |

## Exemple

```yaml
filters:
  - type: add-query-param
    args:
      name: source
      value: gateway
```

## Notes

La valeur s'ajoute à celles que l'appelant a déjà envoyées sous ce nom, qui sont
conservées. Pour les remplacer, utilisez
[set-query-param](/docs/filters/set-query-param).

La chaîne de requête est réencodée en sortie : inutile d'échapper vous-même la
valeur.
