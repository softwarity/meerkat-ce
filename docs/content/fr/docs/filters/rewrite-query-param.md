---
title: rewrite-query-param
section: Filtres
order: 84
summary: Réécrit la valeur d'un paramètre de requête par un remplacement à expression régulière.
---

# rewrite-query-param

Nettoie un paramètre que le service ne doit pas recevoir tel qu'il a été envoyé.
L'exemple qui compte est celui de l'URL de retour :
`?redirect=https://evil.test/back` arrive au service sous la forme
`?redirect=/back`.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `name` | chaîne | oui | Le paramètre réécrit. |
| `pattern` | chaîne | oui | L'expression régulière appliquée à la valeur. |
| `replacement` | chaîne | non | Ce qui remplace la partie reconnue. Vide, elle est supprimée. Les captures s'écrivent `$1`, `$2`. |

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

Quand le paramètre apparaît plusieurs fois, chacune de ses valeurs est réécrite.
Il ne se passe rien si le paramètre est absent.

La syntaxe des expressions régulières est RE2, celle de Go : ni références
arrière, ni assertions avant ou arrière (lookaround).
