---
title: Le point d'entrée agent
section: Exploitation
order: 230
summary: Meerkat parle MCP, pour qu'un assistant lise et modifie la gateway avec des outils plutôt qu'en devinant des appels REST.
---

# Le point d'entrée agent

Meerkat expose son plan de contrôle à un assistant IA via **MCP** (Model Context
Protocol). L'assistant n'apprend pas une API REST : il reçoit une liste d'outils
avec leurs noms, leurs descriptions et le schéma de leurs arguments, et il les
appelle.

## L'activer

Le point d'entrée est **coupé par défaut**. Activez-le dans la console sous
**Infra > MCP**, puis créez un jeton de plan de contrôle (**Infra > Access
tokens**).

Il répond sur `/mcp`, sur le port du **plan de contrôle**, et uniquement avec un
jeton Bearer :

```bash
curl -X POST https://meerkat.internal:9090/mcp \
  -H "Authorization: Bearer mk_..." \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

> [!NOTE]
> Il refuse les appels qui portent une origine de navigateur. Un jeton de plan
> de contrôle n'est pas une session, et une page dans un navigateur n'a pas à en
> détenir un.

## Ce qu'un jeton a le droit de faire

Un jeton porte un **périmètre**, et les outils qu'un appelant voit sont ceux que
ce périmètre ouvre : un jeton en lecture seule ne se voit pas proposer les
outils d'écriture. Les mêmes gardes que dans la console s'appliquent : qui
administre un domaine obtient les outils de ce domaine, et personne n'obtient
davantage en passant par un assistant plutôt que par un clic.

## Les outils

| Outil | Ce qu'il répond |
|---|---|
| `describe_gateway` | Quelle édition, quelle version, combien de tout. À appeler en premier. |
| `list_routes`, `get_route` | Les routes, puis l'une d'elles en entier. |
| `test_routing` | Quelle route une requête donnée atteindrait, et pourquoi. |
| `save_route`, `delete_route` | Écrire une route et l'appliquer aussitôt. |
| `list_route_bricks` | Le catalogue des prédicats et des filtres, avec leurs paramètres. |
| `list_users`, `list_tenants` | Les comptes et les organisations. |
| `read_traffic` | Ce qui passe en ce moment. |
| `read_audit` | Le journal d'audit, dans le périmètre de l'appelant. |
| `get_settings`, `save_portal` | Les réglages globaux, et le portail de navigation. |
| `get_branding`, `save_branding` | L'identité que portent les pages intégrées. |
| `list_themes` | Les palettes de couleurs, et laquelle est active. |
| `export_configuration` | Toute la configuration, sous forme de document lisible. |
| `save_configuration`, `list_configurations` | Des instantanés nommés où revenir. |

## Les images sont décrites, jamais transmises

Un logo ou un fond de page est stocké en data URI, et un mégaoctet de base64
coûterait plus de conversation que tout le reste d'une réponse. Les outils de
lecture renvoient donc **une ligne qui décrit l'image** plutôt que ses octets :

```json
"background": { "image": "<png, 45 KiB>", "fit": "cover", "dim": 30 }
```

Cela compte au moment de réécrire. Un assistant lit, change un champ, et renvoie
l'objet entier : un résumé dans un champ d'image signifie donc **garder ce qui
est stocké**. Pour changer une image, passez une URL `https` et la gateway la
télécharge elle-même ; pour en retirer une, passez `"none"`.

> [!TIP]
> Il en va de même pour l'icône d'un module de portail : elle se nomme, elle ne
> se dessine pas. Passez `"storefront"` et la gateway stocke le dessin.
