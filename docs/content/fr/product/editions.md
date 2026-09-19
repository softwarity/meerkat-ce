---
title: Éditions
section: Le produit
order: 5
summary: Ce que fait l'édition gratuite, ce qu'ajoute Enterprise, et les trois questions qui disent laquelle il vous faut.
---

# Éditions

Un produit, deux images. L'édition **communautaire** n'est ni une démo ni un
essai : c'est toute la passerelle pour le cas courant - une organisation, une
passerelle, des comptes à vous ou un fournisseur OpenID Connect. **Enterprise**
est ce qu'il faut quand l'installation grandit dans une de ces trois
directions : plus d'organisations, plus de passerelles, ou plus de plomberie
d'entreprise.

::: lead
La ligne que nous tenons, et c'est celle qui compte quand vous comparez : **une
primitive de sécurité ne se vend jamais**. TLS et ses certificats, le coffre, le
second facteur, les passkeys, la politique de mot de passe, la protection
anti-force-brute, le journal d'audit, les règles d'accès par route et par
endpoint : tout cela est dans l'image gratuite, et y restera. Vendre la sûreté à
ceux qui peuvent le moins la payer n'est pas un modèle que nous voulons.
:::

## Ce que l'édition gratuite fait déjà

Tout ce que décrit la [page des fonctionnalités](/product/features), moins les
lignes du tableau ci-dessous. Le routage avec ses onze prédicats et ses
trente-trois filtres, les pages de connexion à vos couleurs, les comptes
locaux, OpenID Connect et GitHub, les rôles et les groupes, le portail de
navigation, le coffre, TLS avec ACME, les limites de débit, le journal d'audit,
les écrans de trafic et de métriques, toute la console, et le point d'entrée
agent. En production, en entreprise, commercialement, gratuitement.

## Ce qu'ajoute Enterprise

| | Communautaire | Enterprise |
| --- | --- | --- |
| **Organisations** | Une. Elle n'est jamais nommée dans la console, parce qu'il n'y a rien dont la distinguer. | Plusieurs, avec membres, modes de groupe, propriétaire, sélection à la connexion et politique de session par organisation. |
| **Annuaire d'entreprise** | OpenID Connect, GitHub | LDAP et Active Directory en plus, en search-then-bind. |
| **Les rôles depuis l'annuaire** | Accordés dans Meerkat | Règles de groupe : un groupe LDAP, une équipe GitHub ou un claim OIDC devient une appartenance et ses rôles, à chaque connexion. |
| **Heures ouvrées** | - | Fenêtres d'accès : plages horaires, jours de la semaine, fuseau. Partiel, voir la [feuille de route](/project/roadmap). |
| **Plusieurs passerelles** | Une passerelle sur son stockage embarqué | Actif/actif sur un PostgreSQL partagé : bus de changement, certificats partagés, aucune affinité de session à réclamer. Voir [la page cluster](/docs/deploy/kubernetes). |
| **Supervision** | Écrans de trafic et de métriques, intégrés, rien à installer | Les mêmes écrans, plus une exposition Prometheus pour que la stack que vous avez déjà les collecte. |
| **Configurations enregistrées** | Trois à la fois, et la console dit où vous en êtes avant d'atteindre le plafond | Autant que vous voulez : une par client, une par environnement. |
| **Pages intégrées** | Vos couleurs, votre logo, votre nom, la disposition centrée | Les dispositions split, tiroir, bandeau et nue en plus, et la marque Meerkat retirée des pages que vous servez. |
| **Tunnel de développement** | plug tourne à côté de la passerelle, ce qui est son mode par défaut | Le tunnel dans la passerelle, et chaque substitution attribuée au développeur qui l'a posée. Voir [Mode développement](/product/dev-mode). |
| **Support** | Le dépôt : une issue est lue, et la réponse reste là où elle servira au suivant | Fait partie de l'accord : vous joignez ceux qui ont écrit la passerelle, pas un palier. |

Annoncés et pas encore construits : SAML 2.0, Kerberos, et l'export du journal
d'audit vers des formats analytiques. La [feuille de route](/project/roadmap)
dit où en est chacun.

## Laquelle il vous faut

Trois questions, et un seul oui suffit :

1. Plusieurs clients, filiales ou services doivent-ils être **séparés à
   l'intérieur d'une même installation** ?
2. Vos comptes vivent-ils dans **Active Directory** ?
3. La passerelle doit-elle **survivre à la perte d'une machine** ?

Si les trois sont non, l'image communautaire est tout le produit, et il n'y a
rien à acheter.

## Rien à activer

L'édition, c'est l'image. Aucune clé de licence à installer, aucun serveur
d'activation à joindre, aucun droit à renouveler, et rien qui expire au milieu
d'une nuit : l'essentiel du code Enterprise n'est tout simplement pas dans le
binaire communautaire. La passerelle ne nous appelle jamais : il n'y a ni
rapport d'usage ni vérification de licence nulle part dedans.

Une image communautaire à qui l'on demande ce qu'elle ne porte pas le dit en
une phrase qui nomme l'acte plutôt qu'un mot de catalogue, et elle ne bloque
jamais une installation : ce qui est déjà en place continue d'être servi.

## Les licences

Le tronc est sous
[FSL-1.1-Apache-2.0](https://github.com/softwarity/meerkat-ce/blob/main/LICENSE.md),
la Functional Source License. Lisez-la, modifiez-la, faites-la tourner en
production, embarquez-la dans votre propre produit. La seule chose qu'elle
interdit est d'en construire une passerelle concurrente. **Deux ans après
chaque version, celle-ci devient de l'Apache 2.0** sans la moindre condition.

Les sources Enterprise ne sont pas dans l'arbre public. Elles sont lisibles par
les clients et utilisables seulement sous accord commercial Softwarity.

Voir [les tarifs](/product/pricing).
