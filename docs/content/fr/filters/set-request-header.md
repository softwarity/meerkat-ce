---
title: set-request-header
section: Filtres
order: 90
summary: Fixe un en-tête de requête (en remplaçant la valeur du client).
---

# set-request-header

Décide un en-tête au niveau de la route. C'est le filtre de ce qu'un service doit
apprendre et qu'un appelant ne doit pas pouvoir prétendre.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `name` | chaîne | oui | L'en-tête posé. |
| `value` | chaîne | oui | La valeur écrite. |

## Exemple

```yaml
filters:
  - type: set-request-header
    args:
      name: X-Tenant
      value: northwind
```

## Notes

Toutes les valeurs envoyées par l'appelant sous ce nom sont remplacées.

La valeur est **fixe** : rien n'est pris dans le chemin, la requête ou l'appelant.
Pour l'utilisateur connecté, passez par le transfert d'identité de la route plutôt
que par ce filtre - il tourne après les filtres et récrit les en-têtes qu'il écrit
lui-même, donc un `set-request-header` sur un de ces noms n'a aucun effet.

Une référence au coffre-fort (`$nom`) est résolue ici : c'est ainsi qu'une clé
d'API partagée atteint un service sans être écrite dans la configuration exportée.
