---
title: maintenance
section: Filtres
order: 68
summary: Répond 503 avec la page d'indisponibilité de la gateway au lieu de proxifier.
---

# maintenance

Met une route hors service sans toucher aux autres. La route continue de matcher,
donc aucune autre route ne récupère son trafic pendant que son service est à
l'arrêt, et les visiteurs reçoivent la page d'indisponibilité de l'installation
plutôt qu'un `502`.

C'est un filtre **terminal** : rien n'est proxifié et le service n'est jamais
appelé.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `reason` | chaîne | non | Ce qu'on dit, dans une liste fermée : vide (ne rien dire), `maintenance`, `upgrade`, `incident`. |

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

La raison est une clé, pas une phrase : la page est lue dans vingt langues, et une
phrase tapée ici serait une phrase dans une seule d'entre elles. Une raison vide
ne dit rien, ce qui est parfois la réponse honnête ; une valeur hors de la liste
ne dit rien non plus.

> [!NOTE]
> Pour toute la gateway d'un coup, il y a un interrupteur et non un filtre : le
> mode maintenance met toutes les routes hors service sans rien à modifier ni rien
> à défaire.
