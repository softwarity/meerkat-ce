---
title: dedupe-response-header
section: Filtres
order: 67
summary: Supprime les valeurs en double d'un en-tête de réponse.
---

# dedupe-response-header

Garde une seule valeur là où il en est arrivé deux. La gateway et le service
écrivent le même en-tête, le navigateur refuse le doublon, et la page échoue pour
une raison que rien n'explique dans les journaux. C'est avec CORS que cela arrive
le plus souvent.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `names` | liste de chaînes | oui | Les en-têtes à nettoyer, par exemple `Access-Control-Allow-Origin`. |
| `keep` | chaîne | non | La valeur conservée : `first`, `last` ou `unique`. Par défaut : `first`. |

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

`first` garde la valeur du service, `last` celle qui a été ajoutée ensuite, et
`unique` garde une seule fois chaque valeur distincte.

`unique` respecte l'**ordre de première apparition** : un navigateur lit ces listes
dans l'ordre, et un ensemble les mélangerait différemment à chaque réponse.

Un en-tête qui ne porte qu'une valeur n'est pas modifié.
