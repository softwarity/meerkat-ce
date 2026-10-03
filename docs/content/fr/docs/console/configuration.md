---
title: Configuration
section: La console
order: 162
summary: Déplacer le réglage d'une passerelle : configurations nommées, points de reprise, et instantané complet de la base.
---

# Configuration

**Infra > Configuration**, root seulement. Trois onglets, parce que trois questions
différentes partageaient une page.

Ce qui voyage dans une configuration : **routes, rôles, autorités, relais mail, thèmes
et réglages de passerelle**. Ce qui reste : **utilisateurs, organisations, sessions et
le coffre** - ce n'est pas de la configuration, c'est ce avec quoi cette passerelle
vit.

| Onglet | Répond à |
|---|---|
| **Management** | Ce qui tourne, ce qui est sur l'étagère, et les fichiers qui entrent et sortent |
| **History** | Revenir à n'importe quel moment de la configuration de cette passerelle |
| **Snapshot** | Sauvegarder et restaurer la base entière |

Chaque onglet est une route à lui, donc un marque-page y revient.

![L'écran Configuration sur l'onglet Management : la configuration courante en première ligne, deux enregistrées en dessous](img/console/configuration.webp)

La configuration courante en première ligne, signalée comme identique à
*Known-good baseline*, et deux lignes d'étagère en dessous, dont l'une porte la
pastille *Current*. *Import a file* marche dans les deux éditions ; *Import from
git* porte la pastille Enterprise.

## Management

**La première ligne est ce que cette passerelle sert.** Les lignes en dessous sont des
copies sur une étagère. Enregistrer, dupliquer ou supprimer l'une d'elles ne change
rien à ce qui est servi.

L'icône de la première ligne est une comparaison, et c'est ce qu'il y a de plus utile
sur l'écran :

| Icône | Ce qui tourne |
|---|---|
| verte | est au bit près une configuration enregistrée - son nom est à côté |
| ambre | a dérivé de celle sous laquelle il a été enregistré |
| grise | n'a jamais été enregistré sous aucun nom |

La ligne de l'étagère dont il a été tiré porte le même verdict en pastille :
*Current*, ou *Current, changed*.

**Les actions.** La ligne courante s'enregistre sous un nom et s'exporte. Une ligne
d'étagère peut devenir la courante, être dupliquée, exportée ou supprimée. Cliquer
n'importe quelle ligne ouvre son fichier dans le tiroir, en lecture.

Seul **Set as current** traverse de l'étagère vers la passerelle, donc c'est la seule
action qui demande - et elle demande avec **la liste de ce qu'elle changerait**, plus
un avertissement préalable quand ce qui tourne n'a jamais été enregistré : c'est le
seul geste ici sans rien où revenir.

### Exporter

La boîte de dialogue dit ce que le fichier emporte et ce qu'il n'emporte pas, puis
propose deux formes :

- **Plain YAML** - du texte seulement. Les images restent, et un import garde ce qui
  est en place.
- **Package** - un zip : la configuration se lit et se compare comme du texte, les
  images sont à côté en fichiers.

### Importer

**Import a file** accepte `.yaml`, `.yml`, `.json` ou `.zip`, et demande où cela doit
atterrir :

1. **Save it under a name** - sur l'étagère, sans rien toucher.
2. **Replace the current one** - ce qui n'est pas dans le fichier disparaît.
3. **Add to the current one** - ce qui est dans le fichier est ajouté ou mis à jour, et
   rien n'est retiré.

Les deux qui touchent la passerelle montrent leur plan d'abord, objet par objet. Si le
fichier référence des entrées de coffre que cette passerelle n'a pas, une seconde boîte
les liste pour que vous les remplissiez tout de suite.

> [!NOTE]
> Chaque action marche dans les deux éditions - enregistrer, importer, dupliquer,
> rendre courante, exporter. Ce qui change est la taille de l'étagère : l'édition
> communautaire garde **trois** configurations enregistrées à la fois et dit où
> vous en êtes à côté du bouton Import (*2 of 3 saved*), avant d'atteindre le
> plafond ; Enterprise en garde autant que vous voulez.

### Comparer deux configurations enregistrées

Dès qu'il y en a deux sur l'étagère, une ligne enregistrée propose **Compare with another saved
configuration** : choisissez l'autre, et le dialogue liste ce qui change en passant de la première à
la seconde - ajouté, modifié, retiré, objet par objet, avec les champs qui ont bougé dans une
modification. Ce qui tourne est laissé de côté : c'est la question qu'on pose à la configuration
d'un client et au modèle dont elle est partie, avant de toucher à l'une ou l'autre.
`GET /api/configurations/{id}/compare/{other}`.

## Dépôt git

*Édition Enterprise.* Les configurations peuvent vivre dans un dépôt git : un
historique avec des noms dessus, une relecture avant le changement, et une seule
source de vérité pour plusieurs installations.

Un export est **public par construction** - un champ secret déclaré part sous sa
référence `${nom}` ou pas du tout - et c'est ce qui rend raisonnable de le poser
dans un dépôt. Ses octets sont déterministes, donc deux exports du même état
donnent le même fichier et un diff ne montre que ce qui a bougé.

### Des emplacements, pas des branches

Un **emplacement git** est un dépôt, une branche et un **répertoire** dedans.
Plusieurs emplacements partagent un dépôt, un répertoire par plateforme :

```
platforms/acme/meerkat.yaml
platforms/acme/assets/logo.png
platforms/foo/meerkat.yaml
```

Cette disposition est celle qu'un export produit déjà : aucun fichier à choisir,
rien à convenir.

> [!TIP]
> Préférez un répertoire par plateforme sur une seule branche à une branche par
> plateforme. Une branche sert un changement qui a l'intention de fusionner ; des
> plateformes sont parallèles pour de bon. Une branche par plateforme, c'est
> cueillir chaque correctif dans quatorze branches, pour toujours. Gardez les
> branches pour un changement **proposé** - c'est une pull request - et les tags
> pour « ce que tournait Acme le 2 ».

### La boucle : tirer, lire, activer

**Import from git** lit un emplacement dans une configuration enregistrée et
**n'applique rien**. La passerelle continue de servir ce qu'elle servait ; la
réponse est le plan - ce qu'activer ajouterait, modifierait et retirerait, plus
les entrées de coffre que le document attend et que cette installation n'a pas.
Vous lisez ça, puis **Set as current** quand le plan dit ce que vous attendiez,
et vous relisez le plan au passage.

C'est voulu et ce n'est pas négociable : qui peut écrire sur cette branche ne
doit pas pouvoir reconfigurer une passerelle. Il n'y a aucune réconciliation
continue, et aucune branche n'est surveillée.

**Export to git** committe la **copie enregistrée** - pas l'état courant, donc
enregistrez-le d'abord sous un nom si c'est lui que vous voulez - et le commit est
**attribué à l'opérateur qui a cliqué**. C'est l'intérêt de le faire dans le
produit plutôt que dans un script : un dépôt dont l'historique dit « meerkat » à
chaque changement ne répond à rien six mois plus tard.

Si la branche a bougé depuis la dernière lecture de cette configuration, le push
est **refusé**, avec le remède : tirer dans une copie et comparer. Il n'y a pas de
push forcé.

### Le jeton

HTTPS avec un jeton. Le jeton est une référence de [coffre](/docs/console/vault),
jamais un littéral : un emplacement est une ligne que la console lit et qu'un
snapshot emporte.

Ce qu'il faut accorder, et le **nom d'utilisateur à envoyer à côté du jeton**,
diffèrent d'une forge à l'autre - et toutes rapportent un mauvais nom
d'utilisateur comme le même « authentication failed » qu'un mauvais jeton, ce qui
fait qu'un jeton parfaitement valide coûte un après-midi à quelqu'un. Le
formulaire dit les deux au fur et à mesure que vous tapez l'URL :

| Forge | Nom d'utilisateur | Ce que le jeton demande |
|---|---|---|
| **GitHub** | ignoré | Un jeton **fine-grained**, ce dépôt seulement, *Contents: Read and write*. Pas un jeton classique avec `repo` : c'est le contrôle total de tous les dépôts privés que le compte atteint |
| **GitLab** | `oauth2` | Un project access token avec `write_repository` et le rôle Maintainer (Developer si la branche n'est pas protégée) |
| **Bitbucket Cloud** | `x-token-auth` | Un repository access token avec `repository:write` |
| **Azure DevOps** | ignoré | Un personal access token avec *Code: Read & Write* |
| **Gitea / Forgejo** | le compte du jeton | Un access token avec `write:repository` |

**Check** prouve que le dépôt répond, que l'identifiant est accepté et que la
branche existe, sans rien écrire : un mauvais jeton se découvre sur l'écran qui le
pose.

### Depuis un agent

`list_git_locations`, `pull_configuration` et `push_configuration`
([agents](/docs/agent/overview)). Un agent qui demande à la passerelle de pousser
ne détient aucun identifiant et n'a besoin d'aucun checkout : c'est le jeton du
coffre, limité à un dépôt, qui travaille.

## History

La passerelle tient sa propre bande : **un point de reprise à chaque changement** qui
déplace l'empreinte de la configuration, le plus récent en premier, groupé par jour.
Les lignes se lisent en heure et personne d'abord, puis les mots que le
[journal d'audit](/docs/console/audit-and-issues) a utilisés pour le même changement
à la même seconde.

Ouvrez un point pour lire son fichier. **Restore** montre son plan, comme tout
changement, avant de revenir en arrière, et laisse un point à lui : la bande ne perd
jamais l'état qu'on lui a demandé de quitter. Un point peut aussi être enregistré sur
l'étagère sous un nom, et c'est ainsi que *ce à quoi ça ressemblait mardi* devient une
configuration que l'on garde.

Une configuration enregistrée et un point de reprise sont deux objets différents
volontairement : l'une est intentionnelle et nommée (*le montage Acme*), l'autre
automatique et horodatée (*ce que c'était à 14h32*).

## Snapshot

Une copie cohérente de **toute la base**, prise pendant que la passerelle tourne :
routes et coffre, mais aussi utilisateurs, organisations, sessions et journal d'audit.
C'est ce qu'une sauvegarde restaure ; un export de configuration est ce qu'une seconde
passerelle reproduit.

Meerkat prend l'instantané lui-même, parce que copier une base vivante avec `cp` peut
l'attraper en pleine écriture et que rien ne le dit avant le jour de la restauration.
La planification, la rétention et l'envoi hors de la machine appartiennent à votre
outil de sauvegarde.

**Il n'y a pas de bouton de restauration, volontairement.** Une base ne se remplace pas
sous le processus qui la tient ouverte. L'onglet imprime les commandes exactes à la
place, avec les chemins de cette installation, et rappelle de garder l'ancien fichier
jusqu'à ce que le nouveau ait fait ses preuves.

> [!WARNING]
> Si la clé maîtresse est à côté de la base, les sauvegarder ensemble annule le
> chiffrement au repos - exactement comme ranger un fichier de coffre à côté de sa
> phrase secrète. Gardez la clé dans un gestionnaire de secrets, ou fournissez-la par
> `MEERKAT_VAULT_KEY` ; l'onglet dit lequel des deux cette passerelle fait.

## Pièges

- **Un export n'emporte pas le coffre.** Prenez aussi le
  [fichier de coffre](/docs/console/vault), sinon la seconde passerelle démarre avec
  des références qui ne pointent sur rien.
- **Replace élague, Add non.** Lisez le plan ; c'est la différence entre une fusion et
  un effacement.
- **Une configuration n'est pas une sauvegarde.** Les utilisateurs et les sessions n'y
  sont pas. C'est à cela que sert l'onglet Snapshot.
- **L'image communautaire laisse de côté les parties Enterprise.** Une configuration
  exportée d'une passerelle Enterprise s'importe dans une communautaire sans ses heures
  de travail, sa mise en page, sa marque masquée, son export OpenTelemetry, ses
  annuaires ni ses autorités et commandes ACME (la redirection HTTPS et HSTS
  restent) ; le plan liste ce qui a été laissé de côté.
- **Tirer depuis git ne change rien.** Ça range un document et ça vous montre le
  plan. La passerelle bascule quand quelqu'un active, pas quand le dépôt bouge.
- **Un emplacement appartient à cette installation, pas au document.** Il n'est
  jamais exporté : une configuration qui pourrait repointer la passerelle vers un
  autre dépôt écraserait le répertoire d'un autre client au push suivant.
