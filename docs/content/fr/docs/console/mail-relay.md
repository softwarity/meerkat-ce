---
title: Relais mail
section: La console
order: 160
summary: Le serveur SMTP qui délivre les courriels de compte, et le seul message que Meerkat envoie de lui-même.
---

# Relais mail

**Infra > Mail relay.** Le serveur SMTP qui porte les confirmations, les
réinitialisations de mot de passe et les avis aux administrateurs. C'est de l'infra
parce que c'est un service tiers joint par hôte et port, avec des identifiants - la
même nature que l'amont d'une route.

Plusieurs fonctions sont des interrupteurs sans rien derrière tant que ceci n'est pas
réglé : l'inscription avec adresse confirmée, les liens de réinitialisation, le code à
usage unique par courriel, et le digest quotidien.

## Le relais

- **Host**, **Port**, **Security** - `STARTTLS`, `TLS` ou aucune.
- **Sender address** - elle vit ici parce qu'un fournisseur n'accepte que le compte
  qu'il a authentifié. Vide veut dire *le compte*, tant que ce compte est lui-même
  une adresse. Sous le champ, la console montre ce que les destinataires verront
  vraiment, nom et adresse réunis.

Puis les identifiants, en deux formulaires distincts plutôt qu'un jeu de cases
générique :

| Mode | Quand |
|---|---|
| **Password** | Un compte et un secret. Ce qu'utilise tout relais transactionnel, et ce que Gmail accepte avec un mot de passe d'application |
| **OAuth2** | Un jeton obtenu par client credentials tient la place du mot de passe (XOAUTH2). La seule entrée vers Microsoft 365, qui n'accepte plus de mot de passe en SMTP |

OAuth2 demande l'**URL de jeton** (le point d'entrée de votre tenant), le **client
ID**, le **client secret**, et une **scope** - laissée vide, elle utilise la scope SMTP
de Microsoft.

## Send a test

Choisissez quel message, quelle langue, et le destinataire (votre adresse par défaut),
et un vrai message part par le relais, avec des valeurs factices mais l'allure réelle.
C'est le moyen le plus rapide d'apprendre qu'un port est fermé ou un expéditeur refusé.

## Daily digest

Une fois par jour, les administrateurs qui portent une adresse apprennent quels
comptes vont perdre l'accès, lesquels viennent de le perdre, et quelles entrées du
coffre approchent de leur date de rappel.

- **Hour** - lue sur l'horloge de la passerelle, que le champ imprime en dessous
  (heure et zone).
- **Look ahead** - combien de jours en avant regarder, de un à quatre-vingt-dix.

Un jour sans rien à dire n'envoie rien. Sans relais configuré, l'avis reste dû et part
le matin où un relais répond.

## Pièges

- **Un secret tapé bloque Save.** Le mot de passe et le secret client doivent d'abord
  être rangés dans le [coffre](/docs/console/vault) ; c'est la seule façon dont ils
  sont stockés. Le bouton reste désactivé tant que ce n'est pas fait.
- **Le nom de l'expéditeur ne se tape nulle part.** Le nom affiché devant
  l'adresse est le nom de l'application, pris dans
  [Built-in pages, Branding](/docs/console/built-in-pages), et résolu au moment
  de l'envoi. Ici, il n'y a que l'adresse.
- **Testez avant d'annoncer.** C'est le relais qui juge une adresse, pas la console.
