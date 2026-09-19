---
title: Sauvegarde et restauration
section: Exploitation
order: 215
summary: Ce qu'un instantané contient, pourquoi il n'y a pas de bouton de restauration, et pourquoi un export de configuration n'est pas une sauvegarde.
---

# Sauvegarde et restauration

Trois choses différentes se confondent ici, donc elles vivent sur trois onglets de l'écran
**Infra > Configuration** et elles répondent à trois questions différentes.

| Quoi | Ce que ça contient | À quoi ça sert |
|---|---|---|
| Un instantané | toute la base | restaurer *cette* installation |
| Un export de configuration | routes, rôles, autorités, thèmes, réglages | reproduire une passerelle *ailleurs* |
| Un fichier de coffre | les valeurs secrètes elles-mêmes, chiffrées par une phrase de passe | amorcer ou déménager un environnement |

![L'écran de configuration](img/console/configuration.webp)

## L'instantané

La seule part de la sauvegarde que seule la passerelle peut faire, c'est une copie **cohérente** prise
pendant qu'elle tourne. Copier un fichier de base vivant avec `cp` peut l'attraper en pleine écriture
ou désynchronisé de son journal, et rien ne le dit : la copie a l'air bonne et échoue le jour où on la
restaure, c'est-à-dire le pire jour possible.

Donc la passerelle écrit la copie elle-même, refuse d'écraser un fichier existant, la termine **avant**
de répondre - un échec est alors une erreur qu'un admin lit, pas un téléchargement tronqué découvert des
mois plus tard - et la nomme avec sa date.

Tout le reste d'une politique de sauvegarde est délibérément absent : la planification, la rétention,
la rotation, le chiffrement des archives, leur envoi hors de la machine, l'alerte quand l'une échoue.
Des outils mûrs font tout cela, et en refaire la moitié dans une app-gateway ne servirait personne.

Un instantané porte les comptes, les sessions, le coffre, le journal d'audit, les points de reprise et
les certificats. Il ne porte **pas** la clé maîtresse du coffre.

> [!WARNING]
> L'instantané à chaud est une copie de la base **embarquée**. Avec un PostgreSQL externe, sauvegardez
> la base avec ses propres outils (`pg_dump`, PITR) : c'est la même forme de décision que le cluster
> lui-même, la base externe apporte sa propre histoire de sauvegarde.

## Il n'y a pas de bouton de restauration, et ce n'est pas un oubli

Une base ne se remplace pas sous le processus qui la tient ouverte. Pire, les sessions et les comptes
qu'une restauration ramène sont précisément ceux dont dépend la requête qui la déclenche - et accepter
une base arbitraire comme état de confiance transformerait une session d'admin emprutée en contrôle
permanent de la passerelle.

La restauration se fait donc service arrêté, et la console imprime les commandes exactes avec les
chemins de **cette** installation, pour que la procédure soit un collage et non une énigme :

```bash
# 1. stop meerkat
# 2. keep the current database aside
mv /data/meerkat.db /data/meerkat.db.before-restore
# 3. put the snapshot in its place
cp meerkat-YYYY-MM-DD.db /data/meerkat.db
# 4. start meerkat, then check a route and a sign-in
```

L'ancien fichier est gardé : une restauration qu'on regrette doit avoir un chemin de retour.

> [!WARNING]
> La clé maîtresse est posée à côté de la base, sauf si elle vient de `MEERKAT_VAULT_KEY`. Les
> sauvegarder ensemble annule le chiffrement au repos, exactement comme ranger un fichier de coffre à
> côté de sa phrase de passe. Gardez la clé dans un gestionnaire de secrets, ou fournissez-la par
> l'environnement - la console dit laquelle des deux fait votre installation.

## L'export de configuration

Un document, en YAML : les routes, le catalogue de rôles, les organisations et leurs groupes, les
autorités par lesquelles les gens se connectent, le relais mail, les thèmes et les réglages de la
passerelle (CONSOLE-05, CFG-05).

Ce qu'il ne porte **pas** fait autant partie de la conception que ce qu'il porte :

- **aucun compte**, aucune appartenance, aucune session : ils portent des identifiants, des secrets de
  second facteur et des passkeys ;
- **aucun certificat**, aucune clé de signature, aucune clé de simulation : elles sont générées là où
  elles servent ;
- **aucune valeur secrète.** Un champ secret déclaré voyage sous sa référence `$nom` ou pas du tout.

Un export est donc **public par construction**. Il part dans un ticket, un mail au support ou un dépôt
git sans que personne ait à se demander ce qu'il y a dedans - et le jour où un export peut porter un
secret, plus personne n'ose en partager un et la fonctionnalité meurt de sa propre prudence.

La contrepartie est assumée : un document seul ne démarre pas un environnement. Ses références doivent
exister dans [le coffre](/#/docs/operations/vault), qui est un fichier à part, avec une vie à part.

Deux formes diffèrent par une seule chose : un YAML nu ne porte jamais d'image et dit ce qu'il a laissé
derrière lui ; un paquet `.zip` porte les logos et les specs OpenAPI déposées, pour quand les images
doivent voyager. Un import qui n'en porte aucune laisse celles en place tranquilles.

## Les points de reprise : la bande

La passerelle tient sa propre bande (CFG-06). Un point est écrit quand un changement déplace
l'**empreinte** de la configuration - et non quand un endpoint est appelé, ce qui est ce qui rend la
bande à la fois abordable et complète :

- un endpoint ajouté plus tard est couvert sans que personne y pense ;
- un enregistrement qui ne change rien n'écrit rien ;
- ce qui n'est pas de la configuration - créer un compte, remplir le coffre, ouvrir une session - n'y
  laisse aucune trace du tout.

Un point a une heure, un auteur et la phrase que le journal d'audit a écrite à la même seconde.
Personne ne demande un point et personne ne le nomme : c'est toute la différence avec une configuration
enregistrée.

Une série de changements par la **même** personne en moins de deux minutes se replie en un seul point :
faire glisser un sélecteur de couleur écrit vingt fois pour une seule décision. Atterrir sur un état
que la bande connaît déjà n'est jamais replié, parce que c'est exactement ce que revenir en arrière
*est*.

La bande n'est **pas** bornée, et c'est la même dans les deux éditions. Revenir en arrière est une
fonction de sûreté : raccourcir la mémoire de l'image gratuite ne vendrait pas l'image payante, ça
rendrait seulement le produit plus risqué là où il sert le plus.

## Les configurations nommées

Plusieurs configurations coexistent et exactement une est active (CFG-01, CFG-02). La console les
compare - objets ajoutés, supprimés, modifiés - et montre ce même diff quand un fichier est importé,
avant que rien ne soit écrit. Un agent peut ranger l'état courant sous un nom en un appel, ce qui est
ce qu'un admin prudent demande en mots avant un gros changement.
