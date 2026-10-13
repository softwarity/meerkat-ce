---
title: Sauvegarde et restauration
section: Exploitation
order: 215
summary: Ce que contient un snapshot, pourquoi il n'y a pas de bouton de restauration, et pourquoi un export de configuration n'est pas une sauvegarde.
---

# Sauvegarde et restauration

On confond ici trois choses différentes. Elles occupent donc trois onglets de l'écran
**Meerkat > Configuration**, et répondent à trois questions différentes.

| Quoi | Ce qu'il contient | À quoi il sert |
|---|---|---|
| Un snapshot | la base de données entière | restaurer *cette* installation |
| Un export de configuration | les routes, les rôles, les autorités, les thèmes, les réglages | reproduire une gateway *ailleurs* |
| Un fichier de coffre | les valeurs secrètes elles-mêmes, chiffrées par une phrase de passe | amorcer ou déplacer un environnement |

![L'écran de configuration](img/console/configuration.webp)

## Le snapshot

Dans une sauvegarde, il n'y a qu'une chose que seule la gateway sait faire : une copie
**cohérente**, prise pendant qu'elle tourne. Copier avec `cp` le fichier d'une base en
service, c'est risquer de le saisir en pleine écriture, ou en décalage avec son journal de
transactions, et rien ne le signale : la copie semble bonne, et elle échoue le jour où on la
restaure, c'est-à-dire le pire jour possible.

La gateway écrit donc la copie elle-même. Elle refuse d'écraser un fichier existant,
termine la copie **avant** de répondre - un échec est alors une erreur que l'administrateur
lit, et non un téléchargement tronqué découvert des mois plus tard - et lui donne un nom qui
porte sa date.

Tout le reste d'une politique de sauvegarde est volontairement absent : la planification, la
durée de conservation, la rotation, le chiffrement des archives, leur envoi hors de la
machine, l'alerte en cas d'échec. Des outils éprouvés font tout cela, et en refaire la moitié
dans une app-gateway ne rendrait service à personne.

Un snapshot contient les comptes, les sessions, le coffre, le journal d'audit, les points
de reprise et les certificats. Il ne contient **pas** la clé maîtresse du coffre.

> [!WARNING]
> Le snapshot à chaud est une copie de la base **embarquée**. Avec un PostgreSQL externe,
> sauvegardez la base avec ses propres outils (`pg_dump`, PITR). La logique est la même que
> pour le cluster : une base externe vient avec sa propre stratégie de sauvegarde.

## Il n'y a pas de bouton de restauration, et ce n'est pas un oubli

On ne remplace pas une base de données sous le processus qui la tient ouverte. Pire, les
sessions et les comptes que ramène une restauration sont justement ceux dont dépend la
requête qui la lance - et accepter une base quelconque comme état de confiance ferait d'une
session d'administrateur empruntée une prise de contrôle définitive de la gateway.

La restauration se fait donc service arrêté. La console affiche les commandes exactes, avec
les chemins de **cette** installation et pour la plateforme sur laquelle elle tourne : la
procédure se résume à un copier-coller, pas à un casse-tête. Sur une machine :

```bash
# 1. stop meerkat
# 2. keep the current database aside
mv /data/meerkat.db /data/meerkat.db.before-restore
# 3. put the snapshot in its place
cp meerkat-YYYY-MM-DD.db /data/meerkat.db
# 4. start meerkat, then check a route and a sign-in
```

L'ancien fichier est conservé : il faut pouvoir revenir sur une restauration que l'on
regrette. Sur Kubernetes, les mêmes étapes ramènent le Deployment à zéro et passent par un
pod auxiliaire monté sur le volume ; sur Swarm, compose ou un conteneur, par un conteneur
auxiliaire sur le volume. Dans tous les cas, le fichier reçoit le propriétaire du
répertoire : un fichier copié en root est un fichier que la gateway ne peut pas écrire.

> [!WARNING]
> La clé maîtresse se trouve à côté de la base de données, sauf si elle vient de
> `MEERKAT_VAULT_KEY`. Sauvegarder les deux ensemble annule le chiffrement au repos,
> exactement comme si vous rangiez un fichier de coffre à côté de sa phrase de passe.
> Conservez la clé dans un gestionnaire de secrets, ou fournissez-la par l'environnement -
> la console indique lequel des deux cas est celui de votre installation.

## Déplacer la base : embarquée et PostgreSQL

Le même onglet déplace la base. Une copie du **même type** que celle sur laquelle
tourne la gateway est une sauvegarde ; du **type opposé**, elle déplace la gateway :

| De | Vers | Ce que vous obtenez |
|---|---|---|
| embarquée | embarquée | `meerkat.db`, le snapshot ci-dessus |
| embarquée | PostgreSQL | `meerkat.sql`, un dump au format texte de `pg_dump`, ou une copie directe vers un serveur |
| PostgreSQL | PostgreSQL | `meerkat.sql`, ou une copie vers un autre serveur |
| PostgreSQL | embarquée | `meerkat.db`, construit depuis la base |

PostgreSQL relève de l'image Enterprise : la cible est verrouillée sur l'image Community.

**Mettez d'abord en pause** - l'écran l'exige : une copie vers l'autre type, fichier
ou serveur, attend la pause, et l'API aussi. Une sauvegarde du même type ne l'attend
pas : elle est faite pour être prise à chaud. L'interrupteur **Pause**, en haut de l'onglet, fait
répondre la page de maintenance à toutes les applications et arrête toute écriture,
sur tous les nœuds. Une copie prise pendant que la gateway écrit perdrait ce qui
s'écrit avant la bascule : sessions, journal d'audit, appels audités. La pause n'est
jamais enregistrée : un redémarrage y met fin, et la gateway qui démarre sur la
nouvelle base fonctionne immédiatement. Pendant la pause, la console reste ouverte
en lecture et refuse les modifications.

**Une seule procédure, dès le départ.** Un déplacement est une liste numérotée que
l'onglet affiche avant toute copie, chaque étape cochée quand elle est faite : créer la
base et son utilisateur, tester le serveur, mettre en pause, copier, basculer le
déploiement, vérifier une route et une connexion, et le chemin du retour. La gateway se
souvient de la copie - hôte, port, base, utilisateur, mode SSL, date, nombre de tables
et de lignes, jamais le mot de passe - dans la base qu'elle quitte : un rechargement au
milieu d'une migration, sur n'importe quel nœud, ramène à la même procédure, avec un mot
de passe d'exemple à saisir de nouveau. Écartez-la (*Dismiss*) si vous avez pris un autre chemin ;
elle s'efface d'elle-même quand la gateway tourne sur cette base.

**Créer la base d'abord.** La première étape est le SQL que lance un administrateur de
bases de données, rempli avec la base, l'utilisateur et le mot de passe saisis dans le
formulaire - le formulaire part de valeurs d'exemple, et le SQL comme toutes les
commandes qui suivent reprennent ce qui est saisi :

```sql
-- as an administrator: psql -h canopydb-primary -p 5432 -U postgres
CREATE ROLE meerkat LOGIN PASSWORD 'change-me-now';
CREATE DATABASE meerkat OWNER meerkat;

-- or, into a database that already exists and belongs to another role:
-- \c meerkat
-- GRANT CONNECT, CREATE ON DATABASE meerkat TO meerkat;
-- GRANT USAGE, CREATE ON SCHEMA public TO meerkat;
```

Ce sont exactement les droits que vérifie **Test** : `CREATE` sur la base et sur le
schéma qui recevra les tables. Le propriétaire d'une base les a ; depuis PostgreSQL 15,
le schéma `public` appartient à `pg_database_owner`, c'est-à-dire au propriétaire de la
base. Les verrous consultatifs et `LISTEN/NOTIFY` ne demandent aucun droit.

**Vers un serveur**, l'onglet prend la cible champ par champ - hôte, port, base,
utilisateur, mot de passe, mode SSL - pour une base vide. Dans Kubernetes, il propose
les services PostgreSQL qu'il trouve à côté de la gateway, le primaire de l'opérateur
en tête (`-primary` chez CrunchyData, `-rw` chez CloudNativePG) ; un pooler comme
pgbouncer et un réplica sont listés mais refusés, car le cluster a besoin de
`LISTEN/NOTIFY` et des verrous consultatifs, qu'un pooler en mode transaction casse et
qu'un réplica ne peut pas prendre. **Test** interroge le serveur avant que rien ne
bouge, sans rien y écrire - avant la pause ou pendant : sa version, le mode SSL qui se connecte (chiffré essayé
d'abord, puis retenu pour la copie), si la base est vide, et si l'utilisateur peut y
créer des tables - ce que PostgreSQL 15 et suivants n'accordent plus par défaut sur
`public`. Une cible qui échoue à l'un de ces points ne se voit pas proposer la copie.
Il copie toutes les tables dans une seule transaction, en comptant chacune des deux côtés
avant de valider. Une cible qui contient déjà des comptes ou des routes est refusée :
une copie n'écrase jamais une gateway. Le mot de passe sert à la copie et n'est conservé
nulle part ; le journal en garde l'hôte, dans la nouvelle base, qui commence son
histoire en disant d'où elle vient.

**Vers un fichier**, le dump se charge avec `psql -v ON_ERROR_STOP=1 -f meerkat.sql`
dans une base vide. C'est une seule transaction : un dump tronqué est refusé en entier.

**La bascule appartient au déploiement.** La gateway choisit sa base au démarrage, et
les fichiers de déploiement restent la seule vérité à son sujet : l'onglet ne garde
aucun pointeur à lui. Il affiche la modification pour la plateforme que décrit le
runtime, avec ses noms - *Written for* en choisit une autre :

| Plateforme | La bascule |
|---|---|
| Kubernetes, OpenShift, avec Helm | un Secret avec l'adresse de la base (et la clé du coffre quand c'est un fichier), puis `helm upgrade <release> --reuse-values --set database.existingSecret=...` ; `replicaCount` est facultatif |
| Kubernetes sans Helm | le même Secret, et les entrées `env` à ajouter au Deployment |
| Swarm | `docker service update --env-add MEERKAT_DATABASE_URL=... <service>`, et la ligne du fichier de stack pour que le prochain `docker stack deploy` la garde |
| Compose | les lignes `environment:` du fichier compose, leurs valeurs dans `.env`, puis `docker compose up -d` |
| Un conteneur | le même `docker run` avec `-e MEERKAT_DATABASE_URL=...` |

**Le chemin du retour** est cette bascule défaite, commande par commande, sur la même
plateforme, tant que la base quittée est encore là : avec Helm, la release ramenée avec
`database.existingSecret=""` (et `vault.existingSecret=""` quand la clé a suivi) et un
seul pod, puis `kubectl delete secret` du Secret créé par la bascule ; sur Swarm,
`docker service update --env-rm` de chaque variable ajoutée, et les mêmes lignes
retirées du fichier de stack ; sur Compose, les lignes retirées du fichier et du `.env`,
puis `docker compose up -d` ; pour un conteneur, le `docker run` d'avant.

Sur une release Helm, avec des noms d'exemple :

```bash
# the new database, as a Secret (the vault key is already in the deployment, unchanged)
kubectl -n shop create secret generic meerkat-database \
  --from-literal=database-url='postgres://meerkat:PASSWORD@canopydb-primary:5432/meerkat?sslmode=require'

# point the release at it: the pods restart on PostgreSQL, unpaused
helm upgrade edge meerkat/meerkat -n shop --reuse-values \
  --set database.existingSecret=meerkat-database

# optional - PostgreSQL lets several pods serve as ONE gateway; one is fine too
# helm upgrade edge meerkat/meerkat -n shop --reuse-values --set replicaCount=2
```

> [!WARNING]
> La clé du coffre ne voyage jamais avec la copie. Sans la même clé, tous les secrets
> du coffre sont illisibles sur la nouvelle base : reportez-la dans le Secret, comme
> l'indique la procédure.

Les mêmes opérations, pour un script ou un Job Kubernetes lancé avant une mise à jour :

```bash
meerkat db dump -format postgres -out meerkat.sql    # un dump, pour psql
meerkat db dump -format sqlite -out meerkat.db       # un fichier de base
meerkat db copy -to postgres://meerkat:PASSWORD@host:5432/meerkat
```

Elles lisent la source dans `-data` et `-database-url`, comme la gateway. Arrêtez la
gateway, ou mettez-la en pause depuis la console, avant de les lancer.

## L'export de configuration

Un seul document, en YAML : les routes, le catalogue des rôles, les organisations et leurs
groupes, les autorités auprès desquelles on se connecte, le relais de messagerie, les thèmes
et les réglages de la gateway.

Ce qu'il ne contient **pas** relève de la conception autant que ce qu'il contient :

- **aucun compte**, aucune appartenance, aucune session - ils portent des identifiants, des
  secrets de second facteur et des passkeys ;
- **aucun certificat**, aucune clé de signature, aucune clé de simulation - ils sont générés
  là où ils servent ;
- **aucune valeur secrète.** Un champ déclaré secret voyage sous la forme de sa référence
  `$name`, ou ne voyage pas.

Un export est donc **public par construction**. Il se glisse dans un ticket, dans un e-mail
au support ou dans un dépôt git sans que personne ait à se demander ce qu'il renferme. Le
jour où un export risque de contenir un secret, plus personne n'ose en partager un, et la
fonctionnalité meurt de sa propre prudence.

La contrepartie est assumée : un document ne suffit pas à démarrer un environnement. Ses
références doivent exister dans [le coffre](/docs/operations/vault), qui est un fichier à
part, avec sa propre vie.

Il existe deux formats, qu'une seule chose distingue : un YAML simple ne contient jamais
d'image, et il dit ce qu'il a laissé de côté ; un paquet `.zip` contient les logos et les
spécifications OpenAPI déposées, pour les cas où les images doivent suivre. Un import sans
image ne touche pas à celles qui sont en place.

## Les points de reprise : l'historique

La gateway tient son propre historique. Un point y est écrit chaque fois qu'un
changement modifie l'**empreinte** de la configuration - et non chaque fois qu'un endpoint
est appelé. C'est ce qui rend l'historique à la fois peu coûteux et complet :

- un endpoint ajouté plus tard est couvert sans que personne ait à y penser ;
- un enregistrement qui ne change rien n'écrit rien ;
- ce qui n'est pas de la configuration - créer un compte, remplir le coffre, ouvrir une
  session - n'y laisse aucune trace.

Un point a une heure, un auteur, et la phrase que le journal d'audit a écrite à la même
seconde. Personne ne demande un point et personne ne lui donne de nom : c'est toute la
différence avec une configuration enregistrée.

Une série de changements faits par la **même** personne en moins de deux minutes est
regroupée en un seul point : faire glisser un sélecteur de couleur écrit vingt fois pour une
seule décision. En revanche, le retour à un état que l'historique connaît déjà n'est jamais
regroupé, car c'est précisément cela, revenir en arrière.

La bande n'est **pas** limitée, et elle est identique dans les deux éditions. Revenir en
arrière est une fonction de sécurité : raccourcir la mémoire de l'image gratuite ne ferait
pas vendre l'image payante, cela rendrait seulement le produit plus risqué là où il est le
plus utilisé.

## Les configurations nommées

Plusieurs configurations coexistent, et une seule est active. La console les compare -
objets ajoutés, supprimés, modifiés - et affiche ce même diff à l'import d'un fichier, avant
toute écriture. Un agent peut ranger l'état courant sous un nom en un seul appel : c'est ce
qu'un administrateur prudent demande, en toutes lettres, avant un changement important.
