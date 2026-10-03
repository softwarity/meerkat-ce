---
title: TLS
section: La console
order: 158
summary: Une réserve de certificats placés sur la console et l'application - générer, importer, ou laisser une autorité les émettre.
---

# TLS

**Infra > TLS.** Les certificats que cette passerelle détient, et où chacun est
servi. C'est dans le plan Infra pour la même raison que le relais : c'est une
propriété de l'installation, pas de l'application qu'elle sert.

Il n'y a pas d'interrupteur *activer HTTPS*. **Un certificat placé sur un plan est
ce qui ouvre sa porte**, et le retirer est ce qui la ferme. Un interrupteur qui peut
être allumé sans rien derrière est un interrupteur qui ment.

![L'écran TLS : la réserve de certificats, les zones de dépôt Console et Application avec leurs liens, Force HTTPS et sa durée HSTS](img/console/tls.webp)

Ici la réserve tient quatre lignes : un certificat pour `gateway.acme.example` servi sur
la console, un pour `apps.acme.example` et `docs.acme.example` servi sur l'application,
une demande de signature qui attend sa réponse, et une commande à Let's Encrypt
(staging) pour `status.acme.example`, gardée en réserve : placée sur une porte, elle
serait demandée aussitôt.

## Une réserve, puis un placement

Un certificat répond pour **les noms qu'il porte** - lus dans le matériel, pas
saisis à côté. Un certificat peut porter plusieurs noms, un joker (`*.acme.example`)
ou une adresse IP, et les sert tous.

- **Certificates** - la réserve : tous les certificats que la passerelle détient,
  servis ou non. Chaque ligne donne ses noms, sa provenance (*Imported*,
  *Self-signed*, *Signed on request*), sa durée de vie, son type de clé, et où il
  est servi.
- **Console** et **Application** - les deux portes. **Glissez un certificat sur
  l'une**, ou sur les deux : il y est servi aussitôt. Glissez-le d'une porte à
  l'autre pour le déplacer, vers la réserve ou par sa croix pour le retirer. Les
  entrées **Serve on** du menu d'un certificat font la même chose sans souris.

Le titre de chaque porte donne ses deux ports tels que le monde les atteint -
`HTTP:19090  HTTPS:19443` - le port HTTPS allumé quand un certificat y est placé, en
rouge quand le déploiement ne le publie pas. Sous la porte, les liens HTTPS des noms
qu'elle sert. Quand la console est servie en HTTPS et qu'aucun de ses certificats ne
porte le nom par lequel elle a été atteinte, la porte le dit : le navigateur recevrait
un certificat pour un autre nom, et le refuserait.

La ligne sous un certificat répond à *est-ce que ça va marcher*, jamais à *voici un
code d'état* :

| Elle dit | Ça veut dire |
|---|---|
| Valid until *date* | Rien à faire |
| Expires in *N* days | Moins de trente jours, coloré en conséquence |
| Expired on *date* | Les navigateurs le refusent déjà |
| Awaiting signature | Une demande de signature a été créée et pas encore répondue |

Deux avertissements de plus apparaissent là où ils comptent : un badge
**self-signed** (personne ne s'en porte garant sauf lui-même), et **No
intermediate** - une chaîne qui marche avec `curl` et échoue dans un navigateur qui
n'a jamais rencontré l'émetteur.

## Ajouter un certificat

Le menu **Add certificate** a cinq portes. Chacune met un certificat dans la
réserve, servi nulle part tant qu'il n'est pas placé :

1. **Generate a self-signed one** - les noms (séparés par des espaces ou des
   virgules : noms d'hôte, joker, adresses IP), l'organisation, le type de clé
   (ECDSA P-256 ou P-384, RSA 2048 ou 4096) et la validité en jours. Bien pour un
   labo, et les navigateurs préviendront. `localhost` porte aussi `127.0.0.1` et
   `::1`.
2. **Import a PEM pair** - le certificat (puis ses intermédiaires) et sa clé privée :
   collez-les, choisissez les fichiers, ou déposez-les sur la fenêtre. Chaque
   fichier va où il doit, selon ce qu'il contient.
3. **Import a keystore** - un `.p12` / `.pfx`, choisi ou déposé, et son mot de passe.
4. **Create a signing request** - les noms et la clé, pour votre propre autorité. La
   ligne dit alors *Awaiting signature* ; quand la réponse revient, **Adopt** la
   colle, et le certificat peut être placé.
5. **Ask** suivi du nom d'une autorité - une entrée par autorité ACME configurée
   (voir plus bas). Des noms d'hôte seulement : ni joker ni adresse IP, puisque
   l'autorité vérifie chaque nom en s'y connectant sur le port 443. La ligne nomme
   son autorité et dit *Asked as soon as it is placed on a door*.

## Deux certificats pour un nom

Placer un certificat sur une porte où un autre répond déjà pour l'un de ses noms
demande d'abord : la question nomme celui qui gêne, et **Replace** le retire de la
porte - il reste dans la réserve. Quand celui qui est remplacé portait des noms que
le nouveau ne porte pas, la question dit lesquels cessent d'être servis en HTTPS
sur cette porte.

Renouveler est le même geste : ajoutez le nouveau certificat, déposez-le sur la
porte, remplacez. L'ancien reste dans la réserve jusqu'à ce que vous le supprimiez.

Un client qui n'envoie aucun nom de serveur - venu par l'adresse IP de la
passerelle - reçoit le certificat placé **le plus ancien** sur cette porte.

**Delete** (dans le menu) détruit la clé privée. Quand le certificat est servi, la
question dit quelle porte repasse en HTTP.

Sur un portable ou dans un labo, le bouton **On a local machine** ouvre les étapes
vers un certificat que les navigateurs acceptent sans avertissement : le fichier
hosts, une autorité locale (mkcert), un certificat pour tous les noms, puis
l'import - voir [TLS et certificats](/docs/operations/tls#sur-une-machine-locale).

## Force HTTPS

Dans la porte Application. Les appelants qui arrivent sur le port en clair sont
renvoyés vers HTTPS - ceux qui ont utilisé un nom que porte un certificat placé sur
l'application. Un service qui appelle la passerelle par son nom de cluster
(`http://meerkat:8080`) reste en clair : renvoyé vers HTTPS, il tomberait sur un
certificat pour un autre nom, d'une autorité qu'il ne connaît pas, et cesserait de
fonctionner. La console n'est jamais forcée, ni la sonde de vivacité. Si
tous les certificats expirent, la redirection **se met d'elle-même en retrait** et
le dit, plutôt que d'envoyer les appelants vers une porte qu'aucun ne pourra ouvrir.

L'interrupteur est toujours là, grisé tant que la porte HTTPS des applications n'est
pas ouverte - aucun certificat n'est encore placé sur l'application - avec une ligne
qui le dit. **HSTS duration** est en dessous, active dès que HTTPS est forcé :
forcer HTTPS envoie aussi HSTS, pour que les navigateurs cessent d'envoyer même la
première requête en clair - celle qu'une redirection ne peut pas protéger. Un jour
par défaut, jusqu'à deux ans. Jamais envoyé à localhost ni à une adresse IP ; une
route ou un service qui pose sa propre valeur la garde. Voir
[TLS](/docs/operations/tls).

## Autorités ACME

> [!NOTE] Enterprise
> ACME fait partie de l'édition Enterprise (SSL-05). Sur l'image communautaire le
> bouton est verrouillé ; les certificats se génèrent, s'importent ou se signent sur
> demande. Une configuration importée y laisse sa partie ACME de côté, et son plan
> le dit.

Le bouton **ACME**, à côté de Add certificate, ouvre un tiroir : un formulaire en haut, les
autorités déjà configurées en dessous - chacune avec ce qui lui est demandé. Il y en a
une poignée, jamais cinquante, et chacune enregistrée devient une entrée **Ask** du
menu Add certificate.

Choisissez un **fournisseur** : le formulaire n'affiche que ce qu'il utilise.

| Fournisseur | Ce qu'il demande |
|---|---|
| Let's Encrypt (staging) | les conditions, rien d'autre. Ses certificats ne sont pas reconnus par les navigateurs, mais il ne compte pas dans les quotas hebdomadaires : l'endroit pour vérifier qu'un nom atteint la passerelle |
| Let's Encrypt | les conditions, rien d'autre. Cinquante certificats par semaine et par domaine enregistré |
| ZeroSSL | un identifiant et une clé HMAC de compte - tableau de bord ZeroSSL, Developer, *EAB Credentials for ACME Clients* |
| Google Trust Services | un identifiant et une clé HMAC de compte - `gcloud publicca external-account-keys create` |
| Another authority | l'URL de son répertoire, son certificat racine quand la passerelle ne le connaît pas (`step ca root` pour step-ca), une liaison de compte et un contact si elle les exige - chaque valeur venant de celui qui l'exploite |

L'enregistrement interroge le répertoire de l'autorité : une URL fausse ou une
autorité injoignable est signalée aussitôt. Aucun compte n'est encore ouvert - cela
attend le premier certificat demandé. La clé HMAC est un secret et passe par le
[coffre](/docs/console/vault). Une autorité à qui un certificat est encore demandé ne
peut pas être supprimée ; le refus nomme ce qui la sollicite.

### Ce qui se passe quand on demande

Rien ne part quand une autorité est enregistrée, ni quand un certificat lui est
demandé et gardé en réserve. **Le placer sur une porte** envoie la demande, aussitôt,
en arrière-plan :

1. la première fois, un compte est ouvert chez l'autorité (ses conditions acceptées) ;
2. le certificat est commandé, et l'autorité demande une preuve pour chaque nom ;
3. elle se connecte au nom **sur le port 443**, où la passerelle répond à son défi
   (TLS-ALPN : le port 80 reste fermé) ;
4. elle signe, le certificat est rangé et servi aussitôt, puis renouvelé un mois avant
   sa fin - sans que personne ne touche à rien.

La ligne dit *Asking the authority...* pendant ce temps - quelques secondes, l'écran se
rafraîchit seul - puis affiche l'échéance du certificat, ou **le refus de l'autorité**
avec un bouton **Retry**. Les refus courants sont expliqués : le nom ne pointe pas ici
(ou il est proxifié - chez Cloudflare il doit être en *DNS only*), le port 443
n'atteint pas la passerelle, un enregistrement CAA réserve le domaine à une autre
autorité, le quota est atteint.

![Le tiroir des autorités ACME : le fournisseur et ses champs, les autorités déjà configurées avec ce qui leur est demandé](img/console/tls-authorities.webp)

L'expiration ne repose pas sur les e-mails de l'autorité - Let's Encrypt a cessé de les
envoyer en 2025 : le [digest quotidien](/docs/console/mail-relay) cite un certificat
automatique qui entre dans sa fenêtre, signe que son renouvellement échoue.

## Pièges

- **Un certificat répond pour les noms qu'il porte.** Un navigateur qui demande un
  nom qu'aucun certificat de cette porte ne porte reçoit le plus ancien, et le
  refuse.
- **Une autorité publique ne voit pas votre DNS privé.** Utilisez une autorité
  interne pour les noms internes, ou importez.
- **N'oubliez pas les intermédiaires** à l'import : la console vous le dira, mais
  après coup.
- **La clé HMAC EAB est un secret** et passe par le [coffre](/docs/console/vault)
  comme tous les autres.
