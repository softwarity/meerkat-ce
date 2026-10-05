---
title: rename-request-header
section: Filtres
order: 79
summary: Déplace un en-tête de requête sous un autre nom, avec toutes ses valeurs.
---

# rename-request-header

L'appelant envoie `X-User` et le service attend `REMOTE_USER`. Ce filtre déplace
la valeur sous le nom que lit l'application.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `from` | chaîne | oui | L'en-tête lu, puis supprimé. |
| `to` | chaîne | oui | L'en-tête écrit. |

## Exemple

```yaml
filters:
  - type: rename-request-header
    args:
      from: X-User
      to: REMOTE_USER
```

## Notes

Toutes les valeurs sont déplacées et l'ancien nom disparaît. Ce qui se trouvait
déjà sous `to` est remplacé.

Il ne se passe rien si `from` est absent. Utilisez
[copy-request-header](/docs/filters/copy-request-header) quand l'ancien nom doit
subsister.
