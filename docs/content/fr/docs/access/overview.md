---
title: Authentifier et autoriser
section: Contrôle d'accès
order: 130
summary: La différence entre prouver qui est quelqu'un et décider s'il peut passer, et l'endroit où s'écrit chaque règle.
---

# Authentifier et autoriser

Deux questions différentes, qui trouvent leur réponse à deux endroits
différents.

**Authentifier**, c'est prouver qui est quelqu'un : un mot de passe, un
fournisseur d'identité, une passkey, un jeton. Cela se fait une fois, au début,
et produit une session ou un jeton reconnu. C'est l'objet de la section
[Authentification](/docs/auth/overview).

**Autoriser**, c'est décider si *cet* appelant a droit à *cette* requête. Cela
se fait à chaque requête, et c'est l'objet de cette section.

> [!NOTE]
> Meerkat filtre **en plus** de votre service, jamais à sa place. Il ne dit
> jamais à votre application qu'une requête est légitime ; il refuse celles qui
> n'ont pas le droit d'arriver.

## Où s'écrit une règle

| Règle | Où | Ce qu'elle tranche |
|---|---|---|
| La règle d'accès d'une route | la section *Security* de la route | cet appelant peut-il seulement atteindre cette route |
| Une règle d'endpoint | **Infra > Endpoint security**, à partir de la spec OpenAPI de la route | cet appelant peut-il appeler *cette opération* |
| Appartenances et groupes | les membres et les groupes d'une organisation | quels rôles cette personne détient, ici |
| Le catalogue des rôles | **Application > Roles** | quels noms de rôle existent, et lesquels impliquent lesquels |
| Les capacités | **Application > Users** | qui peut administrer la gateway elle-même |

Les rôles viennent des groupes, les groupes appartiennent à une organisation, et
**les rôles n'existent qu'à l'intérieur d'une organisation** : une session sans
organisation active ne détient aucun rôle, quelles que soient les organisations
dont la personne est membre par ailleurs.

## L'ordre, pour une requête

1. Les **prédicats** des routes déterminent lesquelles pourraient répondre.
2. La **règle d'accès** de la route est évaluée - pendant le choix de la route, et non après. Si la règle ne pose aucune condition, la requête est relayée immédiatement, sans même qu'une session soit recherchée.
3. Une session ou un jeton `Bearer` est résolu, et la règle est appliquée à l'appelant.
4. Les filtres de garde et les rate limits de la route s'exécutent.
5. Si la route porte des règles d'endpoint, l'opération est identifiée et sa propre règle est appliquée.
6. La requête est relayée, avec l'identité sous forme d'en-têtes ou de JWT signé.

Que l'étape 2 fasse partie de la sélection a deux conséquences :

- **C'est la première route reconnaissant la requête, et dont la règle accepte l'appelant, qui l'emporte.** Une route dont la règle écarte cet appelant est sautée, et la route suivante qui reconnaît la requête est essayée. Le refus de la première est gardé en mémoire : c'est lui que reçoit l'appelant si aucune autre route ne répond.
- **Une règle `deny` ne laisse jamais retomber.** Elle refuse sur-le-champ.

## Le comportement par défaut, à ne pas mal comprendre

Une route qui ne déclare **aucune** règle d'accès est *déléguée* : la
gateway ne pose aucune condition, ne résout aucune session et confie la
requête à l'upstream, qui applique ses propres règles s'il en a.

> [!WARNING]
> L'absence de règle signifie **aucun filtrage**. Elle ne signifie ni
> "authentifié" ni "public" - il n'y a pas de niveau *public* à choisir,
> puisque déléguer en est déjà un. Si une route doit exiger une session,
> dites-le : `auth` au minimum.

## À quoi ressemble un refus

Sur une **route UI**, un refus mène à une page, dans la langue du visiteur : le
choix de l'organisation quand en changer résoudrait réellement le problème, la
salle d'attente quand le compte n'appartient à aucune organisation, et sinon
`/refused`, qui nomme la règle à l'origine du refus et propose ce que cette
session *peut* ouvrir.

Sur une **route de service**, un refus est un `403` accompagné d'une phrase :
personne ne lit une page dans un `curl`. Un appelant sans aucune session reçoit
un `401` ou, s'il s'agit d'une navigation, une redirection vers la page de
connexion.

## Ce qui n'existe pas

- **Pas d'interrupteur global pour désactiver le contrôle d'accès.** L'équivalent se fait route par route : laissez une règle vide, et cette route est déléguée.
- **Pas d'emprunt d'identité.** Un administrateur ne dispose d'aucun "se connecter à la place de cette personne", dans aucune édition.
