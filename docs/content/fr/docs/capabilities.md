---
title: Ce que fait Meerkat
section: Démarrer
order: 1
summary: Chaque capacité dans les grandes lignes, ce qu'elle demande sans Meerkat, l'édition qui l'apporte, et ce qui est prévu ensuite.
---

# Ce que fait Meerkat

Tout ce que fait la gateway, dans les grandes lignes, à côté de ce que le même
résultat demande sans elle : le code à écrire, le service à faire tourner, le
port à ouvrir. Chaque ligne mène à sa page.

**Community** est l'image gratuite. **Enterprise** est l'image payante ; l'image
d'évaluation est Enterprise, gratuite, pour tout essayer d'abord
([les éditions](/product/editions)). *En partie* signale une capacité utilisable
aujourd'hui, dont une partie nommée reste à venir. Ce que coûtent les
alternatives assemblées est dans [le dossier Meerkat](/product/the-case).

## Vos API, pilotées par un agent

| Capacité | Ce que fait Meerkat | Sans Meerkat | Édition |
|---|---|---|---|
| [Les API de vos services par MCP](/docs/agent/overview#appeler-les-services-derrire-les-routes) | Tout assistant connecté à la gateway lit l'OpenAPI du service derrière une route, puis l'appelle pour de vrai à travers la gateway, ses règles et l'identité qu'elle transmet, sous n'importe quel utilisateur ou rôle. Chaque appel est audité. | Un serveur MCP dans **chaque** service, sa propre authentification et sa gestion des jetons, et son port exposé sur le cluster - par service, par langage. | Community |
| [La gateway elle-même par MCP](/docs/agent/overview) | Tout ce que fait la console, fait par un assistant à travers les mêmes endpoints et avec les règles imposées à une personne : routes, rôles, organisations, thèmes, réglages, clients OIDC, configurations entières et leur historique. Un jeton en lecture seule le reste, un point de reprise précède chaque changement, et le peu qui reste fermé aux agents - clés privées, valeurs du coffre, portes TLS - dit pourquoi. | Des serveurs MCP communautaires pour chaque outil, ou celui hébergé par un éditeur, et pas d'audit commun. | Community |
| [La connexion de l'agent](/docs/console/access-and-agents) | OAuth sans secret à recopier, ou un jeton limité à un périmètre et à des plages réseau, révoqué en un clic. | Un identifiant par outil, rangé dans la configuration de chaque assistant. | Community |

## Connexion et identité

Les façons d'entrer, toutes servies par la gateway à vos couleurs, en 20 langues.

| Méthode | Édition |
|---|---|
| [Comptes locaux et politique de mots de passe](/docs/auth/local-accounts) | Community |
| [Double facteur : application d'authentification, codes de secours, navigateurs de confiance](/docs/auth/mfa) | Community |
| [Passkeys](/docs/auth/passkeys), en premier facteur | Community, *en partie* |
| [Connexion par code e-mail](/docs/auth/email-code) | Community, *en partie* |
| [OpenID Connect](/docs/auth/oidc) : Entra ID, Okta, Google, Keycloak, tout fournisseur conforme | Community |
| [GitHub](/docs/auth/github) | Community |
| [LDAP et Active Directory](/docs/auth/ldap) | Enterprise |
| [SAML 2.0](/docs/auth/saml) | Enterprise |
| Règles de groupe : un groupe d'annuaire, une équipe GitHub ou un claim OIDC devient une appartenance et des rôles | Enterprise |
| [Meerkat comme fournisseur OpenID de vos applications](/docs/auth/oidc-provider) | Enterprise |
| Kerberos et SPNEGO | prévu, Enterprise |

| Capacité | Ce que fait Meerkat | Sans Meerkat | Édition |
|---|---|---|---|
| [Les pages de connexion](/docs/auth/flow-pages) | Connexion, mot de passe oublié, vérification d'e-mail, inscription facultative, chaque facteur ci-dessus, servies dans votre thème et traduites. | Un serveur d'identité - Keycloak, Zitadel, Authentik - ou un service - Auth0, WorkOS, Clerk - plus un thème à construire et à maintenir. | Community |
| [Les sessions](/docs/auth/sessions) | Une session pour toutes les applications derrière la gateway, révocable depuis la console. | Chaque application gère la sienne, ou un client OIDC dans chacune. | Community |
| [Les jetons d'API](/docs/auth/api-tokens) | Jetons personnels et de machine, secret montré une fois, révocables. | Un système de jetons dans chaque service, ou les client credentials d'un serveur d'identité. | Community |
| [Les applications sans connexion propre](/docs/auth/connect-an-application) | Placées derrière une route, c'est la gateway qui connecte leurs utilisateurs. | oauth2-proxy ou Pomerium devant chacune. | Community |
| [L'authentification unique dans les interfaces tierces](/docs/auth/oidc-provider) | La console d'un broker, une interface de base de données, toute application qui parle OpenID Connect connecte ses utilisateurs par la gateway : pas d'écran de consentement, la route décide qui entre et avec quels rôles. | Un realm Keycloak et un client par application, les rôles rattachés à la main. | Enterprise |

## Accès et organisations

| Capacité | Ce que fait Meerkat | Sans Meerkat | Édition |
|---|---|---|---|
| [Rôles et groupes](/docs/access/roles) | Un catalogue de rôles hiérarchique, des groupes par organisation, cumulés ou choisis à la connexion. | Des tables de rôles dans chaque service, ou les rôles d'un serveur d'identité rattachés à la main dans chacun. | Community |
| [Les organisations](/docs/access/tenants) | Des organisations clientes isolées par la gateway, avec leurs membres et leurs admins. | Des écrans d'administration à construire, ou WorkOS, Frontegg. | une organisation Community, plusieurs Enterprise |
| [La règle d'accès d'une route](/docs/access/route-access) | Qui atteint une application - connecté, organisation, rôles, personnes nommées - décidé à l'entrée ; un refus mène à une page qui explique. | Une vérification dans chaque application, réécrite pour chaque framework. | Community |
| [La sécurité par endpoint](/docs/access/endpoint-security) | Des règles par opération, posées sur l'OpenAPI de la route, refusées avant que le service soit appelé. | Du code dans le service, ou une politique de gateway écrite à la main chemin par chemin. | Community |
| [Les rôles que déclare une application](/docs/access/endpoint-security#les-rles-que-lapplication-dclare) | Lus dans sa propre spec, affichés par opération, importés au catalogue et transmis sous les noms de l'application à ceux qui les détiennent. | Des rôles recopiés à la main entre le code et le serveur d'identité, et tenus à jour. | Community |
| [L'identité transmise à vos services](/docs/auth/service-jwt) | En-têtes, REMOTE_USER ou JWT signé avec des clés publiées qui tournent ; vous choisissez les rôles que reçoit chaque service et leur forme. | Chaque service valide un jeton et traduit lui-même les claims. | Community |
| [L'administration déléguée](/docs/access/capabilities) | Super-admin, admins d'infrastructure, d'application et d'organisation, chacun avec des limites claires. | Des droits sur chaque outil, gérés outil par outil. | Community |

## Routage et trafic

| Capacité | Ce que fait Meerkat | Sans Meerkat | Édition |
|---|---|---|---|
| [Les routes](/docs/concepts/routes) | Modifiées à chaud depuis la console, sur chaque nœud en moins d'une seconde ; 11 [prédicats](/docs/predicates/overview), 34 [filtres](/docs/filters/overview). | Kong, APISIX, Traefik ou Envoy Gateway, configurés dans des fichiers et redéployés. | Community |
| [Limites de débit et quotas](/docs/operations/rate-limits) | Par route, utilisateur, jeton, organisation ou adresse, et par endpoint. | Un plugin par gateway, souvent dans son édition payante. | Community, *en partie* |
| [La santé des amonts](/docs/operations/upstream-health) | Disjoncteur, timeouts à trois niveaux, un cœur par route venu de la découverte et du trafic réel. | La configuration d'un mesh ou d'une gateway, plus un tableau de bord pour la lire. | Community |
| WebSocket, gRPC, streaming | De bout en bout ; un appel gRPC compté par son `grpc-status`. | Des réglages de proxy par protocole. | Community, *en partie* |
| [Réponses et fichiers servis par la gateway](/docs/filters/respond) | Une route répond d'elle-même à partir d'un gabarit, ou sert les fichiers que vous déposez. | Un petit service déployé pour chacun. | Community |
| [La maintenance](/docs/filters/maintenance) | Une page par route ou pour toute la plateforme, traduite, avec une porte pour les administrateurs. | Une page de maintenance par application, et un redéploiement pour l'afficher. | Community |

## Dans vos applications

| Capacité | Ce que fait Meerkat | Sans Meerkat | Édition |
|---|---|---|---|
| [Bouton utilisateur et portail](/docs/concepts/data-plane-chrome) | Profil, déconnexion, organisation, langue, clair ou sombre, et la navigation entre vos applications, ajoutés aux pages que sert la gateway. | À construire dans chaque application. Aucun produit ne le fait. | Community, barre de navigation Enterprise |
| [CSS par rôle, votre CSS et votre JavaScript](/docs/customise/injections) | Les rôles posés sur la page côté serveur, et votre propre code par route. | Du code front dans chaque application. | Community |
| [Notifier vos utilisateurs](/docs/operations/notifications) | Un service envoie une requête ; ses utilisateurs l'apprennent dans l'application qu'ils ont ouverte, avec un compteur, un snackbar, une notification du bureau, un mail si demandé, et un acquittement quand il compte. | Un service de notifications et sa boîte de réception dans chaque application, ou Novu, Knock, Courier. | Enterprise, *en partie* |
| Signalement d'anomalies | Une capture d'écran et son contexte technique, suivis dans la console. | Sentry auto-hébergé, ou Marker.io, Jam, Userback. | Community |
| [Thème et marque](/docs/customise/theme) | Thèmes Material 3, clair et sombre, votre logo et votre fond ; plus de mises en page et sans la marque Meerkat avec Enterprise. | Un thème par outil. | Community, mises en page Enterprise |

## Sécurité de la plateforme

| Capacité | Ce que fait Meerkat | Sans Meerkat | Édition |
|---|---|---|---|
| [TLS](/docs/operations/tls) | Des certificats plan par plan, générés, importés ou signés sur demande, des ports ouverts à chaud. | cert-manager, ou des fichiers sur chaque proxy. | Community |
| [Certificats automatiques](/docs/operations/tls) | ACME : Let's Encrypt, ZeroSSL, Google ou votre propre step-ca, plusieurs côte à côte. | cert-manager, ou une offre de gateway plus chère. | Enterprise |
| [Le coffre](/docs/operations/vault) | Des secrets chiffrés, désignés par leur nom dans la configuration, avec un rappel avant expiration. | Vault, OpenBao, Infisical ou Doppler, branché à chaque outil. | Community |
| En-têtes de sécurité, redirection HTTPS, CSRF | Appliqués à l'entrée. | Configurés par application et par proxy. | Community |

## Exploitation

| Capacité | Ce que fait Meerkat | Sans Meerkat | Édition |
|---|---|---|---|
| [Les configurations](/docs/console/configuration) | Versions nommées, comparaison, export et import YAML, un point de reprise à chaque changement ; dans un dépôt git avec Enterprise. | Une chaîne GitOps à mettre en place. | Community, git Enterprise |
| [La piste d'audit](/docs/operations/audit) | Chaque action d'administration avec le détail champ par champ, chaque connexion et chaque changement de facteur ; envoyée en OTLP avec Enterprise. | Retraced ou WorkOS Audit Logs, puis chaque outil à y brancher. | Community, *en partie* |
| [Trafic et métriques](/docs/operations/traffic) | Trafic, latence et échecs par route et par endpoint, rien à installer ; métriques en OTLP avec Enterprise. | Prometheus et Grafana. | Community |
| [Journaux](/docs/operations/logs) et [traces](/docs/operations/tracing) | Journaux structurés et journal d'accès, refus compris ; un identifiant de trace sur chaque requête, la gateway dans la trace avec Enterprise. | Chaque pièce instrumentée séparément. | Community, *en partie* |
| [Les appels planifiés](/docs/operations/scheduler) | Une route, un chemin et une cadence : appel fait au moins une fois, relancé quand c'est trop tôt, historique dans la console. | Des cron jobs, ou un ordonnanceur dans chaque service ; Temporal, Inngest. | Community |
| [Plusieurs instances de la gateway](/docs/deploy/kubernetes) | Actif/actif sur PostgreSQL, sans affinité de session, sans nœud principal. | Des sessions collantes et un stockage partagé à concevoir. | Enterprise |
| [Le déploiement](/docs/deploy/shapes) | Une image, un stockage embarqué par défaut, Docker, Swarm ou Kubernetes avec un chart Helm. | Un composant par besoin ci-dessus, chacun déployé et mis à jour. | Community |

## Pour vos développeurs

| Capacité | Ce que fait Meerkat | Sans Meerkat | Édition |
|---|---|---|---|
| [Un processus local dans le cluster](/docs/operations/plug) | plug : le service sur le poste d'un développeur prend la place de celui déployé, puis le cluster revient tel qu'il était. Gratuit et autonome à côté de la gateway ; intégré à elle, avec des clés SSH et chaque page qui signale la substitution, avec Enterprise. | mirrord, Telepresence ou Gefyra, installés et lancés à part de la gateway. | plug gratuit, intégré Enterprise, *en partie* |
| [La doc d'API de chaque route](/product/dev-mode) | Un Swagger UI par route, des appels qui passent par la gateway, *Try it out* sous n'importe quel utilisateur, groupe ou rôle, avec les rôles qu'exige chaque opération. | Un Swagger UI par service, hors des vraies règles. | Community |
| Mode test de l'interface | Naviguer avec une identité simulée pour voir ce que voit un rôle. | Des comptes de test à créer pour chaque rôle. | Community |

## Ce qui vient ensuite

Prévu, et pas encore construit (le détail est dans la [feuille de route](/project/roadmap)) :

- **Un assistant de découverte** : parcourir Docker ou un namespace Kubernetes, choisir un conteneur, obtenir une route.
- **Se connecter en tant que** : ce que fait un support tous les jours.
- **Un cache de réponses.**
- **Servir une application sous un sous-chemin sans la reconstruire.**
- **gRPC complet** : sécurité par méthode, et gRPC-Web.
- **Un portail par organisation** : l'icône, le titre et la disposition propres à chaque client.
- **HTTP/3**, dès que le gain se mesure.
- **Un coffre externe** : HashiCorp Vault, secrets Kubernetes, secrets Docker.
- **Les signalements poussés vers GitHub, GitLab ou Jira.**
- **Le Web Push** : une notification du bureau qui atteint une personne même quand aucune application n'est ouverte.
- **Kerberos et SPNEGO**, Enterprise.
