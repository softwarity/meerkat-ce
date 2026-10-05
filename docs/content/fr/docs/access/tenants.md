---
title: Organisations
section: Contrôle d'accès
order: 138
summary: Les appartenances, le choix à la connexion, le propriétaire, et les réglages qu'une organisation peut surcharger.
---

# Organisations

Une organisation - le tenant des URL et de l'API - est ce au nom de quoi une
personne travaille. C'est le périmètre dans lequel vivent les rôles : les
groupes appartiennent à une organisation, et une session sans organisation
active ne détient aucun rôle.

Toute installation en possède une dès son premier démarrage, et bien souvent
elle n'en aura jamais besoin d'une autre.

> [!NOTE] **Édition Enterprise.**
> Disposer de plusieurs organisations fait partie de l'édition Enterprise.
> L'image Community sert une **seule** organisation, qu'aucun écran ne nomme
> jamais : ses groupes, ses membres et ses règles s'administrent depuis
> **Application**, et la notion n'apparaît nulle part ailleurs. Le passage au
> mode multi-organisation est une décision du compte root, sur une image
> Enterprise.

Revenir au mode à une seule organisation ne supprime rien : les autres
organisations cessent simplement d'être servies, et celle qui reste servie est
la plus ancienne.

## Les membres

Une appartenance, c'est une personne dans une organisation, de type **ADMIN**
ou **USER**, active ou non. L'énumération s'arrête là : il n'existe pas
d'appartenance OWNER.

**La propriété est un attribut de l'organisation**, pas une appartenance : une
organisation a toujours un propriétaire, la propriété peut être transférée, et
le propriétaire n'est pas obligatoirement membre. Qui crée une organisation en
est le propriétaire.

Une appartenance **ADMIN**, ou la qualité de propriétaire, permet à une
personne d'administrer cette organisation depuis la console : l'organisation
elle-même, ses membres, ses groupes, ses règles de groupe, la réinitialisation
du mot de passe d'un membre, la lecture de l'historique de connexion d'un
membre. Elle n'accorde rien en dehors de cette organisation, et notamment pas
le catalogue global des rôles.

Une personne peut aussi **quitter** l'organisation dans laquelle elle se
trouve, depuis son profil : *Quitter cette organisation*, à côté de son nom,
puis une confirmation précisant que seul un administrateur pourra l'y rajouter.
L'appartenance disparaît, la session cesse aussitôt de porter l'organisation,
et la ligne `member.leave` s'inscrit dans le [journal
d'audit](/docs/operations/audit) de l'organisation. Cela ne concerne que les
installations multi-organisations : quand il n'y en a qu'une, il n'y a rien à
quitter.

![Les membres d'une organisation, et ce que chacun y est](img/console/members.webp)

## Le choix à la connexion

Une fois le premier et le second facteur validés :

| Appartenances | Ce qui se passe |
|---|---|
| aucune | la session est émise sans organisation. Une personne qui n'administre rien arrive sur `/account-pending`, la salle d'attente |
| une seule | elle est inscrite dans la session, sans rien demander |
| plusieurs | la personne choisit, sur `/select-tenant` |

Seules comptent les appartenances actives dans des organisations actives.

Une session sans organisation, dont le titulaire obtient par la suite
exactement une appartenance, l'adopte à la requête suivante : l'ajout à une
organisation prend effet sans qu'il faille se déconnecter.

Le changement d'organisation se fait ensuite depuis le bouton utilisateur, et
rejoue l'étape du groupe : la nouvelle organisation peut avoir un autre mode de
groupe et d'autres groupes.

## Heures ouvrées

Une organisation peut restreindre les moments où ses membres peuvent entrer :
jours de la semaine, plages horaires, dates de début et de fin, dans un fuseau
horaire nommé.

> [!NOTE] **Édition Enterprise.**
> Les heures ouvrées font partie de l'édition Enterprise.

La fenêtre se résout à partir du niveau le plus précis qui en définit une :
l'appartenance, puis l'organisation, puis l'installation. Quand c'est la
fenêtre de l'organisation qui s'applique, les dates de début et de fin de
l'appartenance s'y superposent malgré tout. La valeur par défaut ouvre tous les
jours, 24 heures sur 24.

> [!WARNING]
> La fenêtre est vérifiée **à la connexion et au changement d'organisation, et
> nulle part ailleurs**. Une session déjà ouverte n'est pas coupée quand la
> fenêtre se ferme, et rien n'est revérifié en cours de session. FEATURES.md
> donne ce contrôle, ainsi que la modification par membre dans la console,
> comme la moitié manquante.

Un refus pour cause d'horaires le dit clairement : il ne ressemble jamais à un
mot de passe erroné.

## Ce qu'une organisation peut surcharger

| Réglage | Niveaux, du plus précis au plus général | Où il se modifie |
|---|---|---|
| Durée de vie de la session | appartenance, organisation, installation | l'installation dans la console ; les deux autres par l'API d'administration |
| Heures ouvrées | appartenance, organisation, installation | l'organisation dans la console |
| Mode de groupe | organisation uniquement | l'écran de l'organisation |
| Second facteur | **compte, puis installation** | les deux dans la console |

Le second facteur n'a volontairement **aucun niveau organisation** : il est
demandé avant que l'organisation soit connue, et une règle par organisation ne
pourrait donc pas être lue à temps.

Tout le reste - la politique de mot de passe, la limitation des tentatives, les
passkeys, le thème, les langues, le relais de messagerie - vaut pour toute
l'installation.

## Isolation

Chaque requête porte l'organisation dans laquelle elle est faite, et c'est à
elle que sont confrontées les règles de route, les règles d'endpoint et
l'identité transmise à l'upstream. Votre service reçoit l'organisation comme un
fait établi au sujet de l'appelant, et non comme une demande de sa part.

Un point à connaître : **désactiver une organisation, ou une appartenance, ne
met pas fin aux sessions qui y travaillent déjà.** Cela empêche les nouvelles
connexions de l'adopter, mais une session qui porte déjà cette organisation la
conserve, avec les rôles qu'elle y détient, jusqu'à son expiration.

Ce qui prend effet immédiatement, en revanche, c'est le retrait d'une personne
d'un **groupe** : ses rôles sont recalculés à la requête suivante. Le moyen le
plus rapide de retirer un accès sur-le-champ est donc de vider les groupes, et
non de désactiver l'appartenance.
