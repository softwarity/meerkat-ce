---
title: Ce qu'elle fait
section: Le produit
order: 2
summary: Chaque domaine de la passerelle, ce qu'il enlève à vos services, et où en est chacun aujourd'hui.
---

# Ce qu'elle fait

Meerkat prend en charge ce dont une application interne a besoin et qu'aucune
équipe ne devrait écrire deux fois. Les domaines ci-dessous sont tout le
produit. Rien ici n'est une projection : le dépôt porte un tableau,
[FEATURES.md](https://github.com/softwarity/meerkat/blob/main/FEATURES.md), une
ligne par fonctionnalité, son état lu dans le code, et livrer quelque chose,
c'est cocher sa case dans le même commit.

## Les domaines

| Domaine | Ce que Meerkat prend en charge | Où cela en est |
| --- | --- | --- |
| [Authentification](/docs/auth/overview) | Les pages de connexion servies par la passerelle elle-même, les comptes locaux, et les autorités d'entreprise - OpenID Connect, LDAP et Active Directory, GitHub - utilisées pour l'authentification seulement. | Livré, testé contre de vrais serveurs. SAML et Kerberos ne sont pas écrits. |
| [Second facteur](/docs/auth/mfa) | TOTP avec navigateurs de confiance, et les [passkeys](/docs/auth/passkeys) comme facteur de plein droit plutôt qu'en rajout. | TOTP livré ; passkeys utilisables, la récupération reste à écrire. |
| [Autorisation](/docs/access/overview) | Un catalogue de rôles hiérarchique, des groupes par organisation, une règle par route, et une [sécurité par endpoint](/docs/access/endpoint-security) lue dans la description OpenAPI d'un service. | Livré. |
| [Organisations](/docs/access/tenants) | Plusieurs locataires dans une installation : membres, modes de groupe, propriétaire, sélection à la connexion, politique de session par organisation. | Livré. Enterprise. |
| [Routage](/docs/concepts/routes) | Onze [prédicats](/docs/predicates/overview) et trente-trois [filtres](/docs/filters/overview), édités à chaud, appliqués sans redémarrage. | Livré. Les configurations versionnées sont à moitié construites. |
| [Habillage injecté](/docs/concepts/data-plane-chrome) | Le bouton de compte, le [portail de navigation](/docs/customise/portal), le thème clair/sombre et le CSS par rôle sont ajoutés à vos pages par la passerelle, quel que soit le framework de l'application. | Livré. |
| [Le coffre](/docs/operations/vault) | Des secrets scellés au repos et des valeurs en clair, référencés par leur nom, pour qu'une configuration s'exporte sans exporter ce qu'elle cache. | Livré. |
| [TLS](/docs/operations/tls) | Des certificats gérés par la passerelle, émission ACME comprise, sérialisée pour qu'un cluster demande une seule fois. | Livré. |
| [Identité vers l'amont](/docs/concepts/identity) | Un JWT signé de courte durée portant qui appelle, ses rôles et son organisation, pour que votre service lise un en-tête au lieu d'authentifier. | La signature est livrée ; l'endpoint d'échange n'est pas écrit. |
| [Quotas et limites](/docs/operations/rate-limits) | Des limites par route et par appelant, avec la sémantique 429 standard et un mode calibration qui ne fait que journaliser. | Limites livrées ; les quotas par consommateur sont en construction. |
| [Audit](/docs/operations/audit) | Chaque changement d'administration enregistré avec son auteur et un diff champ par champ, lisible par domaine, secrets masqués. | Livré. |
| [Trafic et santé](/docs/operations/traffic) | Les chiffres de trafic, la santé des amonts et les anomalies dans la console. Pas de Prometheus, pas de Grafana, pas de YAML. | Livré, avec encore à montrer. |
| [Piloté par un agent](/docs/agent/overview) | Un endpoint MCP sur le plan de contrôle, pour qu'un assistant lise et modifie l'installation selon les mêmes règles qu'un humain. | Livré. |
| [Mode développement](/product/dev-mode) | Le poste d'un développeur remplace un service déployé, et tout le monde sait lequel et par qui. | Le tunnel marche ; ce qui le montre aux utilisateurs est en construction. |
| Déploiement | Un binaire, stockage embarqué, [une passerelle](/docs/deploy/one-gateway) ou un [cluster](/docs/deploy/kubernetes) derrière un PostgreSQL. | Livré. |

## Ce qu'elle refuse d'être

Meerkat est une **app-gateway**, pas une API gateway. Elle existe pour servir
une application faite de plusieurs services, pas pour exposer des API à des
tiers. Cette seule décision explique pourquoi l'identité, les rôles, les
organisations et les pages de connexion sont dans le produit plutôt qu'à côté,
et pourquoi la console est un outil d'opérateur plutôt qu'un éditeur de YAML.

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
