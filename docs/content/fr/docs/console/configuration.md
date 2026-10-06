---
title: Configuration
section: La console
order: 162
summary: Transporter la configuration d'une gateway - configurations nommées, points de reprise et snapshot complet de la base de données.
---

# Configuration

**Meerkat > Configuration**, réservé à root. Trois onglets, parce que trois questions
différentes se partageaient une seule page.

Ce qu'une configuration emporte : **les routes, les rôles, les autorités, le relais de
messagerie, les thèmes et les réglages de la gateway**. Ce qui reste sur place :
**les utilisateurs, les organisations, les sessions et le coffre** - ce n'est pas de la
configuration, c'est ce avec quoi vit cette gateway.

| Onglet | Ce à quoi il répond |
|---|---|
| **Management** | Ce qui tourne, ce qui est en réserve, et l'entrée et la sortie des fichiers |
| **History** | Revenir à n'importe quel moment de la configuration de cette gateway |
| **Snapshot** | Sauvegarder et restaurer la base de données entière |

Chaque onglet est une route à part entière : un favori y ramène donc directement.

![L'écran Configuration sur l'onglet Management : la configuration courante sur la première ligne, et deux configurations enregistrées en dessous](img/console/configuration.webp)

La configuration courante sur la première ligne, signalée comme identique à
*Known-good baseline*, et deux lignes de la réserve en dessous - dont l'une porte la
pastille *Current*. *Import a file* fonctionne dans les deux éditions ; *Import from
git* porte le badge Enterprise.

## Management

**La première ligne est ce que sert cette gateway.** Les lignes du dessous sont des
copies gardées en réserve. Enregistrer, dupliquer ou supprimer l'une d'elles ne change
rien à ce qui est servi.

L'icône de la première ligne est une comparaison, et c'est l'élément le plus utile de
l'écran :

| Icône | Ce qui tourne |
|---|---|
| verte | est identique à l'octet près à une configuration enregistrée - dont le nom figure à côté |
| orange | s'est écarté de la configuration sous laquelle il a été enregistré |
| grise | n'a jamais été enregistré sous aucun nom |

La ligne de la réserve dont provient l'état courant porte le même verdict sous forme de
pastille : *Current*, ou *Current, changed*.

**Les actions.** La ligne courante peut être enregistrée sous un nom et exportée. Une
ligne de la réserve peut être définie comme courante, dupliquée, exportée ou supprimée.
Un clic sur n'importe quelle ligne ouvre son fichier dans le tiroir, en
lecture.

Seule **Set as current** fait passer quelque chose de la réserve à la gateway :
c'est donc la seule action qui demande confirmation - et elle le fait en présentant
**la liste de ce qu'elle modifierait**, précédée d'un avertissement quand ce qui tourne
n'a jamais été enregistré : c'est la seule opération qui ne laisse rien à quoi revenir.

### Exporter

La boîte de dialogue indique ce que le fichier contient et ce qu'il ne contient pas,
puis propose deux formats :

- **Plain YAML** - du texte uniquement. Les images ne sont pas exportées, et un import
  conserve celles qui sont en place.
- **Package** - une archive zip : la configuration se lit et se compare comme du texte,
  et les images l'accompagnent sous forme de fichiers.

### Importer

**Import a file** accepte les fichiers `.yaml`, `.yml`, `.json` ou `.zip`, et demande
où ranger le contenu :

1. **Save it under a name** - en réserve, sans toucher à rien.
2. **Replace the current one** - ce qui ne figure pas dans le fichier disparaît.
3. **Add to the current one** - ce qui figure dans le fichier est ajouté ou mis à jour,
   et rien n'est supprimé.

Les deux options qui touchent à la gateway affichent d'abord leur plan, objet par
objet. Si le fichier fait référence à des entrées du coffre que cette gateway n'a
pas, une seconde boîte de dialogue les liste pour que vous puissiez les renseigner
aussitôt.

> [!NOTE]
> Toutes les actions fonctionnent dans les deux éditions - enregistrer, importer,
> dupliquer, définir comme courante, exporter. La différence tient à la taille de la
> réserve : l'édition Community conserve **trois** configurations enregistrées à la
> fois et indique où vous en êtes à côté du bouton Import (*2 of 3 saved*), avant que
> le plafond soit atteint ; l'édition Enterprise en conserve autant que vous le
> souhaitez.

### Comparer deux configurations enregistrées

Dès que la réserve en contient au moins deux, une ligne enregistrée propose **Compare
with another saved configuration** : choisissez la seconde, et la boîte de dialogue
liste ce qui change quand on passe de la première à la seconde - ajouts, mises à jour,
suppressions, objet par objet, avec les champs qui ont changé à l'intérieur d'une mise
à jour. Ce qui tourne n'entre pas en ligne de compte : c'est la question que l'on pose
à propos de la configuration d'un client et du modèle dont elle est issue, avant de
toucher à l'une ou à l'autre. `GET /api/configurations/{id}/compare/{other}`.

## Dépôt git

*Édition Enterprise.* Les configurations peuvent être conservées dans un dépôt git :
un historique où chaque modification a un auteur, une relecture avant tout changement,
et une source de vérité unique pour plusieurs installations.

Un export est **public par construction** - un champ déclaré secret n'en sort que sous
la forme de sa référence `${name}`, ou n'en sort pas du tout -, et c'est ce qui rend
raisonnable de le déposer dans un dépôt. Son contenu est en outre déterministe : deux
exports du même état produisent le même fichier, et un diff ne montre que ce qui a
changé.

### Des emplacements, pas des branches

Un **emplacement git** désigne un dépôt, une branche et un **répertoire** dans ce
dépôt. Plusieurs emplacements se partagent un même dépôt, à raison d'un répertoire par
plateforme :

```
platforms/acme/meerkat.yaml
platforms/acme/assets/logo.png
platforms/foo/meerkat.yaml
```

Cette arborescence est celle que produit déjà un export : il n'y a donc aucun fichier à
choisir, ni aucune convention à établir.

> [!TIP]
> Préférez un répertoire par plateforme sur une seule branche à une branche par
> plateforme. Une branche sert à porter un changement destiné à être fusionné ; les
> plateformes, elles, restent parallèles pour de bon. Avec une branche par plateforme,
> il faut reporter chaque correctif dans quatorze branches, indéfiniment. Réservez les
> branches à un changement **proposé** - autrement dit une pull request - et les tags
> à *ce que faisait tourner Acme le 2 du mois*.

### La boucle : récupérer, lire, activer

**Import from git** lit un emplacement et le range dans une configuration enregistrée,
et **n'applique rien**. La gateway continue de servir ce qu'elle servait ; ce que
vous obtenez en retour, c'est le plan - ce que l'activation ajouterait, modifierait et
supprimerait, ainsi que les entrées du coffre que la configuration attend et que cette
installation n'a pas. Vous lisez ce plan, puis vous cliquez sur **Set as current**
quand il annonce ce que vous attendiez, et vous le relisez une dernière fois au
passage.

Ce fonctionnement est voulu et n'est pas négociable : quiconque peut écrire sur cette
branche ne doit pas pouvoir reconfigurer une gateway. Il n'y a pas de réconciliation
continue, et aucune branche n'est surveillée.

**Export to git** crée un commit de la **copie enregistrée** - et non de l'état
courant : enregistrez-le d'abord sous un nom si c'est lui que vous voulez pousser - et
ce commit est **attribué à l'exploitant qui a cliqué**. C'est tout l'intérêt de le faire
dans le produit plutôt que dans un script : un dépôt dont l'historique affiche
*meerkat* pour chaque modification n'apprend rien à personne six mois plus tard.

Les deux sens sont simples : **un pull remplace** la configuration enregistrée par ce
que contient le dépôt, **un push remplace** le contenu du répertoire de l'emplacement
par la configuration enregistrée - quoi qu'une autre personne y ait déposé. Rien n'est
perdu : ce qu'un push a remplacé reste le commit précédent de l'historique. Seul le
répertoire de l'emplacement est écrit ; le répertoire d'une autre plateforme dans le
même dépôt n'est pas touché. Pour voir la version du dépôt avant de choisir,
récupérez-la dans une nouvelle configuration (*Import from git*) et comparez.

Chaque ligne indique où elle en est : *synced* avec la date du dernier push ou du
dernier pull, *changed here since*, ou *never pushed*. "Synced" correspond à ce que
cette gateway savait lors de son dernier échange - pour savoir si quelqu'un a
modifié le dépôt depuis, il suffit d'un pull.

### Le jeton

HTTPS avec un jeton. Le jeton est une référence au [coffre](/docs/console/vault),
jamais une valeur en clair - un emplacement est une ligne que la console lit et qu'un
snapshot emporte.

Les droits à accorder et le **nom d'utilisateur à envoyer avec le jeton** diffèrent
d'une forge à l'autre - et toutes signalent un nom d'utilisateur erroné par le même
*authentication failed* qu'un jeton erroné : c'est ainsi qu'un jeton parfaitement
valide fait perdre un après-midi à quelqu'un. Le formulaire commence donc par la
**forge** : GitHub, GitLab, Bitbucket Cloud, Azure DevOps, Gitea / Forgejo, ou un autre
serveur git. Une forge connue renseigne son hôte - modifiez-le pour une instance GitLab
ou Forgejo auto-hébergée - et vous ne saisissez que le dépôt (`owner/repository`) ;
l'URL est construite à partir des deux. En dessous figurent les étapes pour créer le
jeton dans les menus de cette forge, avec un lien vers la page des jetons de ce dépôt
précis, et le nom d'utilisateur qui sera envoyé - il ne vous est demandé que lorsque la
forge vous en laisse le choix :

| Forge | Nom d'utilisateur | Ce qu'il faut au jeton |
|---|---|---|
| **GitHub** | ignoré | Un jeton **fine-grained**, limité à ce dépôt, avec *Contents: Read and write*. Pas un jeton classique avec `repo` : celui-ci donne le contrôle total de tous les dépôts privés auxquels le compte a accès |
| **GitLab** | `oauth2` | Un jeton d'accès de projet avec `write_repository` et le rôle Maintainer (Developer si la branche n'est pas protégée) |
| **Bitbucket Cloud** | `x-token-auth` | Un jeton d'accès de dépôt avec `repository:write` |
| **Azure DevOps** | ignoré | Un jeton d'accès personnel avec *Code: Read & Write* |
| **Gitea / Forgejo** | le compte du jeton | Un jeton d'accès avec `write:repository` |

**Check** vérifie que le dépôt répond, que l'identifiant est accepté et que la branche
existe, sans rien écrire - un jeton erroné se détecte ainsi sur l'écran même où on le
configure.

### Depuis un agent

`list_git_locations`, `pull_configuration` et `push_configuration`
([agents](/docs/agent/overview)). Un agent qui demande à la gateway de pousser ne
détient aucun identifiant et n'a besoin d'aucun clone du dépôt : c'est le jeton rangé
dans le coffre, limité à un seul dépôt, qui fait le travail.

## History

La gateway tient son propre historique : **un point de reprise par modification**
qui change l'empreinte de la configuration, du plus récent au plus ancien, regroupés
par jour. Chaque ligne donne d'abord l'heure et la personne, puis les mots qu'a
employés le [journal d'audit](/docs/console/audit-and-issues) pour la même modification,
à la même seconde.

Ouvrez un point pour lire son fichier. **Restore** affiche son plan, comme toute autre
modification, avant de revenir en arrière, et laisse lui-même un point : l'historique
ne perd jamais l'état qu'on lui a demandé de quitter. Un point peut aussi être mis en
réserve sous un nom : c'est ainsi que *l'état de mardi dernier* devient une
configuration que vous conservez.

Une configuration enregistrée et un point de reprise sont volontairement deux objets
distincts : la première est intentionnelle et nommée (*la configuration d'Acme*), le
second automatique et horodaté (*l'état à 14 h 32*).

## Snapshot

Une copie cohérente de **toute la base de données**, prise pendant que la gateway
tourne : les routes et le coffre, mais aussi les utilisateurs, les organisations, les
sessions et le journal d'audit. C'est ce que restaure une sauvegarde ; un export de
configuration, lui, est ce que reproduit une seconde gateway.

Meerkat prend lui-même le snapshot, car copier avec `cp` une base de données en cours
d'utilisation peut la saisir en pleine écriture, et rien ne le signale avant le jour de
la restauration. La planification, la rétention et l'envoi hors de la machine relèvent
de votre outil de sauvegarde.

**Il n'y a pas de bouton de restauration, et c'est voulu.** Une base de données ne se
remplace pas sous le processus qui la tient ouverte. À la place, l'onglet affiche les
commandes exactes, avec les chemins propres à cette installation, et vous rappelle de
conserver l'ancien fichier jusqu'à ce que le nouveau ait fait ses preuves.

> [!WARNING]
> Si la clé maîtresse se trouve à côté de la base de données, les sauvegarder ensemble
> annule le chiffrement au repos - exactement comme si l'on rangeait un fichier de
> coffre à côté de sa phrase secrète. Conservez la clé dans un gestionnaire de secrets,
> ou fournissez-la par `MEERKAT_VAULT_KEY` ; l'onglet indique laquelle des deux
> solutions utilise cette gateway.


Le même onglet **déplace** la base, entre la base embarquée et PostgreSQL, vers un
fichier ou directement vers un serveur, avec l'interrupteur **Pause** qui rend la
copie complète : voir [déplacer la base](/docs/operations/backup-restore#dplacer-la-base--embarque-et-postgresql).

## Pièges

- **Un export n'emporte pas le coffre.** Prenez aussi le
  [fichier du coffre](/docs/console/vault), sans quoi la seconde gateway démarre
  avec des références qui ne pointent sur rien.
- **Replace supprime ce qui manque, Add non.** Lisez le plan ; c'est toute la
  différence entre une fusion et un écrasement.
- **Une configuration n'est pas une sauvegarde.** Les utilisateurs et les sessions n'en
  font pas partie. C'est à cela que sert l'onglet Snapshot.
- **L'image Community écarte les parties Enterprise.** Une configuration exportée d'une
  gateway Enterprise s'importe dans une gateway Community sans ses heures
  ouvrées, sa disposition, sa mention masquée, son export OpenTelemetry, ses annuaires,
  ni ses autorités et commandes ACME (la redirection HTTPS et HSTS sont conservés) ; le
  plan liste ce qui a été écarté.
- **Un pull depuis git ne change rien.** Il met un document en réserve et vous montre
  le plan. La gateway bascule quand quelqu'un active la configuration, pas quand le
  dépôt évolue.
- **Un emplacement appartient à cette installation, pas au document.** Il n'est jamais
  exporté : une configuration capable de faire pointer la gateway vers un autre
  dépôt écraserait le répertoire d'un autre client au push suivant.
