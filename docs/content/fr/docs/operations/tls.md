---
title: TLS et certificats
section: Exploitation
order: 224
summary: Les quatre portes par lesquelles un certificat entre, ce qui vit en base, et ce qu'un cluster doit partager.
---

# TLS et certificats

Un certificat entre une fois dans une **réserve**, puis est **placé** sur la console,
l'application, ou les deux (SSL-08). Ses noms sont les siens - ce que le matériel dit couvrir -
donc un certificat qui porte plusieurs noms, un joker ou une adresse IP les sert tous, et un
matériel utilisé par les deux plans est une seule entrée, pas deux copies à tenir alignées.

Il n'y a pas de « activer HTTPS » : **un certificat placé sur un plan est ce qui ouvre sa porte**, et
le retirer est ce qui la ferme. Un interrupteur qui peut être allumé sans rien derrière est un
interrupteur qui ment.

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

**Ce sont les ports à l'intérieur.** Entre eux et un navigateur il y a en général une correspondance :
un Service Kubernetes qui publie `9443` en `19443`, un `-p` Docker, un ingress Swarm. La passerelle le
demande à son environnement - son propre pod et les Services qui le sélectionnent, ou son conteneur par
le socket Docker - et les liens de l'écran TLS portent le port que le monde atteint. Quand
l'environnement ne publie pas une porte HTTPS, l'écran le dit plutôt que d'offrir un lien qui n'atteint
rien. Le chart Helm publie les deux portes HTTPS par défaut (`service.appTlsPort`,
`service.adminTlsPort`) et accorde la lecture nécessaire (`rbac.read`).

> [!NOTE] Docker Desktop
> Son Kubernetes publie les ports d'un LoadBalancer sur `localhost` à la création du Service, et ignore
> ceux ajoutés ensuite. Après avoir monté une release vers un chart qui ajoute les ports HTTPS,
> supprimez les deux Services et relancez `helm upgrade`.

Remplacer un certificat, c'est échanger une tranche derrière un verrou : la poignée de main le lit par un
rappel, donc l'écoute ne bouge pas et aucune connexion n'est coupée. Ouvrir une porte se lie **d'abord**
et renvoie l'échec avant que rien n'ait changé - un port déjà pris ne doit pas laisser un exploitant
sans HTTPS du tout.

## Sur une machine locale

Un portable ou un labo n'a ni nom public ni autorité publique, et un certificat auto-signé fait
protester tous les navigateurs. Le bouton **On a local machine** de l'écran TLS ouvre les étapes,
par système (macOS, Linux, Windows), avec les noms que vous saisissez - à commencer par celui par
lequel la console est atteinte - déjà dans les commandes :

1. **Le fichier hosts** fait pointer les noms vers la passerelle - `127.0.0.1` quand elle tourne
   sur la même machine (Docker, un Kubernetes local), son adresse sinon - une VM, minikube, une
   autre machine ; la console propose l'adresse par laquelle elle a été atteinte quand c'en est
   une. `localhost` n'a besoin d'aucune ligne.
2. **Une autorité locale**, fabriquée par [mkcert](https://github.com/FiloSottile/mkcert) et
   approuvée par cette machine : le magasin du système, Chrome, Edge, Safari, et Firefox par
   `nss`. Relancez ensuite les navigateurs.
3. **Un certificat pour tous les noms**, signé par cette autorité : `meerkat.pem` et son
   `meerkat-key.pem`.
4. **L'import** - Import a PEM pair, les deux fichiers déposés sur la fenêtre - puis on le glisse
   sur la console et sur l'application.

![Le tiroir des étapes pour HTTPS sur une machine locale : les noms, le fichier hosts, une autorité locale, un certificat, l'import](img/console/tls-local.webp)

> [!WARNING]
> La clé privée de l'autorité reste dans le dossier qu'affiche `mkcert -CAROOT`. Qui la détient peut
> signer pour n'importe quel nom que cette machine croit : ne la partagez jamais - une autre machine
> fabrique la sienne.

## Quel certificat répond à une poignée de main

Celui du nom demandé par le client, parmi les certificats placés sur ce plan : celui qui le porte,
et quand deux le portent, celui qui reste valable le plus longtemps. Une poignée de main qui ne
nomme aucun hôte - un client qui joint la passerelle par son adresse, ou n'importe quoi de plus
vieux que l'extension SNI - reçoit le certificat **de repli**, le plus ancien placé sur le plan ;
et sans rien de placé elle reçoit une erreur qui nomme ce qui *est* servi, ce qui est plus utile
qu'un certificat au hasard qu'elle rejettera de toute façon.

Placer un certificat là où un autre répond déjà pour l'un de ses noms est refusé tant que
l'appelant ne dit pas **remplacer** (`PUT /api/certificates/{id}/placement` avec
`"replace": true`) : celui qui est remplacé quitte cette porte et reste dans la réserve.

Allumer TLS sans rien à présenter est refusé : ça transformerait un port qui marche en un port qui refuse
tous les visiteurs.

## Le port en clair peut rediriger

Sur le **plan de données uniquement**, le port en clair peut rediriger vers celui en HTTPS (SSL-06). Les
deux sondes de santé sont exemptées : un `308` se lit comme « pas prêt ». Toute requête pour un nom
qu'aucun certificat placé sur l'application ne porte l'est aussi : un service du cluster qui appelle
`http://meerkat-meerkat.ns.svc:8080` - une lecture du JWKS, un appel d'API interne - serait renvoyé
vers un certificat pour un autre nom, signé par une autorité qu'il ne connaît pas, et échouerait.

Avec la redirection, chaque réponse HTTPS du plan de données porte aussi **HSTS**
(`Strict-Transport-Security: max-age=...`). La redirection ne peut pas protéger la requête qu'elle
redirige : cette première-là part en clair, et qui est sur le réseau - un Wi-Fi public, un proxy
compromis - peut y répondre lui-même sans jamais transmettre la redirection, gardant le visiteur en
HTTP pendant qu'il parle HTTPS à la passerelle (SSL stripping). Avec HSTS le navigateur se souvient,
et réécrit lui-même `http://` en `https://` avant d'envoyer quoi que ce soit.

Il suit la redirection plutôt que d'être un interrupteur à part : forcer HTTPS est déjà
l'engagement - un `301` est lui aussi mémorisé par les navigateurs. Seule la **durée** se choisit, un
jour par défaut, jusqu'à deux ans. Il se retire avec la redirection quand tous les certificats ont
expiré. Jamais envoyé à `localhost` (la promesse vaut pour tous les ports d'un nom d'hôte, donc une
passerelle de développement forcerait HTTPS sur toutes les applications locales de son développeur)
ni à une adresse IP (les navigateurs l'y ignorent), jamais par-dessus la valeur qu'un service ou le
filtre [security-headers](/docs/filters/security-headers) d'une route a posée, et sans
`includeSubDomains`.

> [!NOTE] Seulement sur 443 - une limite des navigateurs
> HSTS n'est envoyé que si le HTTPS est atteint sur **443**, le HTTP en clair étant sur 80. Un
> navigateur applique la promesse à tous les ports du nom, et quand il bascule une requête en HTTPS
> il garde le port : avec HSTS sur `8443`, il réécrirait `http://nom:8080` en `https://nom:8080` -
> un port en clair - et toutes les adresses en clair sous ce nom, celle de la console comprise,
> cesseraient de répondre. Sur les autres ports, la redirection seule fait le travail, à chaque
> visite, et les réponses HTTPS portent `max-age=0`, qui fait oublier au navigateur une promesse
> faite avant. Le même `max-age=0` est envoyé quand Force HTTPS est éteint, pour que les
> navigateurs cessent d'insister dès leur visite HTTPS suivante plutôt qu'à la fin de la durée.

> [!WARNING]
> Un navigateur tient la promesse pendant toute la durée, même si les certificats disparaissent, et
> ne propose plus de passer outre un mauvais certificat. Commencez par un jour ; allongez une fois le
> HTTPS installé.

## ACME n'est pas Let's Encrypt

ACME fait partie de l'édition **Enterprise**. Une configuration importée sur l'image
communautaire laisse de côté ses autorités et commandes, et son plan les nomme. Une
passerelle passée de l'image Enterprise à l'image communautaire garde ce que sa base
contient mais ne sollicite aucune autorité, et l'écran TLS le dit : ces certificats ne
seront pas renouvelés.

Une installation configure autant d'**autorités** qu'elle en fréquente - Let's Encrypt pour
un domaine, ZeroSSL pour celui d'un client, le `step-ca` de l'entreprise pour les noms
internes - chacune un compte ACME nommé (SSL-05). Le répertoire d'une autorité « autre » est
une **URL**, et c'est tout l'intérêt : la moitié des installations auxquelles Meerkat est
destiné n'ont aucune route vers internet. Un `step-ca` interne, un EJBCA, une autorité Windows
avec le rôle ACME : n'importe laquelle répond ici.

| Champ | À quoi il sert |
|---|---|
| Fournisseur | un fournisseur connu fixe le répertoire et dit ce qu'il lui faut d'autre ; *Another authority* porte sa propre URL |
| CA racine | l'autorité qui signe le certificat HTTPS **du serveur ACME lui-même**, en PEM - un serveur ACME privé est en général derrière un certificat privé |
| Identifiant et clé HMAC EAB | External Account Binding : sous quel compte cette passerelle s'enregistre. ZeroSSL et Google l'exigent, Let's Encrypt l'ignore |
| Adresse de contact | certaines autorités l'exigent ; l'expiration est surveillée par le digest quotidien |
| Conditions acceptées | un acte juridique, donc jamais supposé |

Chaque autorité garde sa clé de compte et ce qu'elle a émis dans **son propre coin du cache
partagé** ; changer son répertoire lui en donne un neuf, puisqu'un compte enregistré chez une
autorité ne vaut rien chez une autre.

Les noms ne sont pas un champ du compte : ce sont des **commandes** dans la réserve de
certificats (*Ask* suivi du nom de l'autorité), chacune placée sur la console, l'application ou
les deux, comme n'importe quel certificat. Une porte ne demande à une autorité que les noms des
commandes placées sur elle, et la liste est fermée : une politique ouverte laisserait n'importe
qui joindre la passerelle par son adresse avec un SNI inventé et brûler le quota de l'autorité
sur des noms que personne ne possède. Une autorité sans commande placée n'est pas armée du tout.

**Placer une commande envoie la demande aussitôt**, en arrière-plan, par le même chemin
qu'une poignée de main - verrou de cluster compris - plutôt que d'attendre que le premier
visiteur subisse tout l'échange ou son échec. Le noeud qui a demandé garde ce qui s'est passé
(en cours, ou le refus de l'autorité, expliqué) pour que la réserve l'affiche ; le certificat
lui-même arrive dans le cache partagé que tous les noeuds lisent. Commandes et autorités
voyagent avec l'export de configuration - la clé HMAC d'un compte en `$nom`, jamais en clair ;
ce que l'autorité a émis reste dans le cache.

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

## Avant qu'un certificat expire

La console montre le compte à rebours de chaque certificat, et le
[digest quotidien](/docs/console/mail-relay) l'envoie par courriel aux administrateurs :
les certificats qui expirent dans son horizon, puis ceux qui ont expiré. Un certificat
automatique qui entre dans cette fenêtre est un certificat dont le renouvellement échoue.
Un certificat en réserve dont un certificat plus durable porte tous les noms - ce qu'un
renouvellement laisse derrière lui - n'est pas cité.

## Ce qui manque

- **HTTP/3** (SSL-07) : il faudrait une dépendance QUIC hors bibliothèque standard, une écoute UDP et
 `Alt-Svc`.
