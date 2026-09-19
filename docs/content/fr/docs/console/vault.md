---
title: Coffre
section: La console
order: 180
summary: Toutes les valeurs et les secrets que la configuration désigne, référencés par $nom.
---

# Coffre

L'entrée **Vault** du rail porte toutes les valeurs auxquelles la configuration se réfère,
en un seul endroit. Deux genres vivent dans le même espace de noms :

| Genre | Relu | Pour |
|---|---|---|
| **Secret** | Jamais. Chiffré au repos, et l'API dit qu'il est posé, pas ce qu'il vaut | Un secret client, un mot de passe SMTP, une clé HMAC |
| **Valeur** | Oui, en clair | Une URL, un base DN, un client id - tout ce qui n'est pas secret mais change d'un environnement à l'autre |

Les deux se référencent pareil, **`$nom`**, partout où un champ l'accepte. C'est tout
l'intérêt : promouvoir une valeur en secret ne touche jamais à ce qui pointe dessus.

![L'écran Vault : trois entrées - deux secrets et une valeur - avec leur genre, leur valeur et ce qui les utilise](img/console/vault.webp)

Deux secrets qui affichent *encrypted, never shown*, une valeur lisible en clair, et la
colonne *Used by* qui nomme la route pointant sur la première.

## La liste

Nom et description, genre, valeur (ou *encrypted, never shown*), et **Used by** - la
colonne qui distingue une entrée vivante d'un reste. Cliquer une ligne ouvre l'éditeur ;
le + crée une entrée.

Une entrée porte :

- **Name** - ce que `$nom` dira.
- **Kind** - valeur ou secret, choisi sur un bouton à deux positions.
- **Value** - tapée une seule fois pour un secret.
- **Description** - pour celui qui la retrouvera dans six mois.
- **Reminder date** - le jour où le secret expire à sa source. **C'est purement un
  rappel** : la passerelle ne peut pas savoir qu'un jeton a été renouvelé chez le
  fournisseur, donc `$nom` continue de résoudre. La date nourrit le
  [digest quotidien](/docs/console/mail-relay), qui liste ce qui approche et ce qui
  vient de passer - jamais la valeur.

## D'où viennent les entrées

La plupart naissent du champ qui en a besoin. Un champ sensible propose **Move into the
vault**, et refuse d'être enregistré en littéral : l'administrateur tient la valeur, c'est
donc lui qui la range. Un champ qui accepte une valeur en clair propose la même chose pour
les valeurs.

L'ordre habituel est donc l'inverse de celui qu'on attend : vous remplissez un relais, une
autorité ou un certificat, et l'entrée du coffre apparaît comme conséquence.

## Le coffre comme fichier

**Export** écrit le coffre en un seul fichier chiffré, sous une phrase secrète que Meerkat
ne garde pas. **Import** le relit. C'est la moitié qu'un export de configuration n'emporte
jamais : c'est elle qui amorce un second environnement ou déménage une passerelle.

> [!WARNING]
> Un coffre exporté vaut exactement ce que vaut sa phrase secrète, et ranger les deux
> ensemble annule le chiffrement. C'est vrai aussi d'un
> [instantané](/docs/console/configuration) dont la clé maîtresse est à côté de la base.

## Ce que vous voyez

Le coffre se limite à l'appelant : un infra admin voit les entrées d'infra, un app admin
celles de l'application. L'écran est ouvert à quiconque administre un plan qui porte des
entrées.

## Pièges

- **Une référence est publique, un littéral jamais.** `$stripe_key` peut apparaître dans
  un export, un diff ou une capture d'écran sans conséquence. C'est toute la raison d'être
  du coffre.
- **Un secret ne se relit pas**, ni par vous ni par l'API. Le perdre veut dire le remplacer
  à la source.
- **Un import de configuration peut laisser des trous** : des entrées que le fichier
  désigne et que cette passerelle n'a pas. L'import les liste pour que vous les
  remplissiez tout de suite.
- **Supprimer une entrée utilisée** casse ce qui pointait dessus. Lisez d'abord la colonne
  *Used by*.
