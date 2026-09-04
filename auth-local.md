# Tester l'authentification en local

> **Role** : la procedure, de la gateway eteinte a une connexion par annuaire qui
> ressort avec des roles. Comptes, mots de passe, ports, et ce qu'on doit voir a
> chaque etape.
>
> Ce fichier dit **comment faire tourner**. Pour **ce que le produit fait** (les portes
> d'entree, les leviers, ce qui est livre ou pas) : `FEATURES.md`.
> Pour le cycle de developpement courant : `DEV.md`.
>
> Derniere execution de bout en bout : 2026-08-17.

## 1. Les briques et leurs ports

| Quoi | Adresse | Sert a |
|---|---|---|
| Console (ng serve) | `http://localhost:4200` | l'interface, servie a travers le port admin |
| Gateway, plan **donnees** | `http://localhost:8082` | les pages de connexion, le proxy des applications |
| Gateway, plan **admin** | `http://localhost:9092` | la console et l'API d'administration |
| OpenLDAP | `ldap://localhost:3389` | l'annuaire de test |
| Active Directory (Samba) | `ldaps://localhost:3636` | le meme jeu, dans l'autre dialecte |
| Dex (OIDC) | `http://localhost:5556/dex` | une vraie autorite OIDC |

Les deux plans sont **deux portes distinctes**, avec deux cookies de session. On
teste une connexion utilisateur sur **8082**, jamais sur 9092 : le plan admin
accepte toujours le mot de passe local, par construction (voir `FEATURES.md`, AUTH-24).

## 2. Monter la gateway

Deux terminaux, comme dans `DEV.md` :

```bash
# terminal 1 - la console
cd console && npm install && npm start

# terminal 2 - la gateway
MEERKAT_ADMIN_PASSWORD=test1234 air -- \
  -addr :8082 -admin-addr :9092 -console-url http://localhost:4200
```

Puis `http://localhost:9092`, compte **`admin` / `test1234`**.

Le mot de passe admin n'est seme qu'au **premier** demarrage sur un repertoire
`data/` vierge. Oublie : `rm -rf data/` et relancer.

Pour une instance jetable qui n'ecrase rien :

```bash
MEERKAT_ADMIN_PASSWORD=test1234 go run ./cmd/meerkat \
  -addr :18082 -admin-addr :19092 -data "$(mktemp -d)"
```

## 3. Du contenu a regarder

```bash
go run ./cmd/seed-demo
```

Routes, organisations et comptes de demonstration, idempotent. Quatre personnes,
**mot de passe `demo-Pass-1234`** pour toutes :

| Compte | Organisations | Ce qu'il montre |
|---|---|---|
| `marc` | acme-demo, globex-demo | deux organisations, dont une en mode exclusif |
| `nadia` | globex-demo | deux groupes en mode exclusif : le choix a la connexion |
| `leo` | initech-demo, globex-demo | un seul groupe de chaque cote : le choix est silencieux |
| `zoe` | acme-demo | le cas simple, cumulatif |

## 4. Le banc d'annuaires

```bash
make ldap-up      # demarre et seme OpenLDAP, Active Directory et Dex
make ldap-test    # les tests d'integration Go contre les trois
make ldap-down    # arrete et oublie tout
```

Le premier demarrage prend une minute : le controleur de domaine se provisionne.

Les deux annuaires portent **la meme fixture**, parce que LDAP et AD ne se
ressemblent que de loin : AD se lie par UPN, indexe sur `sAMAccountName`, expose
`memberOf` et resout l'imbrication avec sa propre regle.

Neuf personnes, **mot de passe `password`** (cote AD : `Passw0rd!2026`), chacune
pour une question :

| Personne | Groupes annuaire | Ce qu'elle sert a voir |
|---|---|---|
| `alice` | frontend | une equipe et rien d'autre |
| `bob` | aucun | se connecte, n'obtient rien |
| `carla` | backend | `developer` **uniquement** par imbrication |
| `dan` | aucun | connue de l'annuaire, refusee a la porte |
| `evec` | frontend | login different du mail, prenom accentue (Eve Chevalier) |
| `frank` | Brest Agents | un nom de groupe avec espace et majuscules |
| `gina` | operator | vit dans `ou=partners`, hors de `ou=users` |
| `johndoe` | frontend, backend, operator | celle sous laquelle se lient les tests Go |
| `janedoe` | devops, frontend, operator | |

Le groupe `developer` ne contient **aucune personne** : il contient les trois
groupes d'equipe. Un client qui lit l'appartenance a plat en ressort vide.

`dan` est refuse de deux facons selon le dialecte : sans mot de passe cote
OpenLDAP, reellement desactive cote AD. Meme issue, deux routes.

## 5. Brancher l'annuaire sur la gateway

```bash
make ldap-demo
```

C'est le dernier metre, celui qui manquait : sans lui il faut retaper une URL,
une base de recherche, un compte de service, un filtre et une base de groupes
dans la console, puis inventer des roles et des groupes pour avoir quelque chose
a regarder.

La commande enregistre l'autorite, **demande a la gateway de la verifier**
elle-meme, puis pose la chaine complete :

- quatre roles : `demo-front`, `demo-back`, `demo-ops`, `demo-read` ;
- quatre groupes qui les portent : `Front`, `Back`, `Ops`, `Agents Brest` ;
- quatre regles de groupe (RBAC-10) : `frontend` donne `Front`, `backend` donne
  `Back`, `devops` donne `Ops`, `Brest Agents` donne `Agents Brest`.

`developer` et `operator` ne sont **volontairement pas** mappes : ils sont
collectes, ils n'accordent rien, et c'est ce que fait la plupart des noms qu'un
annuaire rapporte.

Tout est cherche par nom et cree seulement s'il manque : relancer ne change rien.

Variables, quand la valeur par defaut ne convient pas :

| Variable | Defaut | Quand la poser |
|---|---|---|
| `MEERKAT_ADMIN_URL` | `http://localhost:9092` | instance sur d'autres ports |
| `MEERKAT_ADMIN_USER` / `MEERKAT_ADMIN_PASSWORD` | `admin` / `test1234` | autre compte root |
| `MEERKAT_LDAP_URL` | `ldap://localhost:3389` | gateway en conteneur : `ldap://host.docker.internal:3389` |
| `MEERKAT_TENANT` | `default`, sinon la premiere | choisir l'organisation qui recoit groupes et regles |

## 6. Ce que l'on doit obtenir

Se connecter sur **`http://localhost:8082/login`**, avec le formulaire ordinaire :
une autorite annuaire n'a pas de bouton a elle, elle est essayee avec ce qui a ete
tape une fois que le mot de passe local a dit non.

| Compte | Resultat attendu | Roles |
|---|---|---|
| `alice` | entre | `demo-front` |
| `carla` | entre | `demo-back` seul : son `developer` est collecte, aucune regle ne le prend |
| `johndoe` | entre | `demo-front`, `demo-back` |
| `janedoe` | entre | `demo-ops`, `demo-front` |
| `evec` | entre | `demo-front` |
| `frank` | entre | `demo-read` |
| `bob` | `/account-pending` | rien : aucune regle ne le concerne |
| `gina` | `/account-pending` | rien : `operator` n'est pas mappe |
| `dan` | refuse sur la page de connexion | |

Les trois premieres lignes ont ete verifiees dans un navigateur le 2026-08-17.

L'attente n'est pas une panne : **une autorite prouve qui on est, elle ne decide
jamais de ce qu'on peut faire ici**. Sans regle qui le concerne, un arrivant est
cree, les admins sont prevenus, et il attend qu'on le place.

## 7. Passer sur Active Directory

Meme fixture, autre dialecte. Dans la console, **Infra / Authentication**, en
modifiant l'autorite posee par `make ldap-demo` :

| Champ | Valeur |
|---|---|
| Dialecte | `Active Directory` |
| URL | `ldaps://localhost:3636` |
| Base de recherche | `DC=ad,DC=example,DC=com` |
| Compte de service | `Administrator@ad.example.com` |
| Mot de passe | `Passw0rd!2026` |
| Certificat | accepter le certificat auto-signe (le controleur se signe pour `dc1.ad.example.com`, on l'atteint par `localhost`) |

Les mots de passe des personnes sont `Passw0rd!2026` de ce cote.

## 8. Les autres portes

**OIDC** : Dex tourne deja sur `http://localhost:5556/dex`, client `meerkat`,
secret `meerkat-secret`. Son URI de redirection est declaree pour le port `9099`
dans `test/ldap/dex/config.yaml` : pour une autorite branchee sur une gateway a
un autre port, ajouter l'URI correspondante dans ce fichier et redemarrer Dex.
Son connecteur `mockCallback` signe sans formulaire, ce qui rend l'aller-retour
testable sans piloter un navigateur.

**GitHub** : une vraie application OAuth, avec l'URL de rappel que la console
affiche et qu'on colle chez GitHub. Rien a monter en local.

**Passkeys et MFA** : les leviers sont globaux (Application puis Security), avec
un remplacement possible par autorite. Un authentificateur virtuel est necessaire
pour automatiser les passkeys ; a la main, le trousseau du systeme suffit.

## 9. Ou regarder dans la console

- **Application / Users** : les arrivants, y compris ceux en attente.
- **Group rules** : sous `Application` quand l'installation n'a qu'une
  organisation, sous `Tenants` puis l'organisation quand elle en a plusieurs.
  Les regles, et l'ecran qui liste
  les noms de groupes que les autorites ont **reellement** ete entendues dire.
  Une regle ecrite de memoire dans une autre casse ne correspond a rien, en
  silence : c'est exactement ce que cet ecran existe pour eviter.
- **Application / Groups** : la matrice groupes et roles.
- **Audit** : chaque mutation d'administration, avec l'ecart champ par champ.

## 10. Nettoyer

```bash
make ldap-down          # les trois serveurs et leurs volumes
rm -rf data/            # la base de la gateway de dev (jetable)
```

## 11. Pieges vecus

- `POST /login`, jamais `/auth/login` : cette seconde adresse n'existe pas et,
  en developpement, tombe dans le proxy du serveur front (reponse HTML avec
  `X-Powered-By: Express`, tres deroutante).
- Les pages servies lisent theme, marque et disposition a travers un **cache de
  cinq secondes**. Un reglage change dans la console met ce delai a apparaitre
  sur `/login` ; l'apercu de la console, lui, bascule tout de suite.
- **Un bind qui echoue tue le processus entier** : plus rien ne repond nulle part.
  `:9090` est deja pris sur cette machine par une autre gateway, d'ou
  `-admin-addr :9092`.
- Le plan **admin** accepte toujours le mot de passe local. Verifier la fermeture
  d'une porte se fait sur le plan **donnees**, port 8082.
- Docker monte un **fichier** par son inode : remplacer `seed.sh` sur l'hote
  pendant que le conteneur tourne ne change rien dedans, il faut recreer le
  conteneur. Symptome vecu : une erreur de syntaxe shell a une ligne qui
  n'existe plus.
- Cote AD, `--userou` de `samba-tool` est **relatif** au domaine et la base y est
  recollee toute seule. Lui donner le DN complet construit
  `OU=partners,DC=ad,DC=example,DC=com,DC=ad,DC=example,DC=com` et echoue sur
  "parent does not exist".
