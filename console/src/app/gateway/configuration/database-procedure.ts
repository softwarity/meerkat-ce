import { BackupInfo, Platform, PlatformName } from '../../api.service';

// The commands of the Database tab, for the platform this gateway runs on.
//
// The gateway picks its database at start (MEERKAT_DATABASE_URL set or not),
// and the deployment files stay the one truth about it: nothing here keeps a
// pointer of its own. So the move ends with a change to the deployment, and
// what is printed is THAT change for THIS platform - a Helm release, a Swarm
// service, a compose file, a docker run - with the names the
// runtime gave, so it is a paste and not a puzzle. Where a name could not be
// read, a placeholder in capitals says what to put there.

// A file to paste: a name the snippet colours by, and its lines.
export interface Snippet {
  name: string;
  content: string;
}

// What the commands are written with.
export interface Ctx {
  b: BackupInfo;
  p: Platform;
  // The new database's URL, with the password typed in the form.
  dbUrl: string;
}

// What the commands can be written for. Meerkat ships as an image: a binary
// run straight on a host is not a way it is deployed, so it is not offered.
export const platformNames: PlatformName[] = ['kubernetes', 'openshift', 'swarm', 'compose', 'docker'];

const SECRET = 'meerkat-database';
const SNAPSHOT_DB = 'meerkat-YYYY-MM-DD.db';
const SNAPSHOT_SQL = 'meerkat-YYYY-MM-DD.sql';

// A key the gateway already reads from its environment is already in the
// deployment, and stays: setting it again would only risk a different one. A
// key in a file has to come along.
function keyNote(b: BackupInfo): string {
  return b.keyFromEnv
    ? '# the vault key: the MEERKAT_VAULT_KEY this gateway already has, unchanged'
    : `# VAULT_KEY: the content of ${b.keyFile}, the same on every node`;
}

function k8s(p: Platform) {
  const ns = p.namespace || 'NAMESPACE';
  const release = p.release || 'RELEASE';
  const deploy = p.workload || (p.release ? `${p.release}-meerkat` : 'DEPLOYMENT');
  return { ns, release, deploy, claim: `${deploy}-data` };
}

function isK8s(p: Platform): boolean {
  return p.name === 'kubernetes' || p.name === 'openshift';
}

// The comments that say where a name came from when it is not this runtime's.
function k8sNotes(p: Platform): string[] {
  const out: string[] = [];
  if (p.helmAssumed) out.push('# Helm assumed: this pod could not be read (' + (p.why || 'no answer') + ')');
  if (!p.namespace) out.push('# NAMESPACE: where the gateway runs');
  if (p.helm && !p.release) out.push('# RELEASE: as helm list -n NAMESPACE names it');
  return out;
}

function swarmService(p: Platform): { svc: string; short: string } {
  const svc = p.workload || 'SERVICE';
  const short = p.stack && svc.startsWith(p.stack + '_') ? svc.slice(p.stack.length + 1) : svc;
  return { svc, short };
}

// `cd <dir> && docker compose -f a -f b`, as the project was brought up.
function compose(p: Platform): string {
  const files = (p.files ?? []).length > 1 ? (p.files ?? []).map((f) => ` -f ${f}`).join('') : '';
  return (p.dir ? `cd ${p.dir} && ` : '') + `docker compose${files}`;
}

function composeFile(p: Platform): string {
  return (p.files ?? [])[0] || 'docker-compose.yml';
}

function container(p: Platform): string {
  return (p.name === 'compose' ? p.container : p.workload) || 'CONTAINER';
}

// ---------------------------------------------------------------------------
// To PostgreSQL: point the deployment at the new database.

export function switchToPostgres(c: Ctx): Snippet[] {
  const { b, p, dbUrl } = c;
  const withKey = !b.keyFromEnv;
  if (isK8s(p)) {
    const { ns, release, deploy } = k8s(p);
    const secret = [
      ...k8sNotes(p),
      withKey ? '# the new database and the vault key, as a Secret' : '# the new database, as a Secret (the vault key is already in the deployment, unchanged)',
      ...(withKey ? [keyNote(b)] : []),
      `kubectl -n ${ns} create secret generic ${SECRET} \\`,
      withKey ? `  --from-literal=database-url='${dbUrl}' \\` : `  --from-literal=database-url='${dbUrl}'`,
      ...(withKey ? [`  --from-literal=vault-key='VAULT_KEY'`] : []),
    ];
    if (p.helm) {
      return [
        {
          name: 'switch-to-postgres.sh',
          content: [
            ...secret,
            '',
            '# point the release at it: the pods restart on PostgreSQL, unpaused',
            `helm upgrade ${release} meerkat/meerkat -n ${ns} --reuse-values \\`,
            withKey ? `  --set database.existingSecret=${SECRET} \\` : `  --set database.existingSecret=${SECRET}`,
            ...(withKey ? [`  --set vault.existingSecret=${SECRET}`] : []),
            '',
            '# optional - PostgreSQL lets several pods serve as ONE gateway; one is fine too',
            `# helm upgrade ${release} meerkat/meerkat -n ${ns} --reuse-values --set replicaCount=2`,
          ].join('\n'),
        },
      ];
    }
    return [
      { name: 'switch-to-postgres.sh', content: secret.join('\n') },
      {
        name: 'deployment.yaml',
        content: [
          `# deployment ${deploy}, the meerkat container, under env:`,
          '- name: MEERKAT_DATABASE_URL',
          '  valueFrom:',
          `    secretKeyRef: { name: ${SECRET}, key: database-url }`,
          ...(withKey
            ? ['- name: MEERKAT_VAULT_KEY', '  valueFrom:', `    secretKeyRef: { name: ${SECRET}, key: vault-key }`]
            : []),
          '# optional - PostgreSQL lets several pods serve as ONE gateway: replicas: 2',
          `# then: kubectl -n ${ns} apply -f deployment.yaml`,
        ].join('\n'),
      },
    ];
  }
  if (p.name === 'swarm') {
    const { svc, short } = swarmService(p);
    return [
      {
        name: 'switch-to-postgres.sh',
        content: [
          ...(p.workload ? [] : ['# SERVICE: as docker service ls names it']),
          ...(withKey ? [keyNote(b)] : []),
          '# the new database, on the service: its tasks restart on PostgreSQL, unpaused',
          'docker service update \\',
          `  --env-add MEERKAT_DATABASE_URL='${dbUrl}' \\`,
          ...(withKey ? [`  --env-add MEERKAT_VAULT_KEY='VAULT_KEY' \\`] : []),
          `  ${svc}`,
          '',
          '# optional - PostgreSQL lets several tasks serve as ONE gateway; one is fine too',
          `# docker service scale ${svc}=3`,
        ].join('\n'),
      },
      {
        name: 'stack.yml',
        content: [
          `# and in the stack file${p.stack ? ' of ' + p.stack : ''}, so the next docker stack deploy keeps it:`,
          'services:',
          `  ${short}:`,
          '    environment:',
          '      MEERKAT_DATABASE_URL: ${MEERKAT_DATABASE_URL}',
          ...(withKey ? ['      MEERKAT_VAULT_KEY: ${MEERKAT_VAULT_KEY}'] : []),
          '# exported, in the shell that deploys - never written in the file',
        ].join('\n'),
      },
    ];
  }
  if (p.name === 'compose') {
    const svc = p.workload || 'meerkat';
    return [
      {
        name: composeFile(p),
        content: [
          `# in ${composeFile(p)}:`,
          'services:',
          `  ${svc}:`,
          '    environment:',
          '      MEERKAT_DATABASE_URL: ${MEERKAT_DATABASE_URL}',
          ...(withKey ? ['      MEERKAT_VAULT_KEY: ${MEERKAT_VAULT_KEY}'] : []),
        ].join('\n'),
      },
      {
        name: 'switch-to-postgres.sh',
        content: [
          '# their values, in the .env file beside it (compose reads it), not in the file',
          `MEERKAT_DATABASE_URL='${dbUrl}'`,
          ...(withKey ? [keyNote(b), `MEERKAT_VAULT_KEY='VAULT_KEY'`] : []),
          '',
          '# the container is recreated on PostgreSQL, unpaused',
          `${compose(p)} up -d ${svc}`,
        ].join('\n'),
      },
    ];
  }
  const name = container(p);
  return [
    {
      name: 'switch-to-postgres.sh',
      content: [
        ...(p.why ? [`# ${p.why}: a Compose project or a Swarm service is switched in its file instead`] : []),
        ...(withKey ? [keyNote(b)] : []),
        '# the container again, with the same docker run as before (image, ports, volume) and:',
        `docker stop ${name} && docker rm ${name}`,
        `docker run -d --name ${name} ... \\`,
        `  -e MEERKAT_DATABASE_URL='${dbUrl}' \\`,
        ...(withKey ? [`  -e MEERKAT_VAULT_KEY='VAULT_KEY' \\`] : []),
        '  IMAGE',
      ].join('\n'),
    },
  ];
}

// The way back from PostgreSQL to the database the copy was taken from: the
// switch, undone command for command on this platform, while the database
// left behind is still there to go back to.
export function wayBackFromPostgres(c: Ctx): Snippet {
  const { b, p } = c;
  const withKey = !b.keyFromEnv;
  const lines: string[] = [];
  if (b.dialect === 'sqlite') {
    lines.push(`# ${b.dbFile} is untouched where it was: keep it until the new database has proven itself`);
  } else {
    lines.push('# the database the copy was taken from is untouched: going back points the deployment at it again');
  }
  if (isK8s(p)) {
    const { ns, release, claim, deploy } = k8s(p);
    if (b.dialect === 'sqlite') {
      lines.push(`# (on the claim ${claim}; an older chart deletes it on the switch - download a snapshot of the same kind first)`);
    }
    if (p.helm) {
      lines.push(
        '',
        b.dialect === 'sqlite' ? '# the release back on its own file, one pod' : '# the release back on the previous Secret',
        `helm upgrade ${release} meerkat/meerkat -n ${ns} --reuse-values \\`,
        ...(b.dialect === 'sqlite'
          ? [
              withKey ? '  --set database.existingSecret="" --set vault.existingSecret="" \\' : '  --set database.existingSecret="" \\',
              '  --set replicaCount=1',
            ]
          : ['  --set database.existingSecret=PREVIOUS_SECRET']),
      );
    } else {
      lines.push(
        '',
        `# deployment ${deploy}: the MEERKAT_DATABASE_URL entry out${withKey ? ' (and MEERKAT_VAULT_KEY)' : ''}, replicas: 1`,
        `kubectl -n ${ns} apply -f deployment.yaml`,
      );
    }
    lines.push('', '# then the Secret the switch created, which nothing reads any more', `kubectl -n ${ns} delete secret ${SECRET}`);
  } else if (p.name === 'swarm') {
    const { svc } = swarmService(p);
    lines.push(
      '',
      '# the variables the switch added, out again; one task',
      'docker service update \\',
      '  --env-rm MEERKAT_DATABASE_URL \\',
      ...(withKey ? ['  --env-rm MEERKAT_VAULT_KEY \\'] : []),
      `  --replicas 1 ${svc}`,
      `# and the same lines out of the stack file${p.stack ? ' of ' + p.stack : ''}, or the next docker stack deploy brings them back`,
    );
  } else if (p.name === 'compose') {
    lines.push(
      '',
      `# the MEERKAT_DATABASE_URL line${withKey ? 's (and MEERKAT_VAULT_KEY)' : ''} out of ${composeFile(p)} and of .env, then`,
      `${compose(p)} up -d ${p.workload || 'meerkat'}`,
    );
  } else {
    const name = container(p);
    lines.push(
      '',
      '# the container again, with the docker run it had before the switch',
      `docker stop ${name} && docker rm ${name}`,
      `docker run -d --name ${name} ... IMAGE   # without -e MEERKAT_DATABASE_URL${withKey ? ' and -e MEERKAT_VAULT_KEY' : ''}`,
    );
  }
  return { name: 'way-back.sh', content: lines.join('\n') };
}

// ---------------------------------------------------------------------------
// The file side: stop, put a file in the data directory, start.

function stop(p: Platform): string[] {
  if (isK8s(p)) {
    const { ns, deploy } = k8s(p);
    return [`kubectl -n ${ns} scale deployment/${deploy} --replicas=0`];
  }
  if (p.name === 'swarm') return [`docker service scale ${swarmService(p).svc}=0`];
  if (p.name === 'compose') return [`${compose(p)} stop ${p.workload || 'meerkat'}`];
  return [`docker stop ${container(p)}`];
}

function start(p: Platform, replicas = '1'): string[] {
  if (isK8s(p)) {
    const { ns, deploy } = k8s(p);
    return [`kubectl -n ${ns} scale deployment/${deploy} --replicas=${replicas}`];
  }
  if (p.name === 'swarm') return [`docker service scale ${swarmService(p).svc}=${replicas}`];
  if (p.name === 'compose') return [`${compose(p)} start ${p.workload || 'meerkat'}`];
  return [`docker start ${container(p)}`];
}

// The current database kept aside (or dropped, for a migration), the file put
// in its place, owned like the directory: a file copied in as root is one the
// gateway cannot write. Into a STOPPED gateway's data directory - a volume
// everywhere but on a host, so a helper with the same volume does it.
function place(b: BackupInfo, p: Platform, aside: boolean): string[] {
  const db = b.dbFile;
  const clear = aside ? `mv ${db} ${db}.before-restore && rm -f ${db}-wal ${db}-shm` : `rm -f ${db} ${db}-wal ${db}-shm`;
  const own = `chown $(stat -c %u:%g ${b.dataDir}) ${db} && chmod 0660 ${db}`;
  const inner = `${clear} && cp /in/${SNAPSHOT_DB} ${db} && ${own}`;
  if (isK8s(p)) {
    const { ns, claim } = k8s(p);
    const pod = 'meerkat-files';
    const overrides = `'{"spec":{"volumes":[{"name":"d","persistentVolumeClaim":{"claimName":"${claim}"}}],"containers":[{"name":"c","image":"busybox","command":["sleep","3600"],"volumeMounts":[{"name":"d","mountPath":"${b.dataDir}"}]}]}}'`;
    return [
      `# a helper pod on the claim ${claim} (kubectl -n ${ns} get pvc)`,
      `kubectl -n ${ns} run ${pod} --image=busybox --restart=Never --overrides=${overrides}`,
      `kubectl -n ${ns} wait --for=condition=Ready pod/${pod}`,
      `kubectl -n ${ns} exec ${pod} -- sh -c '${clear}'`,
      `kubectl -n ${ns} cp ${SNAPSHOT_DB} ${pod}:${db}`,
      `kubectl -n ${ns} exec ${pod} -- sh -c '${own}'`,
      `kubectl -n ${ns} delete pod ${pod}`,
    ];
  }
  if (p.name === 'swarm') {
    return [
      '# on the node holding the volume (docker volume ls)',
      `docker run --rm -v VOLUME:${b.dataDir} -v "$PWD":/in busybox \\`,
      `  sh -c '${inner}'`,
    ];
  }
  return [
    '# a helper on the stopped container\'s volume',
    `docker run --rm --volumes-from ${container(p)} -v "$PWD":/in busybox \\`,
    `  sh -c '${inner}'`,
  ];
}

// The restore of a backup of the same kind.
export function restore(b: BackupInfo, p: Platform): Snippet {
  if (b.dialect === 'sqlite') {
    return {
      name: 'restore.sh',
      content: [
        ...(isK8s(p) ? k8sNotes(p) : []),
        '# 1. stop meerkat',
        ...stop(p),
        '# 2. keep the current database aside, and put the snapshot in its place',
        ...place(b, p, true),
        '# 3. start meerkat, then check a route and a sign-in',
        ...start(p),
        `# the vault key is not in the snapshot: ${b.keyFromEnv ? 'MEERKAT_VAULT_KEY stays as it is' : 'the one in ' + b.keyFile + ' stays as it is'}`,
      ].join('\n'),
    };
  }
  return {
    name: 'restore.sh',
    content: [
      ...(isK8s(p) ? k8sNotes(p) : []),
      '# 1. stop meerkat, every replica',
      ...stop(p),
      '# 2. load the dump into an EMPTY database (a new one, or the old one dropped and created again)',
      `psql -v ON_ERROR_STOP=1 -f ${SNAPSHOT_SQL} "$MEERKAT_DATABASE_URL"`,
      '# 3. start meerkat as many replicas as before, then check a route and a sign-in',
      ...start(p, 'REPLICAS'),
    ].join('\n'),
  };
}

// From PostgreSQL back to one gateway on its own file: the database URL out,
// one replica, the file in the data directory before it starts.
export function switchToEmbedded(b: BackupInfo, p: Platform): Snippet[] {
  const key = keyNote(b);
  if (isK8s(p) && p.helm) {
    const { ns, release } = k8s(p);
    return [
      {
        name: 'migrate-to-embedded.sh',
        content: [
          ...k8sNotes(p),
          '# 1. one gateway, one volume: drop the database URL, one replica (the vault key unchanged)',
          `helm upgrade ${release} meerkat/meerkat -n ${ns} --reuse-values \\`,
          '  --set database.existingSecret="" --set database.url="" --set replicaCount=1',
          '# 2. stop it, put the file in its volume, start it',
          ...stop(p),
          ...place(b, p, false),
          ...start(p),
        ].join('\n'),
      },
    ];
  }
  const out: string[] = ['# 1. stop meerkat, every replica', ...stop(p), `# 2. put the file in the data directory, as ${b.dbFile}`, ...place(b, p, false)];
  if (isK8s(p)) {
    out.push('# 3. the MEERKAT_DATABASE_URL entry out of the Deployment, replicas: 1, a volume on ' + b.dataDir + ', then kubectl apply');
  } else if (p.name === 'swarm') {
    const { svc } = swarmService(p);
    out.push(
      '# 3. the database URL out, one task, on the volume',
      `docker service update --env-rm MEERKAT_DATABASE_URL --replicas 1 \\`,
      `  --mount-add type=volume,source=VOLUME,target=${b.dataDir} ${svc}`,
      '# and the same in the stack file: no MEERKAT_DATABASE_URL, replicas: 1, the volume',
    );
  } else if (p.name === 'compose') {
    out.push(`# 3. the MEERKAT_DATABASE_URL line out of ${composeFile(p)}, then`, `${compose(p)} up -d ${p.workload || 'meerkat'}`);
  } else {
    out.push('# 3. the container again, with the same docker run but without -e MEERKAT_DATABASE_URL');
  }
  out.push(key);
  return [{ name: 'migrate-to-embedded.sh', content: out.join('\n') }];
}

// ---------------------------------------------------------------------------
// The SQL a database administrator runs before the copy.

// An identifier as PostgreSQL reads it: bare when it is lower case and plain,
// quoted otherwise - a name typed with a capital or a dash is that name only
// when quoted.
function ident(name: string): string {
  return /^[a-z_][a-z0-9_$]*$/.test(name) ? name : '"' + name.replace(/"/g, '""') + '"';
}

// Exactly what the gateway's own test asks for (store/probe.go): CREATE on the
// database and on the schema it will create its tables in. A database owned by
// the role has both - on PostgreSQL 15 and later the public schema belongs to
// pg_database_owner, which is the database's owner - so two lines are all of
// it, and the grants are only for a database somebody else owns.
export function createDatabase(host: string, port: number, database: string, user: string, password: string): Snippet {
  const db = ident(database || 'meerkat');
  const role = ident(user || 'meerkat');
  const pass = (password || 'PASSWORD').replace(/'/g, "''");
  return {
    name: 'create-database.sql',
    content: [
      `-- as an administrator: psql -h ${host || 'HOST'} -p ${port || 5432} -U postgres`,
      `CREATE ROLE ${role} LOGIN PASSWORD '${pass}';`,
      `CREATE DATABASE ${db} OWNER ${role};`,
      '',
      '-- or, into a database that already exists and belongs to another role:',
      `-- \\c ${db}`,
      `-- GRANT CONNECT, CREATE ON DATABASE ${db} TO ${role};`,
      `-- GRANT USAGE, CREATE ON SCHEMA public TO ${role};`,
    ].join('\n'),
  };
}
