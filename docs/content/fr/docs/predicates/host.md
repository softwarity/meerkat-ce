---
title: host
section: Prédicats
order: 43
summary: Compare le Host de la requête à des noms exacts ou à des jokers *.suffixe.
---

# host

Compare le `Host` utilisé par l'appelant. C'est le prédicat d'une gateway qui
sert plusieurs sites, ou d'une même application publiée sous le nom d'un client.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `hosts` | liste de chaînes | oui | Des noms exacts ou des jokers `*.suffixe`, par exemple `shop.example.com`, `*.example.com`. Plusieurs valeurs se combinent par OU. |

## Exemple

```yaml
predicates:
  - type: host
    args:
      hosts:
        - shop.example.com
        - '*.shop.example.com'
  - type: path
    args:
      patterns:
        - /**
```

## Notes

Le port est ignoré : `shop.example.com` accepte `shop.example.com:8443`.

Un joker ne couvre **que les sous-domaines** : `*.example.com` accepte
`app.example.com`, mais pas `example.com`. Ajoutez le domaine nu à la liste si
vous voulez les deux.
