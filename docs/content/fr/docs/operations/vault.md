---
title: Le coffre
section: Exploitation
order: 227
summary: Des secrets chiffrés et des valeurs en clair dans un même espace de noms, les références $name, et le piège de la clé maîtresse en cluster.
---

# Le coffre

Le coffre est l'unique endroit vers lequel le reste de la configuration renvoie, plutôt
que de porter une valeur en dur. Il contient deux types d'entrées dans **un même espace
de noms** :

| Type | Au repos | En lecture |
|---|---|---|
| **secret** | chiffré, AES-256-GCM | jamais en clair. L'API indique si un secret est défini, pas ce qu'il contient |
| **valeur** | texte en clair | lisible - un nom d'hôte, un nom d'en-tête, un nom de compte |

Tout l'intérêt est de les réunir : un seul écran pour tout ce à quoi la configuration
fait référence, et une réponse visible à la question "qu'est-ce qui est réellement
utilisé ?". Seul le chiffrement diffère, et la syntaxe des références est la même : faire
d'une valeur un secret ne touche donc jamais aux objets qui la désignent.

L'écran s'appelle **Vault**, une entrée transversale à part entière.

![L'écran du coffre](img/console/vault.webp)

## Références

Dans la configuration, une valeur désigne une entrée par son nom :

```yaml
upstream: "http://${checkout-host}:8080"
password: "$smtp-password"
```

- `$name` et `${name}` sont la même référence ; les accolades permettent de l'accoler à
 ce qui suit.
- `$$` donne un `$` littéral.
- Un nom qui ne correspond à aucune entrée est laissé **tel quel** et signalé : une
 faute de frappe apparaît ainsi pour ce qu'elle est. La remplacer en silence par une
 chaîne vide produirait un upstream vide ou un mot de passe vide, dont les échecs sont bien
 plus déroutants.

Les **secrets** obéissent à une règle supplémentaire : seule une valeur constituée
*entièrement* d'une unique référence compte comme une référence. Un upstream se construit
autour de ses références, un fragment y est donc normal ; un mot de passe, lui, n'est
pas un fragment. `${a}${b}` et `x-$token` sont par conséquent des littéraux - c'est le
choix prudent, puisqu'une valeur dont on ne peut pas garantir qu'elle est une référence
est traitée comme un secret à protéger.

## Portées

Une entrée appartient à une portée, et **un nom est unique au sein d'une portée** - le
même `db-password` peut donc désigner deux choses différentes pour deux organisations.

| Portée | Qui l'administre | Ce qui s'y résout |
|---|---|---|
| `infra` | gateway-admin | les routes, les arguments des filtres, les identifiants des upstreams |
| `app` | app-admin | le relais de messagerie, les fournisseurs d'identité |
| une organisation | les administrateurs de cette organisation | ses propres entrées |

La résolution respecte le même cloisonnement, et c'est ce qui empêche l'administrateur
d'un plan de modifier discrètement ce que résout un autre plan : une route ne se résout
jamais qu'à partir des entrées `infra`. Une organisation cherche d'abord dans ses
propres entrées, puis se rabat sur celles de l'application : elle peut donc **utiliser**
une valeur globale sans pouvoir la modifier, et la masquer en déclarant la sienne sous le
même nom.

## Un champ sensible passe toujours par le coffre

Dans la console, un champ qui contient un secret a quatre états, et la saisie d'une
valeur bloque l'enregistrement tant qu'elle n'a pas été rangée dans le coffre. Un
littéral hérité d'un fichier d'amorçage ou d'un enregistrement plus ancien est déplacé
**côté serveur** : le navigateur envoie un nom, pas une valeur - il n'a jamais reçu ce
littéral et ne pourrait pas le ranger lui-même.

Une référence est publique. Un littéral ne l'est jamais.

## La clé maîtresse

| D'où elle vient | Comment |
|---|---|
| `MEERKAT_VAULT_KEY` | trente-deux octets, sous la forme de soixante-quatre caractères hexadécimaux ou en base64 |
| à défaut | un fichier `vault.key` dans le répertoire de données, en mode `0600`, généré au premier démarrage |

Le fichier généré se trouve à côté de la base de données : il protège donc une **base
volée ou copiée** - une sauvegarde, un export - et non un répertoire de données volé.
Fournir la clé par l'environnement évite de l'écrire sur le disque, et la console
indique lequel des deux cas s'applique à votre installation.

Cette même clé ne scelle pas que le coffre : les **clés privées des certificats**, la
**clé du compte ACME**, la **clé d'hôte** du tunnel développeur et le **secret TOTP** de
chaque compte passent eux aussi par elle. Un secret TOTP est un mot de passe qui ne
change jamais - quiconque le lit peut calculer tous les codes que le compte acceptera à
l'avenir - de sorte qu'une base copiée n'en contient aucun en clair. Ceux qui ont été
écrits auparavant sont scellés au démarrage suivant.

> [!WARNING]
> **La clé reste un fichier local, même avec une base de données externe.** C'est
> voulu : elle scelle le contenu de la base, et la ranger *dans* la base reviendrait à
> verrouiller la porte en laissant la clé dans la serrure. En cluster, la conséquence
> est un piège : chaque nœud génère sa propre clé au premier démarrage, et aucun nœud ne
> peut alors ouvrir ce qu'un autre a scellé. Le message d'erreur le dit - *the private
> key cannot be unsealed - is this the vault key it was written with?* Donnez à
> `MEERKAT_VAULT_KEY` la même valeur sur tous les nœuds, avant le premier démarrage.

### La renouveler

Un redémarrage avec deux clés :

1. Placez la nouvelle clé dans `MEERKAT_VAULT_KEY` et celle qu'elle remplace dans
   `MEERKAT_VAULT_KEY_PREVIOUS` (si vous utilisez le fichier généré, l'ancienne clé est
   son contenu). Le chart Helm prévoit `vault.previousKey` à cet effet.
2. Redémarrez les nœuds. Chacun lit les deux clés, et le premier à démarrer scelle de
   nouveau l'ensemble avec la nouvelle. Le journal indique combien de valeurs il a
   converties.
3. Quand tous les nœuds fonctionnent avec la nouvelle clé, retirez
   `MEERKAT_VAULT_KEY_PREVIOUS`.

Rien ne devient illisible dans l'intervalle. Un nœud démarré avec la nouvelle clé
seule, avant que les autres ne soient passés à la nouvelle, échoue sur la première
valeur scellée qu'il lit, avec la même erreur que pour une mauvaise clé.

## Le coffre sous forme de fichier

L'exact opposé d'un export de configuration, à tous points de vue. Il contient les
valeurs elles-mêmes, il est chiffré avec une phrase de passe que la gateway ne
conserve jamais, et il n'est **pas** fait pour être versionné : il sert à amorcer un
environnement ou à déménager une gateway, puis il se supprime.

Le fichier décrit en clair sa propre recette - version du format, dérivation de clé et
ses paramètres, sel, nonce - parce qu'il survivra de plusieurs années au binaire qui l'a
produit, et qu'un paramètre modifié dans une version ultérieure ne doit pas rendre un
ancien export illisible. La dérivation est Argon2id, et la phrase de passe compte au
minimum douze caractères ; la console propose d'en générer une, justement pour que
personne ne saisisse le nom du produit suivi de l'année.

Une gateway peut en charger un au démarrage : `-vault`, avec la phrase de passe dans
`MEERKAT_VAULT_PASSPHRASE` ou `MEERKAT_VAULT_PASSPHRASE_FILE`.

## Dates de rappel

Une entrée peut porter une **date de rappel** : le jour où l'on sait que son secret
expire à la source - un jeton, un certificat.

Ce n'est qu'un rappel, qui ne change rien. La gateway ne peut pas savoir si un jeton
a été renouvelé chez le fournisseur : la référence `$name` continue donc d'être
résolue. La date alimente seulement le récapitulatif quotidien, qui liste ce qui arrive
à échéance et ce qui vient d'y arriver, jamais la valeur.

## Ce qui manque

- **Un backend externe** : pas de HashiCorp Vault, ni de secrets Kubernetes ou Docker
 comme autre source.
