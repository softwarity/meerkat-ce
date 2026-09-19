---
title: Le coffre
section: Exploitation
order: 227
summary: Secrets chiffrés et valeurs en clair dans un seul espace de noms, les références $nom, et le piège de la clé maîtresse en cluster.
---

# Le coffre

Le coffre est le seul endroit que le reste de la configuration désigne au lieu de porter une valeur en
ligne (VAULT-01). Il tient deux genres d'entrée dans **un seul espace de noms** :

| Genre | Au repos | À la relecture |
|---|---|---|
| **secret** | chiffré, AES-256-GCM | jamais en clair. L'API dit si un secret est posé, pas ce qu'il vaut |
| **valeur** | texte en clair | lisible - un nom d'hôte, un nom d'en-tête, un nom de compte |

Tenir les deux au même endroit est l'intérêt : un seul écran pour tout ce à quoi la configuration se
réfère, et une réponse visible à « qu'est-ce qui sert vraiment ». Seul le chiffrement diffère, et la
syntaxe de référence est la même - donc promouvoir une valeur en secret ne touche jamais les objets qui
la désignent.

L'écran est **Vault**, une entrée transverse à lui.

![L'écran du coffre](img/console/vault.webp)

## Les références

Une valeur de la configuration désigne une entrée par son nom :

```yaml
upstream: "http://${checkout-host}:8080"
password: "$smtp-password"
```

- `$nom` et `${nom}` sont la même référence ; les accolades lui permettent de coller à ce qui suit.
- `$$` est un `$` littéral.
- Un nom qui ne résout pas est laissé **tel quel** et signalé, donc une faute de frappe se voit
  elle-même. La transformer silencieusement en chaîne vide produirait un amont vide ou un mot de passe
  vide, qui échouent de façons beaucoup plus déroutantes.

Pour un **secret** il y a une règle de plus : seule une valeur qui est *entièrement* une référence compte
comme une référence. Un amont se construit autour de ses références, donc un fragment y est normal ; un
mot de passe n'est pas un fragment. `${a}${b}` et `x-$token` sont donc des littéraux - c'est le bon sens
de sécurité, puisqu'une valeur qu'on ne peut pas certifier comme référence est traitée comme un secret à
protéger.

## Les portées

Une entrée appartient à une portée, et **un nom est unique par portée** - donc le même `db-password` peut
vouloir dire deux choses différentes pour deux organisations.

| Portée | Qui l'administre | Ce qui résout dedans |
|---|---|---|
| `infra` | gateway-admin | les routes, les arguments de filtre, les identifiants d'amont |
| `app` | app-admin | le relais mail, les fournisseurs d'identité |
| une organisation | ses administrateurs | ses propres entrées |

La résolution honore le même découpage, et c'est ce qui empêche l'administrateur d'un plan de changer
discrètement ce qu'un autre plan résout : une route ne s'étend jamais que contre les entrées `infra`. Une
organisation résout d'abord les siennes et retombe sur celles de l'application, donc elle peut **utiliser**
une valeur globale sans pouvoir l'éditer, et l'ombrer en déclarant la sienne sous le même nom.

## Un champ sensible passe toujours par le coffre

Un champ qui porte un secret a quatre états dans la console, et la saisie bloque l'enregistrement jusqu'à
ce que la valeur soit rangée (VAULT-05). Un littéral hérité d'un fichier d'amorçage ou d'un ancien
enregistrement est déplacé **côté serveur** : le navigateur envoie un nom, pas une valeur - il n'a jamais
reçu ce littéral et ne pourrait pas le ranger lui-même.

Une référence est publique. Un littéral ne l'est jamais.

## La clé maîtresse

| D'où elle vient | Comment |
|---|---|
| `MEERKAT_VAULT_KEY` | trente-deux octets, en soixante-quatre caractères hexadécimaux ou en base64 |
| sinon | un fichier `vault.key` dans le répertoire de données, mode `0600`, généré au premier démarrage |

Le fichier généré est posé à côté de la base, donc il protège une **base volée ou copiée** - une
sauvegarde, un export - et non un répertoire de données volé. Fournir la clé par l'environnement la garde
entièrement hors du disque, et la console dit laquelle des deux fait votre installation.

La même clé scelle plus que le coffre : les **clés privées des certificats** et la **clé de compte ACME**
passent par elle aussi.

> [!WARNING]
> **La clé reste un fichier local même avec une base externe.** C'est délibéré : elle scelle ce qui est
> dans la base, donc la mettre *en* base reviendrait à sceller la porte avec la clé dans la serrure. La
> conséquence en cluster est le piège : chaque noeud génère la sienne au premier démarrage, et un noeud ne
> peut alors pas ouvrir ce qu'un autre a scellé. L'erreur le nomme - *the private key cannot be unsealed -
> is this the vault key it was written with?* Posez `MEERKAT_VAULT_KEY` à la même valeur sur tous les
> noeuds, avant le premier démarrage.

## Le coffre comme fichier

L'exact inverse d'un export de configuration, sur tous les points (VAULT-03). Il porte les valeurs
elles-mêmes, il est chiffré par une phrase de passe que la passerelle ne stocke jamais, et ce n'est **pas**
quelque chose à versionner : il existe pour amorcer un environnement ou déménager une passerelle, puis pour
être supprimé.

Le fichier écrit sa propre recette en clair - version du format, dérivation de clé, ses paramètres, le sel,
le nonce - parce qu'il survivra des années au binaire qui l'a produit, et qu'un paramètre changé dans une
version ultérieure ne doit pas rendre un vieil export illisible. La dérivation est Argon2id et la phrase de
passe a un plancher de douze caractères ; la console propose d'en générer une, précisément pour que personne
ne tape le nom du produit suivi de l'année.

Une passerelle peut en ingérer un au démarrage : `-vault`, avec la phrase de passe dans
`MEERKAT_VAULT_PASSPHRASE` ou `MEERKAT_VAULT_PASSPHRASE_FILE`.

## Les dates de rappel

Une entrée peut porter une **date de rappel** : le jour où son secret expire à sa source - un jeton, un
certificat (VAULT-06).

C'est purement un rappel et ça ne change rien. La passerelle ne peut pas savoir qu'un jeton a été renouvelé
chez le fournisseur, donc la référence `$nom` continue de résoudre. La date ne fait que nourrir le digest
quotidien, qui liste ce qui approche et ce qui vient de passer, jamais la valeur.

## Ce qui manque

- **La rotation de la clé maîtresse** (VAULT-02) : il n'y a pas de ré-encryption globale, donc il n'y a pas
  de rotation.
- **Un backend externe** (VAULT-04) : ni HashiCorp Vault, ni secrets Kubernetes ou Docker comme source
  alternative.
- **Les secrets TOTP ne sont pas chiffrés au repos** (SEC-06). Ils ne passent pas par le coffre ; c'est
  écrit comme un manque connu, pas comme un détail.
