---
title: cookie
section: Prédicats
order: 41
summary: Matche quand un cookie est présent, contre une liste de valeurs ou une regexp.
---

# cookie

Matche quand le cookie nommé est présent, et éventuellement quand sa valeur est
dans une liste ou a une certaine forme. Un cookie dure toute la session : c'est
lui qui épingle une personne d'un seul côté d'un canari, ou sur une beta.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `name` | chaîne | oui | Le cookie cherché. |
| `values` | liste de chaînes | non | La valeur du cookie doit être l'une d'elles. |
| `regexp` | chaîne | non | Regexp ancrée sur la valeur entière. Pour une forme, pas pour une liste. |

Donnez `values` ou `regexp`, pas les deux : une liste matche des valeurs
entières, une regexp matche une forme, et deux réponses à la même question en
font une de trop. Sans l'un ni l'autre, la présence du cookie suffit.

## Exemple

```yaml
predicates:
  - type: path
    args:
      patterns:
        - /checkout/**
  - type: cookie
    args:
      name: checkout_variant
      values:
        - new
```

## Notes

Le cookie doit être là. Un appelant qui ne l'a pas ne matche pas : la route qui
sert tous les autres se place donc **en dessous** de celle-ci.
