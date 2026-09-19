---
title: Démarrage rapide
section: Démarrer
order: 2
summary: Lancer la gateway, se connecter à la console, et voir passer du trafic - en cinq minutes environ.
---

# Démarrage rapide

Deux ports, un conteneur, aucune base de données à installer. À la fin de cette
page vous avez une gateway qui tourne, un compte administrateur, et une requête
qui est passée par elle.

## Lancer la gateway

```bash
docker run -d --name meerkat \
  -p 8080:8080 -p 9090:9090 \
  -e MEERKAT_ADMIN_PASSWORD='choisissez-en-un' \
  -v meerkat-data:/data \
  docker.io/softwarity/meerkat:latest
```

Les deux ports publiés sont deux métiers différents :

| Port | Plan | Qui l'atteint |
|---|---|---|
| 8080 | plan de données | vos utilisateurs - c'est celui qui fait face au réseau |
| 9090 | plan de contrôle | la console d'administration - gardez-le interne |

`/data` contient tout ce que la gateway sait : les routes, les comptes, le
coffre, les certificats. Sauvegardez ce volume et vous avez sauvegardé la
gateway.

> [!WARNING]
> Ne publiez jamais le plan de contrôle sur Internet. C'est la console et l'API
> d'administration ; rien là-dedans n'est destiné à être public.

## Le premier administrateur

À son tout premier démarrage - et seulement quand la table des comptes est
vide - la gateway crée un compte, `admin`, avec les droits globaux.

- Si `MEERKAT_ADMIN_PASSWORD` est définie, c'est son mot de passe.
- Sinon, un mot de passe est généré et écrit **une seule fois** dans le journal, sur une ligne d'avertissement. Le compte est alors marqué pour changer de mot de passe à la première connexion.

```bash
docker logs meerkat | grep 'admin account created'
```

> [!NOTE]
> La variable n'est lue que tant qu'aucun compte n'existe. La définir plus tard
> ne change rien, et ce n'est pas un moyen de réinitialiser un mot de passe
> oublié.

## Se connecter à la console

Ouvrez **http://localhost:9090** et connectez-vous en `admin`. Si le mot de
passe a été généré, la console en réclame un nouveau avant de vous laisser aller
ailleurs.

La console a elle-même deux plans dans sa navigation de gauche : **Infra** pour
l'installation (routes, TLS, autorités d'authentification, configuration) et
**Application** pour ce que vos utilisateurs rencontrent (comptes, rôles, pages
intégrées, portail).

## Voir passer du trafic

Une gateway sans route répond 404 à tout : une installation neuve est donc
amorcée avec trois routes de démonstration qui pointent vers `httpbin.org`.

| Route | Ce qu'elle prend | Accès |
|---|---|---|
| `demo` | `/demo/**` | ouverte à tous |
| `demo-secure` | `/secure/**` | tout compte connecté |
| `trap` | `/**` | ouverte, ordonnée en dernier - attrape ce que rien d'autre n'a pris |

```bash
curl -i http://localhost:8080/demo/get
```

Le préfixe `/demo` est retiré avant l'appel : ce qui répond est donc
`https://httpbin.org/get`. Demandez `/secure/get` dans un navigateur et vous
atterrissez sur la page de connexion que la gateway sert elle-même.

> [!TIP]
> Ces trois routes sont des routes ordinaires, stockées comme les autres.
> Supprimez-les quand vous aurez les vôtres, en commençant par l'attrape-tout -
> c'est lui qui fait qu'une gateway neuve répond quelque chose sur tous les
> chemins.

## Ensuite

Faire pointer une route vers un de vos propres services : [Votre première
route](/docs/start/first-route).
