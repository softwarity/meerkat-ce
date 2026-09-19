---
title: remove-response-header
section: Filtres
order: 78
summary: Retire un en-tête de réponse avant qu'il n'atteigne le client.
---

# remove-response-header

Retire un en-tête de la réponse. Son usage quotidien, c'est ce qu'un service dit
de lui-même : son framework, sa version, le nom de la machine qui a répondu.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `name` | chaîne | oui | L'en-tête retiré. |

## Exemple

```yaml
filters:
  - type: remove-response-header
    args:
      name: X-Powered-By
```

## Notes

Toutes les valeurs sous ce nom partent avant que la réponse n'atteigne le client.

Quand c'est la valeur qui pose problème plutôt que l'en-tête, utilisez
[rewrite-response-header](/#/docs/filters/rewrite-response-header) : retirer
`Server` en dit autant à un lecteur attentif que de le laisser.
