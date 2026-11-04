---
title: Vérification des mises à jour
section: Exploitation
order: 216
summary: Coupée par défaut. Ce que la gateway lit, ce qu'elle envoie, et ce que la console dit avant que vous installiez une version plus récente.
---

# Vérification des mises à jour

Une gateway ne contacte personne d'elle-même. La vérification des mises à jour est
la seule exception, et elle reste coupée tant que root ne l'active pas : une
installation sans accès sortant n'a rien à faire.

## L'activer

Dans **Meerkat > Release notes**, root active **Check for updates**. La gateway lit
la liste tout de suite, puis une fois par jour, sur une seule instance. Un
redémarrage ne la relit pas. La couper oublie ce qu'elle avait trouvé.

Le changement est inscrit au journal d'audit, et il reste sur cette gateway : il ne
voyage pas dans un export de configuration.

## Ce qui sort de la gateway

Un seul `GET` sur un fichier public, `https://meerkat.softwarity.dev/releases.json`.
La requête ne porte ni version, ni identifiant, ni rien sur votre installation. Elle
suit `HTTP_PROXY` et `HTTPS_PROXY` quand ils sont définis.

Le fichier est servi par GitHub Pages, qui voit l'adresse d'où vient la requête,
comme pour tout visiteur du site. Softwarity ne reçoit rien.

L'adresse ne se change ni depuis la console ni par l'API. Pour garder la vérification
dans votre réseau, publiez une copie du fichier et donnez son adresse à la gateway
avec `MEERKAT_UPDATE_FEED`.

## Ce que montre la console

Quand une version plus récente existe, root et les administrateurs de la gateway
voient un voyant sur l'entrée **Meerkat** du rail et sur **Release notes**, dans
toutes les consoles ouvertes, sans recharger. Les autres comptes ne voient rien : ils
ne peuvent rien faire d'une mise à jour.

**Release notes** s'ouvre alors sur les versions plus récentes, marquées **Not
installed**, chacune avec ce que son installation provoquerait :

| La console dit | Ce que cela veut dire |
|---|---|
| Built after the updates of your license ended | Une fois installée, cette version tourne en évaluation : la mention apparaît sur les pages que voient vos utilisateurs et les plafonds de la licence ne s'appliquent plus. Le trafic n'est pas touché, et renouveler la licence la garde en vigueur. |
| This version migrates the database | Une version plus ancienne n'ouvre plus la base ensuite. Faites une [sauvegarde](/docs/operations/backup-restore) avant la mise à jour. |

Quand une version dit les deux, elle ne migre pas d'elle-même : voir plus bas.

Rien ne s'installe depuis cet écran : la console n'a pas de bouton d'installation.

## Une version hors licence ne migre pas d'elle-même

Faire tourner une version construite après la fin des mises à jour de votre
licence se défait : revenez à la précédente, et la licence est de nouveau en
vigueur. Migrer la base ne se défait pas : la version précédente ne l'ouvre plus,
et le retour passe par une sauvegarde, s'il y en a une.

Une version hors licence qui devrait migrer la base s'arrête donc avant de le
faire, n'écrit rien, et dit pourquoi dans son journal :

```
this version was built on 2028-01-10, after the updates of the license of ACME (...) ended on 2027-12-31,
and it has to migrate the database (schema v102 to v105). [...] Run a version built by 2027-12-31,
or renew the license and start this one again. To migrate anyway and run as an evaluation,
take a backup first, then set MEERKAT_MIGRATE_PAST_LICENSE=true
```

La version d'avant redémarre sur la même base, telle qu'elle était. Sur
Kubernetes avec plusieurs instances, et sur Swarm avec la stack que nous
publions, les instances déjà lancées continuent de servir pendant que la
nouvelle refuse. Sur Kubernetes avec une seule instance, sur Swarm sans
`order: start-first` et sur Compose, la gateway est arrêtée jusqu'à ce que vous
remettiez l'image précédente.

Ce qui y met fin :

- **Renouveler la licence.** La nouvelle couvre la version, qui migre à son
  démarrage suivant. Une licence montée depuis un Secret est lue au démarrage ;
  une licence collée se donne depuis la console de la version encore en place.
- **Passer outre.** Faites une [sauvegarde](/docs/operations/backup-restore),
  puis démarrez avec `MEERKAT_MIGRATE_PAST_LICENSE=true`. La base est migrée, la
  version tourne en évaluation, et la licence reste telle quelle : la renouveler
  plus tard la remet en vigueur. Le journal d'audit le garde (`license.migrate`).

Le contrôle n'a lieu que s'il y a quelque chose à perdre : une licence en
vigueur et une migration en attente. Sans licence, avec une licence d'essai
terminée, sur une base neuve ou avec une version qui ne change rien à la base,
le démarrage se fait comme toujours.

## Quand la vérification échoue

Une gateway qui ne joint pas la liste le dit sur **Release notes**, avec l'heure et la
raison, et écrit une ligne d'information dans son journal. Il n'y a pas d'alerte, et
elle réessaie le lendemain. Ce qu'une vérification précédente avait trouvé reste affiché.
