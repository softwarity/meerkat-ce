---
title: Routes
section: Concepts
order: 21
summary: Une route répond à deux questions - est-ce pour moi, et qu'est-ce que j'en fais - et l'ordre dans lequel elles sont essayées décide de tout.
---

# Routes

La route est l'unité de configuration de Meerkat. Elle porte tout : quelles
requêtes elle veut, ce qu'il faut leur faire, où elles vont, qui a le droit de
passer, combien de temps l'amont peut prendre, combien elle accepte de porter. Il
n'y a pas d'entité service à créer avant - une route avec une URL d'amont est une
route complète.

Les routes vivent en base et s'éditent depuis la console. En enregistrer une
l'applique tout de suite.

![L'écran des routes](img/console/routes-list.webp)

## Les quatre parties

**Les prédicats** répondent à *est-ce que cette requête est pour moi*. C'est la
moitié qui reconnaît, et ils se combinent en **ET** : une route avec un prédicat
de chemin et un prédicat d'hôte veut les deux.

**Les filtres** transforment. Un filtre de requête réécrit ce qui part vers
l'amont, un filtre de réponse réécrit ce qui revient, un garde-fou refuse la
requête d'emblée, et un filtre terminal répond au lieu de proxifier.

**L'amont** est là où la requête va - `http://`, `https://`, ou `h2c://` pour
HTTP/2 en clair, qui est ce que parle un service gRPC dans un cluster.

**L'accès** est la règle de la route : qui a le droit de passer. Vide, la route ne
demande rien et l'amont décide lui-même.

## Sur quoi une route peut reconnaître une requête

| Prédicat | Reconnaît sur |
|---|---|
| `path` | le chemin de la requête, avec les motifs `*` et `**` |
| `host` | l'en-tête Host |
| `method` | le verbe HTTP |
| `header`, `cookie`, `query` | une valeur nommée, présente ou égale à quelque chose |
| `remote-addr` | l'adresse du pair, par CIDR |
| `x-forwarded-remote-addr` | la même, lue dans l'en-tête de transmission |
| `time-window` | une période, pour une route qui n'existe qu'à certaines heures |
| `version` | une plage de version d'API, lue dans un en-tête, un paramètre ou le chemin |
| `weight` | une part du trafic, tirée à chaque requête, pour une mise en canari |

> [!NOTE]
> Un prédicat de langue, l'exclusion par expression régulière et une option sur
> le slash final sont décrits mais pas écrits. La liste ci-dessus est celle qui
> existe.

## Ce qu'une route peut faire à une requête

Les filtres viennent par phases, et la console les regroupe comme ils
s'exécutent :

- **Gates** refusent avant tout le reste : `max-request-body`, `max-request-headers`.
- **Incoming** réécrit la requête : `strip-prefix`, `prefix-path`, `rewrite-path`, `set-path`, `set-host`, `preserve-host`, et toute la famille des opérations sur les en-têtes, les paramètres de requête et les cookies - poser, ajouter, retirer, renommer, copier, réécrire.
- **Outgoing** réécrit la réponse : la même famille d'en-têtes, plus `set-status`, `cache-control`, `security-headers`, `cookie-attributes`, `dedupe-response-header`, `rewrite-location`, `remove-json-fields`.
- Les filtres **terminaux** font que la route répond elle-même : `redirect`, `maintenance`, `respond`.

Une route qui porte un filtre terminal ne proxifie rien : ses filtres entrants et
sa transmission d'identité sont donc abandonnés - la console désactive ces
sections plutôt que de vous laisser écrire des réglages qui seront jetés.

## L'ordre, et pourquoi c'est toute l'histoire

Les routes sont essayées par **ordre croissant**, et la **première qui reconnaît
la requête répond**. Les égalités sont tranchées par le nom, donc une même
configuration compile toujours la même table.

Deux choses surprennent dans cet ordre, et les deux sont délibérées.

**La sécurité fait partie de la reconnaissance.** Deux routes peuvent couvrir les
mêmes chemins et ne différer que par les personnes à qui elles sont destinées. Un
appelant que la règle écarte **retombe sur la route suivante qui reconnaît la
requête**. C'est ce qui permet à une route `/reports/**` réservée aux rôles de la
finance de se placer au-dessus d'une route `/reports/**` ouverte à tous.

**Une règle `deny` ne retombe jamais.** La seule règle écrite pour fermer un
chemin le ferme. Autrement, ce serait une redirection.

Si toutes les routes qui reconnaissent la requête ont écarté l'appelant, c'est la
**première** qui l'a fait qui produit le refus - avec le motif qu'elle connaît, et
le changement d'organisation qu'elle peut proposer. Un 404 sec, là, serait la
seule réponse sur laquelle personne ne peut agir.

Si rien n'a reconnu la requête, la gateway répond 404 et le compte à l'échelle de
la gateway - un nombre de requêtes sans route qui monte est une erreur de
configuration que quelqu'un devrait voir.

## L'attrape-tout

Une installation neuve a une route nommée `trap` : un prédicat de chemin `/**`
ordonné en dernier. C'est une route ordinaire, sans privilège - elle est
simplement essayée après toutes les autres, donc elle attrape ce qui n'a pas été
reconnu, `/` compris.

C'est ce qui fait qu'une gateway neuve répond quelque chose sur tous les chemins.
Supprimez-la, ou faites-la pointer sur votre propre page d'accueil, quand vous
aurez vos routes.

> [!TIP]
> Une route nouvellement créée reçoit l'ordre `0`, ce qui la place **en haut** de
> la table - elle est essayée avant tout ce qui existe déjà. Faites glisser les
> lignes de l'écran des routes pour changer cela.

## Deux genres de route

Toute route est une route de service. **UI** est un drapeau posé par-dessus, et il
débloque les options qui n'ont de sens que pour quelque chose qu'un navigateur
affiche : le bouton utilisateur injecté, le schéma de couleurs, l'identité
estampillée sur la page. Un refus sur une route UI atterrit sur une page ; sur une
route de service il reste un 403, parce que personne ne lit un 403 dans un
navigateur.

Voir [Ce que la gateway injecte](/docs/concepts/data-plane-chrome).

## Ce qu'une route porte encore

| | Quoi, et sur quoi cela retombe |
|---|---|
| Délais | connexion et première réponse, par route, sinon l'installation, sinon 5 s / 15 s. Le corps n'est jamais borné : un téléchargement ou un websocket déjà commencé vit aussi longtemps qu'il faut |
| Disjoncteur | éteint par défaut. Après N échecs consécutifs la route cesse d'appeler et sert la page d'indisponibilité, en laissant passer une seule requête après le refroidissement. Un 500 ne compte pas - un service qui répond 500 est debout et a un bug |
| Limites de débit | plusieurs à la fois, chacune clée sur autre chose - la route, un utilisateur, un jeton, une organisation, une adresse. La première dépassée répond |
| Transmission d'identité | ce que l'amont apprend de l'appelant - en-têtes, ou JWT signé |
| Langues | l'offre de langues de la route, et la façon dont la langue voyage vers l'amont |
| Spec OpenAPI | publiée par le service ou déposée ici ; c'est sur elle que se posent les règles par endpoint |

## Tester avant le trafic

Le **Routing test** de l'écran des routes compose une requête qui n'est jamais
envoyée - méthode, chemin, hôte, en-têtes, cookies, adresse du client, horloge, et
une identité au nom de laquelle être jugé - et rend route par route laquelle la
prend. Les réponses qu'il donne sont les quatre qui comptent : prend cette
requête, écartée par les prédicats, reconnue mais la règle a écarté cet appelant,
ou pas évaluée parce qu'une route au-dessus a répondu.
