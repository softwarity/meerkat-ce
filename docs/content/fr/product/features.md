---
title: Ce qu'elle fait
section: Le produit
order: 3
summary: Toutes les capacités de la gateway, domaine par domaine, avec leur état lu dans le code.
---

# Ce qu'elle fait

Meerkat se charge de ce dont toute application a besoin et qu'aucune équipe ne
devrait écrire deux fois. Voici tout le produit, domaine par domaine.

**Enterprise** signale ce que l'image gratuite Community ne contient pas.
*en partie* signale une capacité utilisable dès aujourd'hui, dont une part,
nommée, reste à venir. Ce que coûte la même liste quand on l'assemble soi-même
est dans [le dossier Meerkat](/product/the-case).

## Connexion et identité

Les pages que voient vos utilisateurs, servies par la gateway, à vos
couleurs.

Pour en savoir plus : [Comment on se connecte](/docs/auth/overview).

- **Pages de connexion servies par la gateway** : connexion, mot de passe oublié, vérification de l'adresse e-mail, inscription en option, en 20 langues, avec votre thème, votre logo et votre image de fond. Chaque traduction peut être corrigée, ou une langue ajoutée, depuis la console.
- **D'autres dispositions de page** - split, drawer, banner, bare - et la marque Meerkat retirée des pages que vous servez. **Enterprise**
- **Des thèmes faits comme dans Material Theme Builder** : six couleurs sources, trois niveaux de contraste, son fichier JSON en entrée comme en sortie, et des polices servies par la gateway elle-même, sans rien chercher sur un CDN.
- **Mots de passe** avec une politique et un historique configurables, une protection contre la force brute commune à tous les nœuds, et des messages qui ne révèlent jamais si un compte existe.
- **Second facteur TOTP** avec QR code et codes de secours, code par e-mail en solution de repli, navigateurs de confiance.
- **Second facteur imposé** globalement, par autorité ou par utilisateur. *en partie*
- **Passkeys WebAuthn** : clé de sécurité, empreinte digitale, Windows Hello, comme premier facteur. *en partie*
- **Fédération OIDC et GitHub** : Entra ID, Okta, Google, Keycloak ou tout fournisseur d'identité conforme, pour le premier facteur.
- **SAML 2.0** pour les annuaires qui n'offrent rien d'autre (ADFS, Entra ID, Okta, Shibboleth) : assertions signées, et une assertion rejouée refusée sur tous les nœuds. **Enterprise**
- **LDAP et Active Directory**, et règles de groupe : un groupe de l'annuaire, une équipe GitHub ou un claim OIDC devient une appartenance et des rôles. **Enterprise**
- **Connexion par code reçu par e-mail** : un code à usage unique à la place du mot de passe, valable dix minutes dans le navigateur qui l'a demandé, et le second facteur s'applique toujours. Désactivée tant que vous ne l'activez pas. *en partie*
- **Jetons d'API**, personnels ou de machine à machine, dont le secret n'est affiché qu'une fois.
- **Comptes** avec une durée de validité en jours, des champs personnalisés propres à votre métier et transmis aux applications, et une alerte par e-mail lors d'une connexion depuis un nouveau navigateur.

## Accès et organisations

Qui accède à quoi se décide à la porte, jamais dans chaque application.

Pour en savoir plus : [Authentifier et autoriser](/docs/access/overview).

- **Rôles hiérarchiques** et groupes de rôles par organisation, cumulés ou choisis à la connexion.
- **Organisations clientes** isolées par la gateway, membres administrateurs ou utilisateurs, organisation choisie à la connexion. Le mode à une seule organisation est gratuit, le mode à plusieurs organisations relève d'Enterprise. **Enterprise**
- **Accès par route** selon l'organisation, le rôle et le compte ; un refus aboutit à une page qui explique, jamais à une ligne de texte.
- **Sécurité par endpoint** déduite de la spécification OpenAPI du service, et modifiée dans une console à la manière de Swagger.
- **Administration déléguée, aux frontières nettes** : super-administrateur, administrateur de l'infrastructure, administrateur des applications, administrateur d'organisation, et libre-service pour chaque utilisateur.
- **Plages horaires d'accès** par organisation, selon le jour, la date et le fuseau horaire. **Enterprise** *en partie*
- **Identité transmise à vos services** sous forme d'en-têtes, de REMOTE_USER ou d'un JWT signé (ES256, EdDSA, RS256), avec des clés publiées qui changent sans interruption ; vous choisissez les rôles que reçoit chaque service.

## Appels planifiés

La gateway appelle vos services à l'heure que vous indiquez, sans serveur de
messages à installer.

Pour en savoir plus : [Appels planifiés](/docs/operations/scheduler).

- **Une planification, c'est une route, un chemin et une cadence** : un intervalle, un calendrier cron dans un fuseau horaire, ou une date unique pour une action différée. L'appel passe par la porte d'entrée : toutes les règles placées devant le service s'y appliquent.
- **Aucun compte de service à créer** : l'appel porte les rôles que demande la planification, et atteint exactement ce qu'atteignent ces rôles. Un service peut créer et gérer ses propres planifications par l'API.
- **Fiable** : chaque appel est passé au moins une fois, avec un identifiant pour repérer les doublons, même si une gateway s'arrête en plein appel. Un long traitement rend compte de son avancement au lieu de garder une requête ouverte pendant des heures.
- **Patient avec un service qui n'est pas prêt** : les réponses qui signifient d'habitude "un peu trop tôt" sont retentées, et un service peut indiquer quand le rappeler.
- **Rien à installer** : ni serveur de messages, ni verrou ; en cluster, les appels se répartissent entre les nœuds.
- **Un écran en direct dans la console** : suspendre, lancer tout de suite, supprimer, et l'historique de chaque exécution - quand, comment elle s'est terminée, ce qui a répondu - avec une exécution en échec relancée en un clic.

## Routage et protection du trafic

Une API gateway complète, pilotée depuis la console.

Pour en savoir plus : [Les routes](/docs/concepts/routes), et ce que coûte une
requête dans [Performance](/product/performance).

- **Routes modifiées à chaud**, sans redémarrage, et propagées à tous les nœuds en moins d'une seconde.
- **11 prédicats et 34 filtres** : chemin, hôte, en-tête, cookie, méthode, poids pour les déploiements canari, fenêtre horaire ; réécriture de la requête et de la réponse.
- **Limites de débit** par route, utilisateur, jeton, organisation ou adresse, avec plusieurs limites à la fois ; **quotas par endpoint** ; réponse 429 standard. En cluster, chaque nœud compte encore de son côté. *en partie*
- **Disjoncteur, timeouts** à trois niveaux, et santé des services dans la console : un cœur par route, d'après la découverte ou un test TCP, et d'après le trafic réel.
- **WebSocket, gRPC et diffusion des corps en flux** de bout en bout ; un appel gRPC est compté d'après son `grpc-status`, et non d'après le 200 qui le transporte. *en partie*
- **Découverte des services** pour Docker, Swarm et Kubernetes, à la création d'une route. *en partie*
- **Testeur de routage** : composez une requête d'exemple et voyez quelle route la prend, et pourquoi.
- **Page de maintenance** par route, ou pour toute la plateforme en un seul geste, traduite, avec une porte réservée aux administrateurs.
- **Réponse depuis un gabarit** : une route peut répondre elle-même un contenu construit à partir de l'utilisateur connecté, au format qu'attend une application - son contrat d'identité se configure au lieu de se coder.
- **Des fichiers servis par une route** : téléversez une police, une feuille de style, un script ou une image, et la route y répond sous son chemin, avec son type, son ETag et CORS - pour l'interface qui a besoin d'une ressource que rien derrière la gateway ne sert, hors ligne avant tout.

## Dans vos applications, sans y toucher

Ce que la gateway ajoute aux pages qu'elle sert.

Pour en savoir plus : [Ce que la gateway injecte](/docs/concepts/data-plane-chrome).

- **Bouton utilisateur injecté** : profil, déconnexion, changement d'organisation, langue, mode clair ou sombre.
- **Portail de navigation** entre vos applications, en barre ou en rail, qui ne montre que ce qu'autorisent les droits d'accès.
- **Masquage de l'interface selon le rôle**, en CSS pur : les rôles sont inscrits dans la page côté serveur.
- **Votre propre CSS et JavaScript, route par route**, écrits dans la console ou téléversés en fichiers, placés en début ou en fin de head ou en fin de body, dans l'ordre que vous choisissez.
- **Signalement d'un problème** : capture d'écran, console du navigateur, contexte technique, avec un suivi dans la console d'administration.
- **Canal temps réel** en WebSocket vers les applications, sans intégration préalable.
- **Rien de personnel dans les caches** : toute page qui porte une identité est rendue impossible à mettre en cache, quoi qu'en dise l'application.

## Sécurité de la plateforme

Les briques que l'on installe d'habitude à côté.

Pour en savoir plus : [Le coffre](/docs/operations/vault).

- **Certificats TLS** dans une seule réserve - générés, importés ou signés sur demande, chacun avec ses noms, un joker ou une IP - placés par glisser-déposer sur la console ou les applications, avec des ports HTTPS ouverts à chaud.
- **Certificats automatiques** par ACME : Let's Encrypt, ZeroSSL, Google ou votre propre step-ca les émettent et les renouvellent, avec plusieurs autorités côte à côte. **Enterprise**
- **Coffre intégré** : secrets chiffrés en AES-256-GCM, référencés par leur nom dans la configuration, export chiffré, rappel avant expiration. La même clé scelle les clés TLS et les secrets TOTP, et se renouvelle par un redémarrage.
- **Redirection HTTPS et en-têtes de sécurité** : le HTTP en clair renvoyé vers HTTPS, HSTS, CSP, X-Frame-Options, Referrer-Policy, et protection CSRF pour la console.
- **Console sur un port distinct** de celui du trafic applicatif : l'administration n'est jamais exposée avec l'application.
- **Fonctionne sans accès à internet** : aucune ressource n'est chargée depuis l'extérieur, ce qui convient aux environnements isolés du réseau.

## Exploitation

Une console qui remplace les fichiers YAML et les pipelines de configuration.

Pour en savoir plus : [Exploitation](/docs/operations/overview).

- **Configurations versionnées** : versions nommées, une seule active, comparaison, export et import en YAML, point de reprise automatique à chaque changement. Trois à la fois en Community, autant que vous voulez en **Enterprise**.
- **Aucune modification perdue** : deux administrateurs qui modifient la même chose en même temps en sont avertis, au lieu que l'un efface l'autre sans le savoir.
- **Console vivante** : une modification faite par un administrateur apparaît sur les écrans des autres sans rechargement. *en partie*
- **Configurations dans un dépôt git** : un répertoire par plateforme dans un dépôt partagé ; un pull range la configuration sans rien appliquer tant que vous ne l'activez pas, un push la committe au nom de l'exploitant qui a cliqué. GitHub, GitLab, Bitbucket, Azure DevOps, Gitea ou votre propre serveur, avec un jeton rangé dans le coffre. **Enterprise**
- **Journal d'audit** de chaque action d'administration, avec les différences champ par champ, et de la sécurité des comptes - chaque connexion, chaque connexion refusée avec sa raison réelle et son adresse, chaque facteur, passkey, mot de passe ou jeton modifié par son propriétaire. En ajout seul, consultable dans la console, conservé de trois mois à cinq ans selon votre choix. *en partie*
- **Audit envoyé au collecteur** en OTLP : les événements de sécurité du plan de données, ainsi que les changements faits dans la console. **Enterprise**
- **Audit des endpoints** : un interrupteur par opération de la spécification OpenAPI d'une route, et chaque appel devient un événement d'audit. **Enterprise** *en partie*
- **Export CSV** du journal d'audit. Le format Parquet reste à livrer. **Enterprise** *en partie*
- **Tableaux de bord intégrés** : trafic, latence et échecs par route et par endpoint, sans rien installer.
- **Métriques envoyées en OTLP** au collecteur qui reçoit déjà les traces, et qui les écrit dans Prometheus - avec un tableau de bord Grafana prêt à l'emploi. **Enterprise**
- **Journaux structurés**, en JSON ou en texte, et un **journal d'accès** : une ligne par requête, refus compris, avec le compte que la gateway a authentifié et le jeton d'API quand l'appel vient d'une machine. C'est la moitié d'un audit qu'aucun service ne peut écrire, puisqu'un service ne voit jamais les appels refusés avant lui. Les deux journaux sont écrits en JSON OpenTelemetry pour votre agent, ou envoyés au collecteur (**Enterprise**). La console les affiche en direct et règle le niveau de journalisation sur tous les nœuds.
- **Traces distribuées** (W3C Trace Context) : dans toutes les éditions, chaque requête reçoit un identifiant - renvoyé à l'appelant, écrit dans le journal, affiché au pied des pages intégrées - qui **relie une ligne du journal de la gateway aux propres enregistrements de votre service**. En Enterprise, la gateway apparaît aussi dans la trace, avec son propre temps mesuré, exporte en **OTLP** vers tout collecteur OpenTelemetry, et peut faire commencer la trace dans le navigateur, au clic. **Enterprise** *en partie*
- **Cluster actif/actif** sur PostgreSQL, sans affinité de session ni nœud primaire. **Enterprise**
- **E-mails transactionnels** aux couleurs de votre thème, et récapitulatif quotidien des comptes qui arrivent à expiration.
- **Déploiement** : une seule image, un stockage embarqué par défaut, Docker, Swarm ou Kubernetes avec un chart Helm, des sondes de vie et de disponibilité, une initialisation à partir d'un fichier.
- **Images signées** : chaque image est signée avec cosign à sa construction, pour que vous puissiez vérifier d'où elle vient. *en partie*
- **Copie et déplacement de la base** : une copie cohérente de la base, téléchargée depuis la console ; la déplacer dans un serveur PostgreSQL, pour passer en cluster, relève d'**Enterprise**.

## Pour vos développeurs

Tester sur le vrai cluster sans rien déployer.

Pour en savoir plus : [Mode développement](/product/dev-mode).

- **Du poste de travail au cluster** avec **softwarity/plug** : le service qui tourne sur la machine d'un développeur prend la place du service déployé le temps de la session, puis le cluster retrouve exactement son état d'origine. Dans n'importe quel langage, sans modifier le code, depuis Linux, macOS ou Windows. `plug`
- **plug autonome, avec Community** : plug est gratuit (licence FSL) et tourne seul, comme conteneur agent dans votre déploiement Docker, Swarm ou Kubernetes, à côté de la gateway. Meerkat n'en sait rien : ni clés de développeur, ni noms, ni annonce. `plug`
- **plug intégré à Meerkat, avec Enterprise** : le tunnel vit dans la gateway, sans rien à déployer à côté ; chaque développeur s'authentifie avec sa clé SSH, et chaque page signale qu'un service est servi depuis un poste de travail. **Enterprise** *en partie*
- **Mode de test de l'interface** : naviguez sous une identité simulée pour voir exactement ce que voit un rôle.
- **Swagger UI intégré** pour l'API de **chaque** route : les appels passent par la gateway et ses règles, et *Try it out* peut agir sous n'importe quel utilisateur, groupe ou rôle, pour voir ce que l'API répond à chacun.

## Pilotée par un agent IA

L'exploitant demande avec des mots, la gateway exécute et enregistre.

Pour en savoir plus : [Le point d'entrée des agents](/docs/agent/overview).

- **Serveur MCP intégré** : Claude Code, Gemini CLI, Kimi CLI et Codex CLI se connectent à la gateway pour lire, tester ou modifier les routes, les rôles et les planifications, importer une configuration entière, et appeler vos services à travers une route sous n'importe quel rôle.
- **Connexion OAuth sans secret à recopier**, jeton à périmètre restreint (planifications seules, lecture seule ou complet, et les plages réseau d'où il peut venir), révocable en un clic.
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
