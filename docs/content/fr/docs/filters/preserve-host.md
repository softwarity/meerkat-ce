---
title: preserve-host
section: Filtres
order: 72
summary: Envoie à l'amont l'hôte de l'appelant plutôt que celui de l'amont.
---

# preserve-host

Le service construit ses liens à partir du nom sous lequel il a été appelé. Sans
ce filtre il voit le nom interne, et tous les liens, redirections et domaines de
cookie qu'il écrit mènent là-bas.

## Paramètres

Ce filtre ne prend aucun argument.

## Exemple

```yaml
filters:
  - type: preserve-host
```

On omet `args` puisqu'il n'y a rien à passer.

## Notes

L'en-tête `Host` et la valeur que Go met sur le fil sont posés tous les deux,
et c'est tout l'intérêt : n'en poser qu'un les met en désaccord, et c'est le bug
d'hôte virtuel qui coûte un après-midi.

Deux pièges à connaître :

- Un amont qui choisit un site **par son nom** reçoit le nom public au lieu de celui qu'il connaît, et répond son site par défaut. C'est à cela que sert [set-host](/docs/filters/set-host).
- Conserver l'hôte ne suffit pas pour une application publiée sous un préfixe : il lui faut aussi connaître ce préfixe, que [strip-prefix](/docs/filters/strip-prefix) annonce en `X-Forwarded-Prefix`.
