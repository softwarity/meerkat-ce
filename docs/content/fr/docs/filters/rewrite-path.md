---
title: rewrite-path
section: Filtres
order: 83
summary: Réécrit le chemin avec un remplacement par regexp.
---

# rewrite-path

Remodèle le chemin quand retirer ou ajouter un préfixe ne suffit pas : réordonner
des segments, en retirer un au milieu, replier deux formes en une.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `pattern` | chaîne | oui | La regexp confrontée au chemin. |
| `replacement` | chaîne | oui | Ce qui le remplace. Les captures s'écrivent `$1`, `$2`. |

Un motif qui ne compile pas est refusé à l'enregistrement de la route.

## Exemple

```yaml
filters:
  - type: rewrite-path
    args:
      pattern: ^/shop/v2/(.*)$
      replacement: /api/$1
```

`/shop/v2/orders/8814` arrive au service en `/api/orders/8814`.

## Notes

Ancrez le motif avec `^` sauf si vous voulez matcher n'importe où : un motif non
ancré réécrit **toutes** les occurrences dans le chemin, ce qui est rarement
l'intention.

Une capture écrite `$1` suivie d'une lettre se lit comme le groupe nommé `1x`, qui
n'existe pas, et le remplacement la perd en silence. Écrivez `${1}` dès qu'une
lettre ou un chiffre suit.

La syntaxe est celle de RE2, la bibliothèque de Go : pas de rétroréférence, pas de
lookaround. Un motif emprunté à une autre gateway peut demander une réécriture.

Pour les deux cas quotidiens il y a des briques plus simples :
[strip-prefix](/docs/filters/strip-prefix) et
[prefix-path](/docs/filters/prefix-path). Contrairement à `strip-prefix`, ce
filtre n'annonce rien : un service qui construit ses liens ne saura pas qu'il vit
sous un préfixe.
