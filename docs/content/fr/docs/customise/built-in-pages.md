---
title: Les pages intégrées
section: Personnalisation
order: 249
summary: Quelles pages la passerelle sert elle-même, comment elles sont disposées, et en combien de langues.
---

# Les pages intégrées

Ce sont les pages auxquelles Meerkat répond lui-même, devant vos applications. C'est du HTML rendu par
le serveur - pas de framework, pas de bundle, rien récupéré d'un CDN - et elles lisent les jetons du
thème, donc elles suivent votre palette sans recompilation (PAGE-01).

| Page | Où elle apparaît |
|---|---|
| `/login` | le formulaire de connexion, et les boutons des autorités externes |
| `/totp` et `/totp-enroll` | la vérification et l'enrôlement d'un second facteur |
| `/update-password` | un mot de passe que la passerelle exige de changer |
| `/forgot-password`, `/reset-password` | le flux de réinitialisation |
| `/register`, `/confirm`, `/account-pending` | l'inscription et sa confirmation |
| `/select-tenant`, `/select-group` | le choix de l'organisation ou du groupe dans lequel agir |
| `/refused` | pourquoi un appelant ne peut pas passer |
| `/profile` et ses sous-pages | les pages du compte : mot de passe, second facteur, passkeys, autorités, historique, jetons |
| la page d'indisponibilité | une route dont le service est tombé, ou le commutateur global |

## Un écran, trois onglets, un aperçu

Couleurs, disposition et identité étaient trois métiers, chacun avec ses options d'un côté et le
**même** aperçu de l'autre, chacun montrant une page que les deux autres décident aussi. C'est un seul
sujet - ce que le visiteur voit - donc c'est une entrée avec trois onglets, et les onglets sont à gauche
seulement. L'aperçu ne bouge jamais, et le carrousel de thèmes reste dessous sur les trois : essayer une
couleur pendant qu'on juge une disposition est le sens normal, pas un cas particulier.

Les onglets sont de vraies URL, donc un marque-page sur la galerie de dispositions revient sur la
galerie de dispositions.

## La disposition

Un catalogue fermé de gabarits, chacun étant un bloc de CSS livré avec le produit (PAGE-02) :

| Gabarit | À quoi ça ressemble |
|---|---|
| `centered` | la marque au-dessus, la carte au milieu, le fond derrière tout. Le défaut |
| `split` | l'image prend une moitié pleine hauteur avec la marque dessus, le formulaire prend l'autre |
| `drawer` | l'image reste entière et un panneau opaque est posé contre un bord, portant marque et formulaire |
| `banner` | la marque dans un bandeau en haut, la carte dessous |
| `bare` | pas de carte du tout : les champs sont posés sur le fond |

`split` et `drawer` sont faits de deux moitiés, donc ils portent un **côté** : gauche ou droite. Les
autres l'ignorent, et le champ est effacé plutôt que gardé et silencieusement inutilisé.

> [!NOTE] Enterprise edition
> Changer la disposition fait partie du white-label, le même achat que retirer la mention : faire que
> ces pages ressemblent à votre produit plutôt qu'au nôtre.

Trois choses en découlent, et chacune est délibérée. Un enregistrement de réglages porte toute la
charge utile, donc tous les autres écrans renvoient la disposition courante sans y toucher - la garde
porte sur le **changement** et non sur la valeur, sinon enregistrer une langue sur une instance
communautaire serait refusé. Une disposition déjà en place **continue d'être servie** si une licence
expire : le modèle est perpétuel, et un fichier périmé qui redessinerait en silence la page de
connexion de tous les clients est le « ça marchait hier » que ce produit refuse. Et revenir à
`centered` est **toujours** permis, sinon une instance pourrait rester coincée sur une disposition
qu'elle ne peut plus quitter.

## Clair, sombre, ou le choix du visiteur

L'intégrateur tranche d'abord, en décochant un schéma sur l'aperçu (THEME-05) :

| Réglage | Ce que les pages font |
|---|---|
| le visiteur décide | son système pour commencer, puis une bascule qui se souvient |
| clair | clair uniquement, et la bascule disparaît |
| sombre | sombre uniquement, et la bascule disparaît |

Imposer n'est pas un caprice. Ces pages sont **devant** une application qui ne connaît peut-être qu'un
seul aspect : une page de connexion qui suit le système sombre du visiteur, et qui passe la main à un
portail uniquement clair, se lit comme deux produits - et vous ne pouvez pas corriger ça de votre côté.

Quand le visiteur choisit, son choix vit dans un cookie pendant un an **et** sur son compte, donc il est
reposé sur un navigateur qui ne l'a jamais vu - exactement comme sa langue. Voir
[clair et sombre](/docs/customise/color-scheme) pour ce qu'une application proxifiée en fait.

## Les langues

Vingt catalogues sont embarqués dans le binaire, un fichier JSON par langue : arabe, allemand, anglais,
espagnol, français, hébreu, hindi, indonésien, italien, japonais, coréen, néerlandais, polonais,
portugais, russe, thaï, turc, ukrainien, vietnamien et chinois simplifié (I18N-01).

L'anglais fait référence : chaque autre catalogue lui est comparé au démarrage, et une clé qu'un
catalogue ne porte pas retombe sur l'anglais plutôt que d'afficher un blanc. Les messages d'erreur du
backend sont localisés de la même façon (I18N-02).

La langue vient du choix du visiteur - un cookie et son compte - et sinon de ce que son navigateur
demande. Les langues *offertes* se règlent sous **Application > Locales**, et une route UI déclare
lesquelles de cette réserve elle sert.

> [!WARNING]
> La console, elle, est **en anglais uniquement**, et c'est une décision plutôt qu'un manque (I18N-03) :
> c'est un outil d'exploitant. Les pages ci-dessus sont celles que vos utilisateurs voient, et
> celles-là sont traduites.

## Ce qui manque

- **Votre propre HTML.** La disposition est un catalogue ; remplacer le balisage d'une page par votre
  gabarit est l'autre moitié de PAGE-02 et n'est pas construit.
- **Des catalogues surchargeables** (PAGE-03) : ajouter une langue reste une recompilation.
- La page de sélection de variante du développeur (DEV-06), qui n'a de sens qu'une fois la portée des
  substitutions par développeur construite.
