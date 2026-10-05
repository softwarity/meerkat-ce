---
title: Jetons d'API
section: Authentification
order: 116
summary: Les jetons personnels pour les appels de machine à machine à travers la gateway, et pourquoi les jetons du plan de contrôle sont tout autre chose.
---

# Jetons d'API

Il existe deux types de jetons. Ils se ressemblent trait pour trait, et ils n'ont pourtant
rien à voir :

| | Jeton d'API personnel | Jeton du plan de contrôle |
|---|---|---|
| Sert à | appeler vos applications **à travers** la gateway | appeler l'**API d'administration** de la gateway, ou y connecter un agent |
| Fonctionne sur | le port du plan de données | le port d'administration uniquement |
| Créé par | toute personne connectée, depuis son profil | toute personne qui administre un domaine (root, infra-admin, app-admin), depuis la console, pour elle-même |
| Géré dans | `/profile/tokens` sur le plan de données ; les administrateurs des applications le voient et le révoquent dans **Access tokens** | **Access tokens**, sous Infra ou Application |
| Porte | l'organisation et le groupe de la session qui l'a créé | un périmètre : jusqu'où, sur quoi, depuis où |

Les deux se présentent de la même façon, et de celle-là seulement :

```bash
curl -H 'Authorization: Bearer mk_...' https://apps.acme.io/orders
```

Pas de paramètre de requête, pas d'autre en-tête. La chaîne commence par `mk_` : une
simple recherche la retrouve ainsi dans un journal, et un détecteur de secrets la
reconnaît. Quand une requête porte aussi un cookie de session, c'est le **cookie** qui est
utilisé : le jeton n'est lu qu'en l'absence de cookie.

Les deux types sont stockés dans la même table et rien ne les distingue à l'œil. Seul le
port qu'ils ouvrent les différencie.

## Jetons d'API personnels

Ils sont destinés à un script, à une tâche cron ou à un service qui doit atteindre une
application derrière la gateway au nom d'une personne réelle.

Ils sont autorisés par défaut. L'interrupteur s'appelle *Allow personal API tokens*, dans
**Application > Security**. Quand il est désactivé, la page n'existe pas - et les jetons
existants cessent de fonctionner.

**Créer un jeton** se fait depuis `/profile/tokens` : un nom, et une durée de validité à
choisir entre trente, soixante, quatre-vingt-dix ou trois cent soixante-cinq jours, ou sans
expiration. Quatre-vingt-dix jours est la valeur présélectionnée. Le secret n'est affiché
qu'**une seule fois**, dans une boîte de dialogue ; ensuite, la liste n'en montre plus que
les douze premiers caractères et la date de dernière utilisation (mise à jour au plus une
fois par minute).

**Ce qu'il porte, c'est le contexte de la session qui l'a créé** : l'organisation active
et, en mode groupe exclusif, le groupe actif. Il n'y a rien à sélectionner. Si vous avez
besoin d'un jeton pour une autre organisation, basculez d'abord sur celle-ci, puis créez le
jeton : la page vous indique le contexte qu'elle s'apprête à retenir, et vous prévient
quand il n'y en a aucun.

**Ce qu'il ne porte pas, ce sont les rôles.** Les rôles sont recalculés à chaque requête à
partir du compte, de l'organisation et du groupe. Un rôle accordé demain s'applique donc à
un jeton créé aujourd'hui, et un rôle retiré cesse aussitôt de s'appliquer.

**La révocation** se fait sur la même page : vous désactivez un jeton pour le mettre en
sommeil, vous le révoquez pour le détruire. Dans les deux cas, l'effet se propage à toutes
les gateways en quelques secondes. Un jeton cesse aussi de fonctionner dès que son
titulaire est désactivé, sort de sa période d'accès ou est supprimé.

Les personnes qui administrent les applications (root, app-admin) voient en outre les
jetons d'application de **tout le monde** sur l'écran **Access tokens** de la console, avec
leur titulaire, leur organisation et leur dernière utilisation. Elles peuvent en révoquer
un depuis son tiroir : le jeton que porte encore le script d'un collègue parti, ou
celui qui a fuité. Elles n'en créent jamais à cet endroit : seul son titulaire peut créer
un moyen d'authentification. La révocation laisse une ligne `token.revoke` dans le
[journal d'audit](/docs/operations/audit), au nom du titulaire.

> [!WARNING]
> Se déconnecter ne révoque **pas** vos jetons, et changer votre mot de passe depuis le
> profil non plus : seule une réinitialisation par le lien reçu par e-mail le fait. Un
> jeton est un moyen d'authentification indépendant, avec sa propre durée de vie :
> révoquez-le explicitement.

## Jetons du plan de contrôle

Ceux-ci ouvrent l'API d'administration et l'endpoint réservé aux agents. Toute personne qui
administre un domaine crée les **siens**, et ne voit que les siens. Un jeton agit avec les
droits de son titulaire, relus à chaque appel, et son périmètre ne fait jamais qu'en
retirer : il ne donne donc jamais plus que ce que détient son titulaire.

L'écran est **Access tokens** (sous Infra ou Application, c'est le même) : un seul tableau
pour les deux types, le détail et les actions se trouvant dans le tiroir d'une
ligne. Les jetons d'un agent connecté n'y figurent pas : la connexion d'un agent se gère
dans la section **MCP**, qui crée le même type de jeton par un parcours de consentement
plutôt que par un secret à recopier.

### Le périmètre, sur deux axes

Un périmètre ne fait jamais que **retirer**. Il n'accorde jamais ce que le titulaire n'a
pas déjà.

**Jusqu'où** - la portée (scope) :

| Portée | Ce qu'elle ouvre |
|---|---|
| `schedules` | `/api/schedules` et rien d'autre : c'est le jeton d'un service qui gère ses [appels planifiés](/docs/operations/scheduler) |
| `readonly` | les lectures, et les outils de test. Ce qui compte comme une lecture se décide endpoint par endpoint, pas d'après le verbe HTTP |
| `full` | tout ce que son titulaire a le droit de faire |

Une portée vide vaut `readonly` : c'est la valeur sûre quand rien n'a été précisé.

Les deux portées restreintes existent pour la même raison : ces jetons-là vivent dans un
fichier de configuration ou un manifeste de déploiement, souvent dans le dépôt d'une autre
équipe, et ce sont ceux que personne ne pense à renouveler. Ce qu'ils ouvrent en cas de
fuite doit tenir en une ligne.

Un jeton de ce plan est un jeton d'ADMINISTRATEUR : il agit avec les pouvoirs de la
personne qui l'a créé, et on ne le crée que pour soi. Ce n'est pas ici que l'on donne à un
compte un accès fin aux APPLICATIONS : il faut pour cela un jeton du plan de données, créé
depuis le profil de ce compte, avec les droits de ce compte.

**Depuis où** - une liste de plages CIDR ; vide, elle signifie "de partout". Elle
s'applique à l'**adresse du pair TCP**, jamais à un en-tête transmis par un intermédiaire.

> [!WARNING]
> Derrière un reverse proxy, le plan de contrôle voit l'adresse du proxy et non celle de
> l'appelant. Une restriction par adresse n'a de sens que si ce qui utilise le jeton
> atteint directement le port.

### Au quotidien

Le nom d'un jeton, sa portée, ses plages d'adresses et sa date d'expiration se modifient
tous **sans changer le secret**. *Renew* fait l'inverse : il remplace le secret et conserve
tout le reste, et l'ancien secret cesse immédiatement de fonctionner.

Toute modification faite avec un jeton est inscrite au journal d'audit avec le nom du jeton
à côté du compte - `admin, via claude-desktop`, et non `admin`. On peut ainsi retrouver
après coup qu'un changement vient d'un agent.

Ni l'interrupteur *Allow personal API tokens* ni aucun autre réglage de l'écran Security de
l'application n'a d'effet sur eux : un jeton du plan de contrôle est le moyen
d'authentification d'un administrateur, il ne relève pas de cette politique.
