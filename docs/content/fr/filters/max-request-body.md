---
title: max-request-body
section: Filtres
order: 69
summary: Refuse un corps de requête plus gros que cette taille, avec un 413.
---

# max-request-body

Borne ce qu'un appelant peut téléverser sur cette route. C'est une **gate** : elle
accepte ou refuse avant que tout autre filtre ne transforme quoi que ce soit, et
elle répond elle-même à l'appelant.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `size` | chaîne | oui | La borne, par exemple `2MB`, `512KB`, ou un nombre d'octets. |

Les unités sont `KB`, `MB`, `GB` (ou `K`, `M`, `G`, `B`) ; sans unité, ce sont des
octets. La taille doit être strictement positive - on lève une limite en retirant
le filtre.

## Exemple

```yaml
filters:
  - type: max-request-body
    args:
      size: 2MB
```

## Notes

Un corps dont la longueur est déclarée est refusé **sans rien lire** : l'appelant a
dit combien arrivait, et refuser sur parole coûte une comparaison au lieu d'un
mégaoctet de transfert. Un corps en chunked, qui ne déclare aucune longueur, est
coupé au fil de son arrivée.

Le refus est un `413` qui porte l'arithmétique : ce qui a été reçu et ce que la
route accepte. 'Trop gros' tout seul est l'erreur qui coûte un après-midi,
puisque la limite se trouve là où ni l'appelant ni le développeur de
l'application ne peuvent la voir.

Les connexions WebSocket ne sont jamais bornées : un socket n'a pas de corps, il a
une conversation, et ses octets passent par le même lecteur.
