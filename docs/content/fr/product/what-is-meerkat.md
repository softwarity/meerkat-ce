---
title: Ce qu'est Meerkat
section: Le produit
order: 1
summary: Un point d'entrée unique devant vos applications internes, qui se charge de tout ce qui n'est pas le cœur de métier de vos équipes.
---

# Ce qu'est Meerkat

Meerkat est une **app-gateway** : une porte unique devant les applications de
votre organisation. Les requêtes arrivent à Meerkat, qui décide quoi en faire,
puis les transmet.

Ce dont elle se charge, pour que vos services n'aient pas à le faire :

- **Qui appelle** - pages de connexion, authentification unique (SSO), authentification à plusieurs facteurs, jetons d'API, sessions.
- **Qui peut passer** - rôles, groupes, organisations, règles par route et par endpoint.
- **Comment la requête voyage** - routage, réécriture, en-têtes, limites de débit, TLS.
- **Ce qui se passe** - trafic, journal d'audit, métriques, santé de vos services.

## Pourquoi une gateway

Une application n'est plus un seul programme : c'est une dizaine de services,
écrits par des équipes différentes. Une gateway place une porte unique devant
eux. Le navigateur ne parle qu'à cette porte ; c'est elle qui décide qui entre
et où va chaque requête, et les services derrière reçoivent des requêtes déjà
vérifiées.

::: figure gateway
Une requête entre par la gateway, qui connecte l'utilisateur, vérifie ses
droits, route et enregistre, puis la remet au bon service avec un JWT signé.
Une requête sans les droits s'arrête à la porte.
:::

Une application interne démarre en général sans rien de tout cela. Puis il lui
faut une page de connexion, et quelqu'un l'écrit. Puis une façon de réinitialiser
un mot de passe oublié, donc d'envoyer des e-mails. Puis un second facteur, parce
que la sécurité l'a demandé. Puis le client veut se connecter avec son propre
annuaire : Active Directory pour l'un, Azure pour le suivant. Puis les sessions
doivent expirer, et être révoquées quand quelqu'un s'en va.

Pendant ce temps, une deuxième application a besoin de la même chose, et les
deux ne s'entendent pas sur ce qu'est une session. Une troisième arrive, écrite
par une autre équipe, dans un autre langage.

Viennent ensuite les questions que personne n'avait prévues. Qui peut voir cet
écran, et qui en décide ? Qui a modifié ce réglage mardi dernier : le journal
d'audit. Pourquoi cette page est lente : les traces, du navigateur jusqu'à la
base de données. Que s'est-il passé à 3 heures du matin : les logs, en un seul
endroit plutôt qu'un par service. Combien d'appels un client peut-il faire : les
limites de débit. Les certificats, qui expirent. La page de maintenance, pour le soir
de la mise à jour.

**Rien de tout cela n'est votre métier.** C'est ce dont chaque application a
besoin avant de pouvoir faire ce pour quoi elle existe, et cela finit écrit dans
chacune d'elles, maintenu dans chacune, relu par la sécurité dans chacune - un
peu différemment à chaque fois. Meerkat est l'endroit où l'on répond à ces
questions une fois pour toutes, devant toutes vos applications, pour que vos
équipes écrivent la partie qu'elles seules savent écrire.

## Pourquoi une APP gateway plutôt qu'une API gateway

Une **API gateway** - Kong, APISIX, Traefik - répond à une seule de ces
questions : comment une requête est routée. Tout le reste est un plugin à
configurer, ou un produit à installer à côté : un fournisseur d'identité pour
la connexion, un proxy d'authentification, un gestionnaire de secrets, une
stack de métriques, un collecteur de logs, un gestionnaire de certificats, un
tunnel pour que les développeurs testent sur le cluster (mirrord,
Telepresence), des écrans à construire vous-même. Huit produits environ à
choisir, déployer, sécuriser et mettre à jour, et à faire tenir ensemble : le
[dossier Meerkat](/product/the-case) les compte.

Une **APP gateway** réunit tout cela, sur étagère et prête à l'emploi : les
pages de connexion, les comptes et les rôles, le coffre, le journal d'audit,
les traces, les certificats, le tunnel développeur et la console qui les
pilote sont dans la même image, et fonctionnent déjà ensemble. Vous la
démarrez, vous ajoutez vos applications.

Tout-en-un ne veut pas dire fermé. Ce que votre entreprise exploite déjà reste
aux commandes, et Meerkat s'y branche :

- **Votre fournisseur d'identité** - Entra ID, Okta, Google, Keycloak ou tout
  fournisseur OpenID Connect, et avec Enterprise SAML 2.0, LDAP ou Active
  Directory - connecte les utilisateurs ; Meerkat ne vous demande pas de
  déplacer vos comptes.
- **Votre stack d'observabilité** reçoit ce que voit Meerkat. Ses journaux sont
  écrits en JSON OpenTelemetry pour l'agent déjà présent sur vos nœuds ; avec
  Enterprise, les traces du navigateur jusqu'à vos services, les métriques, les
  journaux et le journal d'audit partent vers votre collecteur, puis vers
  Grafana, Datadog, Elastic ou ce que vous utilisez.
- **Vos services** gardent leurs propres contrôles s'ils le souhaitent : le JWT
  qu'ils reçoivent se vérifie avec les clés publiées par la gateway.
- **Votre PKI** reste l'autorité : importez vos certificats, ou, avec
  Enterprise, laissez votre propre serveur ACME les émettre et les renouveler.
- **Votre dépôt git** porte la configuration avec Enterprise, un répertoire par
  plateforme, relue comme le reste de votre code.
- **Votre automatisation** pilote le tout par l'API d'administration, ou un
  agent IA par MCP.

Les écrans intégrés sont là pour que vous n'ayez besoin de rien d'autre pour
démarrer - pas pour vous empêcher d'utiliser ce que vous avez. Ce qui demande
Enterprise est listé sur la page [Éditions](/product/editions).

> [!NOTE]
> Meerkat se place devant vos applications telles qu'elles sont. Elle ne leur
> demande ni d'embarquer une bibliothèque, ni de parler un protocole qui lui
> serait propre.

## Une seule image

La gateway est écrite en Go pur et se livre en une seule image. Elle se déploie
sur Docker, Swarm ou Kubernetes, et il n'y a rien d'autre à déployer pour
l'utiliser : ni base de données à côté, ni cache, ni broker de messages. Elle
sert vos applications sur un port et sa console d'administration sur un autre.

En chiffres : l'image Community pèse **70 Mo**, l'image Enterprise **200 Mo**.

## Pourquoi ce nom

::: figure meerkat
Le suricate, en sentinelle.
:::

Le suricate (*meerkat* en anglais) est la sentinelle de la nature : il monte la
garde à l'entrée du terrier et donne l'alerte, pour que le reste de la colonie
vaque à ses occupations sans s'inquiéter de rien. C'est exactement ce que cette
gateway fait pour vos services. Même le tunnel
[plug](https://github.com/softwarity/plug) trouve sa place dans l'image : c'est
par lui que la machine d'un développeur creuse sa galerie jusqu'au terrier. Et
comme, en anglais, un groupe de suricates s'appelle un *mob*, vous savez déjà
comment appeler un cluster de nœuds Meerkat.
