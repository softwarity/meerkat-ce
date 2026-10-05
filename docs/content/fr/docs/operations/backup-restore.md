---
title: Sauvegarde et restauration
section: Exploitation
order: 215
summary: Ce que contient un snapshot, pourquoi il n'y a pas de bouton de restauration, et pourquoi un export de configuration n'est pas une sauvegarde.
---

# Sauvegarde et restauration

On confond ici trois choses différentes. Elles occupent donc trois onglets de l'écran
**Infra > Configuration**, et répondent à trois questions différentes.

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
les chemins de **cette** installation : la procédure se résume à un copier-coller, pas à un
casse-tête.

```bash
# 1. stop meerkat
# 2. keep the current database aside
mv /data/meerkat.db /data/meerkat.db.before-restore
# 3. put the snapshot in its place
cp meerkat-YYYY-MM-DD.db /data/meerkat.db
# 4. start meerkat, then check a route and a sign-in
```

L'ancien fichier est conservé : il faut pouvoir revenir sur une restauration que l'on
regrette.

> [!WARNING]
> La clé maîtresse se trouve à côté de la base de données, sauf si elle vient de
> `MEERKAT_VAULT_KEY`. Sauvegarder les deux ensemble annule le chiffrement au repos,
> exactement comme si vous rangiez un fichier de coffre à côté de sa phrase de passe.
> Conservez la clé dans un gestionnaire de secrets, ou fournissez-la par l'environnement -
> la console indique lequel des deux cas est celui de votre installation.

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
