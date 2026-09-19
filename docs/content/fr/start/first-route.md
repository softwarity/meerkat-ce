---
title: Votre première route
section: Demarrer
order: 3
summary: Mettre un de vos services derrière la gateway, depuis la console, et vérifier que c'est bien la bonne route qui répond.
---

# Votre première route

Une route répond à une question - *est-ce que cette requête est pour moi* - puis
elle dit quoi en faire. Cette page en crée une qui proxifie un service interne,
et la vérifie avant de laisser quiconque s'en approcher.

Disons que le service est une application de facturation, joignable dans votre
cluster à `billing:8080`, et que vous voulez la servir sur `/billing`.

## Ouvrir l'éditeur

Dans la console, allez sur **Infra > Routes** et cliquez **New route**.
L'éditeur s'ouvre dans un tiroir, avec les sections d'une route sur la gauche.
Deux portent une étoile, ce qui veut dire que la route ne peut pas être
enregistrée sans elles : **Target** et **Predicates**.

Nommez la route en haut - `billing` fera l'affaire. Le nom est ce que vous
chercherez dans la table, dans les courbes de trafic et dans le journal
d'audit.

![L'éditeur de route, ouvert sur Target](img/console/route-editor-target.webp)

## Target : ce qui répond

**Target** décide comment la route répond. Quatre modes, et seul le premier
appelle un service :

| Mode | Ce qu'il fait |
|---|---|
| Proxy | va chercher chez un amont et rend ce qu'il a dit |
| Redirect | envoie le navigateur ailleurs |
| Maintenance | sert la page d'indisponibilité intégrée |
| Respond | répond depuis un gabarit, sans rien appeler |

Gardez **Proxy** et saisissez l'amont : `http://billing:8080`. Le `http` est le
défaut à dessein : dans un cluster, TLS s'arrête en général sur la gateway et le
saut jusqu'au service est en clair. `https` et `h2c` sont les deux autres choix.

> [!TIP]
> Là où la gateway peut lire son environnement d'exécution - un socket Docker ou
> Swarm, ou un namespace Kubernetes depuis l'intérieur - les services trouvés
> sont proposés dans ce champ au moment où vous le sélectionnez. La saisie libre
> reste libre : un amont hors du cluster se tape.

La même section porte le temps que cet amont a le droit de prendre (connexion,
puis première réponse) et s'il faut cesser de l'appeler quand il ne répond plus.
Les deux héritent de l'installation tant que vous ne dites rien : laissez-les
tranquilles pour l'instant.

## Predicates : quelles requêtes

**Predicates** est la moitié *est-ce pour moi*. Ajoutez un prédicat **path** avec
le motif `/billing/**`.

Une route peut porter plusieurs prédicats, et ils se combinent en ET : un chemin,
plus un hôte, plus une méthode, c'est une route qui veut les trois. Les prédicats
disponibles aujourd'hui sont path, host, header, cookie, method, query,
remote-addr, x-forwarded-remote-addr, time-window, version et weight.

![Trois prédicats sur une route](img/console/route-editor-predicates.webp)

## Retirer le préfixe

La gateway a reconnu `/billing/**`, mais l'application de facturation ne sait
rien de ce préfixe - elle sert `/invoices`, pas `/billing/invoices`. Ouvrez
**Incoming** et ajoutez un filtre **strip-prefix** avec `1` partie.

Les filtres entrants sont les transformations côté requête : en-têtes,
paramètres de requête, chemin, hôte. **Outgoing** fait la même chose au retour.

![Les filtres entrants d'une route](img/console/route-editor-filters.webp)

## Qui a le droit de passer

**Security** porte la règle d'accès de la route. Laissez-la vide et la route est
ouverte : la gateway proxifie sans demander qui appelle, et le service décide
lui-même. Exigez un compte connecté, des rôles nommés ou des comptes nommés, et
un appelant qui ne remplit pas la condition n'atteint jamais l'amont.

Le détail est dans [Contrôle d'accès](/#/docs/concepts/access-control).

![La section sécurité d'une route](img/console/route-editor-security.webp)

## Enregistrer

**Save** applique tout de suite : la table de routage est recompilée et, en
cluster, les autres noeuds sont avertis. Il n'y a rien à redémarrer et aucun
fichier à recharger.

Une route nouvellement créée reçoit l'ordre `0`, ce qui la place **en haut** de
la table. Cela compte : la première route dont les prédicats reconnaissent la
requête est celle qui répond, donc une nouvelle route est essayée avant tout ce
qui existe déjà.

## Vérifier avant vos utilisateurs

Deux moyens, et le premier d'abord.

**Routing test**, sur l'écran des routes, compose une requête qui n'est jamais
envoyée : méthode, chemin, hôte, en-têtes, cookies, adresse du client, horloge,
et une identité au nom de laquelle être jugé. Il rend ensuite, route par route,
laquelle prend la requête et pourquoi les autres ne l'ont pas prise - pas
reconnue, reconnue mais la règle a écarté cet appelant, ou pas évaluée parce
qu'une route au-dessus a répondu.

Puis pour de vrai :

```bash
curl -i http://localhost:8080/billing/invoices
```

Si rien ne correspond, la gateway répond 404. Si une route a reconnu la requête
mais que sa règle vous a écarté, une route UI atterrit sur une page qui nomme la
règle en cause ; une route de service reçoit un 403 sec.

## Ensuite

- Ce qu'une route contient, en entier : [Routes](/#/docs/concepts/routes)
- Ce que la gateway ajoute à une page proxifiée : [Ce que la gateway injecte](/#/docs/concepts/data-plane-chrome)
