---
title: redirect
section: Filtres
order: 73
summary: Répond par une redirection, sans rien transmettre à l'upstream.
---

# redirect

Répond par une redirection sans appeler quoi que ce soit. Pour un chemin qui a
déménagé, un raccourci qui doit mener ailleurs, ou un ancien point d'entrée
maintenu en vie après une migration.

C'est un filtre **terminal** : rien n'est transmis et aucun upstream n'est appelé.

## Paramètres

| Nom | Type | Obligatoire | Ce que ça fait |
| --- | --- | --- | --- |
| `location` | chaîne | oui | L'adresse vers laquelle l'appelant est envoyé, absolue ou relative. |
| `status` | entier | non | Le statut de la redirection, en `3xx`. Par défaut : `302`. |

Un statut hors de la plage `3xx` est refusé à l'enregistrement de la route.

## Exemple

```yaml
filters:
  - type: redirect
    args:
      location: https://shop.example.com/catalogue
      status: 301
```

## Notes

`301` est permanent et les navigateurs s'en souviennent longtemps : c'est une
décision sur laquelle vous ne pourrez pas revenir pour ceux qui l'ont déjà reçue.
`307` est temporaire et conserve la méthode : un `POST` reste un `POST`.

Une route qui redirige n'a pas besoin d'upstream. Ses filtres de requête sont
ignorés, puisque rien n'est transmis ; ses filtres de réponse s'appliquent
toujours.
