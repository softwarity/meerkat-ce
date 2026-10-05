---
title: query
section: Prédicats
order: 46
summary: Accepte la requête quand un paramètre de requête est présent, avec une liste de valeurs ou une expression régulière.
---

# query

Accepte la requête quand le paramètre de requête nommé est présent et, si vous le
demandez, quand sa valeur figure dans une liste ou respecte une forme. Utile pour
un aiguillage que l'appelant peut glisser dans un lien, comme un aperçu ou un
canal.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `name` | chaîne | oui | Le paramètre recherché. |
| `values` | liste de chaînes | non | La valeur du paramètre doit être l'une d'elles. |
| `regexp` | chaîne | non | Expression régulière que la valeur entière doit respecter. À réserver à une forme, pas à une liste. |

Donnez `values` ou `regexp`, pas les deux. Sans l'un ni l'autre, la présence du
paramètre suffit.

## Exemple

```yaml
predicates:
  - type: path
    args:
      patterns:
        - /catalogue/**
  - type: query
    args:
      name: channel
      values:
        - mobile
        - tablet
```

## Notes

La présence compte même avec une valeur vide : `?preview` et `?preview=`
satisfont tous deux un prédicat `query` qui nomme `preview` sans donner de valeur.

Quand le paramètre apparaît plusieurs fois, seule la **première** valeur est lue.
