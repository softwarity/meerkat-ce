---
title: Connexion par code reçu par e-mail
section: Authentification
order: 108
summary: Se connecter avec un code à usage unique envoyé à son adresse, à la place d'un mot de passe, et le cadre qui rend cette porte défendable.
---

# Connexion par code reçu par e-mail

Se connecter sans mot de passe : la personne saisit son adresse, reçoit un code à six
chiffres, le saisit à son tour, et la session s'ouvre.

Le réglage se trouve dans **Application > Security** : c'est l'interrupteur *Allow signing
in with an e-mailed code*. Il est désactivé par défaut et nécessite un relais de messagerie
(**Infra > Mail relay**). Quand il est désactivé, le lien n'apparaît pas sur la page de
connexion et les adresses correspondantes répondent 404 : un contrôle qui s'affiche pour
refuser ensuite est pire qu'un contrôle absent.

## Ce que cela change

Avec cette porte, **la boîte aux lettres devient le moyen d'authentification**. Le compte
vaut alors ce que vaut la boîte aux lettres, pas davantage. L'argument est moins tranché
qu'il n'y paraît : si "mot de passe oublié" est ouvert, la boîte aux lettres est déjà un
moyen d'entrer, et le code ne fait que rendre explicite ce que la réinitialisation suppose
sans le dire. Mais c'est une décision et non un simple réglage, ce qui explique que
l'interrupteur soit désactivé par défaut.

## Le cadre

| Règle | Ce qu'elle empêche |
|---|---|
| **Plan de données uniquement** | La console ne s'ouvre jamais avec une boîte aux lettres : les pages ne sont pas montées sur le port d'administration, quel que soit le titulaire du compte |
| **Lié au navigateur** | Le code est scellé avec un identifiant de demande aléatoire, conservé dans un cookie. Un code dicté au téléphone n'ouvre rien sur la machine de celui qui appelle |
| **Premier facteur uniquement** | Le second facteur s'applique ensuite, comme après un mot de passe. Le code et un lien de réinitialisation passent par le même canal : dispenser du second facteur reviendrait à donner les deux moitiés à celui qui tient la boîte aux lettres |
| **Dix minutes, usage unique** | Un code resté dans une boîte de réception ne peut pas être rejoué, et une boîte ouverte la semaine suivante n'ouvre rien |
| **Un seul code valide** | Demander un nouveau code invalide le précédent |
| **Pas d'énumération** | La même page, le même statut et le même cookie, que l'adresse existe ou non |
| **Double limitation** | L'envoi est limité par adresse, les essais de code par navigateur |

> [!NOTE]
> `code` devient un identifiant d'autorité réservé. Une autorité portant ce nom se
> trouverait derrière `/login/code`, qui est justement cette page : elle serait masquée
> sans que rien ne le signale. La console refuse ce nom et explique pourquoi.

## Ce que voit la personne

1. Sur la page de connexion, **Se connecter avec un code par e-mail**.
2. Elle saisit son adresse. La page suivante indique qu'un code est en route, sans dire
   s'il existe un compte derrière cette adresse.
3. Elle saisit le code reçu. Si son compte est soumis à un second facteur, il lui est
   demandé ensuite, exactement comme après un mot de passe.

L'historique des connexions nomme la porte empruntée : *code par e-mail*, ou *code par
e-mail + code* quand un second facteur a suivi.

## Ce qui n'existe pas

Le **lien magique** - un clic dans l'e-mail au lieu d'un code - n'est pas développé. C'est
un choix : n'importe quel outil qui analyse le courrier peut cliquer sur un lien, et un
lien se transfère d'un seul geste.

Interdire cette porte **par compte ou par rôle** n'est pas développé non plus :
l'interrupteur est global. Pour un compte d'administration, la protection actuelle tient à
la structure même du produit : la console ne s'ouvre jamais de cette façon.
