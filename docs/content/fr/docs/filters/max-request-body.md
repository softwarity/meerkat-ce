---
title: max-request-body
section: Filtres
order: 69
summary: Refuse, avec un 413, une requête dont le corps dépasse cette taille.
---

# max-request-body

Plafonne ce qu'un appelant peut envoyer sur cette route. C'est une **gate** : elle
accepte ou refuse avant qu'un autre filtre ait transformé quoi que ce soit, et
elle répond elle-même à l'appelant.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `size` | chaîne | oui | Le plafond, par exemple `2MB`, `512KB`, ou un nombre d'octets. |

Les unités sont `KB`, `MB`, `GB` (ou `K`, `M`, `G`, `B`) ; sans unité, la valeur
est en octets. La taille doit être supérieure à zéro : pour lever une limite, on
retire le filtre.

## Exemple

```yaml
filters:
  - type: max-request-body
    args:
      size: 2MB
```

## Notes

Un corps dont la longueur est déclarée est refusé **sans rien lire** : l'appelant
a annoncé ce qu'il allait envoyer, et le refuser sur parole coûte une comparaison
au lieu d'un mégaoctet de transfert. Un corps envoyé par morceaux (chunked), qui
ne déclare aucune longueur, est coupé au fil de son arrivée.

Le refus est un `413` qui donne les chiffres : ce qui a été reçu et ce que cette
route accepte. Un simple "trop volumineux" est l'erreur qui fait perdre un
après-midi, car la limite se trouve là où ni l'appelant ni le développeur de
l'application ne peuvent la voir.

Les connexions WebSocket ne sont jamais plafonnées : un socket n'a pas de corps,
il porte une conversation, et ses octets passent par le même lecteur.
