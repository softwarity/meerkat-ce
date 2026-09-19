---
title: x-forwarded-remote-addr
section: Prédicats
order: 51
summary: Matche l'adresse la plus à droite de X-Forwarded-For contre des plages CIDR - celle que rapporte le dernier proxy.
---

# x-forwarded-remote-addr

Matche la **dernière** adresse de `X-Forwarded-For` contre des plages CIDR. À
utiliser quand un proxy frontal ou un répartiteur de charge se trouve devant
Meerkat : l'adresse de la connexion est alors celle du proxy, et celle-ci est
celle qu'il a rapportée.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `cidrs` | liste de chaînes | oui | Les plages acceptées, par exemple `10.0.0.0/8`, `192.168.1.10/32`. Plusieurs se combinent par OU. |

Un CIDR invalide est refusé à l'enregistrement de la route, en nommant la valeur.

## Exemple

```yaml
predicates:
  - type: path
    args:
      patterns:
        - /partner/**
  - type: x-forwarded-remote-addr
    args:
      cidrs:
        - 203.0.113.0/24
```

## Notes

C'est l'entrée **la plus à droite** qui est lue, celle que le dernier proxy de la
chaîne a ajoutée : la seule qu'un appelant ne choisit pas. Une requête sans
`X-Forwarded-For` ne matche pas.

> [!WARNING]
> Un client peut envoyer le `X-Forwarded-For` qu'il veut. N'utilisez ce prédicat
> que si un proxy de confiance est bien celui qui écrit cet en-tête ; sans rien
> devant la gateway, utilisez
> [remote-addr](/docs/predicates/remote-addr).
