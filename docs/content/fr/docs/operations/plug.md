---
title: Plug
section: Operations
order: 228
summary: Ouvrir le tunnel développeur, dire aux développeurs où il est, et ce qu'ils lancent sous macOS, Linux et Windows.
---

# Plug

[plug](https://github.com/softwarity/plug) fait tourner le processus local d'un
développeur comme un membre du cluster. Son service répond sous son vrai nom et
joint les autres par les leurs : pas de port-forward, pas de changement de code.
Un service déployé du même nom est mis de côté pendant la session, puis remis.
À quoi cela sert, et comment tous ceux qui regardent l'application savent qui la
sert, est sur [Dev mode](/product/dev-mode). Cette page dit comment l'ouvrir et
comment l'installer ; tout le reste de ce que fait plug - profils, plusieurs
clusters à la fois, volumes, environnements - est dans
[la documentation de plug](https://softwarity.github.io/plug/).

> [!NOTE] Edition Enterprise
> Le tunnel est intégré à l'image Enterprise, tout comme le client plug qu'il
> distribue.

## Deux conditions, deux responsables

Le tunnel ne tourne que si **les deux** sont allumés :

| interrupteur | où | à qui | ce qu'il décide |
|---|---|---|---|
| Developer mode | Application, General | l'administrateur d'application | ce que l'installation offre à ses développeurs : le menu Developer, la doc des API, les connexions simulées |
| Open the developer tunnel | **Infra, Plug** | l'administrateur d'infrastructure | un port dans le cluster, et les droits du déploiement dessus (socket Docker, rôle Kubernetes) |

L'interrupteur du tunnel est **livré éteint**. Une passerelle déclarée production
(`MEERKAT_PRODUCTION`) garde toute la surface développeur fermée, quoi que
disent les deux interrupteurs.

Tant que le tunnel est fermé, on ne propose pas au développeur de déposer une
clé : la page n'existe pas, et le menu utilisateur ne la montre pas. Une clé
pour une porte absente est une clé dont personne ne peut se servir.

La page Infra, Plug dit ce que le tunnel **fait**, pas ce qui est coché : en
écoute sur son port, fermé et par quel interrupteur, ou allumé mais incapable de
tourner ici et pourquoi - une passerelle qui ne tourne pas dans un conteneur,
par exemple, ne peut pas créer de nom dans le cluster.

## L'adresse que les développeurs utilisent

La passerelle ne voit pas ce qui se trouve entre elle et un portable : un
NodePort, un LoadBalancer, un port Docker publié. La page Infra, Plug enregistre
donc l'hôte et le port **publiés**, et chaque commande de cette page et de la
page de profil du développeur les porte, prête à copier.

![L'écran Plug : l'interrupteur du tunnel, l'hôte et le port publiés, et qui a la capacité développeur](img/console/plug.webp)

Le tunnel écoute sur **22222** dans le conteneur (`MEERKAT_PLUG_ADDR`).

- **Docker Compose** : le publier, `22222:22222`.
- **Kubernetes** : le chart Helm crée un Service `-plug`, en ClusterIP par
  défaut. Mettre `plug.service.type` à `NodePort` ou `LoadBalancer` pour des
  machines hors du cluster, puis enregistrer cette adresse sur la page.
- **Docker Swarm** : non fourni (voir [Une passerelle](/docs/deploy/one-gateway)).

## Sur la machine d'un développeur

Sur Infra, Plug, le bouton **On a developer's machine** ouvre ces étapes dans un
tiroir, pour macOS, Linux ou Windows, avec des commandes qui portent déjà
l'adresse de cette passerelle. C'est de l'aide, pas de la configuration : rien
n'y est enregistré.

![Le tiroir des étapes pour la machine d'un développeur : installer, nommer le profil, la paire de clés, la déposer, brancher un service](img/console/plug-machine.webp)

Le développeur doit avoir la **capacité développeur** sur son compte
(Application, Users). Ensuite, une fois :

**1. Installer plug.** Il s'installe depuis la passerelle elle-même, pas depuis
un gestionnaire de paquets, et prépare la machine une fois pour que les
lancements suivants ne demandent aucun privilège.

macOS et Linux, depuis un terminal (il peut demander le mot de passe une fois) :

```sh
ssh -p 22222 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null get@<gateway-host> install | sh
```

Windows, depuis **Git Bash** (fourni avec Git for Windows ; il demande une fois
à s'exécuter en Administrateur) :

```bash
ssh -n -p 22222 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null get@<gateway-host> install-windows | bash -s -- <gateway-host> 22222
```

L'installeur crée un profil nommé d'après l'hôte, c'est lui que `-p` désigne
ci-dessous. Un nom à soi se lit mieux dès qu'un second cluster est installé, et
la paire de clés suit le profil :

```sh
plug rn <gateway-host> mon-cluster
```

`<profil>` ci-dessous est ce nom : `mon-cluster` une fois renommé,
`<gateway-host>` sinon.

Les commandes de la page Infra, Plug suivent l'adresse et le nom de profil
saisis sur la page.

**2. Générer la paire de clés.** plug la range sous `~/.plug/keys`, une paire
par cluster, pour qu'une clé puisse être retirée d'un cluster sans toucher aux
autres.

```sh
plug keygen -p <profil>
plug pubkey -p <profil> | pbcopy                        # macOS
plug pubkey -p <profil> | xclip -selection clipboard    # Linux
plug pubkey -p <profil> | clip                          # Windows, Git Bash
```

**3. Déposer la clé publique** sur son profil, connecté aux applications :
`/profile/dev/key`, ou depuis n'importe quelle application, menu utilisateur,
Developer, plug key. La page montre l'empreinte SHA256, à comparer avec ce qu'a
affiché `plug pubkey`. Elle prend une clé publique, jamais un certificat.

Une clé par poste : plug garde une paire par profil, donc un second poste lance
`plug keygen` chez lui et dépose une seconde clé à côté de la première. Chacune
est listée avec son empreinte et le commentaire qu'elle porte, et en retirer une
ne ferme que ce poste. Une clé appartient à un seul compte : la même clé déposée
deux fois, ou par quelqu'un d'autre, est refusée.

**4. Brancher un service**, en préfixant la commande qui le lance :

```sh
plug -p <profil> -s my-service:8080:3000 npm run start
```

Le cluster le joint sous `my-service:8080`, redirigé vers le `3000` local, tant
que la commande tourne.

## Retirer un accès

Retirer la clé du profil, la capacité développeur du compte, ou l'un des deux
interrupteurs ferme la porte à la connexion suivante : rien n'a été émis qui
survivrait au retrait. Infra, Plug liste qui détient la capacité, avec
l'empreinte de sa clé ou le fait qu'il n'en a pas encore.
