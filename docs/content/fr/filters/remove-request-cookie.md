---
title: remove-request-cookie
section: Filtres
order: 76
summary: Retire un cookie de la requête.
---

# remove-request-cookie

Retire un cookie sur le chemin du service et conserve les autres - typiquement un
cookie de session que l'application prendrait pour le sien.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `name` | chaîne | oui | Le cookie retiré. |

## Exemple

```yaml
filters:
  - type: remove-request-cookie
    args:
      name: meerkat_session
```

## Notes

L'en-tête `Cookie` est une seule chaîne qui porte tous les cookies : ce filtre le
reconstruit au lieu de le supprimer, car supprimer l'en-tête emporterait la
session de l'application avec lui. C'est aussi pourquoi
[remove-request-header](/#/docs/filters/remove-request-header) n'est pas le bon
outil pour un cookie.
