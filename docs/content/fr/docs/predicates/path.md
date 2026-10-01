---
title: path
section: Prédicats
order: 45
summary: Matche le chemin de la requête contre un ou plusieurs motifs.
---

# path

Matche le chemin de la requête contre un ou plusieurs motifs. C'est le prédicat
par lequel presque toute route commence : le chemin est ce qui distingue une
application d'une autre.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `patterns` | liste de chaînes | oui | Les motifs contre lesquels le chemin est essayé, par exemple `/api/users/{id}`, `/static/**`. Plusieurs se combinent par OU. |

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

La notation est celle des motifs de chemin **à la Ant** de la famille Spring
([PathPattern](https://docs.spring.io/spring-framework/docs/current/javadoc-api/org/springframework/web/util/pattern/PathPattern.html)),
en sous-ensemble strict : ni `?`, ni joker `*`, ni `{*nom}`.

Un motif doit commencer par `/` et se compare **segment par segment** :

- `{nom}` prend exactement un segment, quel qu'il soit : `/orders/{id}` matche `/orders/8814` et pas `/orders/8814/lines`.
- `**` prend tout le reste du chemin, et n'est autorisé qu'en **dernier segment**. `/orders/**` matche `/orders`, `/orders/8814` et `/orders/8814/lines`.
- tout le reste est un segment littéral.

Les frontières de segment sont respectées : `/demo/**` matche `/demo` et
`/demo/x`, jamais `/demolition`. Un slash final unique est ignoré, donc
`/orders/` et `/orders` sont la même route.

> [!WARNING]
> Une étoile seule n'est **pas** un joker. `/orders/*` matche le chemin littéral
> `/orders/*` et rien d'autre. Écrivez `{id}` pour un segment et `**` pour une
> queue de chemin.

`{nom}` matche un segment mais ne capture rien de réutilisable : aucun filtre ne
peut relire la valeur. Passez par
[rewrite-path](/docs/filters/rewrite-path) quand elle doit être déplacée.

## Exclure un chemin

Un motif ne sait pas dire « tout sauf », et il n'y a pas d'expression régulière non plus : la
passerelle lit le préfixe d'un motif de chemin à plusieurs endroits (la sécurité par endpoint
qui ramène une requête à son opération OpenAPI, le lien du portail vers une application, la
redirection de langue, les métriques par endpoint), et une expression régulière n'a pas de
préfixe à lire.

Pour tenir un chemin à l'écart, donnez-lui une **route à lui**, placée au-dessus des autres, qui
refuse toujours - accès *Nobody*, ou un 404 fixe. Elle couvre toutes les routes qui auraient
attrapé ce chemin, attrape-tout compris, là où une exclusion écrite dans une route devrait être
répétée dans chacune, et serait oubliée dans la suivante.
