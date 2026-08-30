// Package redeem owns the public redeem-code registry, local validation,
// account attempts, source polling, and bounded federation between deployments.
package redeem

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1/mygardenworldv1connect"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/store"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	MaxSubmitBatch                 = 20
	EventKindRedeemAttemptsUpdated = "redeem_attempts_updated"
	maxSourcePages                 = 20
	maxSourceBody                  = 1 << 20
)

type Submission struct {
	Code               string
	Channel            string
	ExpiresAt          *time.Time
	ReportedValidation string
	OriginInstanceID   string
}

type SubmitResult struct {
	Code    *store.RedeemCode
	Created bool
	Err     error
}

type Service struct {
	db         *store.DB
	manager    *runner.Manager
	log        *slog.Logger
	instanceID string
	wake       chan struct{}
	syncMu     sync.Mutex
}

func NewService(ctx context.Context, db *store.DB, manager *runner.Manager, log *slog.Logger) (*Service, error) {
	instanceID, err := db.RedeemInstanceID(ctx)
	if err != nil {
		return nil, err
	}
	if err := db.RecoverRedeemWork(ctx); err != nil {
		return nil, fmt.Errorf("recover redeem work: %w", err)
	}
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		db:         db,
		manager:    manager,
		log:        log.With("component", "redeem_exchange"),
		instanceID: instanceID,
		wake:       make(chan struct{}, 1),
	}, nil
}

func (s *Service) InstanceID() string { return s.instanceID }

func (s *Service) Run(ctx context.Context) {
	var wg sync.WaitGroup
	for _, worker := range []func(context.Context){s.runAttempts, s.runSources, s.runOutbox} {
		wg.Add(1)
		go func(run func(context.Context)) {
			defer wg.Done()
			run(ctx)
		}(worker)
	}
	s.signal()
	wg.Wait()
}

func (s *Service) Submit(ctx context.Context, entries []Submission, senderInstanceID, sourceKey string) []SubmitResult {
	return s.submit(ctx, entries, senderInstanceID, sourceKey, nil)
}

func (s *Service) submit(ctx context.Context, entries []Submission, senderInstanceID, sourceKey string, trustedSourceID *int64) []SubmitResult {
	if len(entries) > MaxSubmitBatch {
		entries = entries[:MaxSubmitBatch]
	}
	senderInstanceID = strings.TrimSpace(senderInstanceID)
	if senderInstanceID != "" && senderInstanceID == s.instanceID {
		out := make([]SubmitResult, len(entries))
		for index := range out {
			out[index].Err = errors.New("submission originated from this instance")
		}
		return out
	}
	if senderInstanceID != "" {
		if sourceKey == "" {
			sourceKey = "peer:" + senderInstanceID
		}
	}
	if sourceKey == "" {
		sourceKey = "public"
	}
	now := time.Now().UTC()
	out := make([]SubmitResult, 0, len(entries))
	for _, entry := range entries {
		entry.Code = store.NormalizeRedeemCode(entry.Code)
		if entry.ExpiresAt != nil && !entry.ExpiresAt.After(now) {
			out = append(out, SubmitResult{Err: errors.New("redeem code expiry must be in the future")})
			continue
		}
		origin := strings.TrimSpace(entry.OriginInstanceID)
		if origin == "" {
			origin = senderInstanceID
		}
		if origin == "" {
			origin = s.instanceID
		}
		item, created, err := s.db.UpsertRedeemCode(ctx, store.RedeemCodeInput{
			Code:               entry.Code,
			Channel:            entry.Channel,
			ExpiresAt:          entry.ExpiresAt,
			ReportedValidation: entry.ReportedValidation,
			OriginInstanceID:   origin,
			SourceID:           trustedSourceID,
			SourceKey:          sourceKey,
		})
		out = append(out, SubmitResult{Code: item, Created: created, Err: err})
	}
	if err := s.db.EnsureRedeemAttempts(ctx); err != nil {
		s.log.Error("ensure redeem attempts after submit", "err", err)
	}
	s.signal()
	return out
}

func (s *Service) List(ctx context.Context, cursor string, limit int, includeExpired, onlyPropagatable bool, channels []string) ([]*store.RedeemCode, string, error) {
	after, err := store.ParseRedeemCursor(cursor)
	if err != nil {
		return nil, cursor, err
	}
	entries, next, err := s.db.ListRedeemCodes(ctx, after, limit, includeExpired, onlyPropagatable, channels)
	if err != nil {
		return nil, cursor, err
	}
	return entries, strconv.FormatInt(next, 10), nil
}

func (s *Service) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *Service) runAttempts(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-s.wake:
		}
		if err := s.db.EnsureRedeemAttempts(ctx); err != nil {
			s.log.Error("ensure redeem attempts", "err", err)
			continue
		}
		attempt, err := s.db.NextRedeemAttempt(ctx)
		if err != nil {
			s.log.Error("claim redeem attempt", "err", err)
			continue
		}
		if attempt == nil {
			continue
		}
		s.processAttempt(ctx, attempt)
		s.signal()
	}
}

func (s *Service) processAttempt(ctx context.Context, attempt *store.RedeemAttempt) {
	resultStatus := store.RedeemValidationRetryable
	retryAt := time.Now().UTC().Add(5 * time.Minute)

	r := s.manager.Get(attempt.AccountID)
	if r == nil || !r.Connected() {
		startCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
		started, err := s.manager.Start(startCtx, attempt.AccountID)
		cancel()
		if err != nil {
			message := err.Error()
			s.completeAttempt(ctx, attempt, resultStatus, message, &retryAt)
			return
		}
		r = started
	}
	attemptCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	result, err := r.RedeemCode(attemptCtx, attempt.Code)
	cancel()
	if err != nil {
		message := err.Error()
		s.completeAttempt(ctx, attempt, resultStatus, message, &retryAt)
		return
	}
	message := strings.TrimSpace(result.Message)
	if message == "" {
		message = redeemOutcomeMessage(result)
	}
	switch result.Outcome {
	case runner.RedeemOutcomeSuccess:
		resultStatus = store.RedeemValidationSuccess
		retryAt = time.Time{}
	case runner.RedeemOutcomeAlreadyRedeemed:
		resultStatus = store.RedeemValidationAlreadyRedeemed
		retryAt = time.Time{}
	case runner.RedeemOutcomeExpired:
		resultStatus = store.RedeemValidationExpired
		retryAt = time.Time{}
	case runner.RedeemOutcomeInvalid:
		resultStatus = store.RedeemValidationInvalid
		retryAt = time.Time{}
	case runner.RedeemOutcomeUnknown:
		resultStatus = store.RedeemValidationUnknown
		retryAt = time.Time{}
	default:
		resultStatus = store.RedeemValidationRetryable
		if result.MessageCode == 337 {
			retryAt = time.Now().UTC().Add(6 * time.Hour)
		}
	}
	var retry *time.Time
	if !retryAt.IsZero() {
		retry = &retryAt
	}
	s.completeAttempt(ctx, attempt, resultStatus, message, retry)
}

func (s *Service) completeAttempt(
	ctx context.Context,
	attempt *store.RedeemAttempt,
	status, message string,
	retryAt *time.Time,
) {
	if err := s.db.CompleteRedeemAttempt(ctx, attempt.ID, status, message, retryAt); err != nil {
		s.log.Error("complete redeem attempt", "account_id", attempt.AccountID, "code", attempt.Fingerprint, "err", err)
		return
	}
	if s.manager == nil || s.manager.Bus() == nil {
		return
	}
	s.manager.Bus().PublishTransient(runner.Event{
		TS:          time.Now().UTC(),
		AccountID:   attempt.AccountID,
		AccountName: attempt.AccountName,
		Kind:        EventKindRedeemAttemptsUpdated,
	})
}

func redeemOutcomeMessage(result runner.RedeemResult) string {
	switch result.Outcome {
	case runner.RedeemOutcomeSuccess:
		return "兑换成功"
	case runner.RedeemOutcomeAlreadyRedeemed:
		return "该账号已经兑换"
	case runner.RedeemOutcomeExpired:
		return "兑换码已经过期"
	case runner.RedeemOutcomeInvalid:
		return "无效兑换码"
	case runner.RedeemOutcomeRetryable:
		return "暂时无法验证，等待重试"
	default:
		return "无法识别游戏返回结果"
	}
}

func (s *Service) runSources(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		sources, err := s.db.DueRedeemSources(ctx, time.Now())
		if err != nil {
			s.log.Error("list due redeem sources", "err", err)
			continue
		}
		for _, source := range sources {
			if err := s.SyncSource(ctx, source.ID); err != nil {
				s.log.Warn("sync redeem source", "source", source.Name, "err", err)
			}
		}
	}
}

func (s *Service) SyncSource(ctx context.Context, sourceID int64) error {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	source, err := s.db.GetRedeemSource(ctx, sourceID)
	if err != nil {
		return err
	}
	if !source.Enabled {
		return errors.New("redeem source is disabled")
	}
	var cursor, remoteID string
	switch source.Type {
	case store.RedeemSourceMyGardenWorld:
		cursor, remoteID, err = s.syncNativeSource(ctx, source)
	case store.RedeemSourceCustomHTTP:
		err = s.syncCustomSource(ctx, source)
	default:
		err = errors.New("unsupported redeem source type")
	}
	if updateErr := s.db.UpdateRedeemSourceSync(ctx, source.ID, cursor, remoteID, errorText(err)); updateErr != nil && err == nil {
		err = updateErr
	}
	return err
}

func (s *Service) syncNativeSource(ctx context.Context, source *store.RedeemSource) (string, string, error) {
	baseURL, err := validateSourceURL(source.BaseURL, true)
	if err != nil {
		return source.Cursor, source.RemoteInstanceID, err
	}
	client := mygardenworldv1connect.NewRedeemExchangeServiceClient(
		newSourceHTTPClient(true, 20*time.Second), strings.TrimRight(baseURL.String(), "/"),
	)
	info, err := client.GetExchangeInfo(ctx, connect.NewRequest(&pb.GetExchangeInfoRequest{}))
	if err != nil {
		return source.Cursor, source.RemoteInstanceID, err
	}
	remoteID := strings.TrimSpace(info.Msg.GetInstanceId())
	if remoteID == "" {
		return source.Cursor, source.RemoteInstanceID, errors.New("remote redeem instance id missing")
	}
	if remoteID == s.instanceID {
		return source.Cursor, remoteID, errors.New("redeem source points to this instance")
	}
	cursor := source.Cursor
	if source.RemoteInstanceID != remoteID {
		// A restored or replaced deployment owns an independent revision space.
		cursor = ""
	}
	// Persist the peer identity before importing its first page so observations
	// are attributed to the configured source immediately.
	if err := s.db.UpdateRedeemSourceSync(ctx, source.ID, cursor, remoteID, ""); err != nil {
		return source.Cursor, source.RemoteInstanceID, err
	}
	for page := 0; page < maxSourcePages; page++ {
		resp, err := client.ListRedeemCodes(ctx, connect.NewRequest(&pb.ListRedeemCodesRequest{
			Cursor: cursor, PageSize: 200, OnlyPropagatable: true,
		}))
		if err != nil {
			return cursor, remoteID, err
		}
		for _, entry := range resp.Msg.GetEntries() {
			if entry.GetValidation() != pb.RedeemValidation_REDEEM_VALIDATION_SUCCESS &&
				entry.GetValidation() != pb.RedeemValidation_REDEEM_VALIDATION_ALREADY_REDEEMED {
				// Treat the remote filter as a request, not a trust boundary. Older
				// or faulty peers must not make unverified data transitively spread.
				continue
			}
			expiresAt := protoTime(entry.GetExpiresAt())
			if expiresAt != nil && !expiresAt.After(time.Now().UTC()) {
				continue
			}
			s.submit(ctx, []Submission{{
				Code:               entry.GetCode(),
				Channel:            ChannelFromProto(entry.GetChannel()),
				ExpiresAt:          expiresAt,
				ReportedValidation: ValidationFromProto(entry.GetValidation()),
				OriginInstanceID:   entry.GetOriginInstanceId(),
			}}, remoteID, fmt.Sprintf("source:%d:%s", source.ID, remoteID), &source.ID)
		}
		next := strings.TrimSpace(resp.Msg.GetNextCursor())
		if next == "" || next == cursor || len(resp.Msg.GetEntries()) == 0 {
			break
		}
		cursor = next
		if len(resp.Msg.GetEntries()) < 200 {
			break
		}
	}
	return cursor, remoteID, nil
}

type customParserConfig struct {
	Type              string `json:"type"`
	ItemsField        string `json:"items_field"`
	CodeField         string `json:"code_field"`
	TimeField         string `json:"time_field"`
	TimeFormat        string `json:"time_format"`
	ExpiresField      string `json:"expires_field"`
	ExpiresFormat     string `json:"expires_format"`
	DefaultTTLSeconds int64  `json:"default_ttl_seconds"`
	Permanent         bool   `json:"permanent"`
}

func (s *Service) syncCustomSource(ctx context.Context, source *store.RedeemSource) error {
	endpoint, err := validateSourceURL(source.BaseURL, false)
	if err != nil {
		return err
	}
	cfg, err := parseCustomParserConfig(source.ParserConfigJSON)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json, text/plain;q=0.9, */*;q=0.1")
	resp, err := newSourceHTTPClient(false, 15*time.Second).Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("source returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxSourceBody+1))
	if err != nil {
		return fmt.Errorf("read source response: %w", err)
	}
	if len(body) > maxSourceBody {
		return fmt.Errorf("source response exceeds %d bytes", maxSourceBody)
	}
	var root any
	if err := json.Unmarshal(body, &root); err != nil {
		return fmt.Errorf("decode source JSON: %w", err)
	}
	if cfg.ItemsField != "" {
		root = valueAtPath(root, cfg.ItemsField)
	}
	items, ok := root.([]any)
	if !ok {
		return errors.New("custom source JSON is not an array")
	}
	now := time.Now().UTC()
	for _, raw := range items {
		code := strings.TrimSpace(fmt.Sprint(valueAtPath(raw, cfg.CodeField)))
		if code == "" || code == "<nil>" {
			continue
		}
		var expiresAt *time.Time
		switch {
		case cfg.Permanent:
		case cfg.ExpiresField != "":
			parsed, parseErr := parseSourceTime(valueAtPath(raw, cfg.ExpiresField), cfg.ExpiresFormat)
			if parseErr != nil {
				continue
			}
			expiresAt = &parsed
		case cfg.DefaultTTLSeconds > 0:
			base := now
			if cfg.TimeField != "" {
				parsed, parseErr := parseSourceTime(valueAtPath(raw, cfg.TimeField), cfg.TimeFormat)
				if parseErr != nil {
					continue
				}
				base = parsed
			}
			expires := base.Add(time.Duration(cfg.DefaultTTLSeconds) * time.Second)
			expiresAt = &expires
		}
		if expiresAt != nil && !expiresAt.After(now) {
			continue
		}
		id := source.ID
		_, _, err := s.db.UpsertRedeemCode(ctx, store.RedeemCodeInput{
			Code: code, Channel: source.Channel, ExpiresAt: expiresAt,
			SourceID: &id, SourceKey: fmt.Sprintf("source:%d", source.ID), OriginInstanceID: s.instanceID,
		})
		if err != nil {
			s.log.Warn("reject custom source code", "source", source.Name, "err", err)
		}
	}
	if err := s.db.EnsureRedeemAttempts(ctx); err != nil {
		return err
	}
	s.signal()
	return nil
}

func (s *Service) runOutbox(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		item, err := s.db.NextRedeemOutbox(ctx)
		if err != nil {
			s.log.Error("claim redeem outbox", "err", err)
			continue
		}
		if item == nil {
			continue
		}
		s.deliverOutbox(ctx, item)
	}
}

func (s *Service) deliverOutbox(ctx context.Context, item *store.RedeemOutboxItem) {
	endpoint, err := validateSourceURL(item.BaseURL, true)
	if err == nil {
		client := mygardenworldv1connect.NewRedeemExchangeServiceClient(
			newSourceHTTPClient(true, 20*time.Second), strings.TrimRight(endpoint.String(), "/"),
		)
		entry := &pb.RedeemCodeSubmission{
			Code:               item.Code.Code,
			Channel:            ChannelToProto(item.Code.Channel),
			Permanent:          item.Code.ExpiresAt == nil,
			ReportedValidation: ValidationToProto(item.Code.Validation),
			OriginInstanceId:   item.Code.OriginInstanceID,
		}
		if item.Code.ExpiresAt != nil {
			entry.ExpiresAt = timestamppb.New(*item.Code.ExpiresAt)
		}
		resp, callErr := client.SubmitRedeemCodes(ctx, connect.NewRequest(&pb.SubmitRedeemCodesRequest{
			Entries: []*pb.RedeemCodeSubmission{entry}, SenderInstanceId: s.instanceID,
		}))
		if callErr != nil {
			err = callErr
		} else if len(resp.Msg.GetResults()) == 0 {
			err = errors.New("remote source returned no submission result")
		} else if result := resp.Msg.GetResults()[0]; result.GetDisposition() == pb.RedeemSubmitDisposition_REDEEM_SUBMIT_DISPOSITION_REJECTED {
			// A semantic rejection is terminal for this target; retrying cannot help.
			_ = s.db.CompleteRedeemOutbox(ctx, item.ID, true, nil, result.GetMessage())
			return
		} else {
			_ = s.db.CompleteRedeemOutbox(ctx, item.ID, true, nil, "")
			return
		}
	}
	delay := time.Minute << min(item.AttemptCount, 6)
	retryAt := time.Now().UTC().Add(delay)
	if item.Code.ExpiresAt != nil && retryAt.After(*item.Code.ExpiresAt) {
		_ = s.db.CompleteRedeemOutbox(ctx, item.ID, true, nil, "code expired before delivery: "+errorText(err))
		return
	}
	_ = s.db.CompleteRedeemOutbox(ctx, item.ID, false, &retryAt, errorText(err))
}

func valueAtPath(value any, path string) any {
	current := value
	for _, part := range strings.Split(strings.TrimSpace(path), ".") {
		if part == "" {
			continue
		}
		object, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = object[part]
	}
	return current
}

func parseSourceTime(value any, format string) (time.Time, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	switch format {
	case "unix", "unix_seconds":
		seconds, err := numericValue(value)
		return time.Unix(seconds, 0).UTC(), err
	case "unix_ms", "unix_milliseconds":
		millis, err := numericValue(value)
		return time.UnixMilli(millis).UTC(), err
	case "rfc3339", "":
		return time.Parse(time.RFC3339, strings.TrimSpace(fmt.Sprint(value)))
	default:
		return time.Time{}, errors.New("unsupported source time format")
	}
}

func numericValue(value any) (int64, error) {
	switch typed := value.(type) {
	case float64:
		return int64(typed), nil
	case json.Number:
		return typed.Int64()
	default:
		return strconv.ParseInt(strings.TrimSpace(fmt.Sprint(value)), 10, 64)
	}
}

func validateSourceURL(raw string, allowPrivate bool) (*url.URL, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Hostname() == "" {
		return nil, errors.New("valid source URL required")
	}
	if parsed.User != nil || parsed.Fragment != "" {
		return nil, errors.New("source URL must not contain credentials or fragments")
	}
	schemeAllowed := parsed.Scheme == "https" || (allowPrivate && parsed.Scheme == "http")
	if !schemeAllowed {
		return nil, errors.New("custom sources require HTTPS")
	}
	if !allowPrivate {
		lookupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ips, err := net.DefaultResolver.LookupIPAddr(lookupCtx, parsed.Hostname())
		if err != nil {
			return nil, fmt.Errorf("resolve source host: %w", err)
		}
		for _, item := range ips {
			if item.IP.IsPrivate() || item.IP.IsLoopback() || item.IP.IsLinkLocalUnicast() || item.IP.IsUnspecified() {
				return nil, errors.New("custom source resolves to a private or local address")
			}
		}
	}
	return parsed, nil
}

// ValidateSourceEndpoint validates an administrator-provided endpoint before
// it is persisted. Native peers may be local HTTP services; arbitrary custom
// sources must use HTTPS and resolve only to public addresses.
func ValidateSourceEndpoint(raw string, native bool) error {
	_, err := validateSourceURL(raw, native)
	return err
}

// ValidateCustomParserConfig rejects unsupported custom-source schemas before
// they are saved. Every source must declare exactly one expiry policy.
func ValidateCustomParserConfig(raw string) error {
	_, err := parseCustomParserConfig(raw)
	return err
}

func parseCustomParserConfig(raw string) (customParserConfig, error) {
	var cfg customParserConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return cfg, fmt.Errorf("parse source config: %w", err)
	}
	if cfg.Type == "" {
		cfg.Type = "json_array"
	}
	if cfg.Type != "json_array" || strings.TrimSpace(cfg.CodeField) == "" {
		return cfg, errors.New("custom source requires json_array and code_field")
	}
	if cfg.DefaultTTLSeconds < 0 {
		return cfg, errors.New("default_ttl_seconds must be non-negative")
	}
	expirationRules := 0
	if cfg.Permanent {
		expirationRules++
	}
	if strings.TrimSpace(cfg.ExpiresField) != "" {
		expirationRules++
	}
	if cfg.DefaultTTLSeconds > 0 {
		expirationRules++
	}
	if expirationRules != 1 {
		return cfg, errors.New("custom source requires exactly one expiration rule: permanent, expires_field, or default_ttl_seconds")
	}
	for name, format := range map[string]string{
		"time_format": cfg.TimeFormat, "expires_format": cfg.ExpiresFormat,
	} {
		if format == "" {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(format)) {
		case "rfc3339", "unix", "unix_seconds", "unix_ms", "unix_milliseconds":
		default:
			return cfg, fmt.Errorf("unsupported %s", name)
		}
	}
	return cfg, nil
}

func newSourceHTTPClient(allowPrivate bool, timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if !allowPrivate {
		dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
		transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			for _, item := range ips {
				if item.IP.IsPrivate() || item.IP.IsLoopback() || item.IP.IsLinkLocalUnicast() || item.IP.IsUnspecified() {
					return nil, errors.New("custom source resolves to a private or local address")
				}
			}
			var lastErr error
			for _, item := range ips {
				conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(item.IP.String(), port))
				if dialErr == nil {
					return conn, nil
				}
				lastErr = dialErr
			}
			if lastErr == nil {
				lastErr = errors.New("source host has no addresses")
			}
			return nil, lastErr
		}
	}
	return &http.Client{
		Timeout:       timeout,
		Transport:     transport,
		CheckRedirect: safeRedirectPolicy(allowPrivate),
	}
}

func safeRedirectPolicy(allowPrivate bool) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, _ []*http.Request) error {
		_, err := validateSourceURL(req.URL.String(), allowPrivate)
		return err
	}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return strings.TrimSpace(err.Error())
}

func protoTime(value *timestamppb.Timestamp) *time.Time {
	if value == nil || !value.IsValid() {
		return nil
	}
	t := value.AsTime().UTC()
	return &t
}

func ChannelFromProto(channel pb.Channel) string {
	return store.ChannelFromProto(channel)
}

func ChannelToProto(channel string) pb.Channel {
	switch channel {
	case "ios":
		return pb.Channel_CHANNEL_IOS
	case "alipay":
		return pb.Channel_CHANNEL_ALIPAY
	default:
		return pb.Channel_CHANNEL_UNSPECIFIED
	}
}

func ValidationFromProto(value pb.RedeemValidation) string {
	switch value {
	case pb.RedeemValidation_REDEEM_VALIDATION_SUCCESS:
		return store.RedeemValidationSuccess
	case pb.RedeemValidation_REDEEM_VALIDATION_ALREADY_REDEEMED:
		return store.RedeemValidationAlreadyRedeemed
	case pb.RedeemValidation_REDEEM_VALIDATION_EXPIRED:
		return store.RedeemValidationExpired
	case pb.RedeemValidation_REDEEM_VALIDATION_INVALID:
		return store.RedeemValidationInvalid
	case pb.RedeemValidation_REDEEM_VALIDATION_RETRYABLE:
		return store.RedeemValidationRetryable
	case pb.RedeemValidation_REDEEM_VALIDATION_UNKNOWN:
		return store.RedeemValidationUnknown
	default:
		return store.RedeemValidationPending
	}
}

func ValidationToProto(value string) pb.RedeemValidation {
	switch value {
	case store.RedeemValidationSuccess:
		return pb.RedeemValidation_REDEEM_VALIDATION_SUCCESS
	case store.RedeemValidationAlreadyRedeemed:
		return pb.RedeemValidation_REDEEM_VALIDATION_ALREADY_REDEEMED
	case store.RedeemValidationExpired:
		return pb.RedeemValidation_REDEEM_VALIDATION_EXPIRED
	case store.RedeemValidationInvalid:
		return pb.RedeemValidation_REDEEM_VALIDATION_INVALID
	case store.RedeemValidationRetryable:
		return pb.RedeemValidation_REDEEM_VALIDATION_RETRYABLE
	case store.RedeemValidationUnknown:
		return pb.RedeemValidation_REDEEM_VALIDATION_UNKNOWN
	default:
		return pb.RedeemValidation_REDEEM_VALIDATION_PENDING
	}
}
