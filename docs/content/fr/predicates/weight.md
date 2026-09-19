---
title: weight
section: Prédicats
order: 50
summary: Répartit le trafic entre les routes d'un groupe (canari) : une route prend poids/total des requêtes.
---

# weight

Répartit le trafic d'un groupe de routes par parts. C'est le canari : le même
chemin, deux amonts, l'essentiel du trafic sur la version en production et un peu
sur la nouvelle.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `group` | chaîne | oui | Le nom que partagent les routes d'une même répartition. |
| `weight` | entier | oui | La part de cette route dans le groupe. Doit être strictement positif. |

Une route prend `weight` divisé par le **total du groupe** des requêtes. Ce sont
des parts, pas des pourcentages : `8` et `2` donnent la même répartition que `80`
et `20`.

## Exemple

```yaml
routes:
  - id: checkout
    name: Checkout
    enabled: true
    order: 100
    upstream: http://checkout-1.internal:8080
    predicates:
      - type: path
        args:
          patterns:
            - /checkout/**
      - type: weight
        args:
          group: checkout
          weight: 8
  - id: checkout-next
    name: Checkout (next)
    enabled: true
    order: 110
    upstream: http://checkout-2.internal:8080
    predicates:
      - type: path
        args:
          patterns:
            - /checkout/**
      - type: weight
        args:
          group: checkout
          weight: 2
```

Une requête sur cinq part vers la nouvelle version.

## Notes

Le tirage a lieu **à chaque requête** : une même personne voit donc les deux
versions au cours d'une session. Ajoutez un prédicat
[cookie](/#/docs/predicates/cookie) sur une route du groupe quand quelqu'un doit
rester d'un seul côté.

Les parts sont calculées sur les routes réellement chargées : **désactiver** une
route du groupe donne sa part aux autres au lieu de perdre ce trafic.

Les autres prédicats s'appliquent toujours : une route ne prend sa part que des
requêtes que son chemin, son hôte et sa méthode acceptaient déjà.
