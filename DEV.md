# Lancer Meerkat en dev (mémo local)

Deux terminaux. Node est épinglé par `.node-version` (fnm bascule seul), Go vient de `go.mod`.

## Terminal 1 - console (:4200)

```bash
cd console
npm install   # une fois, et après tout pull qui touche console/package.json
npm start     # ng serve
```

> La console est en **anglais uniquement** : c'est un outil d'exploitation, et
> les vingt et une copies de la même SPA pesaient 120 Mo dans le binaire. Ce
> sont les **pages du plan data** qui parlent les langues de l'application
> (`internal/auth/i18n.go`).

## Terminal 2 - gateway hot-reload (air), flags en direct

```bash
MEERKAT_ADMIN_PASSWORD=test1234 air -- -addr :8082 -admin-addr :9092 -console-url http://localhost:4200
```

> **La balise `ee` est dans `.air.toml`**, pas sur la ligne de commande. Elle
> compte : sans elle on construit l'image communautaire (ni pilote d'annuaire,
> ni dispositions de page, ni configurations multiples - le code n'est pas dans
> le binaire). Comme `air` la prend de son fichier de config, `air` tapé à la
> main et `make dev` construisent le même binaire. `make dev-ce` construit la
> communautaire.
>
> **Piège corrigé le 2026-08-18** : la ligne portait `air -- -build.tags ee`.
> Or tout ce qui suit `--` va au **binaire**, pas au build : meerkat recevait un
> flag inconnu et sortait aussitôt, pendant qu'air construisait la communautaire.
> C'est ainsi qu'une installation multi-organisations a tourné en mode CE.

> `~/go/bin` est dans le PATH via `~/.bash_profile` (ajouté le 2026-07-28 -
> terminal ouvert avant cette date : `source ~/.bash_profile`).

- Les flags du binaire : `-addr`, `-admin-addr`, `-console-url`, `-data`,
  `-version` (simple ou double tiret, au choix). Les env `MEERKAT_*` n'en sont
  que les valeurs par défaut - le flag gagne.
- Le **mot de passe admin n'a pas de flag, exprès** : un mot de passe en argv est
  visible dans `ps`/l'historique -> il reste en env (`MEERKAT_ADMIN_PASSWORD`).
- Tout ce qui suit `--` est transmis par air **au binaire** (vérifié), jamais au
  build : une option de compilation posée là ne compile rien et fait sortir
  meerkat. `air` vient de `go install github.com/air-verse/air@latest` (résolu
  aussi par `make dev`, mais `make dev` ne transmet pas d'arguments -> utiliser
  `air --` directement pour les flags, ou l'équivalent env :
  `MEERKAT_ADDR=:8082 MEERKAT_ADMIN_ADDR=:9092
  MEERKAT_CONSOLE_URL=http://localhost:4200 MEERKAT_ADMIN_PASSWORD=test1234 make dev`).
- Sans hot-reload : `go run ./cmd/meerkat -addr :8082 -admin-addr :9092
  -console-url http://localhost:4200` (mêmes flags).

Puis naviguer sur **http://localhost:9092** (port admin) - login `admin` / `test1234`.
Le swagger embarqué est sur **http://localhost:9092/apidocs/** (ou rail -> API).

## Terminal 3 - le site de doc (:8765, sur toutes les interfaces)

```bash
cd docs
npm start     # génère le contenu puis ng serve
```

> Le serveur écoute sur **0.0.0.0:8765** (`docs/angular.json`, cible `serve`),
> pas sur `localhost:4200` : 8765 pour ne croiser ni la console ni un autre
> service, et toutes les interfaces pour que **la gateway puisse l'appeler**.
> Une route Meerkat vers `http://<ip-du-poste>:8765` sert alors la doc en cours
> d'écriture à travers le portail, l'authentification et les filtres - ce que
> `localhost` rend impossible dès que la gateway tourne ailleurs (conteneur,
> cluster). Vite affiche les deux adresses au démarrage (`Local` et `Network`).
>
> `allowedHosts: true` va avec : Vite refuse par défaut un `Host` qu'il ne
> connaît pas, donc sans lui une requête proxifiée - qui porte le nom de la
> gateway, pas le mien - repartirait en 403 sans que rien ne le dise.
> Vérifié le 2026-09-27 : `curl -H 'Host: docs.internal' http://192.168.1.51:8765/`
> répond 200.

## Pièges connus (vécus)

- `MEERKAT_ADMIN_PASSWORD` ne seed l'admin qu'au **premier** démarrage d'un `data/`
  vierge. Mot de passe oublié -> `rm -rf data/` (base jetable en dev) et relancer.
- **Si un bind échoue, le process sort entièrement** (rien ne répond nulle part).
  `:9090` est pris sur cette machine par une autre gateway -> toujours passer
  `MEERKAT_ADMIN_ADDR`. Un `bind :8082 in use` = une instance meerkat tourne déjà.
- `make dev` résout `air` depuis le PATH **ou** `$(go env GOPATH)/bin` - pas besoin
  d'avoir `~/go/bin` dans le PATH.

## Variantes

```bash
# Instance jetable (ports alternatifs, base dans /tmp) - n'écrase rien :
# (le répertoire de données = flag -data / env MEERKAT_DATA, défaut ./data)
go build -o bin/meerkat ./cmd/meerkat && \
MEERKAT_ADMIN_PASSWORD=test1234 ./bin/meerkat \
  -addr :18082 -admin-addr :19092 \
  -console-url http://localhost:4200 -data "$(mktemp -d)"

# Contenu de démo (routes, tenants, users) dans la base courante :
go run ./cmd/seed-demo

# Binaire autonome avec la console EMBARQUÉE (sans terminal 1) :
make ui && make build && MEERKAT_ADMIN_ADDR=:9092 MEERKAT_ADMIN_PASSWORD=test1234 ./bin/meerkat

# Le paquet OpenTelemetry servi aux pages UI (OBS-04), et sa barrière de version :
# elle sert le vrai bundle à un vrai navigateur et vérifie que le traceparent
# part vers NOS adresses et vers aucun tiers. Les paquets d'instrumentation sont
# en 0.x, où une version mineure a le droit de casser - d'où la barrière.
make telemetry

# Une trace de bout en bout, pour regarder ce que ça donne :
docker run -d --name jaeger -p 16686:16686 -p 4318:4318 jaegertracing/all-in-one:latest
# puis console > Infra > OpenTelemetry, collecteur http://127.0.0.1:4318 (le
# bouton Test le confirme), et les traces se lisent sur http://localhost:16686.
# Ici 127.0.0.1 est juste : la gateway tourne sur le poste. Dans un cluster
# c'est le nom du service - http://jaeger:4318 -, jamais une boucle locale, qui
# désigne le pod lui-même.

# Suite d'intégration Playwright :
cd e2e && npx playwright test
```
