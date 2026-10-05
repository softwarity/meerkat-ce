---
title: rename-response-header
section: Filtres
order: 80
summary: Déplace un en-tête de réponse sous un autre nom, avec toutes ses valeurs.
---

# rename-response-header

Renomme un en-tête de la réponse, pour un client qui ne lit pas le nom que le
service écrit.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `from` | chaîne | oui | L'en-tête lu, puis supprimé. |
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

Toutes les valeurs passent sous le nouveau nom et l'ancien disparaît. Ce qui se
trouvait déjà sous `to` est remplacé.

Il ne se passe rien si `from` est absent.
