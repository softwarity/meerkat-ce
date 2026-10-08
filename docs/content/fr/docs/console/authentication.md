---
title: Authentification
section: La console
order: 156
summary: Les autorités auprès desquelles on peut se connecter, comptes détenus par la gateway compris.
---

# Authentification

**Infra > Authentication.** Toutes les portes d'entrée du plan de données : les mots de
passe que détient cette gateway, et les fournisseurs d'identité et annuaires
auxquels elle délègue l'authentification.

Une autorité prouve l'identité d'une personne. Elle ne décide jamais de ce que cette
personne a le droit de faire : une première connexion crée un compte qui n'a accès à
rien tant qu'un administrateur ne l'a pas placé dans une organisation et ne lui a pas
attribué de rôles. Désactivez toutes les autorités et plus personne ne se connecte au
plan de données - ce qui peut convenir à une gateway qui ne sert que des routes
publiques.

![L'écran Authentication : l'interrupteur de l'auto-inscription et une ligne par autorité - les comptes locaux, un annuaire et GitHub](img/console/auth-providers.webp)

Une installation toute neuve : l'auto-inscription est désactivée, et il n'y a qu'une
autorité - les mots de passe détenus ici -, qui est activée.

## La liste

Une ligne par autorité : son nom et son identifiant, son type, le serveur qu'elle
contacte et un interrupteur. Activer ou désactiver une autorité se fait d'un clic sur
la ligne - c'est une décision, pas une modification.

Une pastille **Invite only** signale que cette autorité refuse de créer des comptes :
seules les personnes déjà rattachées à un compte peuvent entrer.

Au-dessus du tableau, **Self-registration** donne la réponse valable pour toute
l'application. Chaque autorité peut en décider autrement ; mais si le réglage est
désactivé ici, aucune ne le peut.

## Local accounts

La première ligne correspond aux comptes que Meerkat détient lui-même. Elle n'a aucun
serveur à contacter, mais deux questions relèvent d'elle, et elle les pose quand on
l'ouvre :

- **Self-registration** - hérité, autorisé ou refusé. Autorisé, le réglage ajoute un
  formulaire d'inscription à la page de connexion, avec confirmation de l'adresse par
  e-mail : il faut donc un [relais de messagerie](/docs/console/mail-relay).
- **Anti-robot check** - protège ce formulaire, et rien d'autre. Une personne qui
  arrive par un annuaire a déjà dû faire ses preuves là-bas.

> [!NOTE]
> Désactiver les comptes locaux ferme la connexion par mot de passe sur le **plan de
> données**. Cette console conserve toujours sa propre connexion par mot de passe : vous
> ne risquez donc pas de vous en interdire l'accès depuis cet écran.

## Ajouter une autorité

![Un annuaire ouvert dans son éditeur : le type, le serveur, la base de recherche, le compte de service lu dans le coffre](img/console/auth-provider-editor.webp)

Le type se choisit en premier, et ne peut plus être modifié ensuite.

| Type | Ce que c'est |
|---|---|
| **OpenID Connect** | Un fournisseur d'identité vers lequel le navigateur est redirigé. Son jeton est vérifié à l'aide des clés qu'il publie |
| **Directory** | Un annuaire LDAP ou un Active Directory, interrogé directement avec ce que la personne a saisi. Aucun bouton sur la page de connexion |
| **GitHub** | GitHub atteste le compte mais ne signe rien : la confiance repose sur l'échange et sur TLS |
| **SAML** | Pas encore disponible |

> [!NOTE]
> Édition Enterprise : **Directory** (LDAP et Active Directory).

En haut de l'éditeur, **Register these on the authority** vous fournit les valeurs à
coller dans le formulaire du fournisseur, sous les libellés que celui-ci emploie,
chacune avec son bouton de copie. Pour GitHub, un lien mène directement au formulaire
de création d'une application OAuth. Si vous avez ouvert cette console par une adresse
locale, le bloc vous avertit : un fournisseur à qui l'on donne une URL de retour en
`localhost` renvoie tout le monde vers une machine qui n'est pas la vôtre.

### Les champs qui méritent une explication

- **Identifier** - dérivé du nom, c'est le segment par lequel passe l'URL de
  connexion. Il est figé après la création, car le modifier casserait l'URL de retour
  déjà enregistrée chez le fournisseur.
- **Issuer** (OIDC) - le document de découverte est lu à cette adresse, ce qui évite
  de saisir les autres endpoints.
- **Allowed e-mail domains** (OIDC) - vide, le champ accepte toutes les adresses que
  connaît l'autorité. Renseignez-le quand le fournisseur est partagé avec des personnes
  extérieures à votre organisation.
- **Allowed organisations** (GitHub) - **vide, le champ laisse entrer n'importe quel
  compte GitHub**. Ces organisations et ces équipes sont aussi transmises comme
  groupes, nommés `org` et `org/team`, qu'une
  [règle de groupe](/docs/console/organisation) peut convertir en rôles.
- **Service account** (Directory) - sert à chercher dans l'annuaire, jamais à
  connecter qui que ce soit.
- **User filter** (Directory) - `%s` représente ce que la personne a saisi ; vide, le
  champ prend la valeur par défaut du dialecte.
- **Skip the certificate check** - pour un annuaire dont le certificat est auto-signé.
  C'est le seul champ de cet écran qui supprime une protection.

### Policies

Deux questions auxquelles chaque autorité répond pour son propre compte, avec chaque
fois la possibilité de s'en remettre à l'application :

- **Self-registration** - hérité, autorisé, refusé.
- **Two-factor** - hérité, toujours exigé, ou laissé à l'autorité (pour un fournisseur
  qui demande déjà un second facteur).

## Avant de quitter l'éditeur

![Le pied de l'éditeur : les politiques, et la section Test qui interroge l'annuaire sur quelqu'un et liste qui est entré par lui](img/console/auth-provider-test.webp)

**Test the connection** contacte réellement le serveur et affiche sa réponse.
Lancez ce test avant d'annoncer à qui que ce soit que l'autorité est prête. La
suppression se trouve dans la zone de danger, tout en bas.

## Pièges

- **Une première connexion ne donne aucun droit.** Elle crée un compte sans
  organisation ni rôle. Placez les personnes à la main depuis
  [Members](/docs/console/organisation), ou écrivez des règles de groupe.
- **Une liste d'autorisation vide est une porte ouverte.** GitHub sans aucune
  organisation renseignée laisse entrer tout GitHub.
- **Un secret ne se saisit pas deux fois.** Un secret client va dans le
  [coffre](/docs/console/vault) et ne peut pas être enregistré en clair.
