---
title: remove-request-header
section: Filtres
order: 77
summary: Retire un en-tête de requête avant de proxifier.
---

# remove-request-header

Empêche un en-tête d'atteindre le service : quelque chose que l'appelant n'a pas à
envoyer, ou un nom que l'application lirait comme une instruction.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `name` | chaîne | oui | L'en-tête retiré. |

## Exemple

```yaml
filters:
  - type: remove-request-header
    args:
      name: X-Internal-Debug
```

## Notes

Toutes les valeurs sous ce nom partent. Les noms d'en-tête ne sont pas sensibles à
la casse : `x-internal-debug` et `X-Internal-Debug` sont le même en-tête.

Pour retirer un cookie, utilisez
[remove-request-cookie](/docs/filters/remove-request-cookie) : les cookies
partagent tous un seul en-tête, et le supprimer emporterait la session.
