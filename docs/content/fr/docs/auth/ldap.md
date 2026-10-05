---
title: LDAP et Active Directory
section: Authentification
order: 106
summary: Interroger directement un annuaire, avec l'identifiant et le mot de passe saisis dans le formulaire de connexion habituel.
---

# LDAP et Active Directory

> [!NOTE] **Édition Enterprise.**
> Les annuaires font partie de l'édition Enterprise. L'image Community refuse
> d'enregistrer une autorité LDAP (`403`), et le choix *Directory* est grisé dans la
> console.

Une autorité de type annuaire reprend l'identifiant et le mot de passe saisis dans le
**formulaire de connexion habituel** et interroge elle-même l'annuaire. Aucun bouton
n'apparaît sur la page de connexion : il n'y a nulle part où envoyer le navigateur.

L'ordre est le suivant : le mot de passe local d'abord, puis, s'il est refusé, chaque
annuaire activé à tour de rôle. Une personne dont le mot de passe local et celui de
l'annuaire se trouvent être identiques n'entre par l'annuaire qu'une fois les comptes
locaux désactivés.

> [!WARNING]
> Une ligne d'annuaire peut arriver dans une image Community par une configuration
> importée, ou par une base de données d'abord écrite par une image Enterprise. Le binaire
> Community ne contient **aucun pilote d'annuaire** : une telle autorité n'authentifie donc
> personne. La seule trace est une ligne `directory unavailable` dans le journal, et la
> personne voit le message habituel "identifiant ou mot de passe invalide". Si les comptes
> locaux sont eux aussi désactivés, le formulaire de connexion s'affiche toujours et plus
> personne ne peut entrer.

## Les champs

**Infra > Authentication > New authority > Directory.**

| Champ | Ce qu'il faut y mettre |
|---|---|
| Dialect | *Directory (OpenLDAP and friends)* ou *Active Directory*. Il fixe les valeurs par défaut ci-dessous |
| Server URL | le schéma se choisit, il ne se saisit pas : `ldaps://dc1.acme.io:636` ou `ldap://...` |
| Search base | le point de départ de la recherche, `dc=acme,dc=io` |
| Service account | le DN qui effectue la **recherche**, jamais une connexion : `cn=meerkat,ou=services,dc=acme,dc=io`. Laissez-le vide pour une recherche anonyme |
| Service password | son mot de passe, ou une référence au coffre |
| User filter | `%s` est remplacé par ce que la personne a saisi, après échappement. Vide, c'est la valeur par défaut du dialecte qui s'applique |
| Follow nested groups | activé par défaut : un groupe contenu dans un groupe est pris en compte |
| Skip the certificate check | uniquement pour un annuaire au certificat auto-signé |

Ce que décide un dialecte quand vous laissez vide le champ correspondant :

| | Directory | Active Directory |
|---|---|---|
| Filtre utilisateur | `(&(objectClass=inetOrgPerson)(uid=%s))` | `(&(objectClass=user)(sAMAccountName=%s))` |
| Attribut de l'identifiant | `uid` | `sAMAccountName` |
| Attribut du nom | `cn` | `displayName` |
| Attribut de l'e-mail | `mail` | `mail` |
| Lecture des groupes | une recherche sur `member` ou `uniqueMember` | l'attribut `memberOf`, puis une recherche des appartenances imbriquées |

> [!NOTE]
> Les noms d'attributs, la base de recherche des groupes, le filtre des groupes et
> l'attribut du nom de groupe existent tous dans la configuration (`usernameAttr`,
> `emailAttr`, `nameAttr`, `groupBaseDn`, `groupFilter`, `groupIdAttr`, `memberOfAttr`),
> mais n'ont pas de champ dans la console : ils se règlent par l'API d'administration ou
> par un fichier de configuration importé. Tout le reste est à l'écran.

## Rechercher, puis s'authentifier

La séquence se résume à cela, et elle a son importance : le compte de service ne voit
jamais le mot de passe de qui que ce soit.

1. un identifiant vide **ou un mot de passe vide** est refusé d'emblée - un mot de passe vide équivaut à un bind anonyme, et un bind anonyme réussit ;
2. connexion au serveur ;
3. bind avec le compte de service, s'il y en a un de configuré ;
4. recherche dans le sous-arbre, à partir de la base de recherche et avec le filtre utilisateur, limitée à deux entrées. Aucune entrée signifie "identifiant ou mot de passe invalide". **À partir de deux entrées, la connexion est refusée** en signalant l'ambiguïté, plutôt que d'en choisir une ;
5. bind au nom de la personne, avec le DN qui vient d'être trouvé et le mot de passe qu'elle a saisi. C'est le seul usage qui est fait de ce mot de passe ;
6. nouveau bind avec le compte de service avant de lire les groupes ;
7. lecture de l'identité : le DN de l'entrée devient le sujet stable, auquel s'ajoutent l'identifiant, le nom complet, l'e-mail et les groupes.

Une adresse fournie par un annuaire est considérée comme **vérifiée** : un annuaire fait
autorité sur ses propres membres. C'est ce qui permet à une connexion par annuaire de
reprendre un compte local existant qui porte la même adresse.

Le nom d'un groupe est la feuille du DN, pas le DN entier :
`cn=developer,ou=groups,dc=acme,dc=io` est rapporté sous le nom `developer`. Les groupes
deviennent des appartenances et des rôles par l'intermédiaire d'une règle de groupe définie
sur une organisation.

## TLS

TLS s'applique quand l'URL commence par `ldaps://`, et seulement dans ce cas. La version
minimale est TLS 1.2. Il n'y a ni StartTLS sur le port `389`, ni certificat client, ni
autorité de certification personnalisée : pour un annuaire au certificat auto-signé, la
seule option que propose l'écran, sans détour, est *Skip the certificate check*.

## Test the connection

Ce bouton se connecte, s'authentifie avec le compte de service et exécute une fois votre
filtre utilisateur sur un nom que personne ne porte. Il valide donc l'URL, le certificat et
le compte de service, et confirme que la base de recherche et le filtre sont syntaxiquement
corrects - soit l'essentiel de ce qui pose problème lors d'une première configuration.

## Deux choses que fait un annuaire, et pas une autorité par redirection

**Il revalide.** Quand une personne se connecte avec une passkey, Meerkat demande à
l'annuaire s'il la connaît toujours avant de la laisser passer. Une réponse claire "cet
objet n'existe pas" annule la connexion ; toute autre erreur signifie "impossible de poser
la question", et personne n'est déconnecté parce qu'un serveur a été momentanément
injoignable. Les fournisseurs d'identité atteints par redirection n'offrent aucun moyen de
poser cette question : ils n'annulent donc jamais une connexion par passkey.

**Il peut être la seule autorité.** Quand les comptes locaux sont désactivés et qu'un
annuaire est activé, le formulaire de connexion reste le moyen d'entrer : c'est le
formulaire de l'annuaire tout autant que celui des comptes locaux.
