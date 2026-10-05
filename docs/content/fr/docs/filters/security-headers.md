---
title: security-headers
section: Filtres
order: 86
summary: Ajoute les en-têtes de réponse sur lesquels un navigateur s'appuie pour se durcir.
---

# security-headers

Écrit les en-têtes sur lesquels un navigateur s'appuie pour se durcir, pour une
application qui n'en écrit aucun. Une seule brique au lieu de quatre
`set-response-header`, et les deux décisions sur lesquelles on se trompe
facilement sont déjà prises.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `referrerPolicy` | chaîne | non | `Referrer-Policy`. Par défaut : `strict-origin-when-cross-origin`. Vide, la valeur écrite par l'application est conservée. |
| `frameOptions` | chaîne | non | `X-Frame-Options` : `DENY` ou `SAMEORIGIN`. Par défaut : `SAMEORIGIN`. Vide, la valeur écrite par l'application est conservée. |
| `hstsMaxAge` | entier | non | Durée de vie de `Strict-Transport-Security`, en secondes. Par défaut : `0`, qui ne touche pas à l'en-tête. |
| `contentSecurityPolicy` | chaîne | non | Envoyée telle quelle si elle est renseignée. Une CSP s'écrit pour chaque application, elle ne se devine pas. |

## Exemple

```yaml
filters:
  - type: security-headers
    args:
      frameOptions: DENY
      hstsMaxAge: 31536000
```

## Notes

`X-Content-Type-Options: nosniff` est toujours envoyé. La politique de referrer,
la politique de cadres et la CSP sont envoyées quand elles sont renseignées.
Chacune **remplace** la valeur existante au lieu de s'y ajouter : ce sont des
décisions, et deux décisions n'en font aucune.

HSTS n'est envoyé **que sur TLS**, quelle que soit la valeur de `hstsMaxAge`. Un
navigateur le retient pendant des mois et refuserait ensuite le HTTP simple :
c'est ainsi qu'une gateway de développement devient injoignable. Il est envoyé
avec `includeSubDomains`. Pour toutes les routes à la fois, HSTS suit **Force
HTTPS**, sur l'écran TLS ; une valeur fixée ici l'emporte.

`X-XSS-Protection` et les deux autres en-têtes de l'époque d'Internet Explorer ne
sont pas envoyés du tout : les navigateurs actuels les ignorent.

> [!TIP]
> Laissez `contentSecurityPolicy` vide tant que vous n'avez pas de CSP écrite pour
> cette application. Une CSP devinée bloque la page, ou bien autorise tout.
