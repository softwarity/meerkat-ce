---
title: version
section: Prédicats
order: 49
summary: Matche une plage de versions d'API lue dans un en-tête, un paramètre de requête ou le chemin.
---

# version

Lit une version dans la requête et matche quand elle tombe dans une plage. Deux
routes sur le même chemin, une par version d'API : le `2.x` vers le nouveau
service, le reste vers l'ancien.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `source` | chaîne | non | Où la version est lue : `header`, `query` ou `path`. Défaut : `header`. |
| `name` | chaîne | non | L'en-tête ou le paramètre qui porte la version. Défaut : `X-API-Version`. Inutilisé quand la source est le chemin. |
| `pattern` | chaîne | oui | Regexp qui extrait la version, par exemple `v?(\d+(?:\.\d+)*)`. |
| `from` | chaîne | non | Version la plus basse acceptée, incluse, par exemple `2.0`. |
| `to` | chaîne | non | Première version **refusée**, exclue, par exemple `3.0`. |

Donnez `from`, `to`, ou les deux : une plage ouverte des deux côtés matche toutes
les versions, ce que ne pas mettre de prédicat de version fait déjà.

## Exemple

```yaml
predicates:
  - type: path
    args:
      patterns:
        - /orders/**
  - type: version
    args:
      source: header
      name: X-API-Version
      pattern: v?(\d+(?:\.\d+)*)
      from: '2.0'
      to: '3.0'
```

Mettez les bornes entre quotes en YAML : sans quotes, `2.0` est un nombre et perd
sa forme.

## Notes

Les versions se comparent comme des **nombres**, segment par segment : `1.10`
vient donc après `1.9`, là où une regexp sur le texte brut le lit avant. Trois
segments au maximum, et un plus court est complété : `from: '2'` prend aussi
`2.1`.

Le groupe nommé `version` gagne si le motif en a un, sinon le premier groupe
capturant, sinon le motif entier est pris pour la version.

Une requête sans version, ou avec une version que le motif ne sait pas lire, ne
matche pas. Placez la route des appelants sans version **en dessous** de
celle-ci.

Les bornes sont vérifiées à l'enregistrement : `from` sous `to`, les deux
lisibles comme des versions. Ce qu'envoie un appelant ne peut pas être validé
d'avance et, simplement, ne matche pas.

La console prévisualise ce prédicat pendant la frappe, en exécutant le code que
la gateway exécute : elle montre ce qui a été extrait et si la valeur tombe dans
la plage.
