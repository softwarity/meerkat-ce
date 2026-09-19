---
title: add-query-param
section: Filtres
order: 61
summary: Ajoute un paramètre de requête.
---

# add-query-param

Ajoute une valeur à la chaîne de requête envoyée à l'amont, à côté de ce que
l'appelant a déjà envoyé sous ce nom. À utiliser quand un service lit un drapeau
dans la requête et que c'est la route, pas l'appelant, qui doit le décider.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `name` | chaîne | oui | Le paramètre ajouté. |
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

La valeur est ajoutée et ce que l'appelant avait envoyé sous ce nom est conservé.
Utilisez [set-query-param](/docs/filters/set-query-param) pour la remplacer.

La chaîne de requête est réencodée à la sortie : la valeur n'a pas besoin d'être
échappée.
