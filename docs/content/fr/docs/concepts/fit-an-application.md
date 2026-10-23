---
title: Adapter une application
section: Concepts
order: 24
summary: Une application existante derrière Meerkat - publiée sous un chemin, confiante dans l'identité de la gateway, avec ses propres rôles, appelée en TLS - et les symptômes qui disent lequel de ces points ne va pas.
---

# Adapter une application

::: lead
Personne ne réécrit Grafana, RabbitMQ ou un outil interne de dix ans pour le
mettre derrière une gateway. C'est Meerkat qui se règle pour coller à
l'application, pas l'inverse : cette page passe en revue les cinq endroits où une
application et sa gateway doivent s'accorder, avec les cas rencontrés dans de
vraies installations, puis liste les symptômes et ce que chacun veut dire.
:::

Les cinq endroits, dans l'ordre où une requête les rencontre :

1. **Le chemin** sous lequel elle est publiée, et si elle le sait.
2. **L'adresse** à laquelle la gateway l'appelle, et le certificat qu'elle présente.
3. **L'identité** à laquelle elle fait confiance, et sa propre page de connexion.
4. **Les rôles**, sous les noms de l'application plutôt que ceux de la gateway.
5. **La page** : son thème clair ou sombre, sa langue, ce qu'elle garde dans le
   navigateur.

## 1. Le chemin

Une route publie l'application sous un chemin, `/grafana/**`, et l'application
croit en général être servie depuis `/`. Trois situations, de la plus simple à la
plus difficile :

**Elle écrit des liens relatifs.** La plupart des applications monopages sont
construites avec `<base href="/">` et toutes leurs ressources relatives à cette
base. Ajoutez [strip-prefix](/docs/filters/strip-prefix) : l'application reçoit
`/`, et au retour la gateway réécrit le `<base href>` de la page avec le chemin de
la route, avec ou sans guillemets (une page minifiée écrit `<base href=/>`). C'est
le cas de Hubble UI : `strip-prefix`, rien d'autre.

**Elle écrit des liens absolus, mais on peut lui donner son préfixe.** Grafana
(`root_url` et `serve_from_sub_path`), l'administration de SeaweedFS
(`-urlPrefix`) et la plupart des frameworks (`X-Forwarded-Prefix`, que
`strip-prefix` annonce) savent servir depuis un sous-chemin. La route **garde**
alors le chemin, sans `strip-prefix`, et l'application répond dessous. Regardez
ce que l'option déplace d'autre : le `-urlPrefix` de SeaweedFS déplace aussi son
`/health`, et des sondes écrites pour `/health` échouent ensuite.

**Elle écrit des liens absolus et ne sait rien d'autre.** Un `/static/app.css`
demandé depuis une page sous `/outil/` sort de la route et tombe là où va
`/static/**`, le plus souvent la route attrape-tout. Aucune réécriture de la page
n'attrapera tous les liens qu'un script construit. Donnez-lui plutôt un nom
d'hôte à elle : une route avec un prédicat [host](/docs/predicates/host) sur son
propre nom, et le chemin laissé tel quel.

> [!TIP]
> Les routes sont essayées par ordre croissant, et **à ordre égal, par nom**.
> Deux routes au même ordre dont les chemins se recouvrent, `/geoserver/ows` et
> `/geoserver/**`, ne fonctionnent que tant que leurs noms se trient dans le bon
> sens. Donnez à la plus large un ordre plus grand : renommer la plus étroite ne
> doit pas changer qui répond. Voir
> [L'ordre](/docs/concepts/routes#lordre-et-pourquoi-tout-en-dpend).

## 2. L'adresse et le certificat

Faites pointer l'upstream vers le **nom complet** du service dans la plateforme :
`https://reference-data.canopy.svc:443` sur Kubernetes, `stack_service` sur Swarm.
Le sélecteur d'upstream affiche le nom court pour la lecture et enregistre le nom
complet, et l'éditeur signale une route en `https` enregistrée avec un nom court,
avec un bouton qui écrit le nom complet.

Le nom complet compte dès que l'autorité du cluster est **restreinte en noms** :
une autorité autorisée à certifier les noms en `svc` et `cluster.local` ne peut pas
certifier `reference-data`, et l'appel échoue avec `DNS name "reference-data" is not
permitted`. Deux conséquences :

- la route doit appeler le nom que le certificat a le droit de porter ;
- le certificat du service ne doit pas non plus contenir de nom court. La
  vérification confronte **tous** les noms du certificat aux restrictions, quel que
  soit le nom appelé : un seul nom court dans la liste fait échouer tout le
  certificat.

Quand l'autorité du cluster est remplacée, la gateway ne fait pas confiance à la
nouvelle tant qu'on ne le lui dit pas : la route répond 502 avec `certificate
signed by unknown authority`. Ajoutez-la dans **Infra > Domains & TLS > Trusted
authorities** ([Trouvées dans le cluster](/docs/operations/tls#trouves-dans-le-cluster)).
Un service qui garde le certificat chargé à son démarrage doit être redémarré
pour prendre celui que la nouvelle autorité a réémis.

## 3. L'identité, et la connexion propre à l'application

La route dit qui peut passer. Dire à **l'application** qui est passé, c'est la
section Identity de la route, sous la forme que l'application lit : des en-têtes,
un JWT signé, ou OpenID Connect avec Meerkat comme fournisseur. Les recettes par
application sont dans [Connecter une application](/docs/auth/connect-an-application).

Certaines applications ne savent rien en recevoir : elles ont leurs propres
utilisateurs, leur propre formulaire, et aucun moyen de faire confiance à un
proxy. Deux choix honnêtes :

- **Garder leur connexion.** La personne se connecte deux fois ; rien d'autre ne
  change.
- **Couper leur connexion**, et laisser la règle d'accès de la route seule à la
  porte. C'est ainsi que l'administration de SeaweedFS tourne sans
  `-adminPassword`. Seulement avec une politique réseau qui n'admet que la
  gateway : sans elle, n'importe quel pod du cluster administre l'application
  sans mot de passe.

> [!WARNING]
> Une application qui fait confiance à la gateway, par un en-tête ou une
> administration ouverte, ne doit être joignable **que** par elle. Pas de
> NodePort, pas d'ingress à elle, pas de port publié : qui l'atteint autrement est
> cru de la même façon.

## 4. Les rôles, sous les noms de l'application

Les rôles de la gateway sont ceux du catalogue : `ROLE_ADMIN`,
`satellite-operator`, `grafana-viewer`. L'application a son propre vocabulaire.
Trois mécanismes traduisent, chacun à sa place :

| Où | Ce qu'il traduit | Pour aller plus loin |
|---|---|---|
| Une autorité (LDAP, OIDC, SAML) | les groupes avec lesquels une personne arrive, en rôles du catalogue | [Authentification](/docs/auth/overview) |
| La spec OpenAPI d'une route | les rôles que l'application déclare, vers des rôles du catalogue (`roleMap`) | [Sécurité par endpoint](/docs/access/endpoint-security#les-rles-que-lapplication-dclare) |
| L'Identity d'une route, attribut `roles` | ce qui est envoyé : filtré, renommé, joint, ou réduit à une seule valeur | [La forme des rôles](/docs/auth/service-jwt#la-forme-des-rles) |

Le troisième est une expression, lue comme un pipeline : `.Keep`, `.Drop` et
`.Declared` sélectionnent, `cutHead`, `addHead`, `lower`... retouchent, `join` et
`json` mettent en forme. C'est le text/template de Go, donc elle peut aussi
**décider** : `if`, `else if` et `else` transforment un ensemble de rôles en la
seule valeur qu'une application attend.

Grafana attend `Admin`, `Editor` ou `Viewer`. Avec trois rôles du catalogue
étiquetés `grafana`, une seule route sert tout le monde :

```text
{{if .Roles | .Keep "ROLE_ADMIN" "grafana-admin"}}Admin{{else if .Roles | .Keep "grafana-editor"}}Editor{{else}}Viewer{{end}}
```

envoyée sous `X-WEBAUTH-ROLE`, avec une règle d'accès sur `ROLE_ADMIN`,
`grafana-admin`, `grafana-editor` et `grafana-viewer`. Grafana applique le rôle
reçu à chaque requête : un rôle changé dans Meerkat est le rôle dans Grafana à la
page suivante.

RabbitMQ est l'autre forme courante : un claim à valeur fixe, `["admin"]`, sur une
route réservée aux administrateurs.

## 5. La page

Une route UI injecte le bouton utilisateur, le portail et le thème clair ou sombre
dans des pages que l'application écrit. Là où l'application garde son propre
état, les deux peuvent se contredire :

- **Thème et langue.** Déclarez comment l'application les lit, et cachez son
  propre sélecteur avec le CSS de la route, pour qu'il n'y en ait qu'un, celui de
  la gateway. Voir [Thème clair ou sombre](/docs/customise/color-scheme).
- **Des sessions gardées dans le navigateur.** RabbitMQ écrit une échéance de
  session dans le `localStorage` à la première connexion et ne la déplace jamais ;
  passé ce moment, la page affiche son bouton de connexion pour de bon, alors que
  la session de la gateway est valide. Un script d'une ligne injecté en tête de
  page efface la clé ([Injections](/docs/customise/injections)).
- **Sa propre déconnexion.** Elle ne termine que la session de l'application, et
  la page suivante reconnecte la personne depuis la gateway. Cachez-la, et laissez
  la déconnexion au bouton utilisateur.

## Symptômes

| Symptôme | Cause probable | Remède |
|---|---|---|
| La page arrive, blanche ; ses scripts répondent 404 ou la page d'une autre application | liens absolus, ou un `<base href>` que la gateway n'a pas réécrit | `strip-prefix` pour des liens relatifs ; l'option de préfixe de l'application ; ou un nom d'hôte à elle |
| Un 404 sur le chemin nu, la page marche avec une barre finale | le chemin de montage sans sa barre | rien : une route UI redirige `/outil` vers `/outil/` |
| L'application redirige vers son adresse interne | elle construit ses redirections depuis son propre nom | [rewrite-location](/docs/filters/rewrite-location), ou lui donner son adresse publique |
| 502, `certificate signed by unknown authority` | la gateway ne fait pas confiance à l'autorité qui signe le service | l'ajouter aux autorités de confiance ; redémarrer un service qui sert encore l'ancien certificat |
| 502, `DNS name "x" is not permitted` | une autorité restreinte en noms, et un nom court appelé ou présent dans le certificat | le nom complet du service dans l'upstream ; aucun nom court dans le certificat |
| L'application affiche sa propre page de connexion | elle ne lit pas l'identité que la route envoie, ou lit un autre nom | vérifier les noms d'en-têtes ou de claims des deux côtés ; voir les recettes |
| Tout le monde a les mêmes droits dans l'application | les rôles ne sont pas envoyés, ou pas dans son vocabulaire | une expression de rôles dans l'Identity de la route |
| Un chemin servi par la mauvaise route | deux routes au même ordre qui se recouvrent, départagées par le nom | des ordres différents |
| Connecté à la gateway, déconnecté dans la page au bout d'un moment | un état que l'application garde dans le navigateur | l'effacer par un script injecté, ou aligner sa durée |
</content>
</invoke>
