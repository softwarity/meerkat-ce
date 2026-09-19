---
title: cache-control
section: Filtres
order: 64
summary: Pose le Cache-Control de la réponse.
---

# cache-control

Décide ce qui peut être mis en cache, pour un service qui n'en dit rien. C'est le
filtre qui garde une page personnelle hors d'un cache partagé, et celui qui
autorise à garder un catalogue public pendant une heure.

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

`no-store` pour des données personnelles, `public, max-age=3600` pour du contenu
partagé.

`Expires` et `Pragma` sont retirés au passage : ils pourraient dire le contraire,
et c'est le plus ancien qui gagne dans certains caches. Dire deux fois la même
chose, c'est ainsi qu'une page finit en cache alors qu'elle ne devait pas.
