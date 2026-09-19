---
title: Passkeys
section: Authentification
order: 112
summary: Connexion WebAuthn par clé de sécurité, empreinte ou Windows Hello - ce qui marche aujourd'hui, et ce qui n'est pas là.
---

# Passkeys

Une passkey est une clé WebAuthn détenue par le navigateur, le téléphone ou une
clé de sécurité : Touch ID, Windows Hello, une YubiKey. S'y connecter ne demande
ni identifiant ni mot de passe.

La fonctionnalité est **partielle**. Ce qui suit dit exactement quelle moitié est
construite, et les manques sont à lire avant de s'y appuyer.

## Ce qui marche aujourd'hui

Les passkeys sont permises par défaut, pour toute la gateway : **Application >
Security**, *Allow passkeys*. Il n'y a pas de réglage par organisation - une
connexion par passkey a lieu avant que l'organisation soit connue.

**En enregistrer une** est en self-service, depuis `/profile/passkeys`, et cela
demande une session terminée : on se connecte d'abord normalement, puis on ajoute
une clé. Les clés sont listées avec une étiquette lue dans l'en-tête du navigateur
(`Chrome - macOS`), leur date de création et leur dernière utilisation. Le
propriétaire peut en supprimer n'importe laquelle, y compris après qu'un
administrateur a éteint les passkeys.

**S'y connecter** est un bouton sur la page de connexion, dessiné quand le
navigateur sait faire du WebAuthn et qu'au moins une autorité est active. C'est
*sans identifiant* : Meerkat demande une clé découvrable, et la clé dit de quel
compte il s'agit. Les clés découvrables sont exigées à l'enregistrement, et c'est
ce qui rend cela possible.

La clé mène directement à votre organisation, ou au sélecteur si vous appartenez à
plusieurs - exactement comme une connexion par mot de passe.

## Une passkey vaut les deux facteurs à la fois

C'est le parti pris, et c'est la chose à comprendre avant de l'allumer :

> [!WARNING]
> Une connexion par passkey **ne demande jamais de second facteur**, même pour un
> compte où le second facteur est *exigé*. Elle va droit à la résolution de
> l'organisation. Si votre politique est "tout le monde a deux facteurs", une
> passkey la satisfait par hypothèse plutôt que par vérification - voir ci-dessous.

Pourquoi l'hypothèse n'est pas étanche dans la version actuelle :

- **La vérification de l'utilisateur n'est pas exigée.** Meerkat ne demande pas à l'authentificateur de prouver un code, une empreinte ou un visage. Une clé qui prouve seulement qu'on l'a touchée est acceptée. Sur la plupart des téléphones et des portables, la plateforme vérifie la personne de toute façon, parce qu'elle est faite comme ça - mais une clé de sécurité restée dans le portable suffit à elle seule.
- **L'attestation n'est pas vérifiée.** Il n'y a pas d'ancre de confiance, pas de service de métadonnées FIDO, pas de liste de modèles d'authentificateurs permis. Tout authentificateur que le navigateur propose est accepté.

## Le nom d'hôte compte

La partie de confiance est **dérivée de la requête** : l'hôte qui a servi la page,
port retiré, et l'origine sur laquelle elle a été servie. Rien ne se configure.

Deux conséquences :

- une passkey enregistrée en atteignant la gateway sur `apps.acme.io` ne marche pas sur un autre nom d'hôte : mettez le nom public devant avant que qui que ce soit s'enrôle ;
- la console d'administration est un autre hôte, donc une autre partie de confiance. Un opérateur qui veut une passkey sur la console en enregistre une **là**, depuis la page de profil de la console. La console ignore aussi le réglage *Allow passkeys*, exprès : le choix de l'intégrateur pour l'application ne doit pas enfermer un opérateur dehors de sa propre clé.

## Ce qu'un annuaire ajoute

Quand le compte est lié à une autorité **annuaire**, une connexion par passkey
demande à cet annuaire s'il connaît toujours la personne avant de la laisser
entrer. Un "aucune entrée de ce nom" franc révoque la connexion ; un serveur
injoignable ne déconnecte personne.

Les fournisseurs d'identité atteints par redirection - OIDC, GitHub - ne peuvent
pas se voir poser cette question : une connexion par passkey pour un compte qu'ils
connaissent n'est donc jamais revalidée auprès d'eux. Et toute autorité liée et
active peut interdire les passkeys pour les comptes qu'elle connaît (sa politique
`passkeys` posée sur *no*), ce qui bloque à la fois l'enregistrement et l'usage.

## Ce qui n'est pas là

- **Aucune récupération.** Perdre son unique passkey n'est pas un problème de passkey à résoudre : on se connecte avec son mot de passe, ou avec le lien de mot de passe oublié, et on enregistre une autre clé. Une passkey **s'ajoute** au mot de passe ici, elle ne le remplace pas - un compte local en garde toujours un.
- **Aucune vue administrateur.** Il n'y a ni écran ni point d'entrée pour lister, nommer ou révoquer les passkeys de quelqu'un d'autre. Elles appartiennent au compte. Ce qu'un administrateur possède est l'instrument brutal : supprimer le compte emporte ses clés.
- **Aucune politique par clé** - aucun moyen d'exiger la vérification de l'utilisateur, ou un genre particulier d'authentificateur.
