---
title: set-host
section: Filtres
order: 87
summary: Fixe le Host envoyé à l'upstream.
---

# set-host

Une même machine sert plusieurs sites et choisit d'après le nom. Ce filtre fixe
le nom qu'elle reçoit, quelle que soit l'adresse de l'upstream.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `host` | chaîne | oui | Le `Host` envoyé à l'upstream, par exemple `billing.internal`. |

## Exemple

```yaml
filters:
  - type: set-host
    args:
      host: billing.internal
```

## Notes

Le filtre fixe à la fois l'en-tête `Host` et la valeur que Go écrit sur le réseau.
N'en fixer qu'un met les deux en désaccord, et c'est ce bug d'hôte virtuel qui
fait perdre un après-midi.

Pour envoyer le nom utilisé par l'appelant plutôt qu'un nom fixe, utilisez
[preserve-host](/docs/filters/preserve-host). Ne mettez pas les deux sur la même
route : c'est le dernier écrit qui l'emporte, un tirage à pile ou face que
personne ne remarque dans une liste de filtres.
