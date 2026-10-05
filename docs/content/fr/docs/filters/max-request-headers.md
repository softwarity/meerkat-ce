---
title: max-request-headers
section: Filtres
order: 70
summary: Refuse, avec un 431, une requête dont les en-têtes pèsent plus que cette taille.
---

# max-request-headers

Plafonne le bloc d'en-têtes, pour l'appelant qui envoie des centaines de cookies
ou de rôles. Comme [max-request-body](/docs/filters/max-request-body), c'est une
**gate** : elle décide avant toute transformation et répond elle-même à
l'appelant.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `size` | chaîne | oui | Le plafond, par exemple `16KB`, ou un nombre d'octets. |

Les unités sont `KB`, `MB`, `GB` (ou `K`, `M`, `G`, `B`) ; sans unité, la valeur
est en octets. La taille doit être supérieure à zéro.

## Exemple

```yaml
filters:
  - type: max-request-headers
    args:
      size: 16KB
```

## Notes

Le refus est un `431`, qui indique à l'appelant quelle moitié de sa requête
réduire, et il donne les chiffres : ce qui est arrivé et ce que cette route
accepte.

Ce qui est pesé, c'est la ligne de requête plus chaque en-tête tel qu'il serait
écrit sur le réseau, `Host` compris - le même calcul que celui du serveur de Go.
Ce que la gateway ajoute ensuite, comme les en-têtes de transfert ou
d'identité, n'est pas compté.
