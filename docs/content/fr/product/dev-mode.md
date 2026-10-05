---
title: Mode développement
section: Le produit
order: 7
summary: Le poste d'un développeur rejoint le cluster et prend la place d'un service déployé, et tous ceux qui regardent l'application savent lequel, et qui le remplace.
---

# Mode développement

Toutes les équipes qui développent une application interne connaissent le
problème : le cluster contient tous les services **et toutes les données**, et
cet environnement est difficile, parfois interdit, à reproduire sur la machine
d'un développeur. La réponse de Meerkat n'est pas de dupliquer l'environnement,
mais de faire entrer le poste de travail **dans le maillage de routage**, comme
un invité dont les droits se limitent au routage, grâce à
[plug](https://github.com/softwarity/plug).

## Comment cela marche

Un administrateur marque un utilisateur comme `dev`. Le développeur dépose sa
**clé SSH publique** sur son compte Meerkat (Profil, Développeur, clé), puis
lance son service en local à travers plug :

```bash
plug -p <gateway-host> -s user-mng-service:8080:3000 npm run start
```

- **Du poste vers le cluster** : le processus local résout les noms des
  services du cluster et les atteint comme s'il tournait à l'intérieur - c'est
  le comportement d'origine de plug.
- **Du cluster vers le poste** : `-s` déclare une **substitution**. Le trafic
  destiné à `user-mng-service` emprunte le tunnel inverse jusqu'au processus
  local, qui est vu, en entrée comme en sortie, comme un membre à part entière
  du cluster. Toutes les routes qui désignent ce service suivent
  automatiquement.
- **Elle dure le temps de la session** : la substitution disparaît quand le
  processus s'arrête, et un redémarrage de la gateway ferme les deux
  extrémités.

Pour l'installer selon votre système et ouvrir le tunnel :
[Plug](/docs/operations/plug).

## Retirer un accès

La clé est attachée au compte du développeur, et la gateway la vérifie à
chaque connexion. La retirer, ou retirer le mode développement, ferme le tunnel
en quelques secondes : rien de ce qui a été émis ne survit à ce retrait, il n'y
a donc ni expiration à attendre, ni révocation à propager ailleurs. La page de
profil affiche l'empreinte SHA256 telle qu'OpenSSH l'imprime lui-même.

## Tout le monde voit qui sert quoi

Une substitution change la nature même de l'application que vous avez sous les
yeux. Ce n'est donc pas une préférence de développeur, mais une information
pour tous ceux qui la regardent, et elle porte un nom : "checkout, par Alice"
plutôt que "checkout, par quelqu'un".

- **À la connexion**, une page indique ce qui est substitué et par qui, avant
  de donner accès à l'application.
- **Pendant que vous travaillez**, un bandeau repliable le rappelle sur chaque
  page, et suit en direct les substitutions qui apparaissent et disparaissent.

> [!NOTE]
> Le tunnel intégré à la gateway relève d'Enterprise, et c'est lui qui rend
> cette attribution possible : chaque développeur dépose sa propre clé, et une
> connexion dit donc qui se connecte. L'image Community fait tourner plug comme
> un agent autonome à côté de la gateway, avec une clé partagée : une
> connexion prouve alors que l'appelant *possède* plug, pas qui il est. Voir
> [Une gateway](/docs/deploy/one-gateway) pour le fichier compose et les
> valeurs Helm qui l'activent.

## Le Swagger des développeurs, servi par la gateway

Une gateway voit passer toutes les API de l'installation, et connaît les
spécifications OpenAPI que déclarent les routes. Meerkat en fait donc une page,
à l'adresse `/meerkat/apidocs` sur le plan de données, ouverte aux comptes qui
ont la capacité `dev` et à personne d'autre.

Ce n'est pas un Swagger UI de plus, posé à côté d'un service :

- **Toutes les API au même endroit.** La page liste les spécifications de
  **toutes** les routes, y compris celles qui sont désactivées - il est
  légitime qu'un développeur voie ce qui existe et ce qui est en construction.
- **Rien à installer, rien qui vienne d'un CDN.** Les fichiers de swagger-ui
  sont dans le binaire. Une installation coupée d'internet dispose de la même
  page que les autres, et aucune requête ne part d'un navigateur d'entreprise
  vers un tiers.
- **Les appels passent par la gateway**, donc par l'authentification, les
  règles d'accès et les modificateurs de la route. Ce que montre la page est ce
  que le service répondra en production, pas ce qu'il répondrait si on
  l'appelait directement.
- **Et surtout, l'identité est simulée.** La barre du haut impose l'identité
  utilisée par *Try it out* selon trois modes exclusifs - un **utilisateur**,
  des **groupes**, des **rôles** - et affiche l'identité effective qui en
  résulte. C'est la vraie question que se pose un développeur devant une API
  protégée : "que répond-elle à quelqu'un qui n'a que ce rôle ?". Sans cela, il
  faut un compte de test par cas, et chacun finit par tester avec le sien.

Le plan de contrôle a sa propre page, à l'adresse `/apidocs`, qui documente
l'API d'administration de Meerkat - celle que pilotent la console, la CLI et
l'[agent](/docs/agent/overview).

## Ce qui reste à venir

La **portée** d'une substitution : aujourd'hui, elle vaut pour tout le trafic,
et rien ne permet encore de la limiter aux personnes qui l'ont demandée. Côté
exploitation, il manque l'écran de la console qui liste les sessions en cours,
et l'entrée d'audit pour chaque substitution faite. Une clé déposée, elle, est
déjà une ligne du [journal d'audit](/docs/operations/audit) (`devkey.add`, avec
son empreinte).
