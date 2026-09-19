---
title: strip-prefix
section: Filtres
order: 93
summary: Retire les N premiers segments du chemin avant de proxifier.
---

# strip-prefix

La gateway publie `/demo/orders`, le service ne connaît que `/orders`. Le filtre le
plus utilisé du catalogue : c'est lui qui permet de monter une application sous un
chemin de votre choix sans que l'application le sache.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `parts` | entier | non | Nombre de segments de tête retirés. Défaut : `1`. Au moins `1`. |
| `announcePrefix` | booléen | non | Dire au service où il est publié, en `X-Forwarded-Prefix`. Défaut : `true`. |

## Exemple

```yaml
predicates:
  - type: path
    args:
      patterns:
        - /demo/**
filters:
  - type: strip-prefix
    args:
      parts: 1
```

`/demo/orders/8814` arrive au service en `/orders/8814`, avec
`X-Forwarded-Prefix: /demo`.

## Notes

`X-Forwarded-Prefix` est ce par quoi un service qui **construit** ses liens
apprend où il vit. Sans lui, une application qui voit `/orders` écrit `/orders`, le
navigateur suit, et ça atterrit hors de la route. Spring lit cet en-tête par son
`ForwardedHeaderFilter`, nginx le pose ; un service qui ne fait que répondre n'en a
pas besoin.

Ce qu'un appelant envoie sous ce nom est purgé, sur toutes les routes, même celles
sans `strip-prefix` : un appelant qui poserait son propre préfixe ferait écrire au
service des liens là où il l'a demandé.

Deux `strip-prefix` à la suite fonctionnent : le préfixe annoncé s'élargit à ce qui
a réellement été consommé au lieu d'être écrasé.

Une application qui construit des liens absolus veut en général aussi
[preserve-host](/#/docs/filters/preserve-host) : le préfixe dit où, l'hôte dit sous
quel nom.
