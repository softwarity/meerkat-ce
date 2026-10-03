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
| **Scheduler** | `/scheduler` | Les appels que la passerelle fait à vos services selon un planning |
| **Audit** | `/audit` | Qui a changé quoi |
| **Logs** | `/logs` | Ce que la gateway dit d'elle-même, en direct |
| **Issues** | `/issues` | Ce que vos utilisateurs ont signalé |

Le découpage n'est pas cosmétique. **Infra** parle de l'installation : un amont, un
certificat, un serveur SMTP, un annuaire. **Application** parle du produit que
cette installation sert : qui sont vos utilisateurs, ce qu'ils peuvent faire, à
quoi ressemble votre page de connexion. C'est souvent la même personne, avec les
deux capacités, mais ce sont deux questions posées à deux endroits.

> [!NOTE]
> Metrics vit sur `/traffic`, pas sur `/metrics` : hors `/api`, les chemins du
> plan de contrôle appartiennent au produit, donc la console s'en tient à l'écart.

## Ce que vous voyez dépend de qui vous êtes

La console montre ce que vos capacités autorisent, et l'API d'administration
applique les mêmes périmètres à chaque appel.

| Capacité | Ouvre |
|---|---|
| `root` | Tout, y compris Configuration |
| `infra admin` | Le plan Infra (avec Access tokens et MCP), Metrics, Logs, Audit et Issues, la portée infra du coffre |
| `app admin` | Le plan Application (avec Sessions et Access tokens), Scheduler, Audit et Issues, la portée applicative du coffre |
| `tenant admin` | Les organisations qu'il administre, Audit et Issues limités à elles |
| `tenant creator` | La création d'une organisation depuis le tiroir Tenants |
| `dev` | L'outillage développeur sur les applications servies, pas un écran de console |

La connexion dépose chacun sur la première section qu'il peut utiliser : Infra sur
Routes pour un infra admin, Application sur General pour un app admin, Tenants
sinon. Les capacités se donnent par compte sur [Users](/docs/console/users).

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
- **Les fonctionnalités Enterprise sont marquées.** Sur l'image Enterprise,
  elles portent une petite pastille `EE` (infobulle *Enterprise edition
  feature*). Sur l'image communautaire, elles sont verrouillées et grisées, avec
  une pastille `Enterprise` qui dit ce qu'elles apportent et renvoie vers
  License. Rien n'est caché.
- **Les secrets passent par le coffre.** Un champ sensible propose de ranger sa
  valeur dans le [coffre](/docs/console/vault) et refuse d'être enregistré en
  littéral.

## Les écrans, par groupe

### Infra

- **[Routes](/docs/console/routes)** - la table de routage, dans l'ordre, et l'éditeur de route.
- **[Sécurité et quotas par endpoint](/docs/console/endpoints)** - par opération, depuis la spec OpenAPI d'une route.
- **[Endpoint audit](/docs/operations/audit#auditer-les-oprations-dune-route)** - les opérations gardées dans le journal d'audit.
- **[Authentification](/docs/console/authentication)** - les autorités par lesquelles on se connecte.
- **[Relais mail](/docs/console/mail-relay)** - le serveur SMTP, et le digest quotidien.
- **[TLS](/docs/console/tls)** - un nom, un certificat, et ACME.
- **[OpenTelemetry](/docs/operations/tracing)** - traces, métriques, audit et journaux envoyés à votre collecteur.
- **[Plug](/docs/operations/plug)** - le tunnel développeur.
- **[Jetons d'accès, MCP et API](/docs/console/access-and-agents)** - piloter Meerkat sans navigateur.
- **Model** - les champs que porte un compte, documenté avec [Users](/docs/console/users).
- **[Configuration](/docs/console/configuration)** - configurations, points de reprise, instantanés. En bas du menu, à l'écart des écrans du quotidien.

### Application

- **[Général et Security](/docs/console/application)** - ce que cette installation est, et ses politiques.
- **[Roles](/docs/console/roles)** - le catalogue global des rôles.
- **[Users](/docs/console/users)** - les comptes, leurs capacités, leurs champs.
- **[Groups, Members et Group rules](/docs/console/organisation)** - qui est dans quel groupe.
- **[Built-in pages](/docs/console/built-in-pages)** - thème, disposition et identité des pages servies.
- **[Portal](/docs/console/portal)** - la barre de navigation que portent les applications proxifiées.
- **[Sessions](/docs/auth/sessions#voir-les-sessions)** - qui est connecté, et déconnecter une session.
- **[Access tokens](/docs/console/access-and-agents)** - les jetons de console, et les jetons d'application de tout le monde.

### A travers les deux

- **[Tenants](/docs/console/tenants)** - l'administration propre d'une organisation.
- **[Vault](/docs/console/vault)** - secrets et valeurs, référencés par `$nom`.
- **[Metrics](/docs/console/traffic)** - trafic, latences, classement des routes.
- **[Scheduler](/docs/operations/scheduler)** - les appels planifiés et leurs exécutions.
- **[Audit et Issues](/docs/console/audit-and-issues)** - la trace des changements, et les signalements.
- **[Logs](/docs/operations/logs#dans-la-console)** - les lignes de la gateway, en direct, et son niveau. Un nœud à la fois.
- **License** - quelle édition a répondu, et chaque fonction Enterprise : ce
  qu'elle fait, où elle en est, et l'écran où elle vit, ou qu'elle n'en a pas
  (le cluster actif/actif, les fichiers de déploiement). La liste est lue dans le
  contrat des fonctions du produit, donc elle ne peut pas s'en écarter. C'est le
  seul écran qui parle d'éditions : partout ailleurs, un contrôle verrouillé porte
  sa pastille et renvoie ici.

![L'écran License : l'édition, puis chaque fonction Enterprise avec où elle en est et l'écran où elle vit](img/console/license.webp)

## Votre propre compte

Le bas du rail, c'est vous. La ligne à votre nom ouvre `/profile` : les pages de
profil de la passerelle, celles-là mêmes que vos utilisateurs obtiennent - photo,
mot de passe, second facteur, passkeys, jetons d'API personnels. La déconnexion est
en dessous, et elle déconnecte tous les onglets de la console d'un coup.

### Version et notes de version

![La fenêtre des notes de version : Meerkat 1.0.1, les notes du correctif, puis 1.0.0](img/console/release-notes.webp)

Le menu du compte dit quelle version tourne, et quelle édition : **Meerkat 1.0.1 EE**
(ou **CE**). Un clic ouvre
les notes de version, de la plus récente à la plus ancienne, de cette version
jusqu'à sa version mineure - le même texte que la release GitHub. Une build de
développement affiche la dernière version qu'elle porte, avec ce qui arrive sous
**Next release** en tête ; une section vide indique *Missing information*.

Juste en dessous, **License** ouvre l'écran de l'édition : chaque fonction
Enterprise, où elle en est, et où elle vit.
