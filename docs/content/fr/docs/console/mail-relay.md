---
title: Relais de messagerie
section: La console
order: 160
summary: Le serveur SMTP qui achemine les e-mails liés aux comptes, et le seul message que Meerkat envoie de sa propre initiative.
---

# Relais de messagerie

**Infra > Mail relay.** Le serveur SMTP qui achemine les confirmations, les
réinitialisations de mot de passe et les notifications aux administrateurs. Il relève
de l'infrastructure parce que c'est un service tiers, joint par un hôte et un port,
avec des identifiants - exactement comme l'upstream d'une route.

Plusieurs fonctionnalités ne sont que des interrupteurs sans effet tant que ce relais
n'est pas configuré : l'auto-inscription avec confirmation de l'adresse, les liens de
réinitialisation du mot de passe, le code à usage unique envoyé par e-mail et le
récapitulatif quotidien.

## Le relais

- **Host**, **Port**, **Security** - `STARTTLS`, `TLS` ou aucune.
- **Sender address** - elle se règle ici parce qu'un fournisseur n'accepte que le
  compte qu'il a authentifié. Vide, le champ signifie *le compte*, à condition que le
  compte soit lui-même une adresse. Sous le champ, la console montre ce que verront
  réellement les destinataires, nom et adresse réunis.

Viennent ensuite les identifiants, sous la forme de deux formulaires distincts plutôt
que d'un seul jeu de champs génériques :

| Mode | Quand |
|---|---|
| **Password** | Un compte et un secret. C'est ce qu'utilisent tous les relais transactionnels, et ce qu'accepte Gmail avec un mot de passe d'application |
| **OAuth2** | Un jeton obtenu par client credentials remplace le mot de passe (XOAUTH2). C'est la seule façon d'accéder à Microsoft 365, qui n'accepte plus de mot de passe en SMTP |

OAuth2 demande l'**URL du jeton** (l'endpoint de jeton de votre tenant), le **client
ID**, le **client secret** et un **scope** - laissé vide, il prend le scope SMTP de
Microsoft.

## Send a test

Choisissez le message, la langue et le destinataire (votre propre adresse par défaut) :
un véritable message passe par le relais, avec des valeurs fictives mais l'apparence
réelle. C'est le moyen le plus rapide de découvrir qu'un port est fermé ou qu'un
expéditeur est refusé.

## Daily digest

Une fois par jour, les administrateurs qui ont une adresse e-mail sont informés des
comptes qui sont sur le point de perdre leur accès, de ceux qui viennent de le perdre,
des entrées du coffre qui approchent de leur date de rappel - et, en tête, des
**certificats** qui vont expirer ou ont expiré. Un certificat émis automatiquement
(ACME) est renouvelé des semaines avant son échéance : s'il apparaît ici, c'est que son
renouvellement échoue, et la ligne le précise.

- **Hour** - exprimée dans l'heure de la gateway, que le champ rappelle en dessous (heure et fuseau).
- **Look ahead** - le nombre de jours à venir à couvrir, de un à quatre-vingt-dix.

Un jour où il n'y a rien à signaler, rien n'est envoyé. Sans relais configuré, la
notification reste due et part le matin où un relais répond.

## Pièges

- **Un secret saisi au clavier bloque Save.** Le mot de passe et le secret client
  doivent d'abord être rangés dans le [coffre](/docs/console/vault) ; c'est la seule
  façon de les stocker. Le bouton reste désactivé tant que ce n'est pas fait.
- **Le nom de l'expéditeur ne se saisit nulle part.** Le nom affiché devant l'adresse
  est celui de l'application, défini dans
  [Built-in pages, Branding](/docs/console/built-in-pages) et résolu au moment de
  l'envoi. Seule l'adresse se règle ici.
- **Testez avant d'annoncer.** C'est le relais qui juge une adresse, pas la console.
