---
title: Comptes locaux
section: Authentification
order: 102
summary: Les comptes que Meerkat gère lui-même - politique de mot de passe, limitation des tentatives de connexion et création de compte par les utilisateurs.
---

# Comptes locaux

L'autorité locale est celle pour laquelle Meerkat répond seul : un identifiant, l'empreinte
d'un mot de passe qu'il conserve, et rien à demander à qui que ce soit. Elle est activée au
premier démarrage, et le compte root créé à cette occasion en dépend.

Elle figure dans **Infra > Authentication** sous la forme d'une ligne nommée *Local
accounts*, à côté des fournisseurs d'identité et des annuaires. C'est en la désactivant que
vous rendez les autres autorités exclusives : tant qu'elle répond, chaque mot de passe
local est une porte qui permet de les contourner.

> [!NOTE]
> Le port d'administration ne ferme jamais cette porte. C'est depuis la console que l'on
> répare une autorité en panne : la placer derrière cette même autorité, c'est s'exposer à
> perdre la main sur l'installation au pire moment.

## La politique de mot de passe

Il n'y a qu'une politique pour toute l'installation, dans **Application > Security**. Un
mot de passe est contrôlé là où il est saisi, et les endroits où on le saisit - le
formulaire d'inscription, le changement imposé à la première connexion, le profil, un lien
de réinitialisation - ignorent tout des organisations.

| Champ | Ce qu'il exige | Par défaut |
|---|---|---|
| Length | le nombre minimal de caractères | `8` |
| Lowercase, Uppercase, Digits, Special characters | le nombre minimal de caractères de chaque catégorie | `0` |
| No reuse of the last | le nombre d'anciens mots de passe refusés | `0` |
| Expires after (days) | le nombre de jours au bout desquels le changement est imposé | `0`, jamais |
| Temporary (hours) | la durée de validité d'un mot de passe attribué par un administrateur | `72` |

Un zéro signifie *sans exigence* : la règle n'est ni contrôlée ni affichée. La
configuration d'origine n'exige donc qu'une longueur, et c'est voulu : durcir les règles
pour tout le monde à l'occasion d'une mise à jour bloquerait les personnes au beau milieu
du changement de mot de passe qu'elles étaient en train de faire.

Trois choses à savoir avant de remplir les champs :

- La longueur se compte en **caractères, pas en octets** : une phrase de passe en grec ou en japonais n'est pas trois fois plus longue qu'elle n'en a l'air.
- Une lettre d'une écriture sans casse - kana, chinois, arabe, hébreu - ne compte ni comme minuscule, ni comme majuscule, ni comme caractère spécial.
- Si la longueur demandée est inférieure à la somme des quatre catégories, elle est portée à cette somme : quatre catégories à deux caractères chacune en demandent huit, et une liste de règles impossible à satisfaire est pire que pas de liste du tout.

La console affiche la liste des règles exactement comme les pages de connexion la
présenteront. La même fonction sert des deux côtés, et ce n'est pas un hasard : les gens
croient ce que la liste affiche, y compris quand le serveur applique autre chose.

**L'expiration se vérifie à la connexion, pas à heure fixe.** Un mot de passe qui expire à
trois heures du matin ne déconnecte personne : c'est la connexion suivante qui est refusée
et qui aboutit à la page de changement de mot de passe. Le même écran propose un bouton
*Force a change for everyone* pour le jour où vous en aurez besoin.

**Un mot de passe temporaire a une durée limitée.** Le mot de passe qu'un administrateur
attribue, à la création du compte ou lors d'une réinitialisation, fonctionne pendant la
durée *Temporary (hours)*, ou jusqu'à ce que son titulaire choisisse le sien. Passé ce
délai, la connexion est refusée avec un message qui invite à en demander un nouveau à un
administrateur. Ce mot de passe circule par e-mail, par messagerie ou par téléphone : s'il
n'a jamais servi, il ne doit pas rester un moyen d'entrer. *Force a change* n'est pas
concerné : le mot de passe reste celui de son titulaire.

Les mots de passe sont hachés avec bcrypt. Une empreinte calculée avec un coût plus faible
est recalculée à la connexion suivante de son titulaire, seul moment où le mot de passe est
connu en clair.

## Une limitation, jamais un verrouillage

Les échecs de connexion sont comptés deux fois, sur une fenêtre glissante, dans
**Application > Security > Rate limiting** : par **adresse du client et par compte**, et
par **compte** quelle que soit l'adresse.

| Champ | Ce qu'il compte | Par défaut |
|---|---|---|
| Failed sign-ins from one address | les échecs d'une adresse sur un compte, avant que le formulaire de connexion réponde `429` | `10` |
| Failed sign-ins on one account | les échecs sur un compte depuis n'importe quelle adresse, avant que sa connexion demande aussi un code en image | `5` |
| Wrong 2FA codes | les codes de second facteur erronés tolérés, sur la même fenêtre | `5` |
| Within | la fenêtre, sous forme de durée ISO-8601 | `PT15M` |

Une connexion réussie remet les deux compteurs à zéro. Le refus intervient **avant** la
comparaison des empreintes : un attaquant bloqué ne peut donc même pas consommer du temps
de processeur. Mettre un nombre à zéro désactive la limite correspondante.

Le compteur est tenu dans la base de données, pas dans le processus :

- dix tentatives, c'est dix pour toute l'installation et non dix par gateway - un attaquant qui répartit ses essais derrière un load balancer ne voit plus la limite multipliée par le nombre de nœuds ;
- un redémarrage n'efface plus l'ardoise de celui qui était limité.

Si la base de données ne répond pas, la tentative passe au lieu d'être refusée : la
connexion allait de toute façon avoir besoin de cette même base, et un incident passager ne
doit pas se transformer en blocage général.

**Aucun compte n'est jamais verrouillé.** Rien de ce que fait un tiers ne peut priver de
son compte la personne à qui il appartient.

### Beaucoup d'adresses, un seul compte

Le compteur par adresse arrête une machine qui devine. Il ne voit pas mille machines qui
devinent chacune un peu - un botnet, un lot de proxys qui tournent - puisque chaque adresse
reste indéfiniment sous son propre budget. Ce qu'elles ont toutes en commun, c'est le nom
qu'elles essaient : c'est ce que compte le second compteur.

Au-delà de **Failed sign-ins on one account**, la page de connexion de ce nom demande le
même code en image que le [formulaire d'inscription](#laisser-les-gens-crer-leur-compte),
avec le nom déjà rempli :

- sans le bon code, aucun mot de passe n'est même vérifié, et la page dit que le code manque plutôt que "mauvais mot de passe" - celui qui a saisi le bon ne doit pas s'entendre dire qu'il est faux ;
- le code est consommé quelle que soit la réponse, et chaque page en porte un nouveau ;
- le bon mot de passe **et** le bon code connectent, et remettent le compteur à zéro.

Trois secondes pour une personne, toute l'économie d'un script - et le compte reste
ouvert à son titulaire pendant tout ce temps. Zéro désactive ce compteur.

Un **nom inconnu est traité de la même façon** : même compteur, même code, mêmes phrases.
Faire autrement répondrait à la seule question que la page refuse partout ailleurs - ce
compte existe-t-il.

**Le titulaire est prévenu.** Quand un compte franchit le seuil et que SMTP est configuré,
son titulaire reçoit un e-mail - combien d'échecs dans la fenêtre, depuis combien
d'adresses, et de changer son mot de passe si ce n'était pas lui. Un seul par fenêtre,
aussi longtemps que dure l'assaut, et aucun pour un nom qui n'existe pas, un compte
désactivé ou une adresse qu'un compte auto-inscrit n'a jamais confirmée. L'e-mail part
après la réponse, jamais avant : un refus qui attendrait le relais prendrait sensiblement
plus de temps sur un vrai compte que sur un nom inventé. L'écran du relais sait en envoyer
un exemple (**Failed sign-ins**).

### Quelle adresse est comptée

L'adresse est celle du **pair TCP** - l'autre bout de la connexion que la gateway a
acceptée -, sauf si ce pair est un proxy de confiance : c'est alors ce que ce proxy a
écrit dans `X-Forwarded-For`. Un appelant qui joint la gateway directement et écrit
l'en-tête lui-même est ignoré : sinon, inventer une adresse par requête suffirait à
traverser tous les compteurs de cette page. Les proxys de confiance sont déduits de leur
adresse par défaut, et se règlent avec `MEERKAT_TRUSTED_PROXIES` - voir
[Derrière un proxy ou un load balancer](/docs/deploy/one-gateway#derrire-un-proxy-ou-un-load-balancer).
Le journal de sécurité garde les deux, "203.0.113.5 via 10.42.0.15", pour ne jamais
montrer seulement ce qu'un en-tête prétendait.

Un identifiant inconnu, un mot de passe erroné et un compte désactivé reçoivent tous la
même réponse, dans le même temps : pour un identifiant inconnu, une empreinte factice est
tout de même comparée, si bien que le temps de réponse ne permet de rien distinguer. Seule
exception, un compte en dehors de sa période d'accès : à la personne qui a saisi le **bon**
mot de passe, la page indique la date. Sans elle, chaque expiration se solderait par un
appel au support pour demander la seule chose que la page aurait pu dire.

## Laisser les gens créer leur compte

L'auto-inscription est fermée par défaut. Quand elle est ouverte, la gateway sert
`/register` : un identifiant, une adresse, un mot de passe contrôlé selon la politique
ci-dessus, et un code affiché dans une image à recopier.

Quatre conditions doivent être réunies pour que cette page existe :

1. l'autorité locale est **activée** ;
2. l'auto-inscription lui est permise - soit par *Allowed* sur l'autorité elle-même, soit par *Inherited from the application* avec l'interrupteur situé en haut de **Infra > Authentication** activé ;
3. un relais de messagerie est configuré dans **Infra > Mail relay**, car l'adresse doit être confirmée ;
4. la requête arrive sur le plan de données. Le port d'administration ne sert jamais de formulaire d'inscription.

> [!WARNING]
> L'interrupteur en haut de l'écran Authentication fixe une **valeur par défaut**, il ne
> verrouille rien. Une autorité dont la politique propre indique *Allowed* continue de
> créer des comptes même quand cet interrupteur est désactivé. Pour fermer
> l'auto-inscription pour de bon, réglez l'autorité elle-même sur *Refused*.

Une inscription ne donne volontairement accès à rien : le compte existe, mais il reste
inutilisable tant que l'adresse n'est pas confirmée, et il n'a accès à rien tant qu'un
administrateur ne l'a pas rattaché à une organisation et ne lui a pas attribué de rôles. Le
lien de confirmation est un jeton à usage unique, valable 24 heures, 48 heures ou 7 jours
selon le réglage **Confirmation links valid for**, à côté de l'interrupteur
d'auto-inscription. Une personne qui se connecte avant d'avoir confirmé son adresse reçoit
de nouveau le lien, plutôt qu'une explication.

Un identifiant ou une adresse déjà utilisés mènent à la **même** page qu'une inscription
réussie, et rien n'est créé : le formulaire ne révèle pas à un inconnu qui possède déjà un
compte ici. Le code en image est activé par défaut et se désactive autorité par autorité.
Il est consommé que la réponse soit juste ou fausse : un second essai suppose donc une
nouvelle image.

Les endpoints d'écriture accessibles sans être connecté sont limités par adresse du
client : `/register` accepte cinq essais par quart d'heure, valeur fixe ;
`/forgot-password` a son propre compteur, **Reset requests** dans les rate limits
(cinq par défaut, sur la même fenêtre que les connexions, et impossible à désactiver). Les
deux ne partagent plus le même compteur : auparavant, une page d'inscription très
sollicitée freinait les réinitialisations, et une avalanche de réinitialisations fermait
l'inscription.

## Changer d'adresse

L'adresse est l'endroit où arrive une réinitialisation de mot de passe : c'est donc un
moyen d'entrer dans le compte. Quand un relais de messagerie est configuré, la modifier
depuis le profil ne la change **pas** tout de suite : un lien part vers la **nouvelle**
adresse, et le changement n'est effectif que lorsque ce lien est suivi - il est valable
aussi longtemps qu'un lien d'inscription. L'**ancienne** adresse est prévenue
immédiatement, ce qui alerte le titulaire si la demande ne vient pas de lui. Mettre la main
sur une session ne suffit donc plus à détourner la récupération du compte vers sa propre
boîte aux lettres. Sans relais, rien ne peut acheminer de confirmation : l'adresse change
alors immédiatement, comme auparavant.

## Un lien, ou un code

Chacun de ces mails prouve une boîte : la confirmation d'une inscription, une
nouvelle adresse, une réinitialisation. Un **lien** n'est envoyé que lorsque la
gateway peut se porter garante du nom qu'il contient, de l'une de deux façons :

- la requête est arrivée sur **sa propre porte TLS**, et un certificat placé sur
  cette porte (**Infra > Domains & TLS**, sur Console ou Application) couvre le
  nom demandé par le navigateur ;
- ou le nom est l'un de ceux que l'opérateur a déclarés sur cette porte comme
  **joints par un proxy qui termine le TLS** - nginx, un ingress, un load
  balancer placé devant. La requête arrive alors en HTTP simple, et le lien reste
  en `https://`.

Le lien pointe alors vers ce nom : une réinitialisation demandée sur
`shop.example.com` revient sur `shop.example.com`. Voir
[Derrière un proxy qui termine le TLS](/docs/operations/tls#derrire-un-proxy-qui-termine-le-tls).

Partout ailleurs - HTTP simple, un proxy devant dont personne n'a déclaré les noms,
un nom qu'aucun certificat placé ne porte - le mail contient un **code à six
chiffres** et aucun lien, et la page qui a fait la demande l'attend. C'est le
code de la connexion par e-mail : valable dix minutes, et seulement dans le
navigateur qui l'a demandé. Une nouvelle adresse se confirme par le code, dans
la session qui a demandé le changement.

C'est ce qui empêche un inconnu de retourner ces mails contre leur
destinataire. Le nom d'une requête est ce que l'appelant a écrit : une
réinitialisation demandée pour l'adresse de quelqu'un d'autre avec
`Host: attacker.example` envoyait à cette personne un vrai lien, avec un vrai
jeton, vers le serveur de l'attaquant. Un code envoyé à la victime ne sert à
rien à celui qui l'a demandé : il ne le voit jamais, et il n'ouvre rien dans un
autre navigateur.

## Un mot de passe oublié

`/forgot-password` existe dès qu'un relais de messagerie est configuré et que les mots de
passe locaux sont acceptés, sans attendre que l'auto-inscription soit ouverte. La page
affichée ensuite est la même, que l'adresse soit connue ou non. Un compte créé par une
autorité (OIDC, LDAP, GitHub) n'a pas de mot de passe local et ne reçoit aucun lien : cela
ouvrirait une seconde porte dont l'autorité ne saurait rien.

Le lien est valable une heure et ne sert qu'une fois (un code, dix minutes). Le nouveau mot de passe est contrôlé
selon la politique, règle de non-réutilisation comprise, et le changement **détruit toutes
les sessions en cours de ce compte**, sur toutes les gateways : quiconque en détenait
une, intrus compris, doit se reconnecter ou reste dehors. Le titulaire est prévenu par
e-mail que son mot de passe a changé.

La réinitialisation ferme aussi les autres portes que l'ancien mot de passe a pu ouvrir :
les **jetons d'API** du compte sont révoqués et ses **navigateurs de confiance** oubliés.
On réinitialise souvent un mot de passe parce que "quelqu'un d'autre le connaissait", et ce
quelqu'un a pu s'en servir pour créer un jeton.
