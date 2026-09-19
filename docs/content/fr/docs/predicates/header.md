---
title: header
section: Prédicats
order: 42
summary: Matche quand un en-tête est présent, contre une liste de valeurs ou une regexp.
---

# header

Matche quand l'en-tête nommé est présent, et éventuellement quand sa valeur est
dans une liste ou a une certaine forme. Sert à séparer le trafic sur ce que
l'appelant déclare : un nom de client, un environnement, la forme d'une clé.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `name` | chaîne | oui | L'en-tête cherché. |
| `values` | liste de chaînes | non | La valeur doit être l'une d'elles, par exemple `staging`, `prod`. |
| `regexp` | chaîne | non | Regexp ancrée sur la valeur entière. Pour une forme, pas pour une liste. |

Donnez `values` ou `regexp`, pas les deux. Sans l'un ni l'autre, la présence de
l'en-tête suffit.

## Exemple

```yaml
predicates:
  - type: path
    args:
      patterns:
        - /orders/**
  - type: header
    args:
      name: X-Client
      values:
        - mobile-app
```

## Notes

Les noms d'en-tête ne sont pas sensibles à la casse, les valeurs le sont. Seule
la **première** valeur d'un en-tête répété est lue.
