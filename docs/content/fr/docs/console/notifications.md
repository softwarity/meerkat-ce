---
title: Notifications
section: La console
order: 183
summary: Activer les notifications à trois niveaux, qui peut en publier, où les utilisateurs les lisent, ce qui a été envoyé à un tenant et qui l'a acquitté - et un tiroir pour en envoyer une vous-même.
---

# Notifications

Les services placés derrière la gateway préviennent leurs utilisateurs, qui
l'apprennent dans l'application qu'ils ont ouverte. Cette page en est le côté
administrateur : l'activer, laisser les services publier, choisir où les
utilisateurs lisent, et suivre ce qui a été envoyé. Ce qu'envoie un service, et
ce que voient ses utilisateurs, est décrit dans
[Notifier vos utilisateurs](/docs/operations/notifications). Édition Enterprise.

## Les activer

Trois niveaux, et chacun ne peut que retirer : un tenant reçoit des
notifications quand les trois l'autorisent.

| Niveau | Où | Ce qu'il décide |
|---|---|---|
| root | **Data plane, Notifications, Settings** | l'interrupteur de toute la gateway, coupé à l'installation ; puis tenant par tenant ; le plafond d'audience |
| administrateur de l'application | le même tiroir | tenant par tenant |
| administrateur du tenant (le propriétaire, ou un membre ADMIN) | la section **Notifications** du tenant | son propre tenant |

![Le tiroir Settings de Data plane, Notifications : l'interrupteur de la gateway, une case par tenant, et le plafond d'audience](img/console/notifications-settings.webp)

Dans le tiroir, chaque tenant est une case, lue depuis votre niveau :

- **cochée** : vous l'autorisez, et il les reçoit ;
- **vide** : vous l'avez coupé ;
- **un tiret** : vous l'autorisez, et un niveau en dessous l'a coupé - l'info-bulle
  dit lequel. Un clic sur le tiret le réactive et efface ce qu'ont décidé les
  niveaux du dessous ; ils pourront le couper de nouveau plus tard ;
- **grisée** : un niveau au-dessus l'a coupé, et seul ce niveau le réactive.

Un nouveau tenant est autorisé à tous les niveaux. Sur une gateway à un seul
tenant, le niveau du tenant n'existe pas : root et l'administrateur de
l'application décident.

Une publication vers un tenant coupé est refusée, et le refus nomme le niveau
qui l'a coupé. Une personne voit la page des notifications, son entrée de menu
et son badge dès qu'un de ses tenants reçoit des notifications.

Le **plafond d'audience**, fixé par root, borne le nombre de personnes qu'une
notification peut toucher : 5000 sauf réglage. Au-delà, la publication est
refusée.

## Data plane, Notifications

Root et l'administrateur de l'application suivent ici ce que chaque service a
envoyé, les plus récentes d'abord : le titre et son niveau, l'émetteur, le
tenant, combien de destinataires l'ont lue, et quand elle est partie. Une
notification retirée, remplacée ou expirée reste dans la liste, marquée comme
telle ; un exercice porte l'étiquette EXERCISE devant son titre.

![Data plane, Notifications : les notifications envoyées, avec leur émetteur, leur tenant et leurs lectures, et les boutons Settings, API et Access](img/console/notifications.webp)

Un clic en ouvre une : son texte, son émetteur, son id (celui que l'émetteur
connaît) et son tenant, l'adresse de son
détail, chaque action avec sa méthode et son adresse, ses métadonnées, quand
elle est partie et quand elle expire, et chaque destinataire avec l'heure de sa
lecture et, quand elle est partie avec mail, où en est son mail (à envoyer,
accepté par le relais, refusé avec le code du relais, inconnu, annulé, sans
adresse). **Withdraw** retire une notification active : chaque lecteur la voit
aussitôt devenir la rangée *Cette notification a été retirée*, hors de son
compte, et la liste de qui l'a lue est gardée.

Trois boutons occupent l'en-tête :

- **Settings** - le tiroir ci-dessus.
- **API** - *Publishing notifications*, pour le développeur d'un service, en
  sept volets : les endpoints et le jeton, le message champ par champ, tous les
  tenants d'un coup, les réponses, retrouver et retirer, remplacer et retirer
  par son id (y compris des appels arrivés dans le désordre), et les
  acquittements avec l'appel que fait `onAck`.
- **Access** - [qui peut publier](#qui-peut-publier).

![Le panneau API, Publishing notifications : les endpoints et le jeton, puis le message champ par champ](img/console/notifications-api.webp)

## Qui peut publier

Les services publient sur le plan interne, `POST :7070/api/notifications`. Le
tiroir **Access** décide de ce qu'un appelant doit présenter, exactement comme
celui du scheduler ([qui peut l'appeler](/docs/operations/scheduler#qui-peut-lappeler)) :
aucun justificatif, un jeton Meerkat, ou le jeton de ServiceAccount du pod sur
Kubernetes, puis tout le monde ou seulement les appelants listés. Chaque API a
son propre réglage : les services qui notifient ne sont pas toujours ceux qui
planifient.

Une nouvelle installation demande un **jeton Meerkat** : un jeton interne de
périmètre `notifications`, émis sous **Infra, Access tokens, Internal** (**Mint
a token**, dans le tiroir, y mène). Il publie des notifications et n'ouvre rien
d'autre. Changer l'accès revient à root ou à un administrateur de
l'infrastructure ; l'administrateur de l'application le lit.

## Où les utilisateurs les lisent

Quand root a activé les notifications, **Application, Portal** montre une carte
**Notifications page** :

- **Built-in** - `/meerkat/notifications`, servie par la gateway au thème des
  pages de connexion ;
- **Your page** - une page de l'une de vos applications : une route UI, et un
  chemin dessous comme `/#/inbox`. Elle est enregistrée dès qu'une route est
  choisie.

La barre, le menu du compte, le snackbar et les mails mènent à cette page. Une
personne qui ne peut pas ouvrir sa route reçoit la page intégrée, qui reste
ouverte à tous.

**Une entrée, un badge.** En mode Portal, **Add notifications** place une entrée
**Notifications** dans la barre. Elle se place, se déplace et se retire comme les
autres, et un libellé vide se lit Notifications dans la langue de chacun. C'est
elle qui porte l'unique badge des non lues. Sans elle - en mode Links ou None,
ou dans une barre qui ne l'a pas - l'entrée et le badge sont sur le bouton de
compte. Les entrées des applications elles-mêmes ne portent pas de badge.

![L'écran Portal avec la carte Notifications page, et la barre qui porte l'entrée Notifications et son badge](img/console/portal-notifications.webp)

La page des notifications fait aussi partie des
[pages intégrées](/docs/console/built-in-pages) que prévisualise l'éditeur de
thème, avec trois snackbars empilés dessus, tels que les dessine le bouton de
compte.

## La section Notifications d'un tenant

**Tenants**, un tenant, **Notifications** : pour root, le propriétaire du tenant
et ses membres ADMIN, sur une gateway à plusieurs tenants.

![La section Notifications d'un tenant : l'interrupteur du tenant, New notification, et ce qui a été envoyé, avec qui l'a lu et qui l'a acquitté](img/console/tenant-notifications.webp)

- **Notifications on for this tenant** est le niveau du tenant. Il est grisé,
  avec la raison, quand root ou l'administrateur de l'application a coupé le
  tenant.
- Le tableau liste ce qui a été envoyé à ce tenant : le titre, l'émetteur,
  combien l'ont lu, et pour une notification qui demande un acquittement, où il
  en est - *by alice* ou *Pending* quand un lecteur suffit, *37 / 52* quand
  chaque lecteur doit le faire. Il suit en direct.
- Un clic en ouvre une : son texte, son émetteur et ses dates, qui l'a acquittée
  et qui pas encore, chaque destinataire avec l'heure de sa lecture, et
  **Withdraw**.
- Deux actions de ligne, au bout de chaque ligne : **Replace** ouvre le tiroir
  New notification rempli depuis cette notification (audience, niveau,
  exercice, titre, texte, détail, actions, durée de vie, métadonnées, mail,
  acquittement) sous un nouvel id, son titre disant ce qu'il remplace ;
  **Send** publie la remplaçante, l'ancienne se termine comme remplacée, et ses
  lecteurs ne voient plus que la remplaçante. Vous remplacez ce que n'importe
  quel émetteur a envoyé à ce tenant, comme vous le retirez. **Withdraw**
  demande d'abord, comme depuis le tiroir. Les deux sont grisées, avec la
  raison au survol, sur une notification déjà retirée, remplacée ou expirée.

![Une notification qui demande l'acquittement d'un lecteur : qui l'a close, et chaque destinataire avec l'heure de sa lecture](img/console/tenant-notification-detail.webp)

### En envoyer une vous-même

**New notification** ouvre un tiroir pour essayer une notification sur ce
tenant, ou en envoyer une à la main. Il est proposé tant que le tenant reçoit
des notifications, et a deux onglets.

**Edit** la construit :

- **Audience** - des rôles et des membres. Les deux vides : chaque membre qui
  peut ouvrir ses liens.
- **Message** - le niveau, **An exercise** pour un message envoyé pour
  s'exercer (ses lecteurs le voient étiqueté EXERCICE), le titre et le texte.
- **Detail** - aucun, une route et un chemin dessous, ou une adresse https. Le
  champ montre l'adresse que le lecteur ouvrira.
- **Actions** - jusqu'à cinq boutons : un libellé, une route et un chemin ou une
  adresse https, et devant eux la **Method** : GET, ou POST, PUT, PATCH ou
  DELETE. Toute autre méthode que GET ouvre **Body (JSON)**, un éditeur JSON ;
  un JSON invalide est signalé sur le champ et retient Send. Le corps parvient
  au navigateur du lecteur : n'y mettez jamais de secret.
- **Acknowledgement** - None, One reader ou Every reader, et la route de votre
  service à appeler à chaque acquittement.
- **More** - la durée de vie, l'id (obligatoire : un UUID neuf est proposé, et
  un autre après chaque envoi), les métadonnées, et Also send by mail
  (seulement quand un relais de messagerie est configuré).

![Le tiroir New notification, onglet Edit : un détail sur la route Billing, et une action qui l'appelle en POST avec un corps JSON](img/console/notification-compose.webp)

**JSON** montre le message exact qu'enverrait un service, coloré et prêt à
copier : le plus court chemin vers l'appel qu'écrira votre service.

![L'onglet JSON : le message tel qu'un service l'envoie, avec la méthode et le corps de son action](img/console/notification-compose-json.webp)

**Send**, depuis l'un ou l'autre onglet, le publie aussitôt vers ce tenant, avec
vous pour émetteur. Il passe les mêmes contrôles que la publication d'un
service, et un refus revient sous le champ concerné, dans l'onglet Edit.

## Dans la piste d'audit

| Action | Écrite quand |
|---|---|
| `notification.publish` | un administrateur en envoie une depuis la page d'un tenant, ou un service en publie une sur le plan interne (l'émetteur est l'acteur) |
| `notification.replace` | un service, ou le message d'un administrateur, en remplace une |
| `notification.withdraw` | un administrateur, un agent ou l'émetteur en retire une |
| `notification.mail` | l'envoi des mails d'une notification est terminé : une ligne par notification avec son résultat |
| `notification.given` | quelqu'un a reçu des notifications après leur publication, ayant rejoint le tenant ou reçu un rôle |
| `notification.ack` | un lecteur en acquitte une ; le lecteur est l'acteur |
| `settings.notifications` | l'interrupteur, l'état d'un tenant ou le plafond change, au niveau de root ou de l'administrateur de l'application |
| `tenant.notifications` | l'interrupteur de la section Notifications d'un tenant change |
| `settings.notifications-access` | qui peut publier change |

Lire n'est pas dans la piste : la lecture reste sur la ligne de chaque
destinataire, que montre le tiroir. La liste suit en direct ce qu'un
service publie, remplace ou retire, comme les envois de la console.

Un assistant connecté à la gateway peut lister ce qui a été envoyé et retirer
une notification, jamais en publier : une notification, c'est un service qui
parle à ses utilisateurs de son propre travail.
