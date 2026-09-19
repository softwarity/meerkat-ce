---
title: La règle d'accès d'une route
section: Contrôle d'accès
order: 134
summary: La règle qui dit qui peut atteindre une route - niveau, rôles, comptes nommés - et ce qui se passe quand la route ne dit rien.
---

# La règle d'accès d'une route

Chaque route porte une règle, dans la section **Security** de son éditeur. Elle a
trois parties, et ce ne sont pas des alternatives :

- **un niveau** : quel genre d'appelant, en termes de session et d'organisation ;
- **des rôles** : lesquels, détenus dans l'organisation active ;
- **des comptes nommés** : les personnes qui passent quand même.

Le niveau et les rôles se combinent en **ET**. Les comptes nommés se combinent en
**OU** par-dessus tout le reste.

## Les niveaux

| Niveau | Qui passe |
|---|---|
| **Delegated** (rien de choisi) | tout le monde, connecté ou non. Aucune session n'est même cherchée ; le service décide |
| **Signed in** | quiconque a un compte - y compris un compte qui n'appartient encore à aucune organisation |
| **In an organisation** | une session avec une organisation active. Écarte un compte encore en attente d'accès |
| **In one of these organisations** | l'organisation active doit être l'une de celles que vous nommez |
| **Nobody** | refusé avant que le service soit appelé. Seuls les comptes nommés passent |

Il n'y a pas de niveau *public*, parce que déléguer en est déjà un, et parce qu'un
niveau appelé public promettrait ce que la gateway ne peut pas garantir : votre
service peut toujours refuser.

Que *Signed in* laisse passer un compte sans organisation est délibéré : c'est ce qui
rend atteignable une page de salle d'attente, ou un profil en self-service, pour
quelqu'un qui vient d'être reconnu et à qui rien n'a été accordé.

![La règle propre d'une route : un niveau, les rôles acceptés, et les utilisateurs qui passent malgré tout](img/console/route-editor-security.webp)

## Les rôles

Choisissez-en autant que vous voulez ; **n'importe lequel suffit**. Ils sont lus dans
l'organisation *active*, et ils comprennent tout ce que les rôles de la personne
impliquent par la hiérarchie.

Comme le catalogue est global tandis que les groupes appartiennent à une
organisation, les deux axes disent des choses réellement différentes :

| Règle | Signifie |
|---|---|
| rôle `billing-admin`, niveau *In an organisation* | un administrateur de facturation de l'organisation active, quelle qu'elle soit - une console inter-organisations |
| rôle `billing-admin`, niveau *In one of these*, Acme | un administrateur de facturation **d'Acme** |

Laisser les rôles vides signifie que n'importe quel rôle passe : le niveau seul
décide.

> [!NOTE]
> Une règle qui demande un rôle exige toujours une organisation aussi, que vous
> l'ayez dit ou non : les rôles n'existent qu'à l'intérieur d'une organisation. En
> pratique, choisissez *In an organisation* dès que vous nommez un rôle.

## Les comptes nommés

Une liste d'identifiants qui passe **quoi que le niveau exige** - *Nobody* compris.
C'est le mécanisme d'exception : un compte de service, un identifiant de support,
une application dédiée à une seule personne.

C'est aussi comme cela que s'écrit "seulement ces gens-là" : niveau **Nobody**,
plus les identifiants. Une règle qui nomme des comptes mais ne pose ni niveau ni
rôle n'est **pas une règle**, et les noms sont abandonnés à l'enregistrement : sous
une route déléguée tout le monde passe déjà, donc nommer quelqu'un ne dit rien.

## Quand une route ne dit rien

La requête est proxifiée. Aucune session n'est résolue, aucun cookie n'est lu, rien
n'est refusé.

> [!WARNING]
> Une règle vide n'est **pas** "authentifié" et **pas** "public" : elle est *non
> filtrée*. Meerkat ajoute des conditions, il ne retire jamais celles que votre
> service applique. Si une route doit exiger une session, choisissez *Signed in* au
> minimum.

## Plusieurs routes, une requête

La règle d'une route fait partie du **choix** de la route, ce n'est pas quelque chose
qu'on applique après. Cela a une conséquence surprenante et utile : une route dont la
règle écarte cet appelant est **passée**, et la route suivante qui matche est
essayée.

Deux routes sur le même chemin servent donc deux publics - une page riche pour les
membres d'Acme, une page d'accueil pour les autres - simplement en les ordonnant. Le
refus de la première est retenu et délivré seulement si aucune autre route ne
répond.

La seule exception est **Nobody** : un `deny` refuse sur place et ne passe jamais la
main. C'est ainsi qu'on ferme un chemin au lieu de le réouvrir plus bas.

## À quoi ressemble un refus

Sur une **route UI**, la personne atterrit sur une page dans sa langue :

| Situation | Où elle atterrit |
|---|---|
| Changer d'organisation lèverait le refus | `/select-tenant`, qui dit pourquoi |
| Elle n'appartient à aucune organisation et la règle en exige une | `/account-pending`, la salle d'attente |
| Tout le reste | `/refused`, qui nomme la règle qui l'a écartée et liste ce que cette session peut ouvrir |
| Aucune session | la page de connexion, la destination conservée |

La proposition de changer d'organisation est filtrée par la réalité : elle n'est
faite que si une autre organisation de la personne satisfait effectivement la règle.

Sur une **route de service**, la réponse est un `403` avec une phrase nommant ce qui
manquait - l'organisation, l'un de ces rôles, cet endpoint est fermé - ou un `401`
avec `WWW-Authenticate: Session` quand il n'y a pas de session du tout. Personne ne
lit une page HTML dans un `curl`.
