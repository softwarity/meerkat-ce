---
title: header
section: Prédicats
order: 42
summary: Accepte la requête quand un en-tête est présent, avec une liste de valeurs ou une expression régulière.
---

# header

Accepte la requête quand l'en-tête nommé est présent et, si vous le demandez,
quand sa valeur figure dans une liste ou respecte une forme. Servez-vous-en pour
répartir le trafic selon ce que l'appelant déclare : un nom de client, un
environnement, la forme d'une clé d'API.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `name` | chaîne | oui | L'en-tête recherché. |
| `values` | liste de chaînes | non | La valeur de l'en-tête doit être l'une d'elles, par exemple `staging`, `prod`. |
| `regexp` | chaîne | non | Expression régulière que la valeur entière doit respecter. À réserver à une forme, pas à une liste. |

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

Les noms d'en-tête sont insensibles à la casse, les valeurs ne le sont pas. Seule
la **première** valeur d'un en-tête répété est lue.
