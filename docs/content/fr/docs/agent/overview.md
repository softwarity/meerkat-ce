---
title: Le point d'entrée des agents
section: Exploitation
order: 230
summary: Meerkat parle MCP : un assistant peut ainsi lire et modifier la gateway avec des outils, plutôt qu'en devinant des appels REST.
---

# Le point d'entrée des agents

Meerkat ouvre son plan de contrôle à un assistant IA par **MCP** (Model Context Protocol).
L'assistant n'a pas à apprendre une API REST : il reçoit une liste d'outils, avec leurs
noms, leurs descriptions et les schémas de leurs arguments, et il les appelle.

## L'activer

L'endpoint est **désactivé par défaut**. Activez-le dans la console, sous **Infra > MCP**,
puis créez un jeton du plan de contrôle (**Infra > Access tokens**).

L'endpoint répond sur `/mcp`, sur le port du **plan de contrôle**, et uniquement avec un
jeton Bearer :

```bash
curl -X POST https://meerkat.internal:9090/mcp \
  -H "Authorization: Bearer mk_..." \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

> [!NOTE]
> Il refuse les appels qui portent une origine de navigateur. Un jeton du plan de contrôle
> n'est pas une session, et une page dans un navigateur n'a aucune raison d'en détenir un.

## Ce qu'un jeton permet de faire

Un jeton porte un **périmètre**, et les outils que voit un appelant sont ceux que ce
périmètre ouvre : un jeton en lecture seule ne se voit pas proposer les outils d'écriture.
Les mêmes garde-fous que dans la console s'appliquent : la personne qui administre un
domaine obtient les outils de ce domaine, et personne n'obtient davantage en passant par un
assistant plutôt qu'en cliquant.

## Les outils

| Outil | Ce à quoi il répond |
|---|---|
| `describe_gateway` | Quelle édition, quelle version, combien d'éléments de chaque sorte. À appeler en premier. |
| `list_routes`, `get_route` | Les routes, puis l'une d'elles en entier. |
| `test_routing` | Quelle route atteindrait une requête donnée, et pourquoi. |
| `list_operations`, `call_route` | Ce que propose le service d'une route, puis un vrai appel à travers la gateway. |
| `save_route`, `delete_route` | Écrire une route et l'appliquer immédiatement. |
| `list_route_bricks` | Le catalogue des prédicats et des filtres, avec leurs paramètres. |
| `list_users`, `list_tenants` | Les comptes et les organisations. |
| `list_services` | Ce que la gateway voit tourner autour d'elle. |
| `list_schedules` | Les appels planifiés : ce qui s'exécute la nuit, à quelle fréquence, au nom de qui, et comment s'est terminée la dernière exécution. |
| `pause_schedule`, `run_schedule` | Suspendre un appel planifié, le laisser repartir, ou avancer sa prochaine exécution à maintenant. |
| `read_traffic` | Ce qui passe en ce moment. |
| `read_logs` | Les dernières lignes du journal de la gateway, sur le nœud qui a répondu. |
| `read_audit` | Le journal d'audit, dans le périmètre de l'appelant. |
| `get_settings`, `save_portal` | Les réglages globaux, et le portail de navigation. |
| `get_branding`, `save_branding` | L'identité que portent les pages intégrées. |
| `list_themes` | Les palettes de couleurs, et celle qui est active. |
| `export_configuration` | Toute la configuration, sous la forme d'un document lisible. |
| `save_configuration`, `list_configurations` | Des snapshots nommés, auxquels revenir. |

## Appeler les services derrière les routes

`list_operations` lit la spec OpenAPI d'une route : chaque opération avec le
**chemin public** à appeler - le préfixe de la route ajouté quand la route le
retire - et, pour une opération, ses paramètres, son corps et ses réponses, avec
les schémas qu'ils référencent.

`call_route` envoie une vraie requête à travers la route et renvoie le statut,
les en-têtes et le corps (coupé à 64 Kio). Elle est remise en interne au plan
applicatif : tout ce que rencontre un client s'applique - la règle d'accès de la
route, la sécurité par endpoint, les filtres, les quotas, et l'identité que la
route transmet au service.

- **En tant que qui.** Par défaut l'appelant est `agent`, et porte **tous les
  rôles** du catalogue - dans l'identité que reçoit le service aussi - pour que
  l'agent d'un administrateur ne bute pas sur des droits qu'il faudrait gérer.
  `as` et `roles` font appeler en tant que quelqu'un d'autre, et `tenant` sert
  pour une route réservée à des organisations.
- **Pourquoi un refus.** Un `403` dit pourquoi dans son corps : la gateway nomme
  le rôle ou l'organisation qui manque, et la raison propre d'un service revient
  telle que le service l'a écrite.
- **Pour de vrai.** Un `POST`, un `PUT` ou un `DELETE` modifie les données du
  service. L'appel est proposé à un jeton complet, jamais à un jeton en lecture
  seule, et chaque appel est écrit au journal d'audit avec le jeton qui l'a fait.
  Le service reçoit les en-têtes qui marquent un appel simulé : son propre
  journal distingue un test du trafic réel.

## Les images sont décrites, jamais transmises

Un logo ou un fond de page est stocké sous forme de data URI, et un mégaoctet de base64
occuperait plus de place dans une conversation que tout le reste d'une réponse. Les outils
de lecture renvoient donc **une ligne décrivant l'image** à la place de ses octets :

```json
"background": { "image": "<png, 45 KiB>", "fit": "cover", "dim": 30 }
```

Cela compte au moment d'écrire. Un assistant lit, modifie un champ, puis renvoie l'objet
entier : un résumé dans un champ d'image signifie donc **conserver ce qui est stocké**.
Pour changer une image, passez une URL `https` et la gateway va la chercher elle-même ;
pour en retirer une, passez `"none"`.

> [!TIP]
> Il en va de même pour l'icône d'un module du portail : elle est désignée par son nom,
> pas dessinée. Passez `"storefront"` et la gateway enregistre le dessin.
