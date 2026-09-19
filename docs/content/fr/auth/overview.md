---
title: Comment on se connecte
section: Authentification
order: 100
summary: Les autorités capables de reconnaître une personne - comptes locaux, OIDC, annuaire, GitHub - et l'ordre dans lequel une connexion se déroule.
---

# Comment on se connecte

Chaque porte vers le plan de données est une ligne d'un seul écran : **Infra >
Authentication**. Les comptes que Meerkat détient lui-même sont une de ces
lignes, à côté des fournisseurs d'identité et des annuaires : une seule liste
répond à "qui peut prouver son identité ici".

Une autorité prouve **qui** est quelqu'un. Elle ne décide jamais ce qu'il a le
droit de faire : une première connexion produit un compte qui n'atteint rien
jusqu'à ce qu'un administrateur le place dans une organisation et lui accorde
des rôles. Cette autre moitié, c'est le [contrôle
d'accès](/#/docs/access/overview).

## Les quatre genres

| Genre | À quoi il sert | Sur la page de connexion | Édition |
|---|---|---|---|
| **Comptes locaux** | des comptes que Meerkat détient, avec un mot de passe qu'il conserve | le formulaire identifiant / mot de passe | les deux |
| **OpenID Connect** | un fournisseur d'identité d'entreprise : Keycloak, Entra ID, Okta, Auth0, Google | un bouton par fournisseur | les deux |
| **Annuaire** | un annuaire LDAP ou un Active Directory, interrogé directement | aucun bouton : il répond au même formulaire | **Enterprise** |
| **GitHub** | un compte GitHub, restreint aux organisations que vous nommez | un bouton | les deux |

> [!NOTE]
> SAML apparaît dans la console, grisé, et Kerberos n'apparaît pas du tout.
> Aucun des deux n'est implémenté : le choix SAML existe pour que la forme soit
> visible, et l'enregistrement est refusé. Ne bâtissez pas d'intégration dessus.

Plusieurs autorités du même genre coexistent - deux fournisseurs OIDC, deux
annuaires - chacune avec son nom, son bouton et ses politiques. L'autorité
locale est l'exception : il y en a exactement une, elle ne peut être ni créée,
ni dupliquée, ni supprimée, seulement désactivée.

Tout éteindre est légal et parfois juste : une gateway qui ne sert que des
routes publiques ne laisse personne se connecter, et la page le dit sans dire
quelle porte est fermée.

## Ce que porte toute autorité

| Champ | Ce qu'il décide |
|---|---|
| Name | ce qu'affiche le bouton de la page de connexion |
| Identifier | le segment de l'URL de connexion, et une partie de l'adresse de retour donnée au fournisseur. Figé après création |
| Enabled | si elle répond du tout |
| Order | sa place sur la page |
| Self-registration | si un premier arrivant obtient un compte ici, ou s'il est écarté comme non invité |
| Two-factor | si Meerkat réclame encore son propre second facteur |

Ses paramètres de connexion - un issuer, un secret client, une URL de serveur,
un compte de service - dépendent du genre, et chacun peut être une référence au
coffre plutôt que la valeur elle-même.

## L'ordre d'une connexion

1. **Le premier facteur.** Un mot de passe tapé dans le formulaire, une redirection vers un fournisseur, ou une passkey. Pour un mot de passe tapé, les comptes locaux sont interrogés d'abord ; s'ils refusent, chaque annuaire actif est interrogé à son tour, dans l'ordre de l'écran. Un mauvais mot de passe, un identifiant inconnu et un compte désactivé répondent la même phrase.
2. **Un mot de passe à changer** - temporaire, ou expiré selon la politique. La session existe mais reste bloquée sur cette étape, et toute navigation y ramène.
3. **Le second facteur**, si le compte en doit un.
4. **L'organisation.** Aucune, et la personne attend dans `/account-pending`. Une seule, et elle est posée silencieusement sur la session. Plusieurs, et elle choisit.
5. **Le groupe**, en mode exclusif, quand l'organisation en compte plus d'un.

Une passkey répond aux étapes une à trois d'un coup : elle vaut les deux
facteurs, et mène directement à l'organisation.

## La règle du non-cumul des facteurs

L'autorité qui reconnaît le compte a le dernier mot sur le fait que Meerkat
réclame ou non son propre second facteur. Une autorité réglée sur *Two-factor:
left to the authority* signifie "ce fournisseur les a déjà mis au défi", et
Meerkat ne redemande rien.

Deux précisions qui comptent :

- C'est une **déclaration, pas une preuve**. Meerkat ne lit pas les claims `acr` ni `amr` qu'un fournisseur peut envoyer : rien ne vérifie que le fournisseur a réellement demandé quelque chose. Ne posez cette valeur que là où vous le savez.
- Les deux autres valeurs se comportent de la même façon : *Always required* sur une autorité **ne force pas** le second facteur. Ce qui décide, c'est le réglage du compte, puis celui de l'installation. Voir [Second facteur](/#/docs/auth/mfa).

Il n'y a pas de colonne "source" sur un compte. Les autorités qui connaissent
une personne sont enregistrées comme des liens - un par autorité et par sujet -
donc le même compte est atteignable par un mot de passe, un fournisseur et un
annuaire à la fois, et qui s'est inscrit localement garde tout en passant au SSO.

## Où se règle quoi

| Écran | Ce qui y vit |
|---|---|
| **Infra > Authentication** | les autorités, leurs paramètres et leurs politiques, et le défaut d'auto-inscription |
| **Infra > Mail relay** | le relais par lequel partent les courriels de confirmation, de réinitialisation et de second facteur |
| **Application > Security** | la politique de mot de passe, l'étranglement des tentatives, le second facteur, les navigateurs de confiance, les passkeys, les jetons d'API personnels, la durée de session |
| **Application > Built-in pages** | l'allure des pages de connexion |
| **Application > Users** | les comptes eux-mêmes, et leurs surcharges individuelles |
