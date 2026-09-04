// Package store is Meerkat's storage. Two databases behind one set of
// queries: a single SQLite file, pure Go (no CGO), which is the
// zero-dependency default; and PostgreSQL, which is what several gateways
// serving one installation share (STORE-03). What differs between them is in
// dialect.go and db.go, and nowhere else.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver

	"github.com/softwarity/meerkat/internal/routing"
	"github.com/softwarity/meerkat/internal/vault"
)

// Store wraps the embedded database.
type Store struct {
	db *database
	// locks holds the per-process half of the cluster locks (locks.go): one
	// mutex per name, kept for the life of the store.
	locks sync.Map
	// vaultCipher seals the vault's secret entries at rest (VAULT-01). The
	// master key comes from MEERKAT_VAULT_KEY, or a 0600 key file in dataDir.
	vaultCipher *vault.Cipher
	// dataDir is kept so a snapshot can be written next to the database and
	// the console can name the real paths in its restore procedure (STORE-05).
	dataDir string
}

// Open opens (creating if needed) the embedded database inside dataDir and
// applies migrations.
//
// The embedded one is the default and the promise: a `docker run` with nothing
// else must keep working exactly as it does. OpenAt is the other door.
func Open(dataDir string) (*Store, error) {
	return OpenAt(dataDir, "")
}

// OpenAt opens the database a URL names, falling back to the embedded one when
// the URL is empty.
//
// An external database is what several gateways serving one installation need,
// and PostgreSQL is the one option - chosen for a reason that is not storage.
// It carries LISTEN/NOTIFY, which is how one instance tells the others to
// re-read what changed, and advisory locks, which is the one exclusion the
// product needs. No Redis, no message bus.
func OpenAt(dataDir, url string) (*Store, error) {
	if strings.TrimSpace(url) != "" {
		return openPostgres(dataDir, url)
	}
	return openEmbedded(dataDir)
}

func openEmbedded(dataDir string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)",
		filepath.Join(dataDir, "meerkat.db"))
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	// SQLite serializes writers; a single connection avoids SQLITE_BUSY storms
	// while the skeleton has no connection-pool needs.
	db.SetMaxOpenConns(1)
	s := &Store{db: &database{DB: db, dialect: dialectSQLite}, dataDir: dataDir}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	key, err := vault.LoadOrCreateKey(dataDir, os.Getenv("MEERKAT_VAULT_KEY"))
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if s.vaultCipher, err = vault.NewCipher(key); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the underlying database.
func (s *Store) Close() error { return s.db.Close() }

// schemaVersion is bumped on every schema change; migrate upgrades any older
// database it opens (DEPLOY-06: upgrades without intervention).
// v3 identity (users widened, tenants, memberships, settings); v4 sessions
// carry the active tenant (TENANT-03); v5 the multi-step login flow (AUTH-05:
// sessions carry their pending step, users their must-change-password flag);
// v6 themes for the flow pages (THEME-04); v10 the second factor (MFA-01:
// users carry a TOTP secret + scratch codes); v11 the MFA policy lives on the
// user (mfa_required tri-state) + global - NOT per tenant, since the MFA step
// runs before tenant selection (AUTH-05), so a tenant override is unresolvable;
// v12 passkeys (AUTH-15): per-user WebAuthn credentials + a challenge store;
// v13 role descriptions (RBAC-01); v14 tenant descriptions; v15 business-access
// windows became per-day hour ranges (design phase: no data conversion -
// databases are recreated, the model and schema just move together);
// v16 route types API/UI + per-type options (ROUTE-02);
// v17 sign-in history (login_events: one row per completed login, pruned);
// v18 split administration (RBAC-05): infra_admin (routing plane, built-in
// pages) and app_admin (users, roles, identity settings) capabilities - root
// keeps implying both; tenant administration stays the membership type;
// v19 self-registration (AUTH-20): users carry email_verified +
// self_registered, one-shot email_tokens (confirmation, later resets);
// v20 the webauthn_challenges table became the GENERIC one-shot challenges
// store (WebAuthn ceremonies + the registration captcha);
// v21 exclusive group mode (RBAC-03): sessions carry the ACTIVE group chosen
// at login when the tenant's effective mode is SINGLE;
// v22 API tokens (AUTH-16): personal access tokens for the data-plane API,
// each capturing the tenant+group context it was created in;
// v23 tenants carry created_by (audit: who created the tenant); groups gain a
// human description (RBAC-02);
// v24 tenant ownership is DECOUPLED from membership (TENANT-02 revised) - the
// tenant carries owner_id (always set, the creator by default, transferable),
// so a tenant always has an owner even when created by root, and an owner need
// not be a member; the OWNER membership type is retired (types are ADMIN/USER);
// v25 the audit trail (audit_events): one row per administrative mutation, with
// the actor, the target, and the FIELD-LEVEL diff (before/after) - not "object
// modified" but "groupMode: MULTIPLE -> SINGLE";
// v26 API tokens carry a PLANE (data|admin): a data token authenticates on the
// data plane only, an admin (control-plane) token on the admin port only -
// each plane accepts its own scope, never the other's. Admin tokens are the
// foundation for headless management (CLI/MCP), minted by root.
// v48 accounts carry their colour scheme beside their language (THEME-05):
// both are preferences of the PERSON, so both are re-applied to the cookie at
// sign-in, on a browser that never knew them.
// v47 a route's OpenAPI spec may be DEPOSITED rather than declared upstream
// (SVC-06): route.api.spec names the source (upstream|file) and, for a file,
// the single segment the gateway serves it under; the bytes live in
// route_specs, out of the routes' api column.
// v33 issue reports (ISSUE-01/02): user-filed reports from the injected
// user-button panel - description, captured context (url, console, browser),
// an optional screenshot, statuses and team comments.
// schemaSQL is the WHOLE schema, always current: every column a build knows
// about is declared here, and an existing database catches up through
// addMissingColumns below.
//
// One spelling for both databases, which is what makes that possible - and it
// is why whole numbers are BIGINT and never INTEGER. On the embedded database
// the two are the same word; on PostgreSQL, INTEGER is 32 bits, and every
// timestamp in here is a Unix second. A session valid until 2100 already
// overflowed it, and the rest would have waited until 2038 to say so. The
// guard in schemaguard_test.go refuses the narrow spelling so the next column
// does not learn it by copying its neighbour.
const schemaSQL = `
CREATE TABLE IF NOT EXISTS routes (
  id            TEXT PRIMARY KEY,
  name          TEXT NOT NULL UNIQUE,
  ord           BIGINT NOT NULL DEFAULT 0,
  enabled       BOOLEAN NOT NULL DEFAULT TRUE,
  is_ui         BOOLEAN NOT NULL DEFAULT FALSE,
  upstream      TEXT NOT NULL,
  predicates    TEXT NOT NULL DEFAULT '[]',
  filters       TEXT NOT NULL DEFAULT '[]',
  api           TEXT NOT NULL DEFAULT '{}',
  ui            TEXT NOT NULL DEFAULT '{}',
  identity      TEXT NOT NULL DEFAULT '{}',
  locales       TEXT NOT NULL DEFAULT '{}',
  -- How long this upstream may take (v54, ROUTE-07). '{}' is the
  -- installation's defaults, which is what every route ran with when the
  -- bounds were global.
  timeouts      TEXT NOT NULL DEFAULT '{}',
  -- When to stop calling an upstream that stopped answering (v55, ROUTE-09).
  breaker       TEXT NOT NULL DEFAULT '{}',
  -- The route's unified base security (RBAC-06): who may call it at all,
  -- which per-endpoint rules then override (RBAC-07).
  access        TEXT NOT NULL DEFAULT '{}'
);

-- A spec deposited on a route (SVC-06). Kept out of the routes' api column on
-- purpose: that column is read in full by every ListRoutes - the console's
-- table, every reload of the router - and a spec is the one thing there big
-- enough to make listing names cost megabytes. The key is (route, genre) so a
-- second attachment would not need a table of its own. The file is stored AS
-- DEPOSITED; JSON is what leaves, converted on the way out.
CREATE TABLE IF NOT EXISTS route_specs (
  route_id TEXT NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
  kind     TEXT NOT NULL DEFAULT 'openapi',
  content  TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (route_id, kind)
);

CREATE TABLE IF NOT EXISTS users (
  id                   TEXT PRIMARY KEY,
  username             TEXT NOT NULL UNIQUE,
  password_hash        TEXT NOT NULL,
  root                 BOOLEAN NOT NULL DEFAULT FALSE,
  fullname             TEXT NOT NULL DEFAULT '',
  email                TEXT NOT NULL DEFAULT '',
  enabled              BOOLEAN NOT NULL DEFAULT TRUE,
  dev                  BOOLEAN NOT NULL DEFAULT FALSE,
  tester               BOOLEAN NOT NULL DEFAULT FALSE,
  tenant_creator       BOOLEAN NOT NULL DEFAULT FALSE,
  -- Split administration (RBAC-05): the routing plane and the application's
  -- identity are separate concerns with separate admins. Tenant
  -- administration is NOT a flag here - it lives in memberships (TENANT-02).
  infra_admin          BOOLEAN NOT NULL DEFAULT FALSE,
  app_admin            BOOLEAN NOT NULL DEFAULT FALSE,
  locale               TEXT NOT NULL DEFAULT '',
  -- The colour scheme the person chose: "" (follow the system), light, dark.
  -- Beside the language for the same reason - both are read by the pages the
  -- gateway serves, and both follow the PERSON rather than the machine.
  scheme               TEXT NOT NULL DEFAULT '',
  timezone             TEXT NOT NULL DEFAULT 'UTC',
  created_at           BIGINT NOT NULL DEFAULT 0,
  updated_at           BIGINT NOT NULL DEFAULT 0,
  last_connection_at   BIGINT NOT NULL DEFAULT 0,
  must_change_password BOOLEAN NOT NULL DEFAULT FALSE,
  -- When the password was last set, for the expiry rule (AUTH-10). Checked at
  -- SIGN-IN, never by a clock: expiring at three in the morning would sign
  -- nobody out, it would only refuse the next login.
  password_changed_at  BIGINT NOT NULL DEFAULT 0,
  -- The second factor (MFA-01). totp_secret is the confirmed base32 TOTP
  -- secret ('' = not enrolled); totp_pending holds a secret mid-enrolment,
  -- before the first code confirms it; totp_scratch is a JSON array of the
  -- SHA-256 hashes of single-use backup codes. These never travel on the User
  -- struct - they are read and written only through the mfa.go methods.
  totp_secret          TEXT NOT NULL DEFAULT '',
  totp_pending         TEXT NOT NULL DEFAULT '',
  totp_scratch         TEXT NOT NULL DEFAULT '[]',
  -- The per-user MFA policy (MFA-04): '' inherits the global setting,
  -- 'true'/'false' force the second factor for this user. Resolvable before
  -- tenant selection (unlike a per-tenant override).
  mfa_required         TEXT NOT NULL DEFAULT '',
  avatar               TEXT NOT NULL DEFAULT '',
  -- A DEVELOPER's public SSH key (one authorized_keys line): the credential
  -- their plugged service authenticates with (DEV-11). Self-service, /profile.
  -- A key rather than a signed certificate, so that removing it takes effect
  -- at the next connection instead of at an expiry date.
  dev_key              TEXT NOT NULL DEFAULT '',
  -- Self-registration (AUTH-20). email_verified defaults to 1: admin-created
  -- accounts answer for their address; only the /register flow creates
  -- unverified ones (and confirms them by e-mail).
  email_verified       BOOLEAN NOT NULL DEFAULT TRUE,
  self_registered      BOOLEAN NOT NULL DEFAULT FALSE
);

-- v27 the vault (VAULT-01): named entries the configuration references by
-- $name. The value column holds clear text for a "value" entry, and the sealed
-- blob (base64 nonce||ciphertext, AES-256-GCM) for a "secret" one.
CREATE TABLE IF NOT EXISTS vault_entries (
  name        TEXT NOT NULL,
  kind        TEXT NOT NULL DEFAULT 'value',
  scope       TEXT NOT NULL DEFAULT 'infra',
  value       TEXT NOT NULL DEFAULT '',
  description TEXT NOT NULL DEFAULT '',
  tags        TEXT NOT NULL DEFAULT '[]',
  created_at  BIGINT NOT NULL DEFAULT 0,
  updated_at  BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (scope, name)
);

-- Retired password hashes, so a policy can refuse the last N (AUTH-10). Only
-- hashes, never passwords, and trimmed to the ceiling a policy may ask for:
-- rows nobody will ever compare against are secrets kept for no reason.
CREATE TABLE IF NOT EXISTS password_history (
  user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  password_hash TEXT NOT NULL,
  changed_at    BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_password_history_user ON password_history(user_id, changed_at DESC);

CREATE TABLE IF NOT EXISTS sessions (
  token_hash TEXT PRIMARY KEY,
  user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at BIGINT NOT NULL,
  -- The active tenant chosen at login or on the select-tenant page
  -- (TENANT-03); '' = none chosen (no membership, or selection pending).
  tenant_id  TEXT NOT NULL DEFAULT '',
  -- The active group in SINGLE mode (RBAC-03); '' = none/cumulative.
  group_id   TEXT NOT NULL DEFAULT '',
  -- The login-flow step this session must complete before anything else
  -- (AUTH-05): 'update-password', 'totp'...; '' = flow complete.
  pending    TEXT NOT NULL DEFAULT '',
  -- The post-login destination, carried on the session instead of the URL.
  next       TEXT NOT NULL DEFAULT '',
  -- How the FIRST factor was answered (password, passkey, external). Carried
  -- so the sign-in history can name it once every step is done: the step that
  -- finishes the flow knows about the second factor, not about the first.
  method     TEXT NOT NULL DEFAULT '',
  -- The lifetime this session was issued with, in seconds: the deadline above
  -- slides, and pushing it needs the value the login resolved.
  ttl        BIGINT NOT NULL DEFAULT 0,
  plane      TEXT NOT NULL DEFAULT 'data'
);
CREATE INDEX IF NOT EXISTS sessions_expires ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS tenants (
  id              TEXT PRIMARY KEY,
  name            TEXT NOT NULL UNIQUE,
  description     TEXT NOT NULL DEFAULT '',
  enabled         BOOLEAN NOT NULL DEFAULT TRUE,
  business_access TEXT NOT NULL DEFAULT '{"inherited":true}',
  session_ttl     TEXT NOT NULL DEFAULT '',
  -- '' = inherit the gateway-wide group mode; MULTIPLE/SINGLE = forced here.
  group_mode      TEXT NOT NULL DEFAULT '',
  -- Who created the tenant (audit) - set once, never changed.
  created_by      TEXT NOT NULL DEFAULT '',
  -- The tenant's owner (TENANT-02): always set (the creator by default),
  -- transferable, and INDEPENDENT of membership - an owner need not be a
  -- member. This is why there is no OWNER membership type.
  owner_id        TEXT NOT NULL DEFAULT '',
  created_at      BIGINT NOT NULL DEFAULT 0,
  updated_at      BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS memberships (
  user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  tenant_id       TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  type            TEXT NOT NULL DEFAULT 'USER',
  -- Who placed this person here (v32): '' an administrator, 'rule' a group
  -- rule. A synchronisation only removes what it added itself.
  source          TEXT NOT NULL DEFAULT '',
  enabled         BOOLEAN NOT NULL DEFAULT TRUE,
  business_access TEXT NOT NULL DEFAULT '{"inherited":true}',
  session_ttl     TEXT NOT NULL DEFAULT '',
  created_at      BIGINT NOT NULL DEFAULT 0,
  updated_at      BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (user_id, tenant_id)
);
CREATE INDEX IF NOT EXISTS memberships_tenant ON memberships(tenant_id);

CREATE TABLE IF NOT EXISTS settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS themes (
  id         TEXT PRIMARY KEY,
  name       TEXT NOT NULL UNIQUE,
  active     BOOLEAN NOT NULL DEFAULT FALSE,
  flat       BOOLEAN NOT NULL DEFAULT FALSE,
  dark       TEXT NOT NULL DEFAULT '{}',
  light      TEXT NOT NULL DEFAULT '{}',
  created_at BIGINT NOT NULL DEFAULT 0,
  updated_at BIGINT NOT NULL DEFAULT 0
);

-- RBAC (v8): a GLOBAL role catalogue (hierarchical), per-tenant groups bundling
-- roles, and per-tenant member↔group assignments.
CREATE TABLE IF NOT EXISTS roles (
  id          TEXT PRIMARY KEY,
  name        TEXT NOT NULL UNIQUE,
  description TEXT NOT NULL DEFAULT '',
  parent_id   TEXT REFERENCES roles(id) ON DELETE SET NULL,
  tags        TEXT NOT NULL DEFAULT '[]',
  system      BOOLEAN NOT NULL DEFAULT FALSE,
  created_at  BIGINT NOT NULL DEFAULT 0,
  updated_at  BIGINT NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS groups (
  id          TEXT PRIMARY KEY,
  tenant_id   TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name        TEXT NOT NULL,
  -- A human description (RBAC-02), editable from the matrix's group menu and
  -- shown as the label the roles hang under.
  description TEXT NOT NULL DEFAULT '',
  created_at BIGINT NOT NULL DEFAULT 0,
  updated_at BIGINT NOT NULL DEFAULT 0,
  UNIQUE (tenant_id, name)
);
CREATE INDEX IF NOT EXISTS groups_tenant ON groups(tenant_id);

CREATE TABLE IF NOT EXISTS group_roles (
  group_id TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
  role_id  TEXT NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
  PRIMARY KEY (group_id, role_id)
);

-- source (v32) says who put this row here: '' placed by an administrator,
-- 'rule' derived from a group rule. A synchronisation only ever adds or
-- removes its OWN rows, so it can never undo a hand-made decision.
CREATE TABLE IF NOT EXISTS member_groups (
  tenant_id TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  user_id   TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  group_id  TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
  source    TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (tenant_id, user_id, group_id)
);
CREATE INDEX IF NOT EXISTS member_groups_member ON member_groups(tenant_id, user_id);

-- Group rules (v32, RBAC-10): how what an authority SAYS becomes membership
-- and groups HERE. A rule is declared IN a tenant and can only grant what
-- belongs to that tenant, which is what keeps an authority (infra) from
-- deciding what anyone may do (application).
--
-- external is the authority's own group name, verbatim. EMPTY means "anyone
-- who signs in through this authority", which is how one gives an
-- organisation its own directory rather than one directory carrying per-org
-- groups. group_id NULL grants the membership alone, no group.
CREATE TABLE IF NOT EXISTS group_rules (
  id          TEXT PRIMARY KEY,
  tenant_id   TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  provider_id TEXT NOT NULL DEFAULT '',
  external    TEXT NOT NULL DEFAULT '',
  group_id    TEXT REFERENCES groups(id) ON DELETE CASCADE,
  created_at  BIGINT NOT NULL DEFAULT 0,
  UNIQUE (tenant_id, provider_id, external, group_id)
);
CREATE INDEX IF NOT EXISTS group_rules_tenant ON group_rules(tenant_id);

-- Trusted browsers (v10, MFA-03): after a successful second factor a user may
-- mark the browser as trusted, skipping the TOTP challenge until expiry. Only
-- the token HASH is stored; deleting a row revokes the trust immediately.
CREATE TABLE IF NOT EXISTS trusted_browsers (
  id         TEXT PRIMARY KEY,
  user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL UNIQUE,
  label      TEXT NOT NULL DEFAULT '',
  created_at BIGINT NOT NULL DEFAULT 0,
  expires_at BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS trusted_browsers_user ON trusted_browsers(user_id);

-- Passkeys (v12, AUTH-15): per-user WebAuthn credentials. data is the JSON of
-- the go-webauthn Credential (opaque to the store); credential_id is the
-- base64url raw ID, used to look a credential up on login.
CREATE TABLE IF NOT EXISTS webauthn_credentials (
  id            TEXT PRIMARY KEY,
  user_id       TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  credential_id TEXT NOT NULL UNIQUE,
  data          TEXT NOT NULL,
  label         TEXT NOT NULL DEFAULT '',
  created_at    BIGINT NOT NULL DEFAULT 0,
  last_used_at  BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS webauthn_credentials_user ON webauthn_credentials(user_id);

-- Generic short-lived one-shot challenges (v20): WebAuthn ceremony state
-- between begin and finish, and the registration captcha's expected code.
-- Consuming deletes the row - nothing here is ever replayable.
CREATE TABLE IF NOT EXISTS challenges (
  id         TEXT PRIMARY KEY,
  data       TEXT NOT NULL,
  expires_at BIGINT NOT NULL
);
DROP TABLE IF EXISTS webauthn_challenges;

-- Sign-in history (v17): one row per COMPLETED login; the profile lists them.
-- browser_hash is the hash of the durable MEERKAT_BROWSER token ("this
-- browser" badge); country comes from a CDN/LB geo header when present.
-- Pruned to a fixed depth per user on every insert.
CREATE TABLE IF NOT EXISTS login_events (
  id           TEXT PRIMARY KEY,
  user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  method       TEXT NOT NULL,
  label        TEXT NOT NULL DEFAULT '',
  ip           TEXT NOT NULL DEFAULT '',
  country      TEXT NOT NULL DEFAULT '',
  browser_hash TEXT NOT NULL DEFAULT '',
  at           BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS login_events_user ON login_events(user_id, at);

-- One-shot e-mail tokens (v19, AUTH-20): address confirmation now, password
-- resets later (purpose). Only the HASH is stored; consuming deletes the row.
CREATE TABLE IF NOT EXISTS email_tokens (
  token_hash TEXT PRIMARY KEY,
  user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  purpose    TEXT NOT NULL,
  expires_at BIGINT NOT NULL
);

-- External authentication (v30, AUTH-19): one row per configured authority.
-- Config is kind-specific JSON and may hold $name vault references, so a
-- client secret or a bind password never sits here in clear.
CREATE TABLE IF NOT EXISTS auth_providers (
  id            TEXT PRIMARY KEY,
  kind          TEXT NOT NULL,                    -- oidc | ldap | saml
  name          TEXT NOT NULL,                    -- what the login page shows
  enabled       BOOLEAN NOT NULL DEFAULT FALSE,
  ord           BIGINT NOT NULL DEFAULT 0,
  config        TEXT NOT NULL DEFAULT '{}',
  -- Per-provider policies: '' inherits the application setting. An authority
  -- that already enforces its own second factor has no use for ours.
  mfa_required  TEXT NOT NULL DEFAULT '',         -- '' | yes | no
  passkeys      TEXT NOT NULL DEFAULT '',         -- '' | yes | no
  -- A first sign-in creates a PENDING local account (the self-registration
  -- path): the admin then places it in a tenant and grants roles. Off means
  -- only already-linked accounts may come in.
  -- Tri-state: '' follows the application default, 'yes'/'no' decide here.
  auto_create   TEXT NOT NULL DEFAULT '',
  -- The anti-robot check, LOCAL authority only: it guards a public form. An
  -- arrival through a directory or an identity provider already proved itself
  -- over there.
  captcha       BOOLEAN NOT NULL DEFAULT TRUE,
  created_at    BIGINT NOT NULL DEFAULT 0,
  updated_at    BIGINT NOT NULL DEFAULT 0
);

-- The link between a local account and what an authority calls that person.
-- Keyed by (provider, external id) because the authority's id is the only
-- stable handle: a username or an address can change upstream.
-- groups and last_seen_at are refreshed on EVERY sign-in (v31): what the
-- authority says about someone is a fact of the last sign-in, not of the day
-- the link was made, and an admin has to see the real group names before any
-- mapping can be configured against them.
CREATE TABLE IF NOT EXISTS user_identities (
  provider_id  TEXT NOT NULL REFERENCES auth_providers(id) ON DELETE CASCADE,
  external_id  TEXT NOT NULL,
  user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  groups       TEXT NOT NULL DEFAULT '',
  created_at   BIGINT NOT NULL DEFAULT 0,
  last_seen_at BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (provider_id, external_id)
);
CREATE INDEX IF NOT EXISTS user_identities_user ON user_identities(user_id);

-- Personal API tokens (v22, AUTH-16): authenticate the owner on the data
-- plane's API routes. Each captures the tenant+group context of the session
-- it was minted in; only the token HASH is stored (shown once at creation).
-- Deleting the user cascades; disabling the user stops them (checked live).
CREATE TABLE IF NOT EXISTS api_tokens (
  id           TEXT PRIMARY KEY,
  user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name         TEXT NOT NULL DEFAULT '',
  token_hash   TEXT NOT NULL UNIQUE,
  prefix       TEXT NOT NULL DEFAULT '',
  tenant_id    TEXT NOT NULL DEFAULT '',
  group_id     TEXT NOT NULL DEFAULT '',
  plane        TEXT NOT NULL DEFAULT 'data',
  -- The token's perimeter (v49, MCP-02): 'full' or 'readonly'. Checked on the
  -- control plane, which is where a token can rewrite a gateway.
  scope        TEXT NOT NULL DEFAULT 'full',
  -- The second axis of the perimeter (v50, MCP-02): '' the whole control
  -- plane, 'gateway' the routing plane, 'app' the application's identity. It
  -- MASKS the owner's capabilities, so a token is at most its owner.
  domain       TEXT NOT NULL DEFAULT '',
  -- Where the token may be used from (v50): comma-separated CIDR ranges,
  -- empty for anywhere. Judged on the TCP peer, never on a header.
  from_cidrs   TEXT NOT NULL DEFAULT '',
  -- The registered agent this token belongs to (v51, MCP-07), empty for one
  -- minted by hand. It is what turns the token list into a list of CONNECTED
  -- AGENTS, each revocable where it stands.
  client_id    TEXT NOT NULL DEFAULT '',
  enabled      BOOLEAN NOT NULL DEFAULT TRUE,
  created_at   BIGINT NOT NULL DEFAULT 0,
  expires_at   BIGINT NOT NULL DEFAULT 0,
  last_used_at BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS api_tokens_user ON api_tokens(user_id);

-- The OAuth flow that connects an agent without anybody copying a secret
-- (v51, MCP-07). Three short-lived things and one registration: clients are
-- public (no secret - a CLI cannot keep one), codes live five minutes and are
-- spent once, refresh tokens rotate.
CREATE TABLE IF NOT EXISTS oauth_clients (
  id            TEXT PRIMARY KEY,
  name          TEXT NOT NULL DEFAULT '',
  redirect_uris TEXT NOT NULL DEFAULT '',
  created_at    BIGINT NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS oauth_codes (
  code_hash    TEXT PRIMARY KEY,
  client_id    TEXT NOT NULL DEFAULT '',
  user_id      TEXT NOT NULL DEFAULT '',
  redirect_uri TEXT NOT NULL DEFAULT '',
  challenge    TEXT NOT NULL DEFAULT '',
  scope        TEXT NOT NULL DEFAULT '',
  domain       TEXT NOT NULL DEFAULT '',
  resource     TEXT NOT NULL DEFAULT '',
  expires_at   BIGINT NOT NULL DEFAULT 0
);
-- The refresh token points at the api_tokens row it renews: the access token
-- IS an ordinary control-plane token, so the perimeter, the audit and the
-- revocation all work on it without a second machinery.
CREATE TABLE IF NOT EXISTS oauth_refresh (
  token_hash TEXT PRIMARY KEY,
  token_id   TEXT NOT NULL REFERENCES api_tokens(id) ON DELETE CASCADE,
  expires_at BIGINT NOT NULL DEFAULT 0
);

-- Audit trail (v25): one row per administrative mutation. actor_id/target_id
-- are NOT foreign keys - the trail must outlive a deleted actor or target
-- (knowing who did what matters most once they are gone). target_name is the
-- human label captured at write time; changes is the JSON field-level diff
-- (before/after); tenant_id scopes tenant-admin visibility ("" = global).
CREATE TABLE IF NOT EXISTS audit_events (
  id          TEXT PRIMARY KEY,
  at          BIGINT NOT NULL,
  actor_id    TEXT NOT NULL DEFAULT '',
  action      TEXT NOT NULL,
  target      TEXT NOT NULL DEFAULT '',
  target_id   TEXT NOT NULL DEFAULT '',
  target_name TEXT NOT NULL DEFAULT '',
  -- The NAME of the control-plane token that acted (v49, MCP-03), empty when a
  -- human did it from the console. "root" is who, "claude-desktop" is what.
  actor_token TEXT NOT NULL DEFAULT '',
  tenant_id   TEXT NOT NULL DEFAULT '',
  changes     TEXT NOT NULL DEFAULT '[]',
  detail      TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS audit_events_at ON audit_events(at);
CREATE INDEX IF NOT EXISTS audit_events_tenant ON audit_events(tenant_id, at);
CREATE INDEX IF NOT EXISTS audit_events_actor ON audit_events(actor_id, at);

-- Issue reports (v33, ISSUE-01/02): one row per report filed from the injected
-- user-button panel. reporter_id is NOT a foreign key - the report must
-- outlive its author (reporter_name is captured at write time). tenant_id is
-- the reporter's CURRENT tenant ("" = none: visible to root/infra only). The
-- screenshot data URI lives in its own column and is only ever read on demand
-- (lists never carry it). console_log and comments are JSON arrays.
CREATE TABLE IF NOT EXISTS issues (
  id            TEXT PRIMARY KEY,
  created_at    BIGINT NOT NULL,
  updated_at    BIGINT NOT NULL,
  reporter_id   TEXT NOT NULL DEFAULT '',
  reporter_name TEXT NOT NULL DEFAULT '',
  tenant_id     TEXT NOT NULL DEFAULT '',
  status        TEXT NOT NULL DEFAULT 'open',
  description   TEXT NOT NULL,
  url           TEXT NOT NULL DEFAULT '',
  user_agent    TEXT NOT NULL DEFAULT '',
  viewport      TEXT NOT NULL DEFAULT '',
  screen        TEXT NOT NULL DEFAULT '',
  dpr           REAL NOT NULL DEFAULT 0,
  language      TEXT NOT NULL DEFAULT '',
  console_log   TEXT NOT NULL DEFAULT '[]',
  comments      TEXT NOT NULL DEFAULT '[]',
  screenshot    TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS issues_at ON issues(created_at);
CREATE INDEX IF NOT EXISTS issues_tenant ON issues(tenant_id, created_at);
CREATE INDEX IF NOT EXISTS issues_status ON issues(status, created_at);

-- Named configurations (CFG-01/02). The document is opaque here on purpose:
-- its shape belongs to internal/config, which imports this package - holding
-- the bytes rather than the structure is what keeps the dependency one-way.
--
-- The partial unique index is the "one active" rule, written where it cannot
-- be forgotten: two rows claiming to be the running configuration would make
-- every later question (what is running? what does a rollback go back to?)
-- unanswerable, and no amount of care in the handlers proves they never do.
CREATE TABLE IF NOT EXISTS configurations (
  id          TEXT PRIMARY KEY,
  name        TEXT NOT NULL UNIQUE,
  description TEXT NOT NULL DEFAULT '',
  document    TEXT NOT NULL DEFAULT '',
  -- The document's fingerprint, so "is what is running still this one?" is a
  -- comparison rather than a claim. Stored rather than computed on read: the
  -- list deliberately leaves the documents behind, and loading ten of them to
  -- hash them would undo exactly what that saves.
  digest      TEXT NOT NULL DEFAULT '',
  active      BOOLEAN NOT NULL DEFAULT FALSE,
  created_at  BIGINT NOT NULL DEFAULT 0,
  updated_at  BIGINT NOT NULL DEFAULT 0
);
-- One active configuration at a time, enforced by the index rather than by a
-- transaction everyone has to remember.
CREATE UNIQUE INDEX IF NOT EXISTS configurations_active ON configurations(active) WHERE active = TRUE;

-- Restore points (CFG-06): the gateway's own tape, written whenever a change
-- moves the configuration's fingerprint - never on purpose, never named.
--
-- A different object from a saved configuration, and keeping them apart is what
-- makes both simple: the shelf is intentional and named ("the Acme setup"), the
-- tape is automatic and timestamped ("what it looked like at 14:32"). One is
-- for switching, the other for going back.
--
-- The document is stored WITHOUT its pictures (see config.WithoutImages), which
-- is what makes a tape affordable: a few kilobytes a point instead of a
-- megabyte of base64 repeated two hundred times.
CREATE TABLE IF NOT EXISTS config_points (
  id         TEXT PRIMARY KEY,
  at         BIGINT NOT NULL,
  actor_id   TEXT NOT NULL DEFAULT '',
  label      TEXT NOT NULL DEFAULT '',
  digest     TEXT NOT NULL DEFAULT '',
  document   TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS config_points_at ON config_points(at);

-- TLS material (SSL-01), v43. One row per certificate, whichever of the four
-- doors it came through: imported, generated self-signed, or generated as a
-- signing request and adopted once an authority signed it. The material an
-- ACME authority fetches is NOT here - it lives in acme_cache, because
-- autocert owns its lifecycle and a second copy would be a second truth.
--
-- key_sealed is the private key under the vault's master key (VAULT-01). It is
-- the one field in this database that must never be readable from a stolen
-- file, and the one Archway left in clear beside its keystore.
--
-- csr_pem holds a request waiting for its answer: the row exists, serves
-- nothing, and shows in the console as pending - which is the state an offline
-- signing procedure spends its days in.
CREATE TABLE IF NOT EXISTS certificates (
  id           TEXT PRIMARY KEY,
  -- The HOST this certificate answers for, and the plane it belongs to. A
  -- certificate is not floating material to be matched against a list of
  -- names: it IS the name's certificate. The console has one, the application
  -- has one per host it serves, and the same material used by both is simply
  -- added twice - two entries, two keys, no shared object to reason about.
  plane        TEXT NOT NULL DEFAULT 'app',
  host         TEXT NOT NULL DEFAULT '',
  source       TEXT NOT NULL DEFAULT 'import',
  cert_pem     TEXT NOT NULL DEFAULT '',
  key_sealed   TEXT NOT NULL DEFAULT '',
  csr_pem      TEXT NOT NULL DEFAULT '',
  subject      TEXT NOT NULL DEFAULT '',
  issuer       TEXT NOT NULL DEFAULT '',
  serial       TEXT NOT NULL DEFAULT '',
  algo         TEXT NOT NULL DEFAULT '',
  key_type     TEXT NOT NULL DEFAULT '',
  dns_names    TEXT NOT NULL DEFAULT '[]',
  ip_addresses TEXT NOT NULL DEFAULT '[]',
  chain        BIGINT NOT NULL DEFAULT 0,
  self_signed  BOOLEAN NOT NULL DEFAULT FALSE,
  not_before   BIGINT NOT NULL DEFAULT 0,
  not_after    BIGINT NOT NULL DEFAULT 0,
  created_at   BIGINT NOT NULL DEFAULT 0,
  updated_at   BIGINT NOT NULL DEFAULT 0
);

-- The ACME account key and the certificates an authority issued (SSL-05).
-- This is autocert's own cache, kept in the store rather than on disk for one
-- reason: it MUST be shared. Two instances with two caches register twice and
-- request twice, and the second one meets the authority's rate limit on behalf
-- of the first. Sealed, because the account key signs on this gateway's behalf.
CREATE TABLE IF NOT EXISTS acme_cache (
  key        TEXT PRIMARY KEY,
  value      TEXT NOT NULL DEFAULT '',
  updated_at BIGINT NOT NULL DEFAULT 0
);

-- What each node has to re-read, and when it last moved (v52, STORE-03). One row
-- per topic, a counter that only goes up.
--
-- The NOTIFY that goes out with a write is a HINT, not the truth: it is lost
-- if nobody is listening at that instant, and a listening connection can drop
-- without saying so. This table is the truth - a node that reconnects, or
-- simply wakes up on its slow timer, compares the versions it acted on with
-- the ones in here and reloads what moved. That is also what makes a change
-- made straight in SQL, by a migration or by hand, eventually arrive.
-- Failed sign-in attempts, one row each (v53, AUTH-11). Shared, so a limit of
-- five is five for the installation rather than five per gateway, and a
-- restart no longer forgives whoever was being throttled.
--
-- Milliseconds rather than seconds: the window is read as "how many since a
-- moment", and two attempts inside the same second are two.
CREATE TABLE IF NOT EXISTS login_attempts (
  key TEXT NOT NULL,
  at  BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_login_attempts_key ON login_attempts (key, at);

CREATE TABLE IF NOT EXISTS change_marks (
  topic   TEXT PRIMARY KEY,
  version BIGINT NOT NULL DEFAULT 0,
  at      BIGINT NOT NULL DEFAULT 0
);`

const schemaVersion = 55

func (s *Store) migrate() error {
	v, err := s.db.schemaVersion()
	if err != nil {
		return err
	}
	// Before anything is written to it. A database a LATER build wrote is one
	// this one cannot maintain, and finding that out after the first INSERT is
	// finding it out too late.
	if err := checkNotNewer(v, schemaVersion); err != nil {
		return err
	}
	// A database that has never been opened: v is zero, and the DDL below is
	// already the end state. The ledger says how OLDER databases reach it, and
	// there is no older database here - running its steps would ask a fresh
	// schema to undo changes it never had.
	fresh := v == 0
	if _, err := s.db.Exec(schemaSQL); err != nil {
		return fmt.Errorf("store: migrate: %w", err)
	}
	// The columns a build added since this database was created: CREATE TABLE
	// IF NOT EXISTS is silent on an existing table, so they are added here.
	if err := s.addMissingColumns(); err != nil {
		return err
	}
	// The steps a CREATE cannot express, in order, each once. Skipped on a
	// fresh database for the reason above.
	if !fresh {
		if err := s.runMigrations(migrations, v); err != nil {
			return err
		}
	}
	// Two leftovers from the design phase, run unconditionally on every start
	// because there was nowhere to record that they had already run. They stay
	// until the first release, whose baseline makes them unreachable: no
	// database at or above it can hold either shape.
	//
	// A certificate now belongs to a host (v44). Rows written before that have
	// an empty one and cannot be expressed in the model at all - they belong to
	// no name, so no screen can show them and no handshake can pick them. They
	// are dropped rather than left to render as a nameless ghost, and the count
	// is logged: silent deletion and silent nonsense are both worse than a line
	// saying what happened.
	if res, err := s.db.Exec(`DELETE FROM certificates WHERE host = ''`); err == nil {
		if n, _ := res.RowsAffected(); n > 0 {
			slog.Info("dropped certificates that predate the host they answer for", "count", n)
		}
	}
	// The developer credential became a KEY rather than a certificate (v46).
	// Best effort: on a database that predates it the old column is dead
	// weight, and leaving a dev_cert nobody reads is how a schema starts
	// describing something the product no longer does.
	_, _ = s.db.Exec(`ALTER TABLE users DROP COLUMN dev_cert`)
	if err := s.seedDefaultSettings(); err != nil {
		return err
	}
	if err := s.seedThemes(); err != nil {
		return err
	}
	if err := s.seedDefaultTenant(); err != nil {
		return err
	}
	if err := s.seedLocalProvider(); err != nil {
		return err
	}
	return s.db.setSchemaVersion(schemaVersion)
}

// The token planes (must match session.DataPlane/AdminPlane, same string
// values): a data token authenticates on the data plane, an admin
// (control-plane) token on the admin port, and the resolver only ever accepts
// a token on its own plane.
const (
	PlaneData  = "data"
	PlaneAdmin = "admin"
)

// Route is a routing rule: declarative predicates decide the match,
// declarative filters transform request and response, the upstream receives
// the traffic (routing.Spec is the single shape shared with exports, the
// admin API and the console).

// RouteAPI holds the API-route options: the OpenAPI spec this route exposes,
// and the endpoint-level security (RBAC-07) posed on that spec's operations,
// which secures an upstream that does not enforce access itself.
type RouteAPI struct {
	Spec     *RouteSpec        `json:"spec,omitempty"`
	Security *EndpointSecurity `json:"security,omitempty"`
}

// Access levels (RBAC-06), the BELONGING axis of an access rule. They are
// ordered, and each one is the previous plus one requirement:
//
//	""          delegated - the gateway poses no condition and lets the request
//	            through, session or not. There was a "public" level beside this
//	            one for a while; it did exactly the same thing, so it is gone.
//	            "Delegated" is the honest name: the upstream is ALWAYS free to
//	            apply its own rules on top, at every level below as well. What
//	            Meerkat gates, it gates in ADDITION to the service, never
//	            instead of it.
//	auth        a valid session. Says who it is, nothing more - a person whose
//	            account is confirmed but who belongs to no organisation yet
//	            (AUTH-20's waiting room) passes here, and that is deliberate:
//	            it is how one serves the page explaining how to ask for access.
//	tenant      a session with an ACTIVE organisation. This is what the waiting
//	            room does not pass, and what someone who belongs to several
//	            organisations must choose before passing.
//	tenants     the active organisation is one of AccessLevelTenants' names.
//
// The reason "tenant" exists at all: roles are granted by groups, groups belong
// to an organisation, so without one a caller has NO role - they used to reach
// a route gated on "authenticated" carrying an identity stripped of everything
// that decides anything.
const (
	AccessDelegated = ""
	AccessAuth      = "auth"
	AccessTenant    = "tenant"
	AccessTenants   = "tenants"
	// deny: nobody, except the users named on the rule. The closed end of the
	// scale, and what makes Users mean something - an exception needs a
	// condition to be an exception TO.
	AccessDeny = "deny"
)

// Access is a unified access rule (RBAC-06/07), used both as a route-wide
// default and as a per-endpoint override.
//
// TWO INDEPENDENT AXES, and they are ANDed:
//
//   - Level (+ Tenants when it is "tenants") says what belonging is required;
//   - Roles filters on what the caller HOLDS in the active organisation.
//
// They cross on purpose. The role catalogue is global while groups are
// per-organisation, so `roles: [admin]` alone means "an admin of ANY
// organisation" - a cross-org administration UI - while `tenants: [acme] +
// roles: [admin]` means "an admin OF ACME". Neither could be expressed before.
//
// Users is the exception, not a level: whoever is named passes whatever the
// level requires (a service account, a support login, a UI dedicated to one
// person). It is ORed with the rest.
type Access struct {
	Level   string   `json:"level,omitempty"`
	Tenants []string `json:"tenants,omitempty"` // tenant ids, when Level is "tenants"
	Roles   []string `json:"roles,omitempty"`   // allowed effective role names
	Users   []string `json:"users,omitempty"`   // always allowed, whatever the level
}

// Caller is what a rule is evaluated against: the resolved session, or the
// zero value for an anonymous request.
type Caller struct {
	Authenticated bool
	Username      string
	TenantID      string   // the ACTIVE organisation, "" when none is chosen
	Roles         []string // effective role names IN that organisation
	// Memberships are the organisations this caller could switch to. They do
	// not grant anything by themselves; they are what tells a refusal that is
	// final ("you are not in Acme") from one a tenant switch would fix.
	Memberships []string
}

// Empty reports whether NO gateway rule is set (the request is then delegated
// to the upstream, not made public).
func (a Access) Empty() bool {
	// Named users are an EXCEPTION, and an exception needs a condition to be an
	// exception to. Delegated with no role asked poses no condition, so the
	// list has nothing to let anyone past and the rule conditions nothing -
	// which is what empty means. "Only these people" is deny plus the names.
	//
	// Counting such a rule as posed cost more than a mark in the console: the
	// route required a session to be SELECTED, so an anonymous caller was
	// passed over and served by whatever matched next - the catch-all, in
	// practice, silently and with nothing on screen to say so.
	return a.Level == AccessDelegated && len(a.Roles) == 0
}

// AccessLevels are the levels a rule may take, in the order a form offers
// them: from the one that poses no condition to the one that refuses everyone.
var AccessLevels = []string{AccessDelegated, AccessAuth, AccessTenant, AccessTenants, AccessDeny}

// SanitizeAccess drops what a rule cannot use, so the stored rule says exactly
// what the engine reads - and the screens showing it stop describing a
// restriction nobody applies.
//
// The one that bit: NAMED USERS on a rule that poses no condition. Users are
// an exception, and an exception needs something to be an exception TO (see
// Empty), so the engine ignores such a name entirely - while the routes table
// lit a badge saying "Users: alice" on a route delegated to its service. Worse
// than the wrong badge: raise the level later and that forgotten name comes
// silently back to life as a way in.
//
// Cleaning happens on SAVE rather than on read, because a value nobody can see
// and nobody can remove is the actual defect; a rule saved once is then true
// on disk, and an import or an agent cannot reintroduce one.
func SanitizeAccess(a *Access) error {
	a.Level = strings.ToLower(strings.TrimSpace(a.Level))
	if !slices.Contains(AccessLevels, a.Level) {
		return fmt.Errorf("access level %q: allowed are %s (or none, to delegate to the service)",
			a.Level, strings.Join(AccessLevels[1:], ", "))
	}
	a.Roles = nonEmpty(a.Roles)
	a.Users = nonEmpty(a.Users)
	a.Tenants = nonEmpty(a.Tenants)
	// A tenant list belongs to the level that reads it, and nowhere else.
	if a.Level != AccessTenants {
		a.Tenants = nil
	}
	if a.Empty() {
		a.Users = nil
	}
	return nil
}

// nonEmpty drops blank entries and keeps nil nil: a rule with an empty list
// and a rule with no list are the same rule, and they should serialize alike.
func nonEmpty(in []string) []string {
	var out []string
	for _, v := range in {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// Grants reports whether the gateway lets a caller through.
func (a Access) Grants(c Caller) bool {
	if a.Empty() {
		return true
	}
	// The exception is read FIRST: it is what a named user passes on, up to and
	// including a route closed to everyone else.
	if c.Authenticated && slices.Contains(a.Users, c.Username) {
		return true
	}
	if a.Level == AccessDeny {
		return false
	}
	if !c.Authenticated {
		return false
	}
	switch a.Level {
	case AccessTenant:
		if c.TenantID == "" {
			return false
		}
	case AccessTenants:
		if c.TenantID == "" || !slices.Contains(a.Tenants, c.TenantID) {
			return false
		}
	}
	// Roles are the second axis: named, the caller must hold one of them IN the
	// active organisation (which is why an empty TenantID holds no role).
	if len(a.Roles) > 0 {
		for _, r := range a.Roles {
			if slices.Contains(c.Roles, r) {
				return true
			}
		}
		return false
	}
	return true
}

// Switchable reports whether a refusal would be lifted by choosing another
// organisation: the caller belongs to one the rule accepts but is active
// somewhere else. A UI route then offers the choice instead of a dead end.
// The tenants worth offering are returned; empty means the refusal is final.
func (a Access) Switchable(c Caller) []string {
	if !c.Authenticated || len(c.Memberships) == 0 {
		return nil
	}
	switch a.Level {
	case AccessTenant:
		if c.TenantID == "" {
			return c.Memberships
		}
	case AccessTenants:
		var offer []string
		for _, id := range c.Memberships {
			if id != c.TenantID && slices.Contains(a.Tenants, id) {
				offer = append(offer, id)
			}
		}
		return offer
	}
	return nil
}

// EndpointSecurity secures the operations of a route's OpenAPI spec (RBAC-07),
// the way to gate an upstream one does not control without touching its code.
type EndpointSecurity struct {
	// Endpoints override the route's base Access (Route.Access) for specific
	// operations (method+path, OpenAPI coordinates, {var} templating). First
	// match wins, in list order. The whole-route default is the route's own
	// Access, not a field here.
	Endpoints []EndpointPolicy `json:"endpoints,omitempty"`
}

// EndpointPolicy is one operation's access override. Method is an upper-case
// verb or "*" (any verb on the path). The access fields are embedded.
type EndpointPolicy struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Access
}

// Validate checks an endpoint-security block: every override path must compile
// and every method be a real verb (or *). Methods are upper-cased in place.
func (s *EndpointSecurity) Validate() error {
	if s == nil {
		return nil
	}
	for i := range s.Endpoints {
		e := &s.Endpoints[i]
		e.Method = strings.ToUpper(strings.TrimSpace(e.Method))
		if e.Method != "*" && !validHTTPMethod(e.Method) {
			return fmt.Errorf("endpoint %d (%s %s): invalid method %q", i, e.Method, e.Path, e.Method)
		}
		if _, err := routing.CompilePath(e.Path); err != nil {
			return fmt.Errorf("endpoint %d: %w", i, err)
		}
	}
	return nil
}

// validHTTPMethod reports whether m is one of the OpenAPI-supported verbs.
func validHTTPMethod(m string) bool {
	switch m {
	case "GET", "PUT", "POST", "DELETE", "OPTIONS", "HEAD", "PATCH", "TRACE":
		return true
	default:
		return false
	}
}

// UserButton configures the <meerkat-user-button> web component injected into
// a UI route's pages: pixel height, whether the username shows beside the
// avatar, and a two-word position whose FIRST word is the anchored edge - it
// decides where the menu opens (top-left drops the menu downward, left-top
// opens it to the right).
type UserButton struct {
	Enabled  bool   `json:"enabled"`
	Height   int    `json:"height,omitempty"`   // px, default 24
	Position string `json:"position,omitempty"` // default top-right
	Shape    string `json:"shape,omitempty"`    // round (default) | square
	Name     string `json:"name,omitempty"`     // "" (hidden) | before | after
	// Gaps from the anchored corner's edges, px (default 12 each): PadY from
	// the top/bottom edge, PadX from the left/right one.
	PadX int `json:"padX,omitempty"`
	PadY int `json:"padY,omitempty"`
	// Show it even when the page is framed. A frame means a portal, and a
	// portal carries its own chrome: by default the button stays out of it,
	// because signing out from inside would end a session the portal still
	// shows as open. Set for a frame that carries no identity of its own.
	InFrame bool `json:"inFrame,omitempty"`
}

// UserButtonPositions are the four corners; the menu opens away from the
// anchored edge (top-* drops it downward, bottom-* opens it upward).
var UserButtonPositions = []string{
	"top-left", "top-right", "bottom-left", "bottom-right",
}

// SchemeConfig describes whether and HOW the target application consumes a
// color scheme: the user button always reflects the choice on the CSS
// color-scheme; on top of it, an attribute (name + light/dark values) or a
// pair of classes can be driven for applications with their own mechanism.
type SchemeConfig struct {
	Select    bool   `json:"select"`              // offer the switch in the user button
	Mechanism string `json:"mechanism,omitempty"` // "" (color-scheme only) | attribute | class
	Attribute string `json:"attribute,omitempty"` // attribute name (mechanism=attribute)
	Light     string `json:"light,omitempty"`     // attribute value or class for light
	Dark      string `json:"dark,omitempty"`      // attribute value or class for dark
}

// RolesConfig puts the user's EFFECTIVE role names on the page - as classes
// (default) or one attribute on a chosen tag (default body), or as a <meta>
// tag - so the application can gate elements on roles with pure CSS.
type RolesConfig struct {
	Enabled   bool   `json:"enabled"`
	Mechanism string `json:"mechanism,omitempty"` // class (default) | attribute | meta
	Tag       string `json:"tag,omitempty"`       // target tag for class/attribute, default body
	Attribute string `json:"attribute,omitempty"` // attribute/meta name (defaults data-roles / meerkat-roles)
}

// PageUserFields are the signed-in user's facts a UI route may stamp on its
// pages. Each one's attribute/meta name is configurable and DEFAULTS TO THE
// FIELD ITSELF - `tenantid="acme"`, not `data-tenantid` (roles are the ones
// that carry a prefix, see RolesConfig).
//
// Hence the lower case, which is not a slip: these values are written into the
// page as attribute names, and an HTML attribute is case-insensitive and
// normalised to lower case by the DOM. Spelling one "tenantId" here would
// promise a casing the browser does not keep.
var PageUserFields = []string{"username", "userid", "fullname", "email", "tenant", "tenantid", "timezone", "locale"}

// UserInfoConfig exposes the signed-in user's identity to the page - the
// SELECTED fields land as attributes on a chosen tag (default body) or as
// <meta> tags, each under its configured name.
type UserInfoConfig struct {
	Enabled   bool              `json:"enabled"`
	Mechanism string            `json:"mechanism,omitempty"` // attribute (default) | meta
	Tag       string            `json:"tag,omitempty"`       // target tag for attributes, default body
	Fields    map[string]string `json:"fields,omitempty"`    // field -> attribute/meta name ("" = default)
}

// LocalesConfig says how a route carries the user's language, and how a page
// of it follows a change. ONE mechanism, not a set: the list used to hold
// transports only, where cumulating meant something, and it now decides the
// page's behaviour too - declaring both "the language is in the path" and "the
// application applies it itself" is declaring two contradictory answers to the
// same question.
//
//	accept  Accept-Language alone (always sent anyway); the page reloads
//	custom  plus the Header header;                     the page reloads
//	query   plus the Param parameter (default lg);      the page reloads
//	path    plus /<locale> after what the route matched; the page navigates
//	script  Accept-Language alone; the page applies it in place, no reload
//
// Accept-Language ALWAYS goes, rewritten with the resolved locale in front:
// it is not one of the choices, it is the floor.
type LocalesConfig struct {
	Mechanism string `json:"mechanism,omitempty"`
	// Mechanisms is the retired cumulative form, read once so an instance
	// configured before the choice became exclusive keeps working; the first
	// value wins and the field is dropped on the next save.
	Mechanisms []string `json:"mechanisms,omitempty"`
	Header     string   `json:"header,omitempty"` // custom header name
	Param      string   `json:"param,omitempty"`  // query parameter name
	// Disabled excludes application locales THIS route's UI does not
	// support: they leave the button's menu and the forwarding resolution.
	Disabled []string `json:"disabled,omitempty"`
	// OnChange is the "script" mechanism's body: JavaScript run with the
	// person's language in `locale`, whenever it changes - here, or in another
	// tab of the same browser - and once when a page loads, so an application
	// never has to read or store the language itself. It applies it IN PLACE:
	// reloading is not allowed, since the same call happens at every load.
	OnChange string `json:"onChange,omitempty"`
}

// Mode returns the one mechanism in force, reading the retired cumulative
// field when a route predates the exclusive choice.
func (l LocalesConfig) Mode() string {
	if l.Mechanism != "" {
		return l.Mechanism
	}
	if len(l.Mechanisms) > 0 {
		return l.Mechanisms[0]
	}
	return LocaleAccept
}

// The mechanisms, in the order the console offers them.
const (
	LocaleAccept = "accept"
	LocaleCustom = "custom"
	LocaleQuery  = "query"
	LocalePath   = "path"
	LocaleScript = "script"
)

// LocaleMechanisms is the roster, for the console and for validation.
var LocaleMechanisms = []string{LocaleAccept, LocaleCustom, LocaleQuery, LocalePath, LocaleScript}

// LocaleHookName is where OnChange lands on the page, and the name the injected
// user button looks up before deciding whether to reload. It lives here, with
// the field it serves, because the two ends are in different packages and a
// name written twice is a name that drifts.
const LocaleHookName = "meerkatOnLocaleChange"

// RouteUI holds the UI-route options: the application's color-scheme
// mechanism, the roles/user-info page injections, the user button, the
// locale offer, and free CSS/JS blocks injected into the pages (UIF-02).
type RouteUI struct {
	Scheme     *SchemeConfig   `json:"scheme,omitempty"`
	Roles      *RolesConfig    `json:"roles,omitempty"`
	UserInfo   *UserInfoConfig `json:"userInfo,omitempty"`
	UserButton UserButton      `json:"userButton"`
	// CustomCSS is injected verbatim inside a <style> tag after <head>.
	CustomCSS string `json:"customCss,omitempty"`
	// CustomJS is injected verbatim inside a <script> tag after <head>.
	CustomJS string `json:"customJs,omitempty"`
	// Link is the app's menu label. When set, the route is listed in the user's
	// apps menu (subject to access) under this name; empty = reachable but
	// unlisted.
	Link string `json:"link,omitempty"`
}

// IdentityFields are the signed-in user's facts a route may forward to its
// upstream service. "roles" is the only multi-valued one (see IdentityAttr).
var IdentityFields = []string{"username", "userid", "fullname", "tenant", "tenantid", "email", "timezone", "locale", "roles"}

// IdentityForward sends the signed-in caller to the upstream service. Mechanism
// picks the transport; the SAME selected Attributes (and their per-attribute
// mapping) feed every transport: the "headers" mechanism writes one header per
// attribute, the "jwt"/"signed-jwt" mechanisms write one claim per attribute in
// a token carried by Authorization: Bearer. An empty Mechanism forwards nothing.
type IdentityForward struct {
	Mechanism  string         `json:"mechanism,omitempty"` // "" (off) | headers | jwt | signed-jwt
	Attributes []IdentityAttr `json:"attributes,omitempty"`
	// TTL is the token lifetime (exp) for the jwt/signed-jwt transports, an
	// ISO-8601 duration; empty means the default (2 minutes).
	TTL string `json:"ttl,omitempty"`
	// Algorithm is the signature algorithm for signed-jwt (ES256|EdDSA|RS256);
	// unused by the header and unsigned-jwt transports.
	Algorithm string `json:"algorithm,omitempty"`
}

// IdentityAttr is one caller fact to forward, optionally renamed on the way out
// (As is the target header name or claim name; the field name is the default).
// AsJSON only bears on the multi-valued "roles": true renders a JSON array,
// false a comma-separated string.
type IdentityAttr struct {
	Field  string `json:"field"`
	As     string `json:"as,omitempty"`
	AsJSON bool   `json:"asJson,omitempty"`
	// Expr shapes the ROLES this route sends, as a pipeline:
	//
	//	{{.Roles | .Keep "tag:BILLING" "ROLE_USER" | cut "ROLE_" | join ","}}
	//
	// Empty means the plain list, comma-separated. One expression rather than
	// a switch per wish (a JSON toggle, a prefix to trim, a tag filter): every
	// estate wants a different shape, and the weight is the point - 320 roles
	// is 9 KB of header on every request for a service that uses three.
	Expr string `json:"expr,omitempty"`
}

// Route is one declarative routing rule: predicates match a request, filters
// transform it, and the upstream (or a terminal filter) answers it.
type Route struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Order   int    `json:"order"`
	Enabled bool   `json:"enabled"`
	// Access is the route's base security (RBAC-06): the unified rule applied to
	// the whole route (authenticated + users + roles, OR-combined). Empty means
	// delegated to the upstream. Endpoint-security overrides refine it per
	// operation. It is edited both in the route's Security section and as the
	// "whole route" default of the endpoint-security screen.
	Access Access `json:"access"`
	// IsUI toggles the UI-only options (user button, page injections, path
	// locales): a route is always a service, UI comes on top (ROUTE-02).
	IsUI       bool           `json:"isUi"`
	Upstream   string         `json:"upstream"`
	Predicates []routing.Spec `json:"predicates"`
	Filters    []routing.Spec `json:"filters"`
	API        *RouteAPI      `json:"api,omitempty"`
	UI         *RouteUI       `json:"ui,omitempty"`
	// Identity forwards the signed-in user to the upstream service - valid
	// for both route types (an API service wants the caller too).
	Identity *IdentityForward `json:"identity,omitempty"`
	// Locales is the route's language offer (inherited from the application
	// languages by default) and its forwarding mechanism - both types too:
	// an API takes the locale as a header or query parameter.
	Locales *LocalesConfig `json:"locales,omitempty"`
	// Timeouts bounds how long this upstream may take (ROUTE-07). Nil means
	// the installation's defaults, which is what every route ran with when the
	// bounds were global.
	Timeouts *RouteTimeouts `json:"timeouts,omitempty"`
	// Breaker stops calling an upstream that has stopped answering (ROUTE-09).
	// Nil, or present and off, is off - the state of every route until
	// somebody turns it on.
	Breaker *CircuitBreaker `json:"breaker,omitempty"`
}

// ListRoutes returns every route ordered by ascending Order.
func (s *Store) ListRoutes(ctx context.Context) ([]Route, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, ord, enabled, is_ui, upstream, predicates, filters, api, ui, identity, locales, access, timeouts, breaker
		 FROM routes ORDER BY ord ASC, name ASC`)
	if err != nil {
		return nil, fmt.Errorf("store: list routes: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var routes []Route
	for rows.Next() {
		var r Route
		var preds, filts, api, ui, identity, locales, access, timeouts, breaker string
		if err := rows.Scan(&r.ID, &r.Name, &r.Order, &r.Enabled,
			&r.IsUI, &r.Upstream, &preds, &filts, &api, &ui, &identity, &locales, &access, &timeouts, &breaker); err != nil {
			return nil, fmt.Errorf("store: scan route: %w", err)
		}
		if err := json.Unmarshal([]byte(preds), &r.Predicates); err != nil {
			return nil, fmt.Errorf("store: route %q: bad predicates: %w", r.Name, err)
		}
		if err := json.Unmarshal([]byte(filts), &r.Filters); err != nil {
			return nil, fmt.Errorf("store: route %q: bad filters: %w", r.Name, err)
		}
		if err := decodeRouteOptions(&r, api, ui, identity, locales, access, timeouts, breaker); err != nil {
			return nil, fmt.Errorf("store: route %q: %w", r.Name, err)
		}
		routes = append(routes, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list routes: %w", err)
	}
	return routes, nil
}

// SaveRoute inserts or replaces a route by ID.
func (s *Store) SaveRoute(ctx context.Context, r Route) error {
	// Here rather than in the handlers: six callers write routes - the console,
	// the agent, three OpenAPI paths and a configuration import - and a rule
	// cleaned in five of them is a rule nobody can trust.
	if err := SanitizeAccess(&r.Access); err != nil {
		return invalidf(err)
	}
	if err := SanitizeRouteTimeouts(r.Timeouts); err != nil {
		return invalidf(fmt.Errorf("route %q: %w", r.Name, err))
	}
	if err := SanitizeCircuitBreaker(r.Breaker); err != nil {
		return invalidf(fmt.Errorf("route %q: %w", r.Name, err))
	}
	if r.API != nil && r.API.Security != nil {
		for i := range r.API.Security.Endpoints {
			if err := SanitizeAccess(&r.API.Security.Endpoints[i].Access); err != nil {
				return invalidf(fmt.Errorf("%s %s: %w",
					r.API.Security.Endpoints[i].Method, r.API.Security.Endpoints[i].Path, err))
			}
		}
	}
	preds, err := json.Marshal(orEmpty(r.Predicates))
	if err != nil {
		return fmt.Errorf("store: route %q: %w", r.Name, err)
	}
	filts, err := json.Marshal(orEmpty(r.Filters))
	if err != nil {
		return fmt.Errorf("store: route %q: %w", r.Name, err)
	}
	api, ui, identity, locales := "{}", "{}", "{}", "{}"
	if r.API != nil {
		b, err := json.Marshal(r.API)
		if err != nil {
			return fmt.Errorf("store: route %q: %w", r.Name, err)
		}
		api = string(b)
	}
	if r.UI != nil {
		b, err := json.Marshal(r.UI)
		if err != nil {
			return fmt.Errorf("store: route %q: %w", r.Name, err)
		}
		ui = string(b)
	}
	if r.Identity != nil {
		b, err := json.Marshal(r.Identity)
		if err != nil {
			return fmt.Errorf("store: route %q: %w", r.Name, err)
		}
		identity = string(b)
	}
	if r.Locales != nil {
		b, err := json.Marshal(r.Locales)
		if err != nil {
			return fmt.Errorf("store: route %q: %w", r.Name, err)
		}
		locales = string(b)
	}
	timeouts := "{}"
	if r.Timeouts != nil {
		b, err := json.Marshal(r.Timeouts)
		if err != nil {
			return fmt.Errorf("store: route %q: %w", r.Name, err)
		}
		timeouts = string(b)
	}
	breaker := "{}"
	if r.Breaker != nil {
		b, err := json.Marshal(r.Breaker)
		if err != nil {
			return fmt.Errorf("store: route %q: %w", r.Name, err)
		}
		breaker = string(b)
	}
	access, err := json.Marshal(r.Access)
	if err != nil {
		return fmt.Errorf("store: route %q: %w", r.Name, err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO routes (id, name, ord, enabled, is_ui, upstream, predicates, filters, api, ui, identity, locales, access, timeouts, breaker)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   name = excluded.name, ord = excluded.ord, enabled = excluded.enabled,
		   is_ui = excluded.is_ui, upstream = excluded.upstream,
		   predicates = excluded.predicates, filters = excluded.filters,
		   api = excluded.api, ui = excluded.ui, identity = excluded.identity, locales = excluded.locales,
		   access = excluded.access, timeouts = excluded.timeouts, breaker = excluded.breaker`,
		r.ID, r.Name, r.Order, r.Enabled, r.IsUI, r.Upstream,
		string(preds), string(filts), api, ui, identity, locales, string(access), timeouts, breaker)
	if err != nil {
		return fmt.Errorf("store: save route %q: %w", r.Name, err)
	}
	return nil
}

// ReorderRoutes sets each route's ord to its position in ids - route matching
// is first-match-wins, so this ordering is significant (ROUTE order). Runs in a
// transaction so a partial reorder never lands.
func (s *Store) ReorderRoutes(ctx context.Context, ids []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: reorder routes: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for i, id := range ids {
		if _, err := tx.ExecContext(ctx, `UPDATE routes SET ord = ? WHERE id = ?`, i, id); err != nil {
			return fmt.Errorf("store: reorder route %q: %w", id, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: reorder routes: %w", err)
	}
	return nil
}

// GetRoute returns one route by ID, or an error wrapping sql.ErrNoRows.
func (s *Store) GetRoute(ctx context.Context, id string) (Route, error) {
	var r Route
	var preds, filts, api, ui, identity, locales, access, timeouts, breaker string
	err := s.db.QueryRowContext(ctx,
		`SELECT id, name, ord, enabled, is_ui, upstream, predicates, filters, api, ui, identity, locales, access, timeouts, breaker
		 FROM routes WHERE id = ?`, id).
		Scan(&r.ID, &r.Name, &r.Order, &r.Enabled, &r.IsUI, &r.Upstream, &preds, &filts, &api, &ui, &identity, &locales, &access, &timeouts, &breaker)
	if err != nil {
		return Route{}, fmt.Errorf("store: get route %q: %w", id, err)
	}
	if err := json.Unmarshal([]byte(preds), &r.Predicates); err != nil {
		return Route{}, fmt.Errorf("store: route %q: bad predicates: %w", id, err)
	}
	if err := json.Unmarshal([]byte(filts), &r.Filters); err != nil {
		return Route{}, fmt.Errorf("store: route %q: bad filters: %w", id, err)
	}
	if err := decodeRouteOptions(&r, api, ui, identity, locales, access, timeouts, breaker); err != nil {
		return Route{}, fmt.Errorf("store: route %q: %w", id, err)
	}
	return r, nil
}

// DeleteRoute removes a route and reports whether it existed.
func (s *Store) DeleteRoute(ctx context.Context, id string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM routes WHERE id = ?`, id)
	if err != nil {
		return false, fmt.Errorf("store: delete route %q: %w", id, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// decodeRouteOptions hydrates the per-type option objects; "{}" stays nil so
// the JSON API omits what was never configured.
func decodeRouteOptions(r *Route, api, ui, identity, locales, access, timeouts, breaker string) error {
	if access != "" && access != "{}" {
		if err := json.Unmarshal([]byte(access), &r.Access); err != nil {
			return fmt.Errorf("bad access options: %w", err)
		}
	}
	if api != "" && api != "{}" {
		r.API = &RouteAPI{}
		if err := json.Unmarshal([]byte(api), r.API); err != nil {
			return fmt.Errorf("bad api options: %w", err)
		}
	}
	if ui != "" && ui != "{}" {
		r.UI = &RouteUI{}
		if err := json.Unmarshal([]byte(ui), r.UI); err != nil {
			return fmt.Errorf("bad ui options: %w", err)
		}
	}
	if breaker != "" && breaker != "{}" {
		r.Breaker = &CircuitBreaker{}
		if err := json.Unmarshal([]byte(breaker), r.Breaker); err != nil {
			return fmt.Errorf("bad breaker: %w", err)
		}
	}
	if timeouts != "" && timeouts != "{}" {
		r.Timeouts = &RouteTimeouts{}
		if err := json.Unmarshal([]byte(timeouts), r.Timeouts); err != nil {
			return fmt.Errorf("bad timeouts: %w", err)
		}
	}
	if identity != "" && identity != "{}" {
		r.Identity = &IdentityForward{}
		if err := json.Unmarshal([]byte(identity), r.Identity); err != nil {
			return fmt.Errorf("bad identity options: %w", err)
		}
	}
	if locales != "" && locales != "{}" {
		r.Locales = &LocalesConfig{}
		if err := json.Unmarshal([]byte(locales), r.Locales); err != nil {
			return fmt.Errorf("bad locales options: %w", err)
		}
		// Normalised on the way OUT of the store, so nothing downstream has to
		// know the mechanism was once a list - the console above all: it would
		// show "accept" for a route stored as "path", and saving that screen
		// would quietly take the mechanism away.
		if r.Locales.Mechanism == "" && len(r.Locales.Mechanisms) > 0 {
			r.Locales.Mechanism = r.Locales.Mechanisms[0]
		}
		r.Locales.Mechanisms = nil
	}
	return nil
}

func orEmpty(specs []routing.Spec) []routing.Spec {
	if specs == nil {
		return []routing.Spec{}
	}
	return specs
}

// CountRoutes reports how many routes exist (seed decision at first start).
func (s *Store) CountRoutes(ctx context.Context) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM routes`).Scan(&n); err != nil {
		return 0, fmt.Errorf("store: count routes: %w", err)
	}
	return n, nil
}

// User is a local Meerkat account (the nominal identity model - §1.3 of the
// requirements). Password is stored as a bcrypt hash, never in clear. The
// boolean flags are the cross-cutting SUPERPOWERS (RBAC-05): root administers
// the gateway, dev unlocks the developer tooling, tester can opt into dev
// variants, tenant_creator may create tenants. Tenant administration is not a
// superpower - it is the membership type (TENANT-02).
type User struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	PasswordHash  string `json:"-"`
	Fullname      string `json:"fullname"`
	Email         string `json:"email"`
	Enabled       bool   `json:"enabled"`
	Root          bool   `json:"root"`
	Dev           bool   `json:"dev"`
	Tester        bool   `json:"tester"`
	TenantCreator bool   `json:"tenantCreator"`
	// Split administration (RBAC-05): InfraAdmin runs the routing plane
	// (routes, built-in pages), AppAdmin runs the application's identity
	// (users, roles, settings). Root implies both.
	InfraAdmin bool   `json:"infraAdmin"`
	AppAdmin   bool   `json:"appAdmin"`
	Locale     string `json:"locale"`
	// Scheme is the person's light/dark choice ("" = follow the system). Like
	// the language it is written by the person alone (SetUserScheme), and
	// deliberately absent from UpdateUser: an administrator's save carries no
	// value for it, and would blank what its owner picked.
	Scheme           string `json:"scheme"`
	Timezone         string `json:"timezone"`
	CreatedAt        int64  `json:"createdAt"`
	UpdatedAt        int64  `json:"updatedAt"`
	LastConnectionAt int64  `json:"lastConnectionAt"`
	// MustChangePassword forces the update-password step at next login
	// (temporary passwords from creation or reset - AUTH-05 step 1).
	MustChangePassword bool `json:"mustChangePassword"`
	// MFARequired is the per-user second-factor policy (MFA-04): "" inherits the
	// global setting, "true"/"false" force it for this user.
	MFARequired string `json:"mfaRequired"`
	// EmailVerified is false only for a self-registered account that has not
	// confirmed its address yet (AUTH-20) - such an account cannot sign in.
	EmailVerified bool `json:"emailVerified"`
	// SelfRegistered marks accounts born on /register (purge + admin display).
	SelfRegistered bool `json:"selfRegistered"`
	// HasPassword is derived, never stored: an account born at an authority
	// holds no local password, and the console must say so rather than offer
	// to "reset" one that does not exist. The hash itself never travels.
	HasPassword bool `json:"hasPassword"`
	// PasswordChangedAt feeds the expiry rule (AUTH-10). Zero means NOT KNOWN,
	// not 1970: an account whose password predates the column must not be
	// expired at its next sign-in because the column says the epoch.
	PasswordChangedAt int64 `json:"passwordChangedAt,omitempty"`
}

const userCols = `id, username, password_hash, fullname, email, enabled,
	root, dev, tester, tenant_creator, infra_admin, app_admin, locale, scheme, timezone,
	created_at, updated_at, last_connection_at, must_change_password, mfa_required,
	email_verified, self_registered, password_changed_at`

func scanUser(row interface{ Scan(...any) error }) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Fullname, &u.Email, &u.Enabled,
		&u.Root, &u.Dev, &u.Tester, &u.TenantCreator, &u.InfraAdmin, &u.AppAdmin, &u.Locale, &u.Scheme, &u.Timezone,
		&u.CreatedAt, &u.UpdatedAt, &u.LastConnectionAt, &u.MustChangePassword, &u.MFARequired,
		&u.EmailVerified, &u.SelfRegistered, &u.PasswordChangedAt)
	// Derived here so every read carries it and no caller has to remember: the
	// hash is json:"-", this boolean is what the console is allowed to know.
	u.HasPassword = u.PasswordHash != ""
	return u, err
}

// CreateUser inserts a new user.
func (s *Store) CreateUser(ctx context.Context, u User) error {
	if err := validTristate(u.MFARequired); err != nil {
		return fmt.Errorf("store: create user %q: mfaRequired: %w", u.Username, err)
	}
	now := time.Now().Unix()
	if u.Timezone == "" {
		u.Timezone = "UTC"
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (id, username, password_hash, fullname, email, enabled,
		   root, dev, tester, tenant_creator, infra_admin, app_admin, locale, timezone,
		   created_at, updated_at, must_change_password, mfa_required,
		   email_verified, self_registered)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Username, u.PasswordHash, u.Fullname, u.Email, u.Enabled,
		u.Root, u.Dev, u.Tester, u.TenantCreator, u.InfraAdmin, u.AppAdmin, u.Locale, u.Timezone,
		now, now, u.MustChangePassword, u.MFARequired,
		u.EmailVerified, u.SelfRegistered)
	if err != nil {
		return fmt.Errorf("store: create user %q: %w", u.Username, err)
	}
	return nil
}

// UpdateUser saves the editable identity fields and superpower flags of an
// existing user. The password travels through SetUserPassword only.
func (s *Store) UpdateUser(ctx context.Context, u User) error {
	if err := validTristate(u.MFARequired); err != nil {
		return fmt.Errorf("store: update user %q: mfaRequired: %w", u.Username, err)
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET username = ?, fullname = ?, email = ?, enabled = ?,
		   root = ?, dev = ?, tester = ?, tenant_creator = ?, infra_admin = ?, app_admin = ?,
		   locale = ?, timezone = ?, mfa_required = ?, updated_at = ?
		 WHERE id = ?`,
		u.Username, u.Fullname, u.Email, u.Enabled,
		u.Root, u.Dev, u.Tester, u.TenantCreator, u.InfraAdmin, u.AppAdmin, u.Locale, u.Timezone,
		u.MFARequired, time.Now().Unix(), u.ID)
	if err != nil {
		return fmt.Errorf("store: update user %q: %w", u.Username, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store: update user %q: %w", u.ID, sql.ErrNoRows)
	}
	return nil
}

// SetUserPassword replaces a user's password hash. mustChange marks the new
// password as temporary (an admin reset - the next login forces the
// update-password step); a user changing their own password clears it.
//
// The OUTGOING hash is remembered here, and the change is stamped, so the
// history and expiry rules (AUTH-10) hold wherever a password is set: the
// profile, a reset link, the forced change, an admin reset. Doing it in the
// four callers instead would mean four chances to forget, and the one that
// forgot would be the hole.
func (s *Store) SetUserPassword(ctx context.Context, id, passwordHash string, mustChange bool) error {
	now := time.Now().Unix()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("store: set password for user %q: %w", id, err)
	}
	defer func() { _ = tx.Rollback() }()

	var previous string
	if err := tx.QueryRowContext(ctx, `SELECT password_hash FROM users WHERE id = ?`, id).
		Scan(&previous); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("store: set password for user %q: %w", id, sql.ErrNoRows)
		}
		return fmt.Errorf("store: set password for user %q: %w", id, err)
	}
	// An account born at an authority has no local hash to remember, and the
	// same password set twice in a row would otherwise be filed twice.
	if previous != "" && previous != passwordHash {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO password_history (user_id, password_hash, changed_at) VALUES (?, ?, ?)`,
			id, previous, now); err != nil {
			return fmt.Errorf("store: set password for user %q: %w", id, err)
		}
		// Trimmed to the ceiling the policy can ask for: history is a rule
		// about the RECENT past, and rows nobody will ever compare against are
		// hashes kept for no reason.
		if _, err := tx.ExecContext(ctx,
			// By the hash, because this table has no id of its own and rowid
			// is SQLite's alone. Not by changed_at: it has a second's
			// resolution, and several passwords changed within one second
			// would share a value - the subquery would then keep every row of
			// that second instead of the count asked for, and a password long
			// out of the window would go on being refused.
			`DELETE FROM password_history WHERE user_id = ? AND password_hash NOT IN (
			   SELECT password_hash FROM password_history WHERE user_id = ?
			   ORDER BY changed_at DESC, password_hash DESC LIMIT ?)`,
			id, id, maxPasswordHistory); err != nil {
			return fmt.Errorf("store: set password for user %q: %w", id, err)
		}
	}
	res, err := tx.ExecContext(ctx,
		`UPDATE users SET password_hash = ?, must_change_password = ?, password_changed_at = ?, updated_at = ? WHERE id = ?`,
		passwordHash, mustChange, now, now, id)
	if err != nil {
		return fmt.Errorf("store: set password for user %q: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store: set password for user %q: %w", id, sql.ErrNoRows)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("store: set password for user %q: %w", id, err)
	}
	return nil
}

// maxPasswordHistory is the ceiling PasswordPolicy.Sanitize enforces: the
// table never holds more than a policy could ever ask to compare.
const maxPasswordHistory = 24

// SetMustChangePassword makes an account change its password at the next
// sign-in, KEEPING the one it has. Distinct from a reset, which replaces the
// password with a temporary one somebody then has to carry to its owner.
func (s *Store) SetMustChangePassword(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET must_change_password = ?, updated_at = ? WHERE id = ? AND password_hash != ''`,
		true, time.Now().Unix(), id)
	if err != nil {
		return fmt.Errorf("store: flag password change for user %q: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store: flag password change for user %q: %w", id, sql.ErrNoRows)
	}
	return nil
}

// SetMustChangePasswordForAll flags every LOCAL account and returns how many.
// except is spared - the administrator firing this from the console.
//
// Accounts with no local password are left alone: their credentials live at
// their authority, and sending them to a change page would be sending them to
// a form that cannot help them.
func (s *Store) SetMustChangePasswordForAll(ctx context.Context, except string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE users SET must_change_password = ?, updated_at = ? WHERE password_hash != '' AND id != ?`,
		true, time.Now().Unix(), except)
	if err != nil {
		return 0, fmt.Errorf("store: flag password change for every account: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// RecentPasswordHashes returns the last n retired hashes for a user, newest
// first. The COMPARISON lives in the auth layer: bcrypt is how a password is
// checked, not how it is stored, and the store has no business holding that
// opinion.
func (s *Store) RecentPasswordHashes(ctx context.Context, userID string, n int) ([]string, error) {
	if n <= 0 {
		return nil, nil
	}
	if n > maxPasswordHistory {
		n = maxPasswordHistory
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT password_hash FROM password_history WHERE user_id = ?
		 ORDER BY changed_at DESC, password_hash DESC LIMIT ?`, userID, n)
	if err != nil {
		return nil, fmt.Errorf("store: password history for user %q: %w", userID, err)
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return nil, fmt.Errorf("store: password history for user %q: %w", userID, err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// DeleteUser removes a user (memberships and sessions cascade) and reports
// whether it existed.
func (s *Store) DeleteUser(ctx context.Context, id string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return false, fmt.Errorf("store: delete user %q: %w", id, err)
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// ListUsers returns every user ordered by username.
func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+userCols+` FROM users ORDER BY username ASC`)
	if err != nil {
		return nil, fmt.Errorf("store: list users: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var users []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, fmt.Errorf("store: scan user: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list users: %w", err)
	}
	return users, nil
}

// GetUserByUsername returns the user or sql.ErrNoRows wrapped.
func (s *Store) GetUserByUsername(ctx context.Context, username string) (User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx,
		`SELECT `+userCols+` FROM users WHERE username = ?`, username))
	if err != nil {
		return User{}, fmt.Errorf("store: get user %q: %w", username, err)
	}
	return u, nil
}

// SanitizeAvatar validates a profile photo: an image data URI (png, jpeg or
// webp - a photo, not a logo) of reasonable size; "" clears it. It lands in a
// src attribute, nothing else may.
func SanitizeAvatar(avatar string) error {
	if avatar == "" {
		return nil
	}
	if len(avatar) > 300_000 {
		return fmt.Errorf("avatar is too large (%d bytes): keep it under ~200 KiB", len(avatar))
	}
	for _, prefix := range []string{"data:image/png;base64,", "data:image/jpeg;base64,", "data:image/webp;base64,"} {
		if strings.HasPrefix(avatar, prefix) {
			return nil
		}
	}
	return fmt.Errorf("avatar must be a base64 data URI of type png, jpeg or webp")
}

// SetUserAvatar stores (or clears, with "") a user's profile photo. The
// avatar lives in its own column and is only ever read on demand - user
// LISTS never carry it.
func (s *Store) SetUserAvatar(ctx context.Context, id, avatar string) error {
	if err := SanitizeAvatar(avatar); err != nil {
		return err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE users SET avatar = ? WHERE id = ?`, avatar, id)
	if err != nil {
		return fmt.Errorf("store: set avatar: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNoRows
	}
	return nil
}

// SetUserLocale stores a user's own language (self-service, from the language
// menu of the injected user button - I18N-04). The CODE is validated by the
// caller against the application's offer; "" is allowed here and means "follow
// the browser", which is what a user who never chose has.
func (s *Store) SetUserLocale(ctx context.Context, id, locale string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE users SET locale = ? WHERE id = ?`, strings.TrimSpace(locale), id)
	if err != nil {
		return fmt.Errorf("store: set locale: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNoRows
	}
	return nil
}

// Schemes are the colour-scheme choices a person may store. "" is the fourth
// and the default: follow the system, which is what someone who never chose
// has - and what "auto" means on the pages.
var Schemes = []string{"light", "dark"}

// SetUserScheme stores a user's own light/dark choice (self-service, from the
// scheme switch of the flow pages and of the injected user button - THEME-05).
// "" is allowed and means "follow the system".
func (s *Store) SetUserScheme(ctx context.Context, id, scheme string) error {
	scheme = strings.TrimSpace(scheme)
	if scheme == "auto" {
		// What the pages call auto is what the account calls nothing: storing
		// the word would make "chose to follow the system" and "never chose"
		// two different states nobody can tell apart.
		scheme = ""
	}
	if scheme != "" && !slices.Contains(Schemes, scheme) {
		return fmt.Errorf("store: set scheme %q: allowed are %s, or empty to follow the system",
			scheme, strings.Join(Schemes, ", "))
	}
	res, err := s.db.ExecContext(ctx, `UPDATE users SET scheme = ? WHERE id = ?`, scheme, id)
	if err != nil {
		return fmt.Errorf("store: set scheme: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNoRows
	}
	return nil
}

// SetUserTimezone stores a user's own timezone (self-service, from the profile
// page). The NAME is validated by the caller against the platform's zone
// database; the store only refuses an empty one, which would mean "no zone" -
// a user always has one, defaulting to UTC.
func (s *Store) SetUserTimezone(ctx context.Context, id, tz string) error {
	if strings.TrimSpace(tz) == "" {
		tz = "UTC"
	}
	res, err := s.db.ExecContext(ctx, `UPDATE users SET timezone = ? WHERE id = ?`, tz, id)
	if err != nil {
		return fmt.Errorf("store: set timezone: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNoRows
	}
	return nil
}

// GetUserAvatar returns the user's photo data URI ("" = none).
func (s *Store) GetUserAvatar(ctx context.Context, id string) (string, error) {
	var avatar string
	err := s.db.QueryRowContext(ctx, `SELECT avatar FROM users WHERE id = ?`, id).Scan(&avatar)
	if err != nil {
		return "", fmt.Errorf("store: get avatar: %w", err)
	}
	return avatar, nil
}

// GetUserByID returns the user or an error wrapping sql.ErrNoRows.
func (s *Store) GetUserByID(ctx context.Context, id string) (User, error) {
	u, err := scanUser(s.db.QueryRowContext(ctx,
		`SELECT `+userCols+` FROM users WHERE id = ?`, id))
	if err != nil {
		return User{}, fmt.Errorf("store: get user id %q: %w", id, err)
	}
	return u, nil
}

// TouchLastConnection stamps a successful login (AUTH-13 will build on it).
func (s *Store) TouchLastConnection(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx,
		`UPDATE users SET last_connection_at = ? WHERE id = ?`, time.Now().Unix(), id); err != nil {
		return fmt.Errorf("store: touch last connection of %q: %w", id, err)
	}
	return nil
}

// CountUsers reports how many users exist (admin seed decision).
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return 0, fmt.Errorf("store: count users: %w", err)
	}
	return n, nil
}

// Session is the persisted server-side state behind an opaque cookie. Only a
// hash of the token is stored - a database leak reveals no usable cookies.
// TenantID is the active tenant (TENANT-03): "" when the user has no
// membership or has not selected one yet.
type Session struct {
	TokenHash string
	UserID    string
	TenantID  string
	// GroupID is the ACTIVE group when the tenant's effective group mode is
	// SINGLE (RBAC-03): "" = none chosen (or cumulative mode). Reset on every
	// tenant change - groups are per tenant.
	GroupID string
	Pending string // login-flow step to complete ("" = none - AUTH-05)
	// Next is the post-login destination, carried on the session across the
	// multi-step flow so it need not ride in the URL. Validated (safeNext) before
	// it is stored, immutable by the client thereafter.
	Next      string
	ExpiresAt int64 // unix seconds
	// Method is how the FIRST factor was answered: it rides across the login
	// steps so the history can name it at the end.
	Method string
	// TTL is the lifetime this session was issued with, in seconds - the
	// value RESOLVED at login (membership, then tenant, then global). Kept on
	// the row because the deadline slides: every request pushes ExpiresAt to
	// now + TTL, and the resolution that produced it is not available on the
	// hot path.
	TTL int64
	// Plane isolates the two ports' sessions: cookies are not port-scoped, so
	// each plane uses its own cookie name AND every resolve checks the plane -
	// a data-plane token pasted into the admin cookie dies here.
	Plane string // "data" | "admin"
	// The three fields below are NOT columns: they exist only on the session
	// SYNTHESIZED for an "Authorization: Bearer" call, and they are what tells
	// the guard that this caller is a token rather than a person - which token
	// (the audit names it, MCP-03) and how far it may go (MCP-02).
	TokenID     string
	TokenName   string
	TokenScope  string
	TokenDomain string
}

// CreateSession persists a session.
func (s *Store) CreateSession(ctx context.Context, sess Session) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, user_id, tenant_id, group_id, pending, next, method, expires_at, ttl, plane) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sess.TokenHash, sess.UserID, sess.TenantID, sess.GroupID, sess.Pending, sess.Next, sess.Method, sess.ExpiresAt, sess.TTL, sess.Plane)
	if err != nil {
		return fmt.Errorf("store: create session: %w", err)
	}
	return nil
}

// GetSession returns the session for a token hash, or an error wrapping
// sql.ErrNoRows when absent (revoked or never issued).
func (s *Store) GetSession(ctx context.Context, tokenHash string) (Session, error) {
	var sess Session
	err := s.db.QueryRowContext(ctx,
		`SELECT token_hash, user_id, tenant_id, group_id, pending, next, method, expires_at, ttl, plane FROM sessions WHERE token_hash = ?`, tokenHash).
		Scan(&sess.TokenHash, &sess.UserID, &sess.TenantID, &sess.GroupID, &sess.Pending, &sess.Next, &sess.Method, &sess.ExpiresAt, &sess.TTL, &sess.Plane)
	if err != nil {
		return Session{}, fmt.Errorf("store: get session: %w", err)
	}
	return sess, nil
}

// ExtendSession pushes a session's deadline. The WHERE clause carries the
// expiry the caller believed in: a session another node (or a logout) has
// already ended must not come back to life because a request was in flight.
func (s *Store) ExtendSession(ctx context.Context, tokenHash string, was, until int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET expires_at = ? WHERE token_hash = ? AND expires_at = ?`, until, tokenHash, was)
	if err != nil {
		return fmt.Errorf("store: extend session: %w", err)
	}
	return nil
}

// SetSessionPending records the login-flow step a session still has to
// complete; "" marks the flow done (AUTH-05).
func (s *Store) SetSessionPending(ctx context.Context, tokenHash, pending string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET pending = ? WHERE token_hash = ?`, pending, tokenHash)
	if err != nil {
		return fmt.Errorf("store: set session pending: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store: set session pending: %w", sql.ErrNoRows)
	}
	return nil
}

// SetSessionTenant records the active tenant of a session (login with a single
// membership, or the select-tenant page - TENANT-03).
func (s *Store) SetSessionTenant(ctx context.Context, tokenHash, tenantID string) error {
	// Groups are PER TENANT: changing the tenant always resets the active
	// group - the handler re-runs the group decision for the new tenant.
	res, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET tenant_id = ?, group_id = '' WHERE token_hash = ?`, tenantID, tokenHash)
	if err != nil {
		return fmt.Errorf("store: set session tenant: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store: set session tenant: %w", sql.ErrNoRows)
	}
	return nil
}

// SetSessionGroup records the ACTIVE group of a session (SINGLE mode,
// RBAC-03). The handler validates membership of the group beforehand.
func (s *Store) SetSessionGroup(ctx context.Context, tokenHash, groupID string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET group_id = ? WHERE token_hash = ?`, groupID, tokenHash)
	if err != nil {
		return fmt.Errorf("store: set session group: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("store: set session group: %w", sql.ErrNoRows)
	}
	return nil
}

// DeleteSessionsForUser revokes EVERY session of a user (both planes) - a
// password reset kills whatever a possible intruder still holds.
func (s *Store) DeleteSessionsForUser(ctx context.Context, userID string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
	if err != nil {
		return 0, fmt.Errorf("store: delete sessions for %q: %w", userID, err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// DeleteSession revokes a single session. Deleting an absent session is not
// an error.
func (s *Store) DeleteSession(ctx context.Context, tokenHash string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash); err != nil {
		return fmt.Errorf("store: delete session: %w", err)
	}
	return nil
}

// PurgeExpiredSessions removes every session past its expiry (TTL upkeep,
// STORE-04) and reports how many were removed.
func (s *Store) PurgeExpiredSessions(ctx context.Context, now int64) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, now)
	if err != nil {
		return 0, fmt.Errorf("store: purge sessions: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// addMissingColumns brings an existing database up to the schema above.
//
// Design phase: columns come and go, and nobody writes a migration for each
// one. But CREATE TABLE IF NOT EXISTS does nothing to a table that already
// exists, so a database created two weeks ago keeps the columns it was born
// with and fails at the worst moment - an INSERT naming a column that is not
// there, surfacing as an SQL error three layers up.
//
// So the schema itself is the reference: it is applied to an in-memory
// database, the two are compared table by table, and whatever is missing is
// added. Nothing is ever dropped or rewritten - a column that disappears from
// the schema simply stays behind, unused, which costs nothing and loses no
// data.
func (s *Store) addMissingColumns() error {
	// The reference is the schema THIS BUILD carries, read from a throwaway
	// SQLite: we are only parsing our own DDL, so it stays SQLite whatever the
	// live database is. What has to be dialectal is reading the LIVE one.
	raw, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return fmt.Errorf("store: reference schema: %w", err)
	}
	defer func() { _ = raw.Close() }()
	if _, err := raw.Exec(schemaSQL); err != nil {
		return fmt.Errorf("store: reference schema: %w", err)
	}
	ref := &database{DB: raw, dialect: dialectSQLite}
	tables, err := ref.tableNames()
	if err != nil {
		return err
	}
	for _, table := range tables {
		want, err := ref.columnsOf(table)
		if err != nil {
			return err
		}
		got, err := s.db.columnsOf(table)
		if err != nil {
			return err
		}
		have := make(map[string]bool, len(got))
		for _, c := range got {
			have[c.name] = true
		}
		for _, c := range want {
			if have[c.name] {
				continue
			}
			// SQLite refuses ADD COLUMN for NOT NULL without a default, and for
			// anything unique or primary. Those need a fresh database, and
			// saying so beats a raw SQLite error.
			if _, err := s.db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + c.definition()); err != nil {
				return fmt.Errorf("store: %s.%s could not be added to this database "+
					"(delete the data directory to start from the current schema): %w", table, c.name, err)
			}
			slog.Info("schema caught up", "table", table, "column", c.name)
		}
	}
	return nil
}

type column struct {
	name     string
	dataType string
	notNull  bool
	dflt     sql.NullString
}

// definition rebuilds what ALTER TABLE ADD COLUMN needs from what PRAGMA
// table_info reports.
func (c column) definition() string {
	def := c.name + " " + c.dataType
	if c.notNull {
		def += " NOT NULL"
	}
	if c.dflt.Valid {
		def += " DEFAULT " + c.dflt.String
	}
	return def
}
