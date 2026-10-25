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
  de console destinés à un script ou à un pipeline, et des jetons internes pour un
  service qui appelle le scheduler ou publie des notifications. Sous **Application**, il liste en plus les jetons
  d'application : un administrateur d'application voit ceux de tous les comptes et
  peut les révoquer.
- **Infra > MCP** (administrateurs infra) connecte un assistant, sans produire la
  moindre clé.
- **Meerkat, API reference** est la référence des appels que font l'un et l'autre.

## Access tokens

Un jeton d'administration donne accès à l'API de la console sans navigateur. Il agit
**avec vos propres pouvoirs**, réduits par son périmètre, et s'authentifie sur le port
d'administration avec `Authorization: Bearer mk_...`.

![L'écran Access tokens : deux jetons de console et deux jetons internes, avec leur propriétaire, leur plan, leur périmètre et leur dernière utilisation](img/console/access-tokens.webp)

Quatre jetons : deux pour la console - un en lecture seule et un à accès complet -
et deux jetons internes, `billing` et `orders-api`, que des services présentent au
scheduler ; `orders-api` vient d'écrire les planifications que montre l'écran
Scheduler. Les filtres au-dessus de la liste la restreignent à **Console**,
**Application** (les jetons que chacun a créés depuis son profil, pour un
administrateur d'application) ou **Internal**. Un clic sur une ligne ouvre son
tiroir.

**Internal** liste les jetons de périmètre `schedules` et `notifications` : ils
ouvrent l'[API du scheduler](/docs/operations/scheduler#le-jeton), ou celle des
[notifications](/docs/operations/notifications#publier), sur le plan interne, et
rien sur le port de la console. Sous ce filtre, le bouton devient **New internal
token** : un nom - le service qui le détient, proposé parmi les services que voit
la gateway, ou `global` pour un jeton qu'ils partagent -, les adresses et
l'expiration, le périmètre étant ce qui le rend interne. Qu'un service en ait besoin
ou non dépend du réglage **Access** du scheduler, ou de celui des notifications.

La création d'un jeton demande quatre informations :

| Champ | Ce qu'il détermine |
|---|---|
| **Token name** | Le nom sous lequel vous le retrouverez dans la liste et dans le journal d'audit |
| **Perimeter** | *Scheduled calls only* donne accès à `/api/schedules` et à rien d'autre, sur le plan interne, et range le jeton sous Internal ; *Read only* lit et lance les testeurs ; *Full access* couvre tout ce que vous pouvez faire |
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

Il est livré **désactivé**. L'interrupteur en haut de l'écran l'active et affiche l'URL ;
tant qu'il est désactivé, le reste de l'écran est masqué, puisqu'il n'y a rien à quoi se
connecter.

**Connect an agent** fournit la commande exacte pour Claude Code, Gemini CLI, Kimi CLI
et Codex CLI, ou un bloc JSON générique pour tout autre client. Chaque commande installe
le serveur à la portée la plus large que le client propose, **tous vos projets** :
`-s user` pour Claude Code, `--scope user` pour Gemini CLI, et Codex le garde de toute
façon dans `~/.codex/config.toml`. Le premier appel ouvre
votre navigateur sur cette console : vous vous connectez, vous choisissez ce que
l'agent a le droit de faire, et **aucune clé n'est jamais écrite dans un fichier**.

**Connected agents** liste les agents connectés, avec le périmètre de chacun et la date
de sa dernière utilisation, et permet d'en déconnecter un en un clic.

Quand la console est servie en HTTPS avec un certificat d'une **autorité privée** (ou
auto-signé), l'agent le refuse tant qu'il ne fait pas confiance à cette autorité. Le
panneau *My certificate comes from a private authority*, affiché seulement en HTTPS,
donne la commande pour l'agent choisi et pour macOS, Linux ou Windows : Claude Code et
Gemini CLI tournent sur Node.js et lisent `NODE_EXTRA_CA_CERTS`, qui ajoute votre
autorité aux autorités publiques (sur macOS :
`echo 'export NODE_EXTRA_CA_CERTS=/chemin/vers/ca.crt' >> ~/.zshrc && source ~/.zshrc`,
puis relancer l'agent depuis un nouveau terminal) ; Codex CLI fait confiance aux
autorités du système, la vôtre s'ajoute donc au magasin du système ; Kimi CLI ne
documente aucun réglage propre, et c'est le magasin du système qui est proposé.

Pour un client incapable de s'authentifier par un navigateur, le panneau *My client
cannot do that* affiche la même commande avec un en-tête Bearer et renvoie vers
Access tokens.

Chaque modification faite par un agent est inscrite dans le journal d'audit avec le nom
du jeton à côté de celui du compte - *admin, via claude-desktop*, et non *admin* - et
un [point de reprise](/docs/console/configuration) est créé à sa suite, comme pour toute
autre modification.

## API

![API reference : la référence REST du plan de contrôle, ses appels essayés avec la session de la console](img/console/api-reference.webp)

**Meerkat, API reference** affiche la référence REST du plan de contrôle - la page
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
