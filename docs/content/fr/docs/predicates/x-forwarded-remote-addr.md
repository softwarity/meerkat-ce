---
title: x-forwarded-remote-addr
section: Prédicats
order: 51
summary: Compare à des plages CIDR l'adresse la plus à droite de X-Forwarded-For - celle que rapporte le dernier proxy.
---

# x-forwarded-remote-addr

Compare la **dernière** adresse de `X-Forwarded-For` à des plages CIDR.
Servez-vous-en quand un proxy frontal ou un load balancer est placé devant
Meerkat : l'adresse de la connexion est alors celle du proxy, et celle-ci est
l'adresse qu'il a rapportée.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `cidrs` | liste de chaînes | oui | Les plages acceptées, par exemple `10.0.0.0/8`, `192.168.1.10/32`. Plusieurs plages se combinent par OU. |

Un CIDR invalide est refusé à l'enregistrement de la route, et le message cite la
valeur.

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

C'est l'entrée **la plus à droite** qui est lue, celle qu'a ajoutée le dernier
proxy de la chaîne - la seule qu'un appelant ne peut pas choisir. Une requête sans
`X-Forwarded-For` n'est pas retenue.

> [!WARNING]
> Un client peut envoyer le `X-Forwarded-For` qu'il veut. N'utilisez ce prédicat
> que si c'est un proxy de confiance qui écrit cet en-tête ; si rien n'est placé
> devant la gateway, utilisez [remote-addr](/docs/predicates/remote-addr).
