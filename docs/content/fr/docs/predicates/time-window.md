---
title: time-window
section: Prédicats
order: 48
summary: Accepte les requêtes qui arrivent dans une fenêtre de temps.
---

# time-window

Accepte la requête tant que l'horloge se trouve dans la fenêtre. Ce prédicat ouvre
ou ferme une route à une date donnée, sans que personne ait à veiller : une
migration qui démarre lundi, une offre qui se termine à minuit.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `from` | chaîne | non | Début de la fenêtre, au format RFC 3339, par exemple `2026-10-05T00:00:00+02:00`. Le décalage horaire fait partie de la valeur. |
| `to` | chaîne | non | Fin de la fenêtre, au format RFC 3339. Doit être postérieure à `from`. |

Donnez `from`, `to`, ou les deux. Une fenêtre ouverte des deux côtés accepte tous
les instants, ce que fait déjà l'absence de prédicat de temps : elle est donc
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

Hors de la fenêtre, la route n'est pas retenue : c'est la suivante qui convient
qui répond - ou aucune, et c'est alors un `404`. Placez en dessous la route qui
répond le reste du temps.

Les deux bornes sont exclues : la fenêtre est ouverte strictement après `from` et
strictement avant `to`.

Une date qui n'est pas au format `RFC 3339` est refusée à l'enregistrement de la
route, et non ignorée en silence au moment de la requête. Le décalage horaire fait
partie de la valeur : `+01:00` et `Z` gardent donc leur sens, où que tourne la
gateway.

Le testeur de routes de la console permet de figer l'horloge : c'est ainsi qu'on
essaie une fenêtre à une date qui n'est pas encore arrivée.

## Référence

[RFC 3339](https://www.rfc-editor.org/rfc/rfc3339) - la date et l'heure sur
Internet.
