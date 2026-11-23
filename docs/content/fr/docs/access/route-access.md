---
title: La règle d'accès d'une route
section: Contrôle d'accès
order: 134
summary: La règle unique qui dit qui peut atteindre une route - niveau, rôles ou capacités, utilisateurs nommés - et ce qui se passe quand une route ne dit rien.
---

# La règle d'accès d'une route

Chaque route porte une règle, dans la section **Security** de l'éditeur de
route. Elle comprend trois parties, qui ne sont pas des alternatives :

- **un niveau** : le type d'appelant, en termes de session et de tenant ;
- **des rôles** : lesquels, détenus dans le tenant actif ;
- **des utilisateurs nommés** : des personnes qui passent de toute façon.

Le niveau et les rôles sont combinés par un **ET**. Les utilisateurs nommés s'y
ajoutent par un **OU**, par-dessus le tout.

## Les niveaux

| Niveau | Qui passe |
|---|---|
| **Delegated** (rien de choisi) | tout le monde, connecté ou non. Aucune session n'est même recherchée ; c'est le service qui décide |
| **Signed in** | toute personne qui a un compte - y compris un compte qui n'appartient encore à aucun tenant |
| **In a tenant** | une session qui a un tenant actif. Écarte un compte encore en attente d'accès |
| **In one of these tenants** | le tenant actif doit faire partie de ceux que vous nommez |
| **With capabilities** | un compte qui détient l'une des capacités choisies. Les rôles n'y comptent pas (voir [plus bas](#with-capabilities)) |
| **Nobody** | refusé avant tout appel au service. Seuls les utilisateurs nommés passent |

Il n'existe pas de niveau *public*, parce que déléguer en est déjà un, et qu'un
niveau baptisé public promettrait ce que la gateway ne peut pas garantir :
votre service peut toujours refuser.

Que *Signed in* laisse passer un compte sans appartenance est voulu : c'est ce
qui rend une page de salle d'attente, ou un profil en libre-service, accessible
à quelqu'un qui vient d'être reconnu et à qui rien n'a encore été accordé.

![La règle propre à une route : un niveau, les rôles qu'elle accepte, et les utilisateurs qu'elle laisse passer malgré tout](img/console/route-editor-security.webp)

## Les rôles

Choisissez-en autant que vous voulez ; **il suffit d'en détenir un pour
passer**. Ils sont lus dans le tenant *actif*, et comprennent tout ce que
les rôles de la personne impliquent par la hiérarchie.

Comme le catalogue est global alors que les groupes appartiennent à un
tenant, les deux axes disent réellement des choses différentes :

| Règle | Signification |
|---|---|
| rôles `billing-admin`, niveau *In a tenant* | un administrateur de la facturation du tenant actif, quel qu'il soit - une console commune à tous les tenants |
| rôles `billing-admin`, niveau *In one of these*, Acme | un administrateur de la facturation **d'Acme** |

Si vous laissez les rôles vides, n'importe quel rôle convient : le niveau décide
seul.

> [!NOTE]
> Une règle qui exige un rôle a toujours besoin d'un tenant, que vous
> l'ayez précisé ou non : les rôles n'existent qu'à l'intérieur d'un
> tenant. En pratique, choisissez *In a tenant* dès que vous
> nommez un rôle.

## With capabilities

Certaines pages ne sont pas faites pour qui détient un rôle, mais pour qui
administre : un écran des membres, une page d'exploitation. Un administrateur de
tenant peut mettre n'importe quel rôle du catalogue dans un groupe, `ROLE_ADMIN`
compris : un rôle ne peut donc pas garder une telle page. Le niveau **With
capabilities** demande plutôt ce que le compte **est** :

![La route d'un écran des membres réservée à l'administrateur d'Acme Logistics et aux administrateurs de l'application : le niveau With capabilities, ses capacités et le tenant auquel il se limite](img/console/route-editor-capabilities.webp)

| Capacité | Qui la détient |
|---|---|
| **Root** | root |
| **Infra admin** | un compte qui a la capacité infra admin, ou root |
| **App admin** | un compte qui a la capacité app admin, ou root |
| **Developer** | un compte qui a la capacité dev |
| **Tenant admin** | le propriétaire du tenant actif, ou un membre **ADMIN** actif de ce tenant. Proposée quand l'installation sert plusieurs tenants |

- **Il suffit de détenir l'une des capacités choisies**, et root passe toujours.
- **Ce niveau ne demande aucun rôle** : dans l'éditeur, les rôles laissent la
  place aux capacités.
- **Tenant admin of** restreint l'administrateur de tenant. Vide, c'est
  l'administrateur du tenant actif, quel qu'il soit ; avec des tenants nommés,
  seulement l'administrateur de l'un d'eux, et ce tenant doit être le tenant
  actif. Un administrateur sans tenant actif est refusé.
- **La route entière seulement** : la surcharge d'une opération et une limite de
  débit ne proposent pas ce niveau, et l'API l'y refuse.
- Les utilisateurs nommés passent toujours par-dessus.

La route peut aussi dire à son service ce qu'est l'appelant : **Auth forward**
propose l'attribut `capabilities`, une liste dans un JWT et des valeurs séparées
par des virgules dans un en-tête, pour que la page sache quelles actions montrer.

## Les utilisateurs nommés

Une liste de noms d'utilisateur qui passent **quel que soit le niveau exigé**,
*Nobody* compris. C'est le mécanisme d'exception : un compte de service, un
compte de support, une application réservée à une seule personne.

C'est aussi de cette façon que s'écrit "seulement ces personnes" : le niveau
**Nobody**, plus les noms d'utilisateur. Une règle qui nomme des utilisateurs
sans poser ni niveau ni rôle **n'est pas une règle**, et les noms sont retirés
à l'enregistrement : sur une route déléguée, tout le monde passe déjà, et
nommer quelqu'un ne dit rien.

## Quand une route ne dit rien

La requête est relayée. Aucune session n'est résolue, aucun cookie n'est lu,
rien n'est refusé.

> [!WARNING]
> Une règle vide ne signifie **pas** "authentifié" et **pas** "public" : elle
> signifie *aucun filtrage*. Meerkat ajoute des conditions, il ne retire jamais
> celles qu'applique votre service. Si une route doit exiger une session,
> choisissez au moins *Signed in*.

## Plusieurs routes, une requête

La règle d'une route intervient dans le **choix** de la route ; elle n'est pas
appliquée après coup. Il en découle une conséquence surprenante et utile : une
route dont la règle écarte cet appelant est **sautée**, et la route suivante
qui reconnaît la requête est essayée.

Deux routes sur le même chemin peuvent ainsi servir deux publics - une page
riche pour les membres d'Acme, une page d'accueil pour tous les autres - par
leur seul ordre. Le refus de la première route est gardé en mémoire, et n'est
délivré que si aucune autre route ne répond.

La seule exception est **Nobody** : un `deny` refuse sur-le-champ et ne laisse
jamais retomber. C'est ainsi que l'on ferme un chemin, au lieu de le rouvrir
plus bas.

## À quoi ressemble un refus

Sur une **route UI**, la personne arrive sur une page dans sa propre langue :

| Situation | Où elle arrive |
|---|---|
| Changer de tenant lèverait le refus | `/select-tenant`, qui explique pourquoi |
| Elle n'appartient à aucun tenant et la règle en exige un | `/account-pending`, la salle d'attente |
| Tout autre cas | `/refused`, qui nomme la règle à l'origine du refus et liste ce que cette session peut ouvrir |
| Aucune session | la page de connexion, qui retient la destination |

La proposition de changer de tenant est confrontée à la réalité : elle
n'est faite que si un autre tenant auquel la personne appartient
satisferait effectivement la règle.

Sur une **route de service**, la réponse est un `403` accompagné d'une phrase
qui nomme ce qui manquait - le tenant, l'un de ces rôles, l'une de ces
capacités, cet endpoint est
fermé - ou un `401` avec `WWW-Authenticate: Session` quand il n'y a aucune
session. Personne ne lit une page HTML dans un `curl`.
