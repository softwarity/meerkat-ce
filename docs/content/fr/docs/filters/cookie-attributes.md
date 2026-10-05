---
title: cookie-attributes
section: Filtres
order: 65
summary: Impose des attributs aux cookies écrits par un upstream.
---

# cookie-attributes

Ajoute aux cookies les attributs qu'ils auraient dû porter. L'application tourne
en HTTP simple derrière la gateway et écrit ses cookies en conséquence ; le
navigateur, lui, a joint la gateway en TLS et attend mieux.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `secure` | booléen | non | Ajoute `Secure`. Par défaut : `true`. |
| `httpOnly` | booléen | non | Ajoute `HttpOnly`. Par défaut : `false`. |
| `sameSite` | chaîne | non | `Lax`, `Strict` ou `None`. Vide, l'attribut écrit par l'application est conservé. |

## Exemple

```yaml
filters:
  - type: cookie-attributes
    args:
      secure: true
      httpOnly: true
      sameSite: Lax
```

## Notes

`Set-Cookie: id=abc; Path=/` devient `Set-Cookie: id=abc; Path=/; Secure`. Tous
les cookies de la réponse sont réécrits, et les attributs que l'application leur
avait déjà donnés sont conservés.

`SameSite=None` impose `Secure`, quelle que soit la valeur de `secure` : sans lui,
tous les navigateurs actuels rejettent le cookie. Demander l'un, c'est donc
demander les deux.
