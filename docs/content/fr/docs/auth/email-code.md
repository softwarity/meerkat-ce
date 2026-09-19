---
title: Connexion par code e-mail
section: Authentification
order: 108
summary: Se connecter avec un code à usage unique reçu par e-mail, à la place du mot de passe, et le cadre qui rend cette porte tenable.
---

# Connexion par code e-mail

Se connecter sans mot de passe : la personne saisit son adresse, reçoit un code
à six chiffres, le retape, et la session s'ouvre.

**Application > Security**, interrupteur *Allow signing in with an e-mailed
code*. Il est livré éteint, et il a besoin d'un relais (Infra > Mail relay).
Éteint, le lien n'apparaît pas sur la page de connexion et les adresses
répondent 404 : un contrôle qui s'affiche puis refuse est pire qu'un contrôle
absent.

## Ce que cela déplace

Cette porte fait de la **boîte mail l'identifiant**. Le compte vaut alors ce
que vaut la boîte, ni plus ni moins. L'argument est moins tranché qu'il n'y
paraît : si "mot de passe oublié" est ouvert, la boîte mail est déjà un chemin
d'entrée, et le code ne fait que rendre explicite ce que la réinitialisation
implique en silence. Mais c'est une décision, pas un réglage, et c'est la
raison pour laquelle l'interrupteur arrive éteint.

## Le cadre

| Règle | Ce qu'elle empêche |
|---|---|
| **Plan de données uniquement** | La console n'est jamais ouvrable par une boîte mail : les pages ne sont pas montées sur le port d'administration, quel que soit le compte |
| **Lié au navigateur** | Le code est scellé avec un identifiant de requête tiré au sort, posé en cookie. Un code lu au téléphone n'ouvre rien chez l'appelant |
| **Premier facteur seulement** | Le second facteur tourne derrière, comme derrière un mot de passe. Le code et un lien de réinitialisation voyagent par le même canal : sauter le facteur donnerait les deux moitiés à qui tient la boîte |
| **Dix minutes, usage unique** | Un code laissé dans une boîte ne se rejoue pas, et une boîte ouverte la semaine prochaine n'ouvre rien |
| **Un seul code vivant** | Demander un nouveau code tue le précédent |
| **Aucune énumération** | Même page, même statut, même cookie, que l'adresse existe ou non |
| **Deux limitations** | L'envoi est freiné par adresse, les essais par navigateur |

> [!NOTE]
> `code` devient un identifiant d'autorité réservé. Une autorité nommée ainsi
> vivrait derrière `/login/code`, qui est cette page : elle serait masquée sans
> que rien ne le dise. La console refuse ce nom en disant pourquoi.

## Ce que voit la personne

1. Sur la page de connexion, **Se connecter avec un code par e-mail**.
2. Elle saisit son adresse. La page suivante dit qu'un code est parti, sans
   dire si un compte existe derrière.
3. Elle retape le code. Si son compte doit un second facteur, il est demandé
   ensuite, comme après un mot de passe.

L'historique de connexion nomme la porte : *code par e-mail*, ou *code par
e-mail + code* quand un second facteur a suivi.

## Ce qui n'est pas là

Le **lien magique** - un clic dans le courriel au lieu d'un code - n'est pas
écrit. C'est un choix : un lien est cliquable par tout ce qui scanne le
courrier, et il se transmet d'un geste.

Interdire cette porte **par compte ou par rôle** n'est pas écrit non plus :
l'interrupteur est global. Pour un compte d'administration, la protection
aujourd'hui est structurelle - la console ne s'ouvre jamais ainsi.
