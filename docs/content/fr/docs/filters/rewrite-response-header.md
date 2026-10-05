---
title: rewrite-response-header
section: Filtres
order: 85
summary: Réécrit la valeur d'un en-tête de réponse par un remplacement à expression régulière.
---

# rewrite-response-header

Retire un nom interne de ce que le service dit de lui-même :
`billing-3.internal:8080` repart sous la forme `service:8080`.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `name` | chaîne | oui | L'en-tête réécrit. |
| `pattern` | chaîne | oui | L'expression régulière appliquée à la valeur. |
| `replacement` | chaîne | non | Ce qui remplace la partie reconnue. Vide, elle est supprimée. Les captures s'écrivent `$1`, `$2`. |

Un motif qui ne compile pas est refusé à l'enregistrement de la route.

## Exemple

```yaml
filters:
  - type: rewrite-response-header
    args:
      name: Server
      pattern: ^[^.]+\.internal
      replacement: service
```

## Notes

Quand l'en-tête est répété, chacune de ses valeurs est réécrite. Il ne se passe
rien si l'en-tête est absent.

Pour un `Location`, utilisez [rewrite-location](/docs/filters/rewrite-location) :
il connaît l'origine réellement utilisée par l'appelant, ce qu'un remplacement
fixe ignore.

La syntaxe des expressions régulières est RE2, celle de Go : ni références
arrière, ni assertions avant ou arrière (lookaround).
