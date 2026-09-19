---
title: Général, Locales et Security
section: La console
order: 166
summary: Ce qu'est cette installation, les langues qu'elle parle, et les politiques sous lesquelles vivent tous les comptes.
---

# Général, Locales et Security

Trois écrans en tête du plan **Application**. Ils portent ce qui est vrai de toute
l'application, et y écrire demande la capacité `app admin` (ou `root`).

## General

![L'écran General : l'interrupteur des organisations, le mode développeur, et la grille hebdomadaire des heures ouvrées](img/console/general.webp)

Les deux interrupteurs, puis les heures ouvrées : un fuseau et une ligne par
jour, chacune avec sa fenêtre et un + pour en ajouter une seconde.

Deux affirmations sur ce que cette installation **est**, et une fenêtre d'accès.

**Several organisations.** Eteint, cette passerelle sert une organisation et ne la
nomme jamais : ses groupes et ses membres sont administrés ici même, sous Application.
Allumé, les organisations obtiennent leur entrée dans le rail. Redescendre à une seule
organisation demande d'abord, et compte ce qui cesse d'être servi : rien n'est supprimé,
et rebasculer ramène aussitôt les organisations et leurs accès.

> [!NOTE]
> Edition Enterprise : plusieurs organisations.

**Developer mode.** Allumé, les comptes qui portent la capacité `dev` obtiennent leur
outillage sur les applications que cette passerelle sert : le menu Developer dans le
bouton utilisateur, le mode test de l'UI, la doc API des routes. Eteint, rien de tout
cela n'existe, quel que soit le nombre de comptes qui portent la capacité. Gratuit dans
les deux éditions.

Si la passerelle est déclarée de production là où elle tourne, l'interrupteur est
désactivé et le dit : la surface développeur reste fermée quoi que dise la console, pour
qu'une base restaurée depuis une préproduction ne puisse pas la rouvrir.

**Working hours** est la fenêtre d'accès de toute l'application, que chaque organisation
hérite sauf si elle définit la sienne.

> [!NOTE]
> Edition Enterprise : les heures ouvrées.

## Locales

Les langues que **votre application** prend en charge, en codes ISO (`fr`, `en-GB`,
`pt-BR`...). Elles remplissent les pages intégrées et le bouton utilisateur, et le
choix d'un utilisateur connecté suit chaque requête proxifiée.

Ajoutez-en une avec l'autocomplétion, qui propose les langues courantes et leurs
variantes régionales et refuse un code invalide. Chaque ligne montre le code, son nom
dans votre langue et son nom propre. Ajouter et retirer enregistrent aussitôt : il n'y a
pas de bouton Save ici.

Vide laisse les pages intégrées en anglais.

## Security

Les politiques sous lesquelles vivent tous les comptes. Un bouton Save en bas pour tout
l'écran.

- **Two-factor** - exiger un second facteur pour tout le monde ; les organisations et
  les membres peuvent surcharger. A côté, **un code à usage unique par courriel** comme
  secours pour un utilisateur enrôlé qui n'atteint pas son authentificateur : il faut un
  [relais mail](/#/docs/console/mail-relay), et il n'apparaît que pour les comptes qui
  portent une adresse et ont déjà configuré un authentificateur.
- **Session TTL** - combien de temps une session vit avant qu'il faille se reconnecter.
- **Passwords** - ce qu'un nouveau mot de passe doit contenir : longueur, et combien de
  minuscules, majuscules, chiffres et caractères spéciaux. **Zéro veut dire que la règle
  n'est pas demandée**, et une règle non demandée n'est pas montrée aux utilisateurs non
  plus. L'écran prévisualise exactement la liste qu'ils liront. Si les genres exigent
  déjà plus de caractères que la longueur, il dit que la longueur sera relevée à
  l'enregistrement.
  - **No reuse of the last N** - les anciens mots de passe sont gardés en empreintes et
    comparés au choix d'un nouveau, celui en usage compris.
  - **Expires after (days)** - vérifié à la connexion, pas par une horloge : un mot de
    passe expiré envoie la personne sur la page de changement plutôt que de terminer une
    session dans laquelle elle travaille. Un mot de passe dont l'âge est inconnu n'expire
    jamais.
  - **Force a change for everyone** (root) fait changer chaque compte local à sa
    prochaine connexion, sauf le vôtre. Un compte à la fois se fait dans
    [Users](/#/docs/console/users).
- **Rate limiting** - connexions échouées par adresse et par compte dans la fenêtre, et
  codes de second facteur erronés par compte. Zéro désactive un limiteur.
- **Passkeys** - laisser les utilisateurs enregistrer des passkeys et se connecter avec,
  à la place du mot de passe et du second facteur.
- **API tokens** - laisser les utilisateurs frapper des jetons d'accès personnels depuis
  leur profil, pour appeler les routes API derrière la passerelle sans session de
  navigateur. Un jeton agit avec le contexte de l'utilisateur au moment de sa création.
  Ce sont les jetons des utilisateurs, pas les
  [jetons d'administration](/#/docs/console/access-and-agents).
- **Trusted browsers** - laisser les utilisateurs sauter le challenge du second facteur
  sur un navigateur qu'ils marquent comme sûr, pour une durée que vous choisissez.

## Pièges

- **General enregistre au bouton, mais pas ses deux interrupteurs.** Le mode
  d'organisation et le mode développeur prennent effet au clic ; les heures ouvrées
  attendent Save.
- **Les langues sont celles de l'application, pas de la console.** La console est en
  anglais seulement.
- **Une politique de mot de passe vide accepte n'importe quoi**, aussi court que ce soit.
  L'écran le dit en toutes lettres.
- **Session TTL est sur Security, pas sur General** : combien de temps on reste connecté
  est une politique de sécurité, à côté du second facteur qui garde la même session.
