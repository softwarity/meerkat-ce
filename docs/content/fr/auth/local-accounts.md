---
title: Comptes locaux
section: Authentification
order: 102
summary: Les comptes que Meerkat détient lui-même - politique de mot de passe, étranglement des tentatives, et auto-inscription.
---

# Comptes locaux

L'autorité locale est celle à laquelle Meerkat répond tout seul : un
identifiant, une empreinte de mot de passe qu'il conserve, et rien à demander à
personne. Elle est active au premier démarrage, et le compte root créé à ce
premier démarrage lui appartient.

Elle apparaît dans **Infra > Authentication** comme une ligne nommée *Local
accounts*, à côté des fournisseurs d'identité et des annuaires. L'éteindre est
ce qui rend les autres autorités exclusives : tant qu'elle répond, chaque mot
de passe local est une porte qui les contourne.

> [!NOTE]
> Le port d'administration ne ferme jamais cette porte. La console est ce avec
> quoi on réparera une autorité cassée, et la mettre derrière cette même
> autorité est la façon de rendre une installation irrécupérable au plus mauvais
> moment.

## La politique de mot de passe

Une seule politique pour toute l'installation, dans **Application >
Security**. Un mot de passe est vérifié là où il est tapé, et les endroits où
il est tapé - le formulaire d'inscription, le changement forcé à la première
connexion, le profil, un lien de réinitialisation - ne savent rien des
organisations.

| Champ (console) | Ce qu'il exige | Livré à |
|---|---|---|
| Length | nombre minimum de caractères | `8` |
| Lowercase, Uppercase, Digits, Special characters | compte minimum de chaque genre | `0` |
| No reuse of the last | combien de mots de passe précédents sont refusés | `0` |
| Expires after (days) | force un changement au bout de tant de jours | `0`, jamais |

Un zéro veut dire *peu importe* : la règle n'est ni vérifiée ni affichée. Ce
qui est livré n'exige donc qu'une longueur, et c'est délibéré - relever la
barre pour tout le monde pendant une montée de version enferme les gens dehors
du changement de mot de passe qu'ils étaient en train de faire.

Trois choses à savoir avant de remplir les cases :

- La longueur est comptée en **caractères, pas en octets** : une phrase de passe en grec ou en japonais n'est pas trois fois plus longue qu'elle n'en a l'air.
- Une lettre dans une écriture sans casse - kana, chinois, arabe, hébreu - ne compte ni comme minuscule, ni comme majuscule, ni comme caractère spécial.
- Si la longueur demandée est inférieure à la somme des quatre genres, elle est relevée à cette somme : quatre genres à deux chacun exigent huit caractères, et une liste de contrôle impossible à satisfaire est pire que pas de liste.

La console affiche la liste de contrôle exactement comme les pages de connexion
la dessineront. C'est la même fonction des deux côtés, exprès : une liste qui
dit une chose alors que le serveur en exige une autre est crue.

**L'expiration est lue à la connexion, pas par une horloge.** Un mot de passe
qui expire à trois heures du matin ne déconnecte personne ; il refuse la
connexion suivante, qui atterrit alors sur la page de changement de mot de
passe. Le même écran porte un bouton *Force a change for everyone*
pour le jour où il faudra.

Les mots de passe sont hachés en bcrypt. Une empreinte ancienne n'est pas
réencodée quand vous montez le coût : Meerkat n'a pas encore de réencodage
transparent à la connexion.

## Un étranglement, jamais un verrouillage

Les échecs de connexion sont comptés par **adresse cliente et par compte**, sur
une fenêtre glissante, dans **Application > Security > Rate limiting**.

| Champ (console) | Ce qu'il compte | Livré à |
|---|---|---|
| Failed sign-ins | échecs tolérés avant que le formulaire réponde `429` | `10` |
| Wrong 2FA codes | codes de second facteur erronés tolérés, même fenêtre | `5` |
| Within | la fenêtre, en durée ISO-8601 | `PT15M` |

Une connexion réussie efface le compteur. Le refus arrive **avant** la
comparaison d'empreinte, donc un attaquant bloqué ne fait même pas brûler de
processeur. Mettre un compte à zéro éteint ce limiteur.

Le compteur vit en base, pas dans le processus :

- dix tentatives, c'est dix pour l'installation et non dix par passerelle - un attaquant qui arrose un répartiteur de charge ne voit plus la limite multipliée par le nombre de noeuds ;
- un redémarrage ne pardonne plus à celui qui était étranglé.

Si la base ne répond pas, le limiteur laisse passer la tentative plutôt que de
la refuser : la connexion allait de toute façon avoir besoin de cette même
base, et un incident passager ne doit pas devenir un enfermement dehors.

**Aucun compte n'est jamais verrouillé.** Rien de ce qu'un tiers fait de
l'extérieur ne peut retirer un compte à la personne à qui il appartient.

Un identifiant inconnu, un mauvais mot de passe et un compte désactivé
répondent la même phrase, et coûtent le même temps - un identifiant inconnu
passe quand même par une comparaison d'empreinte contre un leurre, de sorte que
le temps de réponse ne distingue rien. Un compte hors de sa fenêtre d'accès est
la seule exception : à qui a tapé le **bon** mot de passe, la page nomme la
date, parce que sans elle chaque expiration devient un appel au support pour
demander la seule chose que la page aurait pu dire.

## Laisser quelqu'un créer son compte

L'auto-inscription est livrée fermée. Une fois ouverte, la gateway sert
`/register` : un identifiant, une adresse, un mot de passe vérifié contre la
politique ci-dessus, et un code en image à recopier.

Quatre conditions doivent toutes tenir pour que cette page existe :

1. l'autorité locale est **active** ;
2. l'auto-inscription lui est permise - soit *Allowed* sur l'autorité elle-même, soit *Inherited* avec l'interrupteur en haut d'**Infra > Authentication** allumé ;
3. un relais de messagerie est configuré dans **Infra > Mail relay**, parce que l'adresse doit être confirmée ;
4. c'est le plan de données. Le port d'administration ne sert jamais de formulaire d'inscription.

> [!WARNING]
> L'interrupteur en haut de l'écran Authentication est un **défaut**, pas un
> verrou. Une autorité dont la politique propre dit *Allowed* continue de créer
> des comptes alors que cet interrupteur est éteint. Pour fermer
> l'auto-inscription pour de bon, mettez l'autorité elle-même sur *Refused*.

Ce qu'une inscription produit est délibérément inutile : le compte existe, il
est inutilisable jusqu'à la confirmation de l'adresse, et il n'atteint rien
jusqu'à ce qu'un administrateur le place dans une organisation et lui accorde
des rôles. Le lien de confirmation est un jeton à usage unique valable
vingt-quatre heures. Qui se connecte avant d'avoir confirmé reçoit le lien à
nouveau plutôt qu'une explication.

Un identifiant ou une adresse déjà pris atterrissent sur la **même** page
qu'une inscription réussie, et rien n'est créé : le formulaire ne dit pas à un
inconnu qui possède déjà un compte ici. Le code en image est actif par défaut
et se désactive par autorité ; il est consommé que la réponse soit juste ou
fausse, donc un second essai veut dire une nouvelle image.

Les points d'écriture auxquels personne n'est encore connecté - `/register` et
`/forgot-password` - portent leur propre étranglement figé : cinq essais par
adresse cliente par quinze minutes, qu'aucun écran ne change.

## Un mot de passe oublié

`/forgot-password` existe dès qu'un relais de messagerie est configuré et que
les mots de passe locaux sont acceptés ; il n'attend pas que l'auto-inscription
soit ouverte. La page de sortie est la même que l'adresse soit connue ou non.

Le lien vaut une heure et est consommé à l'usage. Le nouveau mot de passe est
vérifié contre la politique, règle de non-réutilisation comprise, et le
changement **détruit toutes les sessions vivantes du compte** sur toutes les
passerelles : quiconque en tenait une, intrus compris, se reconnecte ou reste
dehors. Le propriétaire est averti par courriel que son mot de passe a changé.

Deux choses qu'une réinitialisation ne fait pas encore : elle ne révoque pas
les jetons d'API du compte, et elle n'oublie pas ses navigateurs de confiance.
