---
title: time-window
section: Prédicats
order: 48
summary: Matche les requêtes faites dans une fenêtre de temps.
---

# time-window

Matche tant que l'horloge est dans la fenêtre. Ouvre ou ferme une route à une
date sans que personne ne veille : une migration qui commence lundi, une offre
qui se termine à minuit.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `from` | chaîne | non | Début de la fenêtre, au format RFC 3339, par exemple `2026-10-05T00:00:00+02:00`. Le décalage voyage avec la valeur. |
| `to` | chaîne | non | Fin de la fenêtre, au format RFC 3339. Doit être après `from`. |

Donnez `from`, `to`, ou les deux. Une fenêtre ouverte des deux côtés matche tous
les instants, ce que ne pas mettre de prédicat de temps fait déjà : elle est
refusée.

## Exemple

```yaml
predicates:
  - type: path
    args:
      patterns:
        - /promotions/**
  - type: time-window
    args:
      from: 2026-11-27T00:00:00+01:00
      to: 2026-12-01T00:00:00+01:00
```

## Notes

Hors de la fenêtre la route ne matche pas : c'est la suivante qui matche qui
répond - ou personne, et c'est un `404`. Placez en dessous la route qui répond le
reste du temps.

Les deux bornes sont exclusives : la fenêtre est ouverte strictement après `from`
et strictement avant `to`.

Une date qui n'est pas du `RFC 3339` est refusée à l'enregistrement de la route,
pas silencieusement au moment de la requête. Le décalage fait partie de la
valeur : `+01:00` et `Z` veulent dire ce qu'ils disent, où que tourne la gateway.

Le testeur de routage de la console sait épingler l'horloge : c'est ainsi qu'on
essaie une fenêtre à une date qui n'est pas arrivée.

## Référence

[RFC 3339](https://www.rfc-editor.org/rfc/rfc3339) - date et heure sur Internet.
