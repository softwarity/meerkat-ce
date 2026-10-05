---
title: Passkeys
section: Authentification
order: 112
summary: La connexion WebAuthn avec une clé de sécurité, une empreinte digitale ou Windows Hello - ce qui fonctionne aujourd'hui, et ce qui n'existe pas encore.
---

# Passkeys

Une passkey est un moyen d'authentification WebAuthn détenu par le navigateur, le téléphone
ou une clé de sécurité : Touch ID, Windows Hello, une YubiKey. Se connecter avec une
passkey ne demande ni identifiant ni mot de passe.

La fonctionnalité est **partielle**. Ce qui suit indique précisément quelle moitié est
construite ; lisez les manques avant de vous appuyer dessus.

## Ce qui fonctionne aujourd'hui

Les passkeys sont autorisées par défaut, pour toute la gateway : **Application >
Security**, *Allow passkeys*. Il n'y a pas de réglage par organisation, car une connexion
par passkey a lieu avant que l'organisation soit connue.

**L'enregistrement** se fait en libre-service, depuis `/profile/passkeys`, et suppose une
session complète : vous vous connectez d'abord de la façon habituelle, puis vous ajoutez
une clé. Les clés sont listées avec un libellé tiré du navigateur (`Chrome - macOS`), leur
date de création et leur date de dernière utilisation. Leur titulaire peut supprimer
n'importe laquelle, y compris après qu'un administrateur a désactivé les passkeys.

**La connexion** passe par un bouton de la page de connexion, affiché quand le navigateur
prend en charge WebAuthn et qu'au moins une autorité est activée. Elle se fait *sans
identifiant* : Meerkat demande une clé découvrable (discoverable credential), et c'est la
clé qui désigne le compte. Les clés découvrables sont exigées à l'enregistrement, ce qui
rend la chose possible.

La clé vous amène directement dans votre organisation, ou sur la page de choix si vous
appartenez à plusieurs - exactement comme le ferait une connexion par mot de passe.

## Une passkey vaut pour les deux facteurs

C'est un choix de conception, et c'est ce qu'il faut avoir compris avant d'activer la
fonctionnalité :

> [!WARNING]
> Une connexion par passkey **ne demande jamais de second facteur**, même pour un compte
> où il est *exigé*. Elle passe directement au choix de l'organisation. Si votre politique
> est "tout le monde doit avoir deux facteurs", une passkey y répond par hypothèse et non
> par vérification - voir ci-dessous.

Pourquoi cette hypothèse n'est pas sans faille dans la version actuelle :

- **La vérification de l'utilisateur n'est pas exigée.** Meerkat ne demande pas à l'authentificateur de prouver un code PIN, une empreinte digitale ou un visage. Une clé qui prouve seulement que quelqu'un l'a touchée est acceptée. Sur la plupart des téléphones et des ordinateurs portables, la plateforme vérifie de toute façon la personne, parce qu'elle est conçue ainsi - mais une simple clé de sécurité laissée sur un portable suffit à elle seule.
- **L'attestation n'est pas contrôlée.** Il n'y a ni ancre de confiance, ni service de métadonnées FIDO, ni liste de modèles d'authentificateurs autorisés. Tout authentificateur que propose le navigateur est accepté.

## Le nom d'hôte compte

La partie de confiance (relying party) est **déduite de la requête** : l'hôte qui a servi la
page, sans le port, et l'origine sur laquelle elle a été servie. Aucun réglage n'intervient.

Deux conséquences :

- une passkey enregistrée en atteignant la gateway sur `apps.acme.io` ne fonctionne pas sur un autre nom d'hôte : mettez donc le nom public en place avant que quiconque ne s'enrôle ;
- la console d'administration est un autre hôte, donc une autre partie de confiance. Un exploitant qui veut une passkey sur la console l'enregistre **là**, depuis la page de profil de la console. La console ignore aussi le réglage *Allow passkeys*, et c'est voulu : le choix de l'intégrateur pour l'application ne doit pas priver un exploitant de sa propre clé.

## Ce qu'ajoute un annuaire

Quand le compte est lié à une autorité de type **annuaire**, une connexion par passkey
demande à cet annuaire s'il connaît toujours la personne avant de la laisser entrer. Une
réponse claire "cette entrée n'existe pas" annule la connexion ; un serveur injoignable ne
déconnecte personne.

Les fournisseurs d'identité atteints par redirection - OIDC, GitHub - n'offrent aucun moyen
de poser cette question : une connexion par passkey à un compte qui en dépend n'est donc
jamais revalidée auprès du fournisseur. Par ailleurs, toute autorité liée et activée peut
interdire purement et simplement les passkeys aux comptes qu'elle connaît (politique
`passkeys` de l'autorité réglée sur *no*), ce qui bloque à la fois l'enregistrement et
l'utilisation.

## Ce qui n'existe pas encore

- **Pas de récupération.** Perdre son unique passkey n'est pas un problème que les passkeys ont à résoudre : vous vous connectez avec votre mot de passe, ou par le lien de mot de passe oublié, et vous en enregistrez une autre. Ici, une passkey **complète** le mot de passe, elle ne le remplace pas - un compte local en garde toujours un.
- **Pas de vue pour l'administrateur.** Il n'existe ni écran ni endpoint pour lister, nommer ou révoquer les passkeys de quelqu'un d'autre. Elles appartiennent au compte. L'administrateur ne dispose que de la manière forte : supprimer le compte emporte ses clés.
- **Pas de politique par clé** - aucun moyen d'exiger la vérification de l'utilisateur, ou un type d'authentificateur particulier.
