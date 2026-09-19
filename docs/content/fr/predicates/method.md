---
title: method
section: Prédicats
order: 44
summary: Matche la méthode HTTP.
---

# method

Matche le verbe HTTP. Sert à envoyer les lectures et les écritures d'un même
chemin à deux endroits différents, ou à ne publier que les verbes qu'une
application doit recevoir.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `methods` | liste de chaînes | oui | Les verbes acceptés, par exemple `GET`, `POST`. Plusieurs se combinent par OU. |

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

Les verbes sont comparés en majuscules : `get` et `GET` sont la même chose ici.

Un verbe absent de la liste ne donne pas un `405` : la route ne matche pas, et
c'est la suivante qui matche qui répond - ou personne, et c'est un `404`.
Ajoutez une route qui répond au reste si le refus doit être explicite.
