---
title: Démarrage rapide
section: Démarrer
order: 2
summary: Lancez la gateway, connectez-vous à la console et regardez passer le trafic - en cinq minutes environ.
---

# Démarrage rapide

Deux ports, un conteneur, aucune base de données à installer. À la fin de cette
page, vous disposez d'une gateway en marche, d'un compte administrateur et
d'une requête qui l'a traversée.

## Lancer la gateway

Choisissez votre édition et votre plateforme : les commandes et les fichiers
ci-dessous s'y adaptent.

::: widget install
:::

Les deux ports publiés remplissent deux fonctions distinctes :

| Port | Plan | Qui s'y connecte |
|---|---|---|
| 8080 | plan de données | vos utilisateurs - c'est le port exposé au réseau |
| 9090 | plan de contrôle | la console d'administration - gardez-le en interne |

`/data` contient tout ce que la gateway sait : les routes, les comptes, le
coffre, les certificats. Sauvegarder ce volume, c'est sauvegarder la gateway.

> [!WARNING]
> N'exposez jamais le plan de contrôle sur Internet. Il porte la console et
> l'API d'administration ; rien de tout cela n'a vocation à être public.

## Le premier administrateur

À son tout premier démarrage - et uniquement si la table des comptes est
vide - la gateway crée un compte, `admin`, doté des droits globaux.

- Si `MEERKAT_ADMIN_PASSWORD` est définie, sa valeur devient le mot de passe de ce compte.
- Sinon, un mot de passe est généré et écrit **une seule fois** dans le journal, sur une ligne d'avertissement. Le compte devra alors changer de mot de passe à sa première connexion.

```bash
docker logs meerkat | grep 'admin account created'
```

> [!NOTE]
> La variable n'est lue que tant qu'il n'existe aucun compte. La définir plus
> tard ne change rien : elle ne permet pas de réinitialiser un mot de passe
> oublié.

## Se connecter à la console

Ouvrez **http://localhost:9090** et connectez-vous avec le compte `admin`. Si le
mot de passe a été généré, la console vous en demande un nouveau avant de vous
laisser aller plus loin.

## Faire passer une première requête

Une gateway neuve est **vide** : elle n'a aucune route, donc toutes les adresses
du port 8080 répondent 404. Donnez-lui en une, qui pointe vers un service de
test public :

1. Dans la console, ouvrez **Infra > Routes** et cliquez sur **New route**.
   Nommez-la `demo`.
2. Dans **Target**, gardez **Proxy** et saisissez l'upstream :
   `https://httpbin.org`.
3. Dans **Predicates**, ajoutez un prédicat **path** avec le motif `/demo/**`.
4. Dans **Incoming**, ajoutez un filtre **strip-prefix** avec `1` segment : le
   service de test connaît `/get`, pas `/demo/get`.
5. Cliquez sur **Save**. La route sert aussitôt, sans rien redémarrer.

Ouvrez ensuite **[http://localhost:8080/demo/get](http://localhost:8080/demo/get)**
dans votre navigateur.

Ce que vous voyez est la réponse de httpbin : la requête telle que le service
l'a reçue, avec les en-têtes que la gateway a ajoutés au passage. L'appel est
entré par la gateway et ressorti vers le service. Ouvrez **Metrics** dans la
console pour le voir compté, sur la route `demo`.

## Ensuite

Cette route est ouverte à tous et pointe vers le service de quelqu'un d'autre.
[Votre première route](/docs/start/first-route) place un de vos services
derrière la gateway, dit qui peut l'atteindre, et la vérifie avant vos
utilisateurs.
