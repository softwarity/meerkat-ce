---
title: Utilisateurs et champs de compte
section: La console
order: 170
summary: Les comptes, les capacités qu'ils détiennent, leur sécurité et les champs supplémentaires que cette installation enregistre sur une personne.
---

# Utilisateurs et champs de compte

**Application > Users** présente les comptes : qui existe sur cette gateway, ce que
chacun peut y faire, et comment il s'y connecte. **Infra > Model** en est le
complément : la structure d'un compte, définie une fois pour toutes.

Les deux écrans sont séparés à dessein. Définir un champ et le renseigner sont deux
actes, accomplis par deux personnes : l'une déclare *cette installation enregistre un
centre de coût*, l'autre indique *celui d'Alice est B200*. C'est aussi cette séparation
qui permet de transmettre un champ personnalisé en toute sécurité, puisque personne ne
peut s'attribuer lui-même un attribut auquel un service se fie.

![L'écran Users : six comptes, chacun avec ses cinq badges de capacité](img/console/users.webp)

Six comptes. Les badges sont des boutons : ici, `admin` détient tout, un compte est
app admin, un autre infra admin, et les autres ne détiennent rien.

## La liste des utilisateurs

Une ligne par compte : le point d'activation, le nom d'utilisateur, le nom complet et
l'adresse e-mail, puis les **badges de capacité**. Un badge est un bouton : un clic
accorde ou retire la capacité sans ouvrir le tiroir.

| Capacité | Ce qu'elle ouvre |
|---|---|
| `root` | Toute la gateway : routes, utilisateurs, organisations, réglages |
| `infra admin` | Le plan de routage : les routes et les pages intégrées |
| `app admin` | L'identité applicative : utilisateurs, rôles, réglages |
| `dev` | L'outillage développeur sur les applications servies |
| `tenant creator` | La création d'organisations (en mode multi-organisation uniquement) |

Les actions de ligne activent et désactivent un compte. Vous ne pouvez pas retirer
votre propre capacité `root`, ni désactiver ou supprimer votre propre compte.

La recherche porte sur le compte. Le bouton **+** en crée un.

## Le tiroir d'un compte

Trois pages dans un même tiroir, dont la première est la seule que vous
**remplissez**.

**La page du compte**

- Le nom d'utilisateur, le nom complet, l'adresse e-mail.
- **Access from** et **Access until** - la période de validité, exprimée en **jours
  et non en instants** : *jusqu'au 31* signifie toute la journée du 31. En dehors de
  cette période, la connexion est refusée et le message cite la date ; une session
  déjà ouverte est revérifiée, plutôt que coupée en plein travail.
- **Les champs propres à cette installation** - tous ceux que définit l'écran Model.

**Security** (à un clic, avec un bouton de retour)

- **Two-factor** - le second facteur est exigé, facultatif, ou hérité de la politique
  de l'application ; le libellé précise ce que donne l'héritage en ce moment.
- **Password** - *Reset password* génère un mot de passe à remettre à la personne.
  *Force a change at next sign-in* conserve le mot de passe qu'elle connaît déjà, mais
  l'empêche d'aller plus loin sans en changer, ce qui épargne un appel téléphonique
  par personne. Un compte né auprès d'une autorité n'a pas de mot de passe local, et
  le bouton s'intitule alors *Set a password* : en définir un ouvre, en connaissance
  de cause, un second moyen de se connecter.
- **External authorities** - les autorités liées à ce compte.

**Sign-in history** - toutes les connexions, avec leur origine et leur méthode. C'est
une porte d'entrée vers une autre page plutôt qu'une section : une liste aussi longue
que le compte est ancien repousserait hors de portée tout ce qui se trouve en dessous.

**Danger zone** - la suppression emporte le compte, ses appartenances, ses sessions et
ses jetons. Le journal d'audit en conserve une trace anonymisée.

## Infra > Model : les champs que porte un compte

Ce que cette installation sait d'une personne et que le produit ne pouvait pas
deviner : un matricule, un centre de coût, une référence de contrat.

Chaque définition comporte un **Name** (la clé que transmet une route), un **Label**
(ce qu'affiche l'écran du compte) et un **Type** : texte, nombre, date, choix ou
oui/non. Un champ de type **choix** reçoit aussi sa liste de valeurs, séparées par des
virgules, et c'est cette liste qui fait l'intérêt du type : elle fait d'un centre de
coût une donnée, au lieu de trois orthographes de la même chose.

Dès lors, un champ circule comme n'importe quelle autre information sur l'appelant :
sélectionnez-le dans la transmission d'identité d'une route pour l'envoyer à un
service, ou dans sa section User info pour l'inscrire sur une page.

> [!NOTE]
> **Aucun champ n'est jamais obligatoire.** Un champ défini aujourd'hui est vide sur
> tous les comptes qui existent déjà ; l'exiger bloquerait la prochaine personne qui
> ouvrirait l'un d'eux pour modifier autre chose.

Ajoutez un champ, retirez-en un, puis cliquez sur **Save** : cet écran n'a qu'un seul
bouton pour toute la liste.

## Pièges

- **Une capacité n'est pas un rôle.** Les capacités servent à administrer *cette
  console* ; les rôles sont ce que lisent vos applications. Accorder `app admin` ne
  donne aucun droit dans une application exposée par la gateway.
- **Un compte sans organisation n'a accès à rien.** Créez-le ici, puis rattachez-le à
  une organisation sur l'écran [Members](/docs/console/organisation).
- **Une valeur absente de la liste d'un champ de type choix est refusée**, par un
  message qui nomme les valeurs permises. Il en va de même d'une route qui transmet un
  champ que le modèle ne définit pas.
- **Retirer un champ du modèle met fin à sa transmission.** Il ne reste pas en
  sommeil dans la route.
- **La période de validité est vérifiée à la connexion, puis revérifiée ensuite** :
  personne n'est éjecté en plein travail par une horloge, mais plus rien de nouveau ne
  s'ouvre non plus.
