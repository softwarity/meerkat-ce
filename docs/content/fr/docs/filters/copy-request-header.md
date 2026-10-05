---
title: copy-request-header
section: Filtres
order: 66
summary: Copie un en-tête de requête sous un second nom, sans toucher à l'original.
---

# copy-request-header

Place la même valeur sous deux noms. C'est le filtre des migrations : un service
lit déjà le nouveau nom de l'en-tête, un autre a encore besoin de l'ancien, et la
route sert les deux.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `from` | chaîne | oui | L'en-tête lu. |
| `to` | chaîne | oui | L'en-tête écrit. |

## Exemple

```yaml
filters:
  - type: copy-request-header
    args:
      from: X-Request-Id
      to: X-Correlation-Id
```

## Notes

Toutes les valeurs sont copiées sous le second nom, et le premier est conservé. Il
ne se passe rien si `from` est absent.

L'en-tête cible est **remplacé**, pas complété : sinon, deux copies successives
empileraient la même valeur. Utilisez
[rename-request-header](/docs/filters/rename-request-header) quand l'ancien nom
doit disparaître.
