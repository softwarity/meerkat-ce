---
title: set-response-header
section: Filtres
order: 91
summary: Fixe un en-tête de réponse (en remplaçant la valeur de l'amont).
---

# set-response-header

Décide un en-tête sur la réponse, en remplaçant ce que le service a envoyé. La
brique générique pour un en-tête qu'aucun filtre dédié ne couvre.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `name` | chaîne | oui | L'en-tête posé. |
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

Toutes les valeurs envoyées par le service sous ce nom sont remplacées.

Les en-têtes qui **décrivent le corps**, comme `Content-Type` ou
`Content-Encoding`, ne sont pas appliqués : en poser un ne change pas les octets,
ça fait seulement mentir la réponse à leur sujet.

Pour les en-têtes de durcissement il y a
[security-headers](/#/docs/filters/security-headers), et pour le cache
[cache-control](/#/docs/filters/cache-control) : tous les deux prennent, en un
en-tête, les décisions faciles à rater.
