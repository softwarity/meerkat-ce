---
title: Domains & TLS
section: La console
order: 158
summary: Les plans console, application et interne, un onglet chacun - leurs certificats, générés, importés ou émis par une autorité, qui sert leurs noms publics en HTTPS, et les adresses d'un proxy placé devant.
---

# Domains & TLS

**Infra > Domains & TLS.** Les plans de cette gateway, un onglet chacun : les certificats
que chacun sert, ses noms publics, et qui les sert en HTTPS, la gateway ou un proxy
placé devant. L'écran se trouve dans le plan Infra pour la même raison que le
relais de messagerie : c'est une propriété de l'installation, pas de l'application
qu'elle sert.

Il n'existe pas d'interrupteur *activer HTTPS*. **C'est un certificat placé sur un
plan qui ouvre sa porte**, et c'est son retrait qui la referme. Un interrupteur
que l'on peut activer sans rien derrière est un interrupteur qui ment.

![L'écran Domains & TLS : un onglet par plan, Console, Application et Internal, chacun avec ses ports, qui sert ses noms publics et ses certificats, puis l'onglet Trusted authorities](img/console/tls.webp)

Les boutons **ACME**, **Platform certificates** et **On a local machine** se
trouvent dans le bandeau, à côté du titre ; tout le reste se range dans l'onglet
du plan qu'il concerne.

## Plan par plan

Un certificat répond pour **les noms qu'il porte** : ils sont lus dans le
certificat lui-même, pas saisis à côté. Un même certificat peut porter plusieurs
noms, un joker (`*.acme.example`) ou une adresse IP, et il les sert tous.

L'écran donne à chaque plan un onglet, sous le nom que lui donne le runtime :
**Console**, **Application** et **Internal**, ce dernier étant ce qu'appellent les
services placés derrière la gateway. Un quatrième onglet, **Trusted authorities**,
réunit ce que la gateway accepte quand elle appelle un upstream (voir plus bas).
Un petit cadenas sur l'onglet d'un plan signale que son HTTPS est ouvert. L'onglet
de chaque plan porte sa liste **Certificates** et son propre menu **Add
certificate**. Chaque ligne de la liste donne les noms d'un certificat, sa
provenance (*Imported*, *Self-signed*, *Signed on request*, *File system*,
*cert-manager*) et sa durée de validité.

- **Un certificat ajouté depuis le menu d'un plan arrive sur ce plan**, et y est
  servi aussitôt. Le même fichier peut être ajouté de nouveau depuis l'onglet d'un
  autre plan, pour y être servi aussi.
- **La corbeille de la ligne d'un certificat est la seule façon de l'enlever.**
  Si un autre plan sert aussi le certificat, il quitte seulement ce plan, après
  une confirmation qui nomme où il reste ; si ce plan est le seul, le certificat
  est supprimé, et sa clé privée avec lui - ainsi que son Certificate, s'il a été
  demandé à cert-manager.
- **Unassigned certificates** n'apparaît que lorsqu'un certificat n'est sur aucun
  plan - celui qu'un remplacement a retiré, par exemple (voir
  *Deux certificats pour un même nom*, plus bas). Cliquez sur l'une de ses
  étiquettes de plan pour l'y placer, ou supprimez-le.

La première ligne d'un onglet donne, à droite, les deux ports du plan, tels
qu'ils sont joignables de l'extérieur - `HTTP:19090  HTTPS:19443`. HTTP porte son
interrupteur, HTTPS un cadenas, fermé quand un certificat y est servi, et rouge
quand le déploiement ne le publie pas (voir
[Les ports HTTP et HTTPS](#les-ports-http-et-https)). À gauche, les **Internal names** : les noms par lesquels le runtime
joint le plan, cochés quand un certificat servi sur le plan les porte. Ils
apparaissent sur Internal, et sur Console ou Application quand un proxy est placé
devant (voir [Qui sert les noms publics](#qui-sert-les-noms-publics)). Sous un
plan que la gateway sert elle-même figurent les liens HTTPS de ses noms. Lorsque
la console est servie en HTTPS par Meerkat et qu'aucun de ses certificats ne
porte le nom par lequel vous l'avez jointe, le plan le signale : le navigateur
recevrait un certificat établi pour un autre nom, et le refuserait. Sur le plan
Internal, les Internal names sont tous les noms que le runtime résout vers lui -
les quatre formes d'un Service Kubernetes ou OpenShift, un service Swarm et son
nom court dans la stack, un service compose et son conteneur. Un certificat qui
n'en porte aucun est refusé : il ne peut pas
s'y poser. À l'inverse, un plan Console ou Application que la gateway sert elle-même
refuse un certificat qui ne porte que ces noms - ils mènent aux ports du plan
interne, jamais aux siens ; derrière un proxy, les noms internes au runtime sont
justement ce que porte son certificat (voir
[Qui sert les noms publics](#qui-sert-les-noms-publics)). Le dialogue d'import le dit dès qu'un PEM est
collé, et celui du système de fichiers à côté de chaque certificat monté. En
HTTPS, l'adresse donnée aux services est le premier de ces noms que porte le
certificat. Le
bouton **Platform certificates**, dans le bandeau, ouvre un guide pour la
plateforme où tourne la gateway - Kubernetes (cert-manager), OpenShift (son
service CA), une autorité à vous, Swarm, compose : d'où vient le certificat des
noms d'un service, pour le plan Internal, et pour le saut d'un proxy placé devant
vers la Console ou l'Application. Ses modèles valent pour n'importe quel service de la plateforme,
puisque tous servent du HTTPS de la même façon, avec `<service>` et
`<namespace>` à remplir, et disent d'ajouter l'autorité aux Trusted authorities
pour que la gateway les appelle en HTTPS. **Only Meerkat** les remplit avec les
noms de cette gateway, et ajoute ce qui lui est propre : la valeur du chart qui
le monte, et son étiquette Internal.

**Add certificate, Ask cert-manager**, sur Kubernetes seulement, fait demander à la
gateway, à cert-manager, un certificat portant les noms du plan à l'intérieur du
cluster : ceux du plan Internal, ou ceux du saut d'un proxy placé devant vers la
Console ou l'Application. Aussitôt quand un seul émetteur convient, après l'avoir
choisi quand il y en a plusieurs ; la gateway décide le reste. L'entrée est grisée
ailleurs, avec sa raison - un plan que la gateway sert elle-même tient son
certificat public d'une autorité, d'un import ou du système de fichiers. Voir
[Demandé à cert-manager](/docs/operations/tls#demand-cert-manager).

**Add certificate, From the file system** liste ce que le runtime a monté dans le
conteneur - un Secret Kubernetes, un secret Swarm, un volume compose -, avec la
clé trouvée à côté de chaque certificat. Seuls les chemins sont gardés, et un
renouvellement écrit dans les fichiers est servi tout seul. Voir
[Un certificat depuis le système de fichiers](/docs/operations/tls#un-certificat-depuis-le-systme-de-fichiers).

La ligne affichée sous un certificat répond à la question *est-ce que cela va
fonctionner*, jamais à *voici un code d'état* :

| Ce qu'elle affiche | Ce que cela signifie |
|---|---|
| Valid until *date* | Rien à faire |
| Expires in *N* days | Il reste moins de trente jours, et la couleur le montre |
| Expired on *date* | Les navigateurs le refusent déjà |
| Awaiting signature | Une demande de signature a été créée et n'a pas encore reçu de réponse |

Deux autres avertissements apparaissent là où ils ont leur importance : un badge
**self-signed** (personne d'autre que lui-même ne s'en porte garant), et **No
intermediate**, pour une chaîne qui fonctionne avec `curl` mais échoue dans un
navigateur qui n'a jamais rencontré l'émetteur.

## Ajouter un certificat

Le menu **Add certificate** de chaque plan propose ce qui convient à ce plan et à
son mode, et pose ce qu'il ajoute sur ce plan :

| Plan | Son menu |
|---|---|
| Console ou Application, servi par **Meerkat** | auto-signé, paire PEM, système de fichiers, keystore, demande de signature, et un **Ask** par autorité ACME |
| Console ou Application, **A proxy in front** | paire PEM, système de fichiers, keystore, Ask cert-manager |
| Internal | auto-signé, paire PEM, système de fichiers, keystore, Ask cert-manager, demande de signature |

Les entrées :

1. **Generate a self-signed one** - les noms (séparés par des espaces ou des
   virgules : noms d'hôte, joker, adresses IP), l'organisation, le type de
   clé (ECDSA P-256 ou P-384, RSA 2048 ou 4096) et la validité en jours. Cela
   convient à un environnement de test, et les navigateurs afficheront un
   avertissement. `localhost` porte aussi `127.0.0.1` et `::1`.
2. **Import a PEM pair** - le certificat (suivi de ses intermédiaires) et sa clé
   privée : collez-les, choisissez les fichiers ou déposez-les dans la fenêtre.
   Chaque fichier est rangé à sa place, d'après son contenu.
3. **Import a keystore** - un fichier `.p12` / `.pfx`, choisi ou déposé, et son
   mot de passe.
4. **Create a signing request** - les noms et la clé, à destination de votre
   propre autorité. La ligne affiche alors *Awaiting signature* ; lorsque la
   réponse vous revient, **Adopt** permet de la coller, et le certificat peut être
   placé.
5. **Ask** suivi du nom d'une autorité - une entrée par autorité ACME configurée
   (voir plus bas). Des noms d'hôte uniquement : ni joker ni adresse IP,
   puisque l'autorité vérifie chaque nom en s'y connectant sur le port 443. La
   ligne nomme son autorité ; ajouté depuis le menu d'un plan, il est demandé
   aussitôt.

## Deux certificats pour un même nom

Quand vous placez un certificat sur un plan où un autre répond déjà pour l'un
de ses noms, une confirmation vous est demandée : elle nomme le certificat en
conflit, et **Replace** le retire du plan - il est gardé, sous **Unassigned
certificates** s'il n'est sur aucun autre plan. Si le certificat remplacé portait des noms que le
nouveau ne porte pas, la question précise lesquels cesseront d'être servis en HTTPS
sur ce plan.

Un renouvellement se fait de la même manière : ajoutez le nouveau certificat au
plan, remplacez. L'ancien est gardé jusqu'à ce que vous le supprimiez.

Un client qui n'envoie aucun nom de serveur - parce qu'il a joint la gateway
par son adresse IP - reçoit le certificat **le plus ancien** servi sur ce plan.

La suppression - la corbeille sur le seul plan où se trouve un certificat, ou sur
la carte **Unassigned certificates** - détruit la clé privée. Si le certificat est
servi, la question précise quel plan repasse en HTTP simple.

Sur un ordinateur portable ou dans un environnement de test, le bouton **On a
local machine** détaille les étapes pour obtenir un certificat que les navigateurs
acceptent sans avertissement : le fichier hosts, une autorité locale (mkcert), un
seul certificat pour tous les noms, puis l'import - voir
[TLS et certificats](/docs/operations/tls#sur-une-machine-locale).
Le tiroir se termine sur la destination du certificat, l'une ou l'autre : importé
sur un plan servi par **Meerkat**, la gateway le présente elle-même ; posé
sur un proxy devant, le plan dit **A proxy in front** et déclare les noms.

## Les ports HTTP et HTTPS

Les deux ports, à droite de la première ligne d'un onglet, ne sont pas deux
interrupteurs. **HTTPS n'en a pas** : c'est un état, que montre son cadenas -
fermé, dans la couleur principale, quand un certificat sert HTTPS sur le plan ;
ouvert et gris sinon. Il s'ouvre avec un certificat placé sur le plan et se
ferme quand le dernier est retiré. `HTTP:<port>` a un interrupteur, sur chacun
des trois onglets, et le basculer demande d'abord une confirmation : **Apply**
la donne.

**HTTP éteint n'arrête pas le port.** Il n'écoute plus que sur `127.0.0.1` :
hors de portée du réseau, il répond encore à un `kubectl port-forward` - le
chemin pour réparer un certificat cassé - et au health check que l'image lance
sur elle-même (`meerkat health`). Voir
[Couper le port en clair](/docs/operations/tls#couper-le-port-en-clair).

Deux règles gardent chaque plan joignable :

- **HTTP ne s'éteint que si un certificat sert HTTPS sur le plan.** Sinon
  l'interrupteur revient en place, avec *Add a certificate first: with HTTP off,
  HTTPS is the only way in.*
- **Le dernier certificat reste tant que HTTP est éteint.** Sa corbeille dit
  alors *Switch HTTP on before removing the last certificate: nothing else would
  answer.*, et ne retire rien.

**Sur l'onglet Console, chaque port dit aussi qui l'appelle**, en direct et mis
en commun entre les réplicas de la gateway : *Receiving, from 10.42.1.178* quand
quelque chose l'a appelé dans les 30 dernières secondes - l'adresse d'où vient
la connexion, un proxy placé devant ou le navigateur lui-même - ou *Nothing in
the last 30 s*. Les health checks ne comptent pas. **La porte en usage ne se
coupe pas :**

- cliquer sur HTTP pendant que cette page ou un proxy passe par HTTP affiche
  *Access the console through HTTPS:19443 before switching HTTP off.*, avec le
  port joignable de l'extérieur, et rien ne change ;
- retirer le dernier certificat de la console pendant que cette page ou un
  proxy passe par HTTPS affiche *Access the console through HTTP:19090 before
  removing its last certificate.*

La gateway refuse la même chose par l'API. Pour passer en HTTPS de bout en bout
une console placée derrière un proxy, dans cet ordre :

1. placez le certificat sur l'onglet Console : HTTPS s'ouvre ;
2. dirigez le proxy vers le port HTTPS de la console ;
3. ouvrez la console à travers lui ;
4. vérifiez que HTTP affiche *Nothing in the last 30 s* ;
5. éteignez HTTP.

Les onglets Application et Internal n'affichent aucun trafic et n'ont pas cette
règle : leur HTTP s'éteint dès qu'un certificat y sert HTTPS.

## Force HTTPS

Ce réglage se trouve dans le plan Application, quand cette gateway sert ses noms
(derrière un proxy, rediriger est l'affaire du proxy). Les appelants qui arrivent sur le
port en clair sont renvoyés vers HTTPS, du moins ceux qui ont utilisé un nom porté
par un certificat placé sur l'application. Un service qui appelle la gateway
par son nom dans le cluster (`http://meerkat:8080`) reste en clair : renvoyé vers
HTTPS, il recevrait un certificat établi pour un autre nom, émis par une autorité
qu'il ne connaît pas, et cesserait de fonctionner. La console n'est jamais forcée,
pas plus que le health check de vivacité. Si tous les certificats expirent, la
redirection **se suspend d'elle-même** et le signale, plutôt que d'envoyer les
appelants vers une porte qu'aucun d'eux n'ouvrira.

L'interrupteur est grisé tant que la porte HTTPS de l'application n'est pas
ouverte, c'est-à-dire tant qu'aucun certificat n'est servi sur l'application, et une ligne l'explique. Juste en dessous, **HSTS duration**
devient actif dès que HTTPS est forcé : forcer HTTPS envoie aussi HSTS, de sorte
que les navigateurs cessent d'envoyer en clair jusqu'à la toute première requête,
celle qu'une redirection ne peut pas protéger. La durée par défaut est d'un jour,
et va jusqu'à deux ans. HSTS n'est jamais envoyé à localhost ni à une adresse IP ;
une route ou un service qui fixe sa propre valeur la conserve. Voir
[TLS](/docs/operations/tls).

## Qui sert les noms publics

![L'onglet Application derrière un proxy : qui sert son HTTPS, les noms publics déclarés, les adresses du proxy, puis ses certificats](img/console/tls-proxied.webp)

Les onglets Console et Application disent d'abord **HTTPS served by** :
**Meerkat** ou **A proxy in front**. Le sélecteur ne fait que proposer : choisir
l'autre mode fait apparaître **Apply** et **Cancel**, et Apply demande une
confirmation avant d'enregistrer. Changer de mode garde les noms et les
certificats : un certificat qui ne convient pas au nouveau mode reste placé,
grisé et marqué *Inactive in this mode*, et sert de nouveau dès que le plan
revient en arrière. Chaque changement voyage avec un export de configuration et
atteint chaque nœud.

**Un changement du plan Console qui vous déconnecterait est refusé.** Quand vous
joignez la console par un proxy qui dit à Meerkat que le navigateur était en
HTTPS, un plan Console qui ne croirait plus ce proxy verrait votre propre session
en http simple - son cookie ne reviendrait plus - et vous déconnecterait. La
gateway refuse l'enregistrement et dit pourquoi : gardez **A proxy in front**, ou
ouvrez d'abord la console sur son propre port HTTPS.

**Public names** réunit les noms complets par lesquels les navigateurs joignent le
plan, un à chaque fois. Tapez un nom puis Entrée, une espace ou une virgule ; une
liste collée d'un coup donne une puce par nom, et la croix d'une puce la retire ;
chaque changement est enregistré aussitôt. Un nom refusé dit pourquoi : un schéma, un port, un chemin ou une adresse IP n'y ont
pas leur place, pas plus qu'un joker - un certificat peut porter `*.acme.example`,
un nom déclaré non, puisqu'il se porterait garant de tous les noms en dessous. Un
nom n'appartient qu'à un seul plan. Sur l'Application, **le premier nom** est celui
avec lequel s'écrivent les liens des mails et les adresses absolues quand aucune
requête ne dit lequel.

**Meerkat.** La gateway présente elle-même les noms, avec les certificats
placés sur le plan. Un nom ajouté doit être couvert par l'un d'eux, sinon il est
refusé (*no certificate placed on the application covers ...*) : placez d'abord le
certificat, puis déclarez le nom. Les noms gardés lors d'un passage depuis un
proxy, qu'aucun certificat placé ne couvre, restent listés avec un avertissement -
les navigateurs refuseront leur HTTPS tant qu'un certificat pour eux n'est pas
placé. Dès que des noms sont déclarés, le `X-Forwarded-For` de personne n'est cru
sur le plan : rien ne se tient devant, donc un pair qui en écrit un est un appelant
qui choisit sa propre adresse.

**A proxy in front.** Un Ingress, nginx ou un load balancer de cloud sert les noms
en HTTPS, et la gateway sait qu'une requête pour l'un d'eux a atteint le navigateur
en HTTPS sans croire aucun en-tête : ses cookies sont `Secure`, ses liens en
`https://`, et un mail demandé sous ce nom porte un lien qui y revient plutôt qu'un
code. Ces noms n'ouvrent aucune porte HTTPS de la gateway, ne sollicitent aucune
autorité, ne sont jamais redirigés par Force HTTPS et ne reçoivent jamais HSTS :
c'est le proxy qui tient ces promesses.

Un certificat placé sur un tel plan est facultatif, et **chiffre le saut du proxy
vers la gateway** - un Ingress dont le backend est en HTTPS, une Route OpenShift en
`reencrypt`. Pour que ce saut soit chiffré, le proxy lui-même doit être dirigé
vers le port HTTPS du plan et lui parler en TLS : un Ingress qui vise encore le
port HTTP laisse le saut en clair, et le certificat inutilisé - sur la Console,
la ligne sous chaque port montre lequel le proxy appelle vraiment. Il porte les noms
du plan à l'intérieur du runtime, jamais les noms publics, et l'onglet les liste
comme ses **Internal names**, en cochant ceux que couvre un certificat servi : sur Kubernetes, le Service du chart sous ses quatre
formes (`<release>-meerkat` pour l'application, `<release>-meerkat-admin` pour la
console) ; sous Swarm ou compose, le seul nom de service que partagent tous les
ports. Quand le runtime ne dit rien, le plan sert tous les certificats placés sauf
ceux qui couvrent ses noms publics, que c'est au proxy de présenter. **Add
certificate** y propose une paire PEM, le système de fichiers, un keystore et **Ask
cert-manager**.

**Proxy addresses (optional)** dit de qui le plan croit `X-Forwarded-For` et
`X-Forwarded-Proto` :

- **vide** - le défaut, pour un proxy qui tourne dans le cluster ou sur un réseau
  privé : la boucle locale et les plages privées sont crues, d'où vient un
  contrôleur Ingress ou un load balancer à l'intérieur du cluster ;
- **ses adresses** - des plages CIDR ou des adresses seules, saisies en puces,
  pour un proxy qui joint Meerkat depuis des adresses publiques : un CDN, ou un
  load balancer de cloud comme ceux de Google Cloud (`130.211.0.0/22`,
  `35.191.0.0/16`). Elles remplacent les plages privées : nommez aussi un proxy
  privé s'il y en a un entre les deux.

Un plan dont personne n'a rien dit garde l'ancienne règle, les plages privées.
Voir [Derrière un proxy ou un load balancer](/docs/deploy/one-gateway#derrire-un-proxy-ou-un-load-balancer)
et [Derrière un proxy qui termine le TLS](/docs/operations/tls#derrire-un-proxy-qui-termine-le-tls).

## Autorités ACME

> [!NOTE] Enterprise
> ACME fait partie de l'édition Enterprise (SSL-05). Sur l'image Community, le
> bouton est verrouillé ; les certificats sont générés, importés ou signés sur
> demande. Une configuration que vous y importez laisse de côté sa partie ACME, et
> son plan le signale.

Le bouton **ACME**, dans le bandeau à côté de **On a local machine**, ouvre un tiroir : un formulaire en
haut et, en dessous, les autorités déjà configurées, chacune avec ce qui lui est
demandé. Il y en a une poignée, jamais cinquante, et chaque autorité enregistrée
devient une entrée **Ask** du menu Add certificate d'un plan Console ou Application
que Meerkat sert lui-même.

Choisissez un **fournisseur** : le formulaire n'affiche que ce dont celui-ci a
besoin.

| Fournisseur | Ce qu'il demande |
|---|---|
| Let's Encrypt (staging) | l'acceptation des conditions, rien d'autre. Ses certificats ne sont pas reconnus par les navigateurs, mais ils ne sont pas décomptés des limites hebdomadaires : c'est l'endroit où vérifier qu'un nom atteint bien la gateway |
| Let's Encrypt | l'acceptation des conditions, rien d'autre. Cinquante certificats par semaine et par domaine enregistré |
| ZeroSSL | un identifiant de clé de compte et une clé HMAC - tableau de bord ZeroSSL, Developer, *EAB Credentials for ACME Clients* |
| Google Trust Services | un identifiant de clé de compte et une clé HMAC - `gcloud publicca external-account-keys create` |
| Another authority | l'URL de son annuaire, son certificat racine si cette gateway ne lui fait pas déjà confiance (`step ca root` pour step-ca), une liaison de compte et un contact si elle les exige - toutes ces valeurs vous sont fournies par celui qui l'exploite |

À l'enregistrement, la gateway interroge l'annuaire de l'autorité pour vérifier
qu'il répond : une URL erronée ou une autorité injoignable est signalée aussitôt.
Aucun compte n'est encore ouvert ; cela attend la première demande de certificat.
La clé HMAC est un secret et passe par le [coffre](/docs/console/vault). Une
autorité à laquelle un certificat est encore demandé ne peut pas être supprimée ;
le refus nomme ce qui la sollicite.

### Ce qui se passe lors d'une demande

Rien ne sort de la gateway quand vous enregistrez une autorité. C'est **le
placement sur un plan** d'un certificat qui lui est demandé qui envoie la demande,
aussitôt et en arrière-plan - et l'ajouter depuis le menu d'un plan l'y place :

1. la première fois, un compte est ouvert auprès de l'autorité (et ses conditions
   sont acceptées) ;
2. le certificat est commandé, et l'autorité réclame une preuve pour chaque nom ;
3. elle se connecte au nom **sur le port 443**, où la gateway répond à son défi
   (TLS-ALPN : le port 80 reste fermé) ;
4. elle signe, le certificat est enregistré et servi aussitôt, puis renouvelé un
   mois avant son échéance, sans aucune intervention.

Pendant ce temps, la ligne affiche *Asking the authority...* - cela dure quelques
secondes, et l'écran se met à jour tout seul. Elle affiche ensuite l'échéance du
certificat, ou **le refus de l'autorité** accompagné d'un bouton **Retry**. Les
refus les plus courants sont expliqués : le nom ne pointe pas vers la gateway
(ou il passe par un proxy - chez Cloudflare, il doit être en *DNS only*), le port
443 n'atteint pas la gateway, un enregistrement CAA réserve le domaine à une
autre autorité, le rate limit est atteint.

![Le tiroir des autorités ACME : le fournisseur et ses champs, puis les autorités déjà configurées avec ce qui leur est demandé](img/console/tls-authorities.webp)

Le suivi des expirations ne repose pas sur les e-mails de l'autorité - Let's
Encrypt a cessé d'en envoyer en 2025 : le
[récapitulatif quotidien](/docs/console/mail-relay) cite tout certificat
automatique qui entre dans sa fenêtre de renouvellement, signe que ce
renouvellement échoue.

## Autorités de confiance

Le dernier onglet, **Trusted authorities**, liste ce que la gateway accepte
quand elle appelle un upstream en HTTPS, au-delà des autorités publiques. **Add
authority** ouvre une fenêtre : un nom, et le certificat de l'autorité -
choisissez son fichier, déposez-le sur la fenêtre, ou collez-le.

Sur Kubernetes ou OpenShift, **From the cluster** liste les autorités que le
cluster publie, trouvées plutôt que nommées : l'autorité des services
qu'OpenShift monte dans chaque pod, les émetteurs cert-manager qui signent avec
une autorité à eux, les bundles trust-manager qui atteignent le namespace de la
gateway, et l'autorité du cluster lui-même (qui signe l'API server, rarement
les services). Sous chacune, ce qu'elle signe ici : les routes en https que la
gateway refuse aujourd'hui et qu'elle débloquerait, et les services que la
plateforme enregistre comme signés par elle. Celle qui débloque une route
refusée est cochée d'avance ; une route refusée signée par une autorité que le
cluster ne publie pas est nommée, à importer à la main. Cochez-en une ou plusieurs, puis
**Trust**. La fenêtre propose ensuite les routes encore en http dont le service
répond maintenant en https - essayées par la gateway, avec le port et le nom
que porte le certificat - à passer en **Switch to HTTPS** ou à laisser. Quand le cluster
n'en publie aucune, la fenêtre le dit et montre un émetteur cert-manager qui en
publierait une ; quand une source est là mais que le chart ne laisse pas la
gateway la lire, la fenêtre nomme la valeur à positionner. Voir
[Trouvées dans le cluster](/docs/operations/tls#trouves-dans-le-cluster).

Chaque entrée montre les certificats qu'elle contient et jusqu'à quand. Voir
[Appeler des upstreams en HTTPS](/docs/operations/tls#appeler-des-upstreams-en-https).

## Pièges

- **Un certificat répond pour les noms qu'il porte.** Un navigateur qui demande
  un nom qu'aucun certificat du plan ne porte reçoit le plus ancien, et le
  refuse.
- **Une autorité publique ne voit pas votre DNS privé.** Pour les noms internes,
  utilisez une autorité interne, ou importez un certificat.
- **N'oubliez pas les intermédiaires** lors d'un import : la console vous le
  signalera, mais seulement après coup.
- **La clé HMAC de l'EAB est un secret** et passe par le
  [coffre](/docs/console/vault), comme tous les autres.
