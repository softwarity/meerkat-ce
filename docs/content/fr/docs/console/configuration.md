---
title: Configuration
section: La console
order: 162
summary: Transporter la configuration d'une gateway - configurations nommées, points de reprise, et la base de données entière sauvegardée, restaurée et déplacée.
---

# Configuration

**Meerkat > Configuration**, réservé à root. Trois onglets, parce que trois questions
différentes se partageaient une seule page.

Ce qu'une configuration emporte : **les routes, les rôles, les tenants et leurs
groupes, les autorités, les clients OIDC, le relais de messagerie, les thèmes et les
réglages de la gateway**. Le secret d'un client OIDC ne voyage que sous forme de son
sha256 (`secretHash`), jamais en clair : le client importé ailleurs garde le même
secret, et l'application continue de fonctionner sans rien changer. Ce qui reste sur
place : **les utilisateurs et qui appartient à quoi, les sessions, et les valeurs du
coffre** (une configuration nomme les entrées qu'elle utilise, jamais leur contenu) -
ce n'est pas de la configuration, c'est ce avec quoi vit cette gateway.

| Onglet | Ce à quoi il répond |
|---|---|
| **Compose** | Ce qui diffère entre deux configurations, et ce que doit être le résultat |
| **History** | Revenir à n'importe quel moment de la configuration de cette gateway |
| **Database** | Sauvegarder, restaurer et déplacer la base de données entière |

Chaque onglet est une route à part entière : un favori y revient.

## Compose

![L'onglet Compose : ce qui tourne à gauche, une configuration enregistrée à droite, le résultat entre les deux, chaque différence encadrée de rouge](img/console/configuration.webp)

Trois colonnes : **ce dont on part** à gauche, en bleu ; **le résultat** au milieu, en
vert ; **ce à quoi on le compare** à droite, en orange. Rien ne bouge tant qu'un résultat
n'est pas appliqué ou enregistré : cet écran est un établi, pas un interrupteur.

L'écran s'ouvre avec ce qui tourne des deux côtés, et dit donc que les deux sont
identiques. Choisissez autre chose dans l'en-tête d'une colonne :

| Source | Ce que c'est |
|---|---|
| **Running** | Ce que cette gateway sert |
| **Une configuration enregistrée** | Une des copies gardées sur l'étagère |
| **Un emplacement git** | Le répertoire d'un dépôt, lu à cet instant (*Enterprise*) |
| **Un fichier** | Un YAML, un JSON ou un paquet (`.zip`) choisi ou déposé sur la page. Il arrive à droite ; il est lu, pas gardé |

### Décider

**Chaque différence est encadrée de rouge**, et le cadre disparaît une fois touchée. Un
ruban relie ce qui nourrit le résultat à sa place au milieu. Pour décider :

- **Un objet présent d'un seul côté** a un bouton radio de chaque côté : pris, ou
  laissé dehors.
- **Un objet présent des deux côtés qui diffère** s'ouvre en ses champs, jusqu'aux
  feuilles : `timeouts.response` d'un côté et `upstream` de l'autre donnent un seul
  résultat. Un bouton radio sur chaque valeur dit laquelle le résultat retient, et le
  **x** d'une ligne du résultat laisse la valeur vide.
- **Une liste** se présente en puces : une puce pleine est dans le résultat, une puce
  pâle en pointillés n'y est pas. Un clic l'ajoute ou la retire, un glisser la réordonne.
- **Un objet commun aux deux côtés** a *in the result* coché : décochez pour le laisser
  dehors malgré tout.

La double coche dans l'en-tête d'une colonne prend ce côté pour tout ce qui reste à
décider, et **Reset** à côté du résultat oublie tous les choix. **Only differences**
masque ce qui est identique, et les puces au-dessus des colonnes filtrent par type
d'objet.

Le *rang* d'une route dans le routage n'est jamais proposé : deux routes au même rang
rendraient le routage ambigu, le résultat garde donc ce que dit le côté gauche.

**Les dépendances suivent.** Une route prise à droite emporte les rôles qu'elle nomme,
depuis le côté qui les a. Laissez un tel rôle dehors et la référence est signalée comme
manquante ; seuls le parent d'un rôle ou le tenant d'un groupe bloquent l'application,
car la gateway ne peut pas tenir l'objet sans eux.

**Deux objets de même nom sous deux identifiants** sont proposés comme paire, jamais
appariés d'office. Appariés, le résultat garde l'identifiant de gauche et la gateway met
l'objet à jour au lieu d'en retirer un et d'en ajouter un autre.

### Le portail, dessiné

![Le réglage du portail : sous ses champs, une miniature de la barre décrite par chaque côté](img/console/configuration-portal.webp)

Un catalogue se lit mal en liste : sous les champs du réglage du portail, chaque colonne
montre donc une **miniature de la vraie barre**. Cliquez une entrée dans une miniature
pour l'ouvrir : un conteneur montre ses modules, et c'est ainsi que les sous-routes se
comparent.

### Ce qui manquerait au résultat

Un bandeau au-dessus des colonnes, et un compteur dans l'en-tête du résultat, disent ce
qui manque **avant** que quoi que ce soit ne soit appliqué :

- les entrées du coffre que le document nomme et que cette gateway ne détient pas
  (réservées vides à l'application, à remplir sur l'écran du
  [coffre](/docs/console/vault)) ;
- les specs et fichiers qu'un fichier simple ne porte pas, quand la source en est un ;
- ce que cette édition n'applique pas ;
- les liaisons OIDC en suspens, et les dépendances qu'un choix a laissées dehors.

### Que faire du résultat

![Le menu Save du résultat : l'appliquer, le garder sur l'étagère, l'enregistrer sur le disque](img/console/configuration-save.webp)

**Save** dans l'en-tête du résultat :

| Entrée | Ce qu'elle fait |
|---|---|
| **Apply as current** | Demande d'abord, en chiffres et en noms, avec ce qui serait retiré et ce qui manque. La gateway sert ensuite le résultat, et un [point de reprise](#history) garde ce qu'elle servait avant |
| **On the shelf** | Garde le résultat sous un nom, sans rien appliquer. Un nom qui existe demande à le remplacer |
| **On the filesystem** | Ouvre une fenêtre pour choisir la forme, et dit ce qu'elle laisse derrière |
| **Push to...** | Proposé quand un emplacement git est chargé à droite : voir [Dépôt git](#dpt-git) |

Running des deux côtés et **On the shelf** nomment ce qui tourne. Une configuration
enregistrée à gauche et **On the filesystem** l'exportent.

![Enregistrer sur le disque : un fichier ou un paquet, et ce que la forme choisie laisse derrière](img/console/configuration-file.webp)

La fenêtre propose deux formes, et liste ce que la forme choisie laisse derrière :

- **As a file** - YAML simple, du texte seul. Les images, les specs et les fichiers d'une
  route restent derrière ; l'importer garde les images en place.
- **As a package with its assets** - un zip : la configuration se lit et se compare en
  texte, ses fichiers sont à côté. Proposé d'abord quand il y a quelque chose à emporter,
  grisé sinon.

Aucune des deux ne porte les valeurs des entrées du coffre qu'elle nomme, ni un secret
détenu en clair sur cette gateway : la fenêtre les nomme, pour qu'ils puissent être
déplacés ou copiés avant que le fichier ne parte.

### L'étagère

L'étagère est la liste des configurations enregistrées : des copies, qui ne changent
rien à ce qui est servi tant qu'aucune n'est appliquée. Pour en charger une, choisissez-la
dans l'en-tête d'une colonne. Le bouton **More** de cette colonne agit alors dessus :

| Entrée | Ce qu'elle fait |
|---|---|
| **Read the file** | L'ouvre dans le tiroir, pour la lire et la corriger. Enregistrer remplace son document et n'applique rien. Sur Running, le fichier se lit mais ne se modifie pas : ce qui tourne change sur cet écran, où un changement montre d'abord ce qu'il fait |
| **Rename** | Renomme la copie |
| **Delete** | Supprime la copie ; la colonne revient sur Running |

> [!NOTE]
> Tous les gestes existent dans les deux éditions. Ce qui diffère est la taille de
> l'étagère : l'édition communautaire garde **trois** configurations enregistrées à la
> fois et dit où vous en êtes dans le menu Save (*2 of 3 saved*), avant d'atteindre le
> plafond ; l'Enterprise en garde autant que vous voulez.

### Depuis l'API

L'écran compose sur le serveur à chaque clic : un agent rejoue donc la même recette.
`POST /api/configurations/compose` donne l'aperçu, et `/compose/apply`, `/compose/save`,
`/compose/export` et `/compose/push` agissent sur le résultat. L'outil
[agent](/docs/agent/overview) est `compose_configuration`.

Pour un script qui n'a pas besoin de recette, deux appels simples déplacent un document
entier : `GET /api/config/export` lit ce qui tourne, et `POST /api/config/import` applique
un fichier (`?prune=true` retire ce qu'il ne porte pas, `?keep=kind:id` garde une ligne
telle que la gateway la détient). `POST /api/configurations` nomme ce qui tourne.

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

### Récupérer, c'est charger ; pousser, c'est trancher

**Pour lire un emplacement**, choisissez-le dans l'en-tête d'une colonne. Il est lu à cet
instant, rien n'est stocké et **rien n'est appliqué** : la gateway continue de servir ce
qu'elle servait. À partir de là, c'est un côté comme un autre : composer, appliquer,
garder sur l'étagère.

C'est délibéré et ce n'est pas négociable : qui peut écrire sur cette branche ne doit pas
pouvoir reconfigurer une gateway. Il n'y a pas de réconciliation continue, et aucune
branche n'est surveillée.

**Pour écrire un emplacement**, chargez-le **à droite** et tranchez ce qui diffère :
**Save > Push to...** apparaît alors, nommant cet emplacement et aucun autre. Un push
remplace ce que contient le répertoire de l'emplacement par le résultat : il ne va donc
que là où vous venez de regarder, à la révision lue. Si quelqu'un a poussé entre-temps, il
est refusé et demande de relire le côté droit : il n'y a pas de force. Une fois poussé, le
côté droit est relu, et les deux se lisent comme le dépôt les tient désormais.

Un push écrit le dépôt et **n'enregistre rien ici**. Le commit est **attribué à
l'opérateur qui a cliqué**. C'est tout l'intérêt de le faire dans le produit plutôt que
dans un script : un dépôt dont l'historique dit *meerkat* pour chaque changement ne répond
à rien six mois plus tard. Rien n'est perdu non plus : ce qu'un push a remplacé est le
commit précédent de l'historique. Seul le répertoire de l'emplacement est écrit ; celui
d'une autre plateforme dans le même dépôt reste intact.

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

`list_git_locations`, `save_git_location` (créer ou modifier un emplacement ; le jeton reste une référence du coffre), `pull_configuration` et `push_configuration`
([agents](/docs/agent/overview)). Un agent travaille sur des configurations enregistrées :
un pull rafraîchit une copie enregistrée, un push en committe une vers l'emplacement qu'il
nomme, et la gateway retient quel emplacement et quelle révision pour cette copie. Un agent
qui demande à la gateway de pousser ne détient aucun identifiant et n'a besoin d'aucun
clone du dépôt : c'est le jeton rangé dans le coffre, limité à un seul dépôt, qui fait le
travail.

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

## Database

![L'onglet Database : la pause, la base sur laquelle tourne la gateway et la plateforme qui la déploie, le téléchargement et les étapes de restauration](img/console/database.webp)

Une copie cohérente de **toute la base de données**, prise pendant que la gateway
tourne : les routes et le coffre, mais aussi les utilisateurs, les tenants, les
sessions et le journal d'audit. C'est ce que restaure une sauvegarde ; un export de
configuration, lui, est ce que reproduit une seconde gateway. L'onglet s'appelait
*Snapshot* tant qu'il ne faisait que sauvegarder ; l'ancienne adresse mène toujours ici.

Meerkat prend lui-même le snapshot, car copier avec `cp` une base de données en cours
d'utilisation peut la saisir en pleine écriture, et rien ne le signale avant le jour de
la restauration. La planification, la rétention et l'envoi hors de la machine relèvent
de votre outil de sauvegarde.

**Il n'y a pas de bouton de restauration, et c'est voulu.** Une base de données ne se
remplace pas sous le processus qui la tient ouverte. À la place, l'onglet affiche les
commandes exactes, avec les chemins propres à cette installation, **pour la plateforme
sur laquelle elle tourne** - le runtime le dit : un Deployment Kubernetes ou OpenShift
et sa release Helm, un service Swarm, un projet compose, un conteneur. *Written for* en choisit une autre, pour un runtime que la gateway n'a pas
pu interroger. L'ancien fichier est mis de côté jusqu'à ce que le nouveau ait fait ses
preuves.

> [!WARNING]
> Si la clé maîtresse se trouve à côté de la base de données, les sauvegarder ensemble
> annule le chiffrement au repos - exactement comme si l'on rangeait un fichier de
> coffre à côté de sa phrase secrète. Conservez la clé dans un gestionnaire de secrets,
> ou fournissez-la par `MEERKAT_VAULT_KEY` ; l'onglet indique laquelle des deux
> solutions utilise cette gateway.

Le même onglet **déplace** la base, entre la base embarquée et PostgreSQL, vers un
fichier ou directement vers un serveur. Un déplacement est **une seule procédure
numérotée**, affichée dès le départ : créer la base et son utilisateur (le SQL que
lance un administrateur de bases de données), tester le serveur, mettre en pause,
copier, basculer le déploiement, vérifier une route et une connexion, et le chemin du
retour - chaque étape cochée quand elle est faite.

![Le passage à PostgreSQL avant la copie : la cible, le SQL qui la crée, les étapes qui restent](img/console/database-migrate.webp)

La gateway se souvient de la copie - où elle est allée et ce qu'elle a emporté, jamais
le mot de passe - si bien qu'un rechargement au milieu d'une migration, sur n'importe
quel nœud, ramène à la même procédure. **Dismiss** l'oublie ; elle s'efface d'elle-même
quand la gateway tourne sur cette base. Voir
[déplacer la base](/docs/operations/backup-restore#dplacer-la-base--embarque-et-postgresql).

## Pièges

- **Un export n'emporte pas le coffre.** Prenez aussi le
  [fichier du coffre](/docs/console/vault), sans quoi la seconde gateway démarre
  avec des références qui ne pointent sur rien.
- **Une configuration n'est pas une sauvegarde.** Les utilisateurs et les sessions n'en
  font pas partie. C'est à cela que sert l'onglet Database.
- **L'image Community écarte les parties Enterprise.** Une configuration exportée d'une
  gateway Enterprise s'importe dans une gateway Community sans ses heures
  ouvrées, sa disposition, sa mention masquée, son export OpenTelemetry, ses annuaires,
  ses clients OIDC, ni ses autorités et commandes ACME (la redirection HTTPS et HSTS
  sont conservés) ; le plan liste ce qui a été écarté. Une route liée à un client OIDC
  garde son mécanisme, sans qu'il soit servi.
- **Un import ne fait pas perdre son client OIDC à une route.** Un client qu'une route
  utilise encore est gardé, même par un document qui élague sans le lister : une
  configuration exportée avant que les clients voyagent s'active toujours. Une route qui
  nomme un client que ni le document ni la gateway ne déclarent s'importe telle quelle,
  et le plan le dit : déclarez le client sous **OIDC / JWT**.
- **Lire un emplacement git ne change rien.** C'est un côté de l'écran. La gateway
  bascule quand quelqu'un applique un résultat, pas quand le dépôt évolue.
- **Un emplacement appartient à cette installation, pas au document.** Il n'est jamais
  exporté : une configuration capable de faire pointer la gateway vers un autre
  dépôt écraserait le répertoire d'un autre client au push suivant.
