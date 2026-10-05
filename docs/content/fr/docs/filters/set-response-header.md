---
title: set-response-header
section: Filtres
order: 91
summary: Fixe un en-tête de réponse, en remplaçant toute valeur envoyée par l'upstream.
---

# set-response-header

Fixe un en-tête de la réponse, en remplaçant ce que le service a envoyé. C'est la
brique générique, pour un en-tête qu'aucun filtre dédié ne couvre.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `name` | chaîne | oui | L'en-tête fixé. |
| `value` | chaîne | oui | La valeur écrite. |

## Exemple

```yaml
filters:
  - type: set-response-header
    args:
      name: X-Frame-Options
      value: DENY
```

## Notes

Toutes les valeurs que le service a envoyées sous ce nom sont remplacées.

Les en-têtes qui **décrivent le corps**, comme `Content-Type` ou
`Content-Encoding`, ne sont pas appliqués au contenu : en fixer un ne change pas
les octets, cela fait seulement mentir la réponse à leur sujet.

Pour les en-têtes de durcissement, il existe
[security-headers](/docs/filters/security-headers), et pour le cache,
[cache-control](/docs/filters/cache-control) : tous deux prennent à votre place
les décisions sur lesquelles on se trompe facilement en procédant en-tête par
en-tête.
