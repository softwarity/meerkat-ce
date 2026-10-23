---
title: TLS et certificats
section: Exploitation
order: 224
summary: Les cinq façons de faire entrer un certificat, les trois portes où le placer, ce que contient la base, et ce qu'un cluster doit partager.
---

# TLS et certificats

Un certificat est détenu une seule fois, et **placé** sur la console, sur
l'application, sur le plan interne, ou sur plusieurs d'entre eux (SSL-08) - ajouté
depuis le menu d'un plan, il arrive sur ce plan. Ses noms lui appartiennent - ce
sont ceux pour lesquels le certificat lui-même déclare répondre. Un certificat qui porte
plusieurs noms, un nom générique ou une adresse IP les sert donc tous, et un certificat
utilisé par les deux plans est une seule entrée, pas deux copies à garder synchronisées.

Il n'existe pas d'interrupteur "activer HTTPS" : **c'est le certificat placé sur un plan
qui ouvre sa porte**, et c'est son retrait qui la ferme. Un interrupteur qui peut être
activé sans rien derrière est un interrupteur qui ment.

![L'écran Domains & TLS](img/console/tls.webp)

## Les façons d'entrer

Aucune n'est facultative, car les environnements où tourne Meerkat ne se ressemblent
pas (SSL-01) :

| Façon | Quand c'est la bonne |
|---|---|
| **Import** | vous détenez déjà le certificat et sa clé |
| **Depuis le système de fichiers** | le runtime le monte - un Secret Kubernetes, un secret Swarm, un volume compose - et le renouvelle |
| **Ask cert-manager** | Kubernetes seulement, pour les noms d'un plan à l'intérieur du cluster - le plan interne, ou la console et l'application derrière un proxy qui les joint en rechiffrant : la gateway demande elle-même à cert-manager, quand personne n'a écrit son Certificate |
| **Auto-signé** | un labo, un nom interne, un premier déploiement |
| **Demande de signature** | la gateway crée la clé et la CSR, votre autorité signe, vous importez la réponse |
| **ACME** | une autorité émet et renouvelle toute seule |

Une demande de signature est une ligne qui détient une clé et ne sert rien : elle existe
pour que vous puissiez télécharger de nouveau la demande en attendant la réponse.

## Les ports

Une porte HTTPS par plan, à côté de la porte en clair, que la gateway ouvre et ferme
sans s'arrêter (SSL-02) :

| Plan | En clair | HTTPS |
|---|---|---|
| Données | `MEERKAT_ADDR`, `:8080` | `MEERKAT_TLS_ADDR`, `:8443` |
| Contrôle | `MEERKAT_ADMIN_ADDR`, `:9090` | `MEERKAT_ADMIN_TLS_ADDR`, `:9443` |
| Interne | `MEERKAT_INTERNAL_ADDR`, `:7070` | `MEERKAT_INTERNAL_TLS_ADDR`, `:7443` |

Un port dont on ne peut pas nommer le protocole est un port pour lequel personne ne peut
écrire ni règle de pare-feu ni procédure d'exploitation : voilà pourquoi il y en a
deux par plan. La porte HTTPS du plan interne présente un certificat pour le nom par
lequel les services l'appellent - celui de son Service, dans le cluster -, qui vient de la
PKI du cluster, lu depuis le système de fichiers (ci-dessous). Aucune autorité publique ne
voit ces noms : ACME n'y est donc pas proposé.

**Ce sont les ports vus de l'intérieur.** Entre eux et un navigateur, il y a en général
une correspondance de ports : un Service Kubernetes qui publie `9443` sur `19443`, un
`-p` de Docker, un ingress Swarm. La gateway interroge son environnement d'exécution -
son propre pod et les Services qui le sélectionnent, ou son conteneur via le socket
Docker - et les liens de l'écran Domains & TLS portent le port réellement joignable de
l'extérieur. Quand l'environnement ne publie pas de porte HTTPS, l'écran le dit au lieu
de proposer un lien qui ne mène nulle part. Le chart Helm publie par défaut les deux
portes HTTPS (`service.appTlsPort`, `service.adminTlsPort`) et accorde le droit de
lecture nécessaire (`rbac.read`).

> [!NOTE] Docker Desktop
> Son Kubernetes publie les ports d'un LoadBalancer sur `localhost` à la création du
> Service, et ignore les ports ajoutés par la suite. Après la mise à niveau d'une
> release vers un chart qui ajoute les ports HTTPS, supprimez les deux Services et
> relancez `helm upgrade`.

Remplacer un certificat revient à échanger une liste derrière un verrou : la
négociation TLS la lit par une fonction de rappel, si bien que l'écoute ne bouge pas et
qu'aucune connexion n'est coupée. Pour ouvrir une porte, la gateway réserve
**d'abord** le port, et signale l'échec avant d'avoir changé quoi que ce soit - un port
déjà pris ne doit pas laisser un exploitant sans HTTPS du tout.

## Couper le port en clair

HTTPS n'a pas d'interrupteur ; le port en clair en a un (SSL-15). Sur la première ligne de
l'onglet de chaque plan - Console, Application, Internal -, `HTTP:<port>` s'éteint ou se
rallume après une confirmation, et le HTTPS d'à côté s'affiche comme un état : un cadenas
fermé quand un certificat sert HTTPS sur le plan, ouvert sinon.

**Éteint, le port en clair ne s'arrête pas : il n'écoute plus que sur `127.0.0.1`.** Plus
rien sur le réseau ne le joint, et deux choses le joignent encore :

- **un port-forward**, le chemin du retour quand un certificat casse la porte HTTPS :
  `kubectl port-forward service/<release>-meerkat-admin 9090:9090` arrive sur la boucle
  locale du pod, où la console répond toujours en HTTP simple ;
- **le health check que l'image lance sur elle-même**, `meerkat health`, qui interroge
  `http://127.0.0.1:8080`. Les sondes startup, liveness et readiness du chart Helm lancent
  cette commande dans le conteneur (`exec`) au lieu d'une requête HTTP du kubelet : un plan
  dont le port en clair est éteint reste donc prêt (voir
  [Cluster Kubernetes](/docs/deploy/kubernetes)). Tout ce qui sonde le port en clair depuis
  l'extérieur - le health check d'un load balancer de cloud - le perd : dirigez-le vers le
  port HTTPS, qui répond aussi à `/readyz`.

Deux règles gardent un plan joignable, sur chaque plan. **HTTP ne s'éteint que si un
certificat y sert HTTPS** - sinon HTTPS serait le seul chemin, et fermé. **Le dernier
certificat reste tant que HTTP est éteint** - le retirer ne laisserait plus rien pour
répondre.

**Sur la console, la porte en usage ne se coupe pas.** Chacun de ses deux ports montre qui
l'a appelé dans les 30 dernières secondes - *Receiving, from 10.42.1.178*, le pair TCP, un
proxy placé devant ou le navigateur lui-même, ou *Nothing in the last 30 s* -, en direct, et
mis en commun entre les réplicas par le bus de changement ; les health checks ne comptent
pas. HTTP ne s'éteint pas tant que la requête elle-même ou quoi que ce soit d'autre dans
cette fenêtre passe par HTTP, et le dernier certificat de la console ne se retire pas tant
que quelque chose passe par HTTPS : l'écran demande de joindre d'abord la console par l'autre
port, et la gateway refuse la même chose (`409`). Pour passer en HTTPS de bout en bout une
console placée derrière un proxy :

1. placez le certificat sur la console : son HTTPS s'ouvre ;
2. dirigez le proxy vers le port HTTPS de la console ;
3. ouvrez la console à travers lui ;
4. vérifiez que HTTP affiche *Nothing in the last 30 s* ;
5. éteignez HTTP.

Les plans application et interne n'ont ni affichage du trafic ni cette règle.

L'interrupteur est `PUT /api/settings/tls/ports/{console|app|internal}` avec
`{"http": false}` ou `{"http": true}`, audité sous `tls.ports`. `GET /api/settings/tls` rend
trois champs de plus : `closed`, par plan (`["http"]` quand son port en clair est éteint) ;
`consoleTraffic`, laquelle des portes de la console a été appelée, et d'où ; et
`trafficWindow`, cette fenêtre en secondes (30).

## Sur une machine locale

Un portable ou un labo n'a ni nom public ni autorité publique, et un certificat
auto-signé fait protester tous les navigateurs. Le bouton **On a local machine** de
l'écran Domains & TLS ouvre la marche à suivre, système par système (macOS, Linux, Windows). Les
noms que vous saisissez - à commencer par celui qui sert à joindre la console - figurent
déjà dans les commandes :

1. **Le fichier hosts** fait pointer les noms vers la gateway - `127.0.0.1` quand
   elle tourne sur la même machine (Docker, un Kubernetes local), son adresse dans les
   autres cas - une VM, minikube, une autre machine ; la console propose l'adresse par
   laquelle vous l'avez jointe quand c'en est une. `localhost` ne demande aucune ligne.
2. **Une autorité locale**, créée par [mkcert](https://github.com/FiloSottile/mkcert) et
   reconnue par cette machine : le magasin du système, Chrome, Edge, Safari, et Firefox
   via `nss`. Redémarrez ensuite les navigateurs.
3. **Un seul certificat pour tous les noms**, signé par cette autorité : `meerkat.pem`
   et son `meerkat-key.pem`.
4. **Importez-le** - Add certificate dans l'onglet Console, Import a PEM pair, en déposant
   les deux fichiers sur la boîte de dialogue - puis la même chose dans l'onglet
   Application.

![Le tiroir qui détaille les étapes pour HTTPS sur une machine locale : les noms, le fichier hosts, une autorité locale, un certificat, l'import](img/console/tls-local.webp)

> [!WARNING]
> La clé privée de l'autorité reste dans le dossier qu'affiche `mkcert -CAROOT`. Celui
> qui la détient peut signer pour n'importe quel nom auquel cette machine fait
> confiance : ne la partagez jamais - une autre machine crée la sienne.

## Quel certificat répond à une négociation TLS

Le choix se fait d'après le nom demandé par le client, parmi les certificats placés sur
ce plan et utilisés dans son mode (voir
[Derrière un proxy qui termine le TLS](#derrire-un-proxy-qui-termine-le-tls)) : celui qui porte ce nom, et si deux le portent, celui dont la validité dure le
plus longtemps. Une négociation qui ne nomme aucun hôte - un client qui joint la
gateway par son adresse, ou tout ce qui est antérieur à l'extension SNI - reçoit le
certificat **de repli**, le plus ancien de ceux placés sur le plan. Si rien n'est placé,
elle reçoit une erreur qui nomme ce qui *est* servi, ce qui est plus utile qu'un
certificat pris au hasard que le client rejetterait de toute façon.

Placer un certificat là où un autre répond déjà pour l'un de ses noms est refusé tant
que l'appelant ne demande pas explicitement le **remplacement**
(`PUT /api/certificates/{id}/placement` avec `"replace": true`) : le certificat remplacé
quitte ce plan et est gardé, sous *Unassigned certificates* s'il n'est sur aucun autre.

Activer TLS sans rien avoir à présenter est refusé : un port qui fonctionne deviendrait
un port qui refuse tous les visiteurs.

## Le port en clair peut rediriger

Sur le **plan de données uniquement**, le port en clair peut rediriger vers le port
HTTPS (SSL-06). Les deux health checks en sont dispensés : un `308` serait interprété
comme "pas prêt". Il en va de même de toute requête destinée à un nom qu'aucun
certificat placé sur l'application ne porte : un service interne au cluster qui appelle
`http://meerkat-meerkat.ns.svc:8080` - pour un appel d'API interne - serait renvoyé vers un certificat établi pour un autre nom, signé par une
autorité à laquelle il ne fait pas confiance, et échouerait.

Quand la redirection est active, chaque réponse HTTPS du plan de données porte aussi
l'en-tête **HSTS** (`Strict-Transport-Security: max-age=...`). La redirection ne peut
pas protéger la requête qu'elle redirige : cette première requête part en clair, et
quiconque se trouve sur le réseau - un Wi-Fi public, un proxy compromis - peut y
répondre lui-même sans jamais transmettre la redirection. Il maintient alors le visiteur
en HTTP pendant que lui-même parle en HTTPS à la gateway (SSL stripping). Avec HSTS,
le navigateur s'en souvient et réécrit lui-même `http://` en `https://` avant d'envoyer
quoi que ce soit.

HSTS suit la redirection au lieu d'avoir son propre interrupteur : forcer HTTPS
constitue déjà l'engagement - les navigateurs mémorisent aussi un `301`. Seule la
**durée** se choisit : un jour par défaut, deux ans au plus. L'en-tête disparaît avec la
redirection lorsque tous les certificats ont expiré. Il n'est jamais envoyé à
`localhost` (la promesse couvre tous les ports d'un nom d'hôte : une gateway de
développement imposerait HTTPS à toutes les applications locales du développeur) ni à
une adresse IP (les navigateurs l'y ignorent), jamais à la place d'une valeur déjà
définie par un service ou par le filtre
[security-headers](/docs/filters/security-headers) d'une route, et toujours sans
`includeSubDomains`.

> [!NOTE] Uniquement sur 443 - une limite des navigateurs
> HSTS n'est envoyé que lorsque HTTPS est joint sur le port **443**, avec le HTTP en
> clair sur le port 80. Un navigateur applique la promesse à tous les ports du nom, et
> lorsqu'il bascule une requête en HTTPS, il conserve le port : avec HSTS sur `8443`, il
> réécrirait `http://name:8080` en `https://name:8080` - un port en clair - et toutes
> les adresses en clair sous ce nom, celle de la console comprise, cesseraient de
> répondre. Sur les autres ports, la redirection fait le travail à elle seule, à chaque
> visite, et les réponses HTTPS portent `max-age=0`, ce qui fait oublier au navigateur
> une promesse antérieure. Le même `max-age=0` est envoyé quand Force HTTPS est
> désactivé : les navigateurs cessent ainsi d'insister dès leur prochaine visite en
> HTTPS, sans attendre la fin de la durée.

> [!WARNING]
> Un navigateur tient la promesse pendant toute la durée, même si les certificats sont
> retirés, et ne propose plus de passer outre un certificat invalide. Commencez par un
> jour ; allongez la durée une fois HTTPS bien en place.

## Derrière un proxy qui termine le TLS

Souvent, le HTTPS que voit un navigateur n'est pas celui de la gateway : nginx, un
ingress controller ou un load balancer cloud détient le certificat et transmet à la
gateway - en HTTP en clair, ou en rechiffrant. La gateway ne peut alors savoir, par sa
propre connexion, ni que le navigateur était en HTTPS, ni quels noms sont vraiment les
siens - et les mails qui prouvent une boîte - la confirmation d'une inscription, une
nouvelle adresse, une réinitialisation - porteraient un code au lieu d'un lien,
puisqu'un lien n'est envoyé que vers un nom dont la gateway peut se porter garante (voir
[Un lien, ou un code](/docs/auth/local-accounts#un-lien-ou-un-code)).

Les plans Console et Application disent donc chacun qui sert leurs noms publics,
**HTTPS served by** (SSL-14) : **Meerkat**, avec les certificats placés sur le plan, ou
**A proxy in front**. Les deux déclarent leurs **noms publics**, un nom complet à chaque fois
(`shop.example.com`) ; un nom n'appartient qu'à un plan, et le premier nom de
l'Application est l'adresse d'un lien envoyé par mail ou du callback d'un fournisseur
d'identité quand aucune requête ne dit laquelle. Les noms publics de l'Application sont
aussi ceux parmi lesquels se choisit l'**émetteur** de tous les jetons que signe Meerkat,
écrit `https://<nom>` : le `iss` du JWT d'identité et l'adresse du fournisseur OpenID
(voir [Le JWT d'identité](/docs/auth/service-jwt)). En ajouter, en retirer ou
les réordonner déplace l'émetteur sur chaque nœud. Changer de mode garde les noms et les
certificats : un certificat qui ne convient pas au nouveau mode reste placé, marqué
*Inactive in this mode*, et sert de nouveau quand le plan revient en arrière. À l'écran,
le sélecteur ne fait que proposer ; **Apply**, puis une confirmation, enregistre. Un
changement du plan Console qui ferait voir en http simple la propre session de
l'opérateur - jointe par un proxy qui dit HTTPS, que le nouveau réglage ne croirait
plus - est refusé (`409`), puisqu'il le déconnecterait.

| | Meerkat | A proxy in front |
|---|---|---|
| Qui présente les noms publics | la gateway, avec un certificat placé sur le plan - un nom ajouté qu'aucun ne couvre est refusé | le proxy |
| Le HTTPS se sait par | la porte TLS de la gateway | le nom lui-même : une requête pour un nom public est en https, sans en-tête |
| Un certificat placé sur le plan | porte les noms publics (ACME, import, système de fichiers) | facultatif : il chiffre le saut depuis le proxy, et porte les noms du plan à l'intérieur du runtime |
| Force HTTPS, HSTS | sur les noms que porte un certificat servi | jamais : c'est l'affaire du proxy |
| `X-Forwarded-For`, `X-Forwarded-Proto` crus venant de | personne, dès que des noms sont déclarés | les adresses du proxy, ou les plages privées quand aucune n'est déclarée |

**Le saut depuis le proxy.** Un Ingress dont le backend est en HTTPS, ou une Route
OpenShift en `reencrypt`, ouvre TLS vers la gateway sous le nom par lequel il la joint :
le Service placé devant le plan sur Kubernetes - `<release>-meerkat` du chart pour
l'application, `<release>-meerkat-admin` pour la console, sous leurs quatre formes - ou
le seul nom de service que partagent tous les ports sous Swarm ou compose. Pour que le
saut soit chiffré, le proxy lui-même doit être dirigé vers le port HTTPS du plan et lui
parler en TLS : un Ingress qui vise encore le port HTTP laisse le saut en clair, et le
certificat inutilisé. Le certificat placé sur un plan derrière un proxy porte ces noms,
et le plan les liste comme ses *Internal names*, cochés quand un certificat servi les
couvre. Sur un tel plan, la gateway sert
les certificats qui portent l'un de ces noms ; quand le runtime ne sait pas les dire,
tous les certificats placés sauf ceux qui couvrent les noms publics, que c'est au proxy
de présenter. Sur Kubernetes, le plan peut le demander à cert-manager (**Ask
cert-manager**), comme le plan interne (plus bas).

**Avec Cilium**, un Ingress classique ne sait pas parler HTTPS à son backend : rechiffrer
demande la Gateway API - une Gateway et une HTTPRoute dirigées vers le port HTTPS du
Service, et une **BackendTLSPolicy** qui nomme l'autorité à croire et le nom d'hôte que
porte le certificat de Meerkat, par exemple le nom du Service dans le cluster.

::: details Le saut vers la console par la Gateway API
```yaml
# La HTTPRoute envoie le nom public de la console vers le port HTTPS du Service...
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: meerkat-console
spec:
  parentRefs: [{ name: public }]
  hostnames: ["console.example.com"]
  rules:
    - backendRefs: [{ name: <release>-meerkat-admin, port: 9443 }]
---
# ...et celle-ci dit comment la Gateway vérifie le certificat qu'elle y trouve.
apiVersion: gateway.networking.k8s.io/v1
kind: BackendTLSPolicy
metadata:
  name: meerkat-console
spec:
  targetRefs:
    - { group: "", kind: Service, name: <release>-meerkat-admin }
  validation:
    caCertificateRefs:
      - { group: "", kind: ConfigMap, name: cluster-ca }
    hostname: <release>-meerkat-admin.<namespace>.svc
```
:::

**D'où vient le proxy.** *Proxy addresses (optional)*, laissé vide, veut dire que le
proxy tourne dans le cluster ou sur un réseau privé : la boucle locale et les plages
privées sont crues (un contrôleur Ingress du cluster, un nginx sur la machine). Rempli,
il nomme les plages CIDR ou les adresses seules du proxy, pour un proxy qui joint Meerkat
depuis des adresses publiques - un CDN, les load balancers de Google Cloud
`130.211.0.0/22` et `35.191.0.0/16`. Ce n'est que de là que les en-têtes transmis sont
crus sur ce plan ; voir
[Derrière un proxy ou un load balancer](/docs/deploy/one-gateway#derrire-un-proxy-ou-un-load-balancer).
Un plan dont personne n'a rien dit garde la règle d'avant : les plages privées.

Un nom est un nom d'hôte seulement - ni schéma, ni port, ni chemin -, jamais une adresse
IP, dont personne ne peut se porter garant par un nom, et jamais un joker :
`*.example.com` se porterait garant de tous les noms en dessous, y compris celui qui
pointe encore vers un service que plus personne n'exploite, et une réinitialisation
demandée sous ce nom y enverrait son jeton. Un certificat peut porter un joker ; un nom
déclaré, non. Les plans voyagent avec un export de configuration, avec le reste des
réglages TLS, et un changement atteint chaque nœud par le bus de changement
(`PUT /api/settings/tls/doors/{console|app}` avec
`{"proxied": true, "names": [...], "proxyAddresses": [...]}`, audité sous `tls.door`).
`GET /api/settings/tls` les rend sous `doors` - `console`, `app` et `internal`, chacun
avec ce qui a été dit, ses noms `runtime`, et les noms déclarés qu'aucun certificat ne
couvre (`uncovered`).

> [!WARNING]
> Ne déclarez que des noms qui atteignent vraiment cette gateway par ce proxy. Un lien
> part vers le nom que portait la requête, dès qu'il correspond : la déclaration est ce
> qui se tient entre un `Host` forgé et le mail.

## ACME n'est pas Let's Encrypt

ACME fait partie de l'édition **Enterprise**. Une configuration importée sur l'image
Community laisse de côté ses autorités et ses commandes, et son plan d'import les cite.
Une gateway passée de l'image Enterprise à l'image Community conserve ce que contient
sa base de données, mais ne sollicite plus aucune autorité, et l'écran Domains & TLS le signale :
ces certificats ne seront pas renouvelés.

Une installation déclare autant d'**autorités** qu'elle en utilise - Let's Encrypt pour
un domaine, ZeroSSL pour celui d'un client, le `step-ca` de l'entreprise pour les noms
internes - et chacune est un compte ACME nommé (SSL-05). L'annuaire d'une autorité
personnalisée est une **URL**, et tout est là : la moitié des installations auxquelles
Meerkat se destine n'ont aucun accès à Internet. Un `step-ca` interne, un EJBCA, une
autorité Windows dotée du rôle ACME - toutes répondent ici.

| Champ | À quoi il sert |
|---|---|
| Provider | un fournisseur connu fixe l'annuaire et indique ce qu'il lui faut en plus ; *Another authority* apporte sa propre URL |
| Root CA | l'autorité qui signe le certificat HTTPS **du serveur ACME lui-même**, au format PEM - un serveur ACME privé se trouve en général derrière un certificat privé |
| EAB key id and HMAC key | External Account Binding : le compte sous lequel cette gateway s'enregistre. ZeroSSL et Google l'exigent, Let's Encrypt l'ignore |
| Contact e-mail | certaines autorités en exigent une ; l'expiration est surveillée par le récapitulatif quotidien |
| Terms accepted | un acte juridique, qui n'est donc jamais présumé |

Chaque autorité conserve sa clé de compte et ce qu'elle a émis dans **son propre recoin
du cache partagé** ; changer son annuaire lui en attribue un nouveau, puisqu'un compte
enregistré auprès d'une autorité ne vaut rien auprès d'une autre.

Les noms ne sont pas un champ du compte : ce sont des **commandes** parmi les
certificats (*Ask* suivi du nom de l'autorité), placées chacune sur la console, sur
l'application ou sur les deux, comme n'importe quel certificat. Une porte ne demande à
une autorité que les noms des commandes placées sur elle, et la liste est fermée : une
politique ouverte permettrait à n'importe qui de joindre la gateway par son adresse
avec un SNI inventé, et d'épuiser le rate limit de l'autorité sur des noms qui
n'appartiennent à personne. Une autorité sans aucune commande placée n'est pas armée du
tout.

**Placer une commande envoie la demande immédiatement**, en arrière-plan, par le chemin
qu'emprunterait une négociation TLS - verrou de cluster compris - au lieu d'attendre que
le premier visiteur subisse tout l'échange, ou son échec. Le nœud qui a fait la demande
garde la trace de ce qui s'est passé (demande en cours, ou refus de l'autorité, avec
son explication) pour que l'écran l'affiche ; le certificat lui-même arrive dans le
cache partagé, que tous les nœuds lisent. Les commandes et les autorités voyagent avec
un export de configuration - la clé HMAC d'un compte sous la forme d'un `$name`, jamais
d'un littéral ; ce que l'autorité a émis reste dans le cache.

## Un certificat depuis le système de fichiers

Sur Kubernetes, Swarm ou compose, le certificat que la gateway doit présenter existe le
plus souvent déjà, créé et renouvelé par la plateforme : cert-manager l'écrit dans un
Secret, l'autorité de service d'OpenShift en écrit un pour un Service, Swarm fournit un
secret, compose monte un volume. La gateway ne demande pas où : elle lit sa propre **table
des montages**, qui liste tout ce que le déploiement a monté dans son conteneur, quel que
soit l'endroit choisi par le manifeste.

**Add certificate, From the file system** montre les montages qui contiennent un
certificat, chacun avec la clé privée trouvée à côté - un certificat sans clé ne sert rien
et ne peut pas être choisi. Les montages qui ne sont jamais une PKI sont écartés : les
pseudo-fichiers du processus, ce que reçoit tout conteneur (`/etc/hosts`,
`/etc/resolv.conf`), le jeton du ServiceAccount, et le dossier de données, où se trouvent
la base et la clé du coffre. Le contenu d'un fichier n'est jamais montré : ce qu'il est,
ses noms, sa fin.

Seuls les **chemins** sont gardés : la clé reste dans son fichier et n'entre jamais en
base. Chaque nœud lit ses propres fichiers, toutes les trente secondes, et recharge ses
portes quand ils ont changé - un renouvellement par cert-manager ou par l'autorité de
service est servi sans redémarrage et sans que personne n'enregistre rien. L'écran
affiche la nouvelle date de fin.

Un certificat du système de fichiers se place comme les autres. Le plan **Internal**
n'accepte qu'un certificat qui porte l'un des noms par lesquels les services l'appellent - lu
auprès du runtime, un joker compte -, et il y ouvre la porte HTTPS sur `:7443`, et le tiroir OIDC / JWT comme la page du scheduler nomment
dès lors l'adresse en `https://`. Les services qui l'appellent doivent faire confiance à
l'autorité qui l'a signé - le cluster la distribue généralement déjà : une ConfigMap
annotée `service.beta.openshift.io/inject-cabundle` sur OpenShift, un bundle trust-manager
avec cert-manager.

Avec le chart Helm, le Secret est monté dans `/etc/meerkat/tls/internal` :

```yaml
internalTls:
  # un Secret kubernetes.io/tls existant, de cert-manager par exemple
  secretName: meerkat-internal-tls
  # ou, sur OpenShift, laisser l'autorité de service en émettre et en renouveler un
  openshiftServingCert: true
```

::: details Un Certificate cert-manager pour le plan interne
```yaml
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: meerkat-internal
spec:
  secretName: meerkat-internal-tls
  dnsNames:
    - meerkat-internal
    - meerkat-internal.shop
    - meerkat-internal.shop.svc
    - meerkat-internal.shop.svc.cluster.local
  issuerRef:
    name: cluster-ca
    kind: ClusterIssuer
```
:::

## Demandé à cert-manager

Sur Kubernetes, la façon habituelle de donner son certificat au plan interne est celle
ci-dessus : un Certificate déclaré avec le déploiement, son Secret monté. **Add
certificate, Ask cert-manager** est la voie de secours, pour un cluster qui a cert-manager
et personne pour écrire ce manifeste. Partout ailleurs, l'entrée est grisée avec sa raison :
sur Swarm ou compose, sans cert-manager, ou quand le chart ne l'a pas autorisée.

Les plans Console et Application la proposent aussi, **derrière un proxy placé devant**
qui les joint en rechiffrant : le certificat porte alors les noms du plan à l'intérieur
du cluster, pour le saut depuis le proxy. Un plan que la gateway sert elle-même ne la
propose pas : ses noms publics sont l'affaire d'une autorité publique (ACME), d'un import
ou du système de fichiers.

Avec un seul émetteur de ce genre, le clic le sollicite aussitôt - supprimer le
certificat est le retour en arrière ; avec plusieurs, la console choisit l'**émetteur**, parmi les ClusterIssuers et les Issuers du
namespace que la gateway a trouvés prêts et qui signent avec une autorité - `ca`, `vault`,
`venafi`. Un émetteur `selfSigned` est écarté, son certificat étant sa propre autorité,
une nouvelle à chaque renouvellement ; un émetteur ACME aussi, une autorité publique ne
voyant aucun des noms du cluster. Tout le reste est décidé par la gateway : les noms
sont ceux que le runtime résout vers le plan, le Certificate est créé dans son propre
namespace, sous le seul nom que le chart autorise pour ce plan -
`<release>-meerkat-internal-issued`, `<release>-meerkat-console-issued`,
`<release>-meerkat-app-issued` - avec une nouvelle clé ECDSA à chaque renouvellement.
L'API prend le plan en `?plane=` sur `GET /api/certificates/cert-manager` et en `"plane"`
dans le corps du `POST`, le plan interne quand rien n'est dit.

Le Secret qu'écrit cert-manager est **lu par l'API et surveillé**, pas monté : pas de
Deployment à modifier, pas de redémarrage. Quand cert-manager renouvelle - il réécrit le
même Secret avant l'échéance -, le serveur API prévient chaque nœud, qui recharge sa porte.
Chaque nœud garde la clé en mémoire ; la base garde le nom du Secret et le dernier
certificat, jamais la clé. Le plan affiche qui signe - `cert-manager: <émetteur>` - et quand il sera renouvelé, ou la
raison donnée par cert-manager quand il refuse. Demandé depuis le plan Internal, il y est
placé quand ce plan est vide ; demandé depuis la Console ou l'Application, il est placé sur
ce plan. Le supprimer supprime le Certificate et son Secret.

**Sous une autorité à contraintes de noms.** Une autorité de cluster peut ne permettre que
les noms du cluster (`svc`, `cluster.local`, le namespace). L'émetteur CA de cert-manager
signe tous les noms qu'on lui demande, le nom nu du Service (`meerkat-meerkat`) compris, et
un seul nom hors contraintes fait refuser le certificat entier par tout vérificateur (Go,
OpenSSL), quel que soit le nom appelé. Alors, quand le Secret change, la gateway lit les
contraintes de la chaîne qu'il porte, ramène le Certificate aux noms qu'elles permettent, et
cert-manager l'émet de nouveau ; redemandé depuis la console, les noms refusés ne reviennent
pas. Un nom que l'autorité permet reste, même nu (`meerkat-meerkat-internal`). Quand elle
n'en permet aucun, le Certificate reste tel quel et sa ligne dit quels noms sont refusés, et
par quelles contraintes. Sans contraintes, rien ne change.

```yaml
internalTls:
  # autorise la gateway à créer ses Certificates, lire leurs Secrets, lister les émetteurs
  certManager: true
```

Le chart accorde alors, dans le namespace de la gateway, la création de Certificates -
Kubernetes ne sait pas restreindre une création à un nom - et tout le reste sur les trois
seuls noms ci-dessus : leurs Certificates, leurs Secrets. Lister les ClusterIssuers est une
lecture à l'échelle du cluster. Avec [approver-policy](https://cert-manager.io/docs/policy/approval/approver-policy/),
les demandes du ServiceAccount de la gateway peuvent être limitées à ses propres noms.

## Appeler des upstreams en HTTPS

Les certificats ci-dessus sont ce que la gateway **présente**. Quand c'est la
gateway qui appelle - une route dont la cible est `https://...` -, elle
**vérifie** le certificat que présente l'upstream, et il lui faut pour cela
l'autre moitié : les autorités en qui elle a confiance. Elles ont leur propre
dépôt, l'onglet **Trusted authorities** de l'écran Domains & TLS, à part des certificats
que présente la gateway : des certificats publics uniquement, jamais une clé.

Par défaut, la gateway fait confiance aux autorités publiques que porte son
image. Un upstream interne à un cluster est le plus souvent signé par
l'autorité du cluster ou par celle de l'entreprise, qu'aucune image ne connaît :
ajoutez-la, et les routes vers cet upstream se vérifient. Elle s'ajoute aux
autorités publiques, sans jamais les remplacer : un upstream public continue de
se vérifier.

Une autorité s'ajoute dans une seule fenêtre : un nom, et son certificat - un
fichier choisi ou déposé sur la fenêtre, ou le PEM collé. Un PEM peut contenir
plusieurs certificats - une racine et une intermédiaire, ou l'ancienne et la
nouvelle autorité le temps que l'une remplace l'autre. Une clé privée qu'il
contiendrait est refusée, avec la raison : ce que la gateway présente s'ajoute dans
l'onglet d'un plan.

Vérifier un upstream ne demande que le **certificat** de l'autorité, jamais sa
clé privée - celle-là signe les certificats des Services, et reste dans la PKI
du cluster (cert-manager, une autorité d'entreprise). Prenez le certificat
qu'elle publie.

### Trouvées dans le cluster

Sur Kubernetes et OpenShift, la gateway trouve les autorités que le cluster
publie, quel que soit leur nom : **From the cluster**, dans l'onglet Trusted
authorities. S'il y en a plusieurs, vous choisissez. Chaque source, et ce
qu'elle demande :

| Source | Où la gateway la trouve | Droit dans le chart |
|---|---|---|
| Autorité des services d'OpenShift | `service-ca.crt`, monté dans chaque pod : l'autorité des certificats des Services | aucun |
| cert-manager | les ClusterIssuers, et les Issuers de son namespace, de type `ca`, `vault` ou `venafi` | `trustDiscovery.certManager` (actif par défaut) |
| trust-manager | les Bundles dont la ConfigMap atteint son namespace | `trustDiscovery.trustManager` (actif par défaut) |
| Autorité du cluster Kubernetes | `ca.crt`, monté dans chaque pod : elle signe l'API server, rarement les services | aucun |

**La clé d'un émetteur n'est jamais lue.** Un émetteur cert-manager de type CA
range son certificat et sa clé dans un même Secret, et Kubernetes accorde un
Secret en entier : le droit d'y lire le certificat serait le droit d'y lire la
clé qui signe tous les services. La gateway demande donc à l'émetteur un
certificat jetable, `<release>-meerkat-trust-probe`, pour un nom que rien ne
résout ; cert-manager écrit à côté le certificat de l'émetteur (`ca.crt`), la
gateway le prend, et supprime les deux. Le chart accorde la création de
Certificates dans le namespace de la gateway - Kubernetes ne sait pas accorder
une création sur un seul nom - et la lecture et la suppression de ce seul
Certificate et de son Secret.

**Ce que chacune signe ici** s'affiche sous elle, pour aider à choisir - à titre
indicatif, rien n'est décidé à partir de là. Chaque route en https que la
gateway refuse aujourd'hui est appelée une fois (une route déjà acceptée, ou
qui ne répond pas, ne dit rien du choix de l'autorité) : le certificat que présente son upstream est rattaché à l'autorité
qui l'a signé, par le certificat de cette autorité, ou, pour un émetteur
cert-manager, par les noms que cert-manager enregistre comme signés par lui. Et
les services : chaque `Certificate` cert-manager nomme son émetteur et ses
dnsNames (listés dans le namespace de la gateway et dans
`rbac.watchNamespaces` ; un Certificate ne contient aucune clé), et OpenShift
marque d'une annotation chaque Service à qui il donne un certificat. Un service
qui ne l'a pas encore demandé, ou qui n'est pas déployé, ne se voit pas.

**Les routes encore en http** sont proposées en https une fois une autorité
prise, quand leur Service répond en TLS sur l'un de ses ports, sous un nom que
porte son certificat, avec une chaîne que la gateway accepte maintenant. Le
port et le nom sont essayés, pas devinés : le port https d'un service est
rarement son port http. Chacune reste à basculer par l'opérateur.

Le certificat pris est **copié**, comme un import : une autorité renouvelée à sa
source se reprend. Sur Swarm, compose ou un binaire nu, rien ne publie
d'autorité, et le bouton n'est pas proposé.

**Ce que garantit une autorité n'est pas un nom DNS.** Vérifier un upstream pose
deux questions : son certificat remonte-t-il à une autorité de confiance, et
porte-t-il le nom qu'appelle la route - l'hôte de l'URL de la cible, par exemple
`orders.shop.svc.cluster.local` ? La première relève de ce dépôt ; la seconde,
du certificat de l'upstream lui-même. Noms internes et externes ne se croisent
jamais ici : le certificat que la gateway présente aux navigateurs ne joue aucun
rôle dans ce qu'elle appelle, et une autorité d'entreprise qui signe des
certificats internes comme externes tient en une seule entrée. Une route qui
appelle `https://orders:8443` alors que le certificat ne nomme que
`orders.shop.svc` échoue sur le nom, même avec la bonne autorité en place.

## Ce que contient la base de données

| Stocké | Comment |
|---|---|
| Le certificat lui-même | en clair - c'est ce que la gateway présente à chaque visiteur |
| La clé privée | **scellée** avec la clé maîtresse du coffre. Elle ne quitte jamais la gateway : ni dans une réponse d'API, ni dans un export, ni dans un journal |
| Un certificat du système de fichiers | ses deux chemins, et le dernier certificat lu - jamais sa clé |
| Un certificat de cert-manager | le nom de son Secret, et le dernier certificat lu - jamais sa clé |
| La demande de signature en attente | en clair, et téléchargeable |
| La clé du compte ACME et les certificats émis | scellés, dans le cache ACME |
| Une autorité de confiance | en clair - un certificat public |

La console vérifie aussi en permanence que le certificat répond bien pour l'hôte sous
lequel il a été enregistré. Un certificat qui ne couvre rien échoue au moment de la
négociation TLS, là où personne ne fait le rapprochement avec cet écran.

## En cluster

> [!WARNING]
> Les clés des certificats et la clé du compte ACME sont scellées avec la clé maîtresse
> du coffre, qui est un **fichier local** tant que `MEERKAT_VAULT_KEY` n'est pas
> définie. Un nœud qui possède une autre clé ne peut pas les ouvrir, et il le dit :
> *the private key cannot be unsealed - is this the vault key it was written with?*
> Donnez la même clé à tous les nœuds.

L'émission est sérialisée par un verrou consultatif. Auparavant, cinq nœuds qui
redémarraient ensemble épuisaient en une seconde le quota hebdomadaire de doublons de
l'autorité ; désormais, le nœud qui attend trouve le certificat que le premier vient
d'écrire. Le verrou n'est pris qu'à la première négociation pour un nom et à l'approche
du renouvellement - le placer devant chaque négociation coûterait un aller-retour vers
la base de données pour éviter un événement qui se produit deux fois par an.

Un certificat écrit sur un nœud est rechargé par les autres grâce au bus de changement.

## Avant l'expiration d'un certificat

La console affiche le compte à rebours de chaque certificat, et le
[récapitulatif quotidien](/docs/console/mail-relay) l'envoie par e-mail aux
administrateurs : d'abord les certificats qui expirent dans son horizon, puis ceux qui
ont déjà expiré. Un certificat automatique qui atteint cette fenêtre est un certificat
dont le renouvellement échoue. Un certificat en réserve dont les noms sont portés par un
certificat de plus longue validité - ce qu'un renouvellement laisse derrière lui - n'est
pas cité.

## Ce qui manque

- **HTTP/3** (SSL-07) : il faudrait une dépendance QUIC extérieure à la bibliothèque
 standard, une écoute UDP et `Alt-Svc`.
- **Les autorités de confiance ne servent qu'aux upstreams** (SSL-09). Les autres
 connexions qu'ouvre la gateway - un annuaire en LDAPS, un fournisseur OpenID Connect,
 le relais mail, le collecteur, un emplacement git - ne font encore confiance qu'aux
 autorités publiques ; leur expiration n'est pas encore dans le récapitulatif quotidien.
- **Un certificat client pour un upstream qui en demande un** (SSL-10, mTLS).
- **Les commandes d'une autorité ACME sur le plan interne** : refusées, puisqu'une
 autorité publique ne voit pas les noms du cluster.
