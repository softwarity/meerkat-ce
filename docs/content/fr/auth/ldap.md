---
title: LDAP et Active Directory
section: Authentification
order: 106
summary: Interroger un annuaire directement, avec l'identifiant et le mot de passe tapés dans le formulaire de connexion ordinaire.
---

# LDAP et Active Directory

> [!NOTE] **Enterprise edition.**
> Les annuaires font partie de l'édition Enterprise. L'image communautaire refuse
> d'enregistrer une autorité LDAP (`403`), et le choix *Directory* est grisé dans
> la console.

Une autorité d'annuaire prend l'identifiant et le mot de passe du **formulaire de
connexion ordinaire** et interroge l'annuaire lui-même. Aucun bouton n'apparaît
sur la page de connexion : il n'y a nulle part où envoyer le navigateur.

L'ordre est : le mot de passe local d'abord, puis, s'il refuse, chaque annuaire
actif à son tour. Quelqu'un dont les mots de passe local et annuaire coïncident
n'entre par l'annuaire qu'une fois les comptes locaux désactivés.

> [!WARNING]
> Une ligne d'annuaire peut atterrir dans une image communautaire par une
> configuration importée, ou par une base d'abord écrite par une image Enterprise.
> Le binaire communautaire ne porte **aucun pilote d'annuaire** : une telle
> autorité n'authentifie personne - la seule trace est une ligne `directory
> unavailable` dans le journal, et la personne voit l'habituel "mauvais
> identifiant ou mot de passe". Si les comptes locaux sont désactivés en plus, le
> formulaire est quand même dessiné et plus personne n'entre.

## Les champs

**Infra > Authentication > New authority > Directory.**

| Champ | Ce qu'on y met |
|---|---|
| Dialect | *Directory (OpenLDAP and friends)* ou *Active Directory*. Il choisit les défauts ci-dessous |
| Server URL | le schéma se choisit, il ne se tape pas : `ldaps://dc1.acme.io:636` ou `ldap://...` |
| Search base | là où la recherche commence, `dc=acme,dc=io` |
| Service account | le DN qui effectue la **recherche**, jamais une connexion : `cn=meerkat,ou=services,dc=acme,dc=io`. Vide pour une recherche anonyme |
| Service password | son mot de passe, ou une référence au coffre |
| User filter | `%s` est remplacé par ce que la personne a tapé, échappé. Vide utilise le défaut du dialecte |
| Follow nested groups | actif par défaut : un groupe qui contient un groupe compte |
| Skip the certificate check | pour un annuaire auto-signé, et rien d'autre |

Ce qu'un dialecte décide, quand vous laissez le champ correspondant vide :

| | Directory | Active Directory |
|---|---|---|
| Filtre utilisateur | `(&(objectClass=inetOrgPerson)(uid=%s))` | `(&(objectClass=user)(sAMAccountName=%s))` |
| Attribut d'identifiant | `uid` | `sAMAccountName` |
| Attribut de nom | `cn` | `displayName` |
| Attribut d'adresse | `mail` | `mail` |
| Groupes lus par | une recherche sur `member` ou `uniqueMember` | l'attribut `memberOf`, puis une recherche d'appartenance imbriquée |

> [!NOTE]
> Les noms d'attributs, la base de recherche des groupes, le filtre de groupe et
> l'attribut de nom de groupe existent bien comme configuration (`usernameAttr`,
> `emailAttr`, `nameAttr`, `groupBaseDn`, `groupFilter`, `groupIdAttr`,
> `memberOfAttr`) mais n'ont pas de case dans la console : ils se posent par l'API
> d'administration ou par un fichier de configuration importé. Tout le reste est
> sur l'écran.

## Chercher, puis lier

C'est toute la séquence, et elle compte parce que le compte de service ne voit
jamais le mot de passe de personne :

1. un identifiant vide **ou un mot de passe vide** est refusé tout de suite : un mot de passe vide est un bind anonyme, et un bind anonyme réussit ;
2. connexion ;
3. bind avec le compte de service, s'il y en a un ;
4. recherche dans le sous-arbre depuis la base, avec le filtre utilisateur, deux entrées au plus. Aucune entrée veut dire "mauvais identifiant ou mot de passe". **Deux entrées ou plus et la connexion est refusée**, en nommant l'ambiguïté, plutôt que d'en choisir une ;
5. bind au nom de la personne, avec le DN trouvé et le mot de passe qu'elle a tapé. C'est le seul usage fait du mot de passe ;
6. re-bind avec le compte de service avant de lire les groupes ;
7. lecture de l'identité : le DN de l'entrée devient le sujet stable, plus identifiant, nom complet, adresse et groupes.

Une adresse qui vient d'un annuaire est traitée comme **vérifiée** : un annuaire
fait autorité sur ses propres gens. C'est ce qui permet à une connexion par
annuaire d'adopter un compte local existant portant la même adresse.

Les noms de groupes sont la feuille du DN, pas le DN :
`cn=developer,ou=groups,dc=acme,dc=io` est rapporté comme `developer`. Ils
deviennent des appartenances et des rôles par une règle de groupe posée sur une
organisation.

## TLS

TLS s'applique quand, et seulement quand, l'URL commence par `ldaps://`. La
version minimale est TLS 1.2. Il n'y a pas de StartTLS sur le port `389`, pas de
certificat client et pas d'autorité de certification propre : pour un annuaire au
certificat auto-signé, l'option honnête sur l'écran est *Skip the certificate
check*.

## Test the connection

Le bouton se connecte, lie le compte de service, et exécute votre filtre
utilisateur une fois contre un nom que personne ne porte. Il prouve donc l'URL,
le certificat, le compte de service, et que la base et le filtre s'analysent -
c'est-à-dire l'essentiel de ce qu'un premier réglage rate.

## Deux choses qu'un annuaire fait, et qu'une autorité par redirection ne peut pas

**Il revalide.** Quand quelqu'un se connecte par passkey, Meerkat demande à
l'annuaire s'il connaît toujours cette personne avant de la laisser entrer. Un
"aucune entrée de ce nom" franc révoque la connexion ; toute autre erreur ne
déconnecte personne, parce qu'un serveur momentanément injoignable n'est pas une
réponse.

**Il peut être la seule autorité.** Comptes locaux désactivés et annuaire actif,
le formulaire de connexion reste le chemin d'entrée : c'est le formulaire de
l'annuaire autant que celui des comptes locaux.
