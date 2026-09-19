---
title: set-host
section: Filtres
order: 87
summary: Fixe l'hôte envoyé à l'amont.
---

# set-host

Une machine sert plusieurs sites et choisit par le nom. C'est le nom qu'elle
reçoit, quelle que soit l'adresse de l'amont.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `host` | chaîne | oui | L'`Host` envoyé à l'amont, par exemple `billing.internal`. |

## Exemple

```yaml
filters:
  - type: set-host
    args:
      host: billing.internal
```

## Notes

L'en-tête `Host` et la valeur que Go met sur le fil sont posés tous les deux. N'en
poser qu'un les met en désaccord, et c'est le bug d'hôte virtuel qui coûte un
après-midi.

Pour le nom de l'appelant plutôt qu'un nom fixe, utilisez
[preserve-host](/#/docs/filters/preserve-host). Ne posez pas les deux sur la même
route : c'est le dernier écrit qui gagne, et personne ne lit un tirage au sort dans
une liste de filtres.
