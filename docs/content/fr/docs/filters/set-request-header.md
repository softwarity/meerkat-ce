---
title: set-request-header
section: Filtres
order: 90
summary: Fixe un en-tête de requête, en remplaçant toute valeur envoyée par le client.
---

# set-request-header

Fixe un en-tête au niveau de la route. C'est le filtre de ce qu'un service doit
apprendre et qu'un appelant ne doit pas pouvoir prétendre.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `name` | chaîne | oui | L'en-tête fixé. |
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

Toutes les valeurs que l'appelant a envoyées sous ce nom sont remplacées.

La valeur est **fixe** : rien n'est repris du chemin, de la chaîne de requête ni
de l'appelant. Pour l'utilisateur connecté, passez par le transfert d'identité de
la route plutôt que par ce filtre : il s'exécute après les filtres et écrase les
en-têtes qu'il écrit lui-même, si bien qu'un `set-request-header` sur l'un de ces
noms reste sans effet.

Une référence au coffre (`$name`) est résolue ici : c'est ainsi qu'une clé d'API
partagée parvient à un service sans être écrite dans la configuration exportée.
