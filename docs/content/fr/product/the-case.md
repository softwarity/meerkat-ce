---
title: Le dossier
section: Le produit
order: 4
summary: Ce que coûte le socle de toute application professionnelle quand on l'assemble, quand on l'achète sous forme de service, et quand il est déjà là.
printable: true
---

# Le dossier Meerkat

Avant de montrer son premier écran métier, une application doit faire tout ce
qu'un acheteur attend d'un logiciel professionnel. Rien de tout cela ne
distingue le produit, tout est vérifié par les équipes achats et sécurité, et
chaque élément coûte du temps à construire, puis à maintenir.

::: lead
Cette page fait le calcul de ce socle : ce que vous assemblez sans Meerkat, et
ce que cela coûte dans le cluster, en licences et en jours d'ingénierie. Les
chiffres sont sourcés et datés, et quand un nombre est une estimation, c'est
dit.
:::

## Le socle attendu en 2026

Voici la liste, telle qu'elle apparaît dans les questionnaires de sécurité et
les appels d'offres.

| | |
| --- | --- |
| **Une connexion soignée** | des pages à votre marque, traduites, en clair et en sombre |
| **Second facteur et passkeys** | TOTP, codes de secours, WebAuthn |
| **Le SSO des clients** | Entra ID, Okta, Google, LDAP, Active Directory |
| **Plusieurs organisations** | clients isolés, membres, groupes, administrateurs délégués |
| **Rôles et accès fins** | par écran, par route, par endpoint d'API |
| **Jetons d'API** | pour les intégrations et les machines |
| **Protection du trafic** | rate limits, quotas, disjoncteur, timeouts |
| **TLS automatique** | certificats émis et renouvelés sans intervention |
| **Secrets à l'abri** | chiffrés au repos, jamais dans la configuration |
| **Journal d'audit** | qui a changé quoi, avec l'avant et l'après |
| **Observabilité** | trafic, latence, erreurs, par route et par endpoint |
| **Haute disponibilité** | plusieurs nœuds, aucune session perdue |
| **Maintenance annoncée** | une vraie page à la place d'une erreur 502 |
| **Traitements planifiés** | clôtures, relances, purges, le rapport du matin |
| **Retours des utilisateurs** | signaler un problème avec une capture d'écran et le contexte |
| **Outillage des développeurs** | tester sur le cluster depuis sa propre machine |
| **Documentation des API** | un seul Swagger pour toutes les API, appelées à travers la gateway, sous l'identité de votre choix |

Ce que Meerkat en couvre, ligne par ligne et avec l'état lu dans le code, se
trouve sur la page [Ce qu'elle fait](/product/features).

## Ce que vous assemblez sans elle

Pour chaque besoin, le produit que l'on choisit d'habitude - en version
gratuite auto-hébergée, ou en offre commerciale.

| Besoin | Gratuit, auto-hébergé | Commercial ou SaaS | Meerkat |
| --- | --- | --- | --- |
| Connexion, MFA, passkeys | Keycloak, Zitadel, Authentik | Auth0, WorkOS, Clerk, FusionAuth | inclus |
| Pages à votre marque, 20 langues | un thème à construire | selon le palier | inclus |
| SSO des clients : OIDC, LDAP, AD | Keycloak | Auth0, WorkOS, facturé à la connexion | inclus, Enterprise pour LDAP |
| Organisations gérées par vos clients | des écrans d'administration à construire | WorkOS, Frontegg | inclus, Enterprise |
| Protéger une application dépourvue d'authentification | oauth2-proxy, Pomerium Core | Pomerium Enterprise, Cloudflare Access | inclus |
| Routage, rate limits, quotas, disjoncteur | Kong OSS, APISIX, Traefik, Envoy Gateway | Kong Enterprise, Tyk, Traefik Hub, API7, Gravitee | inclus |
| Certificats TLS automatiques | cert-manager | dans les paliers supérieurs des gateways | inclus |
| Coffre à secrets | Vault Community, OpenBao | HCP Vault, Infisical, Doppler | inclus |
| Tableaux de bord du trafic | Prometheus et Grafana | Grafana Cloud | inclus |
| Audit de chaque action d'administration | Retraced, puis raccorder chaque outil | WorkOS Audit Logs, puis raccorder chaque outil | inclus |
| Menu utilisateur et portail dans vos applications | **à construire** | **aucun produit standard** | inclus |
| Signalement d'un problème avec capture d'écran | Sentry auto-hébergé | Marker.io, Jam, Userback, BugHerd | inclus |
| Du poste du développeur au cluster | plug, mirrord OSS, Telepresence, Gefyra | mirrord Team, Okteto | intégré, Enterprise |
| Configurations versionnées, retour arrière | un pipeline GitOps à mettre en place | un pipeline GitOps à mettre en place | inclus |
| Appels planifiés vers vos services | cron, CronJob Kubernetes, ou Quartz et Celery dans chaque service | Temporal Cloud, Inngest, Trigger.dev, EventBridge Scheduler | inclus |
| Pilotage par un agent IA | serveurs MCP communautaires | Kong Konnect, SaaS uniquement | inclus |

Deux de ces lignes n'ont aucun produit en face : le menu utilisateur et le
portail à l'intérieur de vos propres applications, et un audit transversal qui
couvre tous les outils à la fois. Ces deux-là, vous les écrivez.

> [!WARNING]
> Trois faits, relevés en septembre 2026, pèsent sur la voie gratuite. Kong
> présente la 3.9 comme sa dernière version entièrement gratuite : la branche
> gratuite ne reçoit plus que des correctifs 3.9.x alors qu'Enterprise en est à
> la 3.16, et le mode gratuit de l'image Enterprise a été supprimé en 3.10.
> Vault Community est sous licence BSL, qui interdit les offres concurrentes.
> Le mode hors ligne de Traefik Hub est une option sous licence dont le jeton
> expire au bout d'un an ; la gateway s'arrête alors après un délai de grâce
> de 30 jours.

## Ce que cela coûte dans le cluster

Chaque produit ajouté au cluster arrive avec ses pods, sa base de données, ses
sauvegardes, ses mises à niveau et ses avis de sécurité. Voici la même porte
d'entrée, construite des deux façons, avec le nombre d'instances que la
documentation de chaque produit recommande pour la production.

| Brique | Produit | Instances en production | État à exploiter | Mémoire recommandée |
| --- | --- | --- | --- | --- |
| Identité | Keycloak | 3 pods, l'exemple de la documentation | PostgreSQL | 1 250 Mo par pod |
| Gateway d'API | Kong OSS, sans base de données | 3 nœuds | Redis, pour les rate limits partagés | 2 à 4 Go par nœud |
| Proxy d'authentification | oauth2-proxy | 2 pods | Redis, pour les sessions | non publiée |
| Certificats | cert-manager | 7 pods, la pratique recommandée | - | non publiée |
| Coffre à secrets | OpenBao ou Vault | 5 nœuds Raft | Raft, et les parts de la clé de descellement | 8 à 16 Go par nœud |
| Supervision | kube-prometheus-stack | 5 pods, plus 1 par nœud | sa propre base de séries temporelles | 512 Mo au minimum pour Grafana |
| Audit | Retraced | 5 services | PostgreSQL, Elasticsearch et NSQ | non publiée |
| Du poste au cluster | mirrord Operator | 1 pod, plus un job par session | une licence Team est exigée | non publiée |
| **Tout le socle** | **Meerkat** | **1 pod, ou 3 en cluster** | **base de données embarquée, ou PostgreSQL** | **{{memory.idle}} Mo au repos, {{memory.peak}} en charge** |

::: figure stack
La même porte d'entrée, des deux façons. À gauche, une requête traverse deux
produits avant d'atteindre vos services, et six autres tournent à côté ; à
droite, un seul.
:::

Cela représente **environ 38 pods et cinq moteurs de stockage** à installer, à
sécuriser, à mettre à niveau et à sauvegarder - et une requête traverse deux de
ces produits avant d'atteindre vos services.

L'autre façon : **un pod, {{memory.idle}} Mo au repos et {{memory.peak}} en pleine charge**, mesurés par la CI sur un runner x64.
Trois pods et un PostgreSQL quand vous voulez la
[haute disponibilité](/docs/deploy/kubernetes).

## Et elle tient la charge

Un produit au lieu de huit n'est une bonne nouvelle que si ce produit n'est pas
le plus lent. Cela se mesure au lieu de s'affirmer : Meerkat tourne à côté de
Kong, APISIX et Traefik, chacun limité au même CPU unique, devant le même
service, sous la même charge, dans la même exécution - le tableau compare donc
des produits, et non des machines. La CI le recalcule à chaque commit et le
site le lit en direct, ce qui explique qu'aucun chiffre ne soit écrit ici :
[les mesures](/product/performance).

## Ce que cela coûte en licences

Les briques gratuites ne coûtent rien en licences ; leur prix, c'est le cluster
décrit plus haut et le temps décrit plus bas. Les offres commerciales, elles,
affichent un prix. Pour les comparer, prenons un scénario - un éditeur de
logiciels dont les clients sont des entreprises, le cas où le socle pèse le
plus lourd :

- 5 000 utilisateurs actifs par mois
- 20 organisations clientes, dont 10 avec leur propre SSO
- 15 services, en production et en préproduction
- 50 millions de requêtes par mois
- une équipe de 10 développeurs

| Brique | Offre adaptée la moins chère | Offre courante | Meerkat |
| --- | --- | --- | --- |
| Identité, SSO, MFA | Descope Pro, 499 $ | Auth0 Essentials, 2 000 $ | inclus |
| Gateway d'API | API7 Cloud, 750 $ | Gravitee Planet, 2 500 $ | inclus |
| Coffre à secrets | Infisical Pro, 200 $ | HCP Vault Essentials, 1 881 $ | inclus |
| Supervision | Grafana Cloud Pro, 100 $ | Grafana Cloud Pro, 100 $ | inclus |
| Audit des actions d'administration | WorkOS Audit Logs, 224 $ | WorkOS Audit Logs, 224 $ | inclus |
| Signalement d'un problème | Userback Business, 79 $ | Marker.io Team, 149 $ | inclus |
| Du poste au cluster | mirrord Team annuel, 400 $ | mirrord Team mensuel, 500 $ | plug : intégré |
| Menu et portail dans vos applications | aucun produit | aucun produit | inclus |
| **Par mois** | **2 252 $** | **7 354 $** | Community : gratuite |
| **Par an** | **27 024 $** | **88 248 $** | Team, Enterprise : [sur demande](/pricing/index) |

Prix publics en dollars américains, hors taxes, hors machines et hors temps
d'intégration. Les produits qui n'affichent pas de prix public pour ce scénario
sont écartés plutôt que devinés : Kong Konnect Plus plafonne à 10 millions de
requêtes, le scénario passe donc sur devis, et Tyk, Kong Enterprise, Pomerium
Enterprise et Frontegg au-delà de cinq connexions ne se vendent que sur devis.
Pour donner un ordre de grandeur, Traefik Hub se vend de 30 000 à 50 000 $ par
an sur l'AWS Marketplace, et les offres facturées à l'utilisateur, comme
Cloudflare Access ou Pomerium Zero à 7 $, atteindraient 35 000 $ par mois pour
5 000 utilisateurs.

## Chaque client qui apporte son propre SSO

Les offres d'identité facturent les connexions SSO client par client : le socle
coûte donc plus cher à chaque contrat signé avec une grande entreprise -
précisément au moment où vous gagnez des clients.

::: figure sso-per-customer
Ce qu'ajoute, chaque mois, un client de plus qui a son propre SSO.
:::

Les quotas inclus avant que ce compteur ne démarre : **cinq connexions chez
Stytch et Descope, trois avec Auth0 Essentials, une chez Clerk, aucune chez
WorkOS**. Au-delà de ce seuil, chaque client qui arrive avec son Entra ID ou
son Okta est facturé, et le montant n'a rien à voir avec ce qu'il consomme.

## Ce que cela coûte en temps

Le coût le plus lourd d'un socle assemblé n'est pas la licence, c'est
l'ingénierie. Voici une estimation, en jours-homme, de ce qu'il faut pour
atteindre le même périmètre, lot par lot, de l'hypothèse basse à l'hypothèse
haute. Elle ne compte pas l'adaptation de vos propres services pour qu'ils
lisent l'identité transmise, nécessaire dans tous les cas.

::: figure person-days
Jours-homme nécessaires pour atteindre le même périmètre, de l'estimation basse
à l'estimation haute.
:::

| Lot | Assemblage gratuit | Assemblage SaaS | Meerkat |
| --- | --- | --- | --- |
| Identité : installation, MFA, passkeys, fédération | 10 à 20 | 5 à 10 | 0,5 à 1 |
| Pages de connexion à votre marque, traduites | 5 à 10 | 2 à 4 | 0,5 à 1 |
| Organisations clientes et leur administration | 15 à 30 | 5 à 15 | 0,5 à 1 |
| Proxy d'authentification, identité transmise aux services | 3 à 6 | 3 à 6 | 0,5 à 1 |
| Gateway d'API : routes, rate limits, quotas, disjoncteur | 8 à 15 | 5 à 10 | 1 à 2 |
| Certificats et coffre à secrets | 6 à 13 | 3 à 6 | 0,5 à 1 |
| Supervision et tableaux de bord | 4 à 8 | 2 à 4 | 0 à 0,5 |
| Audit transversal des actions d'administration | 8 à 15 | 5 à 10 | 0 |
| Menu utilisateur et portail dans les applications | 8 à 15 | 8 à 15 | 0,5 à 1 |
| Signalement d'un problème | 1 à 3 | 1 à 2 | 0 à 0,5 |
| La machine d'un développeur dans le cluster | 2 à 5 | 1 à 3 | 0,5 à 1 |
| Configurations versionnées, retour arrière | 5 à 10 | 3 à 6 | 0 |
| Recette de bout en bout | 8 à 15 | 5 à 10 | 1 à 2 |
| **Mise en place** | **83 à 165 jours** | **48 à 101 jours** | **5,5 à 12 jours** |
| À 650 € HT par jour | 54 à 107 k€ | 31 à 66 k€ | 3,6 à 7,8 k€ |
| Maintenance, en jours-homme par an | 20 à 40 | 10 à 20 | 2 à 5 |

L'assemblage SaaS évite d'installer des serveurs, mais il reste l'intégration,
le raccordement des produits entre eux, et le développement qu'aucun produit ne
couvre.

## D'où viennent ces chiffres

Un chiffre sans sa méthode ne vaut rien : voici donc la méthode, et les sources
avec elle.

**Ce qui est mesuré.** La mémoire et le débit proviennent du banc de mesure de
la CI du projet (`tools/bench`), exécuté pour la dernière fois le
**{{memory.date.fr}}** sur les runners GitHub x64 et arm64, la mémoire du
conteneur étant relevée au repos **et en charge**. L'état de chaque
fonctionnalité est lu dans le code et non dans un plan :
[l'inventaire public du dépôt](/product/features) compte aujourd'hui
{{features.built}} fonctionnalités livrées, {{features.partial}} livrées en partie et {{features.todo}} à venir.

**Ce qui est relevé.** Les prix sont les prix PUBLICS, en dollars américains
hors taxes, lus sur les pages officielles le **16 septembre 2026**, sans
remise négociée - un acheteur qui négocie paiera moins, et c'est précisément
pour cela que la comparaison se fait au prix catalogue. Le nombre de pods et la
mémoire recommandée sont ceux que la documentation de chaque produit donne pour
la production.

**Ce qui est estimé.** Les jours-homme sont une estimation de Softwarity,
donnée sous forme de fourchette parce que c'en est une. La conversion en euros
se fait à **650 € HT par jour**, pour un ingénieur DevOps ou sécurité
expérimenté.

::: details Les sources, une par une
**Identité**

- auth0.com/pricing
- workos.com/pricing
- clerk.com/pricing
- descope.com/pricing
- stytch.com/pricing
- keycloak.org, dimensionnement de la mémoire et du CPU

**Gateways et proxys**

- konghq.com/pricing, et les recommandations de dimensionnement de Kong
- Kong : quelles versions sont encore entièrement open source
- api7.ai/pricing
- gravitee.io/pricing
- Traefik Hub sur l'AWS Marketplace, et son mode hors ligne
- pomerium.com/pricing et Cloudflare Zero Trust

**Cluster, coffre, supervision, audit, outillage**

- cert-manager, ses bonnes pratiques de déploiement
- Vault, l'architecture de référence Raft, et OpenBao, le stockage intégré
- HCP Vault, Infisical, Doppler
- grafana.com/pricing, et le chart kube-prometheus-stack
- Retraced
- softwarity/plug et mirrord
- Userback, Marker.io, Jam
:::

Les prix bougent. Si une ligne de cette page est périmée, c'est la ligne qui a
tort : dites-le-nous et nous la corrigerons.

## Ce que vous obtenez à la place

Ce qui ne bouge pas, c'est la forme de l'argument : **un produit au lieu de
huit, un processus au lieu de trente-huit pods, un socle déjà là au lieu d'un
socle à assembler.**

Pour l'équipe qui vivra avec, cela veut dire :

- **Une seule chose à exploiter.** Une image, une console, un journal d'audit,
  une sauvegarde. Pas huit produits à tenir à jour, chacun avec ses bulletins
  de sécurité et son rythme de publication.
- **Un socle qui passe les questionnaires dès le premier jour.** Second
  facteur, passkeys, SSO des clients, audit, TLS, coffre : tout est dans
  l'édition gratuite, et répondre à un appel d'offres cesse d'être un projet.
- **Vos développeurs rendus à votre produit.** Cinq à douze jours de mise en
  place au lieu de quatre-vingt-trois à cent soixante-cinq - et, surtout, au
  lieu des vingt à quarante jours-homme que l'assemblage réclame de nouveau
  chaque année.
- **Le droit de changer d'avis.** L'édition Community est complète et le
  restera ; le code passe sous licence Apache 2.0 au bout de deux ans. Vous ne
  pariez pas votre porte d'entrée sur notre survie.

Ce qui est réellement construit, et ce qui ne l'est pas, tient dans
[un tableau lu dans le code](/project/roadmap) - la même transparence que pour
les chiffres ci-dessus. Ce que contient chaque édition est décrit sur la page
[Éditions](/product/editions), et si vous préférez voir avant de décider, la
[galerie](/showcase/index) montre les écrans.
