---
title: Comment on se connecte
section: Authentification
order: 100
summary: Les autorités capables de reconnaître une personne - comptes locaux, OIDC, annuaire, GitHub - et l'ordre dans lequel se déroule une connexion.
---

# Comment on se connecte

Toutes les portes d'entrée du plan de données tiennent sur un seul écran, à raison d'une
ligne chacune : **Infra > Authentication**. Les comptes que Meerkat gère lui-même y
occupent une ligne, à côté des fournisseurs d'identité et des annuaires. Une seule liste
répond ainsi à la question "qui peut prouver son identité ici ?".

Une autorité établit **qui** est la personne. Elle ne décide jamais de ce que cette
personne a le droit de faire : une première connexion crée un compte qui n'a accès à rien
tant qu'un administrateur ne l'a pas rattaché à une organisation et ne lui a pas attribué
de rôles. Cette seconde moitié relève du [contrôle d'accès](/docs/access/overview).

## Les quatre types

| Type | À quoi il sert | Sur la page de connexion | Édition |
|---|---|---|---|
| **Comptes locaux** | les comptes que Meerkat gère lui-même, avec un mot de passe qu'il conserve | le formulaire identifiant et mot de passe | les deux |
| **OpenID Connect** | un fournisseur d'identité d'entreprise : Keycloak, Entra ID, Okta, Auth0, Google | un bouton par fournisseur | les deux |
| **Annuaire** | un annuaire LDAP ou un Active Directory, interrogé directement | aucun bouton : il répond au même formulaire | **Enterprise** |
| **GitHub** | un compte GitHub, limité aux organisations que vous indiquez | un bouton | les deux |

> [!NOTE]
> SAML figure dans la console, en grisé, et Kerberos n'y figure pas du tout. Ni l'un ni
> l'autre n'est implémenté : le choix SAML n'est là que pour montrer ce qui est prévu, et
> son enregistrement est refusé. Ne bâtissez aucune intégration dessus.

Plusieurs autorités du même type peuvent coexister - deux fournisseurs OIDC, deux
annuaires - chacune avec son nom, son bouton et ses politiques. L'autorité locale fait
exception : il n'en existe qu'une, que l'on ne peut ni créer, ni dupliquer, ni supprimer,
seulement désactiver.

Désactiver toutes les autorités est permis, et c'est parfois le bon choix : une gateway
qui ne sert que des routes publiques ne laisse personne se connecter, et la page l'indique
sans préciser quelle porte est fermée.

## Ce que porte chaque autorité

| Champ | Ce qu'il détermine |
|---|---|
| Name | le libellé du bouton sur la page de connexion |
| Identifier | le segment de l'URL de connexion, qui entre aussi dans l'adresse de retour communiquée au fournisseur. Figé après la création |
| Enabled | si l'autorité répond ou non |
| Order | sa position sur la page |
| Self-registration | si une personne qui se présente pour la première fois obtient un compte, ou si elle est refusée faute d'invitation |
| Two-factor | si Meerkat demande en plus son propre second facteur |

Les paramètres de connexion - un émetteur, un secret client, une URL de serveur, un compte
de service - dépendent du type d'autorité. Chacun peut être une référence au coffre plutôt
que la valeur elle-même.

## L'ordre dans lequel se déroule une connexion

1. **Le premier facteur.** Un mot de passe saisi dans le formulaire, une redirection vers un fournisseur, une passkey ou un [code reçu par e-mail](/docs/auth/email-code). Pour un mot de passe saisi, les comptes locaux sont interrogés en premier ; s'ils refusent, chaque annuaire activé est interrogé à son tour, dans l'ordre de l'écran. Un mot de passe erroné, un identifiant inconnu et un compte désactivé reçoivent tous la même réponse.
2. **Un mot de passe à changer** - temporaire, ou expiré au regard de la politique. La session existe, mais elle reste bloquée à cette étape : toute navigation y ramène.
3. **Le second facteur**, si le compte y est soumis.
4. **L'organisation.** Si le compte n'en a aucune, la personne patiente sur `/account-pending`. S'il en a une seule, elle est inscrite sur la session sans rien demander. S'il en a plusieurs, la personne choisit.
5. **Le groupe**, en mode groupe exclusif, quand l'organisation en compte plusieurs.

Une passkey règle d'un coup les étapes un à trois : elle vaut pour les deux facteurs, et la
personne arrive directement au choix de l'organisation.

## La règle sur le cumul des facteurs

C'est l'autorité qui reconnaît le compte qui a le dernier mot : elle décide si Meerkat
demande ou non son propre second facteur. Une autorité réglée sur *Two-factor: Left to the
authority* déclare que "ce fournisseur a déjà vérifié la personne", et Meerkat ne redemande
rien.

Deux points importants :

- C'est une **déclaration, pas une preuve**. Meerkat ne lit pas les claims `acr` et `amr` qu'un fournisseur peut envoyer : rien ne vérifie donc que le fournisseur a réellement exigé un second facteur. Ne choisissez cette valeur que si vous en êtes certain.
- Les deux autres valeurs ont exactement le même effet : *Always required* sur une autorité n'impose **pas** de second facteur. Ce qui compte, c'est le réglage du compte, puis celui de l'installation. Voir [Second facteur](/docs/auth/mfa).

Un compte n'a pas de colonne *source*. Les autorités qui connaissent une personne sont
enregistrées sous forme de liens - un par autorité et par sujet. Un même compte peut donc
être ouvert à la fois par un mot de passe, par un fournisseur et par un annuaire, et une
personne qui s'est inscrite localement conserve tout lorsqu'elle passe au SSO.

## Où se configure quoi

| Écran | Ce qu'on y trouve |
|---|---|
| **Infra > Authentication** | les autorités, leurs paramètres de connexion et leurs politiques, ainsi que la valeur par défaut de l'auto-inscription |
| **Infra > Mail relay** | le relais par lequel partent les e-mails de confirmation, de réinitialisation et de second facteur |
| **Application > Security** | la politique de mot de passe, la limitation des tentatives de connexion, le second facteur, la connexion par code reçu par e-mail, les navigateurs de confiance, les passkeys, les jetons d'API personnels, la durée de vie des sessions |
| **Application > Built-in pages** | l'apparence des pages de connexion |
| **Application > Users** | les comptes eux-mêmes, et les réglages propres à chacun |
