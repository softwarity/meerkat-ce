---
title: La console
section: La console
order: 150
summary: A quoi sert la console d'administration, comment elle est organisée, et où se trouve chaque écran.
---

# La console

La console est l'application d'administration de Meerkat. Elle est servie par la
passerelle elle-même, sur le **port d'administration** (le plan de contrôle), et
c'est le même binaire : il n'y a rien de plus à déployer, et rien à maintenir en
phase avec la version qui route votre trafic.

C'est un outil d'exploitant, servi en anglais.

![La console sur l'écran Users, avec le rail à gauche et les sections du plan Application à côté](img/console/users.webp)

Tout à gauche, le rail : les deux plans et les écrans transverses. A côté, les
sections du plan dans lequel on se trouve. Le reste de la largeur est l'écran
lui-même.

## Deux plans, et ce qui les traverse

Le rail de gauche porte les deux plans et les écrans transverses.

| Entrée du rail | URL | La question à laquelle il répond |
|---|---|---|
| **Infra** | `/infra/...` | Comment les requêtes circulent : routes, endpoints, autorités, TLS, relais, configuration |
| **Application** | `/application/...` | Le produit que vos utilisateurs voient : identité, rôles, pages, portail, politiques |
| **Tenants** | `/tenants/:id/...` | Une organisation à la fois (mode multi-organisations seulement) |
| **API** | `/api` | La référence REST du plan de contrôle, essayée avec votre session |
| **Vault** | `/vault` | Toutes les valeurs et les secrets que la configuration désigne |
| **Metrics** | `/traffic` | Ce que la passerelle a réellement servi |
| **Audit** | `/audit` | Qui a changé quoi |
| **Issues** | `/issues` | Ce que vos utilisateurs ont signalé |

Le découpage n'est pas cosmétique. **Infra** parle de l'installation : un amont, un
certificat, un serveur SMTP, un annuaire. **Application** parle du produit que
cette installation sert : qui sont vos utilisateurs, ce qu'ils peuvent faire, à
quoi ressemble votre page de connexion. C'est souvent la même personne, avec les
deux capacités, mais ce sont deux questions posées à deux endroits.

> [!NOTE]
> Metrics vit sur `/traffic`, pas sur `/metrics` : ce chemin appartient à
> l'exposition Prometheus servie sur le même port, donc la console ne pouvait pas
> le prendre.

## Ce que vous voyez dépend de qui vous êtes

La console montre ce que vos capacités autorisent, et l'API d'administration
applique les mêmes périmètres à chaque appel.

| Capacité | Ouvre |
|---|---|
| `root` | Tout, y compris Access tokens, MCP et Configuration |
| `infra admin` | Le plan Infra, Metrics, la portée infra du coffre |
| `app admin` | Le plan Application, la portée applicative du coffre |
| `tenant admin` | Les organisations qu'il administre, Audit et Issues limités à elles |
| `tenant creator` | La création d'une organisation depuis le tiroir Tenants |
| `dev` | L'outillage développeur sur les applications servies, pas un écran de console |

La connexion dépose chacun sur la première section qu'il peut utiliser : Infra sur
Routes pour un infra admin, Application sur General pour un app admin, Tenants
sinon. Les capacités se donnent par compte sur [Users](/#/docs/console/users).

## Les habitudes des écrans

Ces cinq-là retenues, la console ne surprend plus.

- **Une liste, puis un tiroir à droite.** Cliquer une ligne ouvre l'objet ; la
  liste ne bouge pas. Sur Routes, Users, Roles, Authentication, Issues et
  Configuration, le tiroir est dans l'URL : un rafraîchissement ou un marque-page
  revient exactement sur ce qui était ouvert.
- **Les actions de ligne vivent dans la dernière colonne** du tableau, et
  apparaissent sur la ligne survolée.
- **L'enregistrement.** Certains écrans écrivent au clic (un interrupteur, une case
  dans une matrice), d'autres ont un bouton Save et disent ce qui manque encore. Là
  où cela compte, l'écran le précise.
- **Les contrôles Enterprise restent visibles.** Un contrôle que cette image ne
  peut pas honorer est grisé et porte une pastille `[Enterprise]` qui explique ce
  qu'il apporte, avec un lien vers l'écran License. Rien n'est caché.
- **Les secrets passent par le coffre.** Un champ sensible propose de ranger sa
  valeur dans le [coffre](/#/docs/console/vault) et refuse d'être enregistré en
  littéral.

## Les écrans, par groupe

### Infra

- **[Routes](/#/docs/console/routes)** - la table de routage, dans l'ordre, et l'éditeur de route.
- **[Sécurité et quotas par endpoint](/#/docs/console/endpoints)** - par opération, depuis la spec OpenAPI d'une route.
- **[Authentification](/#/docs/console/authentication)** - les autorités par lesquelles on se connecte.
- **[Relais mail](/#/docs/console/mail-relay)** - le serveur SMTP, et le digest quotidien.
- **[TLS](/#/docs/console/tls)** - un nom, un certificat, et ACME.
- **[Jetons d'accès, MCP et API](/#/docs/console/access-and-agents)** - piloter Meerkat sans navigateur.
- **[Configuration](/#/docs/console/configuration)** - configurations, points de reprise, instantanés.
- **Model** - les champs que porte un compte, documenté avec [Users](/#/docs/console/users).

### Application

- **[General, Locales et Security](/#/docs/console/application)** - ce que cette installation est, et ses politiques.
- **[Roles](/#/docs/console/roles)** - le catalogue global des rôles.
- **[Users](/#/docs/console/users)** - les comptes, leurs capacités, leurs champs.
- **[Groups, Members et Group rules](/#/docs/console/organisation)** - qui est dans quel groupe.
- **[Built-in pages](/#/docs/console/built-in-pages)** - thème, disposition et identité des pages servies.
- **[Portal](/#/docs/console/portal)** - la barre de navigation que portent les applications proxifiées.

### A travers les deux

- **[Tenants](/#/docs/console/tenants)** - l'administration propre d'une organisation.
- **[Vault](/#/docs/console/vault)** - secrets et valeurs, référencés par `$nom`.
- **[Metrics](/#/docs/console/traffic)** - trafic, latences, classement des routes.
- **[Audit et Issues](/#/docs/console/audit-and-issues)** - la trace des changements, et les signalements.
- **License** - quelle édition a répondu, et ce qu'achète chaque fonctionnalité
  Enterprise. C'est le seul écran qui parle d'éditions : partout ailleurs, un
  contrôle verrouillé porte sa pastille et renvoie ici.

## Votre propre compte

Le bas du rail, c'est vous. La ligne à votre nom ouvre `/profile` : les pages de
profil de la passerelle, celles-là mêmes que vos utilisateurs obtiennent - photo,
mot de passe, second facteur, passkeys, jetons d'API personnels. La déconnexion est
en dessous, et elle déconnecte tous les onglets de la console d'un coup.
