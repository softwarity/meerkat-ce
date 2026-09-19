---
title: Utilisateurs et champs de compte
section: La console
order: 170
summary: Les comptes, les capacités qu'ils portent, leur sécurité, et les champs supplémentaires que cette installation enregistre sur une personne.
---

# Utilisateurs et champs de compte

**Application > Users**, ce sont les comptes : qui existe sur cette passerelle, ce
qu'il peut faire à travers elle, et comment il entre. **Infra > Model** est l'autre
moitié : la forme d'un compte, décidée une fois.

Les deux sont séparés volontairement. Définir un champ et le remplir sont deux gestes
de deux métiers : l'un dit *cette installation enregistre un centre de coût*, l'autre
dit *celui d'Alice est B200*. C'est aussi cette séparation qui rend un champ
transmissible en confiance, puisque personne ne peut s'attribuer un attribut auquel un
service croit.

## La liste des utilisateurs

![L'écran Users : six comptes, chacun avec ses quatre pastilles de capacité](img/console/users.webp)

Six comptes. Les pastilles sont des boutons : ici `admin` porte tout, un compte est
app admin et un autre infra admin, les autres ne portent rien.

Une ligne par compte : le point d'activation, l'identifiant, le nom complet et
l'adresse, et les **pastilles de capacité**. Une pastille est un bouton : la cliquer
donne ou retire le pouvoir sans ouvrir le tiroir.

| Capacité | Ce qu'elle ouvre |
|---|---|
| `root` | Toute la passerelle : routes, utilisateurs, organisations, réglages |
| `infra admin` | Le plan de routage : routes et pages intégrées |
| `app admin` | L'identité applicative : utilisateurs, rôles, réglages |
| `dev` | L'outillage développeur sur les applications servies |
| `tenant creator` | La création d'organisations (mode multi seulement) |

Les actions de ligne activent et désactivent un compte. Vous ne pouvez ni retirer votre
propre `root`, ni vous désactiver, ni vous supprimer.

La recherche porte sur le compte. Le bouton **+** en crée un.

## Le tiroir d'un compte

Trois pages dans un tiroir, et la première est la seule que l'on **remplit**.

**La page du compte**

- Identifiant, nom complet, courriel.
- **Access from** et **Access until** - la fenêtre de validité, en **jours et non en
  instants** : *jusqu'au 31* vaut tout le 31. Hors fenêtre, la connexion est refusée en
  nommant la date, et une session déjà ouverte est revérifiée plutôt que coupée en plein
  travail.
- **Les champs propres à cette installation** - ceux que l'écran Model définit.

**Security** (à une touche, et qui revient)

- **Two-factor** - exigé, optionnel, ou hérité de la politique applicative ; le libellé
  dit ce que l'héritage donne aujourd'hui.
- **Password** - *Reset password* produit un mot de passe à remettre en main propre, et
  *Force a change at next sign-in* garde celui que la personne connaît déjà et refuse
  d'aller plus loin avec, ce qui économise un appel téléphonique par personne. Un compte
  né chez une autorité n'a pas de mot de passe local : le bouton dit alors *Set a
  password*, et le faire ouvre délibérément une seconde porte.
- **External authorities** - quelles autorités sont rattachées à ce compte.

**Sign-in history** - chaque connexion, d'où et comment. C'est une porte et non une
section, parce qu'une liste aussi longue que le compte est vieux repousserait tout le
reste hors de portée.

**Danger zone** - la suppression emporte le compte, ses appartenances, ses sessions et
ses jetons. Le journal d'audit garde une trace anonymisée.

## Infra > Model : les champs que porte un compte

Ce que cette installation sait d'une personne et que le produit ne pouvait pas deviner :
un matricule, un centre de coût, une référence de contrat.

Chaque définition a un **Name** (la clé qu'une route transmet), un **Label** (ce que
l'écran du compte affiche) et un **Type** : texte, nombre, date, choix ou oui/non. Un
champ **choix** prend aussi sa liste, séparée par des virgules - et c'est la liste qui
justifie le type : elle transforme un centre de coût en donnée plutôt qu'en trois
orthographes de la même chose.

Dès lors, un champ voyage comme n'importe quel autre fait sur l'appelant : choisissez-le
dans la transmission d'identité d'une route pour l'envoyer à un service, ou dans son
user info pour le poser sur une page.

> [!NOTE]
> **Aucun champ n'est jamais obligatoire.** Un champ défini aujourd'hui est vide sur tous
> les comptes qui existent déjà, et l'exiger bloquerait la prochaine personne qui en
> ouvre un pour changer autre chose.

Ajoutez un champ, retirez-en un, puis **Save** : cet écran a un seul bouton pour toute la
liste.

## Pièges

- **Une capacité n'est pas un rôle.** Les capacités administrent *cette console* ; les
  rôles sont ce que lisent vos applications. Donner `app admin` ne donne rien dans une
  application proxifiée.
- **Un compte sans organisation n'atteint rien.** Créez-le ici, puis placez-le dans
  [Members](/docs/console/organisation).
- **Une valeur hors de la liste d'un choix est refusée**, par une phrase qui nomme ce qui
  est permis - comme l'est une route qui transmet un champ que le modèle ne définit pas.
- **Retirer un champ du modèle l'empêche de voyager.** Il ne dort pas dans la route.
- **La fenêtre de validité est vérifiée à la connexion et revérifiée ensuite** : personne
  n'est jeté dehors en plein travail par une horloge, mais rien de nouveau ne s'ouvre non
  plus.
