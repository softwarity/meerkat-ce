---
title: Prédicats
section: Prédicats
order: 40
summary: Comment une route décide qu'une requête lui est destinée, et comment les onze prédicats se combinent.
---

# Prédicats

Un **prédicat** est une condition posée sur la requête entrante. Une route en
porte une liste et ne prend la requête que si **tous** l'acceptent : les prédicats
se combinent par ET, jamais par OU. Une route sans aucun prédicat prend tout.

Pour exprimer un OU, écrivez deux routes - ou utilisez un prédicat qui accepte
déjà une liste. Dans un même prédicat, plusieurs motifs, hôtes, méthodes ou
valeurs se combinent par OU.

```yaml
routes:
  - id: orders
    name: Orders API
    enabled: true
    order: 100
    upstream: http://orders.internal:8080
    predicates:
      - type: path
        args:
          patterns:
            - /orders/**
      - type: method
        args:
          methods:
            - GET
            - POST
```

Cette route répond à un `GET` ou à un `POST` sous `/orders`, et à rien d'autre.

## Quelle route répond

Les routes sont essayées par **ordre** croissant (à égalité, par nom), et c'est la
**première** dont tous les prédicats sont satisfaits qui répond. Si aucune ne
convient, la réponse est un simple `404`.

La règle d'accès d'une route participe au choix : un appelant que la règle écarte
passe à la route suivante qui convient, et il n'est refusé par la première route
qui l'a écarté que si aucune autre ne répond. Une règle réglée sur **deny** fait
exception - une porte fermée reste fermée.

Une route désactivée n'est pas chargée : elle n'est donc jamais retenue.

## La route attrape-tout

Une route attrape-tout n'est pas un réglage : c'est une route ordinaire dont le
prédicat de chemin est `/**`, placée en dernier. Elle récupère ce qu'aucune autre
route n'a pris, et c'est ainsi qu'une installation répond mieux qu'un `404` nu.

> [!TIP]
> Laissez de la marge entre les numéros d'ordre - `100`, `200`, `300` - pour
> pouvoir insérer une route entre deux autres sans renuméroter toute la table.

## L'essayer avant de livrer

Le testeur de routes de la console compose une requête fictive (méthode, chemin,
hôte, en-tête, cookie, paramètre de requête, adresse du client, horloge) et montre
le **verdict de chaque prédicat**, une ligne par prédicat, plutôt qu'un simple oui
ou non. L'horloge en fait partie : on peut donc essayer un `time-window` à une
date qui n'est pas encore arrivée.

## Les onze prédicats

| Type | Ce qu'il vérifie |
| --- | --- |
| [cookie](/docs/predicates/cookie) | Un cookie est présent, avec une liste de valeurs ou une expression régulière. |
| [header](/docs/predicates/header) | Un en-tête est présent, avec une liste de valeurs ou une expression régulière. |
| [host](/docs/predicates/host) | Le Host de la requête, par noms exacts ou jokers `*.suffixe`. |
| [method](/docs/predicates/method) | La méthode HTTP. |
| [path](/docs/predicates/path) | Le chemin de la requête, comparé à un ou plusieurs motifs. |
| [query](/docs/predicates/query) | Un paramètre de requête est présent, avec une liste de valeurs ou une expression régulière. |
| [remote-addr](/docs/predicates/remote-addr) | L'adresse du client, comparée à des plages CIDR. |
| [time-window](/docs/predicates/time-window) | La requête arrive dans une fenêtre de temps. |
| [version](/docs/predicates/version) | Une plage de versions d'API, lue dans un en-tête, un paramètre de requête ou le chemin. |
| [weight](/docs/predicates/weight) | Une part du trafic d'un groupe, pour un déploiement canari. |
| [x-forwarded-remote-addr](/docs/predicates/x-forwarded-remote-addr) | L'adresse la plus à droite de `X-Forwarded-For`, comparée à des plages CIDR. |

Tout ce qu'une route fait de la requête une fois qu'elle l'a retenue relève d'un
[filtre](/docs/filters/overview).
