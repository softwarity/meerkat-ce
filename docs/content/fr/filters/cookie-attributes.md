---
title: cookie-attributes
section: Filtres
order: 65
summary: Force des attributs sur les cookies que pose un amont.
---

# cookie-attributes

Ajoute les attributs qu'un cookie aurait dû porter. L'application tourne en HTTP
en clair derrière la gateway et pose ses cookies en conséquence ; le navigateur,
qui a joint la gateway en TLS, en attend mieux.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `secure` | booléen | non | Ajoute `Secure`. Défaut : `true`. |
| `httpOnly` | booléen | non | Ajoute `HttpOnly`. Défaut : `false`. |
| `sameSite` | chaîne | non | `Lax`, `Strict` ou `None`. Vide laisse ce que l'application a posé. |

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
les cookies de la réponse sont réécrits, et ce que l'application avait déjà posé
dessus est conservé.

`SameSite=None` force `Secure`, quoi que dise `secure` : sans lui, tous les
navigateurs actuels jettent le cookie. Demander l'un, c'est demander les deux.
