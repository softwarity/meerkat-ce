---
title: set-path
section: Filtres
order: 88
summary: Remplace tout le chemin envoyé à l'amont.
---

# set-path

Envoie tout ce que la route matche vers un seul chemin fixe, typiquement un
endpoint de santé publié sous un nom plus accueillant.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `path` | chaîne | oui | Le chemin envoyé à l'amont. Absolu, par exemple `/health`. |

Un chemin qui ne commence pas par `/` est refusé à l'enregistrement de la route.

## Exemple

```yaml
filters:
  - type: set-path
    args:
      path: /actuator/health
```

## Notes

Tout le chemin est remplacé, quoi que l'appelant ait demandé : une route qui porte
ce filtre a exactement une destination.

La chaîne de requête n'est pas touchée. Utilisez
[remove-query-param](/#/docs/filters/remove-query-param) si elle ne doit pas
voyager.
