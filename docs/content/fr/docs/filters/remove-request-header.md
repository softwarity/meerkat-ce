---
title: remove-request-header
section: Filtres
order: 77
summary: Retire un en-tête de la requête avant l'envoi à l'upstream.
---

# remove-request-header

Empêche un en-tête d'atteindre le service - un en-tête que l'appelant n'a pas à
envoyer, ou un nom que l'application lirait comme une consigne.

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

Toutes les valeurs portées par ce nom disparaissent. Les noms d'en-tête sont
insensibles à la casse : `x-internal-debug` et `X-Internal-Debug` désignent le
même en-tête.

Pour retirer un cookie, utilisez
[remove-request-cookie](/docs/filters/remove-request-cookie) : tous les cookies
partagent un seul en-tête, et le supprimer emporterait la session avec lui.
