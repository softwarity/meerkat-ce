---
title: Tarifs
section: Tarifs
order: 1
layout: wide
summary: Quatre éditions d'une même gateway. Deux sont gratuites et se téléchargent en une commande, deux commencent par une conversation.
---

# Tarifs

Une gateway, quatre éditions. Deux d'entre elles s'obtiennent par un simple
`docker pull` et ne coûtent rien ; les deux autres sont construites pour vous.

::: cards
### Community - gratuite

**CE.** Toute la gateway pour une organisation sur une instance : le
routage, les pages de connexion, les rôles et les règles d'accès, TLS, le
coffre, le second facteur et les passkeys, le journal d'audit, les écrans de
trafic, la console, le point d'entrée des agents.

Gratuite pour tout usage, **production comprise**, sous Functional Source
License. Pas de compte à créer, pas de clé, pas de date d'expiration.

[Démarrer](/docs/start/quick-start)

### Évaluation - gratuite

**Eval.** Tout le produit, Enterprise compris, pour essayer avant d'en parler :
plusieurs organisations, SAML, LDAP et Active Directory, le cluster, ACME, les
configurations dans git, l'export OpenTelemetry, le tunnel développeur. Aucune limite de durée, aucun compteur,
rien de désactivé.

En contrepartie, elle affiche une mention - *version d'évaluation, sans licence
pour un usage en production* - sur les pages de connexion, dans le bouton
utilisateur, dans les e-mails et sur toute la console. Aucun réglage ne la
retire.

[Démarrer](/docs/start/quick-start)

### Team - parlons-en

**TE.** Tout ce que fait Enterprise, pour une installation de taille connue :
un cluster de quelques gateways, dont votre contrat fixe le nombre. Sans
mention d'évaluation, et avec le support de ceux qui ont écrit la gateway.

Les tailles et leur prix sont en cours de définition. Dites-nous combien de
gateways vous faites tourner, nous vous répondrons par un chiffre.

### Enterprise - parlons-en

**EE.** La même image, sans plafond : autant de gateways et de clusters que
vous en exploitez, dans tous vos environnements et sur tous vos sites.

Le prix porte sur l'installation : **ni par utilisateur, ni par requête, ni par
route**.
:::

## Ce que contient chaque édition

| | Community | Évaluation | Team | Enterprise |
| --- | --- | --- | --- | --- |
| **Prix** | Gratuite | Gratuite | Sur demande | Sur demande |
| **D'où elle vient** | Docker Hub | Docker Hub | Construite pour vous | Construite pour vous |
| **Usage en production** | Oui | Non | Oui | Oui |
| **Les capacités Enterprise** | - | Oui | Oui | Oui |
| **Mention d'évaluation** | - | Partout | - | - |
| **Gateways dans un cluster** | Une | Sans limite | Fixé par votre contrat | Sans limite |
| **Support** | Le dépôt public | Le dépôt public | Compris dans l'accord | Compris dans l'accord |

La page [Éditions](/product/editions) détaille, ligne par ligne, ce que sont
"les capacités Enterprise".

## Une image construite pour vous

Une image Team ou Enterprise est **construite pour votre société**, dans le
cadre de votre contrat. Il n'y a toujours rien à activer : pas de clé à installer, pas de
serveur d'activation à joindre, pas de droit à renouveler, rien qui expire au
milieu de la nuit. La gateway ne nous appelle jamais, et elle ne contient
aucun rapport d'usage.

Les deux éditions gratuites sont le même binaire pour tout le monde, publié
ouvertement.

## Évaluer d'abord

L'édition d'évaluation existe pour que personne n'ait à acheter sur la foi
d'une présentation. Faites-la tourner avec votre annuaire, vos applications et
votre cluster aussi longtemps qu'il le faut pour vous décider : elle ne retient
rien, et elle ne s'arrête pas.

En revanche, elle ne sert pas à faire tourner une production : la mention
figure sur chaque page que voient vos utilisateurs, et la retirer supposerait
de construire un autre binaire, ce que la licence interdit. Passer d'une
évaluation à une image sous licence conserve tout : même base de données, même
configuration, seule l'image change.

## Les questions qu'on nous pose d'abord

**Peut-on utiliser l'édition gratuite en production, en entreprise, dans un
produit commercial ?** Oui. La Functional Source License autorise explicitement
l'usage interne et l'usage en production. Elle n'interdit qu'une chose : s'en
servir pour construire une gateway concurrente.

**Peut-on lire et modifier le code ?** Oui, le tronc est public et modifiable.
Et **deux ans après sa publication, chaque version passe sous licence Apache
2.0**, sans aucune condition : ce que vous déployez aujourd'hui ne pourra pas
vous être retiré plus tard.

**Voyez-vous notre trafic, nos utilisateurs ou notre configuration ?** Non. Le
produit ne contient aucune télémétrie, et rien en lui ne nous envoie quoi que
ce soit.

**Team ou Enterprise ?** Les capacités sont les mêmes. Team convient à une
installation dont vous pouvez donner la taille dès aujourd'hui ; Enterprise est
faite pour celle que vous ne voulez pas compter.

**Que devient une installation si un accord prend fin ?** C'est l'une des
conditions en cours de rédaction ; posez-nous la question, nous y répondrons
clairement, sans langage de contrat. Le produit, lui, ne coupe rien
brutalement : une capacité qu'il ne contient plus est refusée par une phrase
qui le dit, et tout ce qui est déjà en place continue d'être servi.

**Vendez-vous du support pour l'édition gratuite ?** Demandez-nous. Le support
de l'image Community passe par le dépôt : un ticket ouvert sur
[softwarity/meerkat-ce](https://github.com/softwarity/meerkat-ce/issues) est lu.

::: cta
### Parlons-en

Le contact commercial est en cours de mise en place, et son adresse figurera
ici. En attendant, ouvrez un ticket sur
[softwarity/meerkat-ce](https://github.com/softwarity/meerkat-ce/issues) en
précisant ce que vous construisez et à quelle échelle : nous prendrons la
suite.
:::
