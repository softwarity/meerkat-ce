---
title: Feuille de route
section: Le projet
order: 10
summary: Ce qui est construit, ce qui est en cours d'achèvement, et ce qui n'est délibérément pas sur la liste.
---

# Feuille de route

L'autorité sur l'état, c'est
[FEATURES.md](https://github.com/softwarity/meerkat/blob/main/FEATURES.md) dans
le dépôt : une ligne par fonctionnalité, l'état lu dans le code, et une case
cochée dans le commit qui livre la chose. Cette page est ce tableau lu à voix
haute.

## Construit et utilisé

Le chemin critique est entier. Une requête arrive, une route la reconnaît, des
filtres la transforment, une règle d'accès décide, et ce qui s'est passé se voit
ensuite.

- **Routage** : le catalogue de prédicats et de filtres, édité à chaud, appliqué
  à la requête suivante. Voir les [prédicats](/docs/predicates/overview) et les
  [filtres](/docs/filters/overview).
- **Identité** : comptes locaux, OpenID Connect, LDAP et Active Directory,
  GitHub, tous testés contre de vrais serveurs. Sessions, jetons d'API, jetons
  signés vers l'amont.
- **Accès** : un catalogue de rôles hiérarchique, des groupes par organisation,
  les organisations elles-mêmes, une règle par route, et une sécurité par
  endpoint lue dans la description OpenAPI d'un service.
- **Second facteur** : TOTP avec navigateurs de confiance, et passkeys.
- **Le coffre** : des secrets scellés au repos et des valeurs en clair, les deux
  référencés par leur nom.
- **TLS** : certificats et émission ACME, sérialisée pour qu'un cluster demande
  une seule fois.
- **Audit** : chaque changement d'administration avec son auteur et un diff
  champ par champ.
- **La console** : toute l'administration, sur son propre port, avec le partage
  de capacités qui décide qui voit quelle moitié.
- **L'endpoint agent** : MCP sur le plan de contrôle, sous les mêmes règles
  qu'un humain.
- **Le portail de navigation** : une barre unique au travers des applications
  que la passerelle sert, injectée dans des pages qui n'embarquent aucune
  bibliothèque pour cela.
- **Le cluster** : plusieurs passerelles derrière un PostgreSQL, qui se
  coordonnent par la base plutôt qu'entre elles.

## En cours d'achèvement

Ces sujets marchent et ne sont pas finis. Le tableau du dépôt dit, ligne par
ligne, ce qui manque à chacun.

- **Configurations versionnées** : dupliquer, éditer en brouillon, comparer,
  basculer atomiquement, revenir en arrière. Le stockage est là ; les écrans
  sont à moitié construits.
- **Quotas** : par consommateur, avec un mode calibration qui ne fait que
  journaliser. Les limites de débit sont livrées ; les quotas non.
- **Mode développement** : le tunnel marche. Ce qui manque, c'est ce qui le
  montre : la page qui nomme ce qui est substitué, et l'écran de console qui
  liste les sessions en cours.
- **Notifications** : le relais de messagerie est livré. Les gabarits d'e-mail
  multilingues, et le digest quotidien, ne le sont pas.
- **Identité vers l'amont** : le jeton signé est émis. Un endpoint d'échange
  rendant un couple access/refresh n'est pas écrit.
- **Passkeys** : utilisables comme facteur. Récupérer un compte dont l'unique
  passkey est perdue n'est pas encore écrit.
- **Codes à usage unique par e-mail** : rien n'est écrit ; SMTP est le prérequis
  et il est en place.

## Ensuite

- **Un portail par organisation** : la barre de navigation est globale
  aujourd'hui. Laisser une organisation porter son icône, son titre et son
  arrangement serait le premier remplacement visuel par organisation du produit,
  et cela attend que la décision soit prise correctement plutôt que par
  accident.
- **Un assistant de découverte** : la passerelle sait déjà lire le socket Docker
  et un namespace Kubernetes. Transformer cela en « scanner, choisir un
  conteneur, obtenir une route » est l'écran qui manque.
- **SAML**, pour les entreprises dont le fournisseur d'identité ne parle pas
  OpenID Connect. Il est enregistrable aujourd'hui et refuse à la fabrique, ce
  qui est honnête et pas encore utile.

## Pas sur la liste

Kerberos et SPNEGO sont notés comme Enterprise, et rien n'est commencé. Un
système de plugins n'est pas prévu : le catalogue de filtres est trié exprès, et
chaque filtre qui y figure est un filtre que nous savons expliquer et tester.

> [!TIP]
> Ce que la suite d'intégration impose vraiment est sur la page
> [couverture de tests](/project/tests) : c'est le fichier que la suite exécute,
> pas une description de celui-ci.
