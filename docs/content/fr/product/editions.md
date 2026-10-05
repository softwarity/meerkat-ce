---
title: Éditions
section: Le produit
order: 5
summary: Ce que fait l'édition gratuite, ce qu'ajoute Enterprise, comment tout essayer, et les trois questions qui disent laquelle il vous faut.
---

# Éditions

Un produit, quatre éditions. L'édition **Community** n'est ni une démonstration
ni une version d'essai : c'est toute la gateway pour le cas courant - une
organisation, une gateway, vos propres comptes ou un fournisseur OpenID
Connect. **Enterprise** répond au besoin d'une installation qui grandit dans
l'une de ces trois directions : davantage d'organisations, davantage de
gateways, ou davantage de raccordements au système d'information de
l'entreprise. **Team**, c'est Enterprise pour un cluster de taille connue, et
l'édition d'**évaluation**, c'est Enterprise avec une mention affichée,
gratuite, pour tout essayer d'abord.

| Édition | Ce qu'elle est | Comment l'obtenir |
| --- | --- | --- |
| **Community** (CE) | Toute la gateway gratuite, production comprise | `softwarity/meerkat` sur Docker Hub |
| **Évaluation** (Eval) | Tout ce qui suit, avec une mention d'évaluation ; pas pour la production | `softwarity/meerkat:eval` sur Docker Hub |
| **Team** (TE) | Tout ce qu'ajoute Enterprise, pour un cluster dont la licence fixe la taille | Construite pour vous - [parlons-en](/pricing/index) |
| **Enterprise** (EE) | Tout ce qu'ajoute Enterprise, sans plafond | Construite pour vous - [parlons-en](/pricing/index) |

::: lead
Voici la ligne que nous tenons, et c'est celle qui compte au moment de
comparer : **une brique de sécurité ne se vend jamais**. TLS et ses
certificats, le coffre, le second facteur, les passkeys, la politique de mots
de passe, la protection contre les attaques par force brute, le journal
d'audit, les règles d'accès par route et par endpoint : tout cela se trouve
dans l'image gratuite, et y restera. Vendre la sécurité à ceux qui ont le moins
les moyens de la payer n'est pas un modèle économique qui nous intéresse.
:::

La règle, arrêtée le **8 août 2026**, tient en deux phrases : **ce qui coûte à
l'organisation qui grandit se paie ; ce qui protège l'utilisateur ne se paie
pas.** Sa conséquence apparaît dès que vous comparez : là où les produits
d'identité facturent l'authentification unique et le second facteur au palier
supérieur, Meerkat ne pousse jamais personne à déployer une solution moins sûre
pour payer moins cher.

## Ce que l'édition gratuite fait déjà

Tout ce que décrit la [page des fonctionnalités](/product/features), à
l'exception des lignes du tableau ci-dessous. Le routage avec ses onze
prédicats et ses trente-trois filtres, les pages de connexion à vos couleurs,
les comptes locaux, OpenID Connect et GitHub, les rôles et les groupes, le
portail de navigation, le coffre, TLS (certificats générés, importés ou signés
sur demande), les rate limits, le journal d'audit, les écrans de trafic et
de métriques, la console entière et le point d'entrée des agents. En
production, en entreprise, pour un usage commercial, gratuitement.

## Ce qu'ajoute Enterprise

| | Community | Enterprise |
| --- | --- | --- |
| **Organisations** | Une seule. La console ne la nomme jamais, puisqu'il n'y a rien dont il faudrait la distinguer. | Plusieurs, avec leurs membres, leurs modes de groupe, un propriétaire, le choix de l'organisation à la connexion et une politique de session par organisation. |
| **Annuaire d'entreprise** | OpenID Connect, GitHub | LDAP et Active Directory en plus, en recherche puis bind (search-then-bind). |
| **Rôles issus de l'annuaire** | Accordés dans Meerkat | Règles de groupe : un groupe LDAP, une équipe GitHub ou un claim OIDC devient une appartenance et ses rôles, à chaque connexion. |
| **Certificats** | Générés, importés ou signés sur demande, placés à la main sur la console et sur l'application | ACME en plus : Let's Encrypt, ZeroSSL, Google ou votre propre step-ca les émettent et les renouvellent sans intervention, avec plusieurs autorités côte à côte. |
| **Heures ouvrées** | - | Fenêtres d'accès : plages horaires, jours de la semaine, fuseau horaire. Livré en partie - voir la [feuille de route](/project/roadmap). |
| **Plusieurs gateways** | Une gateway sur son stockage embarqué | Actif/actif sur un PostgreSQL partagé : bus de changements, certificats partagés, aucune affinité de session à demander. Voir [la page sur le cluster](/docs/deploy/kubernetes). |
| **Supervision** | Écrans de trafic et de métriques intégrés, rien à installer. Journaux écrits en JSON OpenTelemetry pour un agent installé sur le nœud. | Les mêmes écrans, plus l'export vers votre collecteur OpenTelemetry : traces, métriques et journaux envoyés en OTLP. |
| **Audit** | Le journal d'audit, dans la console | Le journal envoyé à votre collecteur, l'audit des endpoints (Endpoint audit) opération par opération, et l'export CSV. |
| **Configurations enregistrées** | Trois à la fois, et la console vous dit où vous en êtes avant que vous n'atteigniez le plafond | Autant que vous voulez : une par client, une par environnement. |
| **Configurations dans git** | Export et import d'un fichier | Chaque plateforme dans son répertoire d'un dépôt git : pull, comparaison, activation, push - committé au nom de l'exploitant. Voir [Configuration](/docs/console/configuration). |
| **Pages intégrées** | Vos couleurs, votre logo, votre nom, la disposition centrée (*Centered*) | Les dispositions *Split*, *Drawer*, *Banner* et *Bare* en plus, et la marque Meerkat retirée des pages que vous servez. |
| **Tunnel développeur** | plug tourne à côté de la gateway, ce qui est son mode par défaut | Le tunnel dans la gateway, et chaque substitution attribuée au développeur qui l'a faite. Voir [Mode développement](/product/dev-mode). |
| **Support** | Le dépôt : un ticket est lu, et la réponse reste là où elle servira au suivant | Compris dans l'accord : vous joignez ceux qui ont écrit la gateway, pas un premier niveau de support. |

Annoncés mais pas encore construits : SAML 2.0, Kerberos et l'export du journal
d'audit au format Parquet. La [feuille de route](/project/roadmap) dit où en
est chacun.

## Essayer Enterprise d'abord

L'édition d'évaluation est l'image Enterprise à laquelle rien n'a été retiré :
aucune limite de durée, aucun compteur, aucune capacité désactivée. Elle
ajoute en revanche une mention - *Meerkat evaluation version. Not licensed for
production use.* - sur les pages que sert la gateway, dans le bouton
utilisateur, dans les e-mails qu'elle envoie, et en filigrane sur toute la
console. Aucun réglage ne la retire : la mention est dans le binaire, et les
images sous licence sont simplement construites sans elle.

```bash
docker run -p 8080:8080 -p 9090:9090 \
  -e MEERKAT_ADMIN_PASSWORD=choose-one softwarity/meerkat:eval
```

Passer plus tard à une image sous licence conserve la base de données et la
configuration telles quelles : c'est une autre image sur les mêmes données.

## Laquelle il vous faut

Trois questions, et un seul oui suffit :

1. Plusieurs clients, filiales ou services doivent-ils rester **séparés au sein
   d'une même installation** ?
2. Vos comptes sont-ils dans **Active Directory** ?
3. La gateway doit-elle **survivre à la perte d'une machine** ?

Si la réponse est non trois fois, l'image Community est tout le produit, et
il n'y a rien à acheter. Si l'une est oui, faites tourner l'édition
d'évaluation sur votre cas avant d'en parler à qui que ce soit.

## Rien à activer

L'édition, c'est l'image. Pas de clé de licence à installer, pas de serveur
d'activation à joindre, pas de droit à renouveler, et rien qui expire au milieu
de la nuit : l'essentiel du code Enterprise est tout simplement absent du
binaire Community, et une image Team ou Enterprise est construite pour votre
société avec sa licence déjà à l'intérieur. La gateway ne nous appelle
jamais : elle ne contient aucun rapport d'usage.

Quand on demande à une image Community ce qu'elle ne contient pas, elle le dit
en une phrase qui nomme l'action plutôt qu'une ligne de grille tarifaire, et
elle ne laisse jamais une installation en panne : ce qui est déjà en place
continue d'être servi.

## Les licences

Le tronc est sous
[FSL-1.1-Apache-2.0](https://github.com/softwarity/meerkat-ce/blob/main/LICENSE.md),
la Functional Source License. Lisez le code, modifiez-le, faites-le tourner en
production, livrez-le dans votre propre produit. La licence n'interdit qu'une
chose : s'en servir pour construire une gateway concurrente. **Deux ans
après sa publication, chaque version passe sous licence Apache 2.0**, sans plus
aucune condition.

Les sources Enterprise ne sont pas dans l'arbre public. Les clients peuvent les
lire, et elles ne s'utilisent que dans le cadre d'un accord commercial avec
Softwarity.

Voir [les tarifs](/pricing/index).
