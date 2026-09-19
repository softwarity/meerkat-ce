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
| **Retour utilisateur** | signaler un problème avec une capture et le contexte |
| **Outillage développeur** | tester contre le cluster depuis sa propre machine |

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

| Brique | Produit | Instances en production | État à exploiter |
| --- | --- | --- | --- |
| Identité | Keycloak | 3 pods | PostgreSQL |
| Gateway d'API | Kong OSS, sans base | 3 nœuds | Redis, pour les limites partagées |
| Proxy d'authentification | oauth2-proxy | 2 pods | Redis, pour les sessions |
| Certificats | cert-manager | 7 pods | - |
| Coffre à secrets | OpenBao ou Vault | 5 nœuds Raft | son propre stockage Raft |
| Supervision | Prometheus, Grafana | 8 pods | sa propre base de séries |
| Audit | Retraced | 5 pods | PostgreSQL et Elasticsearch |
| Poste vers cluster | opérateur mirrord | 1 pod, plus un job par session |  - |

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
| **Total** | **83 à 165 jours** | **48 à 101 jours** | **5,5 à 12 jours** |

La pile SaaS évite d'installer des serveurs mais garde l'intégration, le
câblage entre produits, et le développement qu'aucun produit ne couvre.

## D'où viennent ces chiffres

Les prix et les faits produits ont été relevés en **septembre 2026** dans les
documentations et les grilles tarifaires publiques. Le chiffre de mémoire est
mesuré par la CI de ce projet ; les nombres de pods sont ceux que chaque
documentation recommande en production ; les jours-homme sont des estimations,
données en fourchette parce que c'est ce qu'elles sont.

Les prix bougent. Si une ligne ici est périmée, c'est la ligne qui a tort :
dites-le nous et nous la corrigerons. Ce qui ne bouge pas, c'est la forme de
l'argument : un produit au lieu de huit, un processus au lieu de trente-huit
pods, et un socle déjà là au lieu d'un socle à assembler.

Ce qui est réellement construit, et ce qui ne l'est pas, tient dans
[un tableau lu dans le code](/project/roadmap). Ce que portent les deux
éditions est sur [éditions](/product/editions).
