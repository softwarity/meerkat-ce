---
title: method
section: Prédicats
order: 44
summary: Compare la méthode HTTP.
---

# method

Compare le verbe HTTP. Servez-vous-en pour envoyer les lectures et les écritures
d'un même chemin vers deux destinations différentes, ou pour ne publier que les
verbes qu'une application doit recevoir.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `methods` | liste de chaînes | oui | Les verbes acceptés, par exemple `GET`, `POST`. Plusieurs valeurs se combinent par OU. |

## Exemple

```yaml
predicates:
  - type: path
    args:
      patterns:
        - /catalogue/**
  - type: method
    args:
      methods:
        - GET
        - HEAD
```

## Notes

Les verbes sont comparés en majuscules : `get` et `GET` reviennent donc au même
ici.

Un verbe absent de la liste ne reçoit pas de `405` : la route n'est simplement pas
retenue, et c'est la suivante qui convient qui répond - ou aucune, et c'est alors
un `404`. Ajoutez une route qui répond au reste si le refus doit être explicite.
