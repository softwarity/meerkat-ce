---
title: maintenance
section: Filtres
order: 68
summary: Répond 503 avec la page d'indisponibilité de la gateway, sans rien transmettre à l'upstream.
---

# maintenance

Met une route hors service sans toucher aux autres. La route continue de capter
ses requêtes : aucune autre ne récupère son trafic pendant que son service est
arrêté, et les visiteurs voient la page d'indisponibilité de l'installation plutôt
qu'un `502`.

C'est un filtre **terminal** : rien n'est transmis à l'upstream et le service n'est
jamais appelé.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `reason` | chaîne | non | Le motif affiché, à choisir dans une liste fermée : vide (ne rien dire), `maintenance`, `upgrade`, `incident`. |

## Exemple

```yaml
filters:
  - type: maintenance
    args:
      reason: upgrade
```

## Notes

La réponse est la page d'indisponibilité de l'installation, dans la langue du
visiteur, avec un `503` et `Cache-Control: no-store`.

Le motif est une clé, pas une phrase : la page est lue dans vingt langues, et une
phrase saisie ici n'existerait que dans l'une d'elles. Un motif vide ne dit rien,
ce qui est parfois la réponse la plus honnête ; une valeur hors de la liste ne dit
rien non plus.

> [!NOTE]
> Pour toute la gateway d'un coup, il existe un interrupteur plutôt qu'un
> filtre : le mode maintenance coupe toutes les routes, sans rien à modifier ni
> rien à défaire.
