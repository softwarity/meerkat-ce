---
title: rewrite-path
section: Filtres
order: 83
summary: Réécrit le chemin par un remplacement à expression régulière.
---

# rewrite-path

Remodèle le chemin quand retirer ou ajouter un préfixe ne suffit pas - réordonner
des segments, en supprimer un au milieu, ramener deux formes à une seule.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `pattern` | chaîne | oui | L'expression régulière appliquée au chemin. |
| `replacement` | chaîne | oui | Ce qui remplace la partie reconnue. Les captures s'écrivent `$1`, `$2`. |

Un motif qui ne compile pas est refusé à l'enregistrement de la route.

## Exemple

```yaml
filters:
  - type: rewrite-path
    args:
      pattern: ^/shop/v2/(.*)$
      replacement: /api/$1
```

`/shop/v2/orders/8814` arrive au service sous la forme `/api/orders/8814`.

## Notes

Ancrez le motif avec `^`, sauf si vous voulez vraiment qu'il s'applique n'importe
où : un motif non ancré réécrit **toutes** les occurrences dans le chemin, ce qui
est rarement l'effet recherché.

Une capture écrite `$1` et suivie d'une lettre est lue comme le groupe nommé `1x`,
qui n'existe pas, et le remplacement la perd sans rien dire. Écrivez `${1}` dès
qu'une lettre ou un chiffre suit.

La syntaxe des expressions régulières est RE2, celle de Go : ni références
arrière, ni assertions avant ou arrière (lookaround). Un motif repris d'une autre
gateway peut demander une réécriture.

Pour les deux cas courants, il existe des briques plus simples :
[strip-prefix](/docs/filters/strip-prefix) et
[prefix-path](/docs/filters/prefix-path). Contrairement à `strip-prefix`, ce
filtre n'annonce rien : un service qui construit ses propres liens ne saura pas
qu'il est publié sous un préfixe.
