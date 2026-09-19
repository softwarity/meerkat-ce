---
title: rename-request-header
section: Filtres
order: 79
summary: Déplace un en-tête de requête sous un autre nom, valeurs comprises.
---

# rename-request-header

L'appelant envoie `X-User` et le service attend `REMOTE_USER`. Déplace la valeur
sous le nom que l'application lit.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `from` | chaîne | oui | L'en-tête lu, puis retiré. |
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

Rien ne se passe si `from` est absent. Utilisez
[copy-request-header](/#/docs/filters/copy-request-header) quand l'ancien nom doit
survivre.
