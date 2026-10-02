---
title: Jetons d'API
section: Authentification
order: 116
summary: Les jetons personnels pour les appels machine à travers la gateway, et pourquoi les jetons du plan de contrôle sont tout autre chose.
---

# Jetons d'API

Il y a deux genres de jetons, ils se ressemblent, et ce ne sont pas du tout la
même chose :

| | Jeton d'API personnel | Jeton du plan de contrôle |
|---|---|---|
| Pour | appeler vos applications **à travers** la gateway | appeler l'**API d'administration** de la gateway, ou brancher un agent |
| Fonctionne sur | le port du plan de données | le port d'administration seulement |
| Émis par | toute personne connectée, depuis son profil | quiconque administre un domaine (root, infra-admin, app-admin), depuis la console, pour lui-même |
| Géré dans | `/profile/tokens` sur le plan de données ; vus et révoqués par les administrateurs des applications sur **Access tokens** | **Access tokens**, sous Infra ou Application |
| Porte | l'organisation et le groupe de la session qui l'a émis | un périmètre : jusqu'où, sur quoi, depuis où |

Les deux se présentent de la même façon, et seulement de cette façon :

```bash
curl -H 'Authorization: Bearer mk_...' https://apps.acme.io/orders
```

Pas de paramètre d'URL, pas d'en-tête alternatif. La chaîne commence par `mk_`,
donc elle se retrouve dans un journal et se fait repérer par un scanner de
secrets. Une requête qui porte aussi un cookie de session utilise le **cookie** :
le jeton n'est lu que s'il n'y a pas de cookie.

Les deux genres vivent dans la même table et se ressemblent à l'oeil nu. Seul le
port qu'ils ouvrent les distingue.

## Jetons d'API personnels

Faits pour un script, une tâche planifiée ou un service qui doit atteindre une
application derrière la gateway en tant que personne réelle.

Ils sont permis par défaut ; l'interrupteur est *Allow personal API tokens* dans
**Application > Security**. Éteint, la page n'existe pas - et les jetons existants
cessent de fonctionner.

**En émettre un**, depuis `/profile/tokens` : un nom, et une durée choisie parmi
trente, soixante, quatre-vingt-dix ou trois cent soixante-cinq jours, ou jamais.
Quatre-vingt-dix jours est présélectionné. Le secret est affiché **une fois**, dans
une boîte de dialogue ; ensuite la liste n'en montre que les douze premiers
caractères et la date de dernier usage (estampillée au plus une fois par minute).

**Ce qu'il porte est le contexte de la session qui l'a créé** : l'organisation
active et, en mode de groupe exclusif, le groupe actif. Il n'y a pas de sélecteur.
Pour un jeton dans une autre organisation, basculez d'abord puis émettez-le : la
page dit quel contexte elle est sur le point de capturer, et le dit aussi quand il
n'y en a aucun.

**Ce qu'il ne porte pas, ce sont les rôles.** Les rôles sont recalculés à chaque
requête depuis le compte, l'organisation et le groupe : un rôle accordé demain
s'applique à un jeton émis aujourd'hui, et un rôle retiré cesse aussitôt de
s'appliquer.

**Le révoquer** se fait sur la même page : désactiver pour le mettre de côté,
révoquer pour le détruire. L'un comme l'autre prend effet sur toutes les
passerelles en quelques secondes. Un jeton s'arrête aussi dès que son propriétaire
est désactivé, sort de sa fenêtre d'accès, ou est supprimé.

Ceux qui administrent les applications (root, app-admin) voient aussi les jetons
d'application de **tout le monde** sur l'écran **Access tokens** de la console, avec leur
propriétaire, leur organisation et leur dernière utilisation, et en révoquent un depuis
son tiroir : le jeton qu'un script d'un collègue parti porte encore, ou celui qui a fuité.
Ils n'en créent jamais là - une crédentiale n'est émise que par son propriétaire. La
révocation est une ligne `token.revoke` du [journal d'audit](/docs/operations/audit), sur
le propriétaire.

> [!WARNING]
> Se déconnecter ne révoque **pas** vos jetons, et changer de mot de passe depuis le
> profil non plus - seule une réinitialisation par le lien envoyé par courriel le fait.
> Un jeton est un identifiant indépendant avec sa propre durée de vie : révoquez-le
> explicitement.

## Jetons du plan de contrôle

Ceux-là ouvrent l'API d'administration et le point d'entrée de l'agent. Quiconque
administre un domaine émet les **siens**, et ne voit que les siens : un jeton agit avec
les capacités de son propriétaire, relues à chaque appel, et son périmètre ne fait
qu'enlever - il ne donne donc jamais plus que ce que son propriétaire détient.

L'écran est **Access tokens** (sous Infra ou Application, le même) : un seul tableau pour
les deux sortes, le détail et les actions dans le tiroir d'une ligne. Les jetons qui appartiennent à un agent
branché n'y figurent pas : le branchement d'un agent se gère dans la section
**MCP**, qui émet le même genre de jeton par un flux de consentement plutôt que
par un secret recopié.

### Le périmètre, sur deux axes

Un périmètre ne fait jamais que **retirer**. Il n'accorde jamais ce que le
propriétaire n'a pas déjà.

**Jusqu'où** - la portée :

| Portée | Ce qu'elle ouvre |
|---|---|
| `schedules` | `/api/schedules` et rien d'autre : l'identifiant d'un service qui gère ses [appels planifiés](/docs/operations/scheduler) |
| `readonly` | les lectures, et les testeurs. Ce qui compte comme une lecture est décidé par point d'entrée, pas par le verbe HTTP |
| `full` | tout ce que son propriétaire peut faire |

Une portée vide se lit `readonly` : la valeur sûre quand rien n'a été dit.

Les deux portées étroites existent pour la même raison : ces jetons-là vivent
dans un fichier de configuration ou un manifeste, souvent dans le dépôt d'une
autre équipe, et ce sont ceux que personne ne pense à faire tourner. Ce qu'ils
ouvrent quand ils fuient doit donc tenir en une ligne.

Un jeton de ce plan est un jeton d'**administration** : il agit avec les
pouvoirs de qui l'a frappé, et on le frappe pour soi. Pour donner à un compte un
accès fin **aux applications**, ce n'est pas ici : c'est un jeton du plan de
données, frappé depuis le profil de ce compte, avec ses droits à lui.

**Depuis où** - une liste de plages CIDR, vide signifiant n'importe où. Elle est
jugée sur l'**adresse du pair TCP**, jamais sur un en-tête transmis.

> [!WARNING]
> Derrière un proxy inverse, le plan de contrôle voit l'adresse du proxy et non
> celle de l'appelant. Une restriction d'adresse n'a de sens que si ce qui utilise
> le jeton atteint le port directement.

### Vivre avec

Le nom, la portée, les plages et l'échéance d'un jeton se modifient
tous **sans changer le secret**. *Renew* fait l'inverse : il fait tourner le secret
et garde tout le reste, et l'ancien secret meurt dès son prochain usage.

Chaque écriture faite avec un jeton est auditée avec le nom du jeton à côté du
compte - `admin, via claude-desktop`, et non `admin` - de sorte qu'un changement
fait par un agent reste attribuable après coup.

Ni l'interrupteur *Allow personal API tokens*, ni rien d'autre sur l'écran Security
de l'application ne les concerne : un jeton du plan de contrôle est une crédentiale
d'administrateur, pas un élément de cette politique.
