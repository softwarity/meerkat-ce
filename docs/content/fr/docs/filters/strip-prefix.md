---
title: strip-prefix
section: Filtres
order: 93
summary: Retire les N premiers segments du chemin avant l'envoi à l'upstream.
---

# strip-prefix

La gateway publie `/demo/orders`, le service ne connaît que `/orders`. C'est le
filtre le plus utilisé du catalogue : il permet de monter une application sous le
chemin de votre choix sans qu'elle en sache rien.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `parts` | entier | non | Nombre de segments retirés en tête du chemin. Par défaut : `1`. Au moins `1`. |
| `announcePrefix` | booléen | non | Indique au service où il est publié, dans `X-Forwarded-Prefix`. Par défaut : `true`. |

## Exemple

```yaml
predicates:
  - type: path
    args:
      patterns:
        - /demo/**
filters:
  - type: strip-prefix
    args:
      parts: 1
```

`/demo/orders/8814` arrive au service sous la forme `/orders/8814`, avec
`X-Forwarded-Prefix: /demo`.

## Notes

C'est par `X-Forwarded-Prefix` qu'un service qui **construit** ses propres liens
apprend où il est publié. Sans lui, une application qui voit `/orders` écrit
`/orders`, le navigateur suit le lien, et il aboutit hors de la route. Spring lit
cet en-tête avec `ForwardedHeaderFilter`, nginx l'écrit ; un service qui se
contente de répondre n'en a pas besoin.

Ce qu'un appelant a envoyé sous ce nom est purgé, sur toutes les routes, même
celles qui n'ont pas de `strip-prefix` : un appelant qui fournirait son propre
préfixe ferait écrire au service ses liens là où il l'a décidé.

**Au retour, le préfixe est rétabli** sur les deux éléments qu'un navigateur suit
sans rien demander : le `<base href>` d'une page (`<base href="/">` devient
`<base href="/demo/">`, faute de quoi ses scripts sont cherchés à la racine de la
gateway) et le `Location` d'une redirection que le service écrit depuis sa
propre racine ou vers sa propre adresse. Une base ou une redirection déjà sous le
préfixe, relative, ou vers un autre site n'est pas modifiée.

Deux `strip-prefix` à la suite fonctionnent : le préfixe annoncé s'élargit à ce qui
a réellement été retiré, au lieu d'être écrasé.

Une application qui construit des liens absolus a généralement besoin aussi de
[preserve-host](/docs/filters/preserve-host) : le préfixe dit où, le Host dit sous
quel nom.
