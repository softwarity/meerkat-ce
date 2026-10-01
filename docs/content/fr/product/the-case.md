---
title: Le dossier
section: Le produit
order: 4
summary: Ce que coûte le socle de toute application professionnelle quand on l'assemble, quand on l'achète en service, et quand il est déjà là.
printable: true
---

# Le dossier Meerkat

Avant qu'une application montre son premier écran métier, elle doit faire tout
ce qu'un acheteur attend d'un logiciel professionnel. Rien de tout cela ne
distingue le produit, tout est vérifié par les services achats et sécurité, et
chaque pièce coûte du temps à construire puis à maintenir.

::: lead
Cette page est l'arithmétique de ce socle : ce que vous assemblez sans Meerkat,
ce que cela coûte dans le cluster, en licences et en jours d'ingénierie. Les
chiffres sont sourcés et datés, et là où un nombre est une estimation, c'est
écrit.
:::

## Le socle attendu en 2026

La liste telle qu'elle apparaît dans les questionnaires de sécurité et les
appels d'offres.

| | |
| --- | --- |
| **Une connexion soignée** | des pages à votre marque, traduites, claires et sombres |
| **Second facteur et passkeys** | TOTP, codes de récupération, WebAuthn |
| **Le SSO du client** | Entra ID, Okta, Google, LDAP, Active Directory |
| **Multi-organisation** | clients isolés, membres, groupes, administrateurs délégués |
| **Rôles et accès fins** | par écran, par route, par endpoint d'API |
| **Jetons d'API** | pour les intégrations et les machines |
| **Protection du trafic** | limites de débit, quotas, disjoncteur, délais |
| **TLS automatique** | certificats émis et renouvelés sans intervention |
| **Des secrets à l'abri** | chiffrés au repos, jamais dans la configuration |
| **Journal d'audit** | qui a changé quoi, avec l'avant et l'après |
| **Observabilité** | trafic, latence, erreurs, par route et par endpoint |
| **Haute disponibilité** | plusieurs nœuds, aucune session perdue |
| **Maintenance annoncée** | une vraie page plutôt qu'une erreur 502 |
| **Travaux planifiés** | clôtures, relances, purges, rapports du matin |
| **Retour utilisateur** | signaler un problème avec une capture et le contexte |
| **Outillage développeur** | tester contre le cluster depuis sa propre machine |
| **La documentation des API** | un Swagger de toutes les API, appelées à travers la passerelle, sous l'identité qu'on choisit |

Ce que Meerkat en couvre, ligne par ligne et avec l'état lu dans le code, est
sur [ce qu'elle fait](/product/features).

## Ce que vous assemblez sans elle

Pour chaque besoin, le produit habituellement choisi - en version libre
auto-hébergée, ou en offre commerciale.

| Besoin | Libre, auto-hébergé | Commercial ou SaaS | Meerkat |
| --- | --- | --- | --- |
| Connexion, MFA, passkeys | Keycloak, Zitadel, Authentik | Auth0, WorkOS, Clerk, FusionAuth | inclus |
| Pages à votre marque, 20 langues | un thème à construire | selon le palier | inclus |
| SSO du client : OIDC, LDAP, AD | Keycloak | Auth0, WorkOS, facturé par connexion | inclus, Enterprise pour LDAP |
| Organisations gérées par vos clients | des écrans d'admin à construire | WorkOS, Frontegg | inclus, Enterprise |
| Protéger une application sans authentification | oauth2-proxy, Pomerium Core | Pomerium Enterprise, Cloudflare Access | inclus |
| Routage, limites, quotas, disjoncteur | Kong OSS, APISIX, Traefik, Envoy Gateway | Kong Enterprise, Tyk, Traefik Hub, API7, Gravitee | inclus |
| Certificats TLS automatiques | cert-manager | dans les paliers supérieurs des gateways | inclus |
| Coffre à secrets | Vault Community, OpenBao | HCP Vault, Infisical, Doppler | inclus |
| Tableaux de bord de trafic | Prometheus et Grafana | Grafana Cloud | inclus |
| Audit de chaque action d'administration | Retraced, puis câbler chaque outil | WorkOS Audit Logs, puis câbler chaque outil | inclus |
| Menu utilisateur et portail dans vos applications | **à construire** | **aucun produit standard** | inclus |
| Signalement d'incident avec capture | Sentry auto-hébergé | Marker.io, Jam, Userback, BugHerd | inclus |
| Poste de développeur vers le cluster | plug, mirrord OSS, Telepresence, Gefyra | mirrord Team, Okteto | intégré, Enterprise |
| Configurations versionnées, retour arrière | un pipeline GitOps à monter | un pipeline GitOps à monter | inclus |
| Appels planifiés vers vos services | cron, CronJob Kubernetes, ou Quartz et Celery dans chaque service | Temporal Cloud, Inngest, Trigger.dev, EventBridge Scheduler | inclus |
| Pilotage par un agent IA | serveurs MCP communautaires | Kong Konnect, SaaS uniquement | inclus |

Deux de ces lignes n'ont aucun produit derrière elles : le menu utilisateur et
le portail à l'intérieur de vos propres applications, et un audit transverse
qui couvre tous les outils d'un coup. Celles-là, vous les écrivez.

> [!WARNING]
> Trois faits, relevés en septembre 2026, pèsent sur la voie libre. Kong
> présente la 3.9 comme sa dernière version pleinement libre : la branche libre
> ne reçoit plus que des correctifs 3.9.x pendant qu'Enterprise est en 3.16, et
> le mode gratuit de l'image Enterprise a été retiré en 3.10. Vault Community
> est sous licence BSL, qui interdit les offres concurrentes. Traefik Hub
> offline est une option sous licence dont le jeton expire au bout d'un an, la
> gateway s'arrêtant après 30 jours de grâce.

## Ce que cela coûte dans le cluster

Chaque produit ajouté au cluster apporte ses pods, sa base, ses sauvegardes,
ses mises à jour et ses bulletins de sécurité. La même porte d'entrée,
construite des deux façons, avec les nombres d'instances que la documentation
de chaque produit recommande en production.

| Brique | Produit | Instances en production | État à exploiter | Mémoire recommandée |
| --- | --- | --- | --- | --- |
| Identité | Keycloak | 3 pods, exemple de la doc | PostgreSQL | 1 250 Mo par pod |
| Gateway d'API | Kong OSS, sans base | 3 nœuds | Redis, pour les limites partagées | 2 à 4 Go par nœud |
| Proxy d'authentification | oauth2-proxy | 2 pods | Redis, pour les sessions | non publiée |
| Certificats | cert-manager | 7 pods, bonne pratique | - | non publiée |
| Coffre à secrets | OpenBao ou Vault | 5 nœuds Raft | Raft, et les parts de descellement | 8 à 16 Go par nœud |
| Supervision | kube-prometheus-stack | 5 pods, plus 1 par nœud | sa propre base de séries | 512 Mo au minimum pour Grafana |
| Audit | Retraced | 5 services | PostgreSQL, Elasticsearch et NSQ | non publiée |
| Poste vers cluster | opérateur mirrord | 1 pod, plus un job par session | licence Team obligatoire | non publiée |
| **Tout le socle** | **Meerkat** | **1 pod, ou 3 en cluster** | **base embarquée, ou PostgreSQL** | **22 Mo au repos** |

::: figure stack
La même porte d'entrée, des deux façons. À gauche une requête traverse deux
produits avant d'atteindre vos services, et six autres tournent à côté ; à
droite, un seul.
:::

Cela fait **environ 38 pods et cinq moteurs de stockage** à installer,
sécuriser, mettre à jour et sauvegarder - et une requête traverse deux de ces
produits avant d'atteindre vos services.

L'autre façon : **un pod, 22 Mo au repos**, mesuré par la CI sur un runner x64.
Trois pods et un PostgreSQL quand vous la voulez
[hautement disponible](/docs/deploy/kubernetes).

## Et elle tient la charge

Un produit au lieu de huit n'est une bonne nouvelle que si ce produit n'est pas
le lent. C'est mesuré plutôt qu'affirmé : Meerkat tourne à côté de Kong, APISIX
et Traefik, chacun épinglé sur le même CPU unique, devant le même service, sous
la même charge, dans la même exécution - le tableau compare donc des produits et
non des machines. La CI le recalcule à chaque commit et le site le lit en
direct, ce qui explique qu'aucun chiffre ne soit écrit ici :
[les mesures](/product/performance).

## Ce que cela coûte en licences

Les briques libres ne coûtent rien en licences : leur prix est le cluster
ci-dessus et le temps ci-dessous. Les offres commerciales, elles, affichent un
prix. Pour les comparer, un scénario - un éditeur qui sert des clients
entreprises, là où le socle est le plus lourd :

- 5 000 utilisateurs actifs par mois
- 20 organisations clientes, dont 10 avec leur propre SSO
- 15 services, en production et en préproduction
- 50 millions de requêtes par mois
- une équipe de 10 développeurs

| Brique | Offre la moins chère qui convient | Offre typique | Meerkat |
| --- | --- | --- | --- |
| Identité, SSO, MFA | Descope Pro, 499 $ | Auth0 Essentials, 2 000 $ | inclus |
| Gateway d'API | API7 Cloud, 750 $ | Gravitee Planet, 2 500 $ | inclus |
| Coffre à secrets | Infisical Pro, 200 $ | HCP Vault Essentials, 1 881 $ | inclus |
| Supervision | Grafana Cloud Pro, 100 $ | Grafana Cloud Pro, 100 $ | inclus |
| Audit des actions d'admin | WorkOS Audit Logs, 224 $ | WorkOS Audit Logs, 224 $ | inclus |
| Signalement d'incident | Userback Business, 79 $ | Marker.io Team, 149 $ | inclus |
| Poste vers cluster | mirrord Team annuel, 400 $ | mirrord Team mensuel, 500 $ | plug : intégré |
| Menu et portail dans vos applications | aucun produit | aucun produit | inclus |
| **Par mois** | **2 252 $** | **7 354 $** | Communautaire : gratuit |
| **Par an** | **27 024 $** | **88 248 $** | Enterprise : [par instance de production](/product/pricing) |

Prix publics en dollars américains, hors taxes, hors machines et hors temps
d'intégration. Les produits sans prix public pour ce scénario sont laissés de
côté plutôt que devinés : Kong Konnect Plus plafonne à 10 millions de requêtes,
donc le scénario bascule sur devis, et Tyk, Kong Enterprise, Pomerium
Enterprise et Frontegg au-delà de cinq connexions sont sur devis. Pour l'ordre
de grandeur, Traefik Hub se vend 30 000 à 50 000 $ par an sur l'AWS
Marketplace, et les offres par utilisateur comme Cloudflare Access ou Pomerium
Zero à 7 $ atteindraient 35 000 $ par mois pour 5 000 utilisateurs.

## Chaque client qui apporte son SSO

Les offres d'identité facturent les connexions SSO par client : le socle coûte
donc plus cher à chaque contrat signé avec une grande entreprise - exactement
au moment où vous gagnez.

::: figure sso-per-customer
Ce qu'un client de plus avec son propre SSO ajoute, chaque mois.
:::

Les quotas inclus avant que ce compteur ne démarre : **cinq connexions chez
Stytch et Descope, trois chez Auth0 Essentials, une chez Clerk, aucune chez
WorkOS**. Passé ce seuil, chaque client qui arrive avec son Entra ID ou son
Okta se facture, et le montant ne dépend pas de ce qu'il consomme.

## Ce que cela coûte en temps

Le coût le plus lourd d'un socle assemblé n'est pas la licence, c'est
l'ingénierie. Une estimation en jours-homme pour atteindre le même périmètre,
lot par lot, de l'hypothèse basse à l'hypothèse haute. Elle exclut l'adaptation
de vos propres services à l'identité transmise, que toutes les options
demandent.

::: figure person-days
Jours-homme pour atteindre le même périmètre, de l'hypothèse basse à la haute.
:::

| Lot | Pile libre | Pile SaaS | Meerkat |
| --- | --- | --- | --- |
| Identité : installation, MFA, passkeys, fédération | 10 à 20 | 5 à 10 | 0,5 à 1 |
| Pages de connexion à la marque, traduites | 5 à 10 | 2 à 4 | 0,5 à 1 |
| Organisations clientes et leur administration | 15 à 30 | 5 à 15 | 0,5 à 1 |
| Proxy d'authentification, identité vers les services | 3 à 6 | 3 à 6 | 0,5 à 1 |
| Gateway d'API : routes, limites, quotas, disjoncteur | 8 à 15 | 5 à 10 | 1 à 2 |
| Certificats et coffre à secrets | 6 à 13 | 3 à 6 | 0,5 à 1 |
| Supervision et tableaux de bord | 4 à 8 | 2 à 4 | 0 à 0,5 |
| Audit transverse des actions d'administration | 8 à 15 | 5 à 10 | 0 |
| Menu utilisateur et portail dans les applications | 8 à 15 | 8 à 15 | 0,5 à 1 |
| Signalement d'anomalies | 1 à 3 | 1 à 2 | 0 à 0,5 |
| Poste de développement vers cluster | 2 à 5 | 1 à 3 | 0,5 à 1 |
| Configurations versionnées, retour arrière | 5 à 10 | 3 à 6 | 0 |
| Recette de bout en bout | 8 à 15 | 5 à 10 | 1 à 2 |
| **Mise en place** | **83 à 165 jours** | **48 à 101 jours** | **5,5 à 12 jours** |
| À 650 € HT par jour | 54 à 107 k€ | 31 à 66 k€ | 3,6 à 7,8 k€ |
| Maintenance, jours-homme par an | 20 à 40 | 10 à 20 | 2 à 5 |

La pile SaaS évite d'installer des serveurs mais garde l'intégration, le
câblage entre produits, et le développement qu'aucun produit ne couvre.

## D'où viennent ces chiffres

Un chiffre sans sa méthode ne vaut rien, alors la voici, et les sources avec.

**Ce qui est mesuré.** La mémoire et le débit viennent du banc de la CI de ce
projet (`tools/bench`), dernière exécution le **17 septembre 2026** sur les
runners GitHub x64 et arm64, mémoire du conteneur relevée au repos **et sous
charge**. L'état des fonctions est lu dans le code, pas dans un plan :
[l'inventaire public du dépôt](/product/features) au **16 septembre 2026**
donne 106 fonctions livrées, 87 livrées en partie et 21 à venir.

**Ce qui est relevé.** Les prix sont les prix **publics**, en dollars US hors
taxes, lus sur les pages officielles le **16 septembre 2026**, sans remise
négociée - un acheteur qui négocie paiera moins, et c'est exactement pour ça
que la comparaison est faite au tarif affiché. Les nombres de pods et la
mémoire recommandée sont ceux que chaque documentation donne pour la
production.

**Ce qui est estimé.** Les jours-homme sont une estimation de Softwarity,
donnée en fourchette parce que c'est ce qu'elle est. La conversion en euros
retient **650 € HT par jour** pour un ingénieur DevOps ou sécurité confirmé.

::: details Les sources, une par une
**Identité**

- auth0.com/pricing
- workos.com/pricing
- clerk.com/pricing
- descope.com/pricing
- stytch.com/pricing
- keycloak.org, dimensionnement mémoire et CPU

**Gateways et proxies**

- konghq.com/pricing, et le dimensionnement Kong
- Kong : quelles versions sont encore entièrement libres
- api7.ai/pricing
- gravitee.io/pricing
- Traefik Hub sur AWS Marketplace, et son mode hors ligne
- pomerium.com/pricing et Cloudflare Zero Trust

**Cluster, coffre, supervision, audit, outillage**

- cert-manager, ses bonnes pratiques de déploiement
- Vault, architecture de référence Raft, et OpenBao, stockage intégré
- HCP Vault, Infisical, Doppler
- grafana.com/pricing, et le chart kube-prometheus-stack
- Retraced
- softwarity/plug et mirrord
- Userback, Marker.io, Jam
:::

Les prix bougent. Si une ligne ici est périmée, c'est la ligne qui a tort :
dites-le nous et nous la corrigerons.

## Ce que vous y gagnez

Ce qui ne bouge pas, c'est la forme de l'argument : **un produit au lieu de
huit, un processus au lieu de trente-huit pods, un socle déjà là au lieu d'un
socle à assembler.**

Concrètement, pour l'équipe qui va vivre avec :

- **Une seule chose à exploiter.** Une image, une console, un journal d'audit,
  une sauvegarde. Pas huit produits à mettre à jour, dont chacun a ses propres
  bulletins de sécurité et son propre rythme de version.
- **Un socle qui passe les questionnaires dès le premier jour.** Second
  facteur, passkeys, SSO client, audit, TLS, coffre : tout est là dans
  l'édition gratuite, donc l'appel d'offres ne devient pas un chantier.
- **Vos développeurs rendus à votre métier.** Les cinq à douze jours de mise en
  place remplacent quatre-vingt-trois à cent soixante-cinq jours, et surtout
  les vingt à quarante jours-homme par an que la pile assemblée redemande
  chaque année.
- **Le droit de changer d'avis.** L'édition Community est complète et le restera
  ; le code passe en Apache 2.0 au bout de deux ans. Vous ne pariez pas votre
  porte d'entrée sur notre survie.

Ce qui est réellement construit, et ce qui ne l'est pas, tient dans
[un tableau lu dans le code](/project/roadmap) - c'est la même transparence que
les chiffres ci-dessus. Ce que portent les deux éditions est sur
[éditions](/product/editions), et si vous voulez voir avant de décider, la
[galerie](/showcase/index) montre les écrans.
