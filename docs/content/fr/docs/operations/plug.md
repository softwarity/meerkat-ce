---
title: Plug
section: Exploitation
order: 228
summary: Ouvrir le tunnel développeur, indiquer son adresse aux développeurs, et ce qu'ils exécutent sous macOS, Linux et Windows.
---

# Plug

[plug](https://github.com/softwarity/plug) fait tourner le processus local d'un développeur
comme s'il était membre du cluster. Son service répond sous son vrai nom et joint les autres
par le leur : pas de port-forward, pas de modification du code. Si un service du même nom est
déjà déployé, il est mis de côté le temps de la session, puis rétabli. À quoi cela sert, et
comment tous ceux qui regardent l'application apprennent qui la sert, est expliqué sur la
page [Mode développement](/product/dev-mode). Cette page-ci explique comment ouvrir le
tunnel et comment installer plug ; tout ce que plug fait par ailleurs - les profils,
plusieurs clusters à la fois, les volumes, les environnements - se trouve dans
[la documentation de plug](https://softwarity.github.io/plug/).

> [!NOTE] Édition Enterprise
> Le tunnel est intégré à l'image Enterprise, de même que le client plug qu'il distribue.

## Deux conditions, deux responsables

Le tunnel ne fonctionne que si ces **deux** interrupteurs sont activés :

| interrupteur | où | qui en décide | ce qu'il détermine |
|---|---|---|---|
| Developer mode | Application, General | l'administrateur de l'application | ce que cette installation propose à ses développeurs : le menu Developer, la documentation des API, les connexions simulées |
| Open the developer tunnel | **Infra, Plug** | l'administrateur de l'infrastructure | un port ouvert sur le cluster, et les droits du déploiement sur celui-ci (socket Docker, rôle Kubernetes) |

L'interrupteur du tunnel est **désactivé à la livraison**. Une gateway déclarée en
production (`MEERKAT_PRODUCTION`) garde fermé tout ce qui est destiné aux développeurs, quel
que soit l'état de ces deux interrupteurs.

Tant que le tunnel est fermé, on ne propose pas au développeur de déposer une clé : la page
n'existe pas, et le menu utilisateur ne l'affiche pas. Une clé pour une porte qui n'existe
pas est une clé dont personne ne peut se servir.

La page Infra, Plug dit ce que le tunnel **fait**, et non ce qui a été coché : il écoute sur
son port, il est fermé et par quel interrupteur, ou bien il est activé mais ne peut pas
fonctionner ici, et pourquoi - une gateway qui ne tourne pas dans un conteneur, par
exemple, ne peut pas créer de nom dans le cluster.

## L'adresse qu'utilisent les développeurs

La gateway ne voit pas ce qui la sépare d'un ordinateur portable : un NodePort, un
LoadBalancer, un port Docker publié. La page Infra, Plug enregistre donc l'hôte et le port
**publiés**, et toutes les commandes de cette page, comme celles de la page de profil du
développeur, les contiennent déjà : il ne reste qu'à les copier.

![L'écran Plug : l'interrupteur du tunnel, l'hôte et le port publiés, et qui détient la capacité développeur](img/console/plug.webp)

Dans le conteneur, le tunnel écoute sur le port **22222** (`MEERKAT_PLUG_ADDR`).

- **Docker Compose** : publiez-le, `22222:22222`.
- **Kubernetes** : le chart Helm crée un Service `-plug`, de type ClusterIP par défaut.
  Passez `plug.service.type` à `NodePort` ou à `LoadBalancer` pour les machines situées hors
  du cluster, puis enregistrez cette adresse sur la page.
- **Docker Swarm** : non fourni (voir [Une gateway](/docs/deploy/one-gateway)).

## Sur la machine d'un développeur

Sur la page Infra, Plug, le bouton **On a developer's machine** ouvre ces étapes dans un
tiroir, pour macOS, Linux ou Windows, avec des commandes qui contiennent déjà
l'adresse de cette gateway. C'est une aide, pas de la configuration : rien n'y est
enregistré.

![Le panneau des étapes pour la machine d'un développeur : installer, nommer le profil, générer la paire de clés, la déposer, brancher un service](img/console/plug-machine.webp)

Le compte du développeur doit avoir la **capacité développeur** (Application, Users).
Ensuite, une seule fois :

**1. Installez plug.** Il s'installe depuis la gateway elle-même, et non depuis un
gestionnaire de paquets. Il prépare la machine une fois pour toutes, si bien que les
exécutions suivantes ne demandent plus aucun privilège.

Sous macOS et Linux, dans un terminal (votre mot de passe peut vous être demandé une fois) :

```sh
ssh -p 22222 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null get@<gateway-host> install | sh
```

Sous Windows, dans **Git Bash** (fourni avec Git for Windows ; l'installation demande une
fois à s'exécuter en tant qu'administrateur) :

```bash
ssh -n -p 22222 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null get@<gateway-host> install-windows | bash -s -- <gateway-host> 22222
```

Le programme d'installation crée un profil qui porte le nom de l'hôte : c'est ce profil que
désigne `-p` ci-dessous. Un nom de votre choix est plus lisible dès qu'un deuxième cluster
est installé, et la paire de clés suit le profil :

```sh
plug rn <gateway-host> my-cluster
```

Ci-dessous, `<profile>` désigne ce nom : `my-cluster` si vous l'avez renommé,
`<gateway-host>` sinon.

Les commandes de la page Infra, Plug reprennent l'adresse et le nom de profil que vous y
avez saisis.

**2. Générez la paire de clés.** plug la conserve dans `~/.plug/keys`, à raison d'une paire
par cluster : vous pouvez ainsi retirer une clé d'un cluster sans toucher aux autres.

```sh
plug keygen -p <profile>
plug pubkey -p <profile> | pbcopy                        # macOS
plug pubkey -p <profile> | xclip -selection clipboard    # Linux
plug pubkey -p <profile> | clip                          # Windows, Git Bash
```

**3. Déposez la clé publique** sur votre profil, en étant connecté aux applications :
`/profile/dev/key`, ou, depuis n'importe quelle application, le menu utilisateur, Developer,
plug key. La page affiche l'empreinte SHA256, à comparer avec celle que `plug pubkey` a
affichée. Elle accepte une clé publique, jamais un certificat.

Une clé par poste de travail : plug conserve une paire par profil. Sur une deuxième machine,
il faut donc lancer `plug keygen` sur place et déposer une deuxième clé à côté de la
première. Chaque clé est listée avec son empreinte et le commentaire qu'elle porte, et en
retirer une ne ferme l'accès qu'à la machine correspondante. Une clé appartient à un seul
compte : la même clé déposée deux fois, ou par quelqu'un d'autre, est refusée.

**4. Branchez un service**, en préfixant la commande qui le lance :

```sh
plug -p <profile> -s my-service:8080:3000 npm run start
```

Le cluster le joint à l'adresse `my-service:8080`, redirigée vers votre port local `3000`,
aussi longtemps que la commande tourne.

## Retirer un accès

Retirer la clé du profil, retirer la capacité développeur du compte ou désactiver l'un des
deux interrupteurs ferme la porte dès la connexion suivante : rien n'a été émis qui puisse
survivre à ce retrait. La page Infra, Plug liste ceux qui détiennent la capacité, avec
l'empreinte de leur clé, ou la mention qu'ils n'en ont pas encore.
