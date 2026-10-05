---
title: add-request-header
section: Filtres
order: 62
summary: Ajoute une valeur à un en-tête de requête ; avec ifNotPresent, rien n'est ajouté si le client en a déjà envoyé une.
---

# add-request-header

Ajoute une valeur d'en-tête à la requête envoyée à l'upstream, en plus de celles qui
s'y trouvent déjà. Avec `ifNotPresent`, le filtre fournit une valeur par défaut :
la route ne renseigne l'en-tête que si l'appelant ne l'a pas fait.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `name` | chaîne | oui | L'en-tête à ajouter. |
| `value` | chaîne | oui | La valeur ajoutée. |
| `ifNotPresent` | booléen | non | N'ajoute rien si l'appelant a déjà envoyé cet en-tête. Par défaut : `false`. |

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

La valeur s'ajoute aux valeurs existantes. Avec `ifNotPresent`, rien n'est ajouté
si l'appelant a déjà envoyé cet en-tête.

Pour remplacer ce que l'appelant a envoyé, utilisez plutôt
[set-request-header](/docs/filters/set-request-header) : deux valeurs sur un
en-tête que le service croit unique, c'est un bug en sommeil, qui se réveille le
jour où le service lit l'autre.
