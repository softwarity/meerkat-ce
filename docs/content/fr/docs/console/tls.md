---
title: TLS
section: La console
order: 158
summary: Un nom, un certificat - importer, générer, ou laisser une autorité les émettre.
---

# TLS

**Infra > TLS.** Les noms auxquels cette passerelle répond, et le certificat que
chacun porte. C'est dans le plan Infra pour la même raison que le relais : c'est une
propriété de l'installation, pas de l'application qu'elle sert.

Il n'y a pas d'interrupteur *activer HTTPS*. **Avoir un certificat est ce qui ouvre la
porte**, et le retirer est ce qui la ferme. Un interrupteur qui peut être allumé sans
rien derrière est un interrupteur qui ment.

![L'écran TLS : le nom et le certificat de la console, deux noms applicatifs, l'interrupteur Force HTTPS et la carte ACME](img/console/tls.webp)

Ici la console répond sur `localhost` avec un certificat auto-signé valable
jusqu'en 2027, l'application sert deux noms - l'un encore en clair, l'autre avec
son propre certificat - et Force HTTPS comme ACME sont éteints.

## Les deux sections

- **Console** - un nom et un certificat. Le nom est un champ, pas une ligne : cette
  console répond à une seule adresse.
- **Application** - une entrée par hôte que la passerelle sert, chacune avec son
  certificat. **Add a name**, puis donnez-lui un certificat.

Chaque entrée se lit comme une ligne : l'hôte, la provenance du certificat
(*Imported*, *Self-signed*, *Signed on request*), ce qu'il dit de lui-même, et les
liens pour ouvrir ce nom en HTTP simple et en HTTPS.

La phrase en bout de ligne répond *est-ce que ça va marcher*, jamais *voici un code
de statut* :

| Ce qu'elle dit | Ce que ça veut dire |
|---|---|
| Valid until *date* | Rien à faire |
| Expires in *N* days | Sous trente jours, coloré en conséquence |
| Expired on *date* | Les navigateurs le refusent déjà |
| Awaiting signature | Une demande de signature a été créée et n'a pas eu de réponse |
| This certificate does not answer for *host* | Bon fichier, mauvais nom |
| No certificate: this name is not served over HTTPS | HTTP simple seulement |

Deux autres avertissements apparaissent là où ils comptent : une pastille
**self-signed** (personne ne se porte garant, sauf lui-même), et **No intermediate** -
une chaîne qui marche dans `curl` et échoue dans un navigateur qui n'a jamais
rencontré l'émetteur.

## Poser un certificat sur un nom

Le menu **Add certificate**, et le menu **Actions** d'une entrée qui en a déjà un,
offrent les trois mêmes portes :

1. **Import a PEM or a keystore** - la paire qu'une autorité renvoie par courriel, ou
   un `.p12` / `.pfx` avec son mot de passe.
2. **Generate a self-signed one** - organisation, type de clé (ECDSA P-256 ou P-384,
   RSA 2048 ou 4096) et validité en jours. Bon pour un labo, et les navigateurs
   avertiront.
3. **Create a signing request** - le CSR à envoyer à votre autorité. L'entrée dit
   alors *Awaiting signature* ; quand la réponse revient, **Adopt** la colle.

Remplacer ne laisse jamais un hôte nu : le nouveau certificat est créé d'abord et
l'ancien lâché seulement quand le nouveau est en place, donc un import raté laisse le
nom encore servi. Le dernier élément du menu Actions est l'inverse assumé : le retirer
et servir ce nom en clair.

## Force HTTPS

Les appelants qui arrivent sur le port en clair sont renvoyés vers HTTPS. La console
n'est jamais forcée, ni la sonde de vivacité. Si tous les certificats expirent, la
redirection **se met elle-même en retrait** et le dit, plutôt que d'envoyer les
appelants vers une porte qu'aucun n'ouvrira.

## Certificats automatiques (ACME)

Un compte, et une case à cocher par nom : ce qui varie n'est jamais le plan, c'est le
nom.

- **Authority** - une URL plutôt qu'une liste de fournisseurs, pour qu'une autorité
  publique et un `step-ca` interne soient également chez eux.
- **Contact email** - là où l'autorité prévient des expirations.
- **Which names the authority may issue** - les cases. Un nom qui n'existe que sur
  cette machine est signalé : une autorité publique doit joindre le nom pour le
  prouver, donc elle ne peut jamais émettre pour un nom qui ne résout pas sur
  l'internet.
- **Private authority** - le certificat racine en PEM quand le serveur ACME est
  derrière un certificat privé, plus la paire de liaison de compte externe
  (identifiant de clé et clé HMAC) que la plupart des autorités privées exigent.
- **I accept the authority's terms of service** - exigé par le protocole.
- **Already issued** liste ce qui est revenu, avec son expiration.

La preuve passe par le port HTTPS que cette passerelle tient déjà : **le port 80 reste
fermé**. Le renouvellement a lieu des semaines avant l'expiration, sans que personne
touche un fichier.

## Pièges

- **Un certificat appartient à un nom.** Importer le bon fichier sous le mauvais hôte
  donne *does not answer for this host*.
- **Une autorité publique ne voit pas votre DNS privé.** Utilisez une autorité interne
  pour les noms internes, ou importez.
- **N'oubliez pas les intermédiaires** à l'import : la console vous le dira, mais
  seulement après coup.
- **La clé HMAC de liaison est un secret** et passe par le
  [coffre](/docs/console/vault) comme tous les autres.
