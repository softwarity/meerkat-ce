---
title: host
section: Prédicats
order: 43
summary: Matche l'hôte de la requête contre des noms exacts ou des jokers *.suffixe.
---

# host

Matche l'`Host` utilisé par l'appelant. C'est le prédicat d'une gateway qui sert
plusieurs sites, ou de la même application publiée sous le nom d'un client.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `hosts` | liste de chaînes | oui | Noms exacts ou jokers `*.suffixe`, par exemple `shop.example.com`, `*.example.com`. Plusieurs se combinent par OU. |

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

Le port est ignoré : `shop.example.com` matche `shop.example.com:8443`.

Un joker ne couvre que les **sous-domaines** : `*.example.com` matche
`app.example.com`, pas `example.com`. Nommez aussi le domaine nu si vous voulez
les deux.
