---
title: Le journal d'audit
section: Exploitation
order: 206
summary: Chaque modification d'administration avec son diff champ par champ, chaque connexion réussie ou refusée, et qui a le droit de les lire.
---

# Le journal d'audit

Le journal comporte deux volets, réunis sur un seul écran :

- **Les modifications.** Tout ce qui est modifié par le plan de contrôle : qui l'a fait, sur
  quel objet, et - pour une mise à jour - les champs précis qui ont changé, avec leur valeur
  avant et après.
- **La sécurité des comptes.** Chaque connexion, chaque connexion refusée avec son vrai motif et
  l'adresse d'où elle venait, et chaque moyen d'accès à un compte que son titulaire a ajouté
  ou retiré.

Le journal fonctionne **en ajout seul**. Il ne connaît que l'insertion et la purge : aucun
endpoint ne permet de modifier un événement.

![Le journal d'audit](img/console/audit.webp)

## Ce qu'un événement porte

| Champ | Ce qu'il dit |
|---|---|
| quand | l'instant de l'événement, à la seconde près |
| acteur | le compte, et le **jeton** quand il y en a eu un |
| action | `route.update`, `theme.branding`, `settings.update`, `maintenance`, `backup.snapshot`... |
| cible | le type d'objet, son identifiant et son nom |
| organisation | quand la modification relève de l'une d'elles |
| changements | une entrée par champ modifié, avec son `from` et son `to` |
| détail | une note, pour les créations, les suppressions et les actions sans diff ; le motif d'une connexion refusée |
| adresse | dans le volet sécurité, l'adresse que la gateway a résolue |

Un import, une restauration ou un changement de configuration n'écrit qu'une ligne, dont le
détail dit ce qui a été fait : les décomptes, puis les objets ajoutés, modifiés et supprimés
et, pour un réglage, les champs qui ont changé - `setting tls (redirect)` plutôt qu'un simple
"8 updated". La liste s'arrête à quatre-vingts noms ; au-delà, le reste est seulement compté.

Une mise à jour dont le diff est vide n'écrit **rien** : enregistrer un formulaire sans
changer aucune valeur n'est pas un événement.

Le diff est calculé de façon générique, en comparant l'ancien objet et le nouveau champ par
champ. Il fonctionne donc pour une route, un compte, une organisation ou un bloc de réglages
sans code propre à chaque type. Un objet imbriqué ou une liste sont comparés sur leur encodage
complet : un changement à l'intérieur, où qu'il soit, fait apparaître le champ entier.

## Le jeton qui a agi est nommé

Une modification faite par un agent ou par un script apparaît comme
`admin, via claude-desktop`, et non comme `admin`. La mention est ajoutée au moment même où
l'événement est écrit, et non dans le code qui appelle cette écriture : aucun endpoint ajouté
plus tard ne peut donc l'oublier. Un journal qui nomme l'agent quatre fois sur cinq est pire
qu'un journal qui ne le nomme jamais : la cinquième fois, on croit qu'une personne a agi.

## Les secrets n'entrent jamais dans le journal

Un champ dont le nom contient `password`, `secret`, `token` ou `hash` est enregistré sous la
forme `***`, quelle que soit sa profondeur dans un objet imbriqué. Une image envoyée sous
forme de data URI est résumée - son type et sa taille - au lieu d'être stockée deux fois en
base64 : un enregistrement de la marque en transporte deux, l'ancienne et la nouvelle.

Les champs qui n'apprennent rien sur une modification sont ignorés : les identifiants, les
horodatages posés par le serveur, et les noms purement décoratifs qui ne font que
l'aller-retour avec l'objet.

## La sécurité des comptes

Ces lignes sont écrites par les pages de connexion et par le profil, jamais par le plan de
contrôle. Elles ont leurs deux types de cible : `account` pour un compte applicatif,
`console` pour une connexion à la console.

| Action | Quand | Détail |
|---|---|---|
| `signin` | une session a été ouverte | par quel moyen : `password`, `totp`, `passkey`, `email-code`, `external-totp`... |
| `signin.organisation` | l'entrée dans une organisation, choisie après la connexion ou rejointe plus tard | son nom |
| `signin.refused` | une porte est restée fermée | le motif (voir ci-dessous) |
| `signin.locked` | la tentative qui a déclenché le blocage | `throttled`, ou `bad-code` pour un second facteur |
| `signout` | le compte s'est déconnecté | `remote` quand la session a été fermée depuis *Active sessions* |
| `password.change` | le titulaire a changé son mot de passe | `required` quand le changement était imposé |
| `password.forgot` | un lien de réinitialisation a été envoyé par e-mail | |
| `password.reset` | le lien a été utilisé | |
| `mfa.enroll`, `mfa.remove` | un second facteur a été ajouté ou retiré | |
| `passkey.add`, `passkey.remove` | une passkey a été enregistrée ou révoquée | le navigateur, pour un ajout |
| `email.change.request` | une nouvelle adresse a été demandée et son lien envoyé | la nouvelle adresse |
| `email.change` | l'adresse a changé (après confirmation, ou immédiatement s'il n'y a pas de relais) | la nouvelle adresse |
| `token.create`, `token.revoke` | un jeton d'API personnel | son nom |
| `register`, `register.confirm` | une auto-inscription et sa confirmation | |
| `devkey.add`, `devkey.remove` | une clé plug déposée ou retirée, une par poste de travail | son empreinte, jamais la clé |

Les motifs de refus d'une connexion : `bad-credentials`, `disabled`, `outside-validity`,
`unconfirmed`, `outside-hours`, `bad-code`, `passkey`, `not-recognised` (l'autorité ne
connaît plus le compte), `not-invited`, et `provider:<id>` quand c'est un fournisseur
d'identité qui a refusé.

La page affichée au visiteur dit la même chose pour un mauvais mot de passe, un nom inconnu
et un compte désactivé : elle ne permet donc pas de deviner quels comptes existent. Le
journal, lui, donne le vrai motif : son lecteur est un administrateur.

Un nom refusé qui ne correspond à aucun compte n'a pas d'acteur. Ce qui a été saisi est
enregistré, tronqué, comme nom de la cible : la ligne dit ainsi ce qui a été tenté.

**Un matraquage ne remplit pas la table.** La tentative qui déclenche le blocage écrit
`signin.locked`, et celles qui sont refusées ensuite n'écrivent rien : un attaquant qui
enchaîne mille essais par seconde ne produit pas mille lignes par seconde.

Chaque ligne porte l'**adresse** que la gateway a résolue, jamais un `X-Forwarded-For`
écrit par l'appelant. Elle porte aussi l'**organisation** quand il y en a une : sur la
connexion elle-même pour un compte qui n'appartient qu'à une seule organisation, sur la ligne
`signin.organisation` quand elle est choisie ensuite. C'est cette mention qui rend la ligne
visible aux administrateurs de l'organisation.

Ces lignes ne sont pas poussées en direct vers la console : une liste de comptes rechargée à
chaque connexion de n'importe qui ne serait que du bruit. L'[historique des
connexions](/docs/auth/flow-pages) propre au compte reste sur le profil, et il disparaît avec
le compte. Le journal, lui, lui survit.

## Qui voit quoi

Le journal est un écran transversal à part entière, et chacun y lit la part que couvrent ses
capacités :

| Appelant | Ce qu'il lit |
|---|---|
| root | tout, y compris les connexions à la console |
| infra-admin | les routes, les fournisseurs d'identité, les certificats, les réglages, les signalements |
| app-admin | les comptes, les rôles, les thèmes, les langues, les planifications, les réglages, les signalements, et la sécurité des comptes applicatifs |
| l'administrateur d'une organisation | les événements des organisations qu'il administre, y compris les connexions de leurs membres à ces organisations |
| tous les autres | rien, et l'endpoint refuse au lieu de renvoyer une page vide |

Les connexions à la console sont réservées à root : elles disent où et quand travaillent
ceux qui exploitent la gateway.

L'outil `read_audit` de l'agent passe par cette même fonction : un agent qui verrait un
journal plus large que celui de la console serait un moyen de contourner le modèle de
capacités, sans que ce soit la faute de qui que ce soit en particulier.

## Envoyé à un collecteur

**Infra, OpenTelemetry**, onglet **Audit** (Enterprise), deux interrupteurs :

- *Send the audit logs* : le plan de données - les connexions des comptes, les refus, les
  changements de mot de passe et de second facteur, ainsi que les appels aux opérations
  auditées dans **Endpoint audit**, qui n'envoient rien tant que cet interrupteur est
  désactivé ;
- *Send Meerkat's console audit too* : les modifications faites dans la console et les
  connexions à celle-ci.

Chaque événement part aussi sous la forme d'un **log** OpenTelemetry dont la ressource porte
`meerkat.stream=audit` : le Collector et le backend le tiennent ainsi à l'écart des logs
ordinaires, avec sa propre requête, sa propre durée de conservation et ses propres lecteurs.
L'action constitue le corps ; l'acteur (`user.id`, `user.name`), la cible, l'organisation,
l'adresse du client et les changements champ par champ sont des attributs ; l'identifiant de
trace est celui de la requête, ce qui relie l'événement à sa trace et à sa ligne d'accès.

C'est une copie, jamais échantillonnée : le journal reste ici. L'enregistrement d'un
événement n'attend jamais le réseau - l'événement est mis en file et envoyé par lots, avec de
nouvelles tentatives tant que le collecteur ne répond pas. Ce qu'une file pleine oblige à
abandonner est compté, et l'onglet l'indique.

## Auditer les opérations d'une route

**Infra, Endpoint audit** (Enterprise) liste les opérations du contrat OpenAPI de chaque
route, avec un interrupteur par opération. Un appel audité devient un événement d'audit,
envoyé au collecteur avec le reste du journal quand l'onglet **Audit** d'OpenTelemetry est
activé. Rien n'est stocké ici : le volume est celui du plan de données.

Sur l'image Community, l'écran est verrouillé, tout comme le téléversement dans l'éditeur de
route : un événement que rien n'enverrait jamais, c'est un journal qui semble tenu et qui ne
l'est pas. Les règles qu'une configuration apporte depuis l'image Enterprise sont conservées
avec la route sans être appliquées, et vous pouvez toujours les supprimer.

![Endpoint audit avec l'opération de remboursement ouverte : deux champs tirés de l'appel, et le corps JSON transmis](img/console/endpoint-audit.webp)

| Toujours transmis | |
|---|---|
| l'appelant | `user.id`, `user.name`, `meerkat.tenant.id`, `meerkat.tenant.name`, `meerkat.group`, `meerkat.roles` |
| l'opération | `meerkat.route`, `http.request.method`, `http.route` (le gabarit), `url.path` (le chemin demandé) |
| la réponse | `http.response.status_code` - un refus est aussi un événement |
| le reste | `client.address`, l'identifiant de trace, et la description en guise de corps (préremplie à partir du résumé OpenAPI) |

Sur demande, pour chaque opération :

- **des champs tirés de l'appel**, transmis sous le nom `audit.field.<name>` : une variable
  de chemin, un paramètre de requête, un en-tête, ou un pointeur JSON dans le corps
  (`/order/id`) ;
- **le corps JSON**, 64 Ko au maximum, dans lequel les champs contenant des secrets sont
  remplacés à n'importe quelle profondeur (`password`, `token`, `secret`, `apiKey`... - la
  liste se modifie opération par opération).

L'audit observe et ne décide de rien : il enveloppe la sécurité des endpoints, voit la
réponse reçue par l'appelant, et ne peut ni ouvrir ni fermer une opération. Une route sans
contrat OpenAPI ne peut pas être auditée de cette façon : déposez son contrat, ou laissez le
service s'auditer lui-même.

### D'une route à l'autre

**Export**, sur l'écran Endpoint audit, télécharge les règles de la route dans un fichier
JSON. Dans l'éditeur de route, la section **OpenTelemetry** propose **Upload audit
configuration** : ce bouton lit un tel fichier, et les règles sont enregistrées avec la
route. Il est grisé sur une route de redirection, qui n'a aucune opération à auditer. Les
règles voyagent aussi avec l'export de la [configuration](/docs/console/configuration).

## Filtres, rétention, et ce qui manque

L'écran filtre sur le volet du journal (**All**, **Changes**, **Data plane sign-ins**,
**Console sign-ins**), sur le type de cible d'une modification, sur la période, et propose
une zone de texte libre qui cherche aussi dans le motif et dans l'adresse. L'API y ajoute
l'acteur et l'identifiant de la cible : `GET /api/audit?kind=security`.

La **rétention** est d'un an par défaut, appliquée par le nettoyage périodique. Root la
choisit en haut de l'écran - trois mois, six mois, un an, deux ans ou cinq ans (**Keep events
for**). Ce choix est réservé à root : celui qui peut raccourcir le journal peut du même coup
effacer ses propres traces. Si la durée est raccourcie, les événements plus anciens sont
supprimés au nettoyage suivant.

**Export CSV** (Enterprise) extrait le journal dans un fichier destiné à un auditeur ou à un
SIEM : il reprend les filtres de l'écran et le périmètre du demandeur - jamais plus que ce
que montre son écran -, avec une ligne par événement et le diff en JSON dans une colonne à
part. L'export écrit lui-même une ligne dans le journal (`audit.export`, lue par root) : le
fichier contient des adresses et des noms. `GET /api/audit/export` accepte les mêmes
paramètres que la liste.

Ce qui manque encore : la pagination côté serveur, l'activité du tunnel lui-même, un export
Parquet.
