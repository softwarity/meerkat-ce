---
title: Authentification
section: La console
order: 156
summary: Les autorités par lesquelles on se connecte, y compris les comptes que cette passerelle détient.
---

# Authentification

**Infra > Authentication.** Toutes les portes vers le plan de données : les mots de
passe que cette passerelle détient, et les fournisseurs d'identité et annuaires
auxquels elle délègue.

Une autorité prouve qui est quelqu'un. Elle ne décide jamais de ce qu'il peut faire :
une première connexion crée un compte qui n'atteint rien jusqu'à ce qu'un
administrateur le place dans une organisation et lui donne des rôles. Eteignez toutes
les autorités et plus personne ne se connecte au plan de données, ce que peut vouloir
une passerelle qui ne sert que des routes publiques.

![L'écran Authentication : l'interrupteur d'auto-inscription et une ligne pour les comptes locaux](img/console/auth-providers.webp)

Une installation neuve : l'auto-inscription éteinte, et une seule autorité - les
mots de passe détenus ici - qui, elle, est allumée.

## La liste

Une ligne par autorité : son nom et son identifiant, son genre, le serveur qu'elle
joint, et un interrupteur. Allumer ou éteindre est un clic sur la ligne : c'est une
décision, pas une modification.

Une pastille **Invite only** dit que cette autorité refuse de créer des comptes :
seules les personnes déjà rattachées à un compte entrent.

Au-dessus du tableau, **Self-registration** est la réponse de toute l'application.
Chaque autorité peut dire autrement ; éteint ici, aucune ne peut.

## Local accounts

La première ligne, ce sont les comptes que Meerkat détient lui-même. Il n'y a pas de
serveur à joindre, mais elle possède deux questions, et l'ouvrir les pose :

- **Self-registration** - hérité, autorisé ou refusé. Autorisé met un formulaire
  d'inscription sur la page de connexion, avec l'adresse confirmée par e-mail : il
  faut donc un [relais mail](/docs/console/mail-relay).
- **Anti-robot check** - garde ce formulaire, et rien d'autre. Quelqu'un qui arrive
  par un annuaire s'est déjà prouvé là-bas.

> [!NOTE]
> Eteindre les comptes locaux ferme la connexion par mot de passe sur le **plan de
> données**. Cette console garde toujours sa propre connexion par mot de passe : vous
> ne pouvez pas vous en verrouiller dehors ici.

## Ajouter une autorité

Le genre se choisit d'abord, et ne se change plus ensuite.

| Genre | Ce que c'est |
|---|---|
| **OpenID Connect** | Un fournisseur d'identité vers lequel le navigateur est envoyé. Son jeton est vérifié contre ses clés publiées |
| **Directory** | Un annuaire LDAP ou un Active Directory, interrogé directement avec ce que la personne a tapé. Aucun bouton sur la page de connexion |
| **GitHub** | GitHub prouve le compte mais ne signe rien : la confiance repose sur l'échange et sur TLS |
| **SAML** | Pas encore disponible |

> [!NOTE]
> Edition Enterprise : **Directory** (LDAP et Active Directory).

En haut de l'éditeur, **Register these on the authority** vous tend les valeurs à
coller dans le formulaire du fournisseur, sous ses propres libellés, chacune avec un
bouton de copie. Pour GitHub, un lien mène droit au formulaire qui crée une
application OAuth. Si vous avez joint cette console par une adresse locale, le bloc
vous avertit : un fournisseur à qui on donne un rappel `localhost` renvoie tout le
monde vers une machine qui n'est pas la vôtre.

### Les champs qui méritent un mot

- **Identifier** - dérivé du nom, et c'est le segment d'URL par lequel une connexion
  voyage. Il est figé après la création, car le changer casserait le rappel déjà
  enregistré chez le fournisseur.
- **Issuer** (OIDC) - le document de découverte est lu là, donc les autres points
  d'entrée n'ont pas à être tapés.
- **Allowed e-mail domains** (OIDC) - vide accepte toutes les adresses que l'autorité
  connaît. A remplir quand le fournisseur est partagé avec des gens qui ne sont pas
  les vôtres.
- **Allowed organisations** (GitHub) - **vide laisse entrer n'importe quel compte
  GitHub**. Ces organisations et équipes reviennent aussi comme groupes, nommés `org`
  et `org/team`, qu'une [règle de groupe](/docs/console/organisation) peut
  transformer en rôles.
- **Service account** (Directory) - sert à *chercher* dans l'annuaire, jamais à
  connecter quelqu'un.
- **User filter** (Directory) - `%s` est ce que la personne a tapé ; vide utilise le
  défaut du dialecte.
- **Skip the certificate check** - pour un annuaire à certificat auto-signé. C'est le
  seul champ ici qui retire une protection.

### Policies

Deux questions que chaque autorité tranche pour elle-même, chacune pouvant s'en
remettre à l'application :

- **Self-registration** - hérité, autorisé, refusé.
- **Two-factor** - hérité, toujours exigé, ou laissé à l'autorité (pour un
  fournisseur qui challenge déjà).

## Avant de quitter l'éditeur

**Test the connection** joint vraiment le serveur et dit ce qui est revenu. Faites-le
avant d'annoncer que l'autorité est prête. La suppression est dans la zone de danger,
en bas.

## Pièges

- **Une première connexion ne donne rien.** Elle crée un compte sans organisation et
  sans rôle. Placez les gens à la main sur
  [Members](/docs/console/organisation), ou écrivez des règles de groupe.
- **Une liste d'autorisation vide est une porte ouverte.** GitHub sans organisation
  nommée laisse entrer tout GitHub.
- **Les secrets ne se tapent pas deux fois.** Un secret client va dans le
  [coffre](/docs/console/vault) et ne peut pas être enregistré en littéral.
