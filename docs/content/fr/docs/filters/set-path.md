---
title: set-path
section: Filtres
order: 88
summary: Remplace en entier le chemin envoyé à l'upstream.
---

# set-path

Envoie tout ce que la route capte vers un seul chemin fixe - le plus souvent, un
endpoint de santé publié sous un nom plus parlant.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `path` | chaîne | oui | Le chemin envoyé à l'upstream. Absolu, par exemple `/health`. |

Un chemin qui ne commence pas par `/` est refusé à l'enregistrement de la route.

## Exemple

```yaml
filters:
  - type: set-path
    args:
      path: /actuator/health
```

## Notes

Le chemin est remplacé en entier, quoi que l'appelant ait demandé : une route qui
porte ce filtre n'a qu'une seule destination.

La chaîne de requête n'est pas modifiée. Utilisez
[remove-query-param](/docs/filters/remove-query-param) si elle ne doit pas
suivre.
