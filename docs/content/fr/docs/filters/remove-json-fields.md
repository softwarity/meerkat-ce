---
title: remove-json-fields
section: Filtres
order: 74
summary: Retire des champs d'une réponse JSON, par leur nom ou par un chemin à points.
---

# remove-json-fields

Le service répond avec plus d'informations que ses appelants ne devraient en voir.
Ce filtre retire des champs du JSON à la sortie, par leur nom ou par leur chemin,
sans modifier le service.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `fields` | liste de chaînes | oui | Des noms de champ ou des chemins à points, par exemple `password`, `user.email`. |

## Exemple

```yaml
filters:
  - type: remove-json-fields
    args:
      fields:
        - password
        - passwordHash
        - user.email
```

## Notes

Un nom seul (`password`) est retiré au premier niveau ; un chemin à points
(`user.email`) descend dans le document. Les deux fonctionnent **à l'intérieur des
tableaux**, et c'est pour ce cas que le filtre existe : sans cela, un service qui
renvoie une liste d'utilisateurs conserverait tous les mots de passe qu'on lui
demandait de retirer.

La réponse doit se déclarer en JSON (`application/json` ou un type de média en
`+json`). Ce qui n'est pas du JSON, ou ce qui prétend l'être mais ne se laisse pas
analyser, passe intact : une gateway qui abîme ce qu'elle ne sait pas lire est
pire qu'une gateway qui le transmet.

Certaines réponses ne sont jamais réécrites, par conception : les flux
d'événements et tout ce qui est marqué `no-transform`, les réponses `204`, `304`
et `206`, les changements de protocole (upgrade), un corps compressé autrement
qu'en gzip ou en brotli, et un corps qui dépasse le plafond de réécriture de
l'installation (`20 MiB`, sauf si un exploitant l'a modifié). Une réponse plus
volumineuse passe entière plutôt que tronquée.
