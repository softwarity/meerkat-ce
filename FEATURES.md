# Meerkat - fonctionnalités

> **Ce fichier est la seule liste de ce que Meerkat fait.** Une ligne par fonctionnalité,
> son état réel, et l'édition qui la porte. Il remplace `requirements.md`,
> `authentication.md` et `licensing.md`, supprimés pour qu'aucune autre page ne raconte
> une version différente du produit.
>
> **L'état se lit dans le code, jamais ici.** Une case cochée est une promesse : elle se
> vérifie en faisant tourner la chose. Livrer, c'est cocher dans le même commit.

## Comment lire le tableau

| Colonne | Ce qu'elle dit |
|---|---|
| **Fait** | `[x]` livré et utilisable, `[~]` partiel - la description dit ce qui manque, `[ ]` rien dans le code |
| **ID** | identifiant stable, **jamais réutilisé** pour un autre sujet ; c'est ce qu'on cite dans un commit ou une discussion |
| **Mot-clé** | de quoi retrouver la ligne d'un coup d'œil |
| **Description** | ce que la fonctionnalité fait, en une phrase |
| **Ce qui manque** | pour une ligne non cochée : ce qui bloque ou ce qui n'a pas été écrit, en une phrase - le détail et la solution sont sous le tableau |
| **Éd.** | `CE` les deux images, `EE` l'image Enterprise seule, `CE/EE` une base communautaire et un cran payant |

Sous le tableau : **ce qui demande des précisions** - les points durs, ce qui bloque, et la
solution retenue quand il y en a une - puis les **décisions structurantes** qui expliquent la
forme du produit.

## Le tableau

### Authentification

| Fait | ID | Mot-clé | Description | Ce qui manque | Éd. |
|:---:|---|---|---|---|:---:|
| [~] | AUTH-01 | **Pages du flux** | La gateway sert elle-même les pages du flux utilisateur (login, mise à jour de mot de passe, vérification/enrôlement TOTP, sélection | la page de sélection du dev à tester (DEV-06) | CE |
| [x] | AUTH-02 | **Mot de passe** | Authentification par formulaire (login/mot de passe) contre la base locale des utilisateurs | - | CE |
| [x] | AUTH-03 | **LDAP / AD** | Fédération LDAP/LDAPS (activable) : en search-then-bind : un compte de service retrouve l'entrée, le bind vérifie le mot de passe, l'e-mail | - | EE |
| [x] | AUTH-04 | **OIDC** | SSO d'entreprise OIDC (activable) : délégation du premier facteur uniquement à tout IdP conforme OIDC (Keycloak, Entra ID, Okta | - | CE |
| [~] | AUTH-05 | **Étapes de connexion** | Flux de connexion multi-étapes ordonné : (1) mise à jour du mot de passe si expiré/temporaire -> (2) MFA -> (3) sélection d'organisation -> | l'organisation est réclamée par route et non comme une étape ; on ne revient jamais sur le groupe | CE |
| [~] | AUTH-06 | **JWT vers l'amont** | Émission de JWT signés vers l'amont : la gateway signe un jeton d'identité court (TTL configurable, 2 min par défaut) pour l'appel qu'elle | un endpoint d'échange rendant un couple access/refresh, si un client le réclame | CE |
| [x] | AUTH-07 | **Rotation des clés** | Rotation des clés JWT sans invalider brutalement les tokens en cours (publication multi-clés type JWKS avec kid). Console : bouton **JWT** à côté de Global sur Routes, son propre tiroir - trois paires distinctes (une par algorithme, pas une clé en trois formats), le JWKS recommandé, chaque clé publique en repli, et les routes qui signent avec chacune | - | CE |
| [~] | AUTH-08 | **Cookie de session** | Session web par cookie httpOnly, Secure, SameSite ; domaine calculé pour être partageable entre sous-domaines de l'application ; identité | aucun attribut Domain, donc pas de partage entre sous-domaines ; Secure repose sur `r.TLS` seul | CE |
| [~] | AUTH-09 | **Jetons d'API** | Tokens d'application (personal access tokens / machine-to-machine) : créés par l'utilisateur, secret affiché une seule fois | le choix explicite du groupe, et un écran console pour les jetons du plan de données | CE |
| [x] | AUTH-10 | **Politique de mots de passe** | Politique de mots de passe configurable : longueur min, minuscules/majuscules/chiffres/spéciaux min, historique (non-réutilisation des N | - | CE |
| [x] | AUTH-11 | **Anti-force brute** | Étranglement des tentatives plutôt qu'un verrouillage de compte : après N échecs dans une fenêtre glissante (N et durée configurables), le compteur vivant **en base** (`login_attempts`, une ligne par échec) : cinq essais, c'est cinq pour l'installation et non cinq par nœud, et un redémarrage ne pardonne plus | - | CE |
| [x] | AUTH-12 | **Auto-inscription** | Auto-inscription (self-registration) : livrée en option, fermée par défaut et adossée à l'autorité locale (AUTH-24) - elle se ferme avec | - | CE |
| [~] | AUTH-13 | **Historique de connexions** | Journalisation de toutes les connexions (succès et échecs) : IP, user-agent (OS/navigateur), organisation, horodatage, motif d'échec | les échecs ne sont pas journalisés, pas de vue root, historique plafonné à 50 par compte | CE |
| [~] | AUTH-14 | **Déconnexion** | Déconnexion propre : la session est détruite côté serveur, pas seulement le cookie effacé, et les jetons d'application (AUTH-09) sont | la liste de ses sessions actives et un « se déconnecter partout » | CE |
| [~] | AUTH-15 | **Passkeys** | Support WebAuthn/Passkeys (clés de sécurité physiques, empreinte, Windows Hello...) comme méthode passwordless de premier facteur, en flux | la récupération quand l'unique passkey est perdue | CE |
| [ ] | AUTH-16 | **Code par e-mail** | Code à usage unique par e-mail (OTP mail / magic link) comme méthode d'authentification ou de récupération, si SMTP configuré | tout : ni code à usage unique, ni lien magique | CE |
| [~] | AUTH-17 | **Passwordless** | Objectif passwordless : un compte doit pouvoir fonctionner sans mot de passe du tout (ex. passkey seule) ; la politique de sécurité permet | aucun compte local sans mot de passe, aucun réglage pour l'imposer, aucune récupération | CE |
| [~] | AUTH-18 | **Non-cumul des facteurs** | Non-cumul des facteurs - l'autorité qui reconnaît le compte fait foi : il n'y a pas de colonne « source » sur le compte, mais un lien par | la vérification des claims `acr`/`amr`, qui remplacerait la déclaration par une preuve | CE |
| [ ] | AUTH-19 | **SAML** | SAML 2.0 (SP-initiated, activable) pour les entreprises dont l'IdP n'expose pas OIDC ; même principe : authentification seule, rien d'autre | tout le protocole (AuthnRequest, réponse signée, métadonnées SP) : le kind existe, la fabrique refuse | EE |
| [ ] | AUTH-20 | **Kerberos** | Kerberos/SPNEGO (SSO Windows intégré) pour les environnements Active Directory homogènes | tout : aucune trace de SPNEGO ni GSSAPI | EE |
| [~] | AUTH-21 | **Mot de passe oublié** | Procédure « mot de passe oublié » (si SMTP configuré - NOTIF-01) : page dédiée servie par la gateway (vanilla, PAGE-01) ; envoi d'un lien | la limite est figée et partagée avec l'inscription ; jetons et navigateurs de confiance non révoqués ; un compte fédéré reçoit le lien | CE |
| [~] | AUTH-22 | **Vérification d'e-mail** | À la création d'un compte et à tout changement d'adresse, envoi d'un lien de confirmation à durée limitée (si SMTP | aucun mail de confirmation au changement d'adresse, délai figé à 24 h | CE |
| [~] | AUTH-23 | **Alertes de sécurité** | Notifications de sécurité à l'utilisateur (si SMTP configuré) : e-mail automatique lors d'une connexion depuis un nouveau navigateur ou une | rien sur une nouvelle IP, un nouvel appareil, un enrôlement MFA ou passkey ; pas de « ce n'était pas moi » | CE |
| [x] | AUTH-24 | **Autorité locale** | Les comptes locaux sont une autorité parmi les autres (kind local, plan infra, écran Authentification) : une seule liste répond à « par | - | CE |

### Second facteur

| Fait | ID | Mot-clé | Description | Ce qui manque | Éd. |
|:---:|---|---|---|---|:---:|
| [x] | MFA-01 | **TOTP** | TOTP (RFC 6238) : enrôlement par QR code (otpauth://), validation avant activation, désactivation possible, scratch codes de secours | - | CE |
| [ ] | MFA-02 | **Code de secours par e-mail** | Code TOTP de secours envoyé par e-mail si SMTP configuré | tout : le chemin MFA ne touche jamais le relais mail | CE |
| [x] | MFA-03 | **Navigateurs de confiance** | Après un MFA réussi, l'utilisateur peut marquer le navigateur comme fiable (empreinte navigateur) pour une durée | - | CE |
| [~] | MFA-04 | **MFA obligatoire** | Le MFA peut être rendu obligatoire globalement, par autorité ou par utilisateur (tri-état qui hérite du niveau au-dessus), et pas seulement | un niveau par rôle, qui bute sur le même ordre d'étapes | CE |
| [x] | MFA-05 | **État du flux en base** | L'état interne du flux MFA (clés temporaires d'enrôlement, etc.) doit être partagé entre les nœuds d'un cluster (dans la V1 : en mémoire | - | CE |

### Autorisation

| Fait | ID | Mot-clé | Description | Ce qui manque | Éd. |
|:---:|---|---|---|---|:---:|
| [x] | RBAC-01 | **Rôles hiérarchiques** | Modèle de rôles nommés, avec hiérarchie (un rôle parent implique ses enfants) et tags de classement ; rôles « système » protégés contre la | - | CE |
| [x] | RBAC-02 | **Groupes** | Groupes de rôles rattachés à une organisation ; les rôles effectifs d'un utilisateur = union des rôles des groupes qui lui sont affectés | - | CE |
| [x] | RBAC-03 | **Groupe par organisation** | Mode de groupe par organisation : SINGLE (un seul groupe par membre, sélectionné à la connexion) ou MULTIPLE (cumul des groupes) | - | CE |
| [x] | RBAC-04 | **Périmètres d'administration** | Périmètres d'administration étanches : me (l'utilisateur sur lui-même, self-service sur /profile/ du plan de données), organization (le | - | CE |
| [x] | RBAC-05 | **Capacités du compte** | Flags utilisateur transverses portés par le compte : root (admin global), infraAdmin (plan routage), appAdmin (identité de l'application) | - | CE |
| [x] | RBAC-06 | **Accès par route** | Contrôle d'accès par route, sur deux axes croisés (ET) | - | CE |
| [~] | RBAC-07 | **Accès par endpoint** | Contrôle d'accès par endpoint au sein d'une route API : règles méthode+path (templates {var}, méthode *) posées sur l'inventaire | l'option deny-by-default : sans règle, une opération retombe sur l'accès de la route | CE |
| [ ] | RBAC-08 | **RBAC désactivable** | RBAC désactivable globalement (mode « tout utilisateur authentifié a accès ») | tout, mais l'équivalent se pose route par route : c'est un confort, pas un mécanisme | CE |
| [x] | RBAC-09 | **Propagation à l'amont** | Les rôles effectifs, l'organisation et l'identité sont propagés aux services amont dans le JWT signé | - | CE |
| [x] | RBAC-10 | **Règles de groupe** | Ce qu'une autorité externe déclare (un groupe LDAP, une équipe GitHub, un claim OIDC) devient une appartenance et des | - | EE |
| [ ] | RBAC-11 | **Impersonation** | Impersonation (« se connecter en tant que ») : un root - ou un admin d'organisation sur les membres de son organisation - peut voir | tout : aucun bandeau, aucune double identité à l'audit, aucun endpoint | CE |

### Organisations

| Fait | ID | Mot-clé | Description | Ce qui manque | Éd. |
|:---:|---|---|---|---|:---:|
| [x] | TENANT-01 | **Organisations** | Entité Organisation : créée par le root ou en self-service (si autorisé) ; activable/désactivable ; un utilisateur peut appartenir à | - | CE/EE |
| [~] | TENANT-02 | **Membres** | Relation membre typée ADMIN ou USER : promotion/rétrogradation, rattachement d'un compte existant, création directe de membres (mot de | le départ volontaire : seul un administrateur peut retirer une appartenance | CE |
| [x] | TENANT-03 | **Organisation active** | Sélection de l'organisation active à la connexion (0 org -> salle d'attente /account-pending, qui explique comment demander un accès, ou la | - | CE |
| [~] | TENANT-04 | **Heures ouvrées** | Plages d'accès métier (business access) : restriction d'accès par plages horaires, jours de semaine, dates de début/fin, fuseau ; définies | le contrôle en cours de session, et l'édition de la fenêtre par membre dans la console | EE |
| [~] | TENANT-05 | **TTL de session** | TTL de session hiérarchique et modifiable par utilisateur : valeur globale (défaut 30 min) -> surcharge par organisation -> surcharge par | l'édition des surcharges organisation et membre, et l'option « session prolongée » au login | CE |
| [x] | TENANT-06 | **Isolation** | L'isolation tenant est garantie côté gateway : toute requête porte l'organisation courante, les données d'une organisation ne sont jamais | - | CE |
| [~] | TENANT-07 | **Export d'une organisation** | Ce qui existe est l'export/import de la configuration de la gateway (CFG-01) : il emporte les organisations, leurs groupes et les réglages | l'export ciblé d'une organisation avec ses membres | CE |
| [x] | TENANT-08 | **Mono ou multi-organisation** | Mode mono-organisation, par défaut (2026-08-08) | - | EE |

### Amonts et routage

| Fait | ID | Mot-clé | Description | Ce qui manque | Éd. |
|:---:|---|---|---|---|:---:|
| [x] | ROUTE-01 | **Routes à chaud** | Routes dynamiques stockées en base, créées/modifiées/supprimées à chaud depuis la console, sans redémarrage ; le rechargement est **propagé à tous les nœuds** par le bus de changement (STORE-03) | - | CE |
| [x] | SVC-01 | **Route autoportante** | La route porte tout : l'entité Service intermédiaire a été cadrée puis retirée, parce qu'une route par service est le cas courant et qu'un | - | CE |
| [~] | ROUTE-02 | **Attributs de route** | Nom, type (API ou UI), amont porté par la route elle-même avec sa spec OpenAPI (SVC-01 - il n'y a pas d'entité | une description et des tags : le modèle stocké ne porte ni l'une ni les autres | CE |
| [~] | SVC-02 | **Découverte cluster** | La gateway interroge son environnement d'exécution et offre la liste des services au moment où l'on crée une route : **Swarm** et **Docker** par le socket, **Kubernetes** depuis l'intérieur du cluster (son propre namespace). Une entrée par service ET par port, avec l'état déclaré (prêts / voulus) ; la saisie libre reste, pour un amont hors cluster. Sans interrupteur : ce qui l'ouvre est ce que le déploiement accorde | le kubeconfig hors cluster (contextes, certificats clients, plugins d'exec), et la découverte par DNS | CE/EE |
| [~] | ROUTE-03 | **Prédicats** | Path (avec/sans trailing slash), Host, Header, Cookie, Method, Query, RemoteAddr, XForwardedRemoteAddr, Weight, time-window | le prédicat de langue, l'exclusion par regex, l'option trailing slash | CE |
| [x] | SVC-03 | **Route simple** | Une route simple n'a rien d'autre à créer : on saisit une URL amont dans la route, et c'est tout | - | CE |
| [~] | ROUTE-04 | **Filtres requête** | Ajout/suppression/modification/renommage de headers (y compris « si absent »), set-host (écrit le champ ET l'en-tête - Go | `prefix-path` n'interprète pas le gabarit `{language}`, il préfixe la chaîne littérale | CE |
| [~] | SVC-04 | **État des amonts** | État des amonts visible dans la console : la table des routes marque celles qui ne répondent plus et dit pourquoi, à partir de ce que le disjoncteur observe déjà sur chaque réponse (ROUTE-09) | le second niveau : la santé applicative d'un service (son propre /health), et le registre du tunnel qui n'alimente rien | CE |
| [~] | ROUTE-05 | **Filtres réponse** | Ajout/suppression/modification/renommage de headers, set-status, redirection, cache-control (pose Cache-Control et fait | le cache de réponse local, qui est un état entre les requêtes et non un filtre | CE |
| [ ] | SVC-05 | **Applications** | Regroupement par application (branding, thème, locales propres à un ensemble de routes) - pertinent si une même gateway sert plusieurs | tout : l'application est la configuration globale, une route ne se rattache à aucun groupe | CE |
| [~] | SVC-06 | **Inventaire d'endpoints** | Inventaire d'endpoints par route API (méthode + path), porté par route.api.spec : publiée par le service, ou déposée ici et servie par la gateway sur le préfixe de la route | le mode record, qui apprendrait les endpoints du trafic observé | CE |
| [x] | ROUTE-07 | **Timeouts** | Délais de connexion et de première réponse à **trois niveaux** : la route, sinon l'installation (Routes > Global), sinon le produit (5 s / 15 s). Chaque borne hérite séparément. Le corps n'est jamais borné : un téléchargement ou un websocket déjà commencé vit aussi longtemps qu'il faut. Les routes qui nomment les mêmes bornes partagent un pool de connexions | - | CE |
| [ ] | ROUTE-08 | **Rate limiting** | Rate limiting & quotas par route et par consommateur, configurables depuis la console - voir §3.17 (QUOTA-01...04) | tout : la brique est annoncée « planned » dans la console | CE |
| [x] | ROUTE-09 | **Disjoncteur** | Disjoncteur par route, **éteint par défaut** : après N échecs consécutifs la route cesse d'appeler l'amont, sert la page d'indisponibilité immédiatement, et laisse passer **une seule** requête après le délai de refroidissement. Un 500 ne compte pas - un service qui répond 500 est debout et a un bug. L'état est par nœud | le retry, qui rejouerait une requête dont on ne sait pas si elle est idempotente | CE |
| [~] | ROUTE-10 | **Routes par défaut** | Routes par défaut fournies par la gateway : des routes de démonstration et un catch-all (TRAP), un / ordonné en dernier qui attrape ce que | les ressources `/meerkat/...` sont servies par le moteur et non par une route | CE |
| [~] | ROUTE-11 | **Sonde d'amont** | La console indique si l'amont d'une route répond - **observé** plutôt que sondé : le disjoncteur regarde chaque réponse réelle, donc aucune requête n'est fabriquée et le verdict porte sur le chemin qu'empruntent les vrais appels | la sonde active, qui seule peut parler d'une route que personne n'appelle | CE |
| [x] | ROUTE-12 | **Import / export** | Import/export des routes dans le document de configuration, en YAML et non en JSON - le modèle se décrit en JSON, mais le fichier qu'on | - | CE |
| [x] | ROUTE-13 | **WebSocket** | Support WebSocket de bout en bout (proxy des connexions WS vers l'amont) | - | CE |
| [~] | ROUTE-14 | **Compression et HTTP/2** | La gateway ne compresse rien elle-même, elle laisse passer ce que l'amont a compressé, et elle ne monte en HTTP/2 | la gateway ne compresse jamais elle-même, et h2 n'existe que par ALPN sous TLS | CE |
| [x] | ROUTE-15 | **Testeur de routage** | Composer depuis la console une requête fictive (méthode, path, host, header, cookie, query, adresse client, horloge | - | CE |
| [x] | ROUTE-16 | **Cible unique** | Cible unique par route : une route répond d'UNE seule façon, choisie dans une section Target - proxifier vers un amont, rediriger le | - | CE |
| [x] | ROUTE-17 | **Réponse par gabarit** | Répondre depuis un gabarit (respond) : une route peut répondre elle-même un contenu construit sur l'identité de l'appelant, au format que | - | CE |
| [x] | ROUTE-18 | **Rôles envoyés** | Ce qui part vers l'amont s'écrit : l'attribut roles porte UNE expression, lue comme un pipeline - {{.Roles / .Keep "tag:BILLING" | - | CE |
| [x] | ROUTE-19 | **REMOTE_USER** | Le compte voyage aussi sous les noms que les applications lisent déjà quand l'identité part en en-têtes : REMOTE_USER et X-Remote-User, en | - | CE |
| [ ] | ROUTE-20 | **gRPC** | gRPC de bout en bout | le h2c vers l'amont, le transport déclaré par amont, les trailers, gRPC-Web | CE |
| [ ] | ROUTE-21 | **Extensions** | Modèle d'extension : peut-on ajouter un prédicat ou un filtre sans recompiler la gateway ? Les briques sont aujourd'hui toutes internes | tout, et rien ne presse tant que le catalogue de briques couvre | CE |

### Filtres applicatifs UI

| Fait | ID | Mot-clé | Description | Ce qui manque | Éd. |
|:---:|---|---|---|---|:---:|
| [ ] | UIF-01 | **base href** | Réécriture du <base href> (HTML/JS/JSON) en cohérence avec le StripPrefix, pour servir une app sous un sous-chemin sans la rebuild | tout : aucune réécriture de `base href` | CE |
| [x] | UIF-02 | **Injection HTML** | Injection de contenus après <head> : scripts et CSS fournis par la gateway dans les pages proxifiées | - | CE |
| [x] | UIF-03 | **Bouton utilisateur** | Bouton utilisateur flottant injecté dans les apps proxifiées : profil, déconnexion, changement d'organisation, et aussi bascule | - | CE |
| [x] | UIF-04 | **Canal live** | Canal WebSocket injecté (socle livré) : les apps proxifiées reçoivent les notifications temps réel de la gateway sans intégration préalable | - | CE |
| [x] | UIF-05 | **Sélecteur de langue** | Sélecteur de langue injecté : un sous-menu du bouton utilisateur (UIF-03), alimenté par l'offre de langues de la route, pas un composant à | - | CE |
| [x] | UIF-06 | **CSS par route** | CSS additionnel par route (éditeur de code dans la console) et CSS conditionné par rôles : les rôles effectifs de la session sont | - | CE |
| [~] | UIF-07 | **Specs OpenAPI réécrites** | Réécriture dynamique des specs OpenAPI (host/basePath en 2.0, servers en 3.x, ajustés au chemin exposé par la gateway, la base propre du | l'appliquer aux réponses proxifiées ; seul le portail de docs dev réécrit son spec | CE |

### Identité transmise à l'amont

| Fait | ID | Mot-clé | Description | Ce qui manque | Éd. |
|:---:|---|---|---|---|:---:|
| [~] | SAUTH-01 | **Identité vers l'amont** | La gateway transmet l'utilisateur signé à l'amont, par en-têtes, par JWT non signé ou par JWT signé (ES256, EdDSA ou RS256 ; kid et JWKS | BASIC, FORM et JWT tiers, c'est-à-dire les modes qui portent un secret vers l'amont | CE |
| [ ] | SAUTH-02 | **Identifiants au coffre** | Les identifiants utilisés par send-auth sont référencés depuis le Vault (jamais en clair dans la config de route) | tout : il n'existe aucun champ d'identifiant send-auth à référencer | CE |
| [~] | SAUTH-03 | **Script post-auth** | Un bloc JS par route est injecté dans les pages HTML servies (UIF-02), avec éditeur de code dans la console : il peut poser ce que l'app | le crochet post-auth lui-même, et les presets pour outils courants | CE |
| [ ] | SAUTH-04 | **Découverte OIDC** | Découverte OIDC du jeton d'identité : Meerkat publie /.well-known/openid-configuration à côté du JWKS déjà servi, pour qu'un amont valide | le document de découverte ; la signature, le `kid` et le JWKS existent déjà | CE |
| [ ] | SAUTH-05 | **Fournisseur OIDC** | Fournisseur OIDC complet : authorize, token, écran de consentement, PKCE, refresh, un client déclaré par application | tout, et c'est à peser avant de s'y engager | CE |

### Coffre

| Fait | ID | Mot-clé | Description | Ce qui manque | Éd. |
|:---:|---|---|---|---|:---:|
| [x] | VAULT-01 | **Coffre** | Coffre à deux genres dans un seul espace de noms : un *secret* est chiffré au repos (AES-256-GCM) et ne repart jamais en clair, une | - | CE |
| [~] | VAULT-02 | **Rotation de la clé** | CRUD des entrées depuis la console, cadré par les portées que l'appelant administre, et la valeur d'un secret ne repart jamais : l'API dit | la ré-encryption globale, donc la rotation de la clé maîtresse | CE |
| [x] | VAULT-03 | **Coffre portable** | Le coffre comme fichier chiffré, export et import, pour amorcer un environnement ou déménager une gateway - l'exact inverse de l'export de | - | CE |
| [ ] | VAULT-04 | **Coffre externe** | Option d'adossement à un coffre externe (HashiCorp Vault, secrets Kubernetes/Docker) comme source alternative | tout : aucun backend externe (HashiCorp, secrets Kubernetes ou Docker) | CE |
| [x] | VAULT-05 | **Champs sensibles** | Un champ sensible passe par le coffre | - | CE |

### TLS

| Fait | ID | Mot-clé | Description | Ce qui manque | Éd. |
|:---:|---|---|---|---|:---:|
| [x] | SSL-01 | **Certificats** | Un certificat entre par l'une de quatre portes, et aucune n'est optionnelle : les lieux où tourne Meerkat ne se ressemblent pas | - | CE |
| [x] | SSL-02 | **Ports HTTPS** | Une porte HTTPS par plan, à côté du port en clair, ouverte et fermée à chaud (MEERKAT_TLS_ADDR, MEERKAT_ADMIN_TLS_ADDR) | - | CE |
| [x] | SSL-03 | **Secrets TLS** | Un secret de TLS passe par le coffre (VAULT-05) | - | CE |
| [~] | SSL-04 | **Alerte d'expiration** | Alerte (console + notification) avant expiration des certificats | la notification ; la console montre déjà le compte à rebours | CE |
| [x] | SSL-05 | **ACME** | Émission et renouvellement automatiques par une autorité ACME, et la porte est ouverte sans dépendre de Let's Encrypt : le répertoire est | - | CE |
| [~] | SSL-06 | **Redirection HTTPS** | Redirection du port en clair vers le port HTTPS, sur le plan de données uniquement | le réglage global HSTS : l'en-tête n'existe que par route | CE |
| [ ] | SSL-07 | **HTTP/3** | HTTP/3 (QUIC) | tout : dépendance QUIC hors bibliothèque standard, écoute UDP, `Alt-Svc` | CE |
| [x] | SSL-08 | **Certificat par nom** | Un certificat appartient à un NOM | - | CE |

### Notifications

| Fait | ID | Mot-clé | Description | Ce qui manque | Éd. |
|:---:|---|---|---|---|:---:|
| [~] | NOTIF-01 | **SMTP** | SMTP configurable à chaud (host, port, smtp/smtps, STARTTLS, auth, mot de passe via Vault), avec test d'envoi depuis la console, qui essaie | des gabarits, et les mails d'invitation, de code TOTP et de mot de passe temporaire | CE |
| [ ] | NOTIF-02 | **Web Push** | Web Push (VAPID) : abonnements navigateur, envoi de notifications push, activable | tout : aucun VAPID, aucun service worker | CE |
| [~] | NOTIF-03 | **Canal WebSocket** | WebSocket serveur->client : canal de notifications temps réel de la gateway vers les UIs (console et apps proxifiées via UIF-04), diffusion | la console n'y est pas abonnée, et le hub est en mémoire donc ne franchit pas les nœuds | CE |

### Thème, marque et langues

| Fait | ID | Mot-clé | Description | Ce qui manque | Éd. |
|:---:|---|---|---|---|:---:|
| [x] | I18N-01 | **Pages traduites** | Pages servies par la gateway traduites (la console ne l'est pas - I18N-03) ; 20 catalogues livrés (en, fr, de, vi, es, it, nl, pt, pl, ru | - | CE |
| [x] | THEME-01 | **Thème global** | Thème et branding choisis une fois, globalement : une seule procédure de login (et pages associées) sert toutes les applications derrière | - | CE |
| [x] | I18N-02 | **Erreurs traduites** | Messages d'erreur backend localisés | - | CE |
| [~] | THEME-02 | **Marque** | Branding : nom et description de l'application, logo téléversé (dessiné en taille normale, grande ou très grande - un logotype large ne remplit pas la même boîte qu'une marque carrée) et icône d'onglet dédiée - visibles sur les pages d'auth | la couleur de fond du logo, remplacée de fait par l'image de fond (THEME-06) | CE/EE |
| [x] | I18N-03 | **Console en anglais** | La console est en anglais, un point c'est tout (décidé le 2026-08-08, remplace « préparée pour la traduction ») | - | CE |
| [~] | THEME-03 | **Palette partagée** | Palette partagée entre les pages servies par la gateway et les mécanismes d'injection : un seul jeu de variables CSS --mk-*, écrit par le | la console ne consomme pas la palette : elle vit sur les tokens Material | CE |
| [~] | I18N-04 | **Langues par route** | Langues gérées par service UI : l'application déclare une réserve de locales (réglage global des langues, écran Application > Locales) et | une locale par défaut déclarable par route ; la réserve est unique pour toute la gateway | CE |
| [~] | THEME-04 | **Éditeur M3** | Système de thème type Material 3 (M3), éditable via l'UI : les pages servies par la gateway (login, sélection, profil...) sont stylées | la génération M3 depuis des couleurs sources, la palette secondaire, les élévations | CE |
| [x] | THEME-05 | **Clair / sombre** | Gestion du color-scheme utilisateur (clair/sombre/système) : l'intégrateur tranche d'abord, en décochant un schéma sur l'aperçu des pages ; le choix de la personne vit dans un cookie d'un an ET sur son compte, reposé à la connexion sur un navigateur qui ne l'a jamais vue - comme la langue | - | CE |
| [x] | THEME-06 | **Fond de marque** | Fond des pages intégrées, et séparation thème / marque : la marque porte une image de fond (cadrage cover/contain/tile et un voile réglable | - | CE |

### Console d'administration

| Fait | ID | Mot-clé | Description | Ce qui manque | Éd. |
|:---:|---|---|---|---|:---:|
| [~] | CONSOLE-01 | **Console d'administration** | Console web d'administration servie par la gateway elle-même, organisée en deux plans plus des écrans transverses : Infra (routes, sécurité | une vue des connexions au niveau Application | CE |
| [x] | CONSOLE-02 | **Menus par droits** | Navigation conditionnée par les droits (un utilisateur ne voit que les sections auxquelles il a accès, repli automatique vers la première | - | CE |
| [x] | CONSOLE-03 | **Éditeur de route** | Éditeur de route complet, en onze sections pilotées par l'URL : sécurité/accès, prédicats, garde-fous, cible, modificateurs entrants | - | CE |
| [~] | CONSOLE-04 | **Sécurité des endpoints** | Écran Sécurité des endpoints : on y choisit une route qui expose une spec OpenAPI, la spec est récupérée et analysée côté serveur, et ses | l'agrégation de plusieurs routes, le mode record, les quotas par endpoint | CE |
| [x] | CONSOLE-05 | **Export / import** | La configuration s'exporte et s'importe en un document : routes, catalogue de rôles, organisations et leurs groupes, fournisseurs | - | CE |
| [~] | CONSOLE-06 | **Réglages** | Configuration centralisée, répartie par plan : politique de mots de passe, second facteur, passkeys, jetons d'API, limitation de débit, TTL | les options d'hôte et le push | CE |
| [~] | CONSOLE-07 | **Utilisateurs** | Gestion des utilisateurs par le root : recherche paginée, création, réinitialisation de mot de passe (temporaire + délai de validation) | la pagination serveur, l'import en masse, un délai de validité sur le mot de passe temporaire | CE |
| [~] | CONSOLE-08 | **Connexions et sessions** | Visionneuse de connexions/sessions (globale pour root, personnelle pour l'utilisateur) avec filtres et pagination | la vue globale pour le root, avec filtres et pagination | CE |
| [~] | CONSOLE-09 | **Profil** | Profil utilisateur complet géré par la gateway : identité (nom complet, e-mail vérifié), avatar (téléversé, sinon des initiales générées | une API profil appelable comme telle par un service | CE |
| [~] | CONSOLE-10 | **Suppression de compte** | Par l'utilisateur (self-service, avec confirmation forte) et par l'admin ; purge ou anonymisation des données | le self-service, l'anonymisation, et une révocation en cascade explicite | CE |
| [x] | CONSOLE-11 | **Console à part** | Console dissociée de l'application - port d'administration dédié : le binaire écoute sur deux plans séparés - le port applicatif (data | - | CE |

### Cycle de vie

| Fait | ID | Mot-clé | Description | Ce qui manque | Éd. |
|:---:|---|---|---|---|:---:|
| [~] | LIFE-01 | **Premier démarrage** | Setup premier lancement : au premier démarrage, la gateway crée elle-même le compte root (admin, mot de passe pris dans | la page de setup : le mot de passe root passe aujourd'hui par le log | CE |
| [~] | LIFE-02 | **Seed déclaratif** | Seed déclaratif au démarrage depuis des fichiers de config montés (routes, rôles, organisations, groupes, autorités LDAP/OIDC, relais SMTP | enregistrer le fichier comme configuration disponible quand il diffère | CE |
| [x] | LIFE-03 | **Mode développeur** | Voir la section dédiée §3.14 (DEV-xx) | - | CE |
| [x] | LIFE-04 | **Doc d'API** | Documentation d'API de la gateway exposée (OpenAPI/Swagger) pour intégration par les services | - | CE |
| [x] | LIFE-05 | **Indisponibilité** | Deux entrées et **une seule page** : une brique de filtre posée sur une route dont le service est tombé, et un **commutateur global** (Routes > Global) qui répond 503 pour toutes les routes d'un coup sans qu'aucune ne soit modifiée. Le titre énonce le **fait**, la raison se **choisit** dans une liste fermée (rien dit, maintenance, mise à jour, incident) et la durée se **choisit** aussi (ISO-8601 : PT1H... P3D), facultative : tout est traduit, rien n'est saisi en toutes lettres, parce que cette page se lit en vingt langues. La page n'annonce jamais l'heure à la minute - dans l'heure, dans quelques heures, d'ici un jour, d'ici quelques jours - elle garde la grandeur choisie tant qu'elle est vraie, et rétrécit d'elle-même à chaque affichage. C'est une **page servie** comme les autres (thème, mise en page, marque, sélecteurs de langue et de thème). Les pages propres à la gateway continuent de répondre ; qui administre voit la même page que les visiteurs, avec une porte pour passer **délibérément** - liée à sa session, donc reprise à chaque connexion - et un liseré traduit le lui rappelle sur chaque page traversée | - | CE |

### Mode développeur

| Fait | ID | Mot-clé | Description | Ce qui manque | Éd. |
|:---:|---|---|---|---|:---:|
| [x] | DEV-01 | **Capacité dev** | Le mode dev d'un utilisateur est activé par un admin (flag dev) ; à partir de là, le développeur accède à des fonctionnalités | - | CE |
| [~] | DEV-02 | **Clé de développeur** | Un utilisateur dev dépose une clé publique SSH sur son compte, en self-service (/profile/dev/key, réservé aux comptes | plusieurs clés par compte, et l'audit du dépôt et du retrait | CE |
| [~] | DEV-03 | **Poste vers cluster** | Routage poste -> cluster : la CLI plug lance un processus sur le poste du développeur - ex | porter l'identité et les rôles du développeur, et les conditions posées par l'admin | EE |
| [~] | DEV-04 | **Cluster vers poste** | Substitution de service (cluster -> poste) : si --service <nom> est spécifié au lancement, plug prend ce nom dans l'orchestrateur et le | la validation du nom faute de catalogue, et la portée : la substitution reste globale | EE |
| [ ] | DEV-05 | **Portée des substitutions** | Portée par défaut des overrides : rien n'est construit - une substitution est aujourd'hui globale (DEV-04) et vaut pour tout le trafic | tout : une substitution vaut aujourd'hui pour tout le trafic | CE |
| [~] | DEV-06 | **Annonce aux utilisateurs** | La substitution étant globale (DEV-04), il n'y a rien à choisir - il y a à savoir | le menu de sélection de variante, qui n'a de sens qu'une fois DEV-05 construit | CE |
| [~] | DEV-07 | **Cycle de vie** | Cycle de vie des substitutions : une substitution vit tant que le plug qui l'a déclarée est actif - arrêt du processus ou déconnexion => | un TTL de sécurité côté passerelle : le retrait dépend entièrement de l'agent | EE |
| [~] | DEV-08 | **Substitutions simultanées** | Plusieurs développeurs servent en parallèle sans se gêner tant qu'ils prennent des noms différents - le | deux devs sur le même nom se marchent dessus, le dernier arrivé gagne | EE |
| [~] | DEV-09 | **Outillage de test** | Outillage dev hérité de la V1 : Swagger UI embarqué (assets dans le binaire, rien d'un CDN) sur les specs OpenAPI que déclarent les routes | rien ne publie sur le topic dev ; le retour passe par le swagger | CE |
| [x] | DEV-10 | **Mode test UI** | Sur une application proxifiée, un développeur active depuis le user-button une barre d'outillage (escamotable, liseré ambre | - | CE |
| [~] | DEV-11 | **Cadre de sécurité** | L'admin définit quelles routes/services sont substituables et par quels devs ; substitutions et connexions plug sont | l'écran console, l'audit des actions, et le cadre par route | EE |
| [x] | DEV-12 | **Console en dev** | En mode dev local, la console intégrée s'efface au profit du serveur de dev front (héritage V1 : route ARCHWAY désactivée en DEV_MODE) | - | CE |

### Configurations versionnées

| Fait | ID | Mot-clé | Description | Ce qui manque | Éd. |
|:---:|---|---|---|---|:---:|
| [~] | CFG-01 | **Collection** | La configuration d'infrastructure - routes, rôles, autorités, relais mail, thèmes, réglages de gateway - constitue une Configuration nommée | le compte « 2 sur 3 » n'est jamais affiché : seul le refus dit combien tiennent | CE |
| [x] | CFG-02 | **Configuration active** | Plusieurs configurations coexistent, une seule active | - | CE |
| [~] | CFG-03 | **Fichier au démarrage** | Si la gateway n'est pas initialisée, le fichier sert de seed et devient la configuration active initiale (« only on | un fichier différent est journalisé et ignoré, jamais enregistré | CE |
| [~] | CFG-04 | **Comparaison** | Diff entre configurations visualisable dans la console (et présenté à l'import d'un fichier) : objets ajoutés/supprimés/modifiés | comparer deux configurations enregistrées sans passer par l'état courant | CE |
| [x] | CFG-05 | **Export** | Export d'une configuration = fichier réimportable (boucle complète avec CONSOLE-05 et CFG-03), pour l'état courant comme pour une | - | CE |
| [x] | CFG-06 | **Points de reprise** | La gateway tient sa propre bande, un point à chaque changement qui déplace l'empreinte de la configuration - jamais | - | CE |

### Pages et front-ends

| Fait | ID | Mot-clé | Description | Ce qui manque | Éd. |
|:---:|---|---|---|---|:---:|
| [~] | PAGE-01 | **Pages vanilla** | Les pages du flux utilisateur servies par la gateway - login, vérification TOTP, enrôlement TOTP, changement de mot de passe, sélection | la page de sélection du dev à tester | CE |
| [~] | PAGE-02 | **Layouts** | Ces pages sont customizables par l'intégrateur sans rebuild : thème, logo, titre de l'application, et disposition | les gabarits remplaçables par l'intégrateur, c'est-à-dire son propre HTML | CE/EE |
| [~] | PAGE-03 | **Traductions surchargeables** | Ces pages sont traduites dans vingt langues embarquées, un fichier JSON par langue (l'anglais fait référence, les autres sont comparées à | des catalogues surchargeables : ajouter une langue reste un rebuild | CE |
| [~] | PAGE-04 | **Web Components** | Le bouton utilisateur (UIF-03) est un Web Component standard (meerkat-user-button, shadow DOM), et il porte le sélecteur de langue (UIF-05) | l'agent de page et le canal WS, encore un script global | CE |
| [x] | PAGE-05 | **Console embarquée** | La console d'administration reste une SPA Angular avec le thème Softwarity (continuité, maîtrise de l'équipe) ; elle est embarquée dans le | - | CE |
| [~] | PAGE-06 | **Layout de la console** | Un rail de navigation à gauche (softwarity/rail-nav) portant les entrées principales et, à son pied, les options | un bandeau partagé en tête de zone principale | CE |
| [x] | PAGE-07 | **Composants softwarity** | La console s'appuie sur l'écosystème de composants @softwarity/* : rail-nav (navigation), row-actions (actions des lignes de tous les | - | CE |

### Quotas

| Fait | ID | Mot-clé | Description | Ce qui manque | Éd. |
|:---:|---|---|---|---|:---:|
| [ ] | QUOTA-01 | **Quotas** | Quotas API définissables par route et par consommateur (utilisateur, token d'application, organisation, IP) : limites en nombre de requêtes | tout : aucun compteur de quota dans le produit | CE |
| [ ] | QUOTA-02 | **Réponse 429** | Dépassement -> 429 avec les headers standards (RateLimit-*, Retry-After) ; politique configurable par quota : bloquer, ralentir (throttle) | tout : aucun 429 hors des endpoints de connexion | CE |
| [ ] | QUOTA-03 | **Consommation** | Consommation visible dans la console : par consommateur, par route, période ; chaque utilisateur voit sa propre consommation (périmètre me) | tout : aucun écran, aucun seuil d'alerte | CE |
| [ ] | QUOTA-04 | **Compteurs en cluster** | Compteurs corrects en cluster (état partagé via la base externe) comme en mono-nœud (mémoire + persistance périodique) ; précision relâchée | tout : les quotas n'existent pas encore (QUOTA-01). Le chemin est tracé - `login_attempts` est exactement cette forme de compteur, partagé et à fenêtre glissante | CE |
| [ ] | QUOTA-05 | **Quotas par endpoint** | Quotas par endpoint (méthode + path) : posés sur l'inventaire d'endpoints de la route, celui que la gateway lit dans son OpenAPI (SVC-06) | tout : l'inventaire d'endpoints ne porte que la sécurité | CE |

### Audit

| Fait | ID | Mot-clé | Description | Ce qui manque | Éd. |
|:---:|---|---|---|---|:---:|
| [~] | AUD-01 | **Journal d'audit** | Journal d'audit intégré et consultable dans la console, livré pour les actions d'administration : qui a modifié quoi, avec le diff champ | la pagination serveur, les activités dev/plug, et les connexions restées à part | CE |
| [~] | AUD-02 | **Rétention et export** | Journal append-only livré - il ne connaît que l'écriture et la purge, aucune modification n'est possible - et consultable sans aucun outil | la rétention configurable, constante à 365 jours, et l'export analytique | CE |

### Anomalies

| Fait | ID | Mot-clé | Description | Ce qui manque | Éd. |
|:---:|---|---|---|---|:---:|
| [x] | ISSUE-01 | **Signalement** | Signalement injecté : une entrée « Signaler un problème » dans le bouton utilisateur des applications proxifiées ouvre un panneau flottant | - | CE |
| [x] | ISSUE-02 | **Stockage** | Stockage embarqué : le signalement est enregistré dans le store du gateway, estampillé côté serveur (rapporteur, tenant courant de la | - | CE |
| [x] | ISSUE-03 | **Suivi en console** | Gestion dans la console : section transverse « Anomalies » - liste filtrable par statut, détail (capture, console, contexte), cycle de vie | - | CE |
| [x] | ISSUE-04 | **Interrupteur** | Interrupteur infra, livré ÉTEINT (posé sur l'écran Anomalies, là où l'on constate qu'il ne remonte rien) : éteint, l'entrée disparaît du | - | CE |
| [ ] | ISSUE-05 | **Connecteurs** | Connecteurs vers un dépôt central (GitHub Issues, GitLab, Jira...) : pousser un signalement vers l'extérieur, manuellement puis par règle | tout : aucun connecteur GitHub, GitLab ni Jira | CE |
| [ ] | ISSUE-06 | **Annotations** | Annoter la capture, et numéroter les annotations (demandé le 2026-08-09) : après la capture, un mode pleine page (l'aperçu du panneau | l'annotation numérotée : seul le recadrage existe | CE |

### Pilotage par un agent (MCP)

| Fait | ID | Mot-clé | Description | Ce qui manque | Éd. |
|:---:|---|---|---|---|:---:|
| [x] | MCP-01 | **Serveur MCP** | Un endpoint `/mcp` sur le plan de contrôle, Model Context Protocol en Streamable HTTP : pas de port à ouvrir, l'admin l'atteint là où est déjà sa console. Livré ÉTEINT, l'interrupteur est dans sa **section MCP** (Infra, sous Configuration) avec la commande prête à coller pour Claude Code, Gemini CLI, Kimi CLI, Codex CLI et un JSON générique, la liste des **agents branchés** et le débranchement d'un clic | - | CE |
| [x] | MCP-02 | **Jeton d'agent** | Un jeton par agent, du plan admin (AUTH-09), révocable, daté de son dernier usage, et portant un **périmètre à trois axes** : jusqu'où (`readonly`/`full`), sur quoi (le plan de routage, l'identité applicative, ou tout), et depuis où (des plages CIDR jugées sur le **pair TCP**, jamais sur un en-tête). Le périmètre ne fait que **retirer** : le domaine masque les capacités du porteur, donc un jeton gateway frappé par root pilote les routes et rien d'autre, écrans root compris. Vérifié dans l'entonnoir unique du plan de contrôle, donc il vaut pour l'API REST autant que pour l'agent, et pour tout ce qu'on ajoutera | - | CE |
| [x] | MCP-03 | **Acteur audité** | Chaque mutation faite par un jeton s'écrit avec le nom du jeton à côté de celui du compte, dans le journal (AUD-01) et dans la console : « admin, via claude-desktop » et non « admin ». Le tampon est posé dans l'écriture d'audit elle-même, pas dans ses appelants, pour qu'aucun événement futur ne l'oublie | - | CE |
| [x] | MCP-04 | **Outils métier** | Treize outils qui parlent le produit et pas le REST : décrire la gateway (édition comprise, pour que l'agent sache dire « ça, c'est l'Enterprise »), lister et lire les routes, lire le vocabulaire des prédicats et filtres, tester où tombe une requête, lister comptes et organisations, lire le journal, exporter la configuration, plus les quatre de MCP-05. Chaque outil déclare qui peut l'appeler, avec les mêmes capacités que l'endpoint REST correspondant : l'agent n'est pas une seconde porte. Un test refuse toute section du plan de contrôle que personne n'a tranchée | - | CE |
| [x] | MCP-05 | **Écriture directe, et le filet** | L'agent écrit **pour de vrai** : `save_route` et `delete_route` passent par la même fonction que l'endpoint de la console (une seule implémentation, donc les mêmes refus) et la gateway recharge aussitôt. Le filet n'est pas une cérémonie mais ce qui existait déjà : un **point de restauration** est écrit tout seul après chaque changement (CFG-06), étiqueté des mots du journal, et le journal nomme le jeton. En plus, `save_configuration` range l'état courant sous un nom - c'est ce qu'un admin prudent demande **en mots** avant un gros changement, et ça ne coûte rien à ceux qui ne le demandent pas | - | CE |
| [x] | MCP-07 | **Branchement OAuth** | Se brancher **sans copier de secret**, comme chez Jira : l'agent reçoit un 401 qui dit où s'autoriser (RFC 9728), s'enregistre tout seul (RFC 7591), ouvre un navigateur sur la page de consentement, et repart avec un jeton que personne n'a tapé. **Le périmètre se choisit là**, au moment où quelqu'un branche vraiment l'agent - c'est ce qui a remplacé la boîte de dialogue de frappe. PKCE S256 obligatoire, clients publics sans secret, redirection sur la boucle locale dont le port est libre, rafraîchissement rotatif. Le jeton produit **est** un jeton de plan admin ordinaire, donc le périmètre, le journal et la révocation marchent dessus sans seconde mécanique. Un jeton frappé à la main reste accepté pour les clients qui ne savent pas danser | - | CE |
| [x] | MCP-06 | **Export pour l'agent** | La configuration entière (CFG-05) en un appel, images retirées comme dans un export à plat : le moyen le moins cher de donner tout le contexte d'un coup. Réservé à root, comme le téléchargement dont il est le jumeau | - | CE |

### Sécurité

| Fait | ID | Mot-clé | Description | Ce qui manque | Éd. |
|:---:|---|---|---|---|:---:|
| [~] | SEC-01 | **CSRF** | Protection active sur toutes les opérations d'état en session cookie (désactivée dans la V1 - à re-spécifier et activer) | aucun jeton CSRF ni vérification d'`Origin` sur les écritures : `SameSite=Lax` est seul | CE |
| [~] | SEC-02 | **CORS** | Politique explicite et configurable (désactivé dans la V1) | la politique CORS n'est pas configurable, une seule origine est câblée | CE |
| [x] | SEC-03 | **En-têtes de sécurité** | En-têtes de sécurité modernes configurables : HSTS, CSP, X-Frame-Options, Referrer-Policy | - | CE |
| [x] | SEC-04 | **Aucun secret en clair** | Aucun secret en clair dans le dépôt ni dans les exports (la V1 committait des mots de passe dans config/ et docker-compose.yml) | - | CE |
| [~] | SEC-05 | **Hachage** | Mots de passe hachés avec un algorithme moderne et ré-encodage transparent à la connexion en cas de changement d'algorithme | le ré-encodage transparent : ni détection d'un coût périmé, ni re-hash à la connexion | CE |
| [~] | SEC-06 | **Chiffrement au repos** | Chiffrement au repos livré pour les secrets du Vault (AES-256-GCM), avec une clé maître prise dans l'environnement ou générée au premier | les secrets TOTP sont **en clair**, et la rotation de la clé maîtresse n'existe pas | CE |
| [~] | SEC-07 | **Révocation** | Sessions, refresh tokens, tokens d'application et navigateurs de confiance doivent tous être révocables individuellement et en | les sessions ne se listent ni ne se révoquent une par une, et désactiver un compte ne les tue pas | CE |
| [~] | SEC-08 | **Audit** | Journal d'audit des actions d'administration - voir §3.17 (AUD-01/02), intégré et consultable dans la console ; la V1 n'avait que les | les mêmes manques qu'AUD-01 | CE |
| [x] | SEC-09 | **Anti-énumération** | Messages d'erreur de login non discriminants, délais/backoff sur échecs | - | CE |

### Performance et montée en charge

| Fait | ID | Mot-clé | Description | Ce qui manque | Éd. |
|:---:|---|---|---|---|:---:|
| [x] | PERF-01 | **Non bloquant** | Modèle d'exécution non bloquant / haute concurrence : la gateway est sur le chemin de toutes les requêtes, la latence ajoutée doit rester | - | CE |
| [x] | PERF-05 | **Bancs de mesure** | `internal/gateway/bench_test.go` : le proxy nu de la bibliothèque standard comme plancher, puis la route minimale, la table de 50, les filtres, le terminal, et la **sélection isolée** du socket sur 1 / 50 / 200 routes. Le chiffre absolu dépend de la machine et ne s'écrit nulle part ; ce qui se lit est le **surcoût relatif** au plancher mesuré dans la même exécution, et le **nombre d'allocations**, qui est un compte et non une durée. Un test bloque la régression en énonçant une **forme** et non un plafond : matcher contre 200 routes doit allouer exactement ce que matcher contre 1 alloue | - | CE |
| [x] | PERF-02 | **Streaming** | Streaming des corps de requête/réponse : pas de mise en mémoire intégrale, sauf les filtres de réécriture de corps, plafonnés par un réglage (Routes > Global, 1 à 256 Mio, 20 par défaut). Au-delà du plafond la réponse passe **intacte et entière** | - | CE |
| [x] | PERF-03 | **Cluster actif/actif** | Cluster actif/actif (optionnel, motivé par la haute disponibilité - la gateway est la porte d'entrée unique) : N nœuds sans état local. Bus de changement (STORE-03), **verrou consultatif** `pg_advisory_lock` autour de l'émission ACME et de l'entretien périodique, caches de sessions et canal live relayés (PERF-04) | - | EE |
| [x] | PERF-04 | **État partagé** | Tout état de flux est partagé ou invalidé entre nœuds : clés TOTP et passkey, jetons OAuth, **clé de simulation** (un jeton frappé par un nœud vaut sur tous) et compteur anti-force brute en base ; le **cache de sessions** (5 s) et le **hub d'événements** restent en mémoire mais sont invalidés et relayés par le bus | - | CE |

### Observabilité

| Fait | ID | Mot-clé | Description | Ce qui manque | Éd. |
|:---:|---|---|---|---|:---:|
| [ ] | OBS-01 | **Tableau de bord** | Observabilité intégrée à la console : tableaux de bord natifs - trafic par route/service, latences (percentiles), codes d'erreur | tout : aucun agrégat de trafic, de latence ni d'erreurs | CE |
| [~] | OBS-02 | **Health checks** | Health checks (liveness/readiness) exploitables par l'orchestrateur | liveness et readiness pointent le même `/healthz`, qui ne vérifie pas la base | CE |
| [~] | OBS-03 | **Logs** | Logs structurés, niveaux configurables à chaud ; journal des requêtes activable | aucun handler configuré : niveau non réglable, pas de journal de requêtes | CE |
| [ ] | OBS-04 | **Tracing** | Tracing distribué (traceparent/W3C propagé aux amonts) | tout : aucun `traceparent` propagé | CE |
| [ ] | OBS-05 | **Prometheus** | Endpoint Prometheus optionnel pour les entreprises déjà équipées d'une stack de monitoring - un complément, jamais un prérequis de | tout : aucun `/metrics` | CE |

### Déploiement

| Fait | ID | Mot-clé | Description | Ce qui manque | Éd. |
|:---:|---|---|---|---|:---:|
| [x] | DEPLOY-01 | **Image unique** | Distribution en image de conteneur unique (publiée sur registres publics), déployable en Docker/Swarm/Kubernetes ; variante « tout-en-un » | - | CE |
| [~] | DEPLOY-02 | **Variables d'environnement** | Toute la configuration surchargée par variables d'environnement ; fichiers de config montés en volume pour le seed (LIFE-02) | l'environnement ne couvre que le démarrage ; tout le reste vit en base | CE |
| [x] | DEPLOY-03 | **Zéro dépendance** | Zéro dépendance obligatoire : stockage embarqué par défaut (STORE-01) ; base externe uniquement pour le cluster (STORE-03) ; broker et SMTP | - | CE |
| [x] | DEPLOY-04 | **CI/CD** | Build, tests, publication d'images versionnées + latest, releases taguées | - | CE |
| [~] | DEPLOY-05 | **Éditions** | Éditions OSS/commerciale en base de code unique (open-core) : un seul dépôt, mais deux images issues du même commit et séparées par le tag | la chaîne d'émission commerciale ; `internal/license` n'est plus lu par le produit | CE |
| [~] | DEPLOY-06 | **Migrations** | Migrations de schéma/données automatiques entre versions (upgrade sans intervention) | aucune migration de DONNÉES versionnée : seul le schéma converge | CE |

### Stockage

| Fait | ID | Mot-clé | Description | Ce qui manque | Éd. |
|:---:|---|---|---|---|:---:|
| [x] | STORE-01 | **Autonome** | Démarrage autonome zéro dépendance : stockage embarqué transactionnel dans un fichier/répertoire local (classe SQLite - transactions | - | CE |
| [~] | STORE-02 | **SQLite ou PostgreSQL** | Abstraction de stockage à backends interchangeables livrée : embarqué (défaut) ou PostgreSQL, même schéma et mêmes requêtes, seules la | aucun chemin de reprise complet SQLite vers PostgreSQL, l'export ne portant que la configuration | CE |
| [~] | STORE-05 | **Montée de version** | Deux moitiés. L'**additive** tourne à chaque démarrage et n'a besoin d'aucun registre : le DDL de la version est appliqué en `CREATE TABLE IF NOT EXISTS`, puis `addMissingColumns` ajoute les colonnes que la base n'a pas - une table, une colonne, un index arrivent sans que personne écrive une étape. L'autre moitié est ce que cette moitié ne sait pas dire : une colonne renommée, un type changé, une valeur remodelée, une ligne rétro-remplie. Ces étapes-là sont **ordonnées, jouées une fois, enregistrées au fur et à mesure** (`internal/store/migrations.go`), donc une montée interrompue reprend où elle s'est arrêtée. Une base écrite par une version **plus récente** est refusée en nommant les deux numéros : revenir en arrière est normal quand une release tourne mal, et c'est exactement là que le refus vaut mieux que la perte. Le registre s'ouvre vide, et c'est légitime tant qu'aucune version n'a été publiée | l'épreuve qui compte : monter une base **amorcée par une version antérieure** en CI, ce qui suppose d'abord une première release à monter depuis | CE |
| [x] | STORE-03 | **Cluster** | Mode cluster = base externe partagée ; la synchronisation inter-nœuds passe par un **bus de changement** (`internal/cluster`) : une version par sujet en base plus un `NOTIFY` pour ce qui se recharge (routes, certificats), un signal porteur pour ce qui s'invalide (sessions) ou se relaie (canal live), et un **verrou consultatif** (`Store.WithLock` / `TryLock`) pour ce qui ne doit se faire qu'une fois | - | EE |
| [x] | STORE-04 | **Tout dans la base** | Toutes les données vivent dans le stockage choisi, y compris les blobs (keystores, avatars, logos - remplace GridFS) et les données à | - | CE |
| [~] | STORE-05 | **Sauvegarde** | Sauvegarde/restauration triviales : en mode embarqué, snapshot à chaud + copie de fichier ; dans tous les modes, export/import complet (cf | aucune restauration : la console imprime une procédure à exécuter service arrêté | CE |
| [ ] | STORE-06 | **Export du journal** | Journal d'audit append-only exportable vers des formats analytiques (Parquet, CSV) pour exploitation externe - Parquet est pertinent ici | tout : aucun export du journal | EE |

### Qualité

| Fait | ID | Mot-clé | Description | Ce qui manque | Éd. |
|:---:|---|---|---|---|:---:|
| [x] | QUAL-01 | **Tests** | Tests automatisés significatifs sur les chemins critiques (auth multi-étapes, RBAC, routage, filtres de corps), la **même suite tournant deux fois** : sur base embarquée, et sur un vrai PostgreSQL en CI (`internal/store/dbtest` donne à chaque test son schéma) | - | CE |
| [~] | QUAL-02 | **Documentation** | Guide d'exploitation (variables, certificats, TOTP/horloge), guide d'architecture, doc des filtres/prédicats | ni guide d'architecture, ni guide d'exploitation | CE |

## Ce qui demande des précisions

Tout ce qui suit est `[~]` ou `[ ]` dans le tableau. On y trouve le manque, **le point dur**,
et la solution retenue quand elle l'est. Ce qui n'a pas de solution écrite ici est ce qui
reste à trancher.

### Les secrets TOTP sont en clair (SEC-06, VAULT-02)

Le coffre chiffre ses entrées en AES-256-GCM, mais `users.totp_secret` est du base32 brut. Une
copie de la base donne donc les seconds facteurs de tout le monde. **Point dur** : les secrets
existants sont déjà posés, on ne peut pas les redemander aux utilisateurs. **Solution** :
chiffrer sous la clé du coffre, déchiffrer à la lecture, et **réécrire chiffré à la première
vérification réussie** - la migration se fait toute seule, au rythme des connexions. Le même
chantier ouvre la **rotation de la clé maîtresse** (VAULT-02), qui n'existe pas non plus :
déchiffrer avec l'ancienne, resceller avec la nouvelle, en une transaction.

### Le multi-instance (PERF-03, PERF-04, STORE-03, AUTH-11) - fait

PostgreSQL est parlé, le schéma est portable, et **la suite entière y tourne** en CI depuis
QUAL-01 - ce qui a immédiatement trouvé deux requêtes fausses, dont une colonne 32 bits portant
une seconde Unix.

**Le bus de changement** (`internal/cluster`) porte trois choses, et la différence entre elles
est la conception :

- **Ce qui se recharge** - le plan de routage, les certificats. Une version par sujet dans
  `change_marks` est la vérité, un `NOTIFY` n'est que l'indice qu'elle a bougé, et chaque nœud
  recharge ce qu'il a en mémoire. Une route enregistrée sur un nœud est servie par l'autre en
  une seconde, et une connexion d'écoute coupée rattrape son retard en se rebranchant.
- **Ce qui s'invalide** - le cache de sessions. Un signal porte le condensat du jeton (jamais le
  jeton) ou l'identifiant de l'utilisateur, et rien ne l'adosse à une table : en perdre un coûte
  les cinq secondes qu'une seule gateway accepte déjà. Une déconnexion sur un nœud vaut
  immédiatement sur l'autre, là où le choix d'organisation fait sur A pouvait être ignoré par B.
- **Ce qui se relaie** - le canal live. Une page est tenue par la gateway que le répartiteur lui
  a donnée, donc un message publié sur un nœud n'atteignait qu'une fraction de l'audience. Chaque
  signal porte l'identifiant du nœud émetteur : sans lui, le nœud qui publie se relaie à
  lui-même et ses pages reçoivent l'événement deux fois.

**Le verrou consultatif** (`Store.WithLock` / `TryLock`, `pg_advisory_lock` sur une connexion
réservée, un mutex local en dessous pour que la promesse tienne sur les deux bases) garde les
deux choses qui ne doivent pas se faire deux fois : l'**émission d'un certificat** - cinq nœuds
redémarrant ensemble dépensaient le quota hebdomadaire de doublons en une seconde, et celui qui
attend trouve maintenant le certificat que le gagnant vient d'écrire dans le cache partagé - et
l'**entretien périodique**, où un nœud qui trouve le verrou pris saute son tour. Le verrou n'est
pris qu'à la première poignée de main pour un nom et à l'approche du renouvellement : le mettre
devant chaque poignée de main coûterait un aller-retour vers la base pour éviter un événement
qui arrive deux fois par an.

Enfin, deux choses qui étaient par processus sont en base : la **clé de simulation**, donc un
jeton de test frappé par un nœud vaut sur tous et un redémarrage n'invalide plus celui qu'un
opérateur venait de copier ; et le **compteur anti-force brute** (AUTH-11), donc cinq essais
c'est cinq pour l'installation et non cinq par nœud.

**Ce qui reste** est un autre sujet : les quotas (ci-dessous) et la découverte automatique des
amonts (SVC-02).

### Les quotas n'existent pas (ROUTE-08, QUOTA-01 à 05)

Le seul limiteur du produit garde les endpoints de connexion. **Point dur** : un compteur exact
et partagé coûte un aller-retour par requête, ce qui est inacceptable sur le chemin de données.
**Solution** : deux mécanismes plutôt qu'un. Une **fenêtre glissante locale** au nœud pour la
protection (approximative, gratuite, suffisante contre l'abus) et un **compteur en base** pour
les quotas qui se facturent ou se montrent (QUOTA-03), écrits par lots. Le refus est un 429 avec
`RateLimit-*` et `Retry-After` (QUOTA-02), pas une porte muette.

### Il n'y a aucune restauration (STORE-05)

La sauvegarde à chaud existe et rend un fichier cohérent ; la remise en service est une procédure
que la console **imprime**, à exécuter service arrêté. **Point dur** : restaurer sous un processus
qui tourne, c'est remplacer le fichier que le moteur tient ouvert. **Solution** : assumer la
restauration hors ligne, mais ajouter un **contrôle d'intégrité** à l'import et un `meerkat
restore` qui refuse de démarrer sur une sauvegarde d'une version plus récente. Et dire clairement
que l'export de configuration n'est **pas** une sauvegarde (STORE-02) : il ne porte ni comptes,
ni coffre, ni certificats.

### Les sessions ne se listent pas (SEC-07, AUTH-14, CONSOLE-08)

Une session se révoque en base, mais personne ne peut voir les siennes ni les fermer toutes, et
**désactiver un compte ne tue pas ses sessions en cours** - il faut attendre l'expiration du cache.
**Solution** : la table existe, il manque un écran et un bouton ; et un appel à la révocation en
masse dans la désactivation, exactement comme la réinitialisation de mot de passe le fait déjà.

### CSRF (SEC-01)

`SameSite=Lax` est aujourd'hui la seule barrière. C'est beaucoup, ce n'est pas tout : un
sous-domaine voisin est same-site. **Solution** : un jeton par session dans les formulaires que
la gateway sert, et une vérification d'`Origin` sur les écritures de l'API d'administration -
l'API est en JSON, donc le coût est une ligne dans le décodeur.

### gRPC et h2c (ROUTE-20)

gRPC roule sur HTTP/2, pas sur HTTP/3 : ce qui manque est le **h2c**, HTTP/2 en clair, que la
bibliothèque standard sait parler des deux côtés depuis Go 1.24. **Point dur** : en clair il n'y
a aucune négociation, donc le protocole ne se devine pas - il se **déclare par amont**. Restent
les trailers, qui portent le statut gRPC, et les flux bidirectionnels, qui ne doivent être ni
bufferisés ni plafonnés.

### La découverte des amonts (SVC-02)

**Point dur** : le client officiel Kubernetes pèse des dizaines de mégaoctets dans un binaire
qui tient à ne rien embarquer. **Solution** : parler l'API en REST avec le jeton du
ServiceAccount, un `watch` n'étant qu'un flux HTTP. L'accès au cluster est déjà demandé par le
tunnel développeur, donc rien de neuf pour l'exploitant. Découpage : **Docker/Compose en
communautaire**, **Kubernetes en Enterprise** sous la clé `cluster`.

**Le port publié n'est pas le port utile.** Un service Swarm ne déclare dans son `Endpoint` que
les ports **publiés**, et un service devant lequel on met une passerelle n'en publie
généralement aucun : on l'atteint dans le réseau overlay, ce qui est précisément la raison de la
passerelle. Sur une pile réelle de trente services, la première version en laissait onze sans
port, et c'étaient les onze intéressants. Le repli est le `EXPOSE` de l'**image** : le numéro que
l'auteur du service a écrit lui-même. Il peut se tromper (une image de base qui expose 80 sous
une application qui écoute 3000), donc il ne fait que **remplir** le champ, jamais le figer - une
entrée que l'exploitant corrige vaut mieux que pas d'entrée, puisque pas d'entrée veut dire
retaper l'URL de mémoire, l'erreur même que cette liste existe pour supprimer. Au mieux de
l'effort : une image que le démon ne détient pas répond 404 et le service reste sans port, une
inspection par image distincte, huit en parallèle.

**Le nom offert est celui qui RÉSOUT, pas l'identité.** Un service de stack répond à deux noms :
l'alias court que la stack enregistre sur le réseau (`mongodb`) et son nom complet
(`neo_mongodb`). Les deux marchent, mais le court est celui que l'on écrit, et proposer le long
est techniquement juste et se lit comme une erreur - pire que l'un ou l'autre. L'alias mène donc,
et le nom complet suit dans l'indice, pour que l'identité ne soit jamais cachée. Un alias n'est
offert que s'il est **sans ambiguïté sur le réseau qui le porte** : le DNS est par réseau, donc
deux stacks peuvent porter `cache` sans se gêner tant qu'elles ne partagent pas de réseau, et
quand elles en partagent un, l'alias résout au hasard - ce qui ne se met pas devant quelqu'un
comme une suggestion. Les réseaux, eux, sont résolus en **noms** : un identifiant de réseau ne
dit pas si cette passerelle peut joindre le service.

**La frontière est le RÉSEAU, pas la stack.** Swarm inscrit un service, et l'alias court que sa
stack demande, dans le DNS de **chaque réseau qu'il rejoint** : une passerelle attachée à ce
réseau les résout tous les deux, quelle que soit la stack de l'un ou de l'autre. Le
transversal-stack marche donc, et le nom court marche avec. L'autre moitié de la phrase est celle
qui mord : un service sur un réseau que la passerelle n'a pas rejoint ne résout sous **aucun**
nom - aller chercher le nom complet n'achète rien, il n'y a personne à qui demander. La
découverte lit donc les réseaux de son propre conteneur (`/proc/self/mountinfo`, puis
l'inspection) et marque ce qu'elle ne peut pas joindre : toujours offert, mais estompé, après les
autres, avec le nom du réseau en guise de réponse à « pourquoi pas celui-là ». Hors conteneur -
une instance de développement sur le poste - rien n'est marqué et l'indice le dit (« 42 dans le
cluster ») : deviner et cacher un amont qui marche serait pire qu'en offrir un douteux.
Corollaire sur les alias : quand la portée est connue, **tous les réseaux joints forment une
seule portée**, car une passerelle attachée à deux réseaux portant chacun un `cache` n'en résout
aucun de façon fiable.

**Ce qui manque se voit avant d'appuyer.** Un Save grisé qui ne dit rien envoie chercher le champ
fautif dans onze sections, et la console laissait partir des routes que la passerelle refusait en
422. Le formulaire porte donc une seule liste de manques, lue de trois façons : Save désactivé,
la section fautive en couleur d'erreur dans le menu de gauche, et à côté de Save un bouton
« N à corriger » dont chaque ligne nomme la section et y saute. **Les règles viennent du
catalogue**, pas d'une liste écrite à la main : un argument marqué `required` laissé vide est
exactement ce que le serveur refusera (la console vide les arguments blancs avant d'envoyer, donc
il arrive absent) - ce qui couvre d'un coup les predicates, les gates, les modifiers et le
terminal, et reste vrai le jour où une brique gagne un paramètre. Le verdict porte sur **ce qui
part**, pas sur le brouillon : une ligne de predicate ajoutée et pas encore remplie est jetée à
l'envoi, l'accuser reviendrait à reprocher au formulaire une faute que la charge utile n'a pas.
S'y ajoutent un amont conforme à `gateway.Validate` en mode proxy, et **au moins un predicate** -
`/**` et « aucun predicate » filtrent identiquement, mais écrit il se lit comme une décision et
se voit dans le tableau, tandis que vide il ne se distingue pas d'un oubli qui fait taire toutes
les routes en dessous. Enfin un champ obligatoire et **vide** se peint en erreur sans attendre
d'être touché : Material attend le premier clic, ce qui est juste pour un formulaire qu'on
remplit et faux pour celui dont la question est « que manque-t-il ».

**Les sections mortes se désactivent.** `CompileFilters` jette **tous** les filtres entrants
quand une route porte un terminal (« incoming filters ignored: this route answers by itself »),
et le renvoi d'identité en est un. Sur une route qui répond d'elle-même, *Incoming* et *Identity*
étaient donc éditables pour rien, et la seule trace du sacrifice était une ligne de log. Ils sont
grisés, avec la raison en infobulle. *Outgoing* et *Gates* restent vivants : le code applique les
filtres de réponse à ce qu'une route répond elle-même exactement comme à une réponse proxifiée.

**Le champ ne compose que du Material standard.** `app-url-input` était un `MatFormFieldControl`
écrit à la main autour d'un `<input>` nu que le `mat-form-field` du parent adoptait : soixante
lignes qui réimplémentaient le focus, le flottement du label et le `describedBy` que Material
fait déjà. Il possède désormais son propre `mat-form-field` et n'assemble que des pièces du
catalogue - un `mat-select` en préfixe pour le schéma, un `matInput`, un `mat-autocomplete` -
exactement la forme d'`app-form-field`, qui existait déjà. Le label flotte toujours : un préfixe
est visible au repos, et un label posé s'y lirait comme une valeur déjà saisie.

**La liste vit DANS le champ**, pas à côté. Un second contrôle pour une seule valeur se lit comme
une seconde valeur, et une boîte vide n'annonce pas qu'elle contient une liste : on retape l'URL
de mémoire, ce que la liste existe pour éviter. Le champ *Upstream* propose donc les services à
la prise de focus et les filtre à la frappe ; la saisie libre reste entière, un amont hors cluster
se tape. Une suggestion ne porte que **l'hôte** (`service:9191`), pas le schéma, qui garde son
propre sélecteur : par défaut **http**, puisque à l'intérieur du cluster TLS s'arrête à la
passerelle.

### La spec OpenAPI déposée (SVC-06)

Quand le service ne publie aucune spec - produite au build, service tiers, amont fermé - on
dépose le fichier : `api.spec { type: upstream|file, path, filename }`. `upstream` est une
référence vivante, relue à chaque écran ; `file` un instantané, que seul un nouveau dépôt change,
et la console le dit. Dans les deux cas la spec **a une URL sur la route** - servie par l'amont,
ou servie par Meerkat - ce qui unifie l'accès pour un `curl`, pour Postman et pour un générateur
de client. Le fichier déposé hérite de la **règle d'accès de la route**, sans case publique : on
décore la route, on ne perce pas un trou dedans. Il masque ce que l'amont sert au même chemin,
ce qui est l'usage voulu (remplacer une spec incomplète) et s'annonce au dépôt, en sondant
l'amont.

**Le stockage.** `api` est une colonne JSON relue par chaque `ListRoutes` - le tableau de la
console, chaque rechargement du routeur : y mettre le contenu, ce sont des mégaoctets relus pour
afficher une liste de noms. Le contenu vit donc dans `route_specs`, clé (route, genre), et le
modèle ne porte que le nom. C'est le premier binaire du produit rangé à part : les images de la
marque sont des data URI dans un réglage.

**Les formats.** OpenAPI n'admet que JSON et YAML, jamais TOML, et `Rewrite` comme
`InjectSimulation` ne travaillent que sur du JSON : une spec YAML leur ressortait inchangée, donc
un amont qui publie du YAML perdait le reciblage vers la gateway et l'Authorize de simulation, en
silence. La conversion se fait **à la volée**, au même endroit pour les deux sources - un seul
chemin, ce bug réparé au passage, et le fichier déposé rendu tel quel à l'export. Pas via
libopenapi, dont le rendu JSON ignore Swagger 2.0, mais par `yaml` puis `json`, qui écrit en
chaîne les codes de réponse que le YAML lit comme des entiers. On sert toujours du JSON : `path`
porte `.json`, `filename` garde l'extension d'origine.

**L'export.** Le contenu est un média, comme le logo : absent du YAML simple, présent dans le
paquet ZIP (CFG-05) sous `assets/specs/<route>/<fichier>` - un dossier par route, parce que les
trois images ont pu prendre des noms fixes seulement puisqu'il n'y a qu'une marque. Un import qui
ne le porte pas ne détruit rien et le signale (`missingFiles`) ; les règles par endpoint, elles,
sont du texte et voyagent toujours.

**Ce qui reste ouvert.** Le swagger développeur lit le fichier déposé **dans la base** plutôt qu'à
travers la route : cette page liste délibérément les routes désactivées, et une route désactivée
n'est pas compilée, donc la demander au routeur répondrait 404 sur exactement celles qu'un
développeur ouvre pour voir ce qui se construit. Deux chemins de lecture subsistent donc, pour
une raison qui tient.

### Le mode dev est global, pas par développeur (DEV-05, DEV-06, DEV-08)

Une substitution vaut aujourd'hui pour tout le trafic ; deux développeurs sur le même nom se
marchent dessus. C'est assumé et compensé par l'annonce (halte après connexion, bandeau
permanent). **Ce qui manque** : que le trafic d'un développeur emprunte ses substitutions et
celui des autres non, avec un menu de choix pour la capacité `tester`. **Point dur** : la
décision se prend par requête, donc elle doit être lisible depuis la session sans coûter une
lecture de base.

### L'observabilité est vide (OBS-01, OBS-04, OBS-05)

Aucune métrique, aucune trace, un `/healthz` qui répond toujours UP - liveness et readiness
pointent le même chemin, donc un nœud dont la base est tombée se déclare prêt. **Solution
minimale** : séparer les deux sondes et faire vérifier la base par la readiness. Ensuite un
`/metrics` optionnel, et la propagation de `traceparent` vers l'amont, qui ne coûte qu'un
en-tête à recopier.

### Le pilotage par un agent (MCP-01 à 06)

La moitié lecture est livrée. Ce qu'il a fallu trancher, et qui vaut d'être retenu :

**Le jeu d'outils n'est pas une frontière de sécurité.** La tentation était de borner l'agent
par ce qu'on lui propose. Faux : le même jeton ouvre toute l'API REST sur le même port, et un
agent a `curl`. La frontière est le **périmètre du jeton**, vérifié dans l'entonnoir unique par
lequel passent les deux surfaces (`admin.authed`).

**Ce qui compte comme une lecture se décide par endpoint, pas par le verbe.** Une dizaine de
POST ici ne changent rien (le testeur de routage, les aperçus, la sonde de relais), et
l'endpoint agent est lui-même un POST qui porte les deux. Une règle par verbe interdirait
exactement ce à quoi sert un jeton en lecture seule. D'où une liste, et un test qui refuse tout
endpoint non-GET que personne n'a classé.

**L'édition se dit, elle ne se refuse pas.** Un outil Enterprise n'est pas dans la liste que
rend l'image communautaire - le code n'y est pas - donc l'agent ne tente pas ce qu'il ne voit
pas. Mais `describe_gateway` nomme l'édition et ce que l'autre apporterait, pour que l'agent
puisse *dire* « ça demande l'Enterprise » au lieu de caler.

**L'écriture passe par un brouillon à activer : essayé, puis retiré.** La première version
faisait déposer à l'agent une configuration qu'une personne activait ensuite dans la console.
L'argument était bon sur le papier (le travail d'un agent est une séquence, appliquée en direct
elle traverse des états que personne n'a dessinés) et faux en pratique : personne ne demande ça
à un agent, pas plus qu'un agent GitHub ne demande d'aller confirmer sa pull request dans
l'interface. On paie une cérémonie sur **chaque** changement pour un risque qui se traite
autrement. L'agent écrit donc directement, et le filet est celui qui existait déjà : le **point
de restauration automatique** après chaque changement (CFG-06), le journal qui nomme le jeton, et
le périmètre qui décide de ce qui est même proposé. Ce qu'un admin prudent fait en plus - « range
la conf actuelle sous tel nom avant de commencer » - est un **outil**, pas un rite : payé par qui
le demande.

**Le périmètre n'a pas eu besoin d'un second modèle de droits.** Le domaine **masque** les
capacités du porteur au lieu d'ajouter des règles à côté : chaque garde de ce paquet lit déjà
ces booléens, donc les masquer une fois confine l'API REST, les outils de l'agent et ce qu'on
ajoutera le mois prochain, sans liste à tenir à jour. Root est **retiré** et non conservé - un
domaine qui laisserait root debout ne confinerait rien.

**La restriction d'adresse se juge sur le pair TCP**, jamais sur `X-Forwarded-For`, qui s'écrit
à la main. La conséquence se dit à l'écran plutôt que de se découvrir : un plan de contrôle
derrière un reverse proxy voit l'adresse du proxy pour tout le monde, donc la restriction n'a de
sens que si les agents atteignent le port directement. Un refus est un « pas de session » muet
pour l'appelant, mais **journalisé** côté serveur, sinon un admin n'aurait rien pour comprendre.

**Le branchement sans secret vaut son coût.** Ce que fait Jira - une URL, un navigateur, une
approbation, aucun jeton copié - demande que Meerkat soit son propre serveur d'autorisation :
métadonnées, enregistrement dynamique du client, page de consentement, PKCE, rafraîchissement.
C'est fait, et ce n'est pas jetable : la moitié est le socle de **SAUTH-05**, où la même
mécanique fera face à des applications au lieu d'agents. Deux choix qui font le reste : le jeton
émis **est** un jeton de plan admin ordinaire (donc rien à réécrire pour le périmètre, le journal
ou la révocation), et le **périmètre se choisit sur la page de consentement** - la personne qui
branche l'agent est celle qui décide de ce qu'il pourra faire, au moment où elle le branche.

**Le jeton reste en secours.** Tous les clients ne savent pas faire le tour par le navigateur ;
retirer le jeton, c'est fermer la porte à ceux-là. La section MCP montre l'OAuth d'abord et
renvoie vers Access tokens pour les autres.

**Écarté** : un client en ligne de commande qui piloterait tout Meerkat. Un admin utilise la
console ou son agent ; le troisième outil serait une deuxième console à maintenir, en moins
bien. Le seul cas qui restait, un pipeline d'intégration continue, est déjà servi par l'API
REST avec le même jeton.

### La documentation (QUAL-02, PAGE-03)

Le site public a cinq pages ; il n'y a ni guide d'architecture, ni guide d'exploitation
réunissant variables d'environnement, certificats et dérive d'horloge TOTP. Et les catalogues de
traduction des pages servies sont compilés dans le binaire : ajouter une langue reste un rebuild
(PAGE-03), alors que l'intégrateur devrait pouvoir en poser une.

## Décisions structurantes

Ce qui suit explique la forme du produit. La raison d'un choix se perd plus vite que le choix.

**Le cœur est en Go**, la console en Angular. Rust a été écarté : l'argument marketing est réel,
mais la vélocité, la lisibilité pour un contributeur et l'écosystème identité (WebAuthn, TOTP,
OIDC, LDAP, clients d'orchestrateur) sont tous du côté de Go. Contrainte tenue depuis : **Go
pur, pas de CGO**, ce qui a décidé du moteur SQLite embarqué comme du refus d'embarquer un
client Kubernetes officiel.

**Le stockage est embarqué par défaut, PostgreSQL au-delà.** MongoDB est abandonné. Le choix
n'appartient pas à l'utilisateur final mais à l'intégrateur, et il se réduit à poser une URL :
tout le reste du produit ignore laquelle des deux répond.

**La propagation entre nœuds passe par les primitives de la base** (`LISTEN`/`NOTIFY`), pas par
un courtier obligatoire - c'est fait (STORE-03). Un courtier reste branchable, il ne sera jamais
un prérequis : une gateway qui exige un Kafka pour démarrer n'est plus une gateway qu'on installe
en une heure.

**Deux formes de session, pas une.** Côté navigateur, un cookie **opaque** dont l'état vit en
base, parce que c'est la seule forme qui se révoque à la seconde ; côté API, un **jeton porteur**.
Les deux sont acceptés sur les mêmes endpoints. Le tout-JWT côté navigateur a été écarté pour
cette seule raison : sa révocation est une promesse, pas un mécanisme.

**Il n'y a pas d'entité Service.** Elle a été cadrée puis retirée avant d'être construite : une
route par service est le cas courant, et l'entité ajoutait un objet à créer, nommer et tenir à
jour pour ne payer que le cas rare. La route porte donc son amont, sa spec OpenAPI, son
send-auth, ses locales et sa sécurité ; le canary se fait par le prédicat `weight` entre deux
routes, et le mode dev substitue un **nom d'amont**.

**Éditions : dépôt public, deux images, aucune clé de licence.** Le cœur est sous **FSL** (usage
interne et production libres, seule la revente concurrente est fermée, conversion automatique en
Apache 2.0 après deux ans) ; le code Enterprise vit dans `/ee` sous licence commerciale, source
visible. La séparation est faite par le **linker** (tag de build `ee`), et **l'image est la
licence** : rien à valider, rien à faire expirer, rien qui appelle un serveur. La ligne de
partage, arrêtée le 2026-08-08 : *ce qui coûte à l'organisation qui grossit se paie, ce qui
protège l'utilisateur ne se paie pas* - faire payer la sécurité pousserait à déployer moins sûr.

**La console est la source de vérité de la configuration**, avec des versions internes qu'on
duplique, compare et active. Un fichier ne sert qu'à **semer** une instance vide au premier
démarrage ; ensuite il devient une version parmi d'autres. Le GitOps reste possible chez le
client en versionnant les exports, sans que la gateway l'impose.

**plug est embarqué, pas installé à côté.** L'agent vit dans le binaire Enterprise. Le transport
est un tunnel inversé - la gateway ne connaît jamais l'adresse du poste - et l'authentification
se fait par **clé publique SSH par développeur** : révoquer, c'est supprimer une ligne. Le fork
envisagé n'a pas eu lieu ; les verbes manquants ont été demandés en amont et livrés dans plug.

**La console est en anglais et ne sera pas traduite** : c'est l'outil d'un exploitant, servi à la
racine sans segment de langue. Les **pages du plan de données**, elles, sont traduites - vingt
catalogues - parce que ce sont les seules pages qu'un utilisateur final voit.
