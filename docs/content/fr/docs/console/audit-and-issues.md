---
title: Audit et Issues
section: La console
order: 184
summary: Les deux écrans que l'on consulte après coup - qui a modifié quoi, qui s'est connecté, et ce que vos utilisateurs ont signalé.
---

# Audit et Issues

Deux écrans transverses, voisins dans le rail. Tous deux se consultent après coup, et
tous deux sont restreints côté serveur à ce qu'administre la personne qui les
consulte : root voit tout, un administrateur infra le plan de routage, un
administrateur d'application l'identité, un administrateur d'organisation ses propres
organisations.

## Audit

Toutes les modifications administratives, avec **les champs exacts qui ont changé et
leur valeur avant et après**, ainsi que la sécurité des comptes : chaque connexion,
chaque connexion refusée avec son motif réel et son adresse, chaque facteur, passkey,
mot de passe ou jeton modifié par son titulaire. En lecture seule.

![L'écran Audit : une liste d'événements, chacun avec son auteur et les champs qui ont changé](img/console/audit.webp)

Deux configurations capturées, deux routes dont les rate limits ont été
enregistrés, un groupe renommé, un slogan réécrit et une durée de session raccourcie -
chaque fois avec l'ancienne valeur barrée et la nouvelle à côté.

Chaque événement se lit ainsi : quand, quelle action, par qui, sur quoi, et en dessous
le détail des différences champ par champ. Quand un agent ou un script a agi, le nom du
jeton figure à côté de celui du compte : *admin, via claude-desktop* au lieu de
*admin*. C'est précisément pour faire cette différence que le jeton est nommé.

Une ligne de sécurité se lit de la même façon, avec son plan (**data plane** pour les
comptes des applications, **console** pour cette console), le motif ou la méthode à
côté, et l'adresse en dessous. Une connexion refusée s'affiche dans la couleur
d'erreur : quand on parcourt le journal à la recherche d'une attaque, ce sont les
lignes à repérer sans avoir à lire. La liste complète des actions et des motifs se
trouve dans [le journal d'audit](/docs/operations/audit).

Le sélecteur choisit la partie du journal à afficher : **All**, **Changes**, **Data
plane sign-ins** (les comptes des applications) ou **Console sign-ins**. Sous
**Changes**, un type de **cible** affine encore. Viennent ensuite la **période**
(24 heures, 7 jours, 30 jours, tout l'historique) et un champ de recherche libre qui
filtre ce qui est déjà chargé, motifs et adresses compris.

En haut, **Export CSV** (Enterprise) télécharge ce que retiennent les filtres, et -
pour root uniquement - **Keep events for** fixe la durée pendant laquelle le journal
conserve un événement. Voir [le journal d'audit](/docs/operations/audit).

- Les secrets sont masqués : le journal consigne qu'un champ a changé, pas sa nouvelle
  valeur.
- Un compte supprimé laisse une trace anonymisée plutôt qu'un trou.
- Les mots qui décrivent une modification sont ceux qu'emploie le
  [point de reprise](/docs/console/configuration) de cette modification, à la même
  seconde. Un événement, un vocabulaire.

## Issues

Les signalements que vos utilisateurs déposent depuis le bouton utilisateur injecté :
une description, une capture d'écran et le contexte relevé par le navigateur.

En haut, **Collect issue reports** - un interrupteur réservé à un administrateur infra,
livré **désactivé**. Il se trouve ici plutôt que sur un écran de réglages divers, car
une liste vide ne signifie rien tant qu'on ignore si la collecte est active.
Désactivé, l'entrée disparaît du bouton utilisateur.

La liste est légère ; ouvrir un signalement charge son détail dans le tiroir,
et ce panneau est inscrit dans l'URL : un rechargement de la page y ramène. Un
signalement comporte :

- Son **statut** - ouvert, en cours, clos - modifiable depuis le tiroir, et
  qui sert de filtre sur la liste.
- Qui l'a déposé, depuis quelle organisation, et quand.
- L'**URL**, la taille de la fenêtre d'affichage et le ratio de pixels, la langue, le
  user agent.
- La **capture d'écran**, sur laquelle un clic l'ouvre en taille réelle.
- La **sortie de la console** que la page avait relevée, avec le niveau de chaque
  ligne.
- Des **commentaires**, et un champ pour en ajouter un.
- La suppression, dans la zone de danger.

Il n'existe aucun connecteur vers GitHub, GitLab ou Jira : un signalement reste ici.

## Pièges

- **Audit n'est pas un journal des requêtes.** Il consigne les modifications
  administratives et les accès aux comptes. Ce qui a traversé la gateway se lit dans
  [Metrics](/docs/console/traffic), et chaque requête est une ligne du
  [journal d'accès](/docs/operations/logs).
- **Vous voyez votre propre périmètre.** Deux administrateurs peuvent consulter le même
  écran et ne pas compter le même nombre d'événements ; c'est l'effet de la restriction
  par périmètre, pas un bug.
- **Une liste Issues vide peut signifier que la collecte est désactivée.** Vérifiez
  l'interrupteur avant de conclure que vos utilisateurs n'ont rien à dire.
- **Une capture d'écran est la page telle que l'utilisateur la voyait**, et elle peut
  contenir ses données. Traitez un signalement comme une donnée personnelle.
