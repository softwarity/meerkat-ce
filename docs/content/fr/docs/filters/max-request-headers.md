---
title: max-request-headers
section: Filtres
order: 70
summary: Refuse une requête dont les en-têtes pèsent plus que cette taille, avec un 431.
---

# max-request-headers

Borne le bloc d'en-têtes, pour un appelant qui envoie des centaines de cookies ou
de rôles. Comme [max-request-body](/docs/filters/max-request-body), c'est une
**gate** : elle décide avant toute transformation et répond elle-même à
l'appelant.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `size` | chaîne | oui | La borne, par exemple `16KB`, ou un nombre d'octets. |

Les unités sont `KB`, `MB`, `GB` (ou `K`, `M`, `G`, `B`) ; sans unité, ce sont des
octets. La taille doit être strictement positive.

## Exemple

```yaml
filters:
  - type: max-request-headers
    args:
      size: 16KB
```

## Notes

Le refus est un `431`, qui dit à l'appelant quelle moitié réduire, et il porte
l'arithmétique : ce qui est arrivé et ce que la route accepte.

Ce qui est pesé, c'est la ligne de requête plus chaque en-tête tel qu'il s'écrirait
sur le fil, `Host` compris - de la même façon que le serveur Go le pèse
lui-même. Ce que la gateway ajoute ensuite, en-têtes de transfert ou d'identité,
n'est pas compté.
