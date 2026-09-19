---
title: query
section: Prédicats
order: 46
summary: Matche quand un paramètre de requête est présent, contre une liste de valeurs ou une regexp.
---

# query

Matche quand le paramètre de requête nommé est présent, et éventuellement quand
sa valeur est dans une liste ou a une certaine forme. Pratique pour un
interrupteur que l'appelant peut poser dans un lien : un aperçu, un canal.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `name` | chaîne | oui | Le paramètre cherché. |
| `values` | liste de chaînes | non | La valeur du paramètre doit être l'une d'elles. |
| `regexp` | chaîne | non | Regexp ancrée sur la valeur entière. Pour une forme, pas pour une liste. |

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

La présence compte même avec une valeur vide : `?preview` et `?preview=` matchent
tous les deux un prédicat `query` qui nomme `preview` sans valeur.

Seule la **première** valeur est lue quand le paramètre apparaît plusieurs fois.
