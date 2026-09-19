---
title: Sessions
section: Authentification
order: 114
summary: Ce qu'est le cookie de session, combien de temps il vit, et ce qui met fin à une session - et ce qui n'y met pas fin.
---

# Sessions

Une session de navigateur est un **cookie opaque**. Il ne porte ni identité, ni
claims, ni signature : trente-deux octets aléatoires, dont la gateway ne garde
qu'une empreinte. Tout ce qui décrit la session - à qui elle appartient, quelle
organisation est active, quelle étape de connexion reste due - vit côté serveur,
dans une ligne.

C'est délibéré. Les JWT sont pour le chemin d'API et pour ce qui est transmis à
l'amont, jamais pour le navigateur : un jeton dans un cookie ne se retire pas,
une ligne si.

## Les cookies

Une session par plan, parce que les cookies ne sont pas cantonnés par port : sur
un hôte qui sert les deux ports, un seul nom ferait partager une même session à
l'application et à la console.

| Cookie | Plan | Ce qu'il porte |
|---|---|---|
| `MEERKAT_SESSION` | plan de données | la valeur de session |
| `MEERKAT_ADMIN_SESSION` | plan de contrôle | la valeur de session de la console |
| `MEERKAT_UNTIL` | plan de données | quand cette session expire, en horodatage |
| `MEERKAT_ADMIN_UNTIL` | plan de contrôle | la même chose, pour la console |

Les cookies de session sont `HttpOnly`, `SameSite=Lax`, `Path=/`, avec un
`Max-Age` égal à la durée de vie de la session. `Secure` est posé quand la requête
est arrivée en HTTPS - soit TLS terminé par la gateway, soit
`X-Forwarded-Proto: https` envoyé par le proxy devant elle.

Les deux cookies `..._UNTIL` ne sont délibérément **pas** `HttpOnly` : une page les
lit pour s'apercevoir que la session s'est terminée, ou est revenue dans un autre
onglet, sans interroger un point d'entrée. Ils portent une échéance et rien
d'autre.

> [!NOTE]
> Aucun attribut `Domain` n'est posé : le cookie est lié à l'hôte. Une session
> n'est donc **pas partagée entre sous-domaines** aujourd'hui - `app.acme.io` et
> `admin.acme.io` se connectent séparément. FEATURES.md liste cela comme la moitié
> manquante du cookie de session.

Une session d'un plan n'est jamais acceptée sur l'autre. La réponse à un cookie du
plan de données sur le port d'administration n'est pas "interdit", c'est "pas de
session".

## Combien de temps vit une session

La durée est un **temps d'inactivité, pas un temps total** : chaque requête repousse
l'échéance à *maintenant plus la durée*. Pour que ce soit bon marché, la nouvelle
échéance n'est écrite en base qu'une fois la session dans sa seconde moitié : une
session de trente minutes écrit au plus toutes les quinze minutes.

Une déconnexion, ou une révocation sur un autre noeud, ne peut pas être annulée par
une requête déjà en vol : le prolongement ne s'applique que si l'échéance dont
elle a été informée est toujours celle qui est stockée.

La durée elle-même se résout depuis le niveau le plus fin qui dit quelque chose :

| Niveau | Où il vit | Modifiable |
|---|---|---|
| Appartenance, une personne dans une organisation | `sessionTTL` sur l'appartenance | API d'administration seulement |
| Organisation | `sessionTTL` sur l'organisation | API d'administration seulement |
| Installation | réglage `session_ttl`, défaut `PT30M` | **Application > Security** |

Les valeurs sont des durées ISO-8601 : `PT15M`, `PT1H`, `P1D`. Jours, heures,
minutes et secondes seulement - semaines, mois et années sont refusés. La console
propose une liste de quinze minutes à un jour, et conserve une valeur posée
ailleurs plutôt que de la perdre.

> [!WARNING]
> Deux aspérités sur les deux niveaux du bas, à connaître avant de s'en servir. Une
> durée invalide n'est **pas** contrôlée à l'écriture : elle est acceptée puis
> retombe silencieusement à trente minutes à la connexion suivante. Et l'écran des
> membres de la console envoie une durée d'appartenance vide dès que vous cochez une
> appartenance ou un groupe, ce qui **efface** une durée par membre posée par l'API.

Une session qui doit encore une étape de connexion - un mot de passe à changer, un
code à saisir - utilise la durée de l'installation, parce que l'organisation n'est
pas encore connue.

## Ce qui met fin à une session

| Cause | Portée | Immédiat |
|---|---|---|
| `POST /logout` | cette session-là | oui, la ligne est supprimée |
| Expiration par inactivité | cette session | oui, refusée sur l'horloge |
| Une réinitialisation de mot de passe par le lien envoyé par courriel | **toutes** les sessions de ce compte, sur toutes les passerelles | oui |
| La suppression du compte | toutes les sessions de ce compte | oui |

La déconnexion est un `POST` - il n'y a pas de `GET /logout` - et elle supprime la
ligne plutôt que d'effacer seulement le cookie, puis efface les deux cookies et
dit aux autres passerelles de l'oublier. Elle termine **cette** session : les
autres navigateurs de la même personne gardent la leur, et la console et
l'application sont indépendantes.

## Ce qui ne met pas fin à une session

Cette liste compte plus que la précédente :

- **Désactiver un compte ne termine pas ses sessions.** La ligne reste. Ce qui se passe, c'est que chaque garde relit le compte : la personne désactivée cesse de passer les règles de route et cesse d'être admise par l'API d'administration. Mais les pages propres à la gateway - `/profile` et les autres - continuent de lui répondre jusqu'à l'expiration.
- **Changer un mot de passe non plus.** Ni le changement volontaire dans le profil, ni le changement forcé à la connexion, ni la réinitialisation par un administrateur. Seul le lien de réinitialisation par courriel révoque les sessions, parce que c'est le flux qui existe pour un compte que quelqu'un d'autre tient peut-être.
- **"Mot de passe à changer" non plus.** Le drapeau est lu à la connexion *suivante*.
- **Les changements de rôle, de groupe et d'appartenance non plus.** Ils prennent effet en quelques secondes : l'identité mémorisée est jetée, la session est gardée, ce qui est précisément pourquoi être ajouté à une organisation prend effet sans se déconnecter. Retirer un rôle marche pareil.
- **Désactiver une organisation non plus.**
- **Sur le plan de contrôle, la fenêtre de validité d'un compte n'est pas revérifiée** sur une session cookie : elle l'est à la connexion et à chaque appel par jeton, mais une session de console déjà ouverte court jusqu'à son expiration.

## Ce qui manque

Il n'y a **aucune liste des sessions actives** et **aucun "me déconnecter
partout"** - ni pour une personne sur son propre compte, ni pour un
administrateur. La table existe et la révocation par compte existe dans le code ;
ce qui manque est un écran, un bouton, et un appel à cette révocation quand un
compte est désactivé.

D'ici là, la façon honnête de sortir quelqu'un de partout est de réinitialiser son
mot de passe par le flux courriel, ou de supprimer le compte.
