---
title: rename-response-header
section: Filtres
order: 80
summary: Déplace un en-tête de réponse sous un autre nom, valeurs comprises.
---

# rename-response-header

Renomme un en-tête de la réponse, pour un client qui lit un autre nom que celui
que le service écrit.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `from` | chaîne | oui | L'en-tête lu, puis retiré. |
| `to` | chaîne | oui | L'en-tête écrit. |

## Exemple

```yaml
filters:
  - type: rename-response-header
    args:
      from: X-Request-Id
      to: X-Correlation-Id
```

## Notes

Toutes les valeurs sont déplacées sous le nouveau nom et l'ancien disparaît. Ce qui
se trouvait déjà sous `to` est remplacé.

Rien ne se passe si `from` est absent.
