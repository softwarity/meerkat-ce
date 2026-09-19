---
title: remove-query-param
section: Filtres
order: 75
summary: Retire un paramètre de la requête proxifiée.
---

# remove-query-param

Retire un paramètre de la chaîne de requête avant que le service ne le voie : une
étiquette de suivi, un interrupteur de débogage que l'appelant ne doit pas pouvoir
activer.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `name` | chaîne | oui | Le paramètre retiré. |

## Exemple

```yaml
filters:
  - type: remove-query-param
    args:
      name: debug
```

## Notes

Toutes les valeurs de ce paramètre partent ; le reste de la chaîne de requête est
inchangé.

Rien ne se passe si le paramètre est absent.
