---
title: Jetons d'accès, MCP et API
section: La console
order: 164
summary: Piloter Meerkat sans navigateur - un jeton pour un script, une connexion MCP pour un assistant, et la référence de l'API.
---

# Jetons d'accès, MCP et API

Trois écrans, un sujet : travailler sur cette passerelle depuis l'extérieur de la
console.

- **Access tokens** est sur les deux plans. Sous **Infra**, il frappe des jetons
  de console pour un script ou un pipeline. Sous **Application**, il liste aussi
  les jetons d'application : un admin d'application voit ceux de tout le monde
  et peut les révoquer.
- **Infra > MCP** (admins infra) branche un assistant, et ne produit aucune
  clé.
- **API** (rail) est la référence des appels que les deux font.

## Access tokens

Un jeton d'administration ouvre l'API de la console sans navigateur. Il agit **avec vos
propres pouvoirs**, restreints par son périmètre, et s'authentifie sur le port
d'administration en `Authorization: Bearer mk_...`.

![L'écran Access tokens : trois jetons de console et deux jetons d'application, avec leur propriétaire, leur plan, leur périmètre et leur dernier usage](img/console/access-tokens.webp)

Cinq jetons : trois pour la console - full access depuis `10.20.0.0/16` seulement,
read only, et scheduled calls only - et deux jetons d'application créés par leurs
propriétaires sur leur profil. Une ligne ouvre son tiroir.

En créer un pose cinq questions :

| Champ | Ce qu'il décide |
|---|---|
| **Token name** | Ce que vous reconnaîtrez dans la liste et dans le journal d'audit |
| **Perimeter** | *Scheduled calls only* n'ouvre que `/api/schedules` ; *Read only* lit et lance les testeurs ; *Full access* fait tout ce que vous pouvez faire |
| **Acts on** | Le plan de routage, l'identité applicative, ou tout ce que vous pouvez faire |
| **Used from** | Des adresses ou des plages CIDR, séparées par des virgules. Jugées sur l'adresse qui se connecte, jamais sur un en-tête transmis |
| **Expiry** | Jamais, 30 jours, 90 jours, ou un an |

**Un périmètre ne fait que retirer** : au plus ce que vous êtes. Un jeton de portée
gateway frappé par root pilote les routes et rien d'autre.

Le secret est **montré une seule fois**. Copiez-le alors ; il ne se récupère pas.
Gardez-le dans une variable d'environnement plutôt que dans un fichier, parce qu'un
fichier de configuration est une chose que les gens commitent.

Ensuite, chaque ligne propose : l'interrupteur (le jeton cesse de marcher, et peut
être rallumé), **Edit** pour changer ce qu'il peut faire sans toucher au secret, **New
secret** pour le tourner, et **Revoke**. La ligne montre le périmètre, le préfixe, la
date de création, l'expiration et le dernier usage.

> [!WARNING]
> Un nouveau secret prend effet immédiatement. Ce qui utilise l'ancien est refusé
> jusqu'à ce que le nouveau soit en place.

## MCP

Meerkat répond au Model Context Protocol sur le port d'administration, pour qu'un
assistant puisse lire cette passerelle et y travailler avec vous. Il n'y a pas de port
à ouvrir : le point d'entrée est là où votre console est déjà.

![L'écran MCP : l'interrupteur du point d'entrée, le sélecteur de client avec la commande à coller, et aucune connexion](img/console/mcp.webp)

Le point d'entrée est allumé et montre son URL, le sélecteur de client est sur
Claude Code avec la commande d'une ligne prête à copier, et aucun agent n'est
encore branché.

Il est livré **éteint**. L'interrupteur en haut l'allume et montre l'URL.

**Connect an agent** donne la commande exacte pour Claude Code, Gemini CLI, Kimi CLI,
Codex CLI, ou un bloc JSON générique pour le reste. Le premier appel ouvre votre
navigateur sur cette console : vous vous connectez, vous choisissez ce que l'agent peut
faire, et **aucune clé n'est jamais écrite dans un fichier**.

**Connected agents** liste ce qui est branché, avec le périmètre de chacun et son
dernier usage, et en débranche un d'un clic.

Pour un client qui ne sait pas s'authentifier par un navigateur, le panneau *My client
cannot do that* montre la même commande avec un en-tête porteur, et renvoie vers Access
tokens.

Chaque changement fait par un agent est écrit dans le journal d'audit avec le nom du
jeton à côté de celui du compte - *admin, via claude-desktop* et non *admin* - et un
[point de reprise](/docs/console/configuration) est écrit après, comme pour tout
autre changement.

## API

L'entrée **API** du rail montre la référence REST du plan de contrôle - la page
swagger-ui que la passerelle sert - et les appels sont essayés **avec votre vraie
session** : être dans la console EST l'autorisation. C'est la même surface qu'un jeton
pilote.

## Pièges

- **Read only est le défaut à la frappe**, et c'est en général ce qu'on veut. Elargir
  est une modification ; un jeton full access qui fuit ne l'est pas.
- **Un jeton agit comme vous.** Supprimer le compte qui le possède emporte ses
  pouvoirs.
- **Un agent branché n'est pas dans la liste des jetons.** C'est une connexion, et elle
  se voit et se coupe sous MCP.
- **Le périmètre des appels planifiés existe pour les services** : une crédentiale qui
  vit dans un manifeste de déploiement est celle qui a le plus de chances de fuiter et le
  moins d'être tournée. Voir [les appels planifiés](/docs/operations/scheduler).
