---
title: Sessions
section: Authentification
order: 114
summary: Ce qu'est le cookie de session, combien de temps il vit, ce qui met précisément fin à une session - et ce qui n'y met pas fin.
---

# Sessions

Une session de navigateur est un **cookie opaque**. Il ne contient ni identité, ni claims,
ni signature : trente-deux octets aléatoires, dont la gateway ne conserve qu'une
empreinte. Tout ce qui concerne la session - à qui elle appartient, quelle organisation est
active, quelle étape de connexion reste à franchir - se trouve côté serveur, dans une ligne
de la base.

C'est voulu. Les JWT servent aux appels d'API et à ce qui est transmis à l'upstream, jamais au
navigateur : on ne peut pas retirer un jeton placé dans un cookie, alors qu'on peut
supprimer une ligne.

## Les cookies

Il y a une session par plan, parce que la portée d'un cookie ne tient pas compte du port :
sur un hôte qui sert les deux ports, un nom unique ferait partager la même session à
l'application et à la console.

| Cookie | Plan | Ce qu'il contient |
|---|---|---|
| `MEERKAT_SESSION` | plan de données | la valeur de la session |
| `MEERKAT_ADMIN_SESSION` | plan de contrôle | la valeur de la session de la console |
| `MEERKAT_UNTIL` | plan de données | le moment où cette session expire, sous forme d'horodatage |
| `MEERKAT_ADMIN_UNTIL` | plan de contrôle | la même chose, pour la console |

Chaque nom se termine par l'identifiant de l'installation - `MEERKAT_SESSION_3fa9c1e2` -
généré à l'installation et jamais exporté avec une configuration. C'est la même raison, un
niveau plus haut : deux gateways qu'un navigateur atteint sous le même nom d'hôte, par
exemple une édition Enterprise et une édition Community côte à côte sur `localhost`,
écraseraient sinon mutuellement leurs sessions. Le cookie des navigateurs de confiance
porte le même suffixe ; les cookies de langue et de mode d'affichage relèvent du choix de
la personne et restent partagés.

En HTTPS, les noms commencent par `__Host-` (`__Host-MEERKAT_ADMIN_SESSION_...`). Un
navigateur interdit à une page servie en clair d'écraser un cookie `Secure` du même nom :
avec un seul nom pour les deux schémas, il devenait impossible de se connecter en HTTP
après une session en HTTPS - or l'accès en clair à la console existe justement pour le jour
où un certificat fait défaut. Ce préfixe oblige aussi le navigateur à garantir que le
cookie a été posé en HTTPS, par cet hôte, pour tout le site.

Les cookies de session sont `HttpOnly`, `SameSite=Lax`, `Path=/`, avec un `Max-Age` égal à
la durée de vie de la session. `Secure` est ajouté quand la requête est arrivée en HTTPS -
soit que la gateway termine elle-même TLS, soit que le proxy placé devant elle envoie
`X-Forwarded-Proto: https`.

Les deux cookies `..._UNTIL` ne sont volontairement **pas** `HttpOnly` : une page les lit
pour s'apercevoir que la session a pris fin, ou qu'elle a été rétablie dans un autre
onglet, sans interroger régulièrement un endpoint. Ils contiennent une échéance et rien
d'autre.

> [!NOTE]
> Aucun attribut `Domain` n'est défini : le cookie est donc limité à l'hôte, et c'est un
> choix. Un `Domain` enverrait la session à **tous** les sous-domaines, y compris ceux que
> Meerkat ne sert pas - un site marketing hébergé ailleurs, un SaaS derrière un CNAME, un
> hôte de préproduction - et il suffirait que l'un d'eux soit compromis pour qu'il reçoive
> les sessions de tout le monde. On se connecte donc séparément à `app.acme.io` et à
> `admin.acme.io` ; les applications servies sous un même nom d'hôte partagent la session.

Une session d'un plan n'est jamais acceptée sur l'autre. À un cookie du plan de données
présenté sur le port d'administration, la réponse n'est pas "interdit" mais "pas de
session".

## Combien de temps vit une session

La durée de vie est un **temps d'inactivité, pas une durée totale** : chaque requête
repousse l'échéance à *maintenant plus la durée de vie*. Pour que cela reste peu coûteux,
la nouvelle échéance n'est écrite en base qu'une fois la session entrée dans la seconde
moitié de sa durée de vie : une session de trente minutes écrit donc au plus toutes les
quinze minutes.

Une déconnexion, ou une révocation sur un autre nœud, ne peut pas être annulée par une
requête qui était déjà en cours : la prolongation ne s'applique que si l'échéance dont la
requête avait connaissance est encore celle qui est enregistrée.

La durée de vie elle-même est prise au niveau le plus précis qui en définit une :

| Niveau | Où elle se trouve | Modifiable |
|---|---|---|
| L'appartenance, une personne dans une organisation | `sessionTTL` sur l'appartenance | API d'administration uniquement |
| L'organisation | `sessionTTL` sur l'organisation | API d'administration uniquement |
| L'installation | réglage `session_ttl`, `PT30M` par défaut | **Application > Security** |

Les valeurs sont des durées ISO-8601 : `PT15M`, `PT1H`, `P1D`. Seuls les jours, les heures,
les minutes et les secondes sont admis - les semaines, les mois et les années sont refusés.
La console propose une liste allant de quinze minutes à un jour, et conserve une valeur
définie ailleurs au lieu de la perdre.

> [!WARNING]
> Les deux niveaux les plus précis présentent deux défauts qu'il vaut mieux connaître avant
> de s'en servir. Une durée invalide n'est **pas** contrôlée à l'écriture : elle est
> acceptée, puis remplacée sans avertissement par trente minutes à la connexion suivante.
> Et l'écran des membres de la console envoie une durée d'appartenance vide chaque fois que
> vous cochez une appartenance ou un groupe, ce qui **efface** une durée par membre définie
> par l'API.

Une session à laquelle il reste une étape de connexion à franchir - un mot de passe à
changer, un code à saisir - utilise la durée de vie de l'installation, car l'organisation
n'est pas encore connue.

## Ce qui met fin à une session

| Cause | Portée | Immédiat |
|---|---|---|
| `POST /logout` | cette session uniquement | oui, la ligne est supprimée |
| L'expiration pour inactivité | cette session | oui, elle est refusée d'après l'heure réelle |
| Une réinitialisation du mot de passe par le lien reçu par e-mail | **toutes** les sessions du compte, sur toutes les gateways | oui |
| La suppression du compte | toutes les sessions du compte | oui |
| La **désactivation** du compte | toutes les sessions du compte, sur les deux plans | oui |
| **Déconnecter** dans les *Sessions actives* du profil, ou *Déconnecter partout ailleurs* | cette session, ou toutes les autres sessions de la personne | oui |
| **Sign out** sur l'écran *Sessions* de la console | cette session | oui |

La déconnexion est un `POST` - il n'y a pas de `GET /logout`. Elle supprime la ligne au
lieu de se contenter d'effacer le cookie, puis efface les deux cookies et demande aux
autres gateways d'oublier la session. Elle met fin à **cette** session : les autres
navigateurs de la même personne gardent la leur, et la console et l'application sont
indépendantes.

## Ce qui ne met pas fin à une session

Cette liste compte davantage que la précédente :

- **Changer de mot de passe n'y met pas fin.** Ni le changement volontaire depuis le profil, ni le changement imposé à la connexion, ni la réinitialisation par un administrateur. Seul le lien de réinitialisation reçu par e-mail révoque les sessions - ainsi que les jetons d'API et les navigateurs de confiance du compte - parce que ce parcours existe précisément pour un compte que quelqu'un d'autre détient peut-être.
- **L'obligation de changer de mot de passe non plus.** L'indicateur est lu à la connexion *suivante*.
- **Les changements de rôle, de groupe et d'appartenance non plus.** Ils prennent effet en quelques secondes : l'identité mémorisée est abandonnée et la session conservée. C'est pourquoi un ajout à une organisation peut prendre effet sans déconnexion. Le retrait d'un rôle fonctionne de la même façon.
- **La désactivation d'une organisation non plus.**
- **Sur le plan de contrôle, la période de validité d'un compte n'est pas contrôlée de nouveau** pour une session par cookie : elle l'est à la connexion et à chaque appel par jeton, mais une session de console déjà ouverte se poursuit jusqu'à son expiration.

## Voir les sessions

Chacun consulte les siennes dans son profil, **Sécurité, Sessions actives** : chaque
navigateur connecté aux applications, depuis quand et depuis quelle adresse, avec la
mention *Ce navigateur* sur celui en cours ; **Déconnecter** sur n'importe quel autre, ou
**Déconnecter partout ailleurs**. Personne ne peut fermer la session d'un autre compte,
quel que soit l'identifiant que transporte un formulaire.

Les administrateurs les consultent dans **Sessions**, dans la barre latérale : qui, quel
navigateur et quelle adresse, quel plan, depuis quand - avec un filtre par compte et une
pagination. Chacun voit son propre périmètre. Root voit les deux plans. Un administrateur
des applications voit les sessions des applications, toutes organisations confondues, mais
pas celles de la console : savoir qui utilise la console, et depuis où, ne regarde que
root. L'administrateur d'une organisation voit les sessions ouvertes dans les organisations
qu'il administre, et ne peut mettre fin qu'à celles-là. Mettre fin à une session écrit
`session.revoke` dans le [journal d'audit](/docs/operations/audit).

![Sessions : qui est connecté, depuis quel navigateur et quelle adresse, sur quel plan, et depuis quand](img/console/sessions.webp)
