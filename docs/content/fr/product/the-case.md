---
title: Le dossier
section: Le produit
order: 4
summary: Ce que coûte le socle d'une application professionnelle quand on l'assemble, quand on le loue, et quand Meerkat le fournit déjà.
printable: true
---

# Le dossier Meerkat

Toute application professionnelle a besoin du même socle avant même son premier
écran métier : une connexion soignée, un second facteur, le SSO de vos clients,
des rôles, un journal d'audit, des certificats, de la supervision. Vos acheteurs
vérifient tout cela. Rien de tout cela ne distingue votre produit. Et tout cela
prend du temps à construire, puis à maintenir.

::: lead
Meerkat est ce socle, en un seul produit. Cette page chiffre ce qu'il vous
épargne : en serveurs, en licences et en jours d'ingénierie. Chaque chiffre est
sourcé et daté, et les estimations sont présentées comme telles.
:::

**En bref :**

- **Un produit au lieu de huit**, et un pod au lieu d'environ 38.
- **5,5 à 12 jours** de mise en place, au lieu de 83 à 165.
- **27 000 à 88 000 $ par an** : les licences de l'équivalent assemblé à partir
  de services payants.
- **Gratuit et complet** dans son édition Community.

## Ce qu'attendent les acheteurs en 2026

Voici la liste, telle qu'on la trouve dans les questionnaires de sécurité et les
appels d'offres.

| | |
| --- | --- |
| **Une connexion soignée** | des pages à votre marque, traduites, en clair et en sombre |
| **Second facteur et passkeys** | application d'authentification, codes de secours, clés de sécurité |
| **Le SSO de vos clients** | Entra ID, Okta, Google, SAML, LDAP, Active Directory |
| **Plusieurs organisations** | chaque client isolé, avec ses membres, ses groupes et ses administrateurs |
| **Rôles et accès fins** | par écran, par application, par endpoint d'API |
| **Jetons d'API** | pour les intégrations et les machines |
| **Protection du trafic** | limites de débit, quotas, délais, coupe-circuit pour les services en panne |
| **HTTPS automatique** | des certificats émis et renouvelés tout seuls |
| **Secrets à l'abri** | chiffrés, jamais écrits dans la configuration |
| **Journal d'audit** | qui a changé quoi, avant et après |
| **Supervision** | trafic, temps de réponse et erreurs, par application et par endpoint |
| **Haute disponibilité** | plusieurs serveurs, aucune session perdue |
| **Maintenance annoncée** | une vraie page à la place d'une erreur |
| **Tâches planifiées** | clôtures, relances, purges, le rapport du matin |
| **Retours des utilisateurs** | signaler un problème avec une capture d'écran et son contexte |
| **Outils pour les développeurs** | tester sur la vraie plateforme depuis sa propre machine |
| **Documentation des API** | un seul Swagger pour toutes les API, essayées en direct à travers la gateway, sous l'identité de votre choix |

Ce que Meerkat en couvre, ligne par ligne, est sur la page
[Ce qu'elle fait](/product/features).

## Ce que vous assemblez sans elle

Pour chaque besoin, le produit que les équipes choisissent d'habitude : un
produit gratuit qu'elles hébergent elles-mêmes, ou un service payant.

| Besoin | Gratuit, auto-hébergé | Payant ou SaaS | Meerkat |
| --- | --- | --- | --- |
| Connexion, second facteur, passkeys | Keycloak, Zitadel, Authentik | Auth0, WorkOS, Clerk, FusionAuth | inclus |
| Pages à votre marque, 20 langues | un thème à construire | selon l'offre | inclus |
| SSO des clients : OIDC, SAML, LDAP, AD | Keycloak | Auth0, WorkOS, facturé à la connexion | inclus, Enterprise pour SAML et LDAP |
| Organisations gérées par vos clients | des écrans d'administration à construire | WorkOS, Frontegg | inclus, Enterprise |
| Protéger une application sans connexion | oauth2-proxy, Pomerium Core | Pomerium Enterprise, Cloudflare Access | inclus |
| Routage, limites de débit, quotas | Kong OSS, APISIX, Traefik, Envoy Gateway | Kong Enterprise, Tyk, Traefik Hub, API7, Gravitee | inclus |
| Certificats HTTPS automatiques | cert-manager | dans les offres haut de gamme des gateways | inclus, Enterprise |
| Coffre à secrets | Vault Community, OpenBao | HCP Vault, Infisical, Doppler | inclus |
| Tableaux de bord du trafic | Prometheus et Grafana | Grafana Cloud | inclus |
| Audit de chaque action d'administration | Retraced, puis y raccorder chaque outil | WorkOS Audit Logs, puis y raccorder chaque outil | inclus |
| Menu utilisateur et portail dans vos applications | **à construire** | **aucun produit n'existe** | inclus |
| Signalement d'un problème avec capture d'écran | Sentry auto-hébergé | Marker.io, Jam, Userback, BugHerd | inclus |
| Brancher le poste d'un développeur sur la plateforme | plug, mirrord OSS, Telepresence, Gefyra | mirrord Team, Okteto | Enterprise (plug seul reste gratuit) |
| Configurations enregistrées, retour arrière | un pipeline GitOps à mettre en place | un pipeline GitOps à mettre en place | inclus |
| Appels planifiés vers vos services | des tâches cron, ou un planificateur dans chaque service | Temporal Cloud, Inngest, Trigger.dev, EventBridge Scheduler | inclus |
| Pilotage par un agent IA | serveurs MCP communautaires | Kong Konnect, en SaaS uniquement | inclus |

Deux besoins n'ont aucun produit en face : un menu utilisateur et un portail à
l'intérieur de vos propres applications, et un journal d'audit unique pour tous
les outils. Sans Meerkat, ces deux-là, vous les écrivez vous-même.

> [!WARNING]
> La voie gratuite se rétrécit (constaté en septembre 2026) :
>
> - **Kong** présente la 3.9 comme sa dernière version entièrement gratuite. La
>   branche gratuite ne reçoit plus que des correctifs 3.9, alors qu'Enterprise
>   en est à la 3.16, et l'image Enterprise a perdu son mode gratuit en 3.10.
> - **Vault Community** est passé sous licence BSL, qui interdit les offres
>   concurrentes.
> - **Traefik Hub** hors ligne exige une licence qui expire au bout d'un an ;
>   la gateway s'arrête 30 jours plus tard.

## Ce que cela coûte à faire tourner

Chaque produit ajouté arrive avec ses serveurs, sa base de données, ses
sauvegardes, ses mises à jour et ses alertes de sécurité. Voici la même porte
d'entrée construite des deux façons, avec le nombre de pods que la documentation
de chaque produit recommande en production.

| Brique | Produit | Pods en production | Demande aussi | Mémoire recommandée |
| --- | --- | --- | --- | --- |
| Identité | Keycloak | 3, l'exemple de sa documentation | PostgreSQL | 1 250 Mo par pod |
| Gateway d'API | Kong OSS, dans sa configuration la plus légère (sans base) | 3 | Redis, pour partager les limites de débit | 2 à 4 Go par pod |
| Proxy de connexion | oauth2-proxy | 2 | Redis, pour les sessions | non publiée |
| Certificats | cert-manager | 7, la pratique recommandée | - | non publiée |
| Coffre à secrets | OpenBao ou Vault | 5 | son propre stockage, et les clés pour le déverrouiller | 8 à 16 Go par pod |
| Supervision | Prometheus et Grafana (kube-prometheus-stack) | 5, plus 1 par serveur | sa propre base de séries temporelles | 512 Mo au minimum pour Grafana |
| Audit | Retraced | 5 | PostgreSQL, Elasticsearch et NSQ | non publiée |
| Tunnel des développeurs | mirrord Operator | 1, plus un par session | une licence Team payante | non publiée |
| **Tout ce qui précède** | **Meerkat** | **1, ou 3 pour la haute disponibilité** | **rien, ou PostgreSQL** | **{{memory.idle}} Mo au repos, {{memory.peak}} en charge** |

::: figure stack
La même porte d'entrée, des deux façons. À gauche, une requête traverse deux
produits avant d'atteindre vos services, et six autres tournent à côté. À
droite, un seul.
:::

Assemblé, cela fait **environ 38 pods et cinq systèmes de stockage** à installer,
sécuriser, mettre à jour et sauvegarder.

Avec Meerkat : **un pod, {{memory.idle}} Mo au repos et {{memory.peak}} en pleine
charge**. Trois pods et un PostgreSQL quand vous voulez la
[haute disponibilité](/docs/deploy/kubernetes).

## Et elle est rapide

Un produit au lieu de huit n'est une bonne nouvelle que s'il n'est pas le plus
lent. Nous le mesurons donc face à Kong, APISIX et Traefik : chacun limité au
même CPU unique, devant le même service, sous la même charge, dans la même
exécution. Le tableau compare des produits, pas des machines. Les résultats sont
recalculés à chaque modification du code et affichés en direct :
[les mesures](/product/performance).

## Ce que cela coûte en licences

Les produits gratuits ne coûtent rien en licences : vous les payez en serveurs
(plus haut) et en temps (plus bas). Les services payants affichent un prix. Pour
les comparer, prenons un éditeur de logiciels qui vend à des entreprises, le cas
où ce socle pèse le plus lourd :

- 5 000 utilisateurs actifs par mois
- 20 organisations clientes, dont 10 avec leur propre SSO
- 15 services, en production et en préproduction
- 50 millions de requêtes par mois
- une équipe de 10 développeurs

Avec 20 organisations, ce cas demande Meerkat Enterprise.

| Brique | La moins chère qui convient | Le choix courant | Meerkat |
| --- | --- | --- | --- |
| Identité, SSO, second facteur | Descope Pro, 499 $ | Auth0 Essentials, 2 000 $ | inclus |
| Gateway d'API | API7 Cloud, 750 $ | Gravitee Planet, 2 500 $ | inclus |
| Coffre à secrets | Infisical Pro, 200 $ | HCP Vault Essentials, 1 881 $ | inclus |
| Supervision | Grafana Cloud Pro, 100 $ | Grafana Cloud Pro, 100 $ | inclus |
| Audit des actions d'administration | WorkOS Audit Logs, 224 $ | WorkOS Audit Logs, 224 $ | inclus |
| Signalement d'un problème | Userback Business, 79 $ | Marker.io Team, 149 $ | inclus |
| Tunnel des développeurs | mirrord Team annuel, 400 $ | mirrord Team mensuel, 500 $ | inclus dans Enterprise |
| Menu et portail dans vos applications | aucun produit | aucun produit | inclus |
| **Par mois** | **2 252 $** | **7 354 $** | Enterprise, sur demande |
| **Par an** | **27 024 $** | **88 248 $** | [voir les tarifs](/pricing/index) |

Prix catalogue publics, en dollars américains hors taxes, sans les serveurs ni le
travail d'intégration. Les produits sans prix public pour ce cas sont écartés
plutôt que devinés (Kong Konnect au-delà de 10 millions de requêtes, Tyk, Kong
Enterprise, Pomerium Enterprise, Frontegg au-delà de cinq connexions). Pour
donner un ordre de grandeur : Traefik Hub se vend de 30 000 à 50 000 $ par an,
et les offres facturées à l'utilisateur, comme Cloudflare Access ou Pomerium
Zero à 7 $, atteindraient 35 000 $ par mois pour 5 000 utilisateurs.

## Chaque client qui apporte son propre SSO

Les services d'identité facturent le SSO client par client. Le socle coûte donc
plus cher à chaque grand compte signé : précisément quand vous gagnez.

::: figure sso-per-customer
Ce qu'ajoute, chaque mois, un client de plus qui a son propre SSO.
:::

Connexions incluses avant que le compteur ne démarre : **cinq chez Stytch et
Descope, trois avec Auth0 Essentials, une chez Clerk, aucune chez WorkOS**.
Au-delà, chaque client qui arrive avec son Entra ID ou son Okta est facturé,
quoi qu'il consomme. Avec Meerkat, le SSO d'un client est un réglage, pas une
ligne de facture.

## Ce que cela coûte en temps

Le coût le plus lourd n'est pas la licence, c'est l'ingénierie. Voici notre
estimation, en jours-homme, pour arriver au même résultat, de l'hypothèse basse à
l'hypothèse haute. L'adaptation de vos propres services, pour qu'ils lisent
l'identité de l'utilisateur, n'est pas comptée : elle est nécessaire dans tous
les cas.

::: figure person-days
Jours-homme pour arriver au même résultat, de l'estimation basse à l'estimation
haute.
:::

| Travail | Produits gratuits | Produits SaaS | Meerkat |
| --- | --- | --- | --- |
| Identité : installation, second facteur, passkeys, SSO | 10 à 20 | 5 à 10 | 0,5 à 1 |
| Pages de connexion à votre marque, traduites | 5 à 10 | 2 à 4 | 0,5 à 1 |
| Organisations clientes et leur administration | 15 à 30 | 5 à 15 | 0,5 à 1 |
| Proxy de connexion, identité transmise aux services | 3 à 6 | 3 à 6 | 0,5 à 1 |
| Gateway d'API : routes, limites de débit, quotas | 8 à 15 | 5 à 10 | 1 à 2 |
| Certificats et coffre à secrets | 6 à 13 | 3 à 6 | 0,5 à 1 |
| Supervision et tableaux de bord | 4 à 8 | 2 à 4 | 0 à 0,5 |
| Un journal d'audit unique pour tous les outils | 8 à 15 | 5 à 10 | 0 |
| Menu utilisateur et portail dans les applications | 8 à 15 | 8 à 15 | 0,5 à 1 |
| Signalement d'un problème | 1 à 3 | 1 à 2 | 0 à 0,5 |
| Brancher le poste d'un développeur | 2 à 5 | 1 à 3 | 0,5 à 1 |
| Configurations enregistrées, retour arrière | 5 à 10 | 3 à 6 | 0 |
| Tester l'ensemble de bout en bout | 8 à 15 | 5 à 10 | 1 à 2 |
| **Mise en place** | **83 à 165 jours** | **48 à 101 jours** | **5,5 à 12 jours** |
| À 650 € HT par jour | 54 à 107 k€ | 31 à 66 k€ | 3,6 à 7,8 k€ |
| Maintenance, en jours par an | 20 à 40 | 10 à 20 | 2 à 5 |

Le SaaS évite d'installer des serveurs, mais pas de raccorder les produits entre
eux, ni de construire ce qu'aucun d'eux ne couvre.

## D'où viennent ces chiffres

**Mesuré.** La mémoire et la vitesse viennent du banc de mesure du projet
(`tools/bench` dans le dépôt), exécuté pour la dernière fois le
**{{memory.date.fr}}** sur les machines x64 et arm64 de GitHub, au repos et en
charge. Ce qui est construit est lu dans le code, pas dans un plan :
[l'inventaire public](/product/features) compte aujourd'hui {{features.built}}
fonctionnalités livrées, {{features.partial}} en partie et {{features.todo}} à
venir.

**Relevé.** Les prix sont les prix catalogue publics, en dollars américains hors
taxes, lus sur les pages officielles le **16 septembre 2026**, sans aucune
remise. Un acheteur qui négocie paiera moins ; la comparaison se fait au prix
catalogue parce que c'est le seul que chacun peut vérifier. Le nombre de pods et
la mémoire sont ceux que la documentation de chaque produit donne pour la
production.

**Estimé.** Les jours-homme sont une estimation de Softwarity, donnée en
fourchette parce que c'en est une, et convertie à **650 € HT par jour** pour un
ingénieur DevOps ou sécurité expérimenté.

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

Les prix changent. Si une ligne de cette page n'est plus à jour, dites-le-nous et
nous la corrigerons.

## Ce que vous obtenez à la place

**Un produit au lieu de huit. Un pod au lieu de trente-huit. Un socle déjà là,
au lieu d'un socle à construire.**

Pour l'équipe qui vit avec :

- **Une seule chose à exploiter.** Une image, une console, un journal d'audit,
  une sauvegarde. Pas huit produits à tenir à jour, chacun avec ses alertes de
  sécurité.
- **Prêt pour le questionnaire de sécurité dès le premier jour.** Second
  facteur, passkeys, authentification unique par OpenID Connect, audit, TLS,
  coffre : tout est dans l'édition gratuite. Répondre à un appel d'offres cesse d'être un projet.
- **Vos développeurs rendus à votre produit.** Cinq à douze jours de mise en
  place au lieu de 83 à 165, et surtout deux à cinq jours d'entretien par an au
  lieu de vingt à quarante, chaque année.
- **Aucun enfermement.** L'édition Community est complète et le restera, et le
  code passe sous licence Apache 2.0 au bout de deux ans. Vous ne pariez pas
  votre porte d'entrée sur nous.

Ce qui est construit et ce qui ne l'est pas tient dans
[un tableau lu dans le code](/project/roadmap). Ce que contient chaque édition
est sur la page [Éditions](/product/editions). Et pour voir avant de décider, la
[galerie](/showcase/index) montre les écrans.
