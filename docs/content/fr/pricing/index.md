---
title: Tarifs
section: Tarifs
order: 1
layout: wide
summary: Quatre éditions d'une même gateway. Deux sont gratuites ; Team coûte 4 000 € par an, Enterprise 24 000 €, mises à jour et support compris. La licence est perpétuelle et vaut quel que soit le nombre d'installations.
---

# Tarifs

Une gateway, quatre éditions. Toutes s'obtiennent par un simple `docker pull`.
Deux ne coûtent rien ; les deux autres, Team et Enterprise, sont l'image
d'évaluation à laquelle vous donnez une licence.

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
plusieurs organisations, SAML, LDAP et Active Directory, plusieurs instances synchronisées, ACME, les
configurations dans git, l'export OpenTelemetry, le tunnel développeur. Aucune limite de durée, aucun compteur,
rien de désactivé.

En contrepartie, elle affiche une mention - *version d'évaluation, sans licence
pour un usage en production* - sur les pages de connexion, dans le bouton
utilisateur, dans les e-mails et sur toute la console. Seule une licence la
retire.

[Démarrer](/docs/start/quick-start)

### Team - 4 000 € par an

**TE.** Enterprise pour une organisation sur une instance : l'annuaire de
l'entreprise (SAML, LDAP, Active Directory), ACME, les configurations dans
git, l'export OpenTelemetry, les dispositions des pages et la marque retirée,
le fournisseur OpenID, le tunnel développeur intégré. Sans mention
d'évaluation. Licence perpétuelle ; le prix annuel inclut les mises à
jour et le support standard.

Faite pour la petite structure, sans contrainte de scalabilité.

Nous écrire : [sales@softwarity.io](mailto:sales@softwarity.io?subject=Meerkat%20Team)

### Enterprise - 24 000 € par an

**EE.** Tout, sans plafond : autant d'organisations que vous en exploitez, et
la gateway en cluster actif/actif sur PostgreSQL, qui monte en charge en
ajoutant des instances. Support étendu compris.

Une licence **perpétuelle** par entreprise, pour toutes ses installations : **ni par
installation, ni par utilisateur, ni par requête, ni par route**. Le prix
annuel inclut les mises à jour et le support ; le droit d'utilisation, lui,
est acquis une fois pour toutes.

Nous écrire : [sales@softwarity.io](mailto:sales@softwarity.io?subject=Meerkat%20Enterprise)
:::

## Ce que contient chaque édition

| | Community | Évaluation | Team | Enterprise |
| --- | --- | --- | --- | --- |
| **Prix par an** (mises à jour et support) | Gratuite | Gratuite | 4 000 € HT | 24 000 € HT |
| **Licence** | - | - | Perpétuelle | Perpétuelle |
| **D'où elle vient** | Docker Hub | Docker Hub | L'image d'évaluation et une licence | L'image d'évaluation et une licence |
| **Usage en production** | Oui | Non | Oui | Oui |
| **Les capacités Enterprise** | - | Oui | Oui | Oui |
| **Mention d'évaluation** | - | Partout | - | - |
| **Organisations** | Une | Sans limite | Une | Sans limite |
| **Installations** | Autant que vous voulez | Autant que vous voulez | Autant que vous voulez | Autant que vous voulez |
| **Instances par installation** | Une | Plusieurs, synchronisées | Une | Plusieurs, synchronisées |
| **Utilisateurs, routes, requêtes** | Sans limite | Sans limite | Sans limite | Sans limite |
| **Support compris** | Le dépôt public | Le dépôt public | Standard | Étendu |

La page [Éditions](/product/editions) détaille, ligne par ligne, ce que sont
"les capacités Enterprise".

## Le support

Le prix annuel inclut un an de mises à jour et un niveau de support. Sans
renouvellement, la licence reste valable : toute version publiée pendant la
période payée reste à vous, à installer et à faire tourner aussi longtemps que
vous voulez. Seules les versions publiées ensuite ne sont plus couvertes. Un
support au-delà de celui compris se commande à part.

| | Ce que c'est | Pour qui |
| --- | --- | --- |
| **Support standard** | Par e-mail, les jours ouvrés : les bugs et l'usage du produit. | Compris dans Team |
| **Support étendu** | Un délai de première réponse garanti, un interlocuteur nommé, l'aide aux montées de version. | Compris dans Enterprise |
| **Options de support** | Des plages horaires élargies et un délai court garanti sur un incident bloquant. | Enterprise, sur demande |
| **Prestations** | À la journée : l'intégration chez vous, la migration depuis un autre proxy ou fournisseur d'identité, la revue d'architecture, la formation. | Toutes les éditions, sur demande |

Le support répond sur le produit ; une prestation travaille sur votre projet.

## Une licence, saisie dans la console

Team et Enterprise ne sont pas des images à part : c'est l'image d'évaluation,
à laquelle vous donnez une licence. Vous la collez dans la console, elle est
vérifiée sur place, et la mention d'évaluation disparaît aussitôt, sans
redémarrage, sur toutes les instances de l'installation. Un déploiement peut
aussi la monter depuis un secret, où chaque instance la lit.

- **Vérifiée hors ligne.** La licence est signée par Softwarity et la gateway
  en contrôle la signature elle-même. Pas de serveur d'activation à joindre :
  la gateway ne nous appelle jamais, et elle ne contient aucun rapport d'usage.
- **Perpétuelle.** Une licence n'expire pas. Sa date dit jusqu'où vont les
  mises à jour couvertes : une version publiée après est hors de l'accord, et
  la console le dit, mais rien ne s'éteint.
- **Rien ne s'arrête au milieu de la nuit.** Une licence ne coupe jamais le
  trafic. Ce qui dépasse ses plafonds est refusé à la création, par une phrase
  qui le dit : une deuxième organisation sur une licence Team ne se crée pas,
  mais tout ce qui est déjà en place continue d'être servi.
- **Une licence par entreprise, autant d'installations que vous voulez.** La
  même licence se colle dans chacune, sans surcoût : une installation Team
  garde ses plafonds, une organisation et une instance, mais rien ne limite
  leur nombre. Elle couvre les installations de l'entreprise qui l'a achetée,
  et seulement celles-là : la transmettre à une autre structure, cliente ou
  partenaire, n'est pas couvert.
- **Le nom du titulaire est affiché** dans la console, avec l'identifiant de
  la licence : une licence collée ailleurs dit à qui elle appartient.

## Évaluer d'abord

L'édition d'évaluation existe pour que personne n'ait à acheter sur la foi
d'une présentation. Faites-la tourner avec votre annuaire, vos applications et
votre cluster aussi longtemps qu'il le faut pour vous décider : elle ne retient
rien, et elle ne s'arrête pas.

En revanche, elle ne sert pas à faire tourner une production : la mention
figure sur chaque page que voient vos utilisateurs, et aucun réglage ne la
retire. Passer d'une évaluation à une édition sous licence conserve tout :
même image, même base de données, même configuration. Seule la licence
s'ajoute.

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
ce soit. La licence se vérifie sur place.

**Team ou Enterprise ?** Les capacités sont les mêmes. Team tient sur une
organisation et une instance ; dès que vous devez séparer plusieurs clients ou
filiales dans une même installation, ou survivre à la perte d'une machine,
c'est Enterprise.

**Peut-on faire tourner plusieurs installations Team au lieu d'une
Enterprise ?** Oui, avec la même licence et sans surcoût. Elles ne partagent
ni leurs comptes, ni leur configuration, ni leurs sessions : c'est ce que le
cluster Enterprise apporte.

**Que devient une installation si on ne renouvelle pas ?** Elle continue de
tourner, sous licence : la licence est perpétuelle. Ce qui s'arrête, ce sont
les mises à jour et le support. Vous gardez les versions publiées pendant la
période payée ; les suivantes ne sont plus couvertes, et le renouvellement
les rouvre.

**Vendez-vous du support pour l'édition gratuite ?** Demandez-nous. Le support
de l'image Community passe par le dépôt : un ticket ouvert sur
[softwarity/meerkat-ce](https://github.com/softwarity/meerkat-ce/issues) est lu.

::: cta
### Parlons-en

Écrivez-nous à [sales@softwarity.io](mailto:sales@softwarity.io) en précisant ce que vous construisez et à
quelle échelle : une licence Team ou Enterprise, un contrat de support, une
intégration, ou simplement le projet dont vous voulez discuter.
:::
