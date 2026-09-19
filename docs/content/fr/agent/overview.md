---
title: Le point d'entree agent
section: Automatisation
order: 280
summary: Meerkat parle MCP, pour qu'un assistant lise et modifie la gateway avec des outils plutot qu'en devinant des appels REST.
---

# Le point d'entree agent

Meerkat expose son plan de controle a un assistant IA via **MCP** (Model Context
Protocol). L'assistant n'apprend pas une API REST : il recoit une liste d'outils
avec leurs noms, leurs descriptions et le schema de leurs arguments, et il les
appelle.

## L'activer

Le point d'entree est **coupe par defaut**. Activez-le dans la console sous
**Infra > MCP**, puis creez un jeton de plan de controle (**Infra > Access
tokens**).

Il repond sur `/mcp`, sur le port du **plan de controle**, et uniquement avec un
jeton Bearer :

```bash
curl -X POST https://meerkat.internal:9090/mcp \
  -H "Authorization: Bearer mk_..." \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

> [!NOTE]
> Il refuse les appels qui portent une origine de navigateur. Un jeton de plan
> de controle n'est pas une session, et une page dans un navigateur n'a pas a en
> detenir un.

## Ce qu'un jeton a le droit de faire

Un jeton porte un **perimetre**, et les outils qu'un appelant voit sont ceux que
ce perimetre ouvre : un jeton en lecture seule ne se voit pas proposer les
outils d'ecriture. Les memes gardes que dans la console s'appliquent : qui
administre un domaine obtient les outils de ce domaine, et personne n'obtient
davantage en passant par un assistant plutot que par un clic.

## Les outils

| Outil | Ce qu'il repond |
|---|---|
| `describe_gateway` | Quelle edition, quelle version, combien de tout. A appeler en premier. |
| `list_routes`, `get_route` | Les routes, puis l'une d'elles en entier. |
| `test_routing` | Quelle route une requete donnee atteindrait, et pourquoi. |
| `save_route`, `delete_route` | Ecrire une route et l'appliquer aussitot. |
| `list_route_bricks` | Le catalogue des predicats et des filtres, avec leurs parametres. |
| `list_users`, `list_tenants` | Les comptes et les organisations. |
| `read_traffic` | Ce qui passe en ce moment. |
| `read_audit` | Le journal d'audit, dans le perimetre de l'appelant. |
| `get_settings`, `save_portal` | Les reglages globaux, et le portail de navigation. |
| `get_branding`, `save_branding` | L'identite que portent les pages integrees. |
| `list_themes` | Les palettes de couleurs, et laquelle est active. |
| `export_configuration` | Toute la configuration, sous forme de document lisible. |
| `save_configuration`, `list_configurations` | Des instantanes nommes ou revenir. |

## Les images sont decrites, jamais transmises

Un logo ou un fond de page est stocke en data URI, et un megaoctet de base64
couterait plus de conversation que tout le reste d'une reponse. Les outils de
lecture renvoient donc **une ligne qui decrit l'image** plutot que ses octets :

```json
"background": { "image": "<png, 45 KiB>", "fit": "cover", "dim": 30 }
```

Cela compte au moment de reecrire. Un assistant lit, change un champ, et renvoie
l'objet entier : un resume dans un champ d'image signifie donc **garder ce qui
est stocke**. Pour changer une image, passez une URL `https` et la gateway la
telecharge elle-meme ; pour en retirer une, passez `"none"`.

> [!TIP]
> Il en va de meme pour l'icone d'un module de portail : elle se nomme, elle ne
> se dessine pas. Passez `"storefront"` et la gateway stocke le dessin.
