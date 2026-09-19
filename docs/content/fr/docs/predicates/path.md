---
title: path
section: Prédicats
order: 45
summary: Matche le chemin de la requête contre un ou plusieurs motifs.
---

# path

Matche le chemin de la requête contre un ou plusieurs motifs. C'est le prédicat
par lequel presque toute route commence : le chemin est ce qui distingue une
application d'une autre.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `patterns` | liste de chaînes | oui | Les motifs contre lesquels le chemin est essayé, par exemple `/api/users/{id}`, `/static/**`. Plusieurs se combinent par OU. |

## Exemple

```yaml
predicates:
  - type: path
    args:
      patterns:
        - /orders/**
        - /invoices/**
```

## Notes

Un motif doit commencer par `/` et se compare **segment par segment** :

- `{nom}` prend exactement un segment, quel qu'il soit : `/orders/{id}` matche `/orders/8814` et pas `/orders/8814/lines`.
- `**` prend tout le reste du chemin, et n'est autorisé qu'en **dernier segment**. `/orders/**` matche `/orders`, `/orders/8814` et `/orders/8814/lines`.
- tout le reste est un segment littéral.

Les frontières de segment sont respectées : `/demo/**` matche `/demo` et
`/demo/x`, jamais `/demolition`. Un slash final unique est ignoré, donc
`/orders/` et `/orders` sont la même route.

> [!WARNING]
> Une étoile seule n'est **pas** un joker. `/orders/*` matche le chemin littéral
> `/orders/*` et rien d'autre. Écrivez `{id}` pour un segment et `**` pour une
> queue de chemin.

`{nom}` matche un segment mais ne capture rien de réutilisable : aucun filtre ne
peut relire la valeur. Passez par
[rewrite-path](/docs/filters/rewrite-path) quand elle doit être déplacée.
