---
title: rewrite-location
section: Filtres
order: 82
summary: Ramène dans l'espace public le Location d'une redirection émise par l'upstream.
---

# rewrite-location

Le service redirige vers sa propre adresse, et le navigateur quitte la gateway.
Ce filtre réécrit le `Location` pour qu'il pointe de nouveau vers le nom utilisé
par l'appelant : `http://billing.internal:8080/login` devient
`https://shop.example.com/login`.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `from` | chaîne | non | Le préfixe à remplacer. Vide : l'origine de l'upstream. |
| `to` | chaîne | non | Ce qui le remplace. Vide : l'origine utilisée par l'appelant. |

Sans aucun des deux, le filtre remplace l'origine de l'upstream par l'origine
publique, et c'est sa raison d'être. Renseignez-les pour réécrire aussi un préfixe
de chemin.

## Exemple

```yaml
filters:
  - type: rewrite-location
    args:
      from: http://billing.internal:8080/app
      to: https://shop.example.com/billing
```

## Notes

Seul un `Location` qui commence par `from` est réécrit ; tout autre est laissé tel
quel, de même qu'une réponse sans `Location`.

L'origine de l'upstream est tirée de la requête **telle qu'elle a réellement été
envoyée**, et non de la configuration de la route : une chaîne de redirections
aboutit ainsi à l'hôte qui a répondu.

L'origine publique est lue dans les en-têtes de transfert que la gateway écrit
elle-même (`X-Forwarded-Host`, `X-Forwarded-Proto`). S'ils manquent, il n'y a rien
de fiable vers quoi pointer, et l'en-tête reste en l'état : un `Location` réécrit
vers un hôte que personne n'a demandé est pire qu'un `Location` intact.
