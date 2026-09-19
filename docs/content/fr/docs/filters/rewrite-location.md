---
title: rewrite-location
section: Filtres
order: 82
summary: Ramène le Location d'une redirection d'amont dans l'espace public.
---

# rewrite-location

Le service redirige vers sa propre adresse et le navigateur quitte la gateway. Ce
filtre réécrit le `Location` pour qu'il pointe vers le nom que l'appelant a
utilisé : `http://billing.internal:8080/login` devient
`https://shop.example.com/login`.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `from` | chaîne | non | Le préfixe à remplacer. Vide signifie l'origine de l'amont. |
| `to` | chaîne | non | Ce qui le remplace. Vide signifie l'origine utilisée par l'appelant. |

Sans l'un ni l'autre, le filtre échange l'origine de l'amont contre l'origine
publique, ce qui est son usage. Renseignez-les pour réécrire aussi un préfixe de
chemin.

## Exemple

```yaml
filters:
  - type: rewrite-location
    args:
      from: http://billing.internal:8080/app
      to: https://shop.example.com/billing
```

## Notes

Seul un `Location` qui commence par `from` est réécrit ; le reste est laissé
tranquille, comme une réponse qui ne porte pas de `Location`.

L'origine de l'amont est prise sur la requête **telle qu'elle a réellement été
envoyée**, et non telle que la route est configurée : une chaîne de redirections
atterrit donc sur l'hôte qui a répondu.

L'origine publique est lue dans les en-têtes de transfert que la gateway pose
elle-même (`X-Forwarded-Host`, `X-Forwarded-Proto`). Quand ils manquent, il n'y a
rien de fiable vers quoi pointer et l'en-tête est laissé tel quel : un `Location`
réécrit vers un hôte que personne n'a demandé est pire qu'un `Location` intact.
