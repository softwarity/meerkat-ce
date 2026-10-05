---
title: Votre première route
section: Démarrer
order: 3
summary: Placez l'un de vos services derrière la gateway, depuis la console, et vérifiez que c'est bien la bonne route qui répond.
---

# Votre première route

Une route répond à une question - *cette requête est-elle pour moi ?* - puis dit
ce qu'il faut en faire. Cette page en crée une qui relaie les requêtes vers un
service interne, et la vérifie avant d'y laisser entrer qui que ce soit.

Prenons une application de facturation, joignable dans votre cluster à l'adresse
`billing:8080`, que vous voulez servir sous `/billing`.

## Ouvrir l'éditeur

Dans la console, allez dans **Infra > Routes** et cliquez sur **New route**.
L'éditeur s'ouvre dans un tiroir, avec les sections de la route
alignées à gauche. Deux d'entre elles portent une étoile : sans elles, la route
ne peut pas être enregistrée. Ce sont **Target** et **Predicates**.

Donnez un nom à la route, en haut - `billing` convient très bien. C'est ce nom
que vous retrouverez dans le tableau, dans les courbes de trafic et dans le
journal d'audit.

![L'éditeur de route, ouvert sur Target](img/console/route-editor-target.webp)

## Target : ce qui répond

**Target** fixe la manière dont la route répond. Il existe quatre modes, et
seul le premier appelle un service :

| Mode | Ce qu'il fait |
|---|---|
| Proxy | interroge un upstream et renvoie sa réponse |
| Redirect | envoie le navigateur ailleurs |
| Maintenance | sert la page d'indisponibilité intégrée |
| Respond | répond à partir d'un gabarit, sans rien appeler |

Gardez **Proxy** et saisissez l'upstream : `http://billing:8080`. Le `http` simple
est volontairement la valeur par défaut : dans un cluster, TLS se termine en
général sur la gateway, et le dernier tronçon jusqu'au service circule en
clair. Les deux autres choix sont `https` et `h2c`.

> [!TIP]
> Quand la gateway peut lire son environnement d'exécution - un socket
> Docker ou Swarm, ou un namespace Kubernetes vu de l'intérieur - ce champ
> propose les services qu'elle a trouvés dès que vous y placez le curseur. La
> saisie reste libre : un upstream extérieur au cluster se tape à la main.

La même section règle le temps accordé à cet upstream (pour la connexion, puis
pour la première réponse) et dit s'il faut cesser de l'appeler quand il ne
répond plus. Ces deux réglages héritent de l'installation tant que vous ne les
modifiez pas : n'y touchez pas pour l'instant.

## Predicates : quelles requêtes

**Predicates** correspond à la moitié *est-ce pour moi ?*. Ajoutez un prédicat
**path** avec le motif `/billing/**`.

Une route peut porter plusieurs prédicats, combinés par un ET : un chemin, un
hôte et une méthode, c'est une route qui exige les trois. Les prédicats
disponibles aujourd'hui sont path, host, header, cookie, method, query,
remote-addr, x-forwarded-remote-addr, time-window, version et weight.

![Trois prédicats sur une même route](img/console/route-editor-predicates.webp)

## Retirer le préfixe

La gateway a reconnu `/billing/**`, mais l'application de facturation ignore
tout de ce préfixe : elle sert `/invoices`, pas `/billing/invoices`. Ouvrez
**Incoming** et ajoutez un filtre **strip-prefix** avec la valeur `1`.

Les filtres de la section Incoming transforment la requête : en-têtes,
paramètres de requête, chemin, hôte. **Outgoing** fait la même chose sur le
chemin du retour.

![Les filtres entrants d'une route](img/console/route-editor-filters.webp)

## Qui peut passer

**Security** porte la règle d'accès de la route. Si vous la laissez vide, la
route est ouverte : la gateway relaie sans demander qui appelle, et c'est le
service qui décide. Exigez un compte connecté, certains rôles ou certains
comptes nommés, et un appelant qui ne remplit pas la condition n'atteint jamais
l'upstream.

Les détails se trouvent dans [Contrôle d'accès](/docs/concepts/access-control).

![La section Security d'une route](img/console/route-editor-security.webp)

## Enregistrer

**Save** s'applique immédiatement : la table de routage est recompilée et, dans
un cluster, les autres nœuds sont prévenus. Il n'y a rien à redémarrer, aucun
fichier à recharger.

Une nouvelle route est créée avec l'ordre `0`, ce qui la place **en tête** de la
table. Ce n'est pas un détail : c'est la première route dont les prédicats
reconnaissent la requête qui répond. Une nouvelle route est donc essayée avant
toutes celles qui existent déjà.

## Vérifier avant vos utilisateurs

Il y a deux moyens ; commencez par le premier.

**Routing test**, sur l'écran Routes, compose une requête qui n'est jamais
envoyée : méthode, chemin, hôte, en-têtes, cookies, adresse du client, heure,
et l'identité sous laquelle elle sera jugée. Il indique ensuite, route par
route, laquelle prend la requête et pourquoi les autres ne l'ont pas prise :
requête non reconnue, requête reconnue mais appelant écarté par la règle, ou
route non évaluée parce qu'une autre, placée plus haut, a répondu avant elle.

Puis l'essai en conditions réelles :

```bash
curl -i http://localhost:8080/billing/invoices
```

Si aucune route ne reconnaît la requête, la gateway répond 404. Si une route
l'a reconnue mais que sa règle vous a écarté, une route UI vous conduit sur une
page qui nomme la règle à l'origine du refus ; une route de service renvoie un
simple 403.

## Ensuite

- Tout ce qui compose une route : [Routes](/docs/concepts/routes)
- Ce que la gateway ajoute à une page qu'elle relaie : [Ce que la gateway injecte](/docs/concepts/data-plane-chrome)
