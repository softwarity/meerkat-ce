---
title: preserve-host
section: Filtres
order: 72
summary: Envoie à l'upstream le Host de l'appelant, et non celui de l'upstream.
---

# preserve-host

Le service construit ses liens à partir du nom sous lequel il a été appelé. Sans
ce filtre, il voit le nom interne, et tous les liens, redirections et domaines de
cookie qu'il écrit y mènent.

## Paramètres

Ce filtre ne prend aucun argument.

## Exemple

```yaml
filters:
  - type: preserve-host
```

`args` est omis, puisqu'il n'y a rien à passer.

## Notes

Le filtre fixe à la fois l'en-tête `Host` et la valeur que Go écrit sur le réseau,
et c'est tout son intérêt : si l'on n'en fixe qu'un, les deux se contredisent, et
on obtient ce bug d'hôte virtuel qui fait perdre un après-midi.

**La console l'ajoute d'elle-même** à une nouvelle route dont l'upstream est une
application jointe directement : un service du cluster, un conteneur, une adresse
privée. Une telle application est seule derrière son adresse et lit le Host
qu'elle reçoit - pour vérifier l'origine d'un WebSocket (le canal live de Grafana
refuse une connexion dont l'origine n'est pas le Host qu'il a reçu), ou pour écrire
ses liens. Le filtre apparaît dans la liste pendant la saisie de l'adresse, et se
retire comme n'importe quel autre. Un nom public n'en reçoit pas, et une route
existante n'est jamais modifiée.

Deux pièges à connaître :

- Un upstream qui choisit un site **d'après le nom** reçoit le nom public au lieu de celui qu'il connaît, et répond avec son site par défaut. C'est à cela que sert [set-host](/docs/filters/set-host).
- Conserver le Host ne suffit pas à une application publiée sous un préfixe : elle doit aussi connaître ce préfixe, que [strip-prefix](/docs/filters/strip-prefix) annonce dans `X-Forwarded-Prefix`.
