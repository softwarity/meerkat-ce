---
title: copy-request-header
section: Filtres
order: 66
summary: Copie un en-tête de requête sous un second nom, en laissant l'original en place.
---

# copy-request-header

Met la même valeur sous deux noms. C'est le filtre des migrations : un service lit
déjà le nouveau nom d'en-tête, un autre a encore besoin de l'ancien, et la route
nourrit les deux.

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

Toutes les valeurs sont copiées sous le second nom et le premier est conservé.
Rien ne se passe si `from` est absent.

La cible est **remplacée**, pas complétée : copier deux fois empilerait sinon la
même valeur. Utilisez
[rename-request-header](/docs/filters/rename-request-header) quand l'ancien nom
doit disparaître.
