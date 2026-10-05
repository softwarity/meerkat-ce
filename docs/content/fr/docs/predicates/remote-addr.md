---
title: remote-addr
section: Prédicats
order: 47
summary: Compare l'adresse du client à des plages CIDR.
---

# remote-addr

Compare l'adresse d'où vient la connexion à des plages CIDR. C'est ainsi qu'on
réserve un chemin d'administration au réseau du bureau, ou une route partenaire à
une seule machine.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `cidrs` | liste de chaînes | oui | Les plages acceptées, par exemple `10.0.0.0/8`, `192.168.1.10/32`. Plusieurs plages se combinent par OU. |
| `useForwarded` | booléen | non | Fait confiance à la **première** entrée de `X-Forwarded-For` plutôt qu'à la connexion. Par défaut : `false`. Uniquement derrière un proxy de confiance. |

Un CIDR invalide est refusé à l'enregistrement de la route, et le message cite la
valeur.

## Exemple

```yaml
predicates:
  - type: path
    args:
      patterns:
        - /admin/**
  - type: remote-addr
    args:
      cidrs:
        - 10.0.0.0/8
        - 192.168.1.10/32
```

## Notes

Derrière un autre proxy, l'adresse de la connexion est celle **du proxy** : toutes
les requêtes semblent venir d'une seule machine. Activez alors `useForwarded`, ou
utilisez [x-forwarded-remote-addr](/docs/predicates/x-forwarded-remote-addr), qui
lit l'adresse rapportée par le dernier proxy.

> [!WARNING]
> Un client peut envoyer le `X-Forwarded-For` qu'il veut. `useForwarded` fait
> confiance à la première entrée de cet en-tête : ne l'activez que si un proxy que
> vous maîtrisez le réécrit.

Une adresse seule est une plage en `/32` (`/128` en IPv6).
