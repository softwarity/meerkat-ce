---
title: Feuille de route
section: Le projet
order: 10
summary: Ce qui est construit, ce qui est en cours d'achèvement, et ce qui n'est délibérément pas sur la liste.
---

# Feuille de route

L'autorité sur l'état, c'est
[FEATURES.md](https://github.com/softwarity/meerkat-ce/blob/main/FEATURES.md) dans
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

- **Configurations versionnées** : plusieurs configurations coexistent, une
  seule est active, l'export et l'import ferment la boucle, et la passerelle
  pose un point de reprise à chaque changement qui déplace l'empreinte. Ce qui
  manque est de comparer deux configurations enregistrées sans passer par
  l'état courant.
- **Quotas** : ils se posent par route, par endpoint et par consommateur -
  utilisateur, jeton, organisation, adresse - et un dépassement répond 429 avec
  les en-têtes standards. Ce qui manque est l'écran qui montre la consommation,
  le ralentissement plutôt que le refus, et des compteurs justes en cluster.
- **Mode développement** : le tunnel marche, la connexion s'arrête sur une page
  qui nomme ce qui est substitué et par qui, et un bandeau le redit pendant
  qu'on travaille. Ce qui manque est la portée d'une substitution - elle vaut
  aujourd'hui pour tout le trafic - puis l'écran de console qui liste les
  sessions en cours, et l'audit.
- **Notifications** : le relais SMTP est livré, avec un gabarit unique aux
  couleurs du thème, et le résumé quotidien des accès qui se ferment part tout
  seul. Ce qui manque est un gabarit par événement, et traduit.
- **Identité vers l'amont** : le jeton signé est émis, avec son JWKS publié et
  la rotation des clés. Ce qui manque est un endpoint d'échange rendant un
  couple access/refresh, et les modes qui portent un secret vers l'amont :
  BASIC, FORM, JWT tiers.
- **Passkeys** : utilisables comme facteur. Récupérer un compte dont l'unique
  passkey est perdue n'est pas encore écrit.
- **Codes à usage unique par e-mail** : se connecter avec un code à la place
  du mot de passe est livré, éteint par défaut, lié au navigateur qui l'a
  demandé et jamais ouvert sur la console. Ce qui manque est le lien magique,
  et pouvoir fermer cette porte compte par compte.

## Ensuite

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
