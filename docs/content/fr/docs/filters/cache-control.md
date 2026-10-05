---
title: cache-control
section: Filtres
order: 64
summary: Fixe l'en-tête Cache-Control de la réponse.
---

# cache-control

Décide de ce qui peut être mis en cache, pour un service qui n'en dit rien. C'est
le filtre qui tient une page personnelle à l'écart d'un cache partagé, et celui
qui autorise à conserver un catalogue public pendant une heure.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `value` | chaîne | oui | Le `Cache-Control` envoyé, par exemple `public, max-age=3600` ou `no-store`. |

## Exemple

```yaml
filters:
  - type: cache-control
    args:
      value: no-store
```

## Notes

`no-store` pour les données personnelles, `public, max-age=3600` pour les contenus
partagés.

`Expires` et `Pragma` sont supprimés par la même occasion : ils pourraient dire le
contraire, et certains caches donnent raison à l'en-tête le plus ancien. Dire deux
fois la même chose, c'est ainsi qu'une page finit dans un cache où elle n'aurait
jamais dû entrer.
