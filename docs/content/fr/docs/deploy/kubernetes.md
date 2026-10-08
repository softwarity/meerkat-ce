---
title: Cluster Kubernetes
section: Déployer
order: 235
summary: Plusieurs gateways pour une même installation : le Deployment, les deux Services, l'Ingress, et comment éviter que le point d'entrée ne devienne un point de défaillance unique.
---

# Cluster Kubernetes

Plusieurs gateways au service d'une même installation, devant les mêmes applications.
Toute l'architecture découle d'une propriété du produit : les nœuds ne se parlent jamais.
Toute la coordination passe par la base de données qu'ils partagent, et c'est ce qui permet
à un déploiement Kubernetes de rester un déploiement ordinaire.

Vous déployez quatre objets et une dépendance : un Deployment de trois répliques, deux
Services ClusterIP (un par plan), un Ingress devant celui qui est public, et un PostgreSQL
externe. Pas de StatefulSet, pas de Service headless, pas de PersistentVolumeClaim, pas de
sidecar, pas de Redis.

Par défaut, le chart Helm de deploy/helm installe l'architecture à une seule gateway : une
réplique, un volume, la stratégie Recreate. Un cluster change trois choses - pas de volume,
une URL de base de données, plusieurs répliques - que règle `values-ee-cluster.yaml` ; elles
sont aussi écrites en entier ci-dessous plutôt que cachées derrière des valeurs.

## Pourquoi un Deployment, et pas un StatefulSet

Un StatefulSet sert à donner aux pods un nom stable, un ordre stable et un volume propre à
chacun, pour qu'ils puissent se trouver les uns les autres et conserver leur état. Meerkat
n'a besoin d'aucun des trois. Un nœud ne s'adresse jamais à un autre nœud : quand l'un écrit
une route, il incrémente une version dans la base et émet un NOTIFY, que les autres
reçoivent sur la connexion qu'ils tiennent déjà ouverte. C'est le LISTEN/NOTIFY de
PostgreSQL, et c'est la raison pour laquelle PostgreSQL est la seule option externe - il a
été choisi pour le transport, pas pour le stockage.

Il n'y a donc ni pair-à-pair, ni gossip, ni Raft, ni quorum, ni liste de nœuds d'amorçage, ni
nœud qui doive apprendre l'adresse d'un autre. Rien ne découvre les pods. La mise à l'échelle
tient en un nombre, et un pod est interchangeable avec n'importe quel autre.

La seule chose que contiendrait un volume par pod, c'est le stockage embarqué - et une base
par pod est précisément ce qu'un cluster n'est pas.

## Aucune affinité de session à demander

Les sessions se trouvent dans la base, pas dans un pod. Chaque nœud garde un cache de cinq
secondes pour répondre vite, et ce cache est invalidé par le même bus : une révocation, ou
une organisation qui vient d'être choisie sur le nœud A, n'est donc pas servie périmée par le
nœud B. Une requête peut ainsi arriver sur n'importe quelle réplique, et c'est ce qui rend
une mise à jour progressive sans histoire : un pod qui s'arrête n'emporte aucune session.

Le compteur d'attaques par force brute est lui aussi en base, à raison d'une ligne par
tentative échouée, et cela compte plus qu'il n'y paraît. Une limite de cinq tentatives était
auparavant de cinq PAR PASSERELLE : un attaquant qui arrosait le load balancer voyait
la limite multipliée par cela même qui était censé rendre le service plus robuste. Cinq
tentatives sont désormais cinq pour toute l'installation, où qu'elles arrivent, et une
connexion réussie remet le compteur à zéro sur tous les nœuds. Un test le vérifie
précisément, et c'est pourquoi rien ici ne demande de sessions persistantes à votre
load balancer.

Ce qui est relayé plutôt que partagé : un message en direct publié sur un nœud parvient aux
pages que les autres nœuds tiennent ouvertes, si bien que personne ne voit une annonce à
moitié. Ce qui reste propre à chaque nœud, et qu'il vaut mieux savoir avant de fixer un
nombre : les rate limits sont comptés en mémoire, par nœud, donc N nœuds laissent
passer jusqu'à N fois la borne - la console le signale à l'endroit où le nombre se saisit.
Deux outils de développement sont propres à chaque processus pour la même raison : le mode
de test des interfaces et les jetons qu'utilise le Swagger embarqué n'existent que dans le
nœud qui les a émis, de sorte qu'une identité simulée peut changer d'une requête à la
suivante quand plusieurs nœuds répondent. C'est dit, et non corrigé : il s'agit d'un outil
pour un seul exploitant devant un seul écran, et l'étendre au cluster reviendrait à
construire une machinerie pour personne.

## Le prérequis : un PostgreSQL qu'elles partagent

Le stockage embarqué est un fichier qui appartient à un seul processus. Pointez trois pods
dessus et vous obtenez trois gateways indépendantes : trois jeux de routes, trois jeux de
comptes, trois mots de passe d'administration, et une console qui montre celle des trois
que votre requête a atteinte par hasard. Du point de vue du binaire, rien ne semblerait
cassé, et c'est ce qui promet un mauvais après-midi.

Un cluster commence donc par une variable MEERKAT_DATABASE_URL qui pointe vers un
PostgreSQL que tous les nœuds peuvent joindre. C'est la seule option externe, et il n'y a
rien d'autre à faire tourner : les notifications sont le bus, et un verrou consultatif
fournit l'exclusion dont le produit a besoin. Pas de Redis, pas de courtier de messages, pas
d'etcd.

> [!NOTE]
> Il s'agit d'une capacité de l'édition Enterprise, et c'est le pilote qui fait l'édition :
> le binaire Community n'embarque aucun pilote PostgreSQL. Une URL de base de données y
> reçoit donc une phrase qui nomme ce qui est disponible, et non une erreur de pilote. Le
> code absent refuse de lui-même, et aucune licence n'est lue.

## Le répertoire de données, et la seule chose qui n'est pas en base

Le répertoire -data est toujours créé avec une base externe, mais il n'est plus la source de
vérité : les routes, les comptes, les sessions, le journal d'audit et les certificats sont
tous des lignes en base. Un emptyDir suffit, ce qui explique l'absence de
PersistentVolumeClaim ici, et donc de volume ReadWriteOnce à faire passer d'une machine à
une autre.

> [!WARNING]
> Un fichier fait exception, et c'est celui qui fait mal : la clé maîtresse du coffre. Elle
> scelle ce qui est DANS la base ; l'y ranger reviendrait à fermer la porte en laissant la
> clé sur la serrure. Livré à lui-même, chaque pod génère sa propre clé dans son propre
> emptyDir - et un secret scellé par un nœud devient alors illisible pour les autres, ce qui
> se manifeste par un mot de passe d'upstream qui fonctionne sur une réplique sur trois.

Dans un cluster, la clé est donc fournie par l'environnement, identique sur tous les pods et
issue du même Secret. Générez-la une seule fois et conservez-la là où vous conservez vos
clés - la perdre, c'est perdre tous les secrets du coffre.

## Le Secret

Trois valeurs, créées par une commande plutôt que versionnées. Un manifeste contenant un
champ en base64 est un secret dans git, et le base64 n'est pas du chiffrement.

```bash
# Trois valeurs, et aucune n'a sa place dans git.
kubectl create secret generic meerkat \
  --from-literal=database-url='postgres://meerkat:PASSWORD@postgres.data.svc:5432/meerkat?sslmode=require' \
  --from-literal=admin-password='the-first-admin-password' \
  --from-literal=vault-key="$(openssl rand -hex 32)"
```

MEERKAT_ADMIN_PASSWORD n'est lue qu'au PREMIER démarrage, pour créer le premier compte
admin : le premier pod qui y parvient le crée, les autres le trouvent déjà en place. Une
fois que vous vous êtes connecté et que vous avez créé votre propre compte, elle peut être
retirée du Secret.

## Le Deployment

Les deux health checks existent sur les deux ports, et ils ne sont pas interchangeables. /healthz
est la LIVENESS probe et répond UP sans condition : elle décide s'il faut tuer
le processus, et une liveness probe qui échouerait parce que la base est injoignable
transformerait un incident passager de la base en redémarrage simultané de tous les nœuds,
chacun tué pour une panne qu'aucun n'a et qu'aucun ne peut réparer en mourant. /readyz est
la READINESS probe : elle interroge le stockage et vérifie que la table de
routage a été compilée, et répond 503 en nommant celui des deux qui manque - un nœud qui
accepte des connexions sans avoir encore compilé sa table répond 404 à tout, et une mise à
jour progressive y verrait une instance saine.

La règle qui en découle : tout ce qui envoie du trafic sonde /readyz, et jamais /healthz.

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
      # Rien de local à transmettre : le nouveau pod peut donc servir avant
      # que l'ancien ne parte. Aucune session n'est perdue et aucune
      # configuration n'est manquée.
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
      # Trois pods sur une seule machine : il suffit d'une machine perdue
      # pour ne plus avoir de gateway du tout.
      topologySpreadConstraints:
        - maxSkew: 1
          topologyKey: kubernetes.io/hostname
          whenUnsatisfiable: ScheduleAnyway
          labelSelector:
            matchLabels:
              app.kubernetes.io/name: meerkat
      containers:
        - name: meerkat
          # La base externe est une capacité Enterprise : l'image Community
          # n'embarque aucun pilote PostgreSQL. Épinglez une version plutôt
          # que de suivre "latest" sur ce qui tient votre porte d'entrée. Pour
          # essayer le cluster d'abord : docker.io/softwarity/meerkat:eval,
          # publique, l'image Enterprise avec une mention d'évaluation - pas
          # pour la production.
          image: ghcr.io/softwarity/meerkat:latest
          ports:
            - name: app
              containerPort: 8080
            - name: admin
              containerPort: 9090
          env:
            # La base partagée. C'est cette seule ligne qui fait de trois
            # pods une gateway au lieu de trois gateways.
            - name: MEERKAT_DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: meerkat
                  key: database-url
            # La clé maîtresse du coffre, LA MÊME sur tous les pods. Elle
            # scelle ce qui est dans la base, elle n'y est donc pas rangée.
            # Sans elle, chaque pod génère la sienne et ne lit aucun des
            # secrets des autres.
            - name: MEERKAT_VAULT_KEY
              valueFrom:
                secretKeyRef:
                  name: meerkat
                  key: vault-key
            # Lu UNE SEULE FOIS, au premier démarrage, pour créer le compte
            # admin.
            - name: MEERKAT_ADMIN_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: meerkat
                  key: admin-password
            # Cette gateway est une gateway de production : la surface
            # développeur y reste fermée, quoi que dise la base.
            - name: MEERKAT_PRODUCTION
              value: "1"
          # STARTUP : le stockage est ouvert et migré avant l'ouverture des
          # ports. Jusqu'à deux minutes pour cela, avant que la vivacité ne
          # compte.
          startupProbe:
            httpGet:
              path: /healthz
              port: app
            periodSeconds: 2
            failureThreshold: 60
          # LIVENESS : ce processus sert-il encore ? Elle répond UP sans rien
          # regarder, et c'est la bonne réponse - cette liveness probe décide s'il
          # faut TUER le processus.
          livenessProbe:
            httpGet:
              path: /healthz
              port: app
            periodSeconds: 10
          # READINESS : ce nœud peut-il prendre une requête maintenant ? Elle
          # vérifie le stockage et la table de routage compilée, et répond
          # 503 en nommant celui des deux qui a échoué. C'est celle-ci qu'un
          # load balancer doit sonder.
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
        # survivre au pod : pas de PersistentVolumeClaim, et donc pas de
        # volume ReadWriteOnce à faire passer d'une machine à une autre.
        - name: data
          emptyDir: {}
```
:::

## Deux Services, parce qu'il y a deux plans

Le port 8080 est le plan de données - vos applications, les pages de connexion, le canal
temps réel. Le port 9090 est le plan de contrôle - la console d'administration, l'API
d'administration et l'endpoint MCP auquel s'adresse un agent. Seul le premier a sa place sur
Internet, et c'est exactement pour cette raison qu'il y a deux Services : un Ingress placé
devant vos applications ne doit pas pouvoir atteindre la console par accident, et un Service
unique à deux ports n'en est qu'à une annotation.

::: details Les deux Services
```yaml
# Le plan de données : ce qu'atteignent vos utilisateurs.
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
# Le plan de contrôle : la console, l'API d'administration et l'endpoint MCP.
# Un SECOND Service, à dessein : un Ingress placé devant les applications n'a
# alors aucun moyen d'atteindre la console par accident.
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

Comment un exploitant atteint alors la console, par ordre de préférence : kubectl
port-forward service/meerkat-admin 9090:9090 pour un usage occasionnel ; un second Ingress
sur un contrôleur réservé au réseau interne, ou sur le même contrôleur restreint par adresse
source et par certificat client, quand une équipe en a besoin tous les jours. Ce qu'il ne
faut pas faire, c'est la placer sur le même hôte public que les applications.

## L'Ingress

Un Ingress par défaut se trompe sur deux points avec cette gateway. La gateway relaie
des WebSockets, et elle tient un canal temps réel ouvert sur /meerkat/events - ce sont deux
réponses de longue durée, si bien qu'un contrôleur qui met le corps en tampon, ou qui ferme
la connexion au bout de ses soixante secondes par défaut, casse des pages qui
fonctionnaient, par intermittence et pour certains utilisateurs seulement.

Second point, le schéma. Meerkat peut terminer TLS lui-même (-tls-addr, -admin-tls-addr,
l'émission ACME étant sérialisée par un verrou consultatif pour que N nœuds demandent un
seul certificat et non N, le certificat et le défi étant des lignes en base à partir
desquelles chaque nœud peut répondre). Dans Kubernetes, on termine plutôt TLS au niveau de
l'Ingress, et les deux fonctionnent. Mais ce qui termine TLS en amont doit envoyer
X-Forwarded-Proto : la gateway y lit le schéma autant que sur la connexion, et sans cet
en-tête trois choses se dérèglent en même temps - les cookies de session perdent leur
attribut Secure, la console annonce des URL en http://, et la redirection vers HTTPS boucle
indéfiniment. nginx et Traefik le posent tous deux par défaut ; tout ce qui est fait maison
en amont doit être vérifié.

::: details L'Ingress complet
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: meerkat
  annotations:
    # La gateway relaie des WebSockets et tient un canal temps réel ouvert
    # (/meerkat/events). Ce sont deux réponses de longue durée : un Ingress
    # qui les met en tampon, ou qui ferme au bout de ses 60 s par défaut,
    # casse des pages qui fonctionnaient.
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

## Ce qui se trouve devant

Un Service ClusterIP n'est joignable que de l'intérieur du cluster : il faut
quelque chose devant lui pour porter l'adresse que saisissent vos utilisateurs.
C'est alors cette adresse qui peut faire tomber l'installation - trois
répliques derrière un nom qui se résout vers une seule machine, c'est un
cluster d'une machine, en plus compliqué.

Sur un cluster managé (EKS, GKE, AKS) : `type: LoadBalancer` sur le Service du
plan de données, et le fournisseur vous rend une adresse stable et répartie.

Sur du bare metal, ce fournisseur n'existe pas et `type: LoadBalancer` reste en
attente. MetalLB (en mode L2 ou BGP), ou une adresse IP virtuelle keepalived
placée devant un contrôleur Ingress, répondent à ce besoin. C'est une décision
d'infrastructure, pas un réglage de Meerkat : la gateway se joint de la même
manière quelle que soit la solution retenue.

> [!NOTE] Traefik n'en remplace que la moitié
> Traefik - comme nginx, HAProxy ou Envoy - **est** un contrôleur Ingress : il
> remplace l'objet Ingress ci-dessus, et vous écririez à la place un
> IngressRoute. Mais ses pods restent des pods : sur du bare metal, il a besoin
> exactement de la même adresse d'entrée hautement disponible. Deux répliques
> de Traefik ne vous donnent pas deux adresses d'entrée.

## Le point fragile, c'est la base

Les répliques permettent à la gateway de survivre à la perte d'une machine. Elles ne
font rien pour ce qu'elles lisent toutes : il n'y a qu'une base, et aucun nombre de pods
n'en fait deux. Une fois que l'adresse d'entrée peut se déplacer, la base est le point de
défaillance unique de l'installation, et c'est là que doit porter le travail qui reste.

Ce qui se passe quand elle tombe a au moins le mérite de l'honnêteté. /readyz répond 503 en
donnant la raison - le stockage ne répond pas - et Kubernetes retire donc ces pods des
endpoints du Service, tandis que /healthz continue de dire UP et que rien n'est redémarré.
La gateway cesse de servir plutôt que de servir des réponses fausses, et elle revient
d'elle-même quand la base revient : pas de redémarrage, pas d'étape manuelle, rien à
débloquer.

Faites donc pointer `MEERKAT_DATABASE_URL` vers un nom qui suit le primaire
plutôt que vers un hôte, et mesurez la fenêtre de bascule de votre base : c'est
pendant cette fenêtre qu'aucun nœud n'est prêt.

## Optionnel : laisser la gateway voir le namespace

La gateway lit SON PROPRE namespace pour deux choses. L'éditeur de routes propose ses
Services : un upstream se choisit alors dans une liste au lieu d'être une URL à saisir. Et
la liste des routes montre, pour chaque service, combien de ses répliques sont prêtes et
quelle image elles exécutent - vert quand toutes le sont, orange quand une partie l'est,
rouge quand aucune ne l'est - tenu à jour par les événements de l'API server plutôt que par
une interrogation à intervalle régulier (voir [Santé des upstreams](/docs/operations/upstream-health)).

Il n'y a d'interrupteur ni pour l'un ni pour l'autre : ce qui les ouvre, ce sont les droits
qu'accorde le déploiement. Le chart accorde les deux (`rbac.read` et `rbac.watch`, actifs
par défaut). À la main, le ServiceAccount a besoin de `list` et `watch` sur les services et
les pods de son propre namespace, et de rien d'autre - aucun secret, aucun autre namespace.
Sans les pods, l'éditeur propose toujours les Services et les cibles sont vérifiées par une
connexion ; sans rien, la console indique le droit qui lui manque, la saisie libre restant
exactement ce qu'elle était.

Une route dont l'upstream vit dans un AUTRE namespace (`grafana.monitoring.svc`) se lit de
la même façon dès que ce namespace figure dans `rbac.watchNamespaces` : le chart crée un Role
dans chaque namespace nommé, jamais un ClusterRole, donc la gateway lit les namespaces que
quelqu'un a choisis, pas le cluster entier. Un namespace absent de la liste est vérifié par
une connexion.

Lire les pods, c'est lire les variables d'environnement que déclarent leurs manifestes. Là
où un manifeste porte un secret en clair, mettez `rbac.watch: false`, ou mieux, rangez ce
secret dans un Secret.

Le tunnel développeur en demande davantage, et les droits qu'il reçoit sont détaillés sur
la page [Une gateway](/docs/deploy/one-gateway).

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
  # Lire les Services et les pods de CE namespace, et rien d'autre : aucun
  # secret, aucun autre namespace. watch est ce qui tient l'écran des routes
  # à jour sans interroger à intervalle régulier.
  - apiGroups: [""]
    resources: ["services", "pods"]
    verbs: ["get", "list", "watch"]
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
# Et nommez-le dans la spécification du pod :
#   serviceAccountName: meerkat
```
:::

## OpenShift et OKD

La même image et le même chart s'installent sur OpenShift et OKD sous la SCC par défaut
`restricted-v2` : aucune SCC supplémentaire à accorder, aucune valeur à changer, rien qui
s'exécute en root. Trois choix rendent cela possible, et il vaut mieux les connaître avant
de surcharger quoi que ce soit :

- **Aucun uid dans le pod.** OpenShift n'exécute pas l'image sous l'utilisateur qu'elle
  déclare : il tire un uid dans la plage du namespace, ajoute le groupe 0, et rejette un pod
  qui nomme le sien - `runAsUser: Invalid value: 65532: must be in the ranges: [...]`. Ni le
  chart ni le manifeste ci-dessus ne définissent donc `runAsUser`, `runAsGroup` ou
  `fsGroup`. **Ne les rajoutez pas** dans vos valeurs : cela n'apporte rien ailleurs,
  puisque l'image porte déjà son uid.
- **`/data` appartient au groupe root, et le groupe a le droit d'écrire.** Quel que soit
  l'uid choisi par OpenShift, il fait partie du groupe 0 : le stockage peut donc écrire son
  premier fichier même là où `/data` est un simple répertoire et non un volume monté.
- **Le `USER` de l'image est numérique (65532).** Dans un namespace Kubernetes ordinaire qui
  applique le niveau Pod Security `restricted`, le kubelet refuse une image dont
  l'utilisateur est un nom, faute de pouvoir prouver que ce n'est pas root. Un nombre passe
  aussi bien là que sur OpenShift.

Tout le reste convient déjà : tous les ports sur lesquels écoute la gateway sont
au-dessus de 1024, elle n'a besoin d'aucune capacité, d'aucune élévation de privilèges ni
d'aucun chemin de l'hôte, et l'accès en lecture seule que demande l'éditeur de routes
(ci-dessus) est un Role ordinaire.

**L'exposer.** Une **Route** remplace l'Ingress, à raison d'une par plan, avec la même
règle : rien ne pointe vers le plan de contrôle à moins que vous ne le vouliez.

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
    # edge : le routeur détient le certificat et Meerkat voit du HTTP avec
    # X-Forwarded-Proto: https. Préférez passthrough quand la gateway
    # détient elle-même le certificat - ce qu'exige ACME, puisque le défi
    # TLS-ALPN doit parvenir intact à Meerkat.
    termination: edge
    insecureEdgeTerminationPolicy: Redirect
```

> [!NOTE] Vérifié chaque nuit
> Ce qui précède n'est pas une promesse : chaque nuit, la CI installe ce chart sur
> MicroShift (OpenShift construit à partir d'OKD), sous la SCC `restricted-v2`, et vérifie
> l'uid qui lui a été attribué, l'écriture dans `/data` et les deux Routes. Voir la
> [plateforme de test](/project/test-platform).

## Avant de dire que c'est fini

- Tout ce qui envoie du trafic sonde /readyz, et non /healthz.
- MEERKAT_VAULT_KEY est définie, avec la même valeur sur tous les pods.
- Le port 9090 n'est publié nulle part : ni règle d'Ingress, ni LoadBalancer, ni NodePort.
- X-Forwarded-Proto arrive, et les réponses de longue durée ne sont ni mises en tampon ni coupées.
- Plus d'une machine peut répondre sur l'adresse publique, et vous l'avez constaté.
- La base a un chemin de bascule, et la restauration a été essayée.
- Tuez un pod pendant un test de charge et regardez les chiffres : c'est la répétition la moins chère que vous aurez.

Une gateway sur un volume est une architecture plus simple, et elle a sa propre page :
[Une gateway](/docs/deploy/one-gateway).
