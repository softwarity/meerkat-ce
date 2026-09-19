---
title: Prédicats
section: Prédicats
order: 40
summary: Comment une route décide qu'une requête est pour elle, et comment les onze prédicats se combinent.
---

# Prédicats

Un **prédicat** est une condition sur la requête entrante. Une route en porte une
liste et ne prend la requête que si **tous** l'acceptent : les prédicats se
combinent par ET, jamais par OU. Une route sans aucun prédicat prend tout.

Pour exprimer un OU, écrivez deux routes - ou servez-vous d'un prédicat qui
accepte déjà une liste. Plusieurs motifs, hôtes, méthodes ou valeurs dans un même
prédicat se combinent par OU.

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

Cette route répond à un `GET` ou un `POST` sous `/orders`, et à rien d'autre.

## Quelle route répond

Les routes sont essayées par **ordre** croissant (à égalité, par nom) et la
**première** dont tous les prédicats matchent répond. Si rien ne matche, c'est un
`404` sec.

La règle d'accès de la route participe au choix : un appelant que la règle écarte
retombe sur la route suivante qui matche, et n'est refusé par la première qui l'a
écarté que si aucune autre ne répond. Une règle réglée sur **deny** est
l'exception : une porte fermée reste fermée.

Une route désactivée n'est pas chargée, donc elle ne matche jamais.

## La route attrape-tout

Un attrape-tout n'est pas un réglage : c'est une route ordinaire dont le prédicat
de chemin est `/**`, placée en dernier. Elle attrape ce qu'aucune autre route n'a
réclamé, ce qui permet de répondre mieux qu'un `404` sec.

> [!TIP]
> Laissez des trous dans les numéros d'ordre - `100`, `200`, `300` - pour pouvoir
> insérer une route entre deux autres sans renuméroter la table.

## L'essayer avant de livrer

Le testeur de routage de la console compose une requête fictive (méthode, chemin,
hôte, en-tête, cookie, paramètre, adresse cliente, horloge) et affiche le
**verdict de chaque prédicat**, une ligne par prédicat, plutôt qu'un oui ou non
global. L'horloge en fait partie : un `time-window` s'essaie donc à une date qui
n'est pas encore arrivée.

## Les onze prédicats

| Type | Ce qu'il matche |
| --- | --- |
| [cookie](/docs/predicates/cookie) | Un cookie est présent, contre une liste de valeurs ou une regexp. |
| [header](/docs/predicates/header) | Un en-tête est présent, contre une liste de valeurs ou une regexp. |
| [host](/docs/predicates/host) | L'hôte de la requête, noms exacts ou jokers `*.suffixe`. |
| [method](/docs/predicates/method) | La méthode HTTP. |
| [path](/docs/predicates/path) | Le chemin de la requête, contre un ou plusieurs motifs. |
| [query](/docs/predicates/query) | Un paramètre de requête est présent, contre une liste de valeurs ou une regexp. |
| [remote-addr](/docs/predicates/remote-addr) | L'adresse du client, contre des plages CIDR. |
| [time-window](/docs/predicates/time-window) | La requête arrive dans une fenêtre de temps. |
| [version](/docs/predicates/version) | Une plage de versions d'API lue dans un en-tête, un paramètre ou le chemin. |
| [weight](/docs/predicates/weight) | Une part du trafic d'un groupe, pour un canari. |
| [x-forwarded-remote-addr](/docs/predicates/x-forwarded-remote-addr) | L'adresse la plus à droite de `X-Forwarded-For`, contre des plages CIDR. |

Tout ce qu'une route fait de la requête une fois qu'elle a matché est un
[filtre](/docs/filters/overview).
