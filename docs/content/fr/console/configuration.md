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
pastille *Current*. *Import a file* porte la pastille Enterprise.

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
> Edition Enterprise : garder plusieurs configurations et basculer entre elles
> (enregistrer, importer, dupliquer, rendre courante). L'export est dans les deux
> éditions.

## History

La passerelle tient sa propre bande : **un point de reprise à chaque changement** qui
déplace l'empreinte de la configuration, le plus récent en premier, groupé par jour.
Les lignes se lisent en heure et personne d'abord, puis les mots que le
[journal d'audit](/#/docs/console/audit-and-issues) a utilisés pour le même changement
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
  [fichier de coffre](/#/docs/console/vault), sinon la seconde passerelle démarre avec
  des références qui ne pointent sur rien.
- **Replace élague, Add non.** Lisez le plan ; c'est la différence entre une fusion et
  un effacement.
- **Une configuration n'est pas une sauvegarde.** Les utilisateurs et les sessions n'y
  sont pas. C'est à cela que sert l'onglet Snapshot.
