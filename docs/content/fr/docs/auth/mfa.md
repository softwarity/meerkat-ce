---
title: Second facteur
section: Authentification
order: 110
summary: Les codes d'une application d'authentification, un code par e-mail en solution de secours, les navigateurs de confiance, et qui est tenu de s'en servir.
---

# Second facteur

Le second facteur propre à Meerkat est **TOTP** : le code à six chiffres qu'affiche une
application d'authentification. Deux compléments l'entourent : un code par e-mail pour le
jour où l'application d'authentification n'est pas à portée de main, et les navigateurs de
confiance pour que le code ne soit pas demandé à chaque connexion.

L'ensemble est présent dans les deux images.

## Les codes d'une application d'authentification

Du TOTP standard, RFC 6238 : `6` chiffres, un nouveau code toutes les `30` secondes, et le
code précédent comme le code suivant sont eux aussi acceptés, si bien qu'un téléphone dont
l'horloge est légèrement décalée fonctionne quand même. Le secret fait `160` bits.

**L'enrôlement** se fait en libre-service, sur `/profile/mfa`. La page affiche un QR code -
dessiné par la gateway elle-même, sans rien aller chercher ailleurs - ainsi que le même
secret sous forme de texte, pour saisir la clé à la main. Rien n'est activé tant que vous
n'avez pas prouvé, avec un code valide, que cela fonctionne.

Le QR code porte le nom de votre application : l'émetteur qu'il contient est le nom de
l'application défini dans **Application > Built-in pages > Branding**. Si vous changez ce
nom, les enrôlements existants gardent l'ancien libellé : l'émetteur fait partie de ce que
l'application d'authentification a déjà enregistré.

**Les codes de secours.** L'enrôlement se termine par `10` codes à usage unique, affichés
une seule fois. Ils ont la forme `k7m2p-3xrqh` et sont tirés d'un alphabet sans caractères
faciles à confondre. Seules leurs empreintes sont conservées : une feuille perdue ne peut
donc pas être retrouvée, seulement remplacée, depuis la même page, qui génère dix nouveaux
codes et retire les anciens. La page indique combien il en reste.

Refaire l'enrôlement, ou désactiver le second facteur, **efface tous les navigateurs de
confiance** du compte. C'est voulu : l'ancienne vérification n'existe plus, et tout ce qui
permettait de s'en dispenser disparaît avec elle.

> [!NOTE]
> Les secrets TOTP sont **scellés** dans la base de données avec la clé maîtresse du
> coffre, comme les secrets du coffre et les clés privées TLS : une copie de la base ne
> contient aucun secret à partir duquel quelqu'un pourrait calculer des codes. Voir
> [la clé maîtresse](/docs/operations/vault#la-cl-matresse).

## Un code par e-mail, en solution de secours

**Désactivé** par défaut. C'est un moyen de revenir pour une personne déjà enrôlée en TOTP
qui n'a pas accès à son application d'authentification - pas un second facteur à part
entière. Personne ne s'y enrôle, et il n'est jamais proposé à un compte sans TOTP.

Le lien n'apparaît sur la page de vérification que si toutes ces conditions sont réunies :

- l'interrupteur *Allow a one-time code by e-mail* est activé, dans **Application > Security** ;
- un relais de messagerie est configuré ;
- le compte a une adresse ;
- le compte est enrôlé en TOTP, ce qui l'a amené sur cette page.

Le code compte `6` chiffres, il est valable `10` minutes, à usage unique et lié au compte.
Il se saisit dans le même champ que le code TOTP. En redemander un dans les `45` secondes
affiche la même page "nous vous l'avons envoyé" sans envoyer de second e-mail. Une
connexion réussie supprime tout code encore en attente.

Quand l'interrupteur est désactivé, le lien est absent et l'endpoint qui se trouve derrière
répond `404`.

## Les navigateurs de confiance

**Désactivés** par défaut. Quand vous les activez, la confiance dure `7` jours (la console
propose un, sept, quatorze ou trente jours, dans **Application > Security**).

Après une vérification réussie, la personne peut cocher *faire confiance à ce navigateur*.
À la connexion suivante depuis ce navigateur, le code n'est plus demandé.

> [!WARNING]
> La confiance repose sur un **jeton aléatoire placé dans un cookie**, pas sur une
> empreinte de la machine. Au retour, rien n'est comparé : ni le navigateur, ni l'adresse,
> ni l'appareil. Le nom affiché dans le profil - `Chrome - macOS` - est un libellé tiré du
> user agent pour vous aider à vous y retrouver ; il n'est jamais vérifié. Quiconque
> détient le cookie échappe à la vérification, d'où l'intérêt de garder une durée courte.

Seule l'empreinte du jeton est enregistrée. Le profil liste les navigateurs de confiance,
signale celui que vous utilisez et permet de les révoquer un par un ou tous à la fois.
Désactiver le réglage rétablit immédiatement la vérification pour tout le monde : la
politique est relue à chaque connexion.

## Qui est tenu d'utiliser un second facteur

Deux niveaux, et rien de plus :

| Niveau | Où | Valeurs |
|---|---|---|
| L'installation | **Application > Security**, *Require two-factor for everyone* | activé ou désactivé, **désactivé** par défaut |
| Un compte | **Application > Users**, dans l'éditeur du compte | *Inherited*, *Required*, *Optional* |

Le réglage du compte l'emporte ; à défaut, c'est celui de l'installation qui s'applique.
*Optional* sur un compte l'en dispense, même quand l'installation l'exige.

Une personne pour qui le second facteur est **exigé** et qui n'en a pas est dirigée vers
l'enrôlement à sa prochaine connexion, avant d'atteindre quoi que ce soit. Elle ne peut pas
le désactiver ensuite : la désactivation en libre-service répond `403` et explique
pourquoi.

> [!NOTE]
> Il n'existe volontairement **aucun niveau par organisation** : le second facteur est
> demandé avant que l'organisation soit connue, et une règle par organisation ne pourrait
> donc pas être lue à temps. Il n'existe **pas non plus de niveau par rôle**, pour la même
> raison ; c'est l'élément que FEATURES.md signale encore comme manquant sur cette
> fonctionnalité.

Une autorité peut **dispenser** de cette vérification les personnes qui arrivent par elle :
c'est *Two-factor: Left to the authority* sur une autorité OIDC, annuaire ou GitHub. Voir
[OpenID Connect](/docs/auth/oidc) pour ce que fait ce réglage, et ce qu'il ne fait pas.

## Le seul manque à anticiper

**Un administrateur ne peut ni réinitialiser, ni effacer, ni désactiver le second facteur
de quelqu'un.** Il n'existe ni endpoint ni écran pour cela : l'enrôlement appartient au
compte, et l'API d'administration ne touche jamais à ces colonnes.

Une personne qui perd son application d'authentification **et** ses codes de secours, sur
une installation où le code par e-mail est désactivé, n'a donc plus aucun moyen de revenir
sur ce compte. Les moyens de revenir sont, dans l'ordre : un code de secours, le code par
e-mail ou - si le second facteur n'est pas exigé pour elle - sa désactivation par la
personne elle-même. À défaut, il ne reste qu'à supprimer le compte et à en créer un autre.

Activer le code par e-mail est l'assurance la moins coûteuse contre ce genre d'appel.
