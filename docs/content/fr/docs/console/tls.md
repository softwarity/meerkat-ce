---
title: TLS
section: La console
order: 158
summary: Une réserve de certificats que vous placez sur la console et sur l'application - en générer, en importer, ou laisser une autorité les émettre.
---

# TLS

**Infra > TLS.** Les certificats que détient cette gateway, et l'endroit où
chacun est servi. L'écran se trouve dans le plan Infra pour la même raison que le
relais de messagerie : c'est une propriété de l'installation, pas de l'application
qu'elle sert.

Il n'existe pas d'interrupteur *activer HTTPS*. **C'est un certificat placé sur un
plan qui ouvre sa porte**, et c'est son retrait qui la referme. Un interrupteur
que l'on peut activer sans rien derrière est un interrupteur qui ment.

![L'écran TLS : la réserve de certificats, les zones de dépôt Console et Application avec leurs liens, Force HTTPS et sa durée HSTS](img/console/tls.webp)

Ici, la réserve contient quatre lignes : un certificat pour `gateway.acme.example`,
servi sur la console ; un autre pour `apps.acme.example` et `docs.acme.example`,
servi sur l'application ; une demande de signature qui attend encore sa réponse ;
et une commande adressée à Let's Encrypt (staging) pour `status.acme.example`,
gardée en réserve : placée sur une porte, elle serait demandée aussitôt.

## Une réserve, puis un placement

Un certificat répond pour **les noms qu'il porte** : ils sont lus dans le
certificat lui-même, pas saisis à côté. Un même certificat peut porter plusieurs
noms, un joker (`*.acme.example`) ou une adresse IP, et il les sert tous.

- **Certificates** - la réserve : tous les certificats que détient cette
  gateway, qu'ils soient servis ou non. Chaque ligne donne les noms du
  certificat, sa provenance (*Imported*, *Self-signed*, *Signed on request*), sa
  durée de validité, son type de clé et l'endroit où il est servi.
- **Console** et **Application** - les deux portes. **Faites glisser un certificat
  sur l'une d'elles**, ou sur les deux : il y est servi aussitôt. Faites-le glisser
  d'une porte à l'autre pour le déplacer ; ramenez-le dans la réserve, ou cliquez
  sur sa croix, pour le retirer. Les entrées **Serve on** du menu d'un certificat
  font la même chose sans souris.

Le titre de chaque porte donne ses deux ports, tels qu'ils sont joignables de
l'extérieur - `HTTP:19090  HTTPS:19443`. Le port HTTPS est allumé quand un
certificat est placé sur la porte, et rouge quand le déploiement ne le publie pas.
Sous la porte figurent les liens HTTPS des noms qu'elle sert. Lorsque la console
est servie en HTTPS et qu'aucun de ses certificats ne porte le nom par lequel vous
l'avez jointe, la porte le signale : le navigateur recevrait un certificat établi
pour un autre nom, et le refuserait.

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

Le menu **Add certificate** propose cinq entrées. Chacune dépose un certificat
dans la réserve ; il n'est servi nulle part tant que vous ne l'avez pas placé :

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
   ligne nomme son autorité et affiche *Asked as soon as it is placed on a door*.

## Deux certificats pour un même nom

Quand vous placez un certificat sur une porte où un autre répond déjà pour l'un
de ses noms, une confirmation vous est demandée : elle nomme le certificat en
conflit, et **Replace** le retire de la porte - il reste dans la réserve. Si le
certificat remplacé portait des noms que le nouveau ne porte pas, la question
précise lesquels cesseront d'être servis en HTTPS sur cette porte.

Un renouvellement se fait de la même manière : ajoutez le nouveau certificat,
déposez-le sur la porte, remplacez. L'ancien reste dans la réserve jusqu'à ce que
vous le supprimiez.

Un client qui n'envoie aucun nom de serveur - parce qu'il a joint la gateway
par son adresse IP - reçoit le certificat **le plus ancien** placé sur cette
porte.

**Delete** (dans le menu) détruit la clé privée. Si le certificat est servi, la
question précise quelle porte repasse en HTTP simple.

Sur un ordinateur portable ou dans un environnement de test, le bouton **On a
local machine** détaille les étapes pour obtenir un certificat que les navigateurs
acceptent sans avertissement : le fichier hosts, une autorité locale (mkcert), un
seul certificat pour tous les noms, puis l'import - voir
[TLS et certificats](/docs/operations/tls#sur-une-machine-locale).

## Force HTTPS

Ce réglage se trouve dans la porte Application. Les appelants qui arrivent sur le
port en clair sont renvoyés vers HTTPS, du moins ceux qui ont utilisé un nom porté
par un certificat placé sur l'application. Un service qui appelle la gateway
par son nom dans le cluster (`http://meerkat:8080`) reste en clair : renvoyé vers
HTTPS, il recevrait un certificat établi pour un autre nom, émis par une autorité
qu'il ne connaît pas, et cesserait de fonctionner. La console n'est jamais forcée,
pas plus que le health check de vivacité. Si tous les certificats expirent, la
redirection **se suspend d'elle-même** et le signale, plutôt que d'envoyer les
appelants vers une porte qu'aucun d'eux n'ouvrira.

L'interrupteur est toujours présent. Il est grisé tant que la porte HTTPS de
l'application n'est pas ouverte, c'est-à-dire tant qu'aucun certificat n'est placé
sur l'application, et une ligne l'explique. Juste en dessous, **HSTS duration**
devient actif dès que HTTPS est forcé : forcer HTTPS envoie aussi HSTS, de sorte
que les navigateurs cessent d'envoyer en clair jusqu'à la toute première requête,
celle qu'une redirection ne peut pas protéger. La durée par défaut est d'un jour,
et va jusqu'à deux ans. HSTS n'est jamais envoyé à localhost ni à une adresse IP ;
une route ou un service qui fixe sa propre valeur la conserve. Voir
[TLS](/docs/operations/tls).

## Autorités ACME

> [!NOTE] Enterprise
> ACME fait partie de l'édition Enterprise (SSL-05). Sur l'image Community, le
> bouton est verrouillé ; les certificats sont générés, importés ou signés sur
> demande. Une configuration que vous y importez laisse de côté sa partie ACME, et
> son plan le signale.

Le bouton **ACME**, à côté de Add certificate, ouvre un tiroir : un formulaire en
haut et, en dessous, les autorités déjà configurées, chacune avec ce qui lui est
demandé. Il y en a une poignée, jamais cinquante, et chaque autorité enregistrée
devient une entrée **Ask** du menu Add certificate.

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

Rien ne sort de la gateway quand vous enregistrez une autorité, ni quand vous
lui demandez un certificat que vous gardez en réserve. C'est **le placement sur
une porte** qui envoie la demande, aussitôt et en arrière-plan :

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

## Pièges

- **Un certificat répond pour les noms qu'il porte.** Un navigateur qui demande
  un nom qu'aucun certificat de la porte ne porte reçoit le plus ancien, et le
  refuse.
- **Une autorité publique ne voit pas votre DNS privé.** Pour les noms internes,
  utilisez une autorité interne, ou importez un certificat.
- **N'oubliez pas les intermédiaires** lors d'un import : la console vous le
  signalera, mais seulement après coup.
- **La clé HMAC de l'EAB est un secret** et passe par le
  [coffre](/docs/console/vault), comme tous les autres.
