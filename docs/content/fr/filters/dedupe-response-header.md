---
title: dedupe-response-header
section: Filtres
order: 67
summary: Supprime les valeurs répétées d'un en-tête de réponse.
---

# dedupe-response-header

Garde une valeur là où deux sont arrivées. La gateway et le service posent le même
en-tête, le navigateur refuse la paire, et la page échoue pour une raison
qu'aucun journal n'explique. C'est avec CORS que ça arrive le plus souvent.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `names` | liste de chaînes | oui | Les en-têtes à nettoyer, par exemple `Access-Control-Allow-Origin`. |
| `keep` | chaîne | non | Quelle valeur survit : `first`, `last` ou `unique`. Défaut : `first`. |

## Exemple

```yaml
filters:
  - type: dedupe-response-header
    args:
      names:
        - Access-Control-Allow-Origin
        - Access-Control-Allow-Credentials
      keep: first
```

## Notes

`first` garde la valeur du service, `last` garde celle ajoutée ensuite, et
`unique` garde chaque valeur distincte une fois.

`unique` conserve l'**ordre de première apparition** : un navigateur lit ces
listes par position, et un ensemble les mélangerait différemment à chaque réponse.

Un en-tête qui ne porte qu'une valeur est laissé tranquille.
