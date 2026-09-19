import { Component, computed, signal } from '@angular/core';
import { RouterLink } from '@angular/router';

// The Kubernetes cluster page, in both languages (same pattern as the
// Performance page: one T table, a lang signal, t(key)).
//
// Everything asserted here is read from the code, not from a habit:
// - the nodes never talk to each other, all coordination goes through the
//   database (ee/pgdriver LISTEN/NOTIFY, internal/cluster),
// - sessions and the brute-force counter are rows, so no session affinity
//   (ee/changebus/bruteforce_test.go),
// - /healthz is liveness and /readyz is readiness, on both ports
//   (cmd/meerkat/main.go),
// - the vault master key is the ONE thing that is not in the database
//   (internal/store/db.go openPostgres, internal/vault LoadOrCreateKey),
// - certificate material and the ACME challenge live in the shared database,
//   issuance is serialised by an advisory lock (internal/certs).

type Lang = 'en' | 'fr';

// The options for what sits in FRONT of the Services, side by side. A table
// because the reader is choosing, and what they are choosing between is three
// columns wide: does it fail over, does it spread, what does it cost to run.
const ENTRY: {
  option: Record<Lang, string>;
  failover: Record<Lang, string>;
  spread: Record<Lang, string>;
  needs: Record<Lang, string>;
}[] = [
  {
    option: {
      en: 'Cloud LoadBalancer Service (EKS, GKE, AKS)',
      fr: 'Service LoadBalancer du fournisseur (EKS, GKE, AKS)',
    },
    failover: { en: 'Yes, the provider owns it', fr: "Oui, c'est le fournisseur qui le porte" },
    spread: { en: 'Yes, across zones', fr: 'Oui, entre zones' },
    needs: {
      en: 'One line in the manifest, and an hourly bill',
      fr: 'Une ligne dans le manifeste, et une facture horaire',
    },
  },
  {
    option: { en: 'MetalLB, layer 2', fr: 'MetalLB, mode L2' },
    failover: {
      en: 'Yes, a few seconds while ARP caches relearn',
      fr: 'Oui, quelques secondes le temps que les caches ARP réapprennent',
    },
    spread: { en: 'No, one node carries everything', fr: 'Non, un nœud porte tout le trafic' },
    needs: {
      en: 'A spare address on the network the nodes sit on',
      fr: 'Une adresse libre sur le réseau des nœuds',
    },
  },
  {
    option: { en: 'MetalLB, BGP', fr: 'MetalLB, mode BGP' },
    failover: { en: 'Yes, the router withdraws the dead next hop', fr: 'Oui, le routeur retire la route morte' },
    spread: { en: 'Yes, ECMP over the nodes', fr: 'Oui, ECMP entre les nœuds' },
    needs: {
      en: 'A BGP session on the router: your network team is in the room',
      fr: "Une session BGP sur le routeur : l'équipe réseau est dans la boucle",
    },
  },
  {
    option: {
      en: 'keepalived (VRRP) over an Ingress controller DaemonSet',
      fr: 'keepalived (VRRP) devant un Ingress controller en DaemonSet',
    },
    failover: { en: 'Yes, the VIP moves to another machine', fr: "Oui, la VIP passe sur une autre machine" },
    spread: { en: 'No, the VIP is on one machine at a time', fr: "Non, la VIP est sur une machine à la fois" },
    needs: {
      en: 'keepalived on the machines, outside Kubernetes',
      fr: 'keepalived sur les machines, hors de Kubernetes',
    },
  },
  {
    option: { en: 'DNS round robin', fr: 'DNS en tourniquet' },
    failover: {
      en: 'Partly, and only because clients retry the next address',
      fr: 'En partie, et seulement parce que les clients réessaient sur une autre adresse',
    },
    spread: { en: 'Roughly, whatever resolvers cache', fr: 'Grossièrement, au gré des caches des résolveurs' },
    needs: { en: 'Several A records, and one of the rows above behind each', fr: "Plusieurs enregistrements A, et une des lignes ci-dessus derrière chacun" },
  },
];

const YAML_SECRET = {
  en: `# Three values, and none of them belongs in git.
kubectl create secret generic meerkat \\
  --from-literal=database-url='postgres://meerkat:PASSWORD@postgres.data.svc:5432/meerkat?sslmode=require' \\
  --from-literal=admin-password='the-first-admin-password' \\
  --from-literal=vault-key="$(openssl rand -hex 32)"`,
  fr: `# Trois valeurs, et aucune n'a sa place dans git.
kubectl create secret generic meerkat \\
  --from-literal=database-url='postgres://meerkat:MOTDEPASSE@postgres.data.svc:5432/meerkat?sslmode=require' \\
  --from-literal=admin-password='le-premier-mot-de-passe-admin' \\
  --from-literal=vault-key="$(openssl rand -hex 32)"`,
};

const YAML_DEPLOYMENT = {
  en: `apiVersion: apps/v1
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
      # Nothing local to hand over, so the new pod can be serving before the
      # old one leaves: no session is lost and no configuration is missed.
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
      # Three pods on one machine are one machine away from no gateway at all.
      topologySpreadConstraints:
        - maxSkew: 1
          topologyKey: kubernetes.io/hostname
          whenUnsatisfiable: ScheduleAnyway
          labelSelector:
            matchLabels:
              app.kubernetes.io/name: meerkat
      containers:
        - name: meerkat
          # The external database is an Enterprise capability: the community
          # image links no PostgreSQL driver at all. Pin a release rather than
          # riding "latest" on something that answers your front door.
          image: ghcr.io/softwarity/meerkat-ee:latest
          ports:
            - name: app
              containerPort: 8080
            - name: admin
              containerPort: 9090
          env:
            # The shared database. This one line is what makes three pods one
            # gateway instead of three gateways.
            - name: MEERKAT_DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: meerkat
                  key: database-url
            # The vault master key, THE SAME on every pod. It seals what is in
            # the database, so it is not kept there. Leave it out and each pod
            # generates its own, then reads none of the others' secrets.
            - name: MEERKAT_VAULT_KEY
              valueFrom:
                secretKeyRef:
                  name: meerkat
                  key: vault-key
            # Read ONCE, on the first start, to create the admin account.
            - name: MEERKAT_ADMIN_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: meerkat
                  key: admin-password
            # This gateway is a production one: the developer surface stays
            # closed here whatever the database says.
            - name: MEERKAT_PRODUCTION
              value: "1"
          # LIVENESS: is this process still serving? It answers UP without
          # looking at anything, and that is the right answer - this probe
          # decides whether to KILL the process.
          livenessProbe:
            httpGet:
              path: /healthz
              port: app
            initialDelaySeconds: 5
            periodSeconds: 10
          # READINESS: can this node take a request right now? It checks the
          # store and the compiled routing table, and answers 503 naming which
          # of the two failed. This is the one a load balancer must probe.
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
            runAsUser: 65532
            allowPrivilegeEscalation: false
            capabilities:
              drop: ["ALL"]
      volumes:
        # With a shared database this directory holds nothing that has to
        # outlive the pod: no PersistentVolumeClaim, and therefore no
        # ReadWriteOnce volume to hand from one machine to another.
        - name: data
          emptyDir: {}`,
  fr: `apiVersion: apps/v1
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
          image: ghcr.io/softwarity/meerkat-ee:latest
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
          # VIVACITÉ : ce processus sert-il encore ? Il répond UP sans rien
          # regarder, et c'est la bonne réponse - cette sonde décide de TUER
          # le processus.
          livenessProbe:
            httpGet:
              path: /healthz
              port: app
            initialDelaySeconds: 5
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
            runAsUser: 65532
            allowPrivilegeEscalation: false
            capabilities:
              drop: ["ALL"]
      volumes:
        # Avec une base partagée, ce répertoire ne contient rien qui doive
        # survivre au pod : pas de PersistentVolumeClaim, donc aucun volume
        # ReadWriteOnce à se passer d'une machine à l'autre.
        - name: data
          emptyDir: {}`,
};

const YAML_SERVICES = {
  en: `# The data plane: what your users reach.
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
# The control plane: console, admin API and the MCP endpoint. A SECOND
# Service, on purpose: an Ingress put in front of the applications has then no
# way of reaching the console by accident.
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
      targetPort: admin`,
  fr: `# Le plan de données : ce que vos utilisateurs atteignent.
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
      targetPort: admin`,
};

const YAML_INGRESS = {
  en: `apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: meerkat
  annotations:
    # The gateway proxies WebSockets and holds a live channel open
    # (/meerkat/events). Both are long-lived responses: an Ingress that
    # buffers them, or closes at its default 60 s, breaks pages that worked.
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
# Nothing here points at meerkat-admin, and that is the point.`,
  fr: `apiVersion: networking.k8s.io/v1
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
# Rien ici ne pointe vers meerkat-admin, et c'est tout l'intérêt.`,
};

const YAML_RBAC = {
  en: `apiVersion: v1
kind: ServiceAccount
metadata:
  name: meerkat
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: meerkat-services
rules:
  # List the Services of THIS namespace, and nothing else: no secret, no pod,
  # no other namespace.
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
# And name it in the pod spec:
#   serviceAccountName: meerkat`,
  fr: `apiVersion: v1
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
#   serviceAccountName: meerkat`,
};

const T = {
  title: { en: 'Kubernetes cluster', fr: 'Cluster Kubernetes' },
  intro: {
    en: 'Several gateways serving one installation, in front of the same applications. The whole shape follows from one property of the product: the nodes never talk to each other. All the coordination goes through the database they share, which is what lets a Kubernetes deployment be an ordinary one.',
    fr: "Plusieurs passerelles servant une seule installation, devant les mêmes applications. Toute la forme découle d'une propriété du produit : les nœuds ne se parlent jamais. Toute la coordination passe par la base qu'ils partagent, et c'est ce qui rend le déploiement Kubernetes ordinaire.",
  },
  shape1: {
    en: 'What you deploy is four objects and one dependency: a Deployment of three replicas, two ClusterIP Services (one per plane), an Ingress in front of the public one, and one external PostgreSQL. No StatefulSet, no headless Service, no PersistentVolumeClaim, no sidecar, no Redis.',
    fr: "Ce que vous déployez tient en quatre objets et une dépendance : un Deployment de trois répliques, deux Services ClusterIP (un par plan), un Ingress devant celui qui est public, et un PostgreSQL externe. Pas de StatefulSet, pas de Service headless, pas de PersistentVolumeClaim, pas de sidecar, pas de Redis.",
  },
  shape2: {
    en: 'The Helm chart in deploy/helm installs the single-gateway shape: one replica, one volume, Recreate. A cluster changes three things - no volume, a database URL, several replicas - so it is written out in full below rather than hidden behind values.',
    fr: "Le chart Helm de deploy/helm installe la forme mono-passerelle : une réplique, un volume, stratégie Recreate. Un cluster change trois choses - plus de volume, une URL de base, plusieurs répliques - donc il est écrit en entier ci-dessous plutôt que caché derrière des valeurs.",
  },

  whyDeployTitle: {
    en: 'Why a Deployment, and not a StatefulSet',
    fr: 'Pourquoi un Deployment, et pas un StatefulSet',
  },
  whyDeploy1: {
    en: 'A StatefulSet exists to give pods a stable name, a stable order and a volume of their own, so that they can find each other and keep their own state. Meerkat needs none of the three. A node never addresses another node: when one writes a route, it bumps a version in the database and sends a NOTIFY, and the others hear it on the connection they are already holding. That is PostgreSQL LISTEN/NOTIFY, and it is the reason PostgreSQL is the one external option - it was chosen for the transport, not for the storage.',
    fr: "Un StatefulSet existe pour donner aux pods un nom stable, un ordre stable et un volume à eux, afin qu'ils se trouvent et gardent leur état. Meerkat n'a besoin d'aucun des trois. Un nœud n'adresse jamais un autre nœud : quand l'un écrit une route, il incrémente une version en base et envoie un NOTIFY, et les autres l'entendent sur la connexion qu'ils tiennent déjà. C'est LISTEN/NOTIFY de PostgreSQL, et c'est la raison pour laquelle PostgreSQL est la seule option externe : il a été choisi pour le transport, pas pour le stockage.",
  },
  whyDeploy2: {
    en: "So there is no peer-to-peer, no gossip, no Raft, no quorum, no seed list, and no node that has to learn another node's address. Nothing discovers pods. Scaling is one number, and a pod is interchangeable with any other.",
    fr: "Donc pas de pair-à-pair, pas de gossip, pas de Raft, pas de quorum, pas de liste d'amorçage, et aucun nœud qui doive apprendre l'adresse d'un autre. Rien ne découvre les pods. Le dimensionnement est un nombre, et un pod est interchangeable avec n'importe quel autre.",
  },
  whyDeploy3: {
    en: 'The one thing a per-pod volume would carry is the embedded storage - and a database per pod is precisely what a cluster is not.',
    fr: "La seule chose qu'un volume par pod porterait, c'est le stockage embarqué - et une base par pod, c'est exactement ce qu'un cluster n'est pas.",
  },

  stickyTitle: { en: 'No session affinity to ask for', fr: 'Aucune affinité de session à demander' },
  sticky1: {
    en: 'Sessions live in the database, not in a pod. Each node keeps a five-second cache to answer fast, and that cache is invalidated through the same bus, so a revocation or an organisation just chosen on node A is not served stale by node B. A request can therefore land on any replica, which is what makes a rolling update uneventful: a pod that goes takes no session with it.',
    fr: "Les sessions vivent en base, pas dans un pod. Chaque nœud garde un cache de cinq secondes pour répondre vite, et ce cache est invalidé par le même bus : une révocation, ou une organisation qui vient d'être choisie sur le nœud A, n'est pas servie périmée par le nœud B. Une requête peut donc atterrir sur n'importe quelle réplique, et c'est ce qui rend une mise à jour progressive sans histoire : un pod qui part n'emporte aucune session.",
  },
  sticky2: {
    en: 'The brute-force counter is in the database too, one row per failed attempt, and that matters more than it looks. A limit of five attempts used to be five PER GATEWAY: an attacker spraying the load balancer got the limit multiplied by the very thing that was supposed to make the service sturdier. Five attempts are now five for the installation wherever they land, and a successful sign-in forgives on every node. A test says exactly that, and it is the reason nothing here asks your load balancer for sticky sessions.',
    fr: "Le compteur anti-force-brute est en base également, une ligne par tentative échouée, et cela compte plus qu'il n'y paraît. Une limite de cinq tentatives valait cinq PAR PASSERELLE : un attaquant qui arrose le répartiteur voyait la limite multipliée par ce qui était censé rendre le service plus solide. Cinq tentatives valent maintenant cinq pour l'installation, où qu'elles tombent, et une connexion réussie pardonne sur tous les nœuds. Un test le dit exactement, et c'est la raison pour laquelle rien ici ne réclame de session collante au répartiteur.",
  },
  sticky3: {
    en: 'What is relayed rather than shared: a live message published on one node reaches the pages held open by the others, so nobody sees half an announcement. What stays per node, and is worth knowing before you set a number: rate limits count in memory, per node, so N nodes allow up to N times the bound. The console says so where the number is typed.',
    fr: "Ce qui est relayé plutôt que partagé : un message live publié sur un nœud atteint les pages tenues ouvertes par les autres, donc personne ne voit la moitié d'une annonce. Ce qui reste par nœud, et qu'il vaut mieux savoir avant de poser un chiffre : les limites de débit comptent en mémoire, par nœud, donc N nœuds autorisent jusqu'à N fois la borne. La console le dit là où le nombre se tape.",
  },

  dbTitle: { en: 'The prerequisite: one PostgreSQL they share', fr: 'Le prérequis : un PostgreSQL partagé' },
  db1: {
    en: 'The embedded storage is a file one process owns. Point three pods at it and you have three independent gateways: three sets of routes, three sets of accounts, three admin passwords, and a console that shows whichever one your request happened to reach. Nothing would look broken from the binary side, which is what makes it a bad afternoon.',
    fr: "Le stockage embarqué est un fichier possédé par un seul processus. Faites-y pointer trois pods et vous avez trois passerelles indépendantes : trois jeux de routes, trois jeux de comptes, trois mots de passe admin, et une console qui montre celle que votre requête a touchée. Rien ne paraîtrait cassé du côté du binaire, et c'est ce qui en fait un mauvais après-midi.",
  },
  db2: {
    en: 'A cluster therefore starts with MEERKAT_DATABASE_URL pointing at a PostgreSQL every node reaches. It is the single external option, and there is nothing else to run: the notifications are the bus, and an advisory lock is the exclusion the product needs. No Redis, no message broker, no etcd.',
    fr: "Un cluster commence donc par MEERKAT_DATABASE_URL pointant vers un PostgreSQL que tous les nœuds atteignent. C'est la seule option externe, et il n'y a rien d'autre à faire tourner : les notifications sont le bus, et un verrou consultatif est la seule exclusion dont le produit a besoin. Pas de Redis, pas de broker, pas d'etcd.",
  },
  db3: {
    en: 'This is an Enterprise capability, and the driver is what the edition is: the community binary links no PostgreSQL driver at all, so a database URL there gets a sentence naming what is available instead of a driver error. Absent code refuses on its own, and no licence is read.',
    fr: "C'est une capacité Enterprise, et c'est le pilote qui EST l'édition : le binaire communautaire ne lie aucun pilote PostgreSQL, donc une URL de base y reçoit une phrase qui nomme ce qui est disponible, pas une erreur de pilote. Du code absent refuse tout seul, et aucune licence n'est lue.",
  },

  dataTitle: {
    en: 'The data directory, and the one thing that is not in the database',
    fr: 'Le répertoire de données, et la seule chose qui ne soit pas en base',
  },
  data1: {
    en: 'The -data directory is still created with an external database, but it is no longer the source of truth: routes, accounts, sessions, audit trail and certificates are all rows. An emptyDir is enough, which is why there is no PersistentVolumeClaim here and therefore no ReadWriteOnce volume to hand from one machine to another.',
    fr: "Le répertoire -data est toujours créé avec une base externe, mais il n'est plus la source de vérité : routes, comptes, sessions, journal d'audit et certificats sont des lignes. Un emptyDir suffit, d'où l'absence de PersistentVolumeClaim ici, et donc aucun volume ReadWriteOnce à se passer d'une machine à l'autre.",
  },
  data2: {
    en: 'One file is the exception, and it is the one that bites: the vault master key. It seals what is IN the database, so keeping it there would be sealing the door with the key in the lock. Left to itself, each pod generates its own key in its own emptyDir - and then a secret sealed by one node cannot be read by the others, which surfaces as an upstream password that works on one replica in three.',
    fr: "Un fichier fait exception, et c'est celui qui mord : la clé maître du coffre. Elle scelle ce qui est DANS la base, la ranger là serait fermer la porte en laissant la clé dans la serrure. Livré à lui-même, chaque pod génère sa propre clé dans son propre emptyDir - et alors un secret scellé par un nœud est illisible par les autres, ce qui se voit comme un mot de passe d'amont qui marche sur une réplique sur trois.",
  },
  data3: {
    en: 'So in a cluster the key is supplied by the environment, identical on every pod, out of the same Secret. Generate it once and keep it wherever you keep keys - losing it loses every secret in the vault.',
    fr: "En cluster, la clé est donc fournie par l'environnement, identique sur tous les pods, depuis le même Secret. Générez-la une fois et rangez-la là où vous rangez vos clés : la perdre, c'est perdre tous les secrets du coffre.",
  },

  secretTitle: { en: 'The Secret', fr: 'Le Secret' },
  secret1: {
    en: 'Three values, created rather than committed. A manifest with a base64 field is a secret in git, and base64 is not encryption.',
    fr: "Trois valeurs, créées plutôt que versionnées. Un manifeste avec un champ base64 est un secret dans git, et base64 n'est pas un chiffrement.",
  },
  secret2: {
    en: 'MEERKAT_ADMIN_PASSWORD is read on the FIRST start only, to create the first admin account: whichever pod gets there first creates it, the others find it already there. Once you have signed in and made your own account, it can come out of the Secret.',
    fr: "MEERKAT_ADMIN_PASSWORD est lu au PREMIER démarrage seulement, pour créer le premier compte admin : le pod qui arrive le premier le crée, les autres le trouvent déjà là. Une fois connecté et votre propre compte créé, il peut sortir du Secret.",
  },

  deployTitle: { en: 'The Deployment', fr: 'Le Deployment' },
  deploy1: {
    en: 'Both probes exist on both ports, and they are not interchangeable. /healthz is LIVENESS and answers UP unconditionally: this probe decides whether to kill the process, and a liveness probe that failed because the database was unreachable would turn a database blip into a restart of every node at once, each killed for a fault none of them has and none can fix by dying. /readyz is READINESS: it pings the store and checks that the routing table has been compiled, and answers 503 naming which of the two is missing - a node that accepted connections but has not compiled its table yet answers 404 to everything, and a rolling update would read that as a healthy instance.',
    fr: "Les deux sondes existent sur les deux ports, et elles ne sont pas interchangeables. /healthz est la VIVACITÉ et répond UP sans condition : cette sonde décide de tuer le processus, et une sonde de vivacité qui échouerait parce que la base ne répond pas transformerait un hoquet de base en redémarrage simultané de tous les nœuds, chacun tué pour une panne qu'aucun n'a et qu'aucun ne répare en mourant. /readyz est la DISPONIBILITÉ : il pingue le stockage et vérifie que la table de routage est compilée, et répond 503 en nommant lequel des deux manque - un nœud qui a accepté des connexions mais n'a pas encore compilé sa table répond 404 à tout, et une mise à jour progressive y lirait une instance en bonne santé.",
  },
  deploy2: {
    en: 'The rule that follows: whatever sends traffic probes /readyz, and never /healthz.',
    fr: "La règle qui en découle : ce qui envoie du trafic sonde /readyz, jamais /healthz.",
  },

  servicesTitle: { en: 'Two Services, because there are two planes', fr: "Deux Services, parce qu'il y a deux plans" },
  services1: {
    en: 'Port 8080 is the data plane - your applications, the sign-in pages, the live channel. Port 9090 is the control plane - the admin console, the admin API and the MCP endpoint an agent talks to. Only the first belongs on the internet, and they are two Services for exactly that reason: an Ingress in front of your applications must not be able to reach the console by accident, and a single Service with two ports is one annotation away from doing so.',
    fr: "Le port 8080 est le plan de données : vos applications, les pages de connexion, le canal live. Le port 9090 est le plan de contrôle : la console d'administration, l'API admin et l'endpoint MCP auquel parle un agent. Seul le premier a sa place sur internet, et ce sont deux Services pour exactement cette raison : un Ingress placé devant vos applications ne doit pas pouvoir atteindre la console par accident, et un Service unique à deux ports en est à une annotation près.",
  },
  services2: {
    en: 'How an operator reaches the console then, in order of preference: kubectl port-forward service/meerkat-admin 9090:9090 for occasional work; a second Ingress on an internal-only controller, or on the same one restricted by source address and client certificate, when a team needs it daily. What you should not do is put it on the same public host as the applications.',
    fr: "Comment un opérateur atteint alors la console, par ordre de préférence : kubectl port-forward service/meerkat-admin 9090:9090 pour un travail occasionnel ; un second Ingress sur un controller interne, ou sur le même restreint par adresse source et certificat client, quand une équipe en a besoin tous les jours. Ce qu'il ne faut pas faire, c'est la poser sur le même hôte public que les applications.",
  },

  ingressTitle: { en: 'The Ingress', fr: "L'Ingress" },
  ingress1: {
    en: 'Two things a default Ingress gets wrong for this gateway. It proxies WebSockets, and it holds a live channel open on /meerkat/events - both are long-lived responses, so a controller that buffers the body or closes the connection at its default sixty seconds breaks pages that were working, intermittently and only for some users.',
    fr: "Deux choses qu'un Ingress par défaut rate avec cette gateway. Elle proxifie des WebSockets et tient un canal live ouvert sur /meerkat/events - ce sont des réponses longues, donc un controller qui tamponne le corps ou coupe la connexion à ses soixante secondes par défaut casse des pages qui marchaient, de façon intermittente et pour certains utilisateurs seulement.",
  },
  ingress2: {
    en: 'And the scheme. Meerkat can terminate TLS itself (-tls-addr, -admin-tls-addr, with ACME issuance serialised by an advisory lock so N nodes ask for one certificate rather than N, the material and the challenge being rows every node can answer from). In Kubernetes you usually terminate at the Ingress instead, and both work. But whatever terminates in front must send X-Forwarded-Proto: the gateway reads the scheme from it as much as from the connection, and without it three things go wrong at once - session cookies lose their Secure attribute, the console announces http:// URLs, and the redirect to HTTPS loops for ever. nginx and Traefik both set it by default; anything hand-rolled in front has to be checked.',
    fr: "Et le schéma. Meerkat sait terminer TLS elle-même (-tls-addr, -admin-tls-addr, avec une émission ACME sérialisée par un verrou consultatif pour que N nœuds demandent un certificat et non N, la matière et le défi étant des lignes auxquelles n'importe quel nœud peut répondre). En Kubernetes on termine plutôt à l'Ingress, et les deux marchent. Mais ce qui termine devant doit envoyer X-Forwarded-Proto : la gateway lit le schéma là autant que sur la connexion, et sans lui trois choses se cassent d'un coup - les cookies de session perdent leur attribut Secure, la console annonce des URL en http://, et la redirection vers HTTPS boucle indéfiniment. nginx et Traefik l'envoient par défaut ; tout montage maison placé devant doit être vérifié.",
  },

  entryTitle: {
    en: 'What sits in front, and how not to make it the single point of failure',
    fr: "Ce qui est devant, et comment ne pas en faire le point unique de défaillance",
  },
  entry1: {
    en: 'A ClusterIP Service is reachable from inside the cluster only. Something in front must therefore own the address your users type - and that address is now the thing that can take the whole installation down. Three replicas behind a name that resolves to one machine is a one-machine cluster with extra steps, and this is the part of a Kubernetes deployment that is genuinely a decision rather than a manifest.',
    fr: "Un Service ClusterIP n'est joignable que depuis l'intérieur du cluster. Quelque chose devant doit donc porter l'adresse que vos utilisateurs tapent - et cette adresse est désormais ce qui peut faire tomber toute l'installation. Trois répliques derrière un nom qui résout vers une seule machine, c'est un cluster d'une machine avec des étapes en plus, et c'est la partie d'un déploiement Kubernetes qui est vraiment une décision plutôt qu'un manifeste.",
  },
  entryCloud: {
    en: 'On a managed cluster (EKS, GKE, AKS), there is nothing to solve: give the data-plane Service type: LoadBalancer and the provider gives you one stable address carried by its own highly available load balancer, health-checked and spread across zones. It is the right answer wherever it is available, and the trade is that the entry path is theirs, billed by the hour, and configured through annotations rather than by you.',
    fr: "Sur un cluster managé (EKS, GKE, AKS), il n'y a rien à résoudre : donnez au Service du plan de données type: LoadBalancer et le fournisseur vous rend une adresse stable, portée par son propre répartiteur hautement disponible, sondé et réparti entre zones. C'est la bonne réponse partout où elle existe, et le compromis est que le chemin d'entrée est le leur, facturé à l'heure, et réglé par des annotations plutôt que par vous.",
  },
  entryMetal: {
    en: 'On bare metal there is no such provider, so type: LoadBalancer stays pending for ever unless you install something that implements it. Three usual answers, and they are not equivalent.',
    fr: "Sur du matériel nu, ce fournisseur n'existe pas : type: LoadBalancer reste en attente indéfiniment si rien n'implémente ce rôle. Trois réponses habituelles, et elles ne sont pas équivalentes.",
  },
  entryL2: {
    en: 'MetalLB in layer 2 mode takes a spare address on the network the nodes sit on and has one node answer the ARP requests for it. When that node dies, another takes the address over and shouts a gratuitous ARP so the switches relearn. That is real failover, in seconds - but it is not load spreading: every packet enters through one machine, which is a bandwidth ceiling and a single machine whose failure is visible, briefly, to everyone.',
    fr: "MetalLB en mode L2 prend une adresse libre sur le réseau des nœuds et fait répondre un seul nœud aux requêtes ARP la concernant. Quand ce nœud meurt, un autre reprend l'adresse et émet un ARP gratuit pour que les commutateurs réapprennent. C'est un vrai basculement, en quelques secondes - mais ce n'est pas de la répartition : chaque paquet entre par une machine, ce qui est un plafond de bande passante et une machine dont la panne se voit, brièvement, de tous.",
  },
  entryBgp: {
    en: 'MetalLB in BGP mode has every node announce the same address to your router, which installs several equal-cost next hops and spreads flows over them. That is failover AND spreading, and it is the honest answer for a bare-metal cluster that has to hold a load. The price is not technical: it needs a BGP session on the router, so your network team is in the room, and the router hashing decides which node a flow lands on - a topology change can move flows that were already open.',
    fr: "MetalLB en mode BGP fait annoncer la même adresse par tous les nœuds à votre routeur, qui installe plusieurs chemins de coût égal et répartit les flux entre eux. C'est du basculement ET de la répartition, et c'est la réponse honnête pour un cluster sur matériel nu qui doit tenir une charge. Le prix n'est pas technique : il faut une session BGP sur le routeur, donc l'équipe réseau est dans la boucle, et le hachage du routeur décide sur quel nœud tombe un flux - un changement de topologie peut déplacer des flux déjà ouverts.",
  },
  entryVrrp: {
    en: 'keepalived, or any VRRP implementation, in front of an Ingress controller running as a DaemonSet with hostNetwork: the controller answers on every machine, and a virtual address floats between them by VRRP. Same trade as MetalLB layer 2 - failover yes, one machine at a time - with one more thing to run outside Kubernetes, and it is the answer where MetalLB cannot be installed or where a VRRP pair already exists.',
    fr: "keepalived, ou toute implémentation de VRRP, devant un Ingress controller en DaemonSet avec hostNetwork : le controller répond sur chaque machine, et une adresse virtuelle flotte entre elles par VRRP. Même compromis que MetalLB en L2 - basculement oui, une machine à la fois - avec une chose de plus à faire tourner hors de Kubernetes, et c'est la réponse là où MetalLB ne peut pas être installé ou là où une paire VRRP existe déjà.",
  },
  entryDns: {
    en: 'DNS round robin - several A records for one name - is often proposed as the cheap version, and on its own it is not a failover mechanism: a record keeps being handed out while the machine behind it is down, for as long as the TTL and the resolvers that ignore it. What rescues it is the clients: browsers and modern HTTP clients try the next address when a connection is refused. So it spreads sessions acceptably and fails over imperfectly, and it belongs on top of one of the rows above - several records, each pointing at an address that can itself move - rather than alone.',
    fr: "Le DNS en tourniquet - plusieurs enregistrements A pour un nom - est souvent proposé comme la version économique, et seul, ce n'est pas un mécanisme de basculement : un enregistrement continue d'être servi alors que la machine derrière est tombée, le temps du TTL et des résolveurs qui l'ignorent. Ce qui le sauve, ce sont les clients : les navigateurs et les clients HTTP modernes essaient l'adresse suivante quand une connexion est refusée. Il répartit donc les sessions acceptablement et bascule imparfaitement, et il se pose par-dessus une des lignes ci-dessus - plusieurs enregistrements, chacun vers une adresse qui peut elle-même bouger - plutôt que seul.",
  },
  traefikTitle: { en: 'We use Traefik. Does that not solve it?', fr: 'On utilise Traefik. Ça ne règle pas la question ?' },
  traefik1: {
    en: 'Half of it, and it is worth being precise because the two things share a name. Traefik - like nginx, HAProxy or Envoy - IS an Ingress controller: it is what reads the Ingress object above and does the routing, TLS termination and header work. In that sense it replaces the Ingress layer of these manifests entirely, and you would write an IngressRoute instead. Fine.',
    fr: "La moitié, et il vaut la peine d'être précis parce que les deux choses portent le même nom. Traefik - comme nginx, HAProxy ou Envoy - EST un Ingress controller : c'est lui qui lit l'objet Ingress ci-dessus et fait le routage, la terminaison TLS et le travail sur les en-têtes. À ce titre il remplace entièrement la couche Ingress de ces manifestes, et vous écririez un IngressRoute à la place. Très bien.",
  },
  traefik2: {
    en: 'What it does not replace is what stands in FRONT of it, because Traefik runs on machines too. Its pods are pods: they are scheduled somewhere, and whatever address your users resolve has to reach them. On a managed cluster that address is the LoadBalancer Service in front of Traefik, and the provider makes it highly available. On bare metal, nothing does it by itself - so Traefik needs exactly the same MetalLB or VRRP address as the Ingress above. Deploying two Traefik replicas does not give you two entry addresses; it gives you two backends behind whatever single address you have not built yet.',
    fr: "Ce qu'il ne remplace pas, c'est ce qui se tient DEVANT lui, parce que Traefik tourne aussi sur des machines. Ses pods sont des pods : ils sont planifiés quelque part, et l'adresse que vos utilisateurs résolvent doit les atteindre. Sur un cluster managé, cette adresse est le Service LoadBalancer devant Traefik, et le fournisseur la rend hautement disponible. Sur du matériel nu, rien ne le fait tout seul : Traefik a besoin exactement de la même adresse MetalLB ou VRRP que l'Ingress ci-dessus. Déployer deux répliques de Traefik ne donne pas deux adresses d'entrée ; cela donne deux backends derrière l'adresse unique que vous n'avez pas encore construite.",
  },
  traefik3: {
    en: 'The short form: the Ingress object and the Ingress controller are two different things, and a controller only removes the need for the first.',
    fr: "En bref : l'objet Ingress et l'Ingress controller sont deux choses différentes, et un controller ne supprime que le besoin de la première.",
  },
  colOption: { en: 'In front', fr: 'Devant' },
  colFailover: { en: 'Fails over', fr: 'Bascule' },
  colSpread: { en: 'Spreads the load', fr: 'Répartit la charge' },
  colNeeds: { en: 'Needs', fr: 'Demande' },

  dbHaTitle: { en: 'The fragile part is the database', fr: "Le point fragile, c'est la base" },
  dbHa1: {
    en: 'Replicas make the gateway survive a machine. They do nothing for the thing all of them read: there is one database, and no number of pods makes it two. Once the entry address can move, the database is the single point of failure of the installation, and that is where the remaining work belongs.',
    fr: "Les répliques font survivre la gateway à la perte d'une machine. Elles ne font rien pour ce que toutes lisent : il y a une base, et aucun nombre de pods ne la rend double. Dès lors que l'adresse d'entrée peut bouger, la base est le point unique de défaillance de l'installation, et c'est là que va le travail restant.",
  },
  dbHa2: {
    en: 'What happens when it goes is at least honest. /readyz answers 503 with the reason - the store is not answering - so Kubernetes takes those pods out of the Service endpoints while /healthz keeps saying UP and nothing gets restarted. The gateway stops serving rather than serving wrong answers, and it comes back by itself when the database does: no restart, no manual step, nothing to unwedge.',
    fr: "Ce qui se passe quand elle tombe est au moins honnête. /readyz répond 503 avec la raison - le stockage ne répond pas - donc Kubernetes retire ces pods des endpoints du Service, tandis que /healthz continue de dire UP et que rien n'est redémarré. La gateway cesse de servir plutôt que de servir des réponses fausses, et elle revient d'elle-même quand la base revient : pas de redémarrage, pas de geste manuel, rien à débloquer.",
  },
  dbHa3: {
    en: 'So: a managed PostgreSQL with automatic failover, or an operator of the Patroni class with a synchronous standby, and MEERKAT_DATABASE_URL pointing at the name that follows the primary rather than at a host. Then measure the failover window, because that window is the one in which no node is ready - and test the restore, which is the only way to know a backup exists.',
    fr: "Donc : un PostgreSQL managé avec basculement automatique, ou un opérateur de la classe Patroni avec une réplique synchrone, et MEERKAT_DATABASE_URL pointant vers le nom qui suit le primaire plutôt que vers un hôte. Mesurez ensuite la fenêtre de basculement, parce que cette fenêtre est celle où aucun nœud n'est prêt - et testez la restauration, seule manière de savoir qu'une sauvegarde existe.",
  },

  discoveryTitle: {
    en: 'Optional: let the route editor see the namespace',
    fr: "Optionnel : laisser l'éditeur de routes voir le namespace",
  },
  discovery1: {
    en: 'The gateway can list the Services of its OWN namespace and offer them when somebody creates a route, so an upstream becomes a pick rather than a typed URL. There is no switch for it: what opens it is what the deployment grants. The ServiceAccount needs list on services in its own namespace and nothing else - no secret, no pod, no other namespace - and without the right the console says which one it lacks while free typing stays exactly as it was.',
    fr: "La gateway sait lister les Services de SON namespace et les proposer au moment où l'on crée une route : un amont devient un choix plutôt qu'une URL tapée. Il n'y a pas d'interrupteur : ce qui l'ouvre est ce que le déploiement accorde. Le ServiceAccount a besoin de list sur services dans son propre namespace et de rien d'autre - aucun secret, aucun pod, aucun autre namespace - et sans ce droit la console dit lequel lui manque, la saisie libre restant telle quelle.",
  },

  checkTitle: { en: 'Before calling it done', fr: "Avant de dire que c'est fini" },
  check1: {
    en: 'Whatever sends traffic probes /readyz, not /healthz.',
    fr: "Ce qui envoie du trafic sonde /readyz, pas /healthz.",
  },
  check2: {
    en: 'MEERKAT_VAULT_KEY is set, and is the same value on every pod.',
    fr: "MEERKAT_VAULT_KEY est posée, et c'est la même valeur sur tous les pods.",
  },
  check3: {
    en: 'Port 9090 is published nowhere: no Ingress rule, no LoadBalancer, no NodePort.',
    fr: "Le port 9090 n'est publié nulle part : aucune règle d'Ingress, aucun LoadBalancer, aucun NodePort.",
  },
  check4: {
    en: 'X-Forwarded-Proto arrives, and the long-lived responses are neither buffered nor cut.',
    fr: "X-Forwarded-Proto arrive, et les réponses longues ne sont ni tamponnées ni coupées.",
  },
  check5: {
    en: 'More than one machine can answer the public address, and you have seen it happen.',
    fr: "Plus d'une machine peut répondre à l'adresse publique, et vous l'avez vu se produire.",
  },
  check6: {
    en: 'The database has a failover path, and the restore has been tried.',
    fr: "La base a un chemin de basculement, et la restauration a été essayée.",
  },
  check7: {
    en: 'Kill one pod during a load test and watch the figures: this is the cheapest rehearsal you will get.',
    fr: "Tuez un pod pendant un test de charge et regardez les chiffres : c'est la répétition la moins chère que vous aurez.",
  },

  backToDeploy: { en: 'Single gateway, Docker Compose and Helm: the Deploy page.', fr: 'Passerelle unique, Docker Compose et Helm : la page Deploy.' },
};

@Component({
  selector: 'app-cluster',
  imports: [RouterLink],
  styles: [
    `
      .lang {
        float: right;
        display: flex;
        gap: 4px;
      }
      .lang button {
        border: 1px solid var(--border-color);
        background: var(--bg-secondary);
        color: var(--text-primary);
        border-radius: 6px;
        padding: 3px 10px;
        cursor: pointer;
        font-size: 0.8em;
      }
      .lang button.on {
        border-color: var(--accent, #25c2e0);
        color: var(--accent, #25c2e0);
      }
      pre {
        background: var(--bg-secondary);
        border: 1px solid var(--border-color);
        border-radius: 6px;
        padding: 14px 16px;
        overflow-x: auto;

        code {
          font-family: 'Courier New', Consolas, monospace;
          font-size: 0.86em;
          color: var(--text-primary);
          white-space: pre;
        }
      }
      table {
        width: 100%;
        border-collapse: collapse;
        margin: 16px 0;
      }
      th,
      td {
        text-align: left;
        padding: 8px 10px;
        border-bottom: 1px solid var(--border-color);
        vertical-align: top;
      }
      th {
        font-size: 0.8em;
        text-transform: uppercase;
        letter-spacing: 0.06em;
        color: var(--text-secondary);
      }
      .muted {
        color: var(--text-muted);
      }
    `,
  ],
  template: `
    <div class="lang">
      <button [class.on]="lang() === 'en'" (click)="lang.set('en')">EN</button>
      <button [class.on]="lang() === 'fr'" (click)="lang.set('fr')">FR</button>
    </div>
    <h2>{{ t('title') }}</h2>
    <p>{{ t('intro') }}</p>
    <p>{{ t('shape1') }}</p>
    <p class="muted">{{ t('shape2') }}</p>

    <h3>{{ t('whyDeployTitle') }}</h3>
    <p>{{ t('whyDeploy1') }}</p>
    <p>{{ t('whyDeploy2') }}</p>
    <p>{{ t('whyDeploy3') }}</p>

    <h3>{{ t('stickyTitle') }}</h3>
    <p>{{ t('sticky1') }}</p>
    <p>{{ t('sticky2') }}</p>
    <p>{{ t('sticky3') }}</p>

    <h3>{{ t('dbTitle') }}</h3>
    <p>{{ t('db1') }}</p>
    <p>{{ t('db2') }}</p>
    <div class="callout">{{ t('db3') }}</div>

    <h3>{{ t('dataTitle') }}</h3>
    <p>{{ t('data1') }}</p>
    <div class="callout warn">{{ t('data2') }}</div>
    <p>{{ t('data3') }}</p>

    <h3>{{ t('secretTitle') }}</h3>
    <p>{{ t('secret1') }}</p>
    <pre><code>{{ yamlSecret() }}</code></pre>
    <p>{{ t('secret2') }}</p>

    <h3>{{ t('deployTitle') }}</h3>
    <p>{{ t('deploy1') }}</p>
    <p>{{ t('deploy2') }}</p>
    <pre><code>{{ yamlDeployment() }}</code></pre>

    <h3>{{ t('servicesTitle') }}</h3>
    <p>{{ t('services1') }}</p>
    <pre><code>{{ yamlServices() }}</code></pre>
    <p>{{ t('services2') }}</p>

    <h3>{{ t('ingressTitle') }}</h3>
    <p>{{ t('ingress1') }}</p>
    <p>{{ t('ingress2') }}</p>
    <pre><code>{{ yamlIngress() }}</code></pre>

    <h3>{{ t('entryTitle') }}</h3>
    <p>{{ t('entry1') }}</p>
    <p>{{ t('entryCloud') }}</p>
    <p>{{ t('entryMetal') }}</p>
    <ul>
      <li>{{ t('entryL2') }}</li>
      <li>{{ t('entryBgp') }}</li>
      <li>{{ t('entryVrrp') }}</li>
    </ul>
    <p>{{ t('entryDns') }}</p>
    <table>
      <tr>
        <th>{{ t('colOption') }}</th>
        <th>{{ t('colFailover') }}</th>
        <th>{{ t('colSpread') }}</th>
        <th>{{ t('colNeeds') }}</th>
      </tr>
      @for (row of entryRows(); track row.option) {
        <tr>
          <td>{{ row.option }}</td>
          <td>{{ row.failover }}</td>
          <td>{{ row.spread }}</td>
          <td>{{ row.needs }}</td>
        </tr>
      }
    </table>

    <h4>{{ t('traefikTitle') }}</h4>
    <p>{{ t('traefik1') }}</p>
    <p>{{ t('traefik2') }}</p>
    <div class="callout">{{ t('traefik3') }}</div>

    <h3>{{ t('dbHaTitle') }}</h3>
    <p>{{ t('dbHa1') }}</p>
    <p>{{ t('dbHa2') }}</p>
    <p>{{ t('dbHa3') }}</p>

    <h3>{{ t('discoveryTitle') }}</h3>
    <p>{{ t('discovery1') }}</p>
    <pre><code>{{ yamlRbac() }}</code></pre>

    <h3>{{ t('checkTitle') }}</h3>
    <ul>
      <li>{{ t('check1') }}</li>
      <li>{{ t('check2') }}</li>
      <li>{{ t('check3') }}</li>
      <li>{{ t('check4') }}</li>
      <li>{{ t('check5') }}</li>
      <li>{{ t('check6') }}</li>
      <li>{{ t('check7') }}</li>
    </ul>

    <p class="muted"><a routerLink="/deploy">{{ t('backToDeploy') }}</a></p>
  `,
})
export class ClusterComponent {
  protected readonly lang = signal<Lang>('en');

  protected readonly yamlSecret = computed(() => YAML_SECRET[this.lang()]);
  protected readonly yamlDeployment = computed(() => YAML_DEPLOYMENT[this.lang()]);
  protected readonly yamlServices = computed(() => YAML_SERVICES[this.lang()]);
  protected readonly yamlIngress = computed(() => YAML_INGRESS[this.lang()]);
  protected readonly yamlRbac = computed(() => YAML_RBAC[this.lang()]);

  protected readonly entryRows = computed(() =>
    ENTRY.map((row) => ({
      option: row.option[this.lang()],
      failover: row.failover[this.lang()],
      spread: row.spread[this.lang()],
      needs: row.needs[this.lang()],
    })),
  );

  protected t(key: keyof typeof T): string {
    return T[key][this.lang()];
  }
}
