// Package store is the SQLite-backed persistence layer for accounts,
// sessions, policies, and an operation log. We use modernc.org/sqlite (pure
// Go, no cgo) so cross-compiling for Windows / Linux / macOS stays trivial.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
)

// DB is the typed handle returned by Open.
type DB struct {
	*sql.DB
	credentialKey []byte
}

// Open initialises the database file and applies all pending versioned
// migrations. Unversioned databases are deliberately rejected; this release
// carries no historical compatibility code.
func Open(ctx context.Context, path string) (*DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)", path)
	sqldb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}
	if err := sqldb.PingContext(ctx); err != nil {
		_ = sqldb.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	if err := applyMigrations(ctx, sqldb); err != nil {
		_ = sqldb.Close()
		return nil, fmt.Errorf("schema migrations: %w", err)
	}
	credentialKey, err := loadOrCreateCredentialKey(path)
	if err != nil {
		_ = sqldb.Close()
		return nil, fmt.Errorf("credential key: %w", err)
	}
	return &DB{DB: sqldb, credentialKey: credentialKey}, nil
}

// Account is the row shape we hand to the API layer. Password is *not*
// returned to clients - callers go through GetCredentials when they need it
// to do a fresh login.
type Account struct {
	ID          int64
	UserID      int64
	Name        string
	Channel     string
	Username    string
	AID         int64
	GsIdx       int32
	WSURL       string
	LastLoginAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CreateAccount inserts a new account. Returns ErrAccountExists if name is
// already present. Channel must be a known babigame.Channel value; the
// daemon's CreateAccount handler validates it before invoking us.
func (d *DB) CreateAccount(ctx context.Context, userID int64, name, channel, username, password string) (*Account, error) {
	return d.CreateAccountWithPolicy(ctx, userID, name, channel, username, password, "")
}

// CreateAccountWithPolicy commits the account and initial policy atomically,
// before any runner can load defaults for the newly created account.
func (d *DB) CreateAccountWithPolicy(ctx context.Context, userID int64, name, channel, username, password, policyJSON string) (*Account, error) {
	if channel == "" {
		return nil, errors.New("CreateAccount: channel required")
	}
	now := time.Now().UTC()
	passwordEnc, err := d.encodePassword(password)
	if err != nil {
		return nil, fmt.Errorf("encode password: %w", err)
	}
	tx, err := d.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx,
		`INSERT INTO accounts(user_id, name, channel, username, password_enc, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		userID, name, channel, username, passwordEnc, now, now,
	)
	if err != nil {
		if isUniqueErr(err) {
			return nil, ErrAccountExists
		}
		return nil, fmt.Errorf("insert account: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("read inserted account id: %w", err)
	}
	if policyJSON != "" {
		if _, err := tx.ExecContext(ctx, `INSERT INTO account_policies(account_id, policy_json, updated_at) VALUES (?, ?, ?)`, id, policyJSON, now); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return d.GetAccountByID(ctx, id)
}

// UpdateAccountCredentials replaces the channel identity and encrypted grant
// used for fresh login. QR-bound channels call this after a successful
// re-authorization so an existing account never retries with a stale grant.
func (d *DB) UpdateAccountCredentials(ctx context.Context, id int64, username, password string) error {
	if id <= 0 || strings.TrimSpace(username) == "" || password == "" {
		return errors.New("UpdateAccountCredentials: id, username and password required")
	}
	passwordEnc, err := d.encodePassword(password)
	if err != nil {
		return fmt.Errorf("encode password: %w", err)
	}
	res, err := d.ExecContext(ctx,
		`UPDATE accounts SET username = ?, password_enc = ?, updated_at = ? WHERE id = ?`,
		username, passwordEnc, time.Now().UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("update account credentials: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("count updated account credentials: %w", err)
	}
	if n == 0 {
		return ErrAccountNotFound
	}
	return nil
}

// GetAccountByChannelUsername finds an already-bound channel identity for one
// dashboard user. It prevents repeated QR scans from creating duplicates.
func (d *DB) GetAccountByChannelUsername(ctx context.Context, userID int64, channel, username string) (*Account, error) {
	row := d.QueryRowContext(ctx,
		`SELECT id, user_id, name, channel, username, aid, gs_idx, ws_url, last_login_at, created_at, updated_at
         FROM accounts WHERE user_id = ? AND channel = ? AND username = ? ORDER BY id ASC LIMIT 1`,
		userID, channel, username)
	acc, err := scanAccount(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAccountNotFound
	}
	return acc, err
}

// UniqueAccountName returns base, or base with a numeric suffix, scoped to the
// owning user. excludeID lets callers keep an existing account's current name.
func (d *DB) UniqueAccountName(ctx context.Context, userID, excludeID int64, base string) (string, error) {
	base = strings.Join(strings.Fields(strings.TrimSpace(base)), " ")
	if base == "" {
		base = "账号"
	}
	for i := 0; i < 100; i++ {
		candidate := base
		if i > 0 {
			candidate = fmt.Sprintf("%s #%d", base, i+1)
		}
		acc, err := d.GetAccountByName(ctx, userID, candidate)
		if errors.Is(err, ErrAccountNotFound) {
			return candidate, nil
		}
		if err != nil {
			return "", err
		}
		if acc.ID == excludeID {
			return candidate, nil
		}
	}
	return fmt.Sprintf("%s #%d", base, time.Now().Unix()), nil
}

// RenameAccount updates the local display name for one account.
func (d *DB) RenameAccount(ctx context.Context, id int64, name string) (*Account, error) {
	name = strings.Join(strings.Fields(strings.TrimSpace(name)), " ")
	if id == 0 || name == "" {
		return nil, errors.New("RenameAccount: id and name required")
	}
	res, err := d.ExecContext(ctx,
		`UPDATE accounts SET name = ?, updated_at = ? WHERE id = ?`,
		name, time.Now().UTC(), id,
	)
	if err != nil {
		if isUniqueErr(err) {
			return nil, ErrAccountExists
		}
		return nil, fmt.Errorf("rename account: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("count renamed accounts: %w", err)
	}
	if n == 0 {
		return nil, ErrAccountNotFound
	}
	return d.GetAccountByID(ctx, id)
}

// DeleteAccount removes the account and its dependent rows by stable id.
func (d *DB) DeleteAccount(ctx context.Context, id int64) error {
	if id <= 0 {
		return errors.New("DeleteAccount: valid id required")
	}
	res, err := d.ExecContext(ctx, `DELETE FROM accounts WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("count deleted accounts: %w", err)
	}
	if n == 0 {
		return ErrAccountNotFound
	}
	return nil
}

// ListAccounts returns accounts ordered by id ascending. If userID > 0,
// only accounts belonging to that user are returned.
func (d *DB) ListAccounts(ctx context.Context, userID int64) ([]*Account, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if userID > 0 {
		rows, err = d.QueryContext(ctx,
			`SELECT id, user_id, name, channel, username, aid, gs_idx, ws_url, last_login_at, created_at, updated_at
			 FROM accounts WHERE user_id = ? ORDER BY id ASC`, userID)
	} else {
		rows, err = d.QueryContext(ctx,
			`SELECT id, user_id, name, channel, username, aid, gs_idx, ws_url, last_login_at, created_at, updated_at
			 FROM accounts ORDER BY id ASC`)
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*Account
	for rows.Next() {
		acc, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, acc)
	}
	return out, rows.Err()
}

// GetAccountByID resolves an account by primary key.
func (d *DB) GetAccountByID(ctx context.Context, id int64) (*Account, error) {
	row := d.QueryRowContext(ctx,
		`SELECT id, user_id, name, channel, username, aid, gs_idx, ws_url, last_login_at, created_at, updated_at
         FROM accounts WHERE id = ?`, id)
	acc, err := scanAccount(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAccountNotFound
	}
	return acc, err
}

// GetAccountByName resolves an account by its user-scoped display name. Names
// are presentation data; public APIs select accounts exclusively by id.
func (d *DB) GetAccountByName(ctx context.Context, userID int64, name string) (*Account, error) {
	if userID <= 0 || strings.TrimSpace(name) == "" {
		return nil, errors.New("GetAccountByName: user id and name required")
	}
	row := d.QueryRowContext(ctx,
		`SELECT id, user_id, name, channel, username, aid, gs_idx, ws_url, last_login_at, created_at, updated_at
         FROM accounts WHERE user_id = ? AND name = ?`,
		userID, name,
	)
	acc, err := scanAccount(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrAccountNotFound
	}
	return acc, err
}

// GetCredentials returns the cleartext username and password. Used by the
// runner to perform a fresh login when the cached session is missing or
// stale. Marked as a separate method so it never accidentally rides along
// with general queries.
func (d *DB) GetCredentials(ctx context.Context, id int64) (username, password string, err error) {
	var pwd string
	err = d.QueryRowContext(ctx, `SELECT username, password_enc FROM accounts WHERE id = ?`, id).Scan(&username, &pwd)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", ErrAccountNotFound
	}
	if err != nil {
		return "", "", err
	}
	password, err = d.decodePassword(pwd)
	return username, password, err
}

// UpdateLogin stamps the post-login fields onto the account row.
func (d *DB) UpdateLogin(ctx context.Context, id int64, aid int64, gsIdx int32, wsURL string, when time.Time) error {
	_, err := d.ExecContext(ctx,
		`UPDATE accounts SET aid = ?, gs_idx = ?, ws_url = ?, last_login_at = ?, updated_at = ? WHERE id = ?`,
		aid, gsIdx, wsURL, when.UTC(), time.Now().UTC(), id,
	)
	return err
}

// SaveSession encrypts and upserts the per-account session blob.
func (d *DB) SaveSession(ctx context.Context, accountID int64, payload []byte, expiresAt *time.Time) error {
	payloadEnc, err := d.encodeSession(accountID, payload)
	if err != nil {
		return fmt.Errorf("encrypt session: %w", err)
	}
	_, err = d.ExecContext(ctx,
		`INSERT INTO sessions(account_id, payload_enc, expires_at, updated_at)
         VALUES (?, ?, ?, ?)
		 ON CONFLICT(account_id) DO UPDATE SET payload_enc = excluded.payload_enc,
                                                 expires_at = excluded.expires_at,
                                                 updated_at = excluded.updated_at`,
		accountID, payloadEnc, expiresAt, time.Now().UTC(),
	)
	return err
}

// LoadSession returns the session blob for the account, or (nil, nil) when
// no session has been stored.
func (d *DB) LoadSession(ctx context.Context, accountID int64) ([]byte, error) {
	var payloadEnc string
	var expiresAt sql.NullTime
	err := d.QueryRowContext(ctx,
		`SELECT payload_enc, expires_at FROM sessions WHERE account_id = ?`, accountID).Scan(&payloadEnc, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if expiresAt.Valid && !time.Now().UTC().Before(expiresAt.Time.UTC()) {
		if err := d.DeleteSession(ctx, accountID); err != nil {
			return nil, fmt.Errorf("delete expired session: %w", err)
		}
		return nil, nil
	}
	payload, err := d.decodeSession(accountID, payloadEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt session: %w", err)
	}
	return payload, nil
}

// DeleteSession removes the cached server login/session blob for an account.
func (d *DB) DeleteSession(ctx context.Context, accountID int64) error {
	_, err := d.ExecContext(ctx, `DELETE FROM sessions WHERE account_id = ?`, accountID)
	return err
}

// SavePolicyJSON stores the full account policy as one protojson blob.
func (d *DB) SavePolicyJSON(ctx context.Context, accountID int64, policyJSON string) error {
	_, err := d.ExecContext(ctx,
		`INSERT INTO account_policies(account_id, policy_json, updated_at)
         VALUES (?, ?, ?)
         ON CONFLICT(account_id) DO UPDATE SET policy_json = excluded.policy_json,
                                                updated_at = excluded.updated_at`,
		accountID, policyJSON, time.Now().UTC(),
	)
	return err
}

// LoadPolicyJSON returns the full account policy blob. Empty string means the
// account has not stored a policy yet and callers should use current defaults.
func (d *DB) LoadPolicyJSON(ctx context.Context, accountID int64) (string, error) {
	var policyJSON string
	err := d.QueryRowContext(ctx,
		`SELECT policy_json FROM account_policies WHERE account_id = ?`, accountID).Scan(&policyJSON)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return policyJSON, err
}

// LogOperation appends an audit row.
func (d *DB) LogOperation(ctx context.Context, accountID int64, kind string, args, result any) error {
	argsJSON, _ := json.Marshal(args)
	resultJSON, _ := json.Marshal(result)
	_, err := d.ExecContext(ctx,
		`INSERT INTO operation_log(account_id, kind, args_json, result_json) VALUES (?, ?, ?, ?)`,
		accountID, kind, string(argsJSON), string(resultJSON),
	)
	return err
}

// AccountToProto converts a store row into the wire shape.
func AccountToProto(a *Account) *pb.Account {
	if a == nil {
		return nil
	}
	out := &pb.Account{
		Id:       a.ID,
		Name:     a.Name,
		Channel:  channelToProto(a.Channel),
		Username: a.Username,
		Aid:      a.AID,
		GsIdx:    a.GsIdx,
		WsUrl:    a.WSURL,
	}
	if a.LastLoginAt != nil {
		out.LastLoginAt = timeToProto(*a.LastLoginAt)
	}
	out.CreatedAt = timeToProto(a.CreatedAt)
	out.UpdatedAt = timeToProto(a.UpdatedAt)
	return out
}

// channelToProto maps the canonical lowercase string we keep in sqlite to
// the proto enum value. Unknown values fall back to UNSPECIFIED so callers
// can decide how to handle stale rows.
func channelToProto(s string) pb.Channel {
	switch s {
	case "ios":
		return pb.Channel_CHANNEL_IOS
	case "alipay":
		return pb.Channel_CHANNEL_ALIPAY
	default:
		return pb.Channel_CHANNEL_UNSPECIFIED
	}
}

// ChannelFromProto is the inverse - call from CreateAccount handlers.
func ChannelFromProto(c pb.Channel) string {
	switch c {
	case pb.Channel_CHANNEL_IOS:
		return "ios"
	case pb.Channel_CHANNEL_ALIPAY:
		return "alipay"
	default:
		return ""
	}
}

// CountAccountsByUser returns the number of game accounts owned by a user.
func (d *DB) CountAccountsByUser(ctx context.Context, userID int64) (int, error) {
	var n int
	err := d.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts WHERE user_id = ?`, userID).Scan(&n)
	return n, err
}
