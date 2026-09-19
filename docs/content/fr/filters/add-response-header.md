---
title: add-response-header
section: Filtres
order: 63
summary: Ajoute une valeur d'en-tête de réponse.
---

# add-response-header

Ajoute une valeur d'en-tête à la réponse au retour, à côté de celles que le
service a envoyées. Son usage, ce sont les en-têtes qui sont des listes par
nature.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `name` | chaîne | oui | L'en-tête ajouté. |
| `value` | chaîne | oui | La valeur ajoutée. |

## Exemple

```yaml
filters:
  - type: add-response-header
    args:
      name: Vary
      value: Accept-Language
```

## Notes

La valeur est ajoutée à côté de celles du service. À réserver aux en-têtes
multivalués comme `Vary` ou `Set-Cookie` : deux valeurs sur un en-tête unique sont
ambiguës, et c'est le navigateur qui tranche.

Pour une décision unique, utilisez
[set-response-header](/#/docs/filters/set-response-header). Pour un en-tête arrivé
deux fois, utilisez
[dedupe-response-header](/#/docs/filters/dedupe-response-header).
