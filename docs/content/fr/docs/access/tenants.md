---
title: Organisations
section: Contrôle d'accès
order: 138
summary: L'appartenance, le choix à la connexion, le propriétaire, et les réglages qu'une organisation peut surcharger.
---

# Organisations

Une organisation - un tenant - c'est en tant que qui quelqu'un travaille. C'est la
portée dans laquelle vivent les rôles : les groupes appartiennent à une
organisation, et une session sans organisation active ne détient aucun rôle.

Chaque installation en possède une dès son premier démarrage, et une seule suffit
souvent pour toujours.

> [!NOTE] **Enterprise edition.**
> Plus d'une organisation fait partie de l'édition Enterprise. L'image communautaire
> sert une **seule** organisation, qu'aucun écran ne nomme jamais : ses groupes, ses
> membres et ses règles s'administrent depuis **Application**, et la notion
> n'apparaît nulle part ailleurs. Passer à la forme multi-organisation est une
> décision de root sur une image Enterprise.

Revenir au mode simple ne supprime rien : les autres organisations cessent
simplement d'être servies, et celle qui est servie est la plus ancienne.

## Les membres

Une appartenance, c'est une personne dans une organisation, de type **ADMIN** ou
**USER**, activée ou non. C'est toute l'énumération : il n'y a pas d'appartenance
OWNER.

**La propriété est une propriété de l'organisation**, pas une appartenance : une
organisation a toujours un propriétaire, le propriétaire est transférable, et il n'a
pas besoin d'être membre. Qui crée une organisation la possède.

Une appartenance **ADMIN**, ou le fait d'être propriétaire, est ce qui permet
d'administrer cette organisation depuis la console : l'organisation elle-même, ses
membres, ses groupes, ses règles de groupe, réinitialiser le mot de passe d'un
membre, lire l'historique de connexion d'un membre. Cela n'accorde rien en dehors de
cette organisation, et en particulier pas le catalogue global de rôles.

## Le choix à la connexion

Une fois le premier facteur et le second facteur faits :

| Appartenances | Ce qui se passe |
|---|---|
| aucune | la session est émise sans organisation. Qui n'administre rien atterrit dans `/account-pending`, la salle d'attente |
| une | elle est posée sur la session, silencieusement |
| plusieurs | la personne choisit, sur `/select-tenant` |

Seules les appartenances activées d'organisations activées comptent.

Une session sans organisation dont le titulaire gagne plus tard exactement une
appartenance l'adopte à la requête suivante : être ajouté à une organisation prend
effet sans se déconnecter.

Changer d'organisation ensuite se fait depuis le bouton utilisateur, et rejoue
l'étape du groupe : une nouvelle organisation peut avoir un autre mode de groupe et
d'autres groupes.

## Heures ouvrées

Une organisation peut restreindre les moments où ses gens entrent : jours de semaine,
plages horaires, une date de début et de fin, dans un fuseau nommé.

> [!NOTE] **Enterprise edition.**
> Les heures ouvrées font partie de l'édition Enterprise.

La fenêtre se résout depuis le niveau le plus fin qui dit quelque chose :
l'appartenance, puis l'organisation, puis l'installation. Quand c'est la fenêtre de
l'organisation qui s'applique, les dates de début et de fin de l'appartenance se
superposent quand même par-dessus. Le défaut ouvre tous les jours, vingt-quatre
heures sur vingt-quatre.

> [!WARNING]
> La fenêtre est vérifiée **à la connexion et au changement d'organisation, et nulle
> part ailleurs**. Une session déjà ouverte n'est pas coupée quand la fenêtre se
> ferme, et il n'y a pas de revérification en cours de session. FEATURES.md liste
> cela, et l'édition par membre dans la console, comme la moitié manquante.

Refuser sur les horaires le dit clairement : cela ne ressemble jamais à un mauvais
mot de passe.

## Ce qu'une organisation peut surcharger

| Réglage | Niveaux, du plus fin au plus large | Modifiable où |
|---|---|---|
| Durée de session | appartenance, organisation, installation | l'installation dans la console ; les deux autres par l'API d'administration |
| Heures ouvrées | appartenance, organisation, installation | l'organisation dans la console |
| Mode de groupe | l'organisation seulement | l'écran de l'organisation |
| Second facteur | **le compte, puis l'installation** | les deux dans la console |

Le second facteur n'a délibérément **pas de niveau organisation** : il est demandé
avant que l'organisation soit connue, donc une règle par organisation ne pourrait pas
être lue à temps.

Tout le reste - la politique de mot de passe, l'étranglement, les passkeys, le thème,
les langues, le relais - est à l'échelle de l'installation.

## L'isolation

Chaque requête porte l'organisation dans laquelle elle est faite, et c'est contre
cela que les règles de routes, les règles d'endpoints et l'identité transmise à
l'amont sont jugées. Votre service reçoit l'organisation comme un fait sur l'appelant,
et non comme quelque chose que l'appelant a demandé.

Une chose à savoir : **désactiver une organisation, ou une appartenance, ne termine
pas les sessions qui y travaillent déjà.** Cela empêche les nouvelles connexions de
l'adopter, mais une session qui porte déjà cette organisation la garde, et y garde
ses rôles, jusqu'à son expiration.

Ce qui prend effet tout de suite, c'est de retirer quelqu'un d'un **groupe** : ses
rôles sont recalculés à la requête suivante. Le moyen le plus rapide de retirer un
accès immédiatement est donc de vider les groupes, pas de désactiver l'appartenance.
