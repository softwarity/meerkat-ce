---
title: TLS et certificats
section: Exploitation
order: 224
summary: Les quatre portes par lesquelles un certificat entre, ce qui vit en base, et ce qu'un cluster doit partager.
---

# TLS et certificats

Un certificat appartient à un **nom** (SSL-08). La console a un nom et un certificat ; l'application en
a un par hôte qu'elle sert. Un matériel utilisé par les deux est ajouté deux fois - deux entrées et deux
clés sont moins chères à comprendre qu'un objet partagé plus la règle qui devine quel nom il couvre.

Il n'y a pas de « activer HTTPS » : **avoir un certificat est ce qui ouvre la porte**, et le supprimer
est ce qui la ferme. Un interrupteur qui peut être allumé sans rien derrière est un interrupteur qui
ment.

![L'écran TLS](img/console/tls.webp)

## Les quatre portes

Aucune n'est optionnelle, parce que les lieux où Meerkat tourne ne se ressemblent pas (SSL-01) :

| Porte | Quand c'est la bonne |
|---|---|
| **Import** | vous détenez déjà le certificat et sa clé |
| **Auto-signé** | un labo, un nom interne, un premier déploiement |
| **Demande de signature** | la passerelle fabrique la clé et la CSR, votre autorité signe, vous adoptez la réponse |
| **ACME** | une autorité émet et renouvelle toute seule |

Une demande de signature est une ligne qui tient une clé et ne sert rien : elle existe pour que la
demande puisse être retéléchargée pendant que vous attendez la réponse.

## Les ports

Une porte HTTPS par plan, à côté de celle en clair, ouverte et fermée pendant que la passerelle tourne
(SSL-02) :

| Plan | En clair | HTTPS |
|---|---|---|
| Données | `MEERKAT_ADDR`, `:8080` | `MEERKAT_TLS_ADDR`, `:8443` |
| Contrôle | `MEERKAT_ADMIN_ADDR`, `:9090` | `MEERKAT_ADMIN_TLS_ADDR`, `:9443` |

Un port dont on ne peut pas nommer le protocole est un port contre lequel personne ne peut écrire une
règle de pare-feu ni une procédure, ce qui est la raison pour laquelle il y en a quatre et non deux.

Remplacer un certificat, c'est échanger une tranche derrière un verrou : la poignée de main le lit par un
rappel, donc l'écoute ne bouge pas et aucune connexion n'est coupée. Ouvrir une porte se lie **d'abord**
et renvoie l'échec avant que rien n'ait changé - un port déjà pris ne doit pas laisser un exploitant
sans HTTPS du tout.

## Quel certificat répond à une poignée de main

Celui du nom demandé par le client. Une poignée de main qui ne nomme aucun hôte - un client qui joint la
passerelle par son adresse, ou n'importe quoi de plus vieux que l'extension SNI - reçoit le certificat
**de repli**, et sans repli elle reçoit une erreur qui nomme ce qui *est* servi, ce qui est plus utile
qu'un certificat au hasard qu'elle rejettera de toute façon.

Allumer TLS sans rien à présenter est refusé : ça transformerait un port qui marche en un port qui refuse
tous les visiteurs.

## Le port en clair peut rediriger

Sur le **plan de données uniquement**, le port en clair peut rediriger vers celui en HTTPS (SSL-06). Les
deux sondes de santé sont exemptées : un `308` se lit comme « pas prêt ».

`HSTS` existe comme en-tête de réponse par route (SEC-03) ; il n'y a pas encore de réglage global.

## ACME n'est pas Let's Encrypt

Le répertoire est une **URL**, et c'est tout l'intérêt : la moitié des installations auxquelles Meerkat
est destiné n'ont aucune route vers internet. Un `step-ca` interne, un EJBCA, une autorité Windows avec le
rôle ACME : n'importe laquelle répond ici.

| Champ | À quoi il sert |
|---|---|
| URL du répertoire | l'autorité. Vide veut dire Let's Encrypt ; le bac à sable est offert comme point de départ |
| Adresse de contact | là où l'autorité avertit des expirations. Exigée par certaines autorités privées |
| Hôtes | la liste **fermée** des noms qui peuvent être demandés |
| CA racine | l'autorité qui signe le certificat HTTPS **du serveur ACME lui-même**, en PEM - un serveur ACME privé est en général derrière un certificat privé |
| Identifiant et clé HMAC EAB | External Account Binding : sous quel compte cette passerelle s'enregistre. Les autorités publiques l'ignorent, la plupart des privées refusent sans |
| Conditions acceptées | un acte juridique, donc jamais supposé |

La liste d'hôtes n'est jamais vide : une politique ouverte laisserait n'importe qui joindre la passerelle
par son adresse avec un SNI inventé et brûler le quota de l'autorité sur des noms que personne ne possède.

## Ce qui vit en base

| Stocké | Comment |
|---|---|
| Le certificat lui-même | en clair - c'est ce que la passerelle tend à chaque visiteur |
| La clé privée | **scellée** avec la clé maîtresse du coffre. Elle ne quitte jamais la passerelle : ni dans une charge utile, ni dans un export, ni dans un journal |
| La demande de signature en attente | en clair, et téléchargeable |
| La clé de compte ACME et le matériel émis | scellés, dans le cache ACME |

La console vérifie aussi en continu que le matériel répond bien pour l'hôte sous lequel il a été rangé. Un
certificat qui ne couvre rien échoue au moment de la poignée de main, là où personne ne fait le lien avec
cet écran.

## En cluster

> [!WARNING]
> Les clés de certificat et la clé de compte ACME sont scellées avec la clé maîtresse du coffre, qui est
> un **fichier local** sauf si `MEERKAT_VAULT_KEY` est posée. Un noeud portant une autre clé ne peut pas
> les ouvrir, et il le dit : *the private key cannot be unsealed - is this the vault key it was written
> with?* Donnez la même clé à tous les noeuds.

L'émission est sérialisée par un verrou consultatif. Cinq noeuds redémarrant ensemble dépensaient le quota
hebdomadaire de doublons de l'autorité en une seconde ; le noeud qui attend trouve maintenant le
certificat que le gagnant vient d'écrire. Le verrou n'est pris qu'à la première poignée de main pour un nom
et à l'approche du renouvellement - le mettre devant chaque poignée de main coûterait un aller-retour vers
la base pour éviter un événement qui arrive deux fois par an.

Un certificat écrit sur un noeud est rechargé par les autres via le bus de changement.

## Ce qui manque

- **L'alerte d'expiration** (SSL-04). La console montre le compte à rebours ; rien ne vous écrit.
- **Un réglage HSTS global** (SSL-06) : l'en-tête n'existe que par route.
- **HTTP/3** (SSL-07) : il faudrait une dépendance QUIC hors bibliothèque standard, une écoute UDP et
  `Alt-Svc`.
