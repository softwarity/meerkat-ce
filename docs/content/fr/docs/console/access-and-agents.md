---
title: Jetons d'accès, MCP et API
section: La console
order: 164
summary: Piloter Meerkat sans navigateur - un jeton pour un script, une connexion MCP pour un assistant, et la référence de l'API.
---

# Jetons d'accès, MCP et API

Trois écrans pour un même sujet : intervenir sur cette gateway sans passer par la
console.

- **Access tokens** figure sur les deux plans. Sous **Infra**, l'écran émet des jetons
  de console destinés à un script ou à un pipeline. Sous **Application**, il liste en
  plus les jetons d'application : un administrateur d'application voit ceux de tous
  les comptes et peut les révoquer.
- **Infra > MCP** (administrateurs infra) connecte un assistant, sans produire la
  moindre clé.
- **API** (dans le rail) est la référence des appels que font l'un et l'autre.

## Access tokens

Un jeton d'administration donne accès à l'API de la console sans navigateur. Il agit
**avec vos propres pouvoirs**, réduits par son périmètre, et s'authentifie sur le port
d'administration avec `Authorization: Bearer mk_...`.

![L'écran Access tokens : trois jetons de console et deux jetons d'application, avec leur propriétaire, leur plan, leur périmètre et leur dernière utilisation](img/console/access-tokens.webp)

Cinq jetons : trois pour la console - un accès complet depuis `10.20.0.0/16`
uniquement, un en lecture seule et un réservé aux appels planifiés - et deux jetons
d'application que leurs propriétaires ont créés depuis leur profil. Un clic sur une
ligne ouvre son tiroir.

La création d'un jeton demande quatre informations :

| Champ | Ce qu'il détermine |
|---|---|
| **Token name** | Le nom sous lequel vous le retrouverez dans la liste et dans le journal d'audit |
| **Perimeter** | *Scheduled calls only* donne accès à `/api/schedules` et à rien d'autre ; *Read only* lit et lance les testeurs ; *Full access* couvre tout ce que vous pouvez faire |
| **Used from** | Des adresses ou des plages CIDR séparées par des virgules. La vérification porte sur l'adresse qui se connecte, jamais sur un en-tête transmis |
| **Expiry** | Jamais, 30 jours, 90 jours ou 1 an |

**Un périmètre ne fait que retirer des droits** : un jeton peut au plus ce que vous
pouvez. Un jeton en lecture seule émis par root lit tout ce que voit root et ne modifie
rien.

Le secret n'est **affiché qu'une seule fois**. Copiez-le à ce moment-là : il est
impossible de le retrouver ensuite. Conservez-le dans une variable d'environnement
plutôt que dans un fichier, car un fichier de configuration finit souvent dans un
commit.

Par la suite, chaque ligne propose : l'interrupteur d'activation (le jeton cesse de
fonctionner et peut être réactivé), **Edit** pour modifier ce qu'il autorise sans
toucher au secret, **New secret** pour renouveler le secret, et **Revoke**. La ligne
indique le périmètre, le préfixe, la date de création, la date d'expiration et la
dernière utilisation.

> [!WARNING]
> Un nouveau secret prend effet immédiatement. Tout ce qui utilise encore l'ancien est
> refusé tant que le nouveau n'est pas en place.

## MCP

Meerkat répond au Model Context Protocol sur le port d'administration : un assistant
peut ainsi lire cette gateway et y travailler avec vous. Aucun port à ouvrir :
l'endpoint se trouve là où se trouve déjà votre console.

![L'écran MCP : l'interrupteur de l'endpoint, le sélecteur de client avec la commande à coller, et une liste d'agents connectés encore vide](img/console/mcp.webp)

L'endpoint est activé et affiche son URL, le sélecteur de client est positionné sur
Claude Code avec la commande d'une ligne prête à être copiée, et aucun agent n'est
encore connecté.

Il est livré **désactivé**. L'interrupteur en haut de l'écran l'active et affiche l'URL.

**Connect an agent** fournit la commande exacte pour Claude Code, Gemini CLI, Kimi CLI
et Codex CLI, ou un bloc JSON générique pour tout autre client. Le premier appel ouvre
votre navigateur sur cette console : vous vous connectez, vous choisissez ce que
l'agent a le droit de faire, et **aucune clé n'est jamais écrite dans un fichier**.

**Connected agents** liste les agents connectés, avec le périmètre de chacun et la date
de sa dernière utilisation, et permet d'en déconnecter un en un clic.

Pour un client incapable de s'authentifier par un navigateur, le panneau *My client
cannot do that* affiche la même commande avec un en-tête Bearer et renvoie vers
Access tokens.

Chaque modification faite par un agent est inscrite dans le journal d'audit avec le nom
du jeton à côté de celui du compte - *admin, via claude-desktop*, et non *admin* - et
un [point de reprise](/docs/console/configuration) est créé à sa suite, comme pour toute
autre modification.

## API

L'entrée **API** du rail affiche la référence REST du plan de contrôle - la page
swagger-ui que sert la gateway - et les appels s'y essaient **avec votre véritable
session** : être dans la console vaut autorisation. C'est la même surface que celle
que pilote un jeton.

## Pièges

- **Read only est la valeur par défaut à la création**, et c'est en général ce qu'il
  vous faut. Élargir un périmètre se fait en une modification ; rattraper la fuite d'un
  jeton à accès complet, non.
- **Un jeton agit en votre nom.** Supprimer le compte qui le détient lui retire ses
  pouvoirs.
- **Un agent connecté n'apparaît pas dans la liste des jetons.** C'est une connexion :
  elle se consulte et se coupe sous MCP.
- **Le périmètre des appels planifiés existe pour les services backend** : un
  identifiant qui vit dans un manifeste de déploiement est celui qui risque le plus de
  fuiter et qui a le moins de chances d'être renouvelé. Voir
  [les appels planifiés](/docs/operations/scheduler).
