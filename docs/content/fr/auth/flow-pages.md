---
title: Les pages servies par la gateway
section: Authentification
order: 118
summary: Connexion, second facteur, changement de mot de passe, choix d'organisation - les pages que Meerkat dessine lui-même, et jusqu'où on peut les habiller.
---

# Les pages servies par la gateway

Tout ce qu'il faut montrer à quelqu'un avant qu'il atteigne votre application est
servi par la gateway elle-même : aucune bibliothèque dans votre application,
aucune page à écrire, aucun gabarit à déployer. Elles sont compilées dans le
binaire, elles portent vos couleurs et votre logo, et elles parlent vingt langues.

Elles sont aussi toutes en `Cache-Control: no-store` : une page qui nomme une
personne ne doit jamais dormir dans un cache partagé.

## Les pages

| Chemin | Ce que c'est |
|---|---|
| `/login` | la page de connexion : le formulaire, un bouton par fournisseur d'identité, le bouton passkey, et les liens pour s'inscrire ou récupérer un mot de passe quand ils sont ouverts |
| `POST /logout` | termine cette session et revient sur la page de connexion |
| `/totp` | le défi du second facteur, avec *trust this browser* et le lien du code par courriel quand ils sont disponibles |
| `/totp-enroll` | l'enrôlement forcé, quand un second facteur est exigé et que le compte n'en a pas |
| `/update-password` | le changement forcé : un mot de passe temporaire ou expiré |
| `/forgot-password`, `/reset-password` | la récupération par courriel, et la page sur laquelle le lien atterrit |
| `/register`, `/confirm` | l'auto-inscription et la confirmation d'adresse |
| `/select-tenant` | dans quelle organisation cette session travaille |
| `/select-group` | dans quel groupe, en mode de groupe exclusif |
| `/account-pending` | la salle d'attente : un compte qui existe et à qui rien n'a encore été accordé |
| `/refused` | connecté, et écarté. La page nomme la règle qui a refusé et propose ce que cette session *peut* ouvrir |
| `/profile/...` | les pages de la personne : identité, mot de passe, second facteur, passkeys, autorités, historique de connexion, jetons d'API |

La page d'indisponibilité n'a pas de chemin à elle : elle répond sur **n'importe
quel** chemin tant que la maintenance est active, en `503`, nomme la raison choisie
dans une liste fermée, et laisse une porte à un administrateur.

La page de connexion de la console est la même machinerie sur le port
d'administration, avec une différence : elle est toujours en anglais et toujours
sombre. La console est la porte de Meerkat lui-même et ne prend pas les réglages de
l'intégrateur.

## Les couleurs

**Application > Built-in pages > Theme.** Dix couleurs, saisies en hexadécimal :
l'accent et son texte, les surfaces (page, carte, champ), le texte et sa variante
atténuée, le contour, et la couleur d'erreur. Elles sont émises en variables CSS -
`--mk-primary`, `--mk-surface`, `--mk-on-surface`... - une fois par page, avec la
valeur claire et la valeur sombre côte à côte, de sorte que le schéma du visiteur
choisit la palette sans clignotement et sans script.

Plusieurs thèmes coexistent ; exactement un est actif. On duplique, on modifie, on
prévisualise, on active, on revient en arrière. Huit préréglages sont livrés, tous
sur la même base avec un accent différent : Sentinel's Watch, celui par défaut,
puis Midnight, Lavender, Orchid, Rose, Crimson, Ember et Forest. Une case *Glow*
éteint d'un geste les effets lumineux et le dégradé du nom, pour une allure plate.

Seules des couleurs hexadécimales sont acceptées. Rien d'autre n'atteint le bloc de
style de la page.

## Clair et sombre

Le choix du visiteur est un bouton à trois états sur la page - suivre le système,
clair, sombre - enregistré dans le cookie `MEERKAT_SCHEME` pour un an **et** sur le
compte, de sorte qu'un navigateur qui n'a jamais vu cette personne affiche quand
même son schéma dès qu'elle se connecte.

Pour retirer le choix, décochez un schéma au-dessus de sa colonne de couleurs dans
l'éditeur de thème : celui qui reste est imposé et le bouton disparaît.

## Disposition et marque

**Layout** choisit l'arrangement : *centered*, la carte au milieu de la page ;
*split*, une image sur une moitié pleine hauteur ; *drawer*, un panneau opaque
contre un bord d'une image plein cadre ; *banner*, un bandeau de marque en haut ;
*bare*, les champs sur l'image, sans carte. Pour *split* et *drawer*, vous choisissez
aussi un côté.

> [!NOTE] **Enterprise edition.**
> Seul *centered* est dans l'image communautaire. Les quatre autres arrangements
> viennent avec l'édition Enterprise, ainsi que la possibilité de retirer la mention
> *powered by softwarity/meerkat*. Une image communautaire à qui l'on donne une
> disposition Enterprise dessine celle du centre plutôt qu'une page que rien
> n'habille, et revenir à *centered* est toujours permis.

**Branding** porte le nom et l'accroche de l'application, le logo (téléversé,
dessiné en taille normale, grande ou très grande), une icône d'onglet - qui suit le
logo si vous la laissez vide - et une image de fond avec son cadrage (cover,
contain ou tile), un voile d'atténuation, et une image distincte pour le mode sombre
si vous en voulez une.

Une page affichée dans l'iframe de quelqu'un abandonne d'elle-même la marque et
remplit le cadre. C'est décidé dans le navigateur, donc le HTML servi est le même
pour tout le monde.

## Les langues

Vingt catalogues sont embarqués : arabe, allemand, anglais, espagnol, français,
hébreu, hindi, indonésien, italien, japonais, coréen, néerlandais, polonais,
portugais, russe, thaï, turc, ukrainien, vietnamien et chinois simplifié. L'anglais
est la référence : une clé absente d'un autre catalogue retombe sur la phrase
anglaise plutôt que d'afficher du vide.

Lesquels sont proposés vous appartient, dans **Application > Locales**. L'offre est
votre liste intersectée avec ce qui est embarqué, et elle ne finit jamais vide :
l'anglais est le plancher.

Pour une requête, la langue est choisie dans cet ordre :

1. le cookie `MEERKAT_LANG`, s'il nomme une langue que vous proposez ;
2. `Accept-Language`, apparié sur l'étiquette de langue ;
3. la première langue que vous proposez, et à défaut l'anglais.

Le choix d'une personne est écrit sur son compte et reposé dans le cookie quand elle
se connecte : sa langue la suit sur un nouveau navigateur. L'arabe et l'hébreu sont
rendus de droite à gauche.

> [!NOTE]
> Les catalogues sont compilés dans le binaire. Ajouter une langue, ou changer une
> phrase d'une langue existante, est une recompilation - il n'y a pas de répertoire
> où déposer un fichier JSON. FEATURES.md liste les catalogues surchargeables comme
> encore manquants.

## Ce que vous ne pouvez pas changer

**Votre propre HTML.** Les pages sont des gabarits dans le binaire ; les coutures
sont le thème, la disposition, la marque et les catalogues de langues. Il n'y a
aucun réglage de remplacement de gabarit et aucun fichier que la gateway lise au
démarrage. S'il vous faut une page de connexion entièrement à vous, la réponse
honnête aujourd'hui est que Meerkat ne sait pas encore le faire.
