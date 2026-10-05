---
title: Ce qu'elle fait
section: Le produit
order: 3
summary: Toutes les capacités de la gateway, domaine par domaine, avec leur état lu dans le code.
---

# Ce qu'elle fait

Meerkat se charge de ce dont a besoin une application, interne ou destinée à
des clients, et qu'aucune équipe ne devrait écrire deux fois. Voici tout le
produit, domaine par domaine.

**Enterprise** signale ce
que l'image Community ne contient pas ; *en partie* signale une capacité
utilisable dont une part, nommée, reste à livrer. Ce que coûte la même liste
quand on l'assemble soi-même est détaillé dans
[le dossier Meerkat](/product/the-case).

## Connexion et identité

Les pages que voient vos utilisateurs, servies par la gateway, à vos
couleurs.

Pour en savoir plus : [Comment on se connecte](/docs/auth/overview).

- **Pages de connexion servies par la gateway** : connexion, mot de passe oublié, vérification de l'adresse e-mail, inscription en option, en 20 langues, avec votre thème, votre logo et votre image de fond.
- **Mots de passe** avec une politique et un historique configurables, une protection contre la force brute commune à tous les nœuds, et des messages qui ne révèlent jamais si un compte existe.
- **Second facteur TOTP** avec QR code et codes de secours, code par e-mail en solution de repli, navigateurs de confiance.
- **Second facteur imposé** globalement, par autorité ou par utilisateur. *en partie*
- **Passkeys WebAuthn** : clé de sécurité, empreinte digitale, Windows Hello, comme premier facteur. *en partie*
- **Fédération OIDC et GitHub** : Entra ID, Okta, Google, Keycloak ou tout fournisseur d'identité conforme, pour le premier facteur.
- **LDAP et Active Directory**, et règles de groupe : un groupe de l'annuaire, une équipe GitHub ou un claim OIDC devient une appartenance et des rôles. **Enterprise**
- **Connexion par code reçu par e-mail** : un code à usage unique à la place du mot de passe, lié au navigateur qui l'a demandé, valable dix minutes, jamais sur la console, et le second facteur s'applique toujours ensuite. Désactivée à la livraison. *en partie*
- **Jetons d'API**, personnels ou de machine à machine, dont le secret n'est affiché qu'une fois.
- **Comptes** avec une durée de validité en jours, des champs personnalisés propres à votre métier et transmis aux applications, et une alerte par e-mail lors d'une connexion depuis un nouveau navigateur.

## Accès et organisations

Qui accède à quoi se décide à la porte, jamais dans chaque application.

Pour en savoir plus : [Authentifier et autoriser](/docs/access/overview).

- **Rôles hiérarchiques** et groupes de rôles par organisation, cumulés ou choisis à la connexion.
- **Organisations clientes** isolées par la gateway, membres administrateurs ou utilisateurs, organisation choisie à la connexion. Le mode à une seule organisation est gratuit, le mode à plusieurs organisations relève d'Enterprise. **Enterprise**
- **Accès par route** selon l'organisation, le rôle et le compte ; un refus aboutit à une page qui explique, jamais à une ligne de texte.
- **Sécurité par endpoint** déduite de la spécification OpenAPI du service, et modifiée dans une console à la manière de Swagger.
- **Administration déléguée et cloisonnée** : super-administrateur, administrateur de l'infrastructure, administrateur des applications, administrateur d'organisation, libre-service.
- **Plages horaires d'accès** par organisation, selon le jour, la date et le fuseau horaire. **Enterprise** *en partie*
- **Identité transmise à vos services** sous forme d'en-têtes, de REMOTE_USER ou d'un JWT signé (ES256, EdDSA, RS256), avec un JWKS publié et une rotation des clés sans interruption ; les rôles transmis sont filtrés par une expression.

## Appels planifiés

La gateway appelle vos services à l'heure que vous indiquez, sans serveur de
messages à installer.

Pour en savoir plus : [Appels planifiés](/docs/operations/scheduler).

- **Une planification, c'est une route, un chemin et une cadence** (ou un calendrier cron, lu dans un fuseau horaire) : le service demande à être appelé, la gateway passe l'appel par sa propre porte d'entrée, et toutes les règles placées devant ce service s'appliquent donc.
- **Ou bien une date, une seule fois** : une action différée, dont le déclencheur est un événement et non un calendrier. Le service indique le moment où il veut être rappelé, et la planification est terminée une fois ce moment passé.
- **L'appel s'exécute sous l'identité `meerkat`, avec les rôles que demande la planification** : aucun compte de service à créer ni à tenir à jour, et une planification atteint exactement ce qu'atteignent ses rôles. Le service gère ses planifications sur le plan de contrôle avec son propre jeton, et retrouve les siennes grâce à ses propres métadonnées.
- **Au moins une fois**, avec un identifiant d'exécution qui permet de dédupliquer : un appel interrompu par l'arrêt d'une gateway est renvoyé par une autre, avec le même identifiant, et une réponse - même un échec - n'est jamais rejouée. Un **202** laisse l'exécution ouverte et le service rend compte de son avancement : un traitement de trois heures s'exprime ainsi sans requête de trois heures.
- **Ni battement, ni verrou** : la gateway dort jusqu'au prochain tour et se réveille à la seconde près ; en cluster, les appels se répartissent entre les nœuds, et un service lent ne retient que son propre appel.
- **Un écran de la console, en direct**, filtré par organisation, par service et par métadonnées : suspendre, avancer, supprimer.
- **Chaque tour est conservé** : quand il s'est terminé, comment, ce qui a répondu, quel nœud a passé l'appel - et un tour abandonné ou en échec se relance depuis l'écran ou depuis l'API.
- **Trois tentatives, puis le tour suivant** : les quelques réponses qui, venant d'un service interne, signifient le plus souvent qu'il est trop tôt - compte pas encore connu, rôles pas encore chargés, personne ne répond - donnent lieu à deux nouvelles tentatives, sans jamais dépasser le délai de rattrapage de la planification. Une erreur 500, non : là, le service dit quelque chose.
- **Ou bien le service indique lui-même le moment** : un `424` accompagné d'un `Retry-After` - l'extraction n'est pas encore publiée - et le tour revient à ce moment-là, la raison étant conservée sur l'exécution qui l'a donnée.

## Routage et protection du trafic

Une API gateway complète, pilotée depuis la console.

Pour en savoir plus : [Les routes](/docs/concepts/routes), et ce que coûte une
requête dans [Performance](/product/performance).

- **Routes modifiées à chaud**, sans redémarrage, et propagées à tous les nœuds en moins d'une seconde.
- **11 prédicats et 33 filtres** : chemin, hôte, en-tête, cookie, méthode, poids pour les déploiements canari, fenêtre horaire ; réécriture de la requête et de la réponse.
- **Rate limits** par route, utilisateur, jeton, organisation ou adresse, avec plusieurs limites à la fois ; **quotas par endpoint** ; réponse 429 standard.
- **Disjoncteur, timeouts** à trois niveaux, et santé des services dans la console : un cœur par route, d'après la découverte ou un test TCP, et d'après le trafic réel.
- **WebSocket, gRPC et diffusion des corps en flux** de bout en bout ; un appel gRPC est compté d'après son `grpc-status`, et non d'après le 200 qui le transporte. *en partie*
- **Découverte des services** pour Docker, Swarm et Kubernetes, à la création d'une route. *en partie*
- **Testeur de routage** : composez une requête d'exemple et voyez quelle route la prend, et pourquoi.
- **Page de maintenance** par route, ou pour toute la plateforme en un seul geste, traduite, avec une porte réservée aux administrateurs.

## Dans vos applications, sans y toucher

Ce que la gateway ajoute aux pages qu'elle sert.

Pour en savoir plus : [Ce que la gateway injecte](/docs/concepts/data-plane-chrome).

- **Bouton utilisateur injecté** : profil, déconnexion, changement d'organisation, langue, mode clair ou sombre.
- **Portail de navigation** entre vos applications, en barre ou en rail, qui ne montre que ce qu'autorisent les droits d'accès.
- **Masquage de l'interface selon le rôle**, en CSS pur : les rôles sont inscrits dans la page côté serveur. Du CSS et du JavaScript peuvent être injectés route par route.
- **Signalement d'un problème** : capture d'écran, console du navigateur, contexte technique, avec un suivi dans la console d'administration.
- **Canal temps réel** en WebSocket vers les applications, sans intégration préalable.
- **Rien de personnel dans les caches** : toute page qui porte une identité est rendue impossible à mettre en cache, quoi qu'en dise l'application.

## Sécurité de la plateforme

Les briques que l'on installe d'habitude à côté.

Pour en savoir plus : [Le coffre](/docs/operations/vault).

- **Certificats TLS** par nom, émis et renouvelés par ACME auprès de Let's Encrypt ou de votre autorité interne, avec des ports HTTPS ouverts à chaud.
- **Coffre intégré** : secrets chiffrés en AES-256-GCM, référencés par leur nom dans la configuration, export chiffré, rappel avant expiration. La même clé scelle les clés TLS et les secrets TOTP, et se renouvelle par un redémarrage.
- **En-têtes de sécurité** HSTS, CSP, X-Frame-Options, Referrer-Policy, et protection CSRF pour la console.
- **Console sur un port distinct** de celui du trafic applicatif : l'administration n'est jamais exposée avec l'application.
- **Fonctionne sans accès à internet** : aucune ressource n'est chargée depuis l'extérieur, ce qui convient aux environnements isolés du réseau.

## Exploitation

Une console qui remplace les fichiers YAML et les pipelines de configuration.

Pour en savoir plus : [Exploitation](/docs/operations/overview).

- **Configurations versionnées** : plusieurs versions nommées, une seule active, comparaison, export et import en YAML, point de reprise automatique à chaque changement.
- **Configurations dans un dépôt git** : un répertoire par plateforme dans un dépôt partagé ; un pull range la configuration sans rien appliquer tant que vous ne l'activez pas, un push la committe au nom de l'exploitant qui a cliqué. GitHub, GitLab, Bitbucket, Azure DevOps, Gitea ou votre propre serveur, avec un jeton rangé dans le coffre. **Enterprise**
- **Journal d'audit** de chaque action d'administration, avec les différences champ par champ, et de la sécurité des comptes - chaque connexion, chaque connexion refusée avec sa raison réelle et son adresse, chaque facteur, passkey, mot de passe ou jeton modifié par son propriétaire. En ajout seul, consultable dans la console. *en partie*
- **Audit envoyé au collecteur** en OTLP : les événements de sécurité du plan de données, ainsi que les changements faits dans la console. **Enterprise**
- **Audit des endpoints** : un interrupteur par opération de la spécification OpenAPI d'une route, et chaque appel devient un événement d'audit. **Enterprise** *en partie*
- **Export CSV** du journal d'audit. Le format Parquet reste à livrer. **Enterprise** *en partie*
- **Tableaux de bord intégrés** : trafic, latence et échecs par route et par endpoint, sans rien installer.
- **Métriques envoyées en OTLP** au collecteur qui reçoit déjà les traces, et qui les écrit dans Prometheus - avec un tableau de bord Grafana prêt à l'emploi. **Enterprise**
- **Journaux structurés**, en JSON ou en texte, et un **journal d'accès** - une ligne par requête qui franchit la porte d'entrée, refus compris, avec le compte tel que la gateway l'a elle-même authentifié, et le jeton d'API quand l'appel vient d'une machine. C'est la moitié d'un audit qu'aucun service ne peut écrire : il n'a jamais vu l'appel qui lui a été refusé, et il ne sait de l'appelant que ce qu'on lui en a dit. Les deux journaux peuvent être écrits en JSON OpenTelemetry pour un agent Collector, ou envoyés au collecteur (**Enterprise**). La console les affiche en direct, et règle le niveau de journalisation sur tous les nœuds.
- **Traces distribuées** (W3C Trace Context) : le contexte est propagé dans toutes les éditions et un identifiant est attribué à chaque requête - renvoyé à l'appelant, écrit dans le journal, affiché au pied des pages intégrées -, ce qui **relie une ligne de la gateway à l'audit métier d'un service**. En Enterprise, la gateway apparaît elle-même dans la trace : son span d'entrée, le span de l'appel à l'upstream, et l'écart entre les deux, qui est son temps propre. Export **OTLP** vers un OpenTelemetry Collector ou vers tout endpoint OTLP. Le paquet OpenTelemetry peut être injecté dans les pages d'interface - servi par Meerkat, jamais par un CDN - pour que la trace commence au clic. **Enterprise** *en partie*
- **Cluster actif/actif** sur PostgreSQL, sans affinité de session ni nœud primaire. **Enterprise**
- **E-mails transactionnels** aux couleurs de votre thème, et récapitulatif quotidien des comptes qui arrivent à expiration.
- **Déploiement** : une seule image, un stockage embarqué par défaut, Docker, Swarm ou Kubernetes avec un chart Helm, des liveness et readiness probes, une initialisation à partir d'un fichier.

## Pour vos développeurs

Tester sur le vrai cluster sans rien déployer.

Pour en savoir plus : [Mode développement](/product/dev-mode).

- **Du poste de travail au cluster** avec **softwarity/plug** : le service qui tourne sur la machine d'un développeur rejoint le cluster sous son nom, remplace le service déployé le temps de la session, puis le cluster retrouve exactement son état d'origine. Dans n'importe quel langage, sans modifier le code, depuis Linux, macOS ou Windows. `plug`
- **plug autonome, avec Community** : un conteneur agent ajouté à votre déploiement Docker, Swarm ou Kubernetes, gratuit sous licence FSL. `plug`
- **plug intégré, avec Enterprise** : l'agent vit dans la gateway, sans rien à déployer à côté ; chaque développeur s'authentifie avec sa clé SSH, et chaque page signale qu'un service est servi depuis un poste de travail. **Enterprise** *en partie*
- **Mode de test de l'interface** : naviguez sous une identité simulée pour voir exactement ce que voit un rôle.
- **Swagger UI embarqué** sur les spécifications OpenAPI de **toutes** les routes, sans CDN : les appels passent par la gateway, donc par l'authentification et les règles de la route, et l'identité utilisée par *Try it out* est **simulée** - un utilisateur, des groupes ou des rôles - pour voir ce que l'API répond à chacun.

## Pilotée par un agent IA

L'exploitant demande avec des mots, la gateway exécute et enregistre.

Pour en savoir plus : [Le point d'entrée des agents](/docs/agent/overview).

- **Serveur MCP intégré** : Claude Code, Gemini CLI et Codex CLI se connectent à la gateway pour lire, tester ou modifier les routes.
- **Connexion OAuth sans secret à recopier**, jeton à périmètre restreint (lecture seule ou complet, domaine, plages réseau), révocable en un clic.
- **Chaque action d'un agent est auditée** sous son nom, avec un point de reprise automatique pour revenir en arrière.

## Une app-gateway, et ce qui en découle

Meerkat sert **une** application composée de nombreux services : des
utilisateurs qui ont un nom, des rôles, une organisation, et une seule porte
devant l'ensemble. Cette décision explique tout le reste - pourquoi l'identité,
les rôles, les organisations et les pages de connexion sont dans le produit
plutôt qu'à côté, et pourquoi la console est un outil d'exploitant plutôt qu'un
éditeur de YAML.

Si votre besoin est d'exposer des API à des tiers - un portail pour
développeurs, une clé par partenaire, une facturation à l'appel - c'est le
travail d'une API gateway, et le dire ici vous fait gagner du temps.

> [!NOTE]
> L'anti-modèle avec lequel elle veut rompre : installer la gateway, puis
> Prometheus, puis Grafana, puis écrire du YAML pour tout. Ici, vous lancez un
> binaire, vous le configurez dans la console, vous exportez, et vous rejouez
> l'export où vous voulez.

## Quatre éditions

Community, la gratuite, est toute la gateway pour une organisation sur une
instance. Enterprise est ce dont une installation a besoin dès qu'elle
grandit : plusieurs organisations, votre annuaire d'entreprise, plusieurs
gateways derrière un même point d'entrée. Team, c'est Enterprise pour un
cluster de taille connue, et l'édition d'évaluation, c'est Enterprise avec une
mention affichée, gratuite, pour tout essayer d'abord.

La règle qui dit ce qui va de chaque côté tient en une ligne, et c'est elle
qu'il faut vérifier quand vous comparez : **jamais une brique de sécurité**.
TLS, le coffre, le second facteur, les passkeys, le journal d'audit et la
sécurité par endpoint sont dans l'image gratuite, et y resteront.

[Comparez les éditions](/product/editions), ou allez directement aux
[tarifs](/pricing/index).
