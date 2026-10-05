---
title: cookie
section: Prédicats
order: 41
summary: Accepte la requête quand un cookie est présent, avec une liste de valeurs ou une expression régulière.
---

# cookie

Accepte la requête quand le cookie nommé est présent et, si vous le demandez,
quand sa valeur figure dans une liste ou respecte une forme. Un cookie dure toute
la session : c'est donc lui qui maintient une personne d'un seul côté d'un
déploiement canari, ou dans une bêta.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `name` | chaîne | oui | Le cookie recherché. |
| `values` | liste de chaînes | non | La valeur du cookie doit être l'une d'elles. |
| `regexp` | chaîne | non | Expression régulière que la valeur entière doit respecter. À réserver à une forme, pas à une liste. |

Donnez `values` ou `regexp`, pas les deux : une liste compare des valeurs
entières, une expression régulière décrit une forme, et deux réponses à la même
question, c'est une de trop. Sans l'un ni l'autre, la présence du cookie suffit.

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

Le cookie doit être présent. Un appelant qui ne l'a pas n'est pas retenu : la
route qui sert tous les autres se place donc **en dessous** de celle-ci.
