---
title: Général et sécurité
section: La console
order: 166
summary: Ce qu'est cette installation, les langues qu'elle parle, et les politiques auxquelles tous les comptes sont soumis.
---

# Général et sécurité

Deux écrans du plan **Application**. Ils regroupent ce qui vaut pour l'application
entière, et il faut la capacité `app admin` (ou `root`) pour y écrire.

## General

![L'écran General : l'interrupteur des organisations, le mode développeur et la grille hebdomadaire des heures ouvrées](img/console/general.webp)

Les deux interrupteurs, puis les heures ouvrées : un fuseau horaire et une ligne par
jour, chacune avec sa plage horaire et un + pour en ajouter une seconde.

Deux déclarations sur ce que cette installation **est**, et une plage d'accès.

**Several organisations.** Désactivé, cette gateway sert une seule organisation et
ne la nomme jamais : ses groupes et ses membres s'administrent ici même, sous
Application. Activé, les organisations ont leur propre entrée dans le rail. **Revenir**
à une seule organisation demande d'abord confirmation, en comptant ce qui cessera
d'être servi : rien n'est supprimé, et réactiver l'option rétablit aussitôt les
organisations et leurs accès.

> [!NOTE]
> Édition Enterprise : plusieurs organisations.

**Developer mode.** Activé, les comptes dotés de la capacité `dev` disposent de leurs
outils sur les applications que sert cette gateway : le menu Developer du bouton
utilisateur, le mode de test de l'interface, la documentation d'API des routes.
Désactivé, rien de tout cela n'existe, quel que soit le nombre de comptes dotés de la
capacité. Gratuit dans les deux éditions.

Si la gateway est déclarée en production là où elle tourne, l'interrupteur est
grisé et l'indique : la surface développeur reste fermée quoi qu'en dise la console,
si bien qu'une base de données restaurée depuis la préproduction ne peut pas la
rouvrir.

**Working hours** définit la plage d'accès de toute l'application : chaque organisation
en hérite, sauf si elle définit la sienne.

> [!NOTE]
> Édition Enterprise : les heures ouvrées.

> **Les langues ont déménagé.** Une liste de langues commune à toute la gateway se
> trouvait auparavant ici, et chaque route décochait celles qu'elle ne prenait pas en
> charge. C'était prendre le problème à l'envers : personne ne sait quelles langues
> parle une gateway, ce sont les applications placées derrière elle qui le savent.
> Une route déclare désormais les langues dans lesquelles elle est écrite, sous
> **Routes > (une route) > Locales**, et l'installation propose l'union de ces
> déclarations - déployez une route qui parle polonais et la page de connexion propose
> le polonais, retirez-la et l'offre se réduit d'autant. Rien à synchroniser, et aucun
> écran à penser à compléter.

## Security

Les politiques auxquelles tous les comptes sont soumis. Un seul bouton Save, en bas,
pour tout l'écran.

![L'écran Security sur la politique de mots de passe : les types de caractères, l'historique, l'expiration et la durée de validité d'un mot de passe temporaire](img/console/security.webp)

- **Two-factor** - impose un second facteur à tout le monde ; les organisations et les
  membres peuvent déroger à ce réglage. Juste à côté, **un code à usage unique envoyé
  par e-mail** sert de solution de secours à un utilisateur déjà enrôlé qui n'a pas
  accès à son application d'authentification : il nécessite un
  [relais de messagerie](/docs/console/mail-relay) et n'est proposé qu'aux comptes qui
  ont une adresse et ont déjà configuré une application d'authentification.
- **Session TTL** - la durée de vie d'une session avant que l'utilisateur doive se
  reconnecter.
- **Passwords** - ce que doit contenir un nouveau mot de passe : sa longueur, et le
  nombre de minuscules, de majuscules, de chiffres et de caractères spéciaux. **Zéro
  signifie que la règle n'est pas exigée**, et une règle qui n'est pas exigée n'est pas
  non plus montrée aux utilisateurs. L'écran affiche un aperçu exact de la liste que
  liront vos utilisateurs. Si les types de caractères exigent déjà plus de caractères
  que la longueur, il signale que la longueur sera relevée à l'enregistrement.
  - **No reuse of the last N** - les anciens mots de passe sont conservés sous forme
    d'empreintes et comparés au nouveau mot de passe choisi, celui en cours compris.
  - **Expires after (days)** - vérifié à la connexion, et non par une horloge : un mot
    de passe expiré envoie la personne sur la page de changement, au lieu de mettre fin
    à une session dans laquelle elle travaille. Un mot de passe dont l'âge est inconnu
    n'expire jamais.
  - **Temporary (hours)** - la durée pendant laquelle un mot de passe attribué par un
    administrateur fonctionne avant que son titulaire le remplace. Passé ce délai, la
    connexion avec ce mot de passe est refusée. La boîte de dialogue qui affiche le mot
    de passe indique sa durée de validité.
  - **Force a change for everyone** (root) oblige chaque compte local à changer de mot
    de passe à sa prochaine connexion, le vôtre excepté. Pour un seul compte à la fois,
    passez par [Users](/docs/console/users).
- **Rate limiting** - le nombre d'échecs de connexion par adresse et par compte sur une
  période donnée, et le nombre de codes de second facteur erronés par compte. Zéro
  désactive une limite.
- **Passkeys** - autorise les utilisateurs à enregistrer des passkeys et à s'en servir
  pour se connecter, à la place du mot de passe et du second facteur.
- **API tokens** - autorise les utilisateurs à créer des jetons d'accès personnels
  depuis leur profil, pour appeler les routes d'API situées derrière la gateway sans
  session de navigateur. Un jeton agit avec le contexte qu'avait l'utilisateur au
  moment de sa création. Il s'agit des jetons des utilisateurs, pas des
  [jetons d'administration](/docs/console/access-and-agents).
- **Trusted browsers** - autorise les utilisateurs à ne plus saisir le second facteur
  sur un navigateur qu'ils déclarent de confiance, pendant une durée que vous
  choisissez.

## Pièges

- **General enregistre avec le bouton, mais pas ses deux interrupteurs.** Le mode
  d'organisation et le mode développeur prennent effet dès le clic ; les heures ouvrées
  attendent Save.
- **Les langues sont celles des applications, pas celles de la console.** La console
  n'existe qu'en anglais, et ce que proposent les pages intégrées vient des routes.
- **Une politique de mots de passe vide accepte n'importe quoi**, même un mot de passe
  très court. L'écran le dit en toutes lettres.
- **Session TTL se trouve dans Security, pas dans General** : la durée pendant laquelle
  on reste connecté est une politique de sécurité, au même titre que le second facteur
  qui protège cette même session.
