---
title: Le journal d'audit
section: Exploitation
order: 206
summary: Chaque changement d'administration avec son diff champ par champ, chaque connexion et chaque connexion refusée, et qui a le droit de les lire.
---

# Le journal d'audit

Le journal a deux moitiés, sur un seul écran :

- **Les changements.** Toute mutation faite par le plan de contrôle : qui l'a faite, ce qu'elle a
  touché, et - pour une modification - les champs exacts qui ont bougé, avant et après.
- **La sécurité des comptes.** Chaque connexion, chaque connexion refusée avec sa vraie raison et
  l'adresse d'où elle venait, et chaque moyen d'entrer dans un compte que son titulaire a ajouté ou
  retiré.

Le journal est **append-only**. Il ne connaît que l'insertion et la purge : aucun endpoint ne peut
modifier un événement.

![Le journal d'audit](img/console/audit.webp)

## Ce qu'un événement porte

| Champ | Ce qu'il dit |
|---|---|
| quand | la seconde où c'est arrivé |
| acteur | le compte, plus le **jeton** quand un jeton a servi |
| action | `route.update`, `theme.branding`, `settings.update`, `maintenance`, `backup.snapshot`... |
| cible | le genre d'objet, son identifiant et son nom |
| organisation | quand le changement appartient à l'une d'elles |
| changements | une entrée par champ qui a bougé, avec son `from` et son `to` |
| détail | une note, pour les créations, les suppressions et les actes sans diff ; la raison d'une connexion refusée |
| adresse | sur la moitié sécurité, l'adresse que la passerelle a résolue |

Un import, une restauration ou un changement de configuration écrit une ligne, et son détail dit ce
qu'il a fait : les compteurs, puis quels objets ont été ajoutés, modifiés et retirés, et pour un
réglage les champs qui ont bougé - `setting tls (appNames)` plutôt qu'un simple « 8 updated ».
Plafonné à quatre-vingts noms, le reste compté.

Une modification dont le diff ressort vide n'écrit **rien** : enregistrer un formulaire sans changer
une valeur n'est pas un événement.

Le diff est calculé de façon générique, en comparant l'ancien et le nouvel objet champ par champ,
donc il marche pour une route, un compte, une organisation ou la charge utile des réglages sans code
par type. Un objet imbriqué ou une liste se comparent par leur encodage entier : un changement
n'importe où dedans fait remonter le champ en entier.

## Le jeton qui a agi est nommé

Un changement fait par un agent ou par un script se lit `admin, via claude-desktop`, et non `admin`.
Le tampon est posé dans l'écriture d'audit elle-même plutôt que chez ses appelants, pour qu'aucun
endpoint ajouté plus tard ne l'oublie. Un journal qui nomme l'agent quatre fois sur cinq est
pire qu'un journal qui ne le nomme jamais : la cinquième se lit comme si une personne l'avait fait.

## Les secrets n'atterrissent jamais dans le journal

Un champ dont le nom contient `password`, `secret`, `token` ou `hash` est enregistré comme `***`, à
n'importe quelle profondeur d'un objet imbriqué. Une image envoyée en data URI est résumée - son genre
et sa taille - plutôt que stockée deux fois en base64 : un enregistrement de marque en porte deux,
l'avant et l'après.

Les champs qui ne disent rien d'un changement sont sautés : les identifiants, les horodatages du
serveur, et les noms d'affichage qui font l'aller-retour sans être modifiables.

## La sécurité des comptes

Écrite par les pages de connexion et par le profil, jamais par le plan de contrôle, sous deux genres
de cible à elle : `account` pour un compte des applications, `console` pour une connexion à la
console.

| Action | Quand | Détail |
|---|---|---|
| `signin` | une session a été émise | comment : `password`, `totp`, `passkey`, `email-code`, `external-totp`... |
| `signin.organisation` | une organisation où l'on entre, choisie après la connexion ou rejointe plus tard | son nom |
| `signin.refused` | une porte est restée fermée | la raison (ci-dessous) |
| `signin.locked` | la tentative qui a déclenché le frein | `throttled`, ou `bad-code` pour un second facteur |
| `signout` | le compte s'est déconnecté | `remote` quand elle est fermée depuis *Active sessions* |
| `password.change` | changé par son titulaire | `required` quand le changement était imposé |
| `password.forgot` | un lien de réinitialisation a été envoyé | |
| `password.reset` | le lien a servi | |
| `mfa.enroll`, `mfa.remove` | un second facteur ajouté ou retiré | |
| `passkey.add`, `passkey.remove` | une passkey enregistrée ou révoquée | le navigateur, pour un ajout |
| `email.change.request` | une nouvelle adresse demandée, son lien envoyé | la nouvelle adresse |
| `email.change` | l'adresse a changé (confirmée, ou aussitôt sans relais) | la nouvelle adresse |
| `token.create`, `token.revoke` | un jeton d'API personnel | son nom |
| `register`, `register.confirm` | une inscription et sa confirmation | |
| `devkey.add`, `devkey.remove` | une clé plug déposée ou retirée, une par poste | son empreinte, jamais la clé |

Les raisons d'un refus : `bad-credentials`, `disabled`, `outside-validity`, `unconfirmed`,
`outside-hours`, `bad-code`, `passkey`, `not-recognised` (l'autorité ne connaît plus le compte),
`not-invited`, et `provider:<id>` quand un fournisseur d'identité a refusé.

La page que voit le visiteur dit la même chose pour un mauvais mot de passe, un nom inconnu et un
compte désactivé, donc rien n'est énuméré. Le journal dit lequel c'était : son lecteur est un
administrateur.

Un nom refusé qui ne correspond à aucun compte n'a pas d'acteur. Ce qui a été tapé est enregistré
comme nom de la cible, tronqué, pour que la ligne dise encore ce qui a été tenté.

**Marteler un compte ne remplit pas la table.** La tentative qui déclenche le frein écrit
`signin.locked`, et celles refusées après n'écrivent rien : un attaquant à mille essais par seconde ne
fait pas mille lignes par seconde.

Chaque ligne porte l'**adresse** que la passerelle a résolue, jamais un `X-Forwarded-For` écrit par
l'appelant. Et l'**organisation** quand il y en a une : sur la connexion elle-même pour un compte qui
n'appartient qu'à une seule, sur la ligne `signin.organisation` quand elle est choisie ensuite. C'est ce
tampon qui montre la ligne aux administrateurs de cette organisation.

Ces lignes ne sont pas poussées en direct dans la console : une liste de comptes relue à chaque
connexion de n'importe qui serait du bruit. L'[historique de connexion](/docs/auth/flow-pages) du
compte reste sur le profil, et il part avec le compte. Le journal lui survit.

## Qui voit quoi

Le journal est un écran transverse à lui, et chaque appelant lit la tranche que couvrent ses capacités :

| Appelant | Ce qu'il lit |
|---|---|
| root | tout, connexions à la console comprises |
| infra-admin | les routes, les fournisseurs d'identité, les certificats, les réglages, les signalements |
| app-admin | les comptes, les rôles, les thèmes, les langues, les appels planifiés, les réglages, les signalements, et la sécurité des comptes des applications |
| un administrateur d'organisation | les événements des organisations qu'il administre, connexions de leurs membres comprises |
| n'importe qui d'autre | rien, et l'endpoint refuse plutôt que de servir une page vide |

Une connexion à la console est l'affaire de root seul : elle dit où et quand travaillent ceux qui font
tourner la passerelle.

L'outil `read_audit` de l'agent partage exactement cette fonction : un agent qui verrait un journal
plus large que la console serait un contournement du modèle de capacités, et ce ne serait la faute de
personne en particulier.

## Filtres, rétention, et ce qui manque

L'écran filtre sur la partie du journal (**All**, **Changes**, **Data plane sign-ins**, **Console
sign-ins**), le genre de cible d'un changement, la période, et
une zone de texte libre qui cherche aussi dans la raison et l'adresse. L'API ajoute l'acteur et
l'identifiant de cible : `GET /api/audit?kind=security`.

La **rétention** est d'un an par défaut, appliquée par l'entretien périodique, et root la choisit en
haut de l'écran - trois mois, six, un, deux ou cinq ans (**Keep events for**). Réservée à root : qui
peut raccourcir le journal peut y effacer ses propres traces. Raccourcie, les événements plus anciens
partent au passage suivant.

**Export CSV** (Enterprise) sort le journal en fichier pour un auditeur ou un SIEM : les filtres de
l'écran, le périmètre de qui le demande - jamais plus que ce que son écran montre -, une ligne par
événement, le diff en JSON dans sa propre colonne. L'export écrit sa propre ligne (`audit.export`,
lue par root) : le fichier porte des adresses et des noms. `GET /api/audit/export` prend les mêmes
paramètres que la liste.

Pas encore là : la pagination côté serveur, l'activité propre du tunnel, un export Parquet, et l'envoi
du journal à un collecteur en OTLP.
