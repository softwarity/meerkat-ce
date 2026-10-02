---
title: Ce qu'elle fait
section: Le produit
order: 3
summary: Chaque capacité de la passerelle, domaine par domaine, avec l'état lu dans le code et l'identifiant qui permet de le vérifier.
---

# Ce qu'elle fait

Meerkat prend en charge ce dont une application interne ou destinée à des
clients a besoin, et qu'aucune équipe ne devrait écrire deux fois. Ce qui suit
est tout le produit, domaine par domaine.

Chaque ligne nomme son identifiant dans
[FEATURES.md](https://github.com/softwarity/meerkat-ce/blob/main/FEATURES.md), le
tableau du dépôt dont l'état est lu dans le code et non dans un plan : chaque
affirmation ici s'y vérifie. **Enterprise** marque ce que seule l'image payante
porte ; *partiel* marque une capacité utilisable dont une part nommée reste à
livrer. Ce que la même liste coûte quand on l'assemble soi-même est sur
[le dossier Meerkat](/product/the-case).

## Connexion et identité

Les pages que vos utilisateurs voient, servies par la gateway, à vos couleurs.

À lire : [Comment on se connecte](/docs/auth/overview).

- **Pages de connexion servies** : login, mot de passe oublié, vérification d'e-mail, inscription en option, dans 20 langues, thème, logo et fond d'écran à votre marque. `AUTH-01 THEME-01`
- **Mot de passe** avec politique configurable et historique, anti-force brute partagé par tous les nœuds, messages qui ne trahissent pas l'existence d'un compte. `AUTH-10 AUTH-11`
- **Second facteur TOTP** avec QR code et codes de secours, repli par code e-mail, navigateurs de confiance. `MFA-01 à 03`
- **MFA obligatoire** global, par autorité ou par utilisateur. *partiel* `MFA-04`
- **Passkeys WebAuthn** : clé physique, empreinte, Windows Hello, en premier facteur. *partiel* `AUTH-15`
- **Fédération OIDC et GitHub** : Entra ID, Okta, Google, Keycloak ou tout IdP conforme, pour le premier facteur. `AUTH-04`
- **LDAP et Active Directory**, et règles de groupe : un groupe d'annuaire, une équipe GitHub ou un claim OIDC devient une appartenance et des rôles. **Enterprise** `AUTH-03 RBAC-10`
- **Connexion par code e-mail** : un code à usage unique à la place du mot de passe, lié au navigateur qui l'a demandé, dix minutes, jamais sur la console, et le second facteur tourne quand même derrière. Livré éteint. *partiel* `AUTH-16`
- **Jetons d'API** personnels et machine à machine, secret affiché une seule fois. `AUTH-09`
- **Comptes** avec fenêtre de validité en jours, champs propres à votre métier transmis aux applications, alerte e-mail sur connexion depuis un nouveau navigateur. `MODEL-01 MODEL-02 AUTH-23`

## Droits et organisations

Qui accède à quoi, décidé à la porte, jamais dans chaque application.

À lire : [Authentifier et autoriser](/docs/access/overview).

- **Rôles hiérarchiques** et groupes de rôles par organisation, en cumul ou en choix unique à la connexion. `RBAC-01 à 03`
- **Organisations clientes** isolées par la gateway, membres administrateurs ou utilisateurs, choix de l'organisation à la connexion. Le mode mono-organisation est gratuit, le multi-organisation est Enterprise. **Enterprise** `TENANT-01 à 08`
- **Accès par route** sur l'organisation, le rôle et le compte ; un refus atterrit sur une page qui explique, jamais sur une ligne de texte. `RBAC-06`
- **Sécurité par endpoint** tirée de la spec OpenAPI du service, éditée dans une console façon Swagger. `RBAC-07 CONSOLE-04`
- **Administration déléguée étanche** : super-administrateur, administrateur d'infrastructure, administrateur applicatif, administrateur d'organisation, self-service. `RBAC-04 RBAC-05`
- **Plages horaires d'accès** par organisation, jours, dates et fuseau. **Enterprise** *partiel* `TENANT-04`
- **Identité transmise à vos services** en en-têtes, en REMOTE_USER ou en JWT signé (ES256, EdDSA, RS256) avec JWKS publié et rotation des clés sans coupure ; les rôles transmis se filtrent par expression. `SAUTH-01 AUTH-07 ROUTE-18`

## Appels planifiés

La passerelle appelle vos services à l'heure dite, sans courtier à installer.

À lire : [Appels planifiés](/docs/operations/scheduler).

- **Une tâche est une route, un chemin et une cadence** (ou un calendrier cron, lu dans un fuseau) : le service demande qu'on l'appelle, la passerelle fait l'appel par sa porte d'entrée, donc toutes les règles posées devant le service s'appliquent. *partiel* `SCHED-01`
- **Ou une date, une fois** : une action différée - le déclencheur est quelque chose qui est arrivé, pas un calendrier. Le service pose le moment où il veut être rappelé, et la tâche est terminée une fois l'appel parti. `SCHED-02`
- **Elle s'exécute comme `meerkat`, munie des rôles que la tâche demande** : aucun compte de service à créer ni à tenir à jour, et une tâche atteint exactement ce que ses rôles atteignent. Le service la gère depuis le plan de contrôle avec un jeton dédié, et retrouve les siennes par ses propres métadonnées. `SCHED-01`
- **Au moins une fois**, avec un identifiant d'exécution à dédupliquer : un appel coupé par une passerelle qui s'arrête est renvoyé par une autre, même identifiant, et une réponse - un échec compris - n'est jamais rejouée. Un **202** garde l'exécution ouverte et le service rapporte sa progression, ce qui rend un travail de trois heures exprimable sans requête de trois heures. `SCHED-01`
- **Ni tic, ni verrou** : la passerelle dort jusqu'au prochain tour dû et se réveille à la seconde ; en cluster les appels se répartissent entre les nœuds, et un service lent ne retient que son propre appel. `SCHED-01`
- **Écran de console en direct**, filtrable par organisation, service et métadonnées : suspendre, avancer, supprimer. `SCHED-01`
- **Chaque tour est gardé** : quand il s'est terminé, comment, ce qui a répondu, quel noeud a fait l'appel - et un tour abandonné ou en échec se rejoue depuis l'écran ou l'API. `SCHED-03`
- **Trois tentatives, puis le tour suivant** : la poignée de réponses qui sont le plus souvent un moment devant un service interne - pas encore connu, rôles pas chargés, personne qui répond - repart deux fois, jamais au-delà du rattrapage de la tâche. Un 500, non : là le service dit quelque chose. `SCHED-04`
- **Ou le service nomme lui-même le moment** : un `424` avec un `Retry-After` - l'extraction n'est pas encore publiée - et le tour revient à ce moment-là, la raison gardée sur l'exécution qui l'a donnée. `SCHED-05`

## Routage et protection du trafic

Une API gateway complète, pilotée depuis la console.

À lire : [Les routes](/docs/concepts/routes), et ce qu'une requête coûte en
[performance](/product/performance).

- **Routes modifiées à chaud**, sans redémarrage, propagées à tous les nœuds en une seconde. `ROUTE-01`
- **11 prédicats et 33 filtres** : chemin, hôte, en-tête, cookie, méthode, poids pour le canary, plage horaire ; réécriture des requêtes et réponses. `ROUTE-03 à 05`
- **Limitation de débit** par route, utilisateur, jeton, organisation ou adresse, plusieurs bornes à la fois ; **quotas par endpoint** ; réponse 429 standard. `ROUTE-08 QUOTA-05`
- **Disjoncteur, timeouts** à trois niveaux et état des services dans la console : un coeur par route, d'après la découverte ou une connexion TCP, et le trafic réel. `ROUTE-07 ROUTE-09 SVC-04`
- **WebSocket, gRPC et streaming** des corps de bout en bout. *partiel* `ROUTE-13 ROUTE-20`
- **Découverte des services** Docker, Swarm et Kubernetes au moment de créer une route. *partiel* `SVC-02`
- **Testeur de routage** : composer une requête fictive et voir quelle route la prend, et pourquoi. `ROUTE-15`
- **Page de maintenance** par route ou pour toute la plateforme d'un seul geste, traduite, avec une porte pour les administrateurs. `LIFE-05`

## Dans vos applications, sans les modifier

Ce que la gateway ajoute aux pages qu'elle sert.

À lire : [Ce que la passerelle injecte](/docs/concepts/data-plane-chrome).

- **Bouton utilisateur** injecté : profil, déconnexion, changement d'organisation, langue, clair ou sombre. `UIF-03 UIF-05`
- **Portail de navigation** entre vos applications, en barre ou en rail, qui ne montre que ce que les droits autorisent. `PORTAL-01`
- **Masquer l'interface selon les rôles** en CSS pur : les rôles sont posés sur la page côté serveur. CSS et JavaScript injectables par route. `UIF-02 UIF-06`
- **Signalement d'anomalies** : capture d'écran, console, contexte technique, suivi dans la console d'administration. `ISSUE-01 à 04`
- **Canal temps réel** WebSocket vers les applications, sans intégration préalable. `UIF-04`
- **Rien de personnel en cache** : toute page qui porte une identité est rendue non stockable, quoi qu'en dise l'application. `SEC-10`

## Sécurité de la plateforme

Les briques qu'on installe d'habitude à côté.

À lire : [Le coffre](/docs/operations/vault).

- **Certificats TLS** par nom, émis et renouvelés par ACME auprès de Let's Encrypt ou de votre autorité interne, ports HTTPS ouverts à chaud. `SSL-01 SSL-05 SSL-08`
- **Coffre intégré** : secrets chiffrés AES-256-GCM, références par nom depuis la configuration, export chiffré, rappel avant expiration. `VAULT-01 à 06`
- **En-têtes de sécurité** HSTS, CSP, X-Frame-Options, Referrer-Policy, et protection CSRF de la console. `SEC-01 SEC-03`
- **Console sur un port séparé** du trafic applicatif : l'administration n'est jamais exposée avec l'application. `CONSOLE-11`
- **Fonctionne sans internet** : aucune ressource chargée à l'extérieur, adapté aux environnements isolés. `DEPLOY-03`

## Exploitation

Une console qui remplace les fichiers YAML et les pipelines de configuration.

À lire : [Exploitation](/docs/operations/overview).

- **Configurations versionnées** : plusieurs versions nommées, une active, comparaison, export et import YAML, point de reprise automatique à chaque changement. `CFG-01 à 06`
- **Journal d'audit** de chaque action d'administration, avec le diff champ par champ, et de la sécurité des comptes - chaque connexion, chaque connexion refusée avec sa vraie raison et son adresse, chaque facteur, passkey, mot de passe ou jeton changé par son titulaire. En ajout seul, consultable dans la console. *partiel* `AUD-01 AUD-02`
- **Audit envoyé au collecteur** en OTLP : les événements de sécurité du plan de données, et les changements de la console aussi. **Enterprise** `AUD-03`
- **Audit des endpoints** : un interrupteur par opération de la spec OpenAPI d'une route, et chaque appel devient un événement d'audit. **Enterprise** *partiel* `AUD-04`
- **Export CSV** du journal d'audit. Parquet reste à livrer. **Enterprise** *partiel* `STORE-06`
- **Tableaux de bord intégrés** : trafic, latence et échecs par route et par endpoint, sans rien installer. `OBS-01`
- **Métriques poussées en OTLP** vers le collecteur des traces, qui les écrit dans Prometheus - avec un tableau de bord Grafana fourni. **Enterprise** `OBS-06`
- **Journaux structurés**, en JSON ou en texte, et un **journal d'accès** - une ligne par requête franchissant la porte d'entrée, refus compris, avec le compte tel que la passerelle l'a authentifié. C'est la moitié de l'audit qu'aucun service ne peut écrire : il n'a jamais vu l'appel qu'on lui a refusé, et il ne sait de l'appelant que ce qu'on lui en a dit. Les deux journaux peuvent s'écrire en JSON OpenTelemetry pour un agent Collector, ou être poussés au collecteur (**Enterprise**). *partiel* `OBS-03`
- **Traces distribuées** (W3C Trace Context) : le contexte traverse dans les deux éditions et un identifiant est posé sur chaque requête - rendu à l'appelant, écrit dans le journal, au pied des pages intégrées -, ce qui **joint une ligne de la passerelle à l'audit métier d'un service**. En Enterprise, la passerelle se déclare sur la trace : son span d'entrée, celui de l'appel amont, et l'écart entre les deux qui est son temps propre. Export **OTLP** vers un OpenTelemetry Collector ou tout endpoint OTLP. Le paquet OpenTelemetry peut être injecté dans les pages UI - servi par Meerkat, jamais par un CDN - pour que la trace commence au clic. **Enterprise** *partiel* `OBS-04`
- **Cluster actif/actif** sur PostgreSQL, sans affinité de session ni nœud primaire. **Enterprise** `PERF-03 STORE-03`
- **E-mails transactionnels** aux couleurs du thème et résumé quotidien des comptes qui expirent. `NOTIF-01 NOTIF-04`
- **Déploiement** : une image, base embarquée par défaut, Docker, Swarm ou Kubernetes avec chart Helm, sondes de vivacité et de disponibilité, amorçage par fichier. `DEPLOY-01 OBS-02 LIFE-02`

## Pour vos développeurs

Tester sur le vrai cluster sans rien déployer.

À lire : [Mode développement](/product/dev-mode).

- **Poste vers cluster** avec **softwarity/plug** : le service lancé sur le poste d'un développeur rejoint le cluster sous son nom, remplace le service déployé le temps de la session, puis le cluster est restauré à l'identique. N'importe quel langage, sans changer le code, depuis Linux, macOS ou Windows. `plug`
- **plug autonome, avec Community** : un conteneur agent ajouté à votre stack Docker, Swarm ou Kubernetes, gratuit sous licence FSL. `plug`
- **plug intégré, avec Enterprise** : l'agent vit dans la gateway, rien à déployer à côté ; chaque développeur s'authentifie par sa clé SSH, et chaque page signale le service servi depuis un poste. **Enterprise** *partiel* `DEV-02 DEV-03 DEV-04`
- **Mode test UI** : naviguer avec une identité simulée pour voir exactement ce qu'un rôle voit. `DEV-10`
- **Swagger UI embarqué** sur les specs OpenAPI de **toutes** les routes, sans CDN : les appels y passent par la passerelle, donc par l'authentification et les règles de la route, et l'identité du *Try it out* se **simule** - un utilisateur, des groupes ou des rôles - pour voir ce que l'API répond à chacun. `DEV-09 LIFE-04`

## Piloté par un agent IA

L'exploitant demande en mots, la gateway exécute et trace.

À lire : [Le point d'entrée agent](/docs/agent/overview).

- **Serveur MCP intégré** : Claude Code, Gemini CLI, Codex CLI se branchent sur la gateway et lisent, testent ou modifient les routes. `MCP-01 MCP-04`
- **Branchement OAuth sans secret copié**, jeton à périmètre (lecture seule ou complet, domaine, plages réseau), révocable d'un clic. `MCP-02 MCP-07`
- **Chaque action de l'agent est auditée** sous son nom, avec un point de reprise automatique pour revenir en arrière. `MCP-03 MCP-05`

## Une app-gateway, et ce que cela implique

Meerkat sert **une** application faite de plusieurs services : des utilisateurs
qui ont un nom, des rôles, une organisation, et une seule porte devant tout
cela. C'est cette décision qui explique le reste - pourquoi l'identité, les
rôles, les organisations et les pages de connexion sont dans le produit plutôt
qu'à côté, et pourquoi la console est un outil d'exploitant plutôt qu'un
éditeur de YAML.

Si votre besoin est d'exposer des API à des tiers - portail développeurs, clés
par partenaire, facturation à l'appel - c'est le travail d'une API gateway, et
le dire ici vous fait gagner du temps.

> [!NOTE]
> L'anti-pattern qu'elle existe pour casser : installer la gateway, puis
> Prometheus, puis Grafana, puis écrire du YAML pour tout. Ici vous lancez un
> binaire, vous configurez dans la console, vous exportez, et vous rejouez
> l'export n'importe où.

## Deux éditions

La gratuite est toute la passerelle pour une organisation sur une instance.
Enterprise est ce dont une installation a besoin quand elle grandit : plusieurs
organisations, votre annuaire d'entreprise, plusieurs passerelles derrière une
seule entrée.

La règle de ce qui tombe de chaque côté tient en une ligne, et c'est celle à
vérifier quand vous comparez : **jamais une primitive de sécurité**. TLS, le
coffre, le second facteur, les passkeys, le journal d'audit et la sécurité par
endpoint sont dans l'image gratuite, et y resteront.

[Comparer les éditions](/product/editions), ou aller directement aux
[tarifs](/product/pricing).
