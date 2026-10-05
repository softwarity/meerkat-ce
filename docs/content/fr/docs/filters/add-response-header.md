---
title: add-response-header
section: Filtres
order: 63
summary: Ajoute une valeur à un en-tête de réponse.
---

# add-response-header

Ajoute une valeur d'en-tête à la réponse sur le chemin du retour, en plus de
celles que le service a envoyées. Il est fait pour les en-têtes qui sont des
listes par nature.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `name` | chaîne | oui | L'en-tête à ajouter. |
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

La valeur s'ajoute à celles que le service a envoyées. Réservez ce filtre aux
en-têtes à valeurs multiples, comme `Vary` ou `Set-Cookie` : deux valeurs sur un
en-tête à valeur unique sont ambiguës, et c'est le navigateur qui choisit laquelle
l'emporte.

Pour imposer une seule valeur, utilisez
[set-response-header](/docs/filters/set-response-header). Pour un en-tête arrivé
en double, utilisez
[dedupe-response-header](/docs/filters/dedupe-response-header).
