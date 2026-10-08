---
title: La console
section: La console
order: 150
summary: À quoi sert la console d'administration, comment elle s'organise et où se trouve chaque écran.
---

# La console

La console est l'application d'administration de Meerkat. La gateway la sert
elle-même, sur le **port d'administration** (le plan de contrôle), depuis le même
binaire : vous n'avez rien d'autre à déployer, ni rien à garder aligné sur la
version qui achemine votre trafic.

C'est un outil d'exploitant, proposé en anglais uniquement.

![La console sur l'écran Users, avec le rail à gauche et, à côté, les sections du plan Application](img/console/users.webp)

Tout à gauche, le rail : les deux plans et les écrans transverses. Juste à côté,
les sections du plan où vous vous trouvez. Le reste de la largeur revient à
l'écran lui-même.

## Deux plans, et ce qui les traverse

Le rail de gauche regroupe les deux plans et les écrans transverses.

| Entrée du rail | URL | Ce qu'elle vous apprend |
|---|---|---|
| **Infra** | `/infra/...` | Le trajet des requêtes : routes, endpoints, autorités, TLS, relais de messagerie |
| **Application** | `/application/...` | Le produit que voient vos utilisateurs : identité, rôles, pages, portail, politiques |
| **Tenants** | `/tenants/:id/...` | Une organisation à la fois (en mode multi-organisation uniquement) |
| **Vault** | `/vault` | Toutes les valeurs et tous les secrets nommés auxquels la configuration fait référence |
| **Data plane** | `/data-plane/...` | Ce que font les applications : leurs connexions (Audit), qui y est connecté (Sessions), les appels planifiés qui leur sont adressés (Scheduler), ce que la gateway a servi (Metrics), ce que vos utilisateurs ont signalé (Issues) |
| **Meerkat** | `/system/...` | La gateway elle-même : qui a modifié quoi et qui s'est connecté à la console (Audit), ce qu'elle dit d'elle-même en direct (Logs), qui tient la console (Sessions), l'installation entière comme document ou comme base (Configuration) - et, en bas, la référence REST du plan de contrôle (API reference), ce qu'apporte cette version (Release notes) et l'édition (License) |

Ce découpage n'a rien de cosmétique. **Infra** concerne l'installation : un upstream,
un certificat, un serveur SMTP, un annuaire. **Application** concerne le produit
que sert cette installation : qui sont vos utilisateurs, ce qu'ils ont le droit de
faire, à quoi ressemble votre page de connexion. C'est souvent la même personne
qui s'occupe des deux, et elle détient alors les deux capacités, mais les
questions se posent à deux endroits distincts.

> [!NOTE]
> Meerkat se trouve à l'adresse `/system`, et non `/meerkat` : en dehors de
> `/api`, les chemins du plan de contrôle appartiennent au produit (`/meerkat/...`
> sert les scripts de la gateway), et la console évite donc de les occuper.

## Ce que vous voyez dépend de qui vous êtes

La console n'affiche que ce que vos capacités autorisent, et l'API
d'administration applique les mêmes périmètres à chaque appel.

| Capacité | Ce qu'elle ouvre |
|---|---|
| `root` | Tout, y compris Configuration |
| `infra admin` | Le plan Infra (avec Access tokens et MCP), Metrics, Logs, Audit et Issues, ainsi que le périmètre infra du coffre |
| `app admin` | Le plan Application (avec Access tokens), Sessions, Scheduler, Audit et Issues, ainsi que le périmètre applicatif du coffre |
| `tenant admin` | Les organisations que le compte administre, avec Sessions, Audit et Issues restreints à celles-ci |
| `tenant creator` | La création d'une organisation depuis le tiroir Tenants |
| `dev` | L'outillage développeur sur les applications servies ; ce n'est pas un écran de la console |

Une fois connecté, vous arrivez sur la première section qui vous est ouverte :
Infra, sur Routes, pour un administrateur infra ; Application, sur General, pour un
administrateur applicatif ; Tenants dans les autres cas (License lorsqu'il n'y a
qu'une seule organisation). Les capacités s'accordent compte par compte, sur
l'écran [Users](/docs/console/users).

## Les habitudes des écrans

Retenez ces cinq principes et la console ne vous surprendra plus.

- **Une liste, puis un tiroir à droite.** Un clic sur une ligne ouvre l'objet, et
  la liste reste où elle était. Sur Routes, Users, Roles, Authentication, Issues et
  Configuration, le tiroir figure dans l'URL : après un rechargement ou depuis un
  favori, vous retrouvez exactement ce qui était ouvert.
- **Les actions de ligne sont dans la dernière colonne** du tableau, et
  apparaissent sur la ligne que vous survolez.
- **L'enregistrement.** Certains écrans enregistrent dès le clic (un interrupteur,
  une case dans une matrice) ; d'autres ont un bouton Save et indiquent ce qui
  manque encore. Lorsque la différence compte, l'écran précise son fonctionnement.
- **Les fonctionnalités Enterprise sont signalées.** Sur l'image Enterprise,
  elles portent un petit badge `EE` (infobulle *Enterprise edition feature*). Sur
  l'image Community, elles sont verrouillées et grisées, avec un badge
  `Enterprise` qui explique ce qu'elles apportent et renvoie vers License. Rien
  n'est masqué.
- **Les secrets passent par le coffre.** Un champ sensible propose de ranger sa
  valeur dans le [coffre](/docs/console/vault) et refuse de l'enregistrer sous
  forme littérale.

## Les écrans, par groupe

### Infra

- **[Routes](/docs/console/routes)** - la table de routage, dans l'ordre, et l'éditeur de route.
- **[Endpoint security et Endpoint rate limits](/docs/console/endpoints)** - opération par opération, à partir de la spécification OpenAPI d'une route.
- **[Endpoint audit](/docs/operations/audit#auditer-les-oprations-dune-route)** - les opérations consignées dans le journal d'audit.
- **[Authentication](/docs/console/authentication)** - les autorités auprès desquelles on peut se connecter.
- **[Mail relay](/docs/console/mail-relay)** - le serveur SMTP et le récapitulatif quotidien.
- **[TLS](/docs/console/tls)** - un nom, un certificat, et ACME.
- **[OpenTelemetry](/docs/operations/tracing)** - les traces, les métriques, l'audit et les journaux envoyés à votre collecteur.
- **[Plug](/docs/operations/plug)** - le tunnel développeur.
- **[Access tokens, MCP et API](/docs/console/access-and-agents)** - piloter Meerkat sans navigateur.
- **Model** - les champs que porte un compte, documentés avec [Users](/docs/console/users).

### Application

- **[General et Security](/docs/console/application)** - ce qu'est cette installation, et ses politiques.
- **[Roles](/docs/console/roles)** - le catalogue global des rôles.
- **[Users](/docs/console/users)** - les comptes, leurs capacités et leurs champs.
- **[Groups, Members et Group rules](/docs/console/organisation)** - qui appartient à quel groupe.
- **[Built-in pages](/docs/console/built-in-pages)** - le thème, la disposition et la marque des pages que sert la gateway.
- **[Portal](/docs/console/portal)** - la barre de navigation qui habille les applications exposées par la gateway.
- **[Sessions](/docs/auth/sessions#voir-les-sessions)** - qui est connecté, et comment fermer une session.
- **[Access tokens](/docs/console/access-and-agents)** - les jetons de la console, et les jetons d'application de tous les comptes.

### Communs aux deux plans

- **[Tenants](/docs/console/tenants)** - l'administration propre à une organisation.
- **[Vault](/docs/console/vault)** - les secrets et les valeurs, référencés sous la forme `$name`.
- **[Metrics](/docs/console/traffic)** - le trafic, la latence, le classement des routes.
- **[Scheduler](/docs/operations/scheduler)** - les appels planifiés et leurs exécutions.
- **[Audit et Issues](/docs/console/audit-and-issues)** - l'historique des modifications, et les signalements.
- **[Logs](/docs/operations/logs#dans-la-console)** - les lignes de journal de la gateway, en direct, et son niveau de journalisation. Un seul nœud à la fois.
- **[Configuration](/docs/console/configuration)** (sous Meerkat, root uniquement) - les configurations, les points de reprise, les snapshots et la migration : l'installation entière, qui couvre les deux plans.
- **License** (sous Meerkat, ouverte à tous) - l'édition qui a répondu, et chaque fonctionnalité Enterprise : ce
  qu'elle fait, son degré d'avancement et l'écran où elle se trouve, ou le fait
  qu'elle n'en a pas (le cluster actif/actif, les fichiers de déploiement). La
  liste est lue dans le contrat de fonctionnalités du produit lui-même : elle ne
  peut donc pas s'en écarter. C'est le seul écran qui parle d'éditions : partout
  ailleurs, un contrôle verrouillé porte son badge et renvoie ici.

![L'écran License : l'édition, puis chaque fonctionnalité Enterprise avec son degré d'avancement et l'écran où elle se trouve](img/console/license.webp)

## Votre propre compte

Le bas du rail, c'est vous. La ligne à votre nom ouvre `/profile` : ce sont les
pages de profil de la gateway, celles-là mêmes que voient vos utilisateurs,
avec la photo, le mot de passe, le second facteur, les passkeys et les jetons
d'API personnels. La déconnexion est juste en dessous ; elle déconnecte d'un coup
tous les onglets de la console.

## Version et notes de version

![Les notes de version, ouvertes sur ce qu'apporte la prochaine version](img/console/release-notes.webp)

**Meerkat, Release notes** indique dans son titre la version en service et son
édition : **Meerkat 1.0.1 EE** (ou **CE**). En dessous, les notes de version, de la
plus récente à la plus ancienne, depuis cette version jusqu'à sa version mineure : c'est le même
texte que celui de la version publiée sur GitHub. Un build de développement
affiche la dernière version publiée qu'il contient, précédée de ce qui arrive sous
**Next release** ; une section vide indique *Missing information*.

Juste en dessous, **License** ouvre l'écran de l'édition : chaque fonctionnalité
Enterprise, son degré d'avancement et l'endroit où elle se trouve. Ces deux écrans
sont ouverts à quiconque tient la console.
