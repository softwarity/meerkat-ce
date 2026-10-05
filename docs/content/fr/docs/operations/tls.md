---
title: TLS et certificats
section: Exploitation
order: 224
summary: Les quatre façons de faire entrer un certificat, ce que contient la base de données, et ce qu'un cluster doit partager.
---

# TLS et certificats

Un certificat entre une seule fois dans une **réserve**, puis il est **placé** sur la
console, sur l'application, ou sur les deux (SSL-08). Ses noms lui appartiennent - ce
sont ceux pour lesquels le certificat lui-même déclare répondre. Un certificat qui porte
plusieurs noms, un nom générique ou une adresse IP les sert donc tous, et un certificat
utilisé par les deux plans est une seule entrée, pas deux copies à garder synchronisées.

Il n'existe pas d'interrupteur "activer HTTPS" : **c'est le certificat placé sur un plan
qui ouvre sa porte**, et c'est son retrait qui la ferme. Un interrupteur qui peut être
activé sans rien derrière est un interrupteur qui ment.

![L'écran TLS](img/console/tls.webp)

## Les quatre portes

Aucune n'est facultative, car les environnements où tourne Meerkat ne se ressemblent
pas (SSL-01) :

| Porte | Quand c'est la bonne |
|---|---|
| **Import** | vous détenez déjà le certificat et sa clé |
| **Auto-signé** | un labo, un nom interne, un premier déploiement |
| **Demande de signature** | la gateway crée la clé et la CSR, votre autorité signe, vous importez la réponse |
| **ACME** | une autorité émet et renouvelle toute seule |

Une demande de signature est une ligne qui détient une clé et ne sert rien : elle existe
pour que vous puissiez télécharger de nouveau la demande en attendant la réponse.

## Les ports

Une porte HTTPS par plan, à côté de la porte en clair, que la gateway ouvre et ferme
sans s'arrêter (SSL-02) :

| Plan | En clair | HTTPS |
|---|---|---|
| Données | `MEERKAT_ADDR`, `:8080` | `MEERKAT_TLS_ADDR`, `:8443` |
| Contrôle | `MEERKAT_ADMIN_ADDR`, `:9090` | `MEERKAT_ADMIN_TLS_ADDR`, `:9443` |

Un port dont on ne peut pas nommer le protocole est un port pour lequel personne ne peut
écrire ni règle de pare-feu ni procédure d'exploitation : voilà pourquoi il y en a
quatre et non deux.

**Ce sont les ports vus de l'intérieur.** Entre eux et un navigateur, il y a en général
une correspondance de ports : un Service Kubernetes qui publie `9443` sur `19443`, un
`-p` de Docker, un ingress Swarm. La gateway interroge son environnement d'exécution -
son propre pod et les Services qui le sélectionnent, ou son conteneur via le socket
Docker - et les liens de l'écran TLS portent le port réellement joignable de
l'extérieur. Quand l'environnement ne publie pas de porte HTTPS, l'écran le dit au lieu
de proposer un lien qui ne mène nulle part. Le chart Helm publie par défaut les deux
portes HTTPS (`service.appTlsPort`, `service.adminTlsPort`) et accorde le droit de
lecture nécessaire (`rbac.read`).

> [!NOTE] Docker Desktop
> Son Kubernetes publie les ports d'un LoadBalancer sur `localhost` à la création du
> Service, et ignore les ports ajoutés par la suite. Après la mise à niveau d'une
> release vers un chart qui ajoute les ports HTTPS, supprimez les deux Services et
> relancez `helm upgrade`.

Remplacer un certificat revient à échanger une liste derrière un verrou : la
négociation TLS la lit par une fonction de rappel, si bien que l'écoute ne bouge pas et
qu'aucune connexion n'est coupée. Pour ouvrir une porte, la gateway réserve
**d'abord** le port, et signale l'échec avant d'avoir changé quoi que ce soit - un port
déjà pris ne doit pas laisser un exploitant sans HTTPS du tout.

## Sur une machine locale

Un portable ou un labo n'a ni nom public ni autorité publique, et un certificat
auto-signé fait protester tous les navigateurs. Le bouton **On a local machine** de
l'écran TLS ouvre la marche à suivre, système par système (macOS, Linux, Windows). Les
noms que vous saisissez - à commencer par celui qui sert à joindre la console - figurent
déjà dans les commandes :

1. **Le fichier hosts** fait pointer les noms vers la gateway - `127.0.0.1` quand
   elle tourne sur la même machine (Docker, un Kubernetes local), son adresse dans les
   autres cas - une VM, minikube, une autre machine ; la console propose l'adresse par
   laquelle vous l'avez jointe quand c'en est une. `localhost` ne demande aucune ligne.
2. **Une autorité locale**, créée par [mkcert](https://github.com/FiloSottile/mkcert) et
   reconnue par cette machine : le magasin du système, Chrome, Edge, Safari, et Firefox
   via `nss`. Redémarrez ensuite les navigateurs.
3. **Un seul certificat pour tous les noms**, signé par cette autorité : `meerkat.pem`
   et son `meerkat-key.pem`.
4. **Importez-le** - Import a PEM pair, en déposant les deux fichiers sur la boîte de
   dialogue - puis faites-le glisser sur la console et sur l'application.

![Le tiroir qui détaille les étapes pour HTTPS sur une machine locale : les noms, le fichier hosts, une autorité locale, un certificat, l'import](img/console/tls-local.webp)

> [!WARNING]
> La clé privée de l'autorité reste dans le dossier qu'affiche `mkcert -CAROOT`. Celui
> qui la détient peut signer pour n'importe quel nom auquel cette machine fait
> confiance : ne la partagez jamais - une autre machine crée la sienne.

## Quel certificat répond à une négociation TLS

Le choix se fait d'après le nom demandé par le client, parmi les certificats placés sur
ce plan : celui qui porte ce nom, et si deux le portent, celui dont la validité dure le
plus longtemps. Une négociation qui ne nomme aucun hôte - un client qui joint la
gateway par son adresse, ou tout ce qui est antérieur à l'extension SNI - reçoit le
certificat **de repli**, le plus ancien de ceux placés sur le plan. Si rien n'est placé,
elle reçoit une erreur qui nomme ce qui *est* servi, ce qui est plus utile qu'un
certificat pris au hasard que le client rejetterait de toute façon.

Placer un certificat là où un autre répond déjà pour l'un de ses noms est refusé tant
que l'appelant ne demande pas explicitement le **remplacement**
(`PUT /api/certificates/{id}/placement` avec `"replace": true`) : le certificat remplacé
quitte cette porte et reste dans la réserve.

Activer TLS sans rien avoir à présenter est refusé : un port qui fonctionne deviendrait
un port qui refuse tous les visiteurs.

## Le port en clair peut rediriger

Sur le **plan de données uniquement**, le port en clair peut rediriger vers le port
HTTPS (SSL-06). Les deux health checks en sont dispensés : un `308` serait interprété
comme "pas prêt". Il en va de même de toute requête destinée à un nom qu'aucun
certificat placé sur l'application ne porte : un service interne au cluster qui appelle
`http://meerkat-meerkat.ns.svc:8080` - pour récupérer le JWKS, pour un appel d'API
interne - serait renvoyé vers un certificat établi pour un autre nom, signé par une
autorité à laquelle il ne fait pas confiance, et échouerait.

Quand la redirection est active, chaque réponse HTTPS du plan de données porte aussi
l'en-tête **HSTS** (`Strict-Transport-Security: max-age=...`). La redirection ne peut
pas protéger la requête qu'elle redirige : cette première requête part en clair, et
quiconque se trouve sur le réseau - un Wi-Fi public, un proxy compromis - peut y
répondre lui-même sans jamais transmettre la redirection. Il maintient alors le visiteur
en HTTP pendant que lui-même parle en HTTPS à la gateway (SSL stripping). Avec HSTS,
le navigateur s'en souvient et réécrit lui-même `http://` en `https://` avant d'envoyer
quoi que ce soit.

HSTS suit la redirection au lieu d'avoir son propre interrupteur : forcer HTTPS
constitue déjà l'engagement - les navigateurs mémorisent aussi un `301`. Seule la
**durée** se choisit : un jour par défaut, deux ans au plus. L'en-tête disparaît avec la
redirection lorsque tous les certificats ont expiré. Il n'est jamais envoyé à
`localhost` (la promesse couvre tous les ports d'un nom d'hôte : une gateway de
développement imposerait HTTPS à toutes les applications locales du développeur) ni à
une adresse IP (les navigateurs l'y ignorent), jamais à la place d'une valeur déjà
définie par un service ou par le filtre
[security-headers](/docs/filters/security-headers) d'une route, et toujours sans
`includeSubDomains`.

> [!NOTE] Uniquement sur 443 - une limite des navigateurs
> HSTS n'est envoyé que lorsque HTTPS est joint sur le port **443**, avec le HTTP en
> clair sur le port 80. Un navigateur applique la promesse à tous les ports du nom, et
> lorsqu'il bascule une requête en HTTPS, il conserve le port : avec HSTS sur `8443`, il
> réécrirait `http://name:8080` en `https://name:8080` - un port en clair - et toutes
> les adresses en clair sous ce nom, celle de la console comprise, cesseraient de
> répondre. Sur les autres ports, la redirection fait le travail à elle seule, à chaque
> visite, et les réponses HTTPS portent `max-age=0`, ce qui fait oublier au navigateur
> une promesse antérieure. Le même `max-age=0` est envoyé quand Force HTTPS est
> désactivé : les navigateurs cessent ainsi d'insister dès leur prochaine visite en
> HTTPS, sans attendre la fin de la durée.

> [!WARNING]
> Un navigateur tient la promesse pendant toute la durée, même si les certificats sont
> retirés, et ne propose plus de passer outre un certificat invalide. Commencez par un
> jour ; allongez la durée une fois HTTPS bien en place.

## ACME n'est pas Let's Encrypt

ACME fait partie de l'édition **Enterprise**. Une configuration importée sur l'image
Community laisse de côté ses autorités et ses commandes, et son plan d'import les cite.
Une gateway passée de l'image Enterprise à l'image Community conserve ce que contient
sa base de données, mais ne sollicite plus aucune autorité, et l'écran TLS le signale :
ces certificats ne seront pas renouvelés.

Une installation déclare autant d'**autorités** qu'elle en utilise - Let's Encrypt pour
un domaine, ZeroSSL pour celui d'un client, le `step-ca` de l'entreprise pour les noms
internes - et chacune est un compte ACME nommé (SSL-05). L'annuaire d'une autorité
personnalisée est une **URL**, et tout est là : la moitié des installations auxquelles
Meerkat se destine n'ont aucun accès à Internet. Un `step-ca` interne, un EJBCA, une
autorité Windows dotée du rôle ACME - toutes répondent ici.

| Champ | À quoi il sert |
|---|---|
| Provider | un fournisseur connu fixe l'annuaire et indique ce qu'il lui faut en plus ; *Another authority* apporte sa propre URL |
| Root CA | l'autorité qui signe le certificat HTTPS **du serveur ACME lui-même**, au format PEM - un serveur ACME privé se trouve en général derrière un certificat privé |
| EAB key id and HMAC key | External Account Binding : le compte sous lequel cette gateway s'enregistre. ZeroSSL et Google l'exigent, Let's Encrypt l'ignore |
| Contact e-mail | certaines autorités en exigent une ; l'expiration est surveillée par le récapitulatif quotidien |
| Terms accepted | un acte juridique, qui n'est donc jamais présumé |

Chaque autorité conserve sa clé de compte et ce qu'elle a émis dans **son propre recoin
du cache partagé** ; changer son annuaire lui en attribue un nouveau, puisqu'un compte
enregistré auprès d'une autorité ne vaut rien auprès d'une autre.

Les noms ne sont pas un champ du compte : ce sont des **commandes** dans la réserve de
certificats (*Ask* suivi du nom de l'autorité), placées chacune sur la console, sur
l'application ou sur les deux, comme n'importe quel certificat. Une porte ne demande à
une autorité que les noms des commandes placées sur elle, et la liste est fermée : une
politique ouverte permettrait à n'importe qui de joindre la gateway par son adresse
avec un SNI inventé, et d'épuiser le rate limit de l'autorité sur des noms qui
n'appartiennent à personne. Une autorité sans aucune commande placée n'est pas armée du
tout.

**Placer une commande envoie la demande immédiatement**, en arrière-plan, par le chemin
qu'emprunterait une négociation TLS - verrou de cluster compris - au lieu d'attendre que
le premier visiteur subisse tout l'échange, ou son échec. Le nœud qui a fait la demande
garde la trace de ce qui s'est passé (demande en cours, ou refus de l'autorité, avec
son explication) pour que la réserve l'affiche ; le certificat lui-même arrive dans le
cache partagé, que tous les nœuds lisent. Les commandes et les autorités voyagent avec
un export de configuration - la clé HMAC d'un compte sous la forme d'un `$name`, jamais
d'un littéral ; ce que l'autorité a émis reste dans le cache.

## Ce que contient la base de données

| Stocké | Comment |
|---|---|
| Le certificat lui-même | en clair - c'est ce que la gateway présente à chaque visiteur |
| La clé privée | **scellée** avec la clé maîtresse du coffre. Elle ne quitte jamais la gateway : ni dans une réponse d'API, ni dans un export, ni dans un journal |
| La demande de signature en attente | en clair, et téléchargeable |
| La clé du compte ACME et les certificats émis | scellés, dans le cache ACME |

La console vérifie aussi en permanence que le certificat répond bien pour l'hôte sous
lequel il a été enregistré. Un certificat qui ne couvre rien échoue au moment de la
négociation TLS, là où personne ne fait le rapprochement avec cet écran.

## En cluster

> [!WARNING]
> Les clés des certificats et la clé du compte ACME sont scellées avec la clé maîtresse
> du coffre, qui est un **fichier local** tant que `MEERKAT_VAULT_KEY` n'est pas
> définie. Un nœud qui possède une autre clé ne peut pas les ouvrir, et il le dit :
> *the private key cannot be unsealed - is this the vault key it was written with?*
> Donnez la même clé à tous les nœuds.

L'émission est sérialisée par un verrou consultatif. Auparavant, cinq nœuds qui
redémarraient ensemble épuisaient en une seconde le quota hebdomadaire de doublons de
l'autorité ; désormais, le nœud qui attend trouve le certificat que le premier vient
d'écrire. Le verrou n'est pris qu'à la première négociation pour un nom et à l'approche
du renouvellement - le placer devant chaque négociation coûterait un aller-retour vers
la base de données pour éviter un événement qui se produit deux fois par an.

Un certificat écrit sur un nœud est rechargé par les autres grâce au bus de changement.

## Avant l'expiration d'un certificat

La console affiche le compte à rebours de chaque certificat, et le
[récapitulatif quotidien](/docs/console/mail-relay) l'envoie par e-mail aux
administrateurs : d'abord les certificats qui expirent dans son horizon, puis ceux qui
ont déjà expiré. Un certificat automatique qui atteint cette fenêtre est un certificat
dont le renouvellement échoue. Un certificat en réserve dont les noms sont portés par un
certificat de plus longue validité - ce qu'un renouvellement laisse derrière lui - n'est
pas cité.

## Ce qui manque

- **HTTP/3** (SSL-07) : il faudrait une dépendance QUIC extérieure à la bibliothèque
 standard, une écoute UDP et `Alt-Svc`.
