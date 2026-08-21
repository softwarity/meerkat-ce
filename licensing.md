# Licences : modèle, briques à construire, et comment tester

> **Rôle** : décrire comment Meerkat passe de l'édition communautaire à l'édition
> Enterprise, comment une licence est émise et livrée, et ce qu'il reste à
> construire pour vendre. Le contrat produit reste `requirements.md` ; ici on
> décrit **le mécanisme** et **la chaîne commerciale** autour.
>
> Dernière revue : 2026-08-09.

> ⚠ **Ce document est en retard sur le produit depuis le 2026-08-18.** Ce qui
> reste vrai : deux images depuis un commit (§1), la balise `ee`, le repli
> perpétuel comme principe commercial. Ce qui est **mort dans le code** et
> qu'aucune section ci-dessous n'a encore rattrapé :
>
> - **il n'y a plus de fichier de licence** : ni flag `-license`, ni
>   `MEERKAT_LICENSE_FILE`, ni chargement au démarrage. L'image EST la licence,
>   et le registre privé est ce qui la remet. `internal/license` survit sans
>   aucun appelant - à supprimer ou à ressusciter, mais pas à croire sur parole ;
> - **il n'y a plus de clés de fonctions** : plus de `MEERKAT_FEATURES`, plus de
>   catalogue, plus de `features.Require`. Une constante, `edition.Enterprise`,
>   et `edition.Require("ce que ça fait")` là où le code est nécessairement
>   commun ; le reste est **absent** du binaire communautaire ;
> - **la boucle de dev** : `make dev` (Enterprise, balise dans `.air.toml`) et
>   `make dev-ce`. Rien d'autre - `dev-locked` n'existe plus ;
> - `GET /api/edition` ne renvoie plus ni `features` ni `known`, et la console
>   ne pose plus qu'une classe `ee` sur `<body>`.
>
> À reprendre en entier lors du prochain passage sur le sujet commercial.

## 1. Le modèle, et pourquoi

**Deux images, un commit** (décidé le 2026-08-17, revient sur « un seul
binaire »). Le code Enterprise n'est plus dormant dans l'artefact gratuit : il
n'y est pas. La balise de compilation `ee` décide, dans `internal/edition`, ce
que l'éditeur de liens embarque.

```
go build ./cmd/meerkat            -> image communautaire (Docker Hub, publique)
go build -tags ee ./cmd/meerkat   -> image Enterprise (GH Packages, privée)
```

Pourquoi le revirement : une garde `features.Require` dans du code publié ne
protège rien - la FSL autorise justement de la retirer et de recompiler pour son
usage interne. Deux images doivent avoir deux **comportements**, pas un drapeau.
L'image communautaire ne lit donc aucune licence (`edition_ce.go`) et ne peut
rien ouvrir ; l'image Enterprise lit la licence et n'ouvre que ce qu'elle
couvre. Passer de l'une à l'autre, c'est **changer d'image**, pas déposer un
fichier.

Le prix, assumé : deux artefacts à publier, et surtout **à tester tous les
deux** (`make test` et `make test-ee`). Une fonction qui ne compile qu'avec la
balise casserait l'image communautaire en silence, et la publication serait le
seul endroit où on l'apprendrait.

**La licence est perpétuelle.** Qui a payé garde le droit d'exécuter ce qu'il a
payé, pour toujours. `ExpiresAt` n'éteint rien : c'est la **limite de couverture
des mises à jour**, comparée à la date de compilation du binaire, jamais à
l'horloge.

```go
// internal/license/license.go
func (l *License) Covered(buildDate time.Time) bool {
    return !buildDate.After(l.ExpiresAt)
}
```

Conséquence : une version publiée pendant la période couverte fonctionne
indéfiniment, même dix ans plus tard. Une version publiée après ne l'est pas.
Rien ne s'éteint tout seul, il n'y a pas d'horloge à surveiller, pas d'appel
maison, et jamais de « ça marchait hier ». C'est le modèle à repli perpétuel de
JetBrains ou Sidekiq.

Couper l'authentification d'une production pour une facture en retard rendrait
le produit indéployable : personne de sérieux ne met ça devant un service.

## 2. Ce qui existe déjà

| Brique | Où | État |
|---|---|---|
| Format signé (Ed25519) + validation | `internal/license/license.go` | fait |
| Signature (`Sign`) au même endroit que la validation | idem | fait, appelé nulle part |
| Politique perpétuelle (`Covered`) | idem | fait |
| Rotation de clés (plusieurs clés publiques acceptées) | `productionKeys` | prévu, **tableau vide** |
| Chargement au démarrage | `internal/edition/edition_ee.go` (`Setup`) | fait, **image EE seulement** |
| Drapeau et variable | `-license`, `MEERKAT_LICENSE_FILE` | fait |
| Bac à sable sans licence | `MEERKAT_FEATURES` (avertit bruyamment) | fait |
| Catalogue des fonctions | **supprimé le 2026-08-18** | une constante, `edition.Enterprise` |
| Endpoint console | `GET /api/edition` (`internal/admin/edition.go`) | fait |
| Tests de validation | `internal/license/license_test.go` | 6 tests |

### Les fonctions, et où elles sont gardées

| Clé | Gardée dans | Code sous `ee/` | Effective |
|---|---|---|---|
| `multi-tenant` | `main.go`, `edition.go`, `identity.go` | non : rien à déplacer (le code est le même en mono) | oui |
| `directories` (LDAP/AD) | `authproviders.go`, `grouprules.go` | **oui** : `ee/directories/` | oui |
| `saml` | `authproviders.go` | — | oui (SAML non implémenté) |
| `business-hours` | `identity.go` | non (voir ci-dessous) | oui |
| `white-label` | `theme.go`, `i18n.go`, `auth.go`, et `identity.go` pour la disposition | non : décisions d'une ligne | oui |
| `scim` | — | — | pas construit |
| `cluster` | — | — | pas construit |
| `audit-export` | **aucune garde** | — | à vérifier |

**Où vit le code payant, et pourquoi ça compte.** La FSL autorise l'usage, la
copie et la **modification** ; elle n'interdit que l'usage concurrent. Une garde
`features.Require` écrite dans `internal/` est donc un verrou dont la licence
donne la clé : retirer la ligne et recompiler pour son propre usage est permis.
Sous `ee/LICENSE.md`, le même geste est une violation - le code s'y lit et s'y
modifie, il ne s'y **utilise** qu'avec une clé.

D'où la règle : **ce qui a un corps de code substantiel et se vend descend sous
`ee/`**. C'est le cas des annuaires (437 lignes de protocole : dialectes,
groupes imbriqués, règle de correspondance AD). Ce n'est pas le cas de
`business-hours`, dont la partie vendue tient en une soixantaine de lignes
d'évaluation d'horaires : le schéma, l'API et les écrans resteraient de toute
façon dans le tronc, et déplacer si peu ne protège rien de sérieux.

**Le mécanisme** : `internal/idp` porte le contrat et un registre,
`ee/directories` s'y enregistre dans son `init()`, et `cmd/meerkat` porte
l'import blanc qui l'embarque - dans le fichier à balise `ee` de
`internal/edition`. Une compilation sans la balise obtient une édition
communautaire propre plutôt qu'une édition cassée, et le compilateur le prouve.

**Ce que la console ne fait pas encore** : tout son TypeScript est dans le tronc
FSL, y compris les écrans des fonctions vendues. Le verrou côté serveur reste la
seule frontière juridique.

OIDC et GitHub n'ont **pas** de clé : ils sont dans le tronc commun, et c'est
délibéré. OIDC couvre Google, Microsoft et les fournisseurs modernes — c'est ce
qui rend le produit adoptable sans acheter. Les annuaires d'entreprise sont
précisément ce que les entreprises ont et que les autres n'ont pas.

**À trancher** : `audit-export` est déclarée et gardée nulle part. Si la
fonction naît sans garde, elle naît gratuite.

## 3. La décision ouverte : mettre à jour au-delà de la couverture

Aujourd'hui, **rien ne se passe**. L'ordre dans `cmd/meerkat/main.go` est :

```go
features.Enable(lic.Features...)   // 347 : tout est ouvert
...
if !lic.Covered(built) {           // 350 : un avertissement, et c'est tout
    slog.Warn("this build was released after the licensed term: everything keeps working...")
}
```

Un client peut donc acheter un an et mettre à jour dix ans, contre une ligne
dans des journaux que personne ne lit. La limite est décorative.

Trois sorties :

1. **Le binaire trop récent démarre en communautaire.** On n'appelle pas
   `features.Enable`, on le dit au démarrage et dans la console. Reste honnête :
   la version que le client possédait tourne toujours, il choisit de ne pas
   mettre à jour. Mettre à jour est un acte volontaire, donc personne n'est
   coupé par surprise. **Recommandé**, avec un avertissement visible AVANT
   l'échéance et un message de démarrage qui nomme la version, la date de
   couverture et le fait que revenir en arrière restaure tout.
2. **Contrôle par la distribution** : seuls les clients à jour reçoivent les
   nouvelles versions. **Devenu possible le 2026-08-17** : l'image Enterprise
   part sur GH Packages en privé, l'image publique est la communautaire. Qui
   n'est plus couvert cesse simplement de recevoir la nouvelle image EE, sans
   qu'aucune ligne de code ne coupe quoi que ce soit.
3. **Statu quo**, en assumant que ce qui se paie est le support. Défendable,
   mais alors il ne faut pas afficher une date de couverture qui ne fait rien.

## 4. Les briques à construire

### 4.1 La paire de clés

Ed25519. La publique va dans `productionKeys` (embarquée dans chaque binaire
publié), la privée **ne doit jamais entrer dans ce dépôt, ni dans une CI
publique, ni dans une image**.

C'est le secret le plus critique du produit : qui l'a fabrique des licences à
volonté, et **on ne peut pas révoquer** — une licence signée est valable pour
toujours. La seule sortie en cas de fuite est de tourner la clé et de réémettre
pour tout le monde, d'où le tableau `productionKeys` qui accepte plusieurs clés.

### 4.2 L'outil d'émission

Un binaire privé qui remplit `license.License{Licensee, Plan, Features,
IssuedAt, ExpiresAt}` et appelle `license.Sign()`. `Sign` vit volontairement
dans le même paquet que le validateur pour qu'ils ne divergent jamais.

### 4.3 Le dépôt d'émission (privé, dédié)

Un dépôt séparé, qui ne sert qu'à ça :

- la clé privée en **secret de dépôt** ;
- un workflow `workflow_dispatch` paramétré (titulaire, plan, fonctions, fin de
  couverture) qui compile l'outil, signe, **commite le fichier** et l'envoie par
  courriel ;
- un second workflow de renvoi, qui retrouve un fichier déjà émis et le
  réexpédie.

**Le stockage se fait par commit, pas en artifact** : les artifacts expirent
(90 jours par défaut, quelques centaines au maximum selon le plan) alors qu'une
licence perpétuelle doit être retrouvable dans dix ans. Une *release* convient
aussi, ses pièces jointes ne s'effacent pas. **GitHub Packages ne convient pas**
— c'est un registre par écosystème (npm, Maven, conteneurs), sans dépôt de
fichiers arbitraires.

**Protection du secret** : quiconque peut modifier un workflow de ce dépôt peut
écrire un job qui exfiltre la clé. D'où : dépôt privé dédié, protection de
branche, et le job de signature dans un *environment* avec approbation
obligatoire.

Bénéfice de ce découpage : la clé reste **hors du portail**. Le portail ne sert
que des fichiers déjà signés, il peut être compromis sans que la clé bouge.

### 4.4 Stripe

Stripe fait le commerce, pas la licence : encaissement, abonnement de
maintenance, relance de carte, factures, portail client, et TVA européenne avec
Stripe Tax. Il ne signera jamais quoi que ce soit.

Le modèle perpétuel s'y exprime bien : chaque `invoice.paid` réémet la licence
avec `ExpiresAt` repoussé d'un an. Le client qui arrête de payer ne reçoit
simplement plus de nouvelle licence — la dernière reste valide pour toujours,
date figée. **Rien à révoquer, rien à désactiver.**

`Customer.metadata` peut porter le titulaire et le plan : Stripe sert alors de
registre commercial. Garder tout de même la trace des licences **émises** — un
client perdra la sienne.

À prévoir dès le début : **l'émission manuelle**. Les grands comptes achètent
sur bon de commande, par virement, avec une facture à 30 jours. Ce sera
probablement le premier vrai client.

### 4.5 Ce qui manque côté produit

- **Écran Licence dans la console** : déposer un fichier sans accès au serveur
  ni redémarrage, et voir titulaire, plan, fonctions, couverture.
- **Bandeau de couverture dépassée** : aujourd'hui c'est un `slog.Warn` que
  personne ne lit. Le dire dans la console transforme la surprise en
  renouvellement.
- **`MEERKAT_FEATURES`** ouvre tout sans licence, avec avertissement. Parfait en
  développement ; à revoir avant la première release publique, c'est la porte de
  service la plus évidente.
- **Une clé publique de test injectable** : `productionKeys` est une variable
  privée et vide, donc aucun parcours de bout en bout n'est possible sans
  recompiler (voir §5.3).

## 5. Comment tester

### 5.1 Ce qui est déjà couvert

```bash
go test ./internal/license/ -v
```

Six tests : signature valide, mauvaise clé, charge modifiée, **validité au-delà
de l'échéance** (le coeur du perpétuel), licence pas encore valide, aucune clé.

### 5.2 Les fonctions sans licence

`MEERKAT_FEATURES` ouvre les fonctions EE sans fichier — c'est ainsi qu'on teste
une fonction Enterprise en développement. La boucle de dev le fait déjà :
`make dev` pose `MEERKAT_FEATURES=all` (toutes les clés du registre, pas une
copie de la liste qui cesserait de grandir avec lui), `make dev-ce` donne la
forme communautaire.

Pour n'en ouvrir que quelques-unes :

```bash
MEERKAT_FEATURES=multi-tenant,directories,business-hours \
MEERKAT_ADMIN_PASSWORD=test1234 \
go run ./cmd/meerkat -data /tmp/mk-ee -addr :18080 -admin-addr :19090
```

Vérifier ensuite qu'une fonction gardée répond correctement dans les deux sens :

```bash
# sans la fonction : 4xx qui NOMME la fonction manquante
curl -s -b cookies -X PUT localhost:19090/api/auth-providers/corp \
  -H 'Content-Type: application/json' \
  -d '{"kind":"ldap","name":"Corp","enabled":true,"config":{"url":"ldaps://x","baseDn":"dc=x"}}'
# avec MEERKAT_FEATURES=directories : 200
```

`GET /api/edition` dit ce que le binaire croit être : édition, fonctions
actives, catalogue complet.

### 5.3 Le parcours complet (à débloquer)

Aujourd'hui impossible sans recompiler : `productionKeys` est vide et privée,
donc `license.Load()` refusera toujours toute licence. Deux options, à trancher :

- une variable d'environnement de **clé publique supplémentaire**, réservée au
  développement et bruyamment journalisée ;
- ou l'injection de la clé par `-ldflags` au moment du build, comme la version.

Une fois débloqué, le parcours à vérifier, sur une gateway jetable :

1. générer une paire de clés de test ;
2. émettre une licence avec l'outil (titulaire, `directories`, couverture
   dépassée volontairement) ;
3. démarrer avec `-license` : les journaux doivent dire `license loaded` avec le
   titulaire et les fonctions ;
4. vérifier que la fonction gardée passe ;
5. vérifier l'avertissement de couverture quand la date de build est postérieure
   à `ExpiresAt` — se simuler avec un build `-ldflags "-X …version.Date=…"` ;
6. retirer le fichier, redémarrer : retour en communautaire, la fonction gardée
   refuse à nouveau.

### 5.4 La chaîne d'émission

- signer une licence de test par le workflow, avec des clés de test ;
- vérifier que le fichier est bien **commité** (et non seulement en artifact) ;
- vérifier que le courriel part et que la pièce jointe se charge dans une
  gateway ;
- lancer le workflow de renvoi et vérifier qu'il retrouve le même fichier, à
  l'octet près.

## 6. Le parcours d'achat, par étapes

**Aujourd'hui, zéro client — et c'est suffisant** : un lien de paiement Stripe,
un courriel de notification, tu lances le workflow avec les bons paramètres, le
client reçoit sa licence dans la minute. Aucun serveur, aucun secret exposé sur
Internet, et le parcours d'achat se vit en vrai avant d'être industrialisé. Ça
tient jusqu'à une vingtaine de clients.

**Ensuite**, dans cet ordre :

1. émission automatique à la réception du paiement — Stripe ne peut pas
   déclencher GitHub directement (un webhook ne sait pas ajouter le jeton
   d'authentification qu'exige l'API), il faut un intermédiaire minuscule, une
   fonction Cloudflare Worker par exemple ;
2. portail de re-téléchargement — le premier besoin réel d'un portail est un
   client qui a perdu son fichier, pas un client qui achète.

**Piège d'autorisation** : l'identifiant Stripe ne doit pas suffire à
télécharger. Une adresse comme `/licences/cus_abc123.lic` est une fuite : ces
identifiants circulent dans les courriels, les captures et les tickets. Passer
par une session du portail Stripe, ou un lien signé à durée de vie courte.

**Pas de service tiers pour l'instant** (Keygen, Cryptolens : ~200 €/mois). La
vérification est déjà écrite et maîtrisée ; ce qu'ils vendent est un back-office
qui, ici, tient dans un webhook. À reconsidérer quand la gestion manuelle
devient pénible — jamais avant le premier client. Et de toute façon, garder la
capacité de signer soi-même : les clients ont des licences perpétuelles, elles
doivent survivre à la fermeture d'un fournisseur.

## 7. Repères de prix

Tarifer **par instance de production** (développement et pré-production
gratuits), jamais par utilisateur : au tarif utilisateur, la comparaison se fait
avec Cloudflare Access ou Auth0, sur un terrain où un éditeur indépendant ne
gagne pas. Meerkat est une brique d'infrastructure.

| | Perpétuel | Maintenance annuelle |
|---|---|---|
| Une instance de production | 2 500 – 4 000 € | 20 à 25 % (500 – 1 000 €) |
| Site (toutes instances d'une entreprise) | 8 000 – 15 000 € | idem |

Les 20 à 25 % sont le standard historique de l'édition logicielle et tombent
juste avec le modèle : payer une fois, garder l'usage pour toujours, renouveler
pour continuer à recevoir les nouvelles versions.

Commencer plus haut que son instinct : baisser un prix est facile, l'augmenter
demande de renégocier. Un composant qui tient l'authentification d'une
production à 400 € par an inquiète plus qu'il ne rassure.

Pour le tout premier client, ne pas chercher le bon prix mais le premier oui :
moitié prix contre un témoignage et le droit de citer son nom.
