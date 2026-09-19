---
title: add-request-header
section: Filtres
order: 62
summary: Ajoute une valeur d'en-tête de requête ; ifNotPresent n'ajoute rien si le client en a déjà envoyé une.
---

# add-request-header

Ajoute une valeur d'en-tête à la requête envoyée à l'amont, à côté de celles qui
sont déjà là. Avec `ifNotPresent` cela devient un défaut : la route ne remplit
l'en-tête que si l'appelant ne l'a pas fait.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `name` | chaîne | oui | L'en-tête ajouté. |
| `value` | chaîne | oui | La valeur ajoutée. |
| `ifNotPresent` | booléen | non | N'ajoute rien si l'appelant a déjà envoyé cet en-tête. Défaut : `false`. |

## Exemple

```yaml
filters:
  - type: add-request-header
    args:
      name: Accept-Language
      value: fr-FR
      ifNotPresent: true
```

## Notes

La valeur est ajoutée à côté des valeurs existantes. Avec `ifNotPresent`, rien
n'est ajouté quand l'appelant a déjà envoyé cet en-tête.

Pour remplacer ce que l'appelant a envoyé, utilisez
[set-request-header](/docs/filters/set-request-header) : deux valeurs sur un
en-tête que le service lit comme unique, c'est un bug qui attend le jour où le
service choisira l'autre.
