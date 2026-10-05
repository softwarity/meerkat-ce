---
title: Coffre
section: La console
order: 180
summary: Toutes les valeurs et tous les secrets nommés auxquels la configuration fait référence, sous la forme $name.
---

# Coffre

L'entrée **Vault** du rail rassemble, en un seul endroit, toutes les valeurs auxquelles
la configuration fait référence. Deux types d'entrée partagent le même espace de noms :

| Type | Relecture | Usage |
|---|---|---|
| **Secret** | Jamais. Il est chiffré au repos, et l'API indique qu'il est défini, pas ce qu'il contient | Un secret client, un mot de passe SMTP, une clé HMAC |
| **Valeur** | Oui, en clair | Une URL, un DN de base, un identifiant client - tout ce qui n'est pas secret mais change d'un environnement à l'autre |

Les deux se référencent de la même manière, par **`$name`**, partout où un champ accepte
une référence. C'est tout l'intérêt : transformer une valeur en secret ne change rien à
ce qui pointe dessus.

![L'écran Vault : trois entrées - deux secrets et une valeur - avec leur type, leur valeur et ce qui les utilise](img/console/vault.webp)

Deux secrets, qui affichent *encrypted, never shown*, une valeur lisible en clair, et la
colonne *Used by*, qui nomme la route pointant sur chaque entrée.

## La liste

Le nom et la description, le type, la valeur (ou *encrypted, never shown*) et **Used
by**, la colonne qui permet de distinguer une entrée utilisée d'un reliquat. Un clic sur
une ligne ouvre l'éditeur ; le + crée une entrée.

Une entrée comprend :

- **Name** - ce que désignera `$name`.
- **Kind** - valeur ou secret, à choisir avec un bouton à bascule.
- **Value** - saisie une seule fois dans le cas d'un secret.
- **Description** - pour la personne qui retrouvera l'entrée dans six mois.
- **Reminder date** - le jour où le secret expire chez celui qui l'a émis. **Ce n'est
  qu'un rappel** : la gateway ne peut pas savoir qu'un jeton a été renouvelé chez le
  fournisseur, et `$name` continue donc d'être résolu. Cette date alimente le
  [récapitulatif quotidien](/docs/console/mail-relay), qui liste les échéances proches
  et celles qui viennent de passer - jamais la valeur.

## D'où viennent les entrées

La plupart sont créées depuis le champ qui en a besoin. Un champ sensible propose
**Move into the vault** et refuse d'être enregistré sous forme littérale :
l'administrateur détient la valeur, c'est donc à lui de la ranger. Un champ qui accepte
une valeur en clair propose la même chose pour les valeurs.

L'ordre habituel est donc l'inverse de celui que vous pourriez attendre : vous
renseignez un relais de messagerie, une autorité ou un certificat, et l'entrée du coffre
en découle.

## Le coffre sous forme de fichier

**Export** écrit le coffre dans un seul fichier chiffré, protégé par une phrase secrète
que Meerkat ne conserve pas. **Import** relit un tel fichier. C'est la moitié qu'un
export de configuration ne contient jamais, et c'est elle qui permet d'amorcer un second
environnement ou de déménager une gateway.

> [!WARNING]
> Un coffre exporté ne vaut que ce que vaut sa phrase secrète, et conserver les deux au
> même endroit annule le chiffrement. Il en va de même d'un
> [snapshot](/docs/console/configuration) dont la clé maîtresse est stockée à côté de
> la base de données.

## Ce que vous pouvez voir

Le coffre se restreint de lui-même au périmètre de l'appelant : un administrateur infra
voit les entrées de l'infra, un administrateur applicatif celles de l'application.
L'écran est ouvert à quiconque administre un plan qui contient des entrées.

## Pièges

- **Une référence est publique, un littéral ne l'est jamais.** `$stripe_key` peut
  apparaître dans un export, un diff ou une capture d'écran sans conséquence. C'est
  toute la raison d'être du coffre.
- **Un secret ne peut pas être relu**, ni par vous ni par l'API. Si vous le perdez, il
  faut le remplacer à la source.
- **Un import de configuration peut laisser des manques** : des entrées auxquelles le
  fichier fait référence et que cette gateway n'a pas. L'import en dresse la liste
  pour que vous puissiez les renseigner immédiatement.
- **Supprimer une entrée utilisée** casse tout ce qui pointait dessus. Consultez
  d'abord la colonne *Used by*.
