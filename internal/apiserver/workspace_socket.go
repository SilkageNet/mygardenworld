package apiserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/auth"
	"github.com/SilkageNet/mygardenworld/internal/buildinfo"
	redeemsvc "github.com/SilkageNet/mygardenworld/internal/redeem"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/store"
	"github.com/coder/websocket"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	WorkspaceProtocolVersion uint32 = 1

	workspaceAuthTimeout      = 5 * time.Second
	workspaceWriteTimeout     = 10 * time.Second
	workspacePatchInterval    = 400 * time.Millisecond
	workspaceSafetyInterval   = 15 * time.Second
	workspaceHeartbeat        = 25 * time.Second
	workspaceAlipayPoll       = time.Second
	workspaceMaxClientMessage = 1 << 20
	workspaceCloseAuthExpired = websocket.StatusCode(4401)
)

type WorkspaceHandlerOptions struct {
	// Context owns every accepted workspace session. Passing the daemon context
	// ensures HTTP shutdown also terminates upgraded WebSocket connections.
	Context                context.Context
	OriginPatterns         []string
	InsecureAllowAnyOrigin bool
}

func (svc *Services) WorkspaceHandler(opts WorkspaceHandlerOptions) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		svc.serveWorkspace(w, r, opts)
	})
}

func (svc *Services) serveWorkspace(w http.ResponseWriter, r *http.Request, opts WorkspaceHandlerOptions) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns:     opts.OriginPatterns,
		InsecureSkipVerify: opts.InsecureAllowAnyOrigin,
		CompressionMode:    websocket.CompressionNoContextTakeover,
	})
	if err != nil {
		return
	}
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()
	conn.SetReadLimit(workspaceMaxClientMessage)

	parentCtx := opts.Context
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	baseCtx, cancel := context.WithCancel(parentCtx)
	defer cancel()
	authCtx, cancelAuth := context.WithTimeout(baseCtx, workspaceAuthTimeout)
	first, err := readWorkspaceFrame(authCtx, conn)
	cancelAuth()
	if err != nil {
		_ = conn.Close(workspaceCloseAuthExpired, "authentication required")
		return
	}
	open := first.GetOpen()
	if open == nil {
		_ = conn.Close(workspaceCloseAuthExpired, "open frame required")
		return
	}
	if open.GetProtocolVersion() != WorkspaceProtocolVersion {
		_ = writeWorkspaceFrame(baseCtx, conn, &pb.WorkspaceServerFrame{
			Sequence:  1,
			RequestId: first.GetRequestId(),
			Payload: &pb.WorkspaceServerFrame_Error{Error: &pb.WorkspaceError{
				Code:      "protocol_version_mismatch",
				Message:   fmt.Sprintf("workspace protocol %d required", WorkspaceProtocolVersion),
				Retryable: false,
			}},
		})
		_ = conn.Close(websocket.StatusPolicyViolation, "protocol version mismatch")
		return
	}
	identity, expiresAt, err := svc.authenticateWorkspace(baseCtx, open.GetAccessToken())
	if err != nil {
		_ = conn.Close(workspaceCloseAuthExpired, "authentication failed")
		return
	}
	ctx := auth.ContextWithIdentity(baseCtx, identity)
	session := newWorkspaceSession(ctx, svc, conn, identity, expiresAt)
	if err := session.run(first.GetRequestId(), open); err != nil && svc.Log != nil && !workspaceNormalClose(err) {
		svc.Log.Warn("workspace websocket ended", "user_id", identity.UserID, "error", err)
	}
}

func (svc *Services) authenticateWorkspace(ctx context.Context, token string) (*auth.Identity, time.Time, error) {
	if svc.JWT == nil || token == "" {
		return nil, time.Time{}, auth.ErrTokenInvalid
	}
	claims, err := svc.JWT.ValidateAccessToken(token)
	if err != nil {
		return nil, time.Time{}, err
	}
	user, err := svc.DB.GetUserByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, store.ErrUserNotFound) {
			return nil, time.Time{}, auth.ErrTokenInvalid
		}
		return nil, time.Time{}, err
	}
	if user.Status != "active" {
		return nil, time.Time{}, auth.ErrIdentityDisabled
	}
	var expiresAt time.Time
	if claims.ExpiresAt != nil {
		expiresAt = claims.ExpiresAt.Time
	}
	return &auth.Identity{UserID: user.ID, Role: user.Role}, expiresAt, nil
}

type workspaceSession struct {
	ctx       context.Context
	svc       *Services
	conn      *websocket.Conn
	identity  *auth.Identity
	expiresAt time.Time

	sequence          uint64
	selectedID        int64
	selectedAccount   *store.Account
	logHighWater      int64
	catchingUp        bool
	lastState         *pb.WorkspaceState
	lastStatuses      []*pb.AccountStatus
	allowedAccount    map[int64]struct{}
	pendingLogs       []*pb.Event
	dirtyState        bool
	dirtyStatuses     bool
	redeemSubscribed  bool
	redeemFilter      pb.AccountRedeemAttemptFilter
	redeemWindowLimit int32
	dirtyRedeem       bool
	alipayLoginID     string
	alipayPolling     bool
}

type alipayPollResult struct {
	loginID  string
	snapshot alipayLoginSnapshot
	err      error
}

func newWorkspaceSession(ctx context.Context, svc *Services, conn *websocket.Conn, identity *auth.Identity, expiresAt time.Time) *workspaceSession {
	return &workspaceSession{
		ctx:            ctx,
		svc:            svc,
		conn:           conn,
		identity:       identity,
		expiresAt:      expiresAt,
		allowedAccount: make(map[int64]struct{}),
	}
}

func (s *workspaceSession) run(openRequestID uint64, open *pb.OpenWorkspace) error {
	statuses, err := s.svc.accountStatuses(s.ctx)
	if err != nil {
		return err
	}
	s.setStatuses(statuses)
	if err := s.send(openRequestID, &pb.WorkspaceServerFrame_Ready{Ready: &pb.WorkspaceReady{
		ProtocolVersion:     WorkspaceProtocolVersion,
		ServerTime:          timestamppb.Now(),
		Accounts:            statuses,
		FeatureCapabilities: featureCapabilitiesProto(),
		HeartbeatSeconds:    int32(workspaceHeartbeat.Seconds()),
		ServerVersion:       buildinfo.GetVersion(),
	}}); err != nil {
		return err
	}
	var liveEvents <-chan runner.Event
	cancelEvents := func() {}
	if s.svc.Manager != nil {
		liveEvents, cancelEvents = s.svc.Manager.Bus().SubscribeLive(512)
	}
	defer cancelEvents()
	if open.GetSelectedAccountId() > 0 {
		if err := s.selectAccount(openRequestID, open.GetSelectedAccountId(), open.GetAfterLogId()); err != nil {
			if sendErr := s.sendError(openRequestID, "select_account_failed", err.Error(), false); sendErr != nil {
				return sendErr
			}
		}
	}

	clientFrames := make(chan *pb.WorkspaceClientFrame, 16)
	readErrors := make(chan error, 1)
	go readWorkspaceFrames(s.ctx, s.conn, clientFrames, readErrors)

	patchTicker := time.NewTicker(workspacePatchInterval)
	safetyTicker := time.NewTicker(workspaceSafetyInterval)
	heartbeatTicker := time.NewTicker(workspaceHeartbeat)
	alipayTicker := time.NewTicker(workspaceAlipayPoll)
	defer patchTicker.Stop()
	defer safetyTicker.Stop()
	defer heartbeatTicker.Stop()
	defer alipayTicker.Stop()

	expiry := time.NewTimer(tokenExpiryDelay(s.expiresAt))
	defer expiry.Stop()
	alipayResults := make(chan alipayPollResult, 1)

	for {
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case err := <-readErrors:
			return err
		case frame := <-clientFrames:
			if err := s.handleClientFrame(frame); err != nil {
				if sendErr := s.sendError(frame.GetRequestId(), "request_failed", err.Error(), false); sendErr != nil {
					return sendErr
				}
			}
		case event, ok := <-liveEvents:
			if !ok {
				liveEvents = nil
				continue
			}
			s.acceptEvent(event)
		case <-patchTicker.C:
			if err := s.flushChanges(); err != nil {
				return err
			}
			if s.catchingUp {
				if err := s.replayMissedLogs(); err != nil {
					return err
				}
			}
		case <-safetyTicker.C:
			if err := s.validateIdentity(); err != nil {
				_ = s.sendError(0, "authentication_expired", "登录状态已失效", false)
				_ = s.conn.Close(workspaceCloseAuthExpired, "authentication expired")
				return err
			}
			s.dirtyStatuses = true
			s.dirtyState = s.selectedID > 0
			if err := s.replayMissedLogs(); err != nil {
				return err
			}
		case <-heartbeatTicker.C:
			pingCtx, cancel := context.WithTimeout(s.ctx, workspaceWriteTimeout)
			err := s.conn.Ping(pingCtx)
			cancel()
			if err != nil {
				return err
			}
		case <-expiry.C:
			_ = s.sendError(0, "authentication_expired", "访问令牌已过期，正在重新连接", true)
			_ = s.conn.Close(workspaceCloseAuthExpired, "access token expired")
			return auth.ErrTokenExpired
		case <-alipayTicker.C:
			s.startAlipayPoll(alipayResults)
		case result := <-alipayResults:
			s.alipayPolling = false
			if result.loginID != s.alipayLoginID {
				continue
			}
			if err := s.sendAlipayProgress(result); err != nil {
				return err
			}
		}
	}
}

func (s *workspaceSession) handleClientFrame(frame *pb.WorkspaceClientFrame) error {
	switch payload := frame.GetPayload().(type) {
	case *pb.WorkspaceClientFrame_SelectAccount:
		return s.selectAccount(frame.GetRequestId(), payload.SelectAccount.GetAccountId(), payload.SelectAccount.GetAfterLogId())
	case *pb.WorkspaceClientFrame_Resync:
		if s.selectedID == 0 {
			return errors.New("no account selected")
		}
		return s.selectAccount(frame.GetRequestId(), s.selectedID, payload.Resync.GetAfterLogId())
	case *pb.WorkspaceClientFrame_LoadLogs:
		page, err := s.svc.workspaceLogPage(s.ctx, payload.LoadLogs.GetAccountId(), payload.LoadLogs.GetBeforeId(), payload.LoadLogs.GetLimit())
		if err != nil {
			return err
		}
		return s.send(frame.GetRequestId(), &pb.WorkspaceServerFrame_Logs{Logs: page})
	case *pb.WorkspaceClientFrame_LoadRedeemAttempts:
		page, err := s.svc.workspaceRedeemAttemptPage(
			s.ctx,
			payload.LoadRedeemAttempts.GetAccountId(),
			payload.LoadRedeemAttempts.GetBeforeId(),
			payload.LoadRedeemAttempts.GetLimit(),
			payload.LoadRedeemAttempts.GetFilter(),
		)
		if err != nil {
			return err
		}
		if page.GetAccountId() == s.selectedID {
			requestLimit := int32(normalizeWorkspaceRedeemPageLimit(payload.LoadRedeemAttempts.GetLimit()))
			if !s.redeemSubscribed || payload.LoadRedeemAttempts.GetBeforeId() <= 0 || s.redeemFilter != page.GetFilter() {
				s.redeemWindowLimit = requestLimit
			} else {
				s.redeemWindowLimit = min(
					int32(maxWorkspaceRedeemPageLimit),
					s.redeemWindowLimit+int32(len(page.GetEntries())),
				)
			}
			s.redeemSubscribed = true
			s.redeemFilter = page.GetFilter()
		}
		return s.send(frame.GetRequestId(), &pb.WorkspaceServerFrame_RedeemAttempts{RedeemAttempts: page})
	case *pb.WorkspaceClientFrame_WatchAlipayLogin:
		if payload.WatchAlipayLogin.GetLoginId() == "" {
			return errors.New("login_id required")
		}
		s.alipayLoginID = payload.WatchAlipayLogin.GetLoginId()
		return nil
	case *pb.WorkspaceClientFrame_Open:
		return errors.New("workspace is already open")
	default:
		return errors.New("unsupported workspace frame")
	}
}

func (s *workspaceSession) selectAccount(requestID uint64, accountID, afterLogID int64) error {
	if accountID <= 0 {
		return errors.New("valid account id required")
	}
	acc, err := s.svc.resolveAccount(s.ctx, accountID)
	if err != nil {
		return err
	}
	state, err := s.svc.workspaceStateForAccount(s.ctx, acc)
	if err != nil {
		return err
	}
	logs, highWater, err := s.svc.workspaceRecentLogsForAccount(s.ctx, acc, afterLogID)
	if err != nil {
		return err
	}
	s.selectedID = accountID
	s.selectedAccount = acc
	s.logHighWater = highWater
	s.catchingUp = logs.GetHasMoreAfter()
	s.lastState = state
	s.pendingLogs = nil
	s.dirtyState = false
	s.redeemSubscribed = false
	s.dirtyRedeem = false
	return s.send(requestID, &pb.WorkspaceServerFrame_Snapshot{Snapshot: &pb.WorkspaceSnapshot{
		State: state,
		Logs:  logs,
	}})
}

func (s *workspaceSession) acceptEvent(event runner.Event) {
	if _, ok := s.allowedAccount[event.AccountID]; !ok {
		return
	}
	if event.AccountID != s.selectedID {
		return
	}
	if event.Kind == redeemsvc.EventKindRedeemAttemptsUpdated {
		s.dirtyRedeem = s.redeemSubscribed
		return
	}
	s.dirtyState = true
	if s.catchingUp {
		return
	}
	if event.ID > 0 && event.ID <= s.logHighWater {
		return
	}
	s.pendingLogs = append(s.pendingLogs, eventToProto(event))
	if event.ID > s.logHighWater {
		s.logHighWater = event.ID
	}
}

func (s *workspaceSession) flushChanges() error {
	if len(s.pendingLogs) > 0 {
		logs := s.pendingLogs
		s.pendingLogs = nil
		for left, right := 0, len(logs)-1; left < right; left, right = left+1, right-1 {
			logs[left], logs[right] = logs[right], logs[left]
		}
		if err := s.send(0, &pb.WorkspaceServerFrame_Logs{Logs: &pb.WorkspaceLogPage{
			AccountId: s.selectedID,
			Kind:      pb.WorkspaceLogPageKind_WORKSPACE_LOG_PAGE_KIND_LIVE,
			Events:    logs,
		}}); err != nil {
			return err
		}
	}
	if s.dirtyStatuses {
		s.dirtyStatuses = false
		statuses, err := s.svc.accountStatuses(s.ctx)
		if err != nil {
			return err
		}
		if !proto.Equal(&pb.AccountStatusBatch{Accounts: s.lastStatuses}, &pb.AccountStatusBatch{Accounts: statuses}) {
			s.setStatuses(statuses)
			if err := s.send(0, &pb.WorkspaceServerFrame_AccountStatuses{AccountStatuses: &pb.AccountStatusBatch{Accounts: statuses}}); err != nil {
				return err
			}
		}
	}
	if s.dirtyRedeem && s.selectedID > 0 {
		s.dirtyRedeem = false
		page, err := s.svc.workspaceRedeemAttemptPage(
			s.ctx,
			s.selectedID,
			0,
			s.redeemWindowLimit,
			s.redeemFilter,
		)
		if err != nil {
			return err
		}
		if err := s.send(0, &pb.WorkspaceServerFrame_RedeemAttempts{RedeemAttempts: page}); err != nil {
			return err
		}
	}
	if s.dirtyState && s.selectedID > 0 {
		s.dirtyState = false
		next, err := s.svc.workspaceStateForAccount(s.ctx, s.selectedAccount)
		if err != nil {
			return err
		}
		patch := buildWorkspacePatch(s.lastState, next)
		s.lastState = next
		if patch != nil {
			return s.send(0, &pb.WorkspaceServerFrame_Patch{Patch: patch})
		}
	}
	return nil
}

func (s *workspaceSession) replayMissedLogs() error {
	if s.selectedID == 0 || s.selectedAccount == nil {
		return nil
	}
	logs, highWater, err := s.svc.workspaceRecentLogsForAccount(s.ctx, s.selectedAccount, s.logHighWater)
	if err != nil {
		return err
	}
	if len(logs.GetEvents()) == 0 {
		s.catchingUp = false
		return nil
	}
	s.logHighWater = highWater
	s.catchingUp = logs.GetHasMoreAfter()
	return s.send(0, &pb.WorkspaceServerFrame_Logs{Logs: logs})
}

func (s *workspaceSession) setStatuses(statuses []*pb.AccountStatus) {
	s.lastStatuses = statuses
	allowed := make(map[int64]struct{}, len(statuses))
	for _, status := range statuses {
		allowed[status.GetAccountId()] = struct{}{}
	}
	s.allowedAccount = allowed
}

func (s *workspaceSession) validateIdentity() error {
	user, err := s.svc.DB.GetUserByID(s.ctx, s.identity.UserID)
	if err != nil {
		return err
	}
	if user.Status != "active" || user.Role != s.identity.Role {
		return auth.ErrIdentityDisabled
	}
	return nil
}

func (s *workspaceSession) startAlipayPoll(results chan<- alipayPollResult) {
	if s.alipayLoginID == "" || s.alipayPolling {
		return
	}
	s.alipayPolling = true
	loginID := s.alipayLoginID
	go func() {
		snapshot, err := s.svc.pollAlipayLogin(s.ctx, loginID)
		select {
		case results <- alipayPollResult{loginID: loginID, snapshot: snapshot, err: err}:
		case <-s.ctx.Done():
		}
	}()
}

func (s *workspaceSession) sendAlipayProgress(result alipayPollResult) error {
	if result.err != nil {
		s.alipayLoginID = ""
		return s.sendError(0, "alipay_login_failed", result.err.Error(), false)
	}
	snapshot := result.snapshot
	if snapshot.Status == pb.AlipayLoginStatus_ALIPAY_LOGIN_STATUS_COMPLETE ||
		snapshot.Status == pb.AlipayLoginStatus_ALIPAY_LOGIN_STATUS_FAILED ||
		snapshot.Status == pb.AlipayLoginStatus_ALIPAY_LOGIN_STATUS_EXPIRED {
		s.alipayLoginID = ""
	}
	if snapshot.Status == pb.AlipayLoginStatus_ALIPAY_LOGIN_STATUS_COMPLETE {
		s.dirtyStatuses = true
	}
	return s.send(0, &pb.WorkspaceServerFrame_AlipayLogin{AlipayLogin: &pb.AlipayLoginProgress{
		LoginId:    result.loginID,
		Status:     snapshot.Status,
		Account:    store.AccountToProto(snapshot.Account),
		LoginError: snapshot.Error,
		ExpiresAt:  alipayTimestampOrNil(snapshot.ExpiresAt),
	}})
}

func (s *workspaceSession) sendError(requestID uint64, code, message string, retryable bool) error {
	return s.send(requestID, &pb.WorkspaceServerFrame_Error{Error: &pb.WorkspaceError{
		Code:      code,
		Message:   message,
		Retryable: retryable,
	}})
}

func (s *workspaceSession) send(requestID uint64, payload any) error {
	frame := &pb.WorkspaceServerFrame{RequestId: requestID}
	switch value := any(payload).(type) {
	case *pb.WorkspaceServerFrame_Ready:
		frame.Payload = value
	case *pb.WorkspaceServerFrame_AccountStatuses:
		frame.Payload = value
	case *pb.WorkspaceServerFrame_Snapshot:
		frame.Payload = value
	case *pb.WorkspaceServerFrame_Patch:
		frame.Payload = value
	case *pb.WorkspaceServerFrame_Logs:
		frame.Payload = value
	case *pb.WorkspaceServerFrame_AlipayLogin:
		frame.Payload = value
	case *pb.WorkspaceServerFrame_RedeemAttempts:
		frame.Payload = value
	case *pb.WorkspaceServerFrame_Error:
		frame.Payload = value
	default:
		return errors.New("unsupported workspace response")
	}
	s.sequence++
	frame.Sequence = s.sequence
	return writeWorkspaceFrame(s.ctx, s.conn, frame)
}

func readWorkspaceFrames(ctx context.Context, conn *websocket.Conn, frames chan<- *pb.WorkspaceClientFrame, errs chan<- error) {
	for {
		frame, err := readWorkspaceFrame(ctx, conn)
		if err != nil {
			select {
			case errs <- err:
			default:
			}
			return
		}
		select {
		case frames <- frame:
		case <-ctx.Done():
			return
		}
	}
}

func readWorkspaceFrame(ctx context.Context, conn *websocket.Conn) (*pb.WorkspaceClientFrame, error) {
	typ, data, err := conn.Read(ctx)
	if err != nil {
		return nil, err
	}
	if typ != websocket.MessageBinary {
		return nil, errors.New("workspace frames must be protobuf binary messages")
	}
	frame := new(pb.WorkspaceClientFrame)
	if err := proto.Unmarshal(data, frame); err != nil {
		return nil, fmt.Errorf("decode workspace frame: %w", err)
	}
	return frame, nil
}

func writeWorkspaceFrame(ctx context.Context, conn *websocket.Conn, frame *pb.WorkspaceServerFrame) error {
	data, err := proto.Marshal(frame)
	if err != nil {
		return fmt.Errorf("encode workspace frame: %w", err)
	}
	writeCtx, cancel := context.WithTimeout(ctx, workspaceWriteTimeout)
	defer cancel()
	return conn.Write(writeCtx, websocket.MessageBinary, data)
}

func tokenExpiryDelay(expiresAt time.Time) time.Duration {
	if expiresAt.IsZero() {
		return auth.AccessTokenDuration
	}
	delay := time.Until(expiresAt)
	if delay <= 0 {
		return time.Nanosecond
	}
	return delay
}

func workspaceNormalClose(err error) bool {
	if errors.Is(err, auth.ErrTokenExpired) {
		return true
	}
	status := websocket.CloseStatus(err)
	return status == websocket.StatusNormalClosure || status == websocket.StatusGoingAway
}
