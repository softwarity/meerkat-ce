---
title: security-headers
section: Filtres
order: 86
summary: Pose les en-têtes de réponse sur lesquels un navigateur se durcit.
---

# security-headers

Pose les en-têtes sur lesquels un navigateur se durcit, pour une application qui
n'en pose aucun. Une brique au lieu de quatre `set-response-header`, avec les deux
décisions faciles à rater déjà prises.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `referrerPolicy` | chaîne | non | `Referrer-Policy`. Défaut : `strict-origin-when-cross-origin`. Vide laisse ce que l'application a posé. |
| `frameOptions` | chaîne | non | `X-Frame-Options` : `DENY` ou `SAMEORIGIN`. Défaut : `SAMEORIGIN`. Vide laisse ce que l'application a posé. |
| `hstsMaxAge` | entier | non | Durée de vie de `Strict-Transport-Security`, en secondes. Défaut : `0`, qui laisse l'en-tête tranquille. |
| `contentSecurityPolicy` | chaîne | non | Envoyé tel quel si renseigné. Une CSP s'écrit par application, elle ne se devine jamais. |

## Exemple

```yaml
filters:
  - type: security-headers
    args:
      frameOptions: DENY
      hstsMaxAge: 31536000
```

## Notes

`X-Content-Type-Options: nosniff` est toujours envoyé. La politique de référent,
la politique de cadre et la CSP sont envoyées si elles sont renseignées. Chacune
est **posée**, pas ajoutée : ce sont des décisions, et deux décisions n'en font
aucune.

HSTS n'est envoyé **qu'en TLS**, quoi que dise `hstsMaxAge`. Un navigateur le
retient des mois et refuserait ensuite le HTTP en clair, ce qui est la façon dont
une gateway de développement devient injoignable. Il part avec
`includeSubDomains`.

`X-XSS-Protection` et les deux autres en-têtes de l'époque d'Internet Explorer ne
sont pas envoyés du tout : les navigateurs actuels les ignorent.

> [!TIP]
> Laissez `contentSecurityPolicy` vide jusqu'à ce que vous en ayez écrit une pour
> cette application. Une CSP devinée bloque la page ou autorise tout.
