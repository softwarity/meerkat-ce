---
title: path
section: Prédicats
order: 45
summary: Compare le chemin de la requête à un ou plusieurs motifs.
---

# path

Compare le chemin de la requête à un ou plusieurs motifs. C'est le prédicat par
lequel presque toutes les routes commencent : c'est le chemin qui distingue une
application d'une autre.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `patterns` | liste de chaînes | oui | Les motifs auxquels le chemin est comparé, par exemple `/api/users/{id}`, `/static/**`. Plusieurs motifs se combinent par OU. |

## Exemple

```yaml
predicates:
  - type: path
    args:
      patterns:
        - /orders/**
        - /invoices/**
```

## Notes

La notation est celle des motifs de chemin **de style Ant**, de la famille Spring
([PathPattern](https://docs.spring.io/spring-framework/docs/current/javadoc-api/org/springframework/web/util/pattern/PathPattern.html)),
dans un sous-ensemble strict : pas de `?`, pas de joker `*`, pas de `{*name}`. Un
motif doit commencer par `/` et se compare **segment par segment** :

- `{name}` prend exactement un segment, quel que soit son contenu : `/orders/{id}` accepte `/orders/8814`, mais pas `/orders/8814/lines`.
- `**` prend tout le reste du chemin, et n'est autorisé qu'en **dernier segment**. `/orders/**` accepte `/orders`, `/orders/8814` et `/orders/8814/lines`.
- tout le reste est un segment littéral.

Les frontières de segment sont respectées : `/demo/**` accepte `/demo` et
`/demo/x`, jamais `/demolition`. Une barre oblique finale est ignorée : `/orders/`
et `/orders` désignent la même route.

> [!WARNING]
> Un `*` seul n'est **pas** un joker. `/orders/*` accepte le chemin littéral
> `/orders/*` et rien d'autre. Écrivez `{id}` pour un segment et `**` pour une fin
> de chemin.

`{name}` accepte un segment mais ne capture rien d'exploitable ailleurs : aucun
filtre ne peut le relire. Utilisez [rewrite-path](/docs/filters/rewrite-path)
quand la valeur doit être déplacée.

## Exclure un chemin

Un motif ne sait pas dire "tout sauf", et il n'y a pas non plus d'expression
régulière : la gateway lit le préfixe d'un motif de chemin à plusieurs endroits
(la sécurité par endpoint, qui rattache une requête à son opération OpenAPI, le
lien du portail vers une application, la redirection de langue, les métriques par
endpoint), et une expression régulière n'a pas de préfixe à lire.

Pour tenir un chemin à l'écart, donnez-lui une **route à lui**, placée au-dessus
des autres, qui refuse toujours - accès *Nobody*, ou un 404 fixe. Elle couvre
toutes les routes qui auraient capté ce chemin, la route attrape-tout comprise,
là où une exclusion écrite dans une route devrait être répétée dans chacune
d'elles, et serait oubliée dans la suivante.
