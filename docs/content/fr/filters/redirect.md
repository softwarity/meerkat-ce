---
title: redirect
section: Filtres
order: 73
summary: Répond une redirection au lieu de proxifier.
---

# redirect

Répond une redirection sans rien appeler. Pour un chemin qui a déménagé, un
raccourci qui doit atterrir ailleurs, ou une ancienne porte d'entrée maintenue
après une migration.

C'est un filtre **terminal** : rien n'est proxifié et aucun amont n'est appelé.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `location` | chaîne | oui | Où l'appelant est envoyé, en absolu ou en relatif. |
| `status` | entier | non | Le statut de redirection, un `3xx`. Défaut : `302`. |

Un statut hors des `3xx` est refusé à l'enregistrement de la route.

## Exemple

```yaml
filters:
  - type: redirect
    args:
      location: https://shop.example.com/catalogue
      status: 301
```

## Notes

Le `301` est permanent et les navigateurs le retiennent longtemps : c'est une
décision qu'on ne peut plus reprendre pour ceux qui l'ont déjà reçue. Le `307` est
temporaire et conserve la méthode, donc un `POST` reste un `POST`.

Une route qui redirige n'a pas besoin d'amont. Ses filtres de requête sont
abandonnés, puisque rien n'est proxifié ; les filtres de réponse s'appliquent
toujours.
