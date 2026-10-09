---
title: files
section: Filtres
order: 67
summary: Sert des fichiers téléversés sur la route au lieu de transmettre - la police, la feuille de style, le script ou l'image qu'une UI demande quand rien derrière la gateway ne les sert.
---

# files

La route répond d'elle-même avec des **fichiers téléversés sur elle**. Le cas
d'usage : une UI qui a besoin d'une ressource que rien derrière la gateway ne
sert - une police, une feuille de style, un script, une image -, typiquement
**hors ligne**, quand le CDN qu'elle nomme est injoignable. On déclare la route
qui prend ce chemin, on choisit le mode **Files** dans Target, et on téléverse
les fichiers.

C'est un filtre **terminal** : rien n'est transmis, aucun upstream n'est appelé.

![Une route en mode Files : une feuille de style et deux polices téléversées sous /fonts, chacune avec le nom sous lequel elle est servie et son type, puis l'index, le cache navigateur et CORS](img/console/route-editor-files.webp)

## Paramètres

| Nom | Type | Obligatoire | Rôle |
| --- | --- | --- | --- |
| `index` | chaîne | non | Le fichier répondu sur le chemin même de la route, sans nom derrière - l'`index.html` d'un petit site. Vide : ce chemin répond 404, seules les adresses des fichiers répondent. |
| `cors` | booléen | non | Autoriser les pages de toute origine à lire les fichiers. Par défaut : `true`. |
| `maxAge` | entier | non | Secondes pendant lesquelles un navigateur garde un fichier avant de le revalider. Par défaut : `3600`. |

Les fichiers eux-mêmes ne sont pas des paramètres : ils se téléversent sur la
route, dans Target, et vivent à côté d'elle.

## Exemple

Une route sur `/fonts/**` en mode Files, avec `inter.css` et
`files/inter-latin.woff2` téléversés :

```yaml
predicates:
  - type: path
    args:
      patterns: ["/fonts/**"]
filters:
  - type: files
    args:
      index: inter.css
      cors: true
```

`/fonts/files/inter-latin.woff2` répond la police, `/fonts` répond `inter.css`,
tout autre chemin sous `/fonts` répond 404.

## Notes

- Un fichier est servi au chemin de la route suivi du **nom sous lequel il est
  servi**, qui peut contenir des dossiers (`files/inter-latin.woff2`) : les URL
  relatives d'une feuille de style continuent de fonctionner telles qu'elles ont
  été écrites. Ce nom part de celui du fichier téléversé, nettoyé pour une
  adresse (`Mon fichier - v2.pdf` devient `Mon-fichier-v2.pdf`), et se
  **modifie dans la liste** : renommer ne téléverse pas le fichier à nouveau, et
  l'index qui le nommait suit.
- Son **type** se lit sur son nom (`.woff2` donne `font/woff2`, `.css` donne
  `text/css`, `.js` donne `text/javascript`...), puis sur ses octets. Il se
  modifie aussi dans la liste, et une alerte signale le type que la passerelle
  n'a pu que deviner.
- Le lien de chaque fichier l'ouvre **sur le plan de données**, l'origine où
  répondent les applications - pas celle de cette console.
- Chaque réponse porte un **ETag**, l'empreinte du fichier : passé `maxAge`, le
  navigateur redemande et reçoit un `304` si rien n'a changé.
- **CORS** compte pour les polices et les modules : un navigateur refuse une
  police chargée par une page d'une autre origine si la réponse ne l'y autorise
  pas.
- **Limites** : 10 Mio par fichier, 32 Mio par route - les fichiers voyagent dans
  un paquet de configuration, et ce que personne ne peut transporter n'est plus
  de la configuration.
- Un **export** emporte les fichiers à côté du YAML, sous
  `assets/files/<id de la route>/` ; un import les remet en place. Une route
  importée sans ses fichiers est signalée dans le plan, puisqu'elle répondrait
  404 à tout.

> [!TIP]
> Une UI qui nomme `fonts.googleapis.com` ne passera pas par la route d'elle-même.
> Soit on la fait pointer vers la gateway, soit - hors ligne - on résout ce nom
> vers la gateway dans le DNS et on lui donne, dans la réserve TLS, un certificat
> de sa propre autorité : la route répond alors à l'hôte que l'UI demande.
