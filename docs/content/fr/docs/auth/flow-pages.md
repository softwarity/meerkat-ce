---
title: Les pages servies par la gateway
section: Authentification
order: 118
summary: Connexion, second facteur, changement de mot de passe, choix de l'organisation - les pages que Meerkat affiche lui-même, et jusqu'où vous pouvez les personnaliser.
---

# Les pages servies par la gateway

Tout ce qu'une personne doit voir avant d'atteindre votre application est servi par la
gateway elle-même : aucune bibliothèque dans votre application, aucune page à écrire,
aucun gabarit à déployer. Ces pages sont compilées dans le binaire, elles portent vos
couleurs et votre logo, et elles parlent vingt langues.

Elles sont toutes servies avec `Cache-Control: no-store` : une page qui nomme une personne
ne doit jamais séjourner dans un cache partagé.

## Les pages

| Chemin | Ce que c'est |
|---|---|
| `/login` | la page de connexion : le formulaire d'identification, un bouton par fournisseur d'identité, le bouton passkey, et les liens pour s'inscrire ou récupérer un mot de passe quand ces fonctions sont ouvertes |
| `POST /logout` | met fin à cette session et revient à la page de connexion |
| `/totp` | la demande du second facteur, avec la case *faire confiance à ce navigateur* et le lien pour recevoir un code par e-mail quand ils sont disponibles |
| `/totp-enroll` | l'enrôlement imposé, quand un second facteur est exigé et que le compte n'en a pas |
| `/update-password` | le changement imposé : mot de passe temporaire ou expiré |
| `/forgot-password`, `/reset-password` | la récupération par e-mail, et la page où mène le lien |
| `/register`, `/confirm` | l'auto-inscription et la confirmation de l'adresse |
| `/confirm-email` | le lien qui confirme une nouvelle adresse, modifiée depuis le profil |
| `/select-tenant` | le choix de l'organisation dans laquelle travaille cette session |
| `/select-group` | le choix du groupe, en mode groupe exclusif |
| `/account-pending` | la salle d'attente : un compte qui existe mais auquel rien n'a encore été accordé |
| `/refused` | la personne est connectée, mais refusée. La page nomme la règle qui a refusé et propose ce que cette session *peut* ouvrir |
| `/profile/...` | les pages personnelles : identité, mot de passe, second facteur, passkeys, autorités, historique des connexions, sessions actives, jetons d'API, départ d'une organisation |

La page d'indisponibilité n'a pas de chemin propre : pendant une maintenance, elle répond
`503` sur **tous** les chemins, donne un motif tiré d'une liste fermée et laisse à un
administrateur un moyen de passer.

La page de connexion de la console repose sur le même mécanisme, sur le port
d'administration, à une différence près : elle est toujours en anglais et toujours sombre.
La console est la porte de Meerkat lui-même, elle ne reprend pas les réglages de
l'intégrateur.

## Les couleurs

**Application > Built-in pages > Theme.** Dix couleurs, saisies en hexadécimal : la couleur
d'accent et celle de son texte, les surfaces (page, carte, champ), le texte et sa variante
atténuée, le contour, et la couleur d'erreur. Elles sont émises sous forme de variables
CSS - `--mk-primary`, `--mk-surface`, `--mk-on-surface`... - une seule fois par page, la
valeur claire et la valeur sombre côte à côte. La palette suit ainsi le mode d'affichage du
visiteur, sans clignotement et sans script.

Plusieurs thèmes peuvent coexister ; un seul est actif. Vous dupliquez un thème, le
modifiez, le prévisualisez, l'activez, revenez en arrière. Huit thèmes prédéfinis sont
fournis, tous construits sur la même base avec une couleur d'accent différente : Sentinel's
Watch, le thème par défaut, puis Midnight, Lavender, Orchid, Rose, Crimson, Ember et
Forest. Une case *Glow* désactive d'un seul geste les effets et le dégradé, pour un rendu
plus sobre.

Seules les couleurs hexadécimales sont acceptées. Rien d'autre n'atteint le bloc de style
de la page.

## Clair et sombre

Le visiteur choisit avec un bouton à trois états présent sur la page - suivre le système,
clair, sombre. Ce choix est conservé pendant un an dans le cookie `MEERKAT_SCHEME` **et**
sur le compte : un navigateur qui n'a jamais vu cette personne applique donc son mode
d'affichage dès qu'elle s'est connectée.

Pour retirer ce choix, décochez un mode au-dessus de sa colonne de couleurs dans l'éditeur
de thème : le mode restant est imposé et le bouton disparaît.

## Disposition et marque

**Layout** détermine la disposition : *centered*, la carte au milieu de la page ; *split*,
une image sur une moitié de l'écran, sur toute la hauteur ; *drawer*, un panneau opaque
contre un bord d'une image plein cadre ; *banner*, un bandeau à vos couleurs en haut de la
page ; *bare*, les champs posés directement sur l'image, sans carte. Pour *split* et
*drawer*, vous choisissez aussi le côté.

> [!NOTE] **Édition Enterprise.**
> Seule la disposition *centered* figure dans l'image Community. Les quatre autres sont
> fournies avec l'édition Enterprise, de même que la possibilité de masquer la mention
> *powered by softwarity/meerkat*. Une image Community qui reçoit une disposition
> Enterprise affiche la disposition centrée plutôt qu'une page sans habillage, et il est
> toujours possible de revenir à *centered*.

**Branding** regroupe le nom de l'application et son slogan, le logo (téléversé, affiché en
taille normale, grande ou très grande), l'icône de l'onglet du navigateur - qui reprend le
logo si vous la laissez vide - et une image de fond de page avec son cadrage (cover,
contain ou tile), un voile d'assombrissement et, si vous le souhaitez, une image distincte
pour le mode sombre.

Une page affichée dans l'iframe d'un autre site abandonne la marque et occupe seule tout le
cadre. Cela se décide dans le navigateur : le HTML servi est donc le même pour tout le
monde.

## Les langues

Vingt catalogues sont intégrés : arabe, allemand, anglais, espagnol, français, hébreu,
hindi, indonésien, italien, japonais, coréen, néerlandais, polonais, portugais, russe,
thaï, turc, ukrainien, vietnamien et chinois simplifié. L'anglais sert de référence : une
clé absente d'un autre catalogue est remplacée par la phrase anglaise au lieu de laisser un
vide.

Les langues proposées sont DÉDUITES, jamais déclarées : c'est l'union de celles que vos
routes déclarent parler (**Routes > une route > Locales**), croisée avec celles qui sont
intégrées. Cette liste n'est jamais vide - l'anglais est le minimum. Déployez une route
écrite en polonais et cette page propose le polonais ; retirez-la, et l'offre se réduit
d'autant.

Pour une requête donnée, la langue est choisie dans cet ordre :

1. le cookie `MEERKAT_LANG`, s'il désigne une langue que vous proposez ;
2. `Accept-Language`, comparé sur le code de langue ;
3. l'anglais si une route le parle, sinon la première langue proposée.

Le choix d'une personne est enregistré sur son compte et replacé dans le cookie à sa
connexion : sa langue la suit ainsi sur un nouveau navigateur. L'arabe et l'hébreu
s'affichent de droite à gauche.

> [!NOTE]
> Les catalogues sont compilés dans le binaire, mais ils ne sont pas figés pour autant :
> **Application > Built-in pages > Locale** permet de corriger une phrase d'une langue
> existante, ou d'ajouter une langue que le binaire ne contient pas, sans recompilation.
> Seul ce qui diffère du catalogue intégré est enregistré : les corrections apportées par
> une version ultérieure continuent donc de vous parvenir.

## Ce que vous ne pouvez pas changer

**Votre propre HTML.** Les pages sont des gabarits inclus dans le binaire ; les points de
personnalisation sont le thème, la disposition, la marque et les catalogues de langues. Il
n'existe ni réglage pour remplacer un gabarit, ni fichier que la gateway lirait au
démarrage. Si vous avez besoin d'une page de connexion entièrement à vous, la réponse
honnête est qu'aujourd'hui Meerkat ne le permet pas encore.
