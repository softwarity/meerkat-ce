---
title: prefix-path
section: Filtres
order: 71
summary: Ajoute un préfixe au début du chemin avant l'envoi à l'upstream.
---

# prefix-path

L'appelant demande `/orders` et le service reçoit `/api/orders`. Ce filtre sert
aux applications montées sous un chemin que la gateway ne montre pas.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `prefix` | chaîne | oui | Ce qui est ajouté au début du chemin, par exemple `/api`. |

## Exemple

```yaml
filters:
  - type: prefix-path
    args:
      prefix: /api
```

## Notes

Le préfixe est ajouté tel quel : écrivez la barre oblique initiale, et pas de
barre finale. Ce n'est pas un gabarit : `{language}` ou `{id}` dans un préfixe est
ajouté sous la forme de ces caractères, à la lettre.

Le chemin de base propre à l'upstream est ajouté **après** le passage des filtres. Si
l'upstream s'écrit déjà `http://orders.internal:8080/api`, il n'y a rien à préfixer
ici.

[strip-prefix](/docs/filters/strip-prefix) fait l'inverse.
