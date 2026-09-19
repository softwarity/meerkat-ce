---
title: rewrite-query-param
section: Filtres
order: 84
summary: Réécrit la valeur d'un paramètre de requête avec un remplacement par regexp.
---

# rewrite-query-param

Nettoie un paramètre que le service ne doit pas recevoir tel qu'il a été envoyé.
L'exemple qui compte est une URL de retour : `?redirect=https://evil.test/back`
qui arrive au service en `?redirect=/back`.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `name` | chaîne | oui | Le paramètre réécrit. |
| `pattern` | chaîne | oui | La regexp confrontée à la valeur. |
| `replacement` | chaîne | non | Ce qui la remplace. Vide retire ce qui a matché. Les captures s'écrivent `$1`, `$2`. |

Un motif qui ne compile pas est refusé à l'enregistrement de la route.

## Exemple

```yaml
filters:
  - type: rewrite-query-param
    args:
      name: redirect
      pattern: ^https?://[^/]+
      replacement: ''
```

## Notes

Toutes les valeurs du paramètre sont réécrites quand il apparaît plusieurs fois.
Rien ne se passe si le paramètre est absent.

La syntaxe est celle de RE2, la bibliothèque de Go : pas de rétroréférence, pas de
lookaround.
