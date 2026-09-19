---
title: Second facteur
section: Authentification
order: 110
summary: Les codes d'une application d'authentification, un code par courriel en secours, les navigateurs de confiance, et qui doit s'en servir.
---

# Second facteur

Le second facteur propre à Meerkat, c'est le **TOTP** : le code à six chiffres
qu'affiche une application d'authentification. Deux pièces l'entourent - un code
par courriel pour le jour où l'application est hors d'atteinte, et les navigateurs
de confiance pour ne pas réclamer le défi à chaque connexion.

Tout cela est dans les deux images.

## Les codes d'une application d'authentification

Du TOTP standard, RFC 6238 : `6` chiffres, un nouveau code toutes les `30`
secondes, et le code précédent comme le suivant sont acceptés, de sorte qu'un
téléphone légèrement décalé fonctionne quand même. Le secret fait `160` bits.

**L'enrôlement** est en self-service, sur `/profile/mfa`. La page affiche un QR
code - dessiné par la gateway elle-même, sans rien aller chercher nulle part - et
le même secret en texte pour une clé tapée à la main. Rien n'est activé avant que
la personne ait prouvé que ça marche avec un code vivant.

Le QR code nomme votre application, parce que l'issuer qu'il porte est le nom
d'application défini dans **Application > Built-in pages > Branding**. Changez ce
nom et les enrôlements existants gardent l'ancien libellé : l'issuer est figé dans
ce que l'application a déjà enregistré.

**Les codes de secours.** L'enrôlement se termine sur `10` codes à usage unique,
affichés une fois et jamais plus. Ils s'écrivent `k7m2p-3xrqh`, dans un alphabet
sans caractères confondables. Seules leurs empreintes sont conservées : une liste
perdue ne se retrouve pas, elle se remplace, depuis la même page, ce qui en émet
dix nouveaux et retire les anciens. La page dit combien il en reste.

Se réenrôler, ou éteindre le second facteur, **oublie tous les navigateurs de
confiance** du compte. C'est délibéré : l'ancien défi a disparu, et avec lui tout
ce qui avait le droit de le sauter.

> [!WARNING]
> Les secrets TOTP sont stockés **en clair** dans la base. Le chiffrement au repos
> couvre les secrets du coffre, les clés privées TLS et la clé de branchement des
> développeurs - pas ceux-là. Traitez un export de base en conséquence.

## Un code par courriel, en secours

Livré **éteint**. C'est une porte de secours pour quelqu'un qui est déjà enrôlé en
TOTP et n'atteint pas son application d'authentification - pas un second facteur à
part entière. Personne ne s'y enrôle, et un compte sans TOTP ne se le voit jamais
proposer.

Le lien n'apparaît sur la page de défi que si tout ceci tient :

- l'interrupteur *Allow a one-time code by e-mail* est allumé, dans **Application > Security** ;
- un relais de messagerie est configuré ;
- le compte porte une adresse ;
- le compte est enrôlé en TOTP, ce qui est précisément ce qui l'a mené sur cette page.

Le code fait `6` chiffres, vaut `10` minutes, est à usage unique et lié au compte -
il se tape dans le même champ que le code TOTP. Le redemander dans les `45`
secondes affiche la même page "c'est envoyé" sans envoyer un second courriel.
Une connexion réussie efface le code encore en attente.

Interrupteur éteint, le lien est absent et le point d'entrée derrière répond
`404`.

## Navigateurs de confiance

Livré **éteint**, avec une confiance de `7` jours quand vous l'allumez (la console
propose un jour, sept, quatorze ou trente, dans **Application > Security**).

Après un défi réussi, la personne peut cocher *trust this browser*. La connexion
suivante depuis ce navigateur saute le code.

> [!WARNING]
> La confiance est un **jeton aléatoire dans un cookie**, pas une empreinte de la
> machine. Rien du navigateur, de l'adresse ni de l'appareil n'est comparé au
> retour. Le nom que vous voyez dans le profil - `Chrome - macOS` - est une
> étiquette lue dans l'en-tête du navigateur pour votre confort ; elle n'est jamais
> vérifiée. Qui détient le cookie saute le défi : c'est pourquoi la durée gagne à
> rester courte.

Seule l'empreinte du jeton est stockée. Le profil liste les navigateurs de
confiance, marque celui sur lequel on est, et les révoque un par un ou tous d'un
coup. Éteindre le réglage rétablit le défi pour tout le monde immédiatement : la
politique est relue à chaque connexion.

## Qui doit utiliser un second facteur

Deux niveaux, et c'est tout ce qui existe :

| Niveau | Où | Valeurs |
|---|---|---|
| L'installation | **Application > Security**, *Require two-factor for everyone* | allumé ou éteint, livré **éteint** |
| Un compte | **Application > Users**, dans l'éditeur du compte | *Inherited*, *Required*, *Optional* |

Le réglage du compte gagne ; sinon celui de l'installation répond. *Optional* sur
un compte l'exempte même quand l'installation l'exige.

Quelqu'un pour qui le second facteur est **exigé** et qui n'en a pas est envoyé à
l'enrôlement à sa prochaine connexion, avant d'atteindre quoi que ce soit, et ne
peut plus l'éteindre ensuite : le bouton self-service répond `403` en disant
pourquoi.

> [!NOTE]
> Il n'y a délibérément **pas de niveau par organisation** : le second facteur est
> demandé avant que l'organisation soit connue, donc une règle par organisation ne
> pourrait pas être lue à temps. Il n'y a **pas de niveau par rôle** non plus, pour
> la même raison ; c'est la pièce que FEATURES.md liste encore comme manquante sur
> cette fonctionnalité.

Une autorité peut **dispenser** du défi les gens entrés par elle - *Two-factor:
left to the authority* sur une autorité OIDC, annuaire ou GitHub. Voir [OpenID
Connect](/#/docs/auth/oidc) pour ce que ce réglage fait, et ne fait pas.

## Le trou à prévoir

**Un administrateur ne peut ni réinitialiser, ni effacer, ni éteindre le second
facteur de quelqu'un.** Il n'y a ni point d'entrée ni écran pour cela :
l'enrôlement appartient au compte, et l'API d'administration ne touche jamais ces
colonnes.

Donc une personne qui perd son application d'authentification **et** ses codes de
secours, sur une installation où le secours par courriel est éteint, n'a aucun
chemin de retour vers ce compte. Les chemins de retour sont, dans cet ordre : un
code de secours, le code par courriel, ou - si le second facteur ne lui est pas
exigé - l'éteindre elle-même. À défaut des trois, il reste à supprimer le compte
et à en refaire un.

Allumer le secours par courriel est l'assurance la moins chère contre cet appel.
