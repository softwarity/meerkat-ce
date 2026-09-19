---
title: prefix-path
section: Filtres
order: 71
summary: Préfixe le chemin avant de proxifier.
---

# prefix-path

L'appelant demande `/orders` et le service reçoit `/api/orders`. Pour une
application montée sous un chemin que la gateway ne montre pas.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `prefix` | chaîne | oui | Ce qui est ajouté devant le chemin, par exemple `/api`. |

## Exemple

```yaml
filters:
  - type: prefix-path
    args:
      prefix: /api
```

## Notes

Le préfixe est ajouté au mot : écrivez le slash de tête et pas celui de fin. Ce
n'est pas un gabarit : un `{language}` ou un `{id}` dans un préfixe est ajouté tel
quel, caractère par caractère.

Le chemin de base de l'amont est ajouté **après** les filtres. Quand l'amont est
déjà écrit `http://orders.internal:8080/api`, il n'y a rien à préfixer ici.

[strip-prefix](/#/docs/filters/strip-prefix) fait l'inverse.
