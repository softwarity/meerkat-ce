---
title: set-status
section: Filtres
order: 92
summary: Remplace le code de statut de la réponse de l'amont.
---

# set-status

Remplace le code de statut de la réponse. Son usage honnête est un service qui
répond le mauvais code pour un cas qu'il a bien traité, et qu'on ne peut pas
modifier aujourd'hui.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `status` | entier | oui | Le statut envoyé au client, entre `100` et `599`. |

Un statut hors de cette plage est refusé à l'enregistrement de la route.

## Exemple

```yaml
filters:
  - type: set-status
    args:
      status: 404
```

## Notes

Le corps n'est pas touché : un `200` posé sur une page d'erreur cache donc l'erreur
à tout ce qui ne lit que le code - supervision comprise, et métriques de la route
avec elle.

Les métriques de la route et son disjoncteur lisent tous les deux le statut qui
atteint le client, pas celui que le service a envoyé. Un `200` posé sur un `502`
leur cache donc aussi la panne : le disjoncteur compte une réussite et ne s'ouvre
jamais.
