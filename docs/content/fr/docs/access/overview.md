---
title: Authentifier et autoriser
section: Contrôle d'accès
order: 130
summary: La différence entre prouver qui on est et décider si on passe, et où chaque règle se pose.
---

# Authentifier et autoriser

Deux questions différentes, tranchées à deux endroits différents.

**Authentifier**, c'est prouver qui est quelqu'un : un mot de passe, un
fournisseur, une passkey, un jeton. Cela arrive une fois, au début, et cela produit
une session ou un jeton reconnu. C'est la section
[Authentification](/docs/auth/overview).

**Autoriser**, c'est décider si *cet* appelant a droit à *cette* requête. Cela
arrive à chaque requête, et c'est le sujet de cette section.

> [!NOTE]
> Meerkat filtre **en plus** de votre service, jamais à sa place. Il ne dit jamais à
> votre application qu'une requête est légitime ; il écarte celles qui n'ont pas le
> droit d'arriver.

## Où une règle peut s'écrire

| Règle | Où | Répond à |
|---|---|---|
| La règle d'accès d'une route | la section *Security* de la route | cet appelant peut-il atteindre cette route |
| Une règle d'endpoint | **Infra > Endpoint security**, depuis la spec OpenAPI de la route | cet appelant peut-il appeler *cette opération* |
| Appartenance et groupes | les membres et les groupes d'une organisation | quels rôles cette personne détient, ici |
| Le catalogue de rôles | **Application > Roles** | quels noms de rôles existent, et lesquels impliquent lesquels |
| Les capacités | **Application > Users** | qui peut administrer la gateway elle-même |

Les rôles viennent des groupes, les groupes appartiennent à une organisation, et
**les rôles n'existent qu'à l'intérieur d'une organisation** : une session sans
organisation active ne détient aucun rôle, quoi que la personne soit ailleurs.

## L'ordre sur une requête

1. Les **prédicats** des routes décident lesquelles pourraient répondre.
2. La **règle d'accès** de la route est évaluée - pendant le choix de la route, pas après. Si la règle ne pose aucune condition, la requête est proxifiée immédiatement et aucune session n'est même cherchée.
3. Une session ou un jeton `Bearer` est résolu, et la règle est jugée contre l'appelant.
4. Les garde-fous et les limites de débit de la route s'exécutent.
5. Si la route porte des règles d'endpoint, l'opération est appariée et sa propre règle est jugée.
6. La requête est proxifiée, avec l'identité en en-têtes ou en JWT signé.

Deux conséquences du fait que l'étape deux fasse partie du choix :

- **La première route qui matche et dont la règle accepte l'appelant gagne.** Une route dont la règle écarte cet appelant est passée, et la suivante est essayée. Le refus de la première est retenu, et c'est ce que l'appelant reçoit si rien d'autre ne répond.
- **Une règle `deny` ne passe jamais la main.** Elle refuse là, tout de suite.

## Le défaut qu'il ne faut pas se tromper

Une route qui ne déclare **aucune** règle d'accès est *déléguée* : la gateway ne
pose aucune condition, ne résout pas de session, et remet la requête à l'amont, qui
applique les règles qu'il a.

> [!WARNING]
> Aucune règle veut dire **pas filtré**. Cela ne veut pas dire "authentifié" et
> cela ne veut pas dire "public" - il n'y a pas de niveau *public* à choisir,
> parce que déléguer en est déjà un. Si une route doit exiger une session,
> dites-le : `auth` au minimum.

## À quoi ressemble un refus

Un refus sur une **route UI** atterrit sur une page, dans la langue du visiteur :
le sélecteur d'organisation quand changer d'organisation aiderait vraiment, la
salle d'attente quand le compte n'appartient nulle part, et sinon `/refused`, qui
nomme la règle qui l'a écarté et propose ce que cette session *peut* ouvrir.

Un refus sur une **route de service** est un `403` avec une phrase : personne ne lit
une page dans un `curl`. Un appelant sans aucune session reçoit un `401` et, sur une
navigation, une redirection vers la page de connexion.

## Ce qui n'existe pas

- **Aucun interrupteur global pour éteindre le contrôle d'accès.** L'équivalent se fait route par route : laissez une règle vide et cette route est déléguée.
- **Aucune usurpation d'identité.** Il n'y a pas de "se connecter en tant que" pour un administrateur, dans aucune édition.
