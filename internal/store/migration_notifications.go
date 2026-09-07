package store

const notificationMigrationSQL = `
CREATE TABLE user_notifications (
    user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    enabled INTEGER NOT NULL DEFAULT 0 CHECK(enabled IN (0, 1)),
    endpoint_enc TEXT NOT NULL DEFAULT '',
    cooldown_minutes INTEGER NOT NULL DEFAULT 30 CHECK(cooldown_minutes BETWEEN 1 AND 1440),
    revision INTEGER NOT NULL DEFAULT 1,
    event_cursor INTEGER NOT NULL DEFAULT 0,
    last_test_ms INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE notification_incidents (
    user_id INTEGER NOT NULL REFERENCES user_notifications(user_id) ON DELETE CASCADE,
    account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    kind TEXT NOT NULL,
    opened_ms INTEGER NOT NULL,
    last_sent_ms INTEGER NOT NULL,
    occurrences INTEGER NOT NULL,
    severity INTEGER NOT NULL,
    PRIMARY KEY(user_id, account_id, kind)
);
CREATE TABLE notification_outbox (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    delivery_key TEXT NOT NULL UNIQUE,
    user_id INTEGER NOT NULL REFERENCES user_notifications(user_id) ON DELETE CASCADE,
    account_id INTEGER REFERENCES accounts(id) ON DELETE CASCADE,
    revision INTEGER NOT NULL,
    payload TEXT NOT NULL,
    title TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'sending', 'sent', 'failed', 'cancelled')),
    attempts INTEGER NOT NULL DEFAULT 0,
    next_ms INTEGER NOT NULL,
    lease_ms INTEGER NOT NULL DEFAULT 0,
    created_ms INTEGER NOT NULL,
    last_error TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_notification_outbox_due ON notification_outbox(status, next_ms);
CREATE INDEX idx_notification_outbox_user ON notification_outbox(user_id, id);
`
