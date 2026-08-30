package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

const currentSchemaVersion = 6

var (
	ErrUnversionedDatabase = errors.New("unversioned database is not supported")
	ErrNewerDatabase       = errors.New("database schema is newer than this binary")
)

type migration struct {
	version int
	name    string
	sql     string
	apply   func(context.Context, *sql.Tx) error
}

var migrations = []migration{
	{
		version: 1,
		name:    "clean baseline",
		sql: `
CREATE TABLE users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT    NOT NULL UNIQUE,
    email         TEXT    NOT NULL UNIQUE,
    password_hash TEXT    NOT NULL,
    role          TEXT    NOT NULL DEFAULT 'user',
    max_accounts  INTEGER NOT NULL DEFAULT 5,
    status        TEXT    NOT NULL DEFAULT 'active',
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE refresh_tokens (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT    NOT NULL UNIQUE,
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_refresh_user ON refresh_tokens(user_id);

CREATE TABLE accounts (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name            TEXT    NOT NULL,
    channel         TEXT    NOT NULL,
    username        TEXT    NOT NULL,
    password_enc    TEXT    NOT NULL,
    aid             INTEGER NOT NULL DEFAULT 0,
    gs_idx          INTEGER NOT NULL DEFAULT 0,
    ws_url          TEXT    NOT NULL DEFAULT '',
    last_login_at   DATETIME,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, name),
    UNIQUE(user_id, channel, username),
    CHECK(channel IN ('ios', 'alipay'))
);
CREATE INDEX idx_accounts_user ON accounts(user_id);

CREATE TABLE sessions (
    account_id  INTEGER PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    payload_enc TEXT NOT NULL,
    expires_at  DATETIME,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE account_policies (
    account_id  INTEGER PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    policy_json TEXT    NOT NULL,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE operation_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id  INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    ts          DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    kind        TEXT    NOT NULL,
    args_json   TEXT    NOT NULL DEFAULT '{}',
    result_json TEXT    NOT NULL DEFAULT '{}'
);
CREATE INDEX idx_oplog_account_ts ON operation_log(account_id, ts);
CREATE INDEX idx_oplog_ts ON operation_log(ts);

CREATE TABLE event_log (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    account_id   INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    account_name TEXT    NOT NULL DEFAULT '',
    ts           DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    kind         TEXT    NOT NULL,
    message      TEXT    NOT NULL DEFAULT '',
    payload_json TEXT    NOT NULL DEFAULT '{}',
    category     TEXT    NOT NULL DEFAULT '',
    domain       TEXT    NOT NULL DEFAULT '',
    action       TEXT    NOT NULL DEFAULT '',
    label        TEXT    NOT NULL DEFAULT '',
    level        TEXT    NOT NULL DEFAULT 'info'
);
CREATE INDEX idx_event_log_account_id ON event_log(account_id, id);
CREATE INDEX idx_event_log_kind_id ON event_log(kind, id);
CREATE INDEX idx_event_log_ts ON event_log(ts);
`,
	},
	{
		version: 2,
		name:    "strict versioned policies",
		apply:   migratePoliciesV2,
	},
	{
		version: 3,
		name:    "remove retired dessert policy",
		apply:   migratePoliciesV3,
	},
	{
		version: 4,
		name:    "pearl hire daily ticket usage",
		sql: `
CREATE TABLE account_pearl_hire_usage (
    account_id  INTEGER PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    day_id      INTEGER NOT NULL,
    used_count  INTEGER NOT NULL DEFAULT 0 CHECK(used_count >= 0),
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`,
	},
	{
		version: 5,
		name:    "federated redeem exchange",
		sql: `
CREATE TABLE redeem_node_state (
    id            INTEGER PRIMARY KEY CHECK(id = 1),
    instance_id   TEXT    NOT NULL UNIQUE,
    next_revision INTEGER NOT NULL DEFAULT 0 CHECK(next_revision >= 0),
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE redeem_sources (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    name                  TEXT    NOT NULL UNIQUE,
    type                  TEXT    NOT NULL CHECK(type IN ('mygardenworld', 'custom_http')),
    base_url              TEXT    NOT NULL,
    channel               TEXT    NOT NULL DEFAULT '' CHECK(channel IN ('', 'ios', 'alipay')),
    parser_config_json    TEXT    NOT NULL DEFAULT '{}',
    enabled               INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0, 1)),
    push_enabled          INTEGER NOT NULL DEFAULT 0 CHECK(push_enabled IN (0, 1)),
    poll_interval_seconds INTEGER NOT NULL DEFAULT 300 CHECK(poll_interval_seconds >= 60),
    remote_instance_id    TEXT    NOT NULL DEFAULT '',
    cursor                TEXT    NOT NULL DEFAULT '',
    last_sync_at          DATETIME,
    last_error            TEXT    NOT NULL DEFAULT '',
    accepted_count        INTEGER NOT NULL DEFAULT 0 CHECK(accepted_count >= 0),
    invalid_count         INTEGER NOT NULL DEFAULT 0 CHECK(invalid_count >= 0),
    created_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_redeem_sources_due ON redeem_sources(enabled, last_sync_at);

CREATE TABLE redeem_codes (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    fingerprint           TEXT    NOT NULL UNIQUE,
    code                  TEXT    NOT NULL,
    normalized_code       TEXT    NOT NULL,
    channel               TEXT    NOT NULL CHECK(channel IN ('ios', 'alipay')),
    expires_at            DATETIME,
    validation            TEXT    NOT NULL DEFAULT 'pending' CHECK(validation IN ('pending', 'success', 'already_redeemed', 'expired', 'invalid', 'retryable', 'unknown')),
    propagation_state     TEXT    NOT NULL DEFAULT 'waiting_validation' CHECK(propagation_state IN ('waiting_validation', 'eligible', 'sent', 'suppressed_expired', 'suppressed_invalid')),
    local_verified_at     DATETIME,
    community_verified_at DATETIME,
    origin_instance_id    TEXT    NOT NULL DEFAULT '',
    last_message          TEXT    NOT NULL DEFAULT '',
    revision              INTEGER NOT NULL CHECK(revision > 0),
    first_seen_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(normalized_code, channel)
);
CREATE INDEX idx_redeem_codes_revision ON redeem_codes(revision);
CREATE INDEX idx_redeem_codes_active ON redeem_codes(channel, expires_at, validation);

CREATE TABLE redeem_code_observations (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    redeem_code_id     INTEGER NOT NULL REFERENCES redeem_codes(id) ON DELETE CASCADE,
    source_id          INTEGER REFERENCES redeem_sources(id) ON DELETE SET NULL,
    source_key         TEXT    NOT NULL,
    origin_instance_id TEXT    NOT NULL DEFAULT '',
    expires_at         DATETIME,
    validation         TEXT    NOT NULL DEFAULT 'pending' CHECK(validation IN ('pending', 'success', 'already_redeemed', 'expired', 'invalid', 'retryable', 'unknown')),
    observed_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(redeem_code_id, source_key)
);
CREATE INDEX idx_redeem_observations_source ON redeem_code_observations(source_id, observed_at);

CREATE TABLE redeem_attempts (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    redeem_code_id INTEGER NOT NULL REFERENCES redeem_codes(id) ON DELETE CASCADE,
    account_id     INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    status         TEXT    NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'running', 'success', 'already_redeemed', 'expired', 'invalid', 'retryable', 'unknown')),
    message        TEXT    NOT NULL DEFAULT '',
    attempt_count  INTEGER NOT NULL DEFAULT 0 CHECK(attempt_count >= 0),
    attempted_at   DATETIME,
    retry_at       DATETIME,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(redeem_code_id, account_id)
);
CREATE INDEX idx_redeem_attempts_pending ON redeem_attempts(status, retry_at, id);

CREATE TABLE redeem_exchange_outbox (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    source_id      INTEGER NOT NULL REFERENCES redeem_sources(id) ON DELETE CASCADE,
    redeem_code_id INTEGER NOT NULL REFERENCES redeem_codes(id) ON DELETE CASCADE,
    status         TEXT    NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'sending', 'sent')),
    attempt_count  INTEGER NOT NULL DEFAULT 0 CHECK(attempt_count >= 0),
    next_attempt_at DATETIME,
    last_error     TEXT    NOT NULL DEFAULT '',
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(source_id, redeem_code_id)
);
CREATE INDEX idx_redeem_outbox_pending ON redeem_exchange_outbox(status, next_attempt_at, id);
`,
	},
	{
		version: 6,
		name:    "derive redeem source statistics",
		sql: `
ALTER TABLE redeem_sources DROP COLUMN accepted_count;
ALTER TABLE redeem_sources DROP COLUMN invalid_count;
`,
	},
}

func applyMigrations(ctx context.Context, db *sql.DB) error {
	version, err := databaseVersion(ctx, db)
	if err != nil {
		return err
	}
	if version == 0 {
		empty, err := databaseIsEmpty(ctx, db)
		if err != nil {
			return err
		}
		if !empty {
			return fmt.Errorf("%w; this breaking release only accepts versioned databases; use `gardend reset-data --yes` to create the v%d baseline", ErrUnversionedDatabase, currentSchemaVersion)
		}
	}
	if version > currentSchemaVersion {
		return fmt.Errorf("%w: got v%d, binary supports v%d", ErrNewerDatabase, version, currentSchemaVersion)
	}

	for _, item := range migrations {
		if item.version <= version {
			continue
		}
		if item.version != version+1 {
			return fmt.Errorf("migration sequence gap after v%d", version)
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin migration v%d (%s): %w", item.version, item.name, err)
		}
		if item.sql != "" {
			if _, err := tx.ExecContext(ctx, item.sql); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("apply migration v%d (%s): %w", item.version, item.name, err)
			}
		}
		if item.apply != nil {
			if err := item.apply(ctx, tx); err != nil {
				_ = tx.Rollback()
				return fmt.Errorf("apply migration v%d (%s): %w", item.version, item.name, err)
			}
		}
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", item.version)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration v%d (%s): %w", item.version, item.name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration v%d (%s): %w", item.version, item.name, err)
		}
		version = item.version
	}
	if version != currentSchemaVersion {
		return fmt.Errorf("database ended at v%d, want v%d", version, currentSchemaVersion)
	}
	return nil
}

func databaseVersion(ctx context.Context, db *sql.DB) (int, error) {
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return 0, fmt.Errorf("read database version: %w", err)
	}
	return version, nil
}

func databaseIsEmpty(ctx context.Context, db *sql.DB) (bool, error) {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("inspect database tables: %w", err)
	}
	return count == 0, nil
}
