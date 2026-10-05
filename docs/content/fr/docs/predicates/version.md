---
title: version
section: Prédicats
order: 49
summary: Accepte une plage de versions d'API, lue dans un en-tête, un paramètre de requête ou le chemin.
---

# version

Lit une version dans la requête et l'accepte quand elle tombe dans une plage.
Deux routes sur le même chemin, une par version d'API : la `2.x` vers le nouveau
service, le reste vers l'ancien.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `source` | chaîne | non | L'endroit où la version est lue : `header`, `query` ou `path`. Par défaut : `header`. |
| `name` | chaîne | non | L'en-tête ou le paramètre qui porte la version. Par défaut : `X-API-Version`. Inutilisé quand la source est le chemin. |
| `pattern` | chaîne | oui | L'expression régulière qui extrait la version, par exemple `v?(\d+(?:\.\d+)*)`. |
| `from` | chaîne | non | La plus petite version acceptée, incluse, par exemple `2.0`. |
| `to` | chaîne | non | La première version **refusée**, donc exclue, par exemple `3.0`. |

Donnez `from`, `to`, ou les deux : une plage ouverte des deux côtés accepte toutes
les versions, ce que fait déjà l'absence de prédicat de version.

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

Mettez les bornes entre guillemets dans le YAML : sans guillemets, `2.0` est un
nombre et perd sa forme.

## Notes

Les versions se comparent comme des **nombres**, segment par segment : `1.10`
vient donc après `1.9`, alors qu'une expression régulière appliquée au texte brut
le place avant. Trois segments au plus, et une version plus courte est complétée :
`from: '2'` accepte aussi `2.1`.

Si le motif contient un groupe nommé `version`, c'est lui qui est retenu ; sinon,
le premier groupe capturant ; sinon, toute la partie reconnue est prise pour la
version.

Une requête sans version, ou dont le motif ne parvient pas à lire la version,
n'est pas retenue. Placez la route des appelants sans version **en dessous** de
celle-ci.

Les bornes sont vérifiées à l'enregistrement de la route : `from` inférieur à
`to`, et les deux lisibles comme des versions. Ce qu'envoie un appelant ne peut
pas être validé à l'avance : une version illisible n'est simplement pas retenue.

La console affiche un aperçu de ce prédicat pendant la saisie, en exécutant le
même code que la gateway : elle montre ce qui a été extrait et si la version
tombe dans la plage.
