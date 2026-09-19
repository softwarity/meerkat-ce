---
title: remove-json-fields
section: Filtres
order: 74
summary: Retire des champs d'une réponse JSON, par nom ou par chemin pointé.
---

# remove-json-fields

Le service répond plus que ce que ses appelants devraient voir. Retire des champs
du JSON à la sortie, par nom ou par chemin, sans que le service soit modifié.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `fields` | liste de chaînes | oui | Noms de champs ou chemins pointés, par exemple `password`, `user.email`. |

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

Un nom seul (`password`) est retiré au premier niveau ; un chemin pointé
(`user.email`) descend dans le document. Les deux fonctionnent **dans les
tableaux**, et c'est le cas pour lequel ce filtre existe : un service qui répond
une liste d'utilisateurs garderait sinon tous les mots de passe qu'on lui a
demandé de retirer.

La réponse doit déclarer du JSON (`application/json` ou un type de média en
`+json`). Ce qui n'est pas du JSON, ou ce qui prétend l'être sans s'analyser,
passe intact : une gateway qui abîme ce qu'elle ne sait pas lire est pire qu'une
gateway qui transmet.

Certaines réponses ne sont jamais réécrites, par construction : les flux
d'événements et tout ce qui est marqué `no-transform`, les réponses `204`, `304`
et `206`, les changements de protocole, un corps compressé autrement qu'en gzip ou
brotli, et un corps au-delà du plafond de réécriture de l'installation
(`20 MiB` sauf si un opérateur l'a changé). Une réponse plus grosse passe entière
plutôt que tronquée.
