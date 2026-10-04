---
title: Cluster Kubernetes
section: Déployer
order: 235
summary: Plusieurs passerelles pour une seule installation : le Deployment, les deux Services, l'Ingress, et comment ne pas faire du point d'entrée le point unique de défaillance.
---

# Cluster Kubernetes

Plusieurs passerelles servant une seule installation, devant les mêmes applications. Toute la forme découle d'une propriété du produit : les nœuds ne se parlent jamais. Toute la coordination passe par la base qu'ils partagent, et c'est ce qui rend le déploiement Kubernetes ordinaire.

Ce que vous déployez tient en quatre objets et une dépendance : un Deployment de trois répliques, deux Services ClusterIP (un par plan), un Ingress devant celui qui est public, et un PostgreSQL externe. Pas de StatefulSet, pas de Service headless, pas de PersistentVolumeClaim, pas de sidecar, pas de Redis.

Le chart Helm de deploy/helm installe la forme mono-passerelle : une réplique, un volume, stratégie Recreate. Un cluster change trois choses - plus de volume, une URL de base, plusieurs répliques - donc il est écrit en entier ci-dessous plutôt que caché derrière des valeurs.

## Pourquoi un Deployment, et pas un StatefulSet

Un StatefulSet existe pour donner aux pods un nom stable, un ordre stable et un volume à eux, afin qu'ils se trouvent et gardent leur état. Meerkat n'a besoin d'aucun des trois. Un nœud n'adresse jamais un autre nœud : quand l'un écrit une route, il incrémente une version en base et envoie un NOTIFY, et les autres l'entendent sur la connexion qu'ils tiennent déjà. C'est LISTEN/NOTIFY de PostgreSQL, et c'est la raison pour laquelle PostgreSQL est la seule option externe : il a été choisi pour le transport, pas pour le stockage.

Donc pas de pair-à-pair, pas de gossip, pas de Raft, pas de quorum, pas de liste d'amorçage, et aucun nœud qui doive apprendre l'adresse d'un autre. Rien ne découvre les pods. Le dimensionnement est un nombre, et un pod est interchangeable avec n'importe quel autre.

La seule chose qu'un volume par pod porterait, c'est le stockage embarqué - et une base par pod, c'est exactement ce qu'un cluster n'est pas.

## Aucune affinité de session à demander

Les sessions vivent en base, pas dans un pod. Chaque nœud garde un cache de cinq secondes pour répondre vite, et ce cache est invalidé par le même bus : une révocation, ou une organisation qui vient d'être choisie sur le nœud A, n'est pas servie périmée par le nœud B. Une requête peut donc atterrir sur n'importe quelle réplique, et c'est ce qui rend une mise à jour progressive sans histoire : un pod qui part n'emporte aucune session.

Le compteur anti-force-brute est en base également, une ligne par tentative échouée, et cela compte plus qu'il n'y paraît. Une limite de cinq tentatives valait cinq PAR PASSERELLE : un attaquant qui arrose le répartiteur voyait la limite multipliée par ce qui était censé rendre le service plus solide. Cinq tentatives valent maintenant cinq pour l'installation, où qu'elles tombent, et une connexion réussie pardonne sur tous les nœuds. Un test le dit exactement, et c'est la raison pour laquelle rien ici ne réclame de session collante au répartiteur.

Ce qui est relayé plutôt que partagé : un message live publié sur un nœud atteint les pages tenues ouvertes par les autres, donc personne ne voit la moitié d'une annonce. Ce qui reste par nœud, et qu'il vaut mieux savoir avant de poser un chiffre : les limites de débit comptent en mémoire, par nœud, donc N nœuds autorisent jusqu'à N fois la borne - la console le dit là où le nombre se tape. Deux outils de développement sont par processus pour la même raison : le mode test UI et les jetons du Swagger embarqué vivent dans le nœud qui les a émis, donc une identité simulée peut sauter d'une requête à l'autre entre plusieurs nœuds. C'est dit plutôt que corrigé : c'est un outil pour un exploitant devant un écran, et le rendre commun au cluster serait de la machinerie pour personne.

## Le prérequis : un PostgreSQL partagé

Le stockage embarqué est un fichier possédé par un seul processus. Faites-y pointer trois pods et vous avez trois passerelles indépendantes : trois jeux de routes, trois jeux de comptes, trois mots de passe admin, et une console qui montre celle que votre requête a touchée. Rien ne paraîtrait cassé du côté du binaire, et c'est ce qui en fait un mauvais après-midi.

Un cluster commence donc par MEERKAT_DATABASE_URL pointant vers un PostgreSQL que tous les nœuds atteignent. C'est la seule option externe, et il n'y a rien d'autre à faire tourner : les notifications sont le bus, et un verrou consultatif est la seule exclusion dont le produit a besoin. Pas de Redis, pas de broker, pas d'etcd.

> [!NOTE]
> C'est une capacité Enterprise, et c'est le pilote qui EST l'édition : le binaire communautaire ne lie aucun pilote PostgreSQL, donc une URL de base y reçoit une phrase qui nomme ce qui est disponible, pas une erreur de pilote. Du code absent refuse tout seul, et aucune licence n'est lue.

## Le répertoire de données, et la seule chose qui ne soit pas en base

Le répertoire -data est toujours créé avec une base externe, mais il n'est plus la source de vérité : routes, comptes, sessions, journal d'audit et certificats sont des lignes. Un emptyDir suffit, d'où l'absence de PersistentVolumeClaim ici, et donc aucun volume ReadWriteOnce à se passer d'une machine à l'autre.

> [!WARNING]
> Un fichier fait exception, et c'est celui qui mord : la clé maître du coffre. Elle scelle ce qui est DANS la base, la ranger là serait fermer la porte en laissant la clé dans la serrure. Livré à lui-même, chaque pod génère sa propre clé dans son propre emptyDir - et alors un secret scellé par un nœud est illisible par les autres, ce qui se voit comme un mot de passe d'amont qui marche sur une réplique sur trois.

En cluster, la clé est donc fournie par l'environnement, identique sur tous les pods, depuis le même Secret. Générez-la une fois et rangez-la là où vous rangez vos clés : la perdre, c'est perdre tous les secrets du coffre.

## Le Secret

Trois valeurs, créées plutôt que versionnées. Un manifeste avec un champ base64 est un secret dans git, et base64 n'est pas un chiffrement.

```bash
# Trois valeurs, et aucune n'a sa place dans git.
kubectl create secret generic meerkat \
  --from-literal=database-url='postgres://meerkat:MOTDEPASSE@postgres.data.svc:5432/meerkat?sslmode=require' \
  --from-literal=admin-password='le-premier-mot-de-passe-admin' \
  --from-literal=vault-key="$(openssl rand -hex 32)"
```

MEERKAT_ADMIN_PASSWORD est lu au PREMIER démarrage seulement, pour créer le premier compte admin : le pod qui arrive le premier le crée, les autres le trouvent déjà là. Une fois connecté et votre propre compte créé, il peut sortir du Secret.

## Le Deployment

Les deux sondes existent sur les deux ports, et elles ne sont pas interchangeables. /healthz est la VIVACITÉ et répond UP sans condition : cette sonde décide de tuer le processus, et une sonde de vivacité qui échouerait parce que la base ne répond pas transformerait un hoquet de base en redémarrage simultané de tous les nœuds, chacun tué pour une panne qu'aucun n'a et qu'aucun ne répare en mourant. /readyz est la DISPONIBILITÉ : il pingue le stockage et vérifie que la table de routage est compilée, et répond 503 en nommant lequel des deux manque - un nœud qui a accepté des connexions mais n'a pas encore compilé sa table répond 404 à tout, et une mise à jour progressive y lirait une instance en bonne santé.

La règle qui en découle : ce qui envoie du trafic sonde /readyz, jamais /healthz.

::: details Le Deployment complet
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: meerkat
  labels:
    app.kubernetes.io/name: meerkat
spec:
  replicas: 3
  strategy:
    type: RollingUpdate
    rollingUpdate:
      # Rien de local à transmettre : le nouveau pod peut servir avant que
      # l'ancien parte. Aucune session perdue, aucune configuration manquée.
      maxUnavailable: 0
      maxSurge: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: meerkat
  template:
    metadata:
      labels:
        app.kubernetes.io/name: meerkat
    spec:
      # Trois pods sur une seule machine, c'est une machine à perdre pour ne
      # plus avoir de gateway du tout.
      topologySpreadConstraints:
        - maxSkew: 1
          topologyKey: kubernetes.io/hostname
          whenUnsatisfiable: ScheduleAnyway
          labelSelector:
            matchLabels:
              app.kubernetes.io/name: meerkat
      containers:
        - name: meerkat
          # La base externe est une capacité Enterprise : l'image
          # communautaire ne lie aucun pilote PostgreSQL. Figez une version
          # plutôt que de suivre "latest" sur ce qui tient votre porte
          # d'entrée.
          image: ghcr.io/softwarity/meerkat:latest
          ports:
            - name: app
              containerPort: 8080
            - name: admin
              containerPort: 9090
          env:
            # La base partagée. Cette ligne est ce qui fait de trois pods UNE
            # gateway au lieu de trois.
            - name: MEERKAT_DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: meerkat
                  key: database-url
            # La clé maître du coffre, LA MÊME sur tous les pods. Elle scelle
            # ce qui est dans la base, elle n'y est donc pas rangée. Sans
            # elle, chaque pod génère la sienne et ne lit aucun secret des
            # autres.
            - name: MEERKAT_VAULT_KEY
              valueFrom:
                secretKeyRef:
                  name: meerkat
                  key: vault-key
            # Lu UNE FOIS, au premier démarrage, pour créer le compte admin.
            - name: MEERKAT_ADMIN_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: meerkat
                  key: admin-password
            # Cette gateway est une gateway de production : la surface de
            # développement y reste fermée, quoi que dise la base.
            - name: MEERKAT_PRODUCTION
              value: "1"
          # DÉMARRAGE : le stockage est ouvert et migré avant que les ports
          # s'ouvrent. Jusqu'à deux minutes pour cela, avant que la vivacité
          # compte.
          startupProbe:
            httpGet:
              path: /healthz
              port: app
            periodSeconds: 2
            failureThreshold: 60
          # VIVACITÉ : ce processus sert-il encore ? Il répond UP sans rien
          # regarder, et c'est la bonne réponse - cette sonde décide de TUER
          # le processus.
          livenessProbe:
            httpGet:
              path: /healthz
              port: app
            periodSeconds: 10
          # DISPONIBILITÉ : ce nœud peut-il prendre une requête maintenant ?
          # Il vérifie le stockage et la table de routage compilée, et répond
          # 503 en nommant lequel des deux manque. C'est celle-là que le
          # répartiteur doit sonder.
          readinessProbe:
            httpGet:
              path: /readyz
              port: app
            periodSeconds: 5
            timeoutSeconds: 3
          volumeMounts:
            - name: data
              mountPath: /data
          resources:
            requests:
              cpu: 200m
              memory: 128Mi
            limits:
              memory: 512Mi
          securityContext:
            runAsNonRoot: true
            seccompProfile:
              type: RuntimeDefault
            allowPrivilegeEscalation: false
            capabilities:
              drop: ["ALL"]
      volumes:
        # Avec une base partagée, ce répertoire ne contient rien qui doive
        # survivre au pod : pas de PersistentVolumeClaim, donc aucun volume
        # ReadWriteOnce à se passer d'une machine à l'autre.
        - name: data
          emptyDir: {}
```
:::

## Deux Services, parce qu'il y a deux plans

Le port 8080 est le plan de données : vos applications, les pages de connexion, le canal live. Le port 9090 est le plan de contrôle : la console d'administration, l'API admin et l'endpoint MCP auquel parle un agent. Seul le premier a sa place sur internet, et ce sont deux Services pour exactement cette raison : un Ingress placé devant vos applications ne doit pas pouvoir atteindre la console par accident, et un Service unique à deux ports en est à une annotation près.

::: details Les deux Services
```yaml
# Le plan de données : ce que vos utilisateurs atteignent.
apiVersion: v1
kind: Service
metadata:
  name: meerkat
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/name: meerkat
  ports:
    - name: app
      port: 8080
      targetPort: app
---
# Le plan de contrôle : console, API d'administration et endpoint MCP. Un
# SECOND Service, volontairement : un Ingress placé devant les applications
# n'a alors aucun moyen d'atteindre la console par accident.
apiVersion: v1
kind: Service
metadata:
  name: meerkat-admin
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/name: meerkat
  ports:
    - name: admin
      port: 9090
      targetPort: admin
```
:::

Comment un opérateur atteint alors la console, par ordre de préférence : kubectl port-forward service/meerkat-admin 9090:9090 pour un travail occasionnel ; un second Ingress sur un controller interne, ou sur le même restreint par adresse source et certificat client, quand une équipe en a besoin tous les jours. Ce qu'il ne faut pas faire, c'est la poser sur le même hôte public que les applications.

## L'Ingress

Deux choses qu'un Ingress par défaut rate avec cette gateway. Elle proxifie des WebSockets et tient un canal live ouvert sur /meerkat/events - ce sont des réponses longues, donc un controller qui tamponne le corps ou coupe la connexion à ses soixante secondes par défaut casse des pages qui marchaient, de façon intermittente et pour certains utilisateurs seulement.

Et le schéma. Meerkat sait terminer TLS elle-même (-tls-addr, -admin-tls-addr, avec une émission ACME sérialisée par un verrou consultatif pour que N nœuds demandent un certificat et non N, la matière et le défi étant des lignes auxquelles n'importe quel nœud peut répondre). En Kubernetes on termine plutôt à l'Ingress, et les deux marchent. Mais ce qui termine devant doit envoyer X-Forwarded-Proto : la gateway lit le schéma là autant que sur la connexion, et sans lui trois choses se cassent d'un coup - les cookies de session perdent leur attribut Secure, la console annonce des URL en http://, et la redirection vers HTTPS boucle indéfiniment. nginx et Traefik l'envoient par défaut ; tout montage maison placé devant doit être vérifié.

::: details L'Ingress complet
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: meerkat
  annotations:
    # La gateway proxifie des WebSockets et tient un canal live ouvert
    # (/meerkat/events). Ce sont des réponses longues : un Ingress qui les
    # tamponne, ou qui coupe à ses 60 s par défaut, casse des pages qui
    # marchaient.
    nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "3600"
    nginx.ingress.kubernetes.io/proxy-buffering: "off"
spec:
  ingressClassName: nginx
  tls:
    - hosts: ["apps.example.com"]
      secretName: meerkat-tls
  rules:
    - host: apps.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: meerkat
                port:
                  name: app
# Rien ici ne pointe vers meerkat-admin, et c'est tout l'intérêt.
```
:::

## Ce qui est devant

Un Service ClusterIP n'est joignable que depuis l'intérieur du cluster : quelque
chose devant doit porter l'adresse que vos utilisateurs tapent. Cette adresse
devient alors ce qui peut faire tomber l'installation - trois répliques
derrière un nom qui résout vers une seule machine, c'est un cluster d'une
machine avec des étapes en plus.

Sur un cluster managé (EKS, GKE, AKS) : `type: LoadBalancer` sur le Service du
plan de données, et le fournisseur rend une adresse stable et répartie.

Sur du matériel nu, ce fournisseur n'existe pas et `type: LoadBalancer` reste
en attente. MetalLB (en L2 ou en BGP) ou une VIP keepalived devant un Ingress
controller répondent à ce besoin. C'est une décision d'infrastructure, pas un
réglage de Meerkat : la passerelle est joignable de la même façon quelle que
soit la réponse retenue.

> [!NOTE] Traefik ne remplace que la moitié
> Traefik - comme nginx, HAProxy ou Envoy - **est** un Ingress controller : il
> remplace l'objet Ingress ci-dessus, et vous écririez un IngressRoute à la
> place. Mais ses pods sont des pods : sur matériel nu, il a besoin exactement
> de la même adresse d'entrée hautement disponible. Deux répliques de Traefik
> ne donnent pas deux adresses d'entrée.

## Le point fragile, c'est la base

Les répliques font survivre la gateway à la perte d'une machine. Elles ne font rien pour ce que toutes lisent : il y a une base, et aucun nombre de pods ne la rend double. Dès lors que l'adresse d'entrée peut bouger, la base est le point unique de défaillance de l'installation, et c'est là que va le travail restant.

Ce qui se passe quand elle tombe est au moins honnête. /readyz répond 503 avec la raison - le stockage ne répond pas - donc Kubernetes retire ces pods des endpoints du Service, tandis que /healthz continue de dire UP et que rien n'est redémarré. La gateway cesse de servir plutôt que de servir des réponses fausses, et elle revient d'elle-même quand la base revient : pas de redémarrage, pas de geste manuel, rien à débloquer.

Pointez donc `MEERKAT_DATABASE_URL` vers un nom qui suit le primaire plutôt que
vers un hôte, et mesurez la fenêtre de basculement de votre base : c'est la
fenêtre pendant laquelle aucun nœud n'est prêt.

## Optionnel : laisser l'éditeur de routes voir le namespace

La gateway sait lister les Services de SON namespace et les proposer au moment où l'on crée une route : un amont devient un choix plutôt qu'une URL tapée. Il n'y a pas d'interrupteur : ce qui l'ouvre est ce que le déploiement accorde. Le ServiceAccount a besoin de list sur services dans son propre namespace et de rien d'autre - aucun secret, aucun pod, aucun autre namespace - et sans ce droit la console dit lequel lui manque, la saisie libre restant telle quelle. Le tunnel de développement, lui, en demande davantage, et ce qu'il accorde est détaillé sur [Une passerelle](/docs/deploy/one-gateway).

::: details Le Role et son binding
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: meerkat
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: meerkat-services
rules:
  # Lister les Services de CE namespace, et rien d'autre : aucun secret,
  # aucun pod, aucun autre namespace.
  - apiGroups: [""]
    resources: ["services"]
    verbs: ["list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: meerkat-services
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: meerkat-services
subjects:
  - kind: ServiceAccount
    name: meerkat
# Puis nommez-le dans le pod :
#   serviceAccountName: meerkat
```
:::

## OpenShift et OKD

La même image et le même chart s'installent sur OpenShift et OKD sous la SCC par
défaut `restricted-v2` : aucune SCC supplémentaire à accorder, aucune valeur à changer,
rien qui tourne en root. Trois choix le rendent vrai, et ils sont à connaître avant de
surcharger quoi que ce soit :

- **Aucun uid dans le pod.** OpenShift ne lance pas l'utilisateur de l'image : il tire
  un uid de la plage du namespace, ajoute le groupe 0, et refuse un pod qui fixe le sien -
  `runAsUser: Invalid value: 65532: must be in the ranges: [...]`. Ni le chart ni le
  manifeste ci-dessus ne posent donc `runAsUser`, `runAsGroup` ou `fsGroup`. **Ne les
  remettez pas** dans vos valeurs : ils n'apportent rien ailleurs, l'image porte déjà
  son uid.
- **`/data` appartient au groupe root, et le groupe écrit.** Quel que soit l'uid choisi
  par OpenShift, il est dans le groupe 0 : le store écrit son premier fichier même là
  où `/data` est un simple répertoire et non un volume monté.
- **Le `USER` de l'image est numérique (65532).** Sur un namespace Kubernetes ordinaire
  qui impose Pod Security `restricted`, le kubelet refuse une image dont l'utilisateur
  est un nom, faute de pouvoir prouver qu'il n'est pas root. Un nombre passe, là comme
  sur OpenShift.

Le reste convient déjà : tous les ports écoutés sont au-dessus de 1024, la passerelle
n'a besoin d'aucune capacité, d'aucune élévation de privilèges ni d'aucun chemin de
l'hôte, et l'accès en lecture que demande l'éditeur de routes (plus haut) est un Role
ordinaire.

**L'exposer.** Une **Route** remplace l'Ingress, une par plan, avec la même règle :
rien ne pointe vers le plan de contrôle sans que vous le vouliez.

```yaml
apiVersion: route.openshift.io/v1
kind: Route
metadata:
  name: meerkat
spec:
  host: apps.example.com
  to:
    kind: Service
    name: meerkat
  port:
    targetPort: app
  tls:
    # edge : le routeur porte le certificat et Meerkat voit du HTTP avec
    # X-Forwarded-Proto: https. passthrough si c'est la passerelle qui porte le
    # certificat - ce qu'exige ACME, dont le défi TLS-ALPN doit atteindre
    # Meerkat intact.
    termination: edge
    insecureEdgeTerminationPolicy: Redirect
```

> [!NOTE] Vérifié chaque nuit
> Ce qui précède n'est pas une promesse : chaque nuit, la CI installe ce chart sur
> MicroShift (OpenShift construit depuis OKD), sous la SCC `restricted-v2`, et
> vérifie l'uid attribué, l'écriture de `/data` et les deux Routes. Voir la
> [plateforme de test](/project/test-platform).

## Avant de dire que c'est fini

- Ce qui envoie du trafic sonde /readyz, pas /healthz.
- MEERKAT_VAULT_KEY est posée, et c'est la même valeur sur tous les pods.
- Le port 9090 n'est publié nulle part : aucune règle d'Ingress, aucun LoadBalancer, aucun NodePort.
- X-Forwarded-Proto arrive, et les réponses longues ne sont ni tamponnées ni coupées.
- Plus d'une machine peut répondre à l'adresse publique, et vous l'avez vu se produire.
- La base a un chemin de basculement, et la restauration a été essayée.
- Tuez un pod pendant un test de charge et regardez les chiffres : c'est la répétition la moins chère que vous aurez.

Une passerelle sur un volume est une forme plus simple, et elle a sa page : [Docker et Helm](/docs/deploy/one-gateway).
