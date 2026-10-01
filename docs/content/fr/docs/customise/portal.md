---
title: Le catalogue d'applications
section: Personnalisation
order: 252
summary: La liste des applications que la passerelle offre, et les trois façons de l'offrir - rien, un menu, ou une barre de navigation.
---

# Le catalogue d'applications

Plusieurs routes UI derrière une passerelle, ce sont plusieurs applications : un
visiteur atterrit sur l'une et n'a aucun moyen d'atteindre la suivante. Le
catalogue est la liste de celles que vous offrez, dans l'ordre que vous
choisissez.

Il se règle sous **Application > Portal**, il est **global** comme le thème, et
il est livré vide.

![L'écran Portal en mode Portal : le bouton à trois états, l'arrangement, et la vraie barre en maquette live](img/console/portal.webp)

## Un catalogue, trois rendus

Un bouton à trois états décide de ce qui est dessiné. **La liste, elle, ne bouge
pas** : passer d'un menu à une barre est une décision de rendu, pas une raison
de retaper vos applications.

| Mode | Ce que voit le visiteur |
|---|---|
| **None** | Aucune liste. Le bouton utilisateur et les pages que Meerkat sert lui-même portent le nom de votre marque, sans rien à cliquer. C'est la bonne réponse quand il n'y a qu'une application |
| **Links** | La liste, dans l'ordre choisi, dans le sous-menu **Applications** du bouton utilisateur et sur les pages du plan de données |
| **Portal** | Une barre de navigation sur chaque page de chaque application. Les pages intégrées n'offrent alors qu'**un** lien, la première entrée que l'appelant peut ouvrir : la barre est la navigation, une page hors des applications a juste besoin d'une porte |

## Ce qu'une entrée porte

| Champ | Ce qu'il fait |
|---|---|
| Route | la route UI que cette entrée ouvre. L'entrée hérite de son adresse **et de son accès** |
| Libellé | le nom sous lequel l'application est offerte. Vide retombe sur le nom de la route |
| Description | l'infobulle de l'entrée |
| Désactivé | éteint pour tout le monde, sans le retirer de la liste |

En mode **Portal**, une entrée porte en plus de quoi dessiner une barre : une
icône, des enfants, et le libellé de la ligne « retour ici » quand elle en a.

| Champ | Ce qu'il fait |
|---|---|
| Icône | un glyphe choisi dans la banque de la console, stocké en SVG et dessiné en masque CSS - aucune police d'icônes n'est jamais chargée. Vide retombe sur l'initiale du libellé |
| Libellé d'accueil | ce que lit la ligne « retour à cette application », quand elle a des enfants |
| Enfants | des sous-applications montrées sur la surface secondaire |

## Le catalogue dit ce qui existe, la route dit qui le voit

Une entrée hérite de l'**accès de la route à laquelle elle se lie**, donc un
visiteur ne voit offert que ce que ses droits autorisent. Aucune règle d'accès
ne se décide ici : ce serait mettre une décision de sécurité dans un réglage
d'affichage.

La charge utile servie au navigateur ne porte d'ailleurs aucune règle : le
filtrage se fait avant qu'elle soit écrite, ce qui est pourquoi il n'y a rien à
lire ni à trafiquer dans la page.

C'est aussi pourquoi le catalogue est global plutôt que par organisation : il
est déjà personnalisé, par les routes.

> [!NOTE] Une route ne s'inscrit plus toute seule
> Le nom d'une application se décidait avant sur **chaque route UI**, dans un
> champ `Link`. Trois endroits pouvaient donc nommer la même application, et
> comme la liste était déduite des routes, il fallait deviner lesquelles
> étaient la même chose - une installation fronte couramment un produit avec
> plusieurs routes, une par organisation ou par version, qui diffèrent par ce
> qu'elles proxifient et jamais par où l'on va.
>
> Une nouvelle route UI n'apparaît donc plus d'elle-même : vous l'ajoutez ici.
> C'est le même nombre de décisions qu'avant, prises au même endroit.

## En-tête ou rail

En mode **Portal** seulement. Un axe, et il se bascule :

| Disposition | Les entrées | Leurs enfants |
|---|---|---|
| `header` | un bandeau d'onglets en haut | un rail |
| `rail` | un rail | un bandeau d'onglets en haut |

Le rail prend le côté que vous choisissez, et ce côté veut toujours dire quelque
chose puisque l'une des deux surfaces est toujours un rail.

Les entrées se rendent en icône seule, en libellé seul, ou les deux - un seul
réglage pour toute la barre. Le nom d'application de la marque peut se poser à
côté du logo ; c'est éteint par défaut, parce que le logo seul est la marque.

## Ce que la barre remplace

En mode **Portal**, elle prend la place du bouton utilisateur par route sur
**toutes** les routes UI. Le bouton ne disparaît pas : il déménage **dans** la
barre, et il y perd son sous-menu Applications, puisque c'est désormais la barre
elle-même. La navigation et le menu du compte sont une seule surface, pas deux
coins.

La barre n'est jamais dessinée dans une iframe : une page embarquée ailleurs
n'est pas l'endroit d'une navigation.

Techniquement, c'est un simple élément personnalisé à shadow DOM, servi comme le
bouton utilisateur, portant le thème du plan de données et le clair ou sombre du
visiteur.

## L'éditer

En mode **Links**, la console montre la liste telle qu'elle est : des entrées
numérotées, deux flèches pour l'ordre. En mode **Portal**, elle montre une
maquette live de la barre pendant que vous la construisez, pour que la
disposition se juge là où elle sera lue plutôt que dans une liste de champs.

## Ce qui n'est pas construit

- **Le canal de badge.** Une entrée porte une clé de badge et l'emplacement est
 réservé, mais rien ne pousse encore un compte dessus - c'est prévu sur le canal
 live.
- **L'atterrissage sur la première application accessible.** Un visiteur qui ne
 peut pas ouvrir l'application sur laquelle il arrive n'est pas redirigé vers
 une qu'il peut ouvrir.
- **Le glisser-déposer** : réordonner à la souris, ou faire glisser une entrée
 pour changer de niveau. Les flèches font le travail.
- **L'arrangement par organisation**. Ce serait la première surcharge visuelle
 par tenant du produit, et le thème et la marque sont globaux aujourd'hui.
