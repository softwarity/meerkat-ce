---
title: Feuille de route
section: Le projet
order: 10
summary: Ce qui est construit, ce qui reste à finir, et ce qui est volontairement absent de la liste.
---

# Feuille de route

Sur l'état du produit, c'est
[FEATURES.md](https://github.com/softwarity/meerkat-ce/blob/main/FEATURES.md), dans
le dépôt, qui fait foi : une ligne par fonctionnalité, un état lu dans le code, et
une case cochée dans le commit qui livre la fonctionnalité. Cette page se contente
de lire ce tableau à voix haute.

## Construit et utilisé

Le chemin critique est complet. Une requête arrive, une route la reconnaît, des
filtres la transforment, une règle d'accès tranche, et ce qui s'est passé reste
visible après coup.

- **Le routage** : le catalogue de prédicats et de filtres, modifiable à chaud et
  appliqué dès la requête suivante. Voir les [prédicats](/docs/predicates/overview)
  et les [filtres](/docs/filters/overview).
- **L'identité** : comptes locaux, OpenID Connect, SAML 2.0, LDAP et Active
  Directory, GitHub, tous testés face à de vrais serveurs. Sessions, jetons d'API, jetons
  signés transmis aux upstreams.
- **L'accès** : un catalogue hiérarchique de rôles, des groupes par organisation,
  les organisations elles-mêmes, une règle par route, et une sécurité par endpoint
  lue dans la description OpenAPI d'un service.
- **Le second facteur** : TOTP avec navigateurs de confiance, et passkeys.
- **Le coffre** : des secrets scellés au repos et des valeurs en clair, tous
  référencés par leur nom. La même clé maîtresse scelle les clés TLS et les
  secrets TOTP, et un redémarrage suffit à la renouveler.
- **TLS** : les certificats et l'émission ACME, sérialisée pour qu'un cluster ne
  fasse la demande qu'une fois.
- **L'audit** : chaque changement d'administration, avec son auteur et le détail
  champ par champ, ainsi que la sécurité des comptes - chaque connexion, chaque
  refus avec son motif et son adresse, chaque moyen d'accès modifié par son
  titulaire.
- **La console** : toute l'administration, sur son propre port, avec le découpage
  par capacités qui décide qui en voit quelle moitié.
- **Le point d'entrée des agents** : MCP sur le plan de contrôle, soumis aux mêmes règles
  qu'un humain.
- **Le portail de navigation** : une barre commune à toutes les applications que
  sert la gateway, injectée dans des pages qui n'embarquent aucune bibliothèque
  pour cela.
- **Le cluster** : plusieurs gateways derrière un même PostgreSQL, qui se
  coordonnent par la base plutôt qu'entre elles.
- **Les configurations versionnées** : plusieurs coexistent, une seule est active,
  l'export et l'import bouclent la boucle, chaque changement crée un point de
  reprise, et deux configurations enregistrées se comparent.
- **Les sessions ouvertes** : sur votre profil, la liste de vos appareils
  connectés, avec *Sign out everywhere else*. Les administrateurs voient les
  sessions et y mettent fin depuis la console.
- **L'export OpenTelemetry** (Enterprise) : une seule adresse de collecteur pour
  les traces, les métriques, le journal d'audit et les journaux. Voir
  [les traces](/docs/operations/tracing).
- **Les appels planifiés** : la gateway appelle un service à intervalle
  régulier, selon un calendrier cron ou une seule fois, avec des reprises et un
  historique des exécutions. Le service les crée par l'API, depuis ses propres
  écrans ; la console les surveille et intervient au besoin. Voir
  [les appels planifiés](/docs/operations/scheduler).
- **Le journal de la gateway**, en direct dans la console, d'où son niveau se
  relève pour une demi-heure. Voir [les journaux](/docs/operations/logs).

## En cours d'achèvement

Ces fonctionnalités marchent, mais ne sont pas terminées. Le tableau du dépôt
indique, ligne par ligne, ce qui manque à chacune.

- **L'audit des endpoints** (Enterprise) - un interrupteur par opération envoie
  ses appels au journal d'audit. Il reste à pouvoir choisir les champs du corps à
  conserver, et à auditer un refus prononcé avant que l'opération soit connue.
- **Les traces** - le contexte traverse la gateway et les traces partent vers
  votre collecteur. Il reste à ce que la gateway se déclare dans `tracestate`
  et `baggage`, et à relier une ligne d'audit à sa trace.
- **Les quotas** - ils se définissent par route, par endpoint et par
  consommateur - utilisateur, jeton, organisation, adresse - et un dépassement
  reçoit un 429 avec les en-têtes standard. Il manque l'écran qui montre la
  consommation, la possibilité de ralentir plutôt que de refuser, et des compteurs
  qui restent justes en cluster.
- **Le mode développeur** - le tunnel fonctionne, la connexion s'arrête sur une
  page qui dit ce qui est substitué et par qui, et un bandeau le rappelle pendant
  que vous travaillez. Il manque la portée d'une substitution - aujourd'hui, elle
  vaut pour tout le trafic - puis l'écran de la console qui liste les sessions en
  cours, et l'audit de chaque substitution (le dépôt d'une clé, lui, est déjà
  audité).
- **Les notifications** - le relais SMTP est livré, avec un gabarit unique aux
  couleurs du thème, et le récapitulatif quotidien des accès qui arrivent à
  échéance part tout seul. Il manque un gabarit par événement, chacun traduit.
- **L'identité transmise aux upstreams** - le jeton signé est émis, avec son JWKS
  publié et la rotation des clés. Il manque un endpoint d'échange qui rende un
  couple jeton d'accès / jeton de rafraîchissement, et les modes qui portent un
  secret jusqu'à l'upstream : BASIC, FORM, JWT tiers.
- **Les passkeys** - utilisables comme facteur. La récupération d'un compte dont
  l'unique passkey est perdue n'est pas encore écrite.
- **Les codes à usage unique par e-mail** - se connecter avec un code à la place
  du mot de passe est livré : désactivé par défaut, lié au navigateur qui a fait
  la demande, et jamais ouvert sur la console. Il manque le lien magique, et le
  moyen de fermer cette porte compte par compte.

## Ensuite

- **Un assistant de découverte** : la gateway sait déjà lire le socket Docker
  et un namespace Kubernetes. Il manque l'écran qui en fait "analyser, choisir un
  conteneur, obtenir une route".
- **Se connecter en tant que** : ce qu'un support fait tous les jours, et qui
  passe aujourd'hui par demander son mot de passe à quelqu'un.
- **Un cache de réponses**, puisque la gateway est déjà le seul endroit qui
  voit passer deux fois la même requête.
- **Servir une application sous un sous-chemin sans la reconstruire** : le préfixe
  est retiré à l'aller ; il reste à réécrire ce que l'application renvoie.
- **gRPC pour de bon** : la sécurité par méthode, et gRPC-Web. Les compteurs
  lisent déjà `grpc-status`.
- **Un portail par organisation** : une icône, un titre et un agencement propres à
  chaque client. Ce serait la première personnalisation visuelle par organisation
  du produit.
- **HTTP/3**, le jour où le gain se mesurera au lieu de se raconter.
- **Un coffre externe** : HashiCorp Vault, les secrets Kubernetes et les secrets
  Docker, pour les installations qui en ont déjà un et n'en veulent pas un second.
- **Les signalements poussés vers GitHub, GitLab ou Jira**, plutôt que lus dans
  un écran de plus.
- **Les notifications Web Push**, pour ce qu'un exploitant doit savoir sans
  garder un onglet ouvert.

## Pas sur la liste

Kerberos et SPNEGO sont inscrits comme fonctionnalités Enterprise, et rien n'est
commencé. Aucun système de plugins n'est prévu : le catalogue de filtres est
volontairement restreint, et chaque filtre qui y figure est un filtre que nous
savons expliquer et tester.

> [!TIP]
> Ce que la suite d'intégration vérifie réellement se trouve sur la page
> [couverture de tests](/project/tests) - c'est le fichier que la suite exécute,
> pas une description de ce fichier.
