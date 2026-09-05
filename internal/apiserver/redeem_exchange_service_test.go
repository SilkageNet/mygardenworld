package apiserver

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"connectrpc.com/connect"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1/mygardenworldv1connect"
	redeemsvc "github.com/SilkageNet/mygardenworld/internal/redeem"
	"github.com/SilkageNet/mygardenworld/internal/runner"
	"github.com/SilkageNet/mygardenworld/internal/store"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestPublicRedeemSubmissionAndListing(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mgr := runner.NewManager(db, runner.NewBus(), log)
	redeemService, err := redeemsvc.NewService(ctx, db, mgr, log)
	if err != nil {
		t.Fatal(err)
	}
	svc := &Services{DB: db, Manager: mgr, Redeem: redeemService, RedeemLimiter: NewRedeemSubmitLimiter()}

	expires := time.Now().Add(10 * time.Minute)
	req := connect.NewRequest(&pb.SubmitRedeemCodesRequest{Entries: []*pb.RedeemCodeSubmission{{
		Code: " 礼包-A ", Channel: pb.Channel_CHANNEL_IOS, ExpiresAt: timestamppb.New(expires),
	}}})
	created, err := svc.SubmitRedeemCodes(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if got := created.Msg.GetResults()[0].GetDisposition(); got != pb.RedeemSubmitDisposition_REDEEM_SUBMIT_DISPOSITION_ACCEPTED {
		t.Fatalf("first disposition=%v", got)
	}
	duplicate, err := svc.SubmitRedeemCodes(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if got := duplicate.Msg.GetResults()[0].GetDisposition(); got != pb.RedeemSubmitDisposition_REDEEM_SUBMIT_DISPOSITION_DUPLICATE {
		t.Fatalf("duplicate disposition=%v", got)
	}
	listed, err := svc.ListRedeemCodes(ctx, connect.NewRequest(&pb.ListRedeemCodesRequest{PageSize: 10}))
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Msg.GetEntries()) != 1 || listed.Msg.GetEntries()[0].GetCode() != "礼包-A" {
		t.Fatalf("listed entries=%+v", listed.Msg.GetEntries())
	}
	browsed, err := svc.BrowseRedeemCodes(ctx, connect.NewRequest(&pb.BrowseRedeemCodesRequest{
		PageSize: 1,
		Filter:   pb.RedeemBrowseFilter_REDEEM_BROWSE_FILTER_ACTIVE,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if browsed.Msg.GetActiveTotal() != 1 || browsed.Msg.GetHistoryTotal() != 0 || len(browsed.Msg.GetEntries()) != 1 {
		t.Fatalf("browsed=%+v", browsed.Msg)
	}
}

func TestPublicRedeemSubmissionRequiresExpiryOrPermanent(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mgr := runner.NewManager(db, runner.NewBus(), log)
	redeemService, err := redeemsvc.NewService(ctx, db, mgr, log)
	if err != nil {
		t.Fatal(err)
	}
	svc := &Services{DB: db, Manager: mgr, Redeem: redeemService, RedeemLimiter: NewRedeemSubmitLimiter()}
	_, err = svc.SubmitRedeemCodes(ctx, connect.NewRequest(&pb.SubmitRedeemCodesRequest{Entries: []*pb.RedeemCodeSubmission{{
		Code: "NO-EXPIRY", Channel: pb.Channel_CHANNEL_ALIPAY,
	}}}))
	if connect.CodeOf(err) != connect.CodeInvalidArgument {
		t.Fatalf("SubmitRedeemCodes error=%v", err)
	}
}

func TestRedeemSubmitLimiterBoundsSourceAndGlobalTraffic(t *testing.T) {
	limiter := NewRedeemSubmitLimiter()
	if !limiter.Allow("one", redeemSubmitPerSource) {
		t.Fatal("initial source allowance rejected")
	}
	if limiter.Allow("one", 1) {
		t.Fatal("source limit not enforced")
	}
}

type redeemTestNode struct {
	db      *store.DB
	manager *runner.Manager
	redeem  *redeemsvc.Service
	server  *httptest.Server
}

type untrustedRedeemPeer struct {
	onlyPropagatable bool
}

func (p *untrustedRedeemPeer) GetExchangeInfo(_ context.Context, _ *connect.Request[pb.GetExchangeInfoRequest]) (*connect.Response[pb.GetExchangeInfoResponse], error) {
	return connect.NewResponse(&pb.GetExchangeInfoResponse{InstanceId: "untrusted-peer"}), nil
}

func (p *untrustedRedeemPeer) BrowseRedeemCodes(_ context.Context, _ *connect.Request[pb.BrowseRedeemCodesRequest]) (*connect.Response[pb.BrowseRedeemCodesResponse], error) {
	return connect.NewResponse(&pb.BrowseRedeemCodesResponse{}), nil
}

func (p *untrustedRedeemPeer) ListRedeemCodes(_ context.Context, req *connect.Request[pb.ListRedeemCodesRequest]) (*connect.Response[pb.ListRedeemCodesResponse], error) {
	p.onlyPropagatable = req.Msg.GetOnlyPropagatable()
	future := timestamppb.New(time.Now().UTC().Add(time.Hour))
	past := timestamppb.New(time.Now().UTC().Add(-time.Hour))
	return connect.NewResponse(&pb.ListRedeemCodesResponse{Entries: []*pb.RedeemCode{
		{Code: "PENDING", Channel: pb.Channel_CHANNEL_IOS, Permanent: true, Validation: pb.RedeemValidation_REDEEM_VALIDATION_PENDING},
		{Code: "INVALID", Channel: pb.Channel_CHANNEL_IOS, Permanent: true, Validation: pb.RedeemValidation_REDEEM_VALIDATION_INVALID},
		{Code: "GOOD", Channel: pb.Channel_CHANNEL_IOS, ExpiresAt: future, Validation: pb.RedeemValidation_REDEEM_VALIDATION_SUCCESS},
		{Code: "EXPIRED", Channel: pb.Channel_CHANNEL_IOS, ExpiresAt: past, Validation: pb.RedeemValidation_REDEEM_VALIDATION_SUCCESS},
	}}), nil
}

func (*untrustedRedeemPeer) SubmitRedeemCodes(_ context.Context, _ *connect.Request[pb.SubmitRedeemCodesRequest]) (*connect.Response[pb.SubmitRedeemCodesResponse], error) {
	return connect.NewResponse(&pb.SubmitRedeemCodesResponse{}), nil
}

func newRedeemTestNode(t *testing.T) *redeemTestNode {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "garden.db"))
	if err != nil {
		t.Fatal(err)
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	manager := runner.NewManager(db, runner.NewBus(), log)
	exchange, err := redeemsvc.NewService(ctx, db, manager, log)
	if err != nil {
		t.Fatal(err)
	}
	services := &Services{DB: db, Manager: manager, Redeem: exchange, RedeemLimiter: NewRedeemSubmitLimiter()}
	path, handler := mygardenworldv1connect.NewRedeemExchangeServiceHandler(NewHandlers(services).Redeem)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	node := &redeemTestNode{db: db, manager: manager, redeem: exchange, server: httptest.NewServer(mux)}
	t.Cleanup(func() {
		node.server.Close()
		node.manager.Shutdown()
		_ = node.db.Close()
	})
	return node
}

func addRedeemTestAccount(t *testing.T, node *redeemTestNode, name string) {
	t.Helper()
	ctx := context.Background()
	user, err := node.db.CreateUser(ctx, name, name+"@example.test", "hash")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := node.db.CreateAccount(ctx, user.ID, name, "ios", name, "secret"); err != nil {
		t.Fatal(err)
	}
}

func completeRedeemTestCode(t *testing.T, node *redeemTestNode, code, validation string) *store.RedeemCode {
	t.Helper()
	ctx := context.Background()
	expires := time.Now().UTC().Add(time.Hour)
	entry, _, err := node.db.UpsertRedeemCode(ctx, store.RedeemCodeInput{
		Code: code, Channel: "ios", ExpiresAt: &expires, SourceKey: "test:" + code,
		OriginInstanceID: node.redeem.InstanceID(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := node.db.EnsureRedeemAttempts(ctx); err != nil {
		t.Fatal(err)
	}
	attempt, err := node.db.NextRedeemAttempt(ctx)
	if err != nil || attempt == nil || attempt.CodeID != entry.ID {
		t.Fatalf("attempt=%+v entry=%+v err=%v", attempt, entry, err)
	}
	if err := node.db.CompleteRedeemAttempt(ctx, attempt.ID, attempt.RunToken, validation, validation, nil); err != nil {
		t.Fatal(err)
	}
	entries, _, err := node.db.ListRedeemCodes(ctx, 0, 100, true, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range entries {
		if candidate.ID == entry.ID {
			return candidate
		}
	}
	t.Fatalf("redeem code %q missing after completion", code)
	return nil
}

func TestNativeRedeemNodesExchangeOnlyLocallyVerifiedCodesWithoutLooping(t *testing.T) {
	ctx := context.Background()
	origin := newRedeemTestNode(t)
	subscriber := newRedeemTestNode(t)
	addRedeemTestAccount(t, origin, "origin")
	addRedeemTestAccount(t, subscriber, "subscriber")

	good := completeRedeemTestCode(t, origin, "GOOD", store.RedeemValidationSuccess)
	_ = completeRedeemTestCode(t, origin, "BAD", store.RedeemValidationInvalid)
	_ = completeRedeemTestCode(t, origin, "OLD", store.RedeemValidationExpired)
	if _, _, err := origin.db.UpsertRedeemCode(ctx, store.RedeemCodeInput{
		Code: "PENDING", Channel: "ios", SourceKey: "test:pending", OriginInstanceID: origin.redeem.InstanceID(),
	}); err != nil {
		t.Fatal(err)
	}

	feed, _, err := origin.redeem.List(ctx, "", 100, true, true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(feed) != 1 || feed[0].ID != good.ID {
		t.Fatalf("propagatable feed=%+v, want only GOOD", feed)
	}

	originSource, err := subscriber.db.UpsertRedeemSource(ctx, store.RedeemSourceInput{
		Name: "origin", Type: store.RedeemSourceMyGardenWorld, BaseURL: origin.server.URL,
		Enabled: true, PushEnabled: true, PollIntervalSeconds: 300,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := subscriber.redeem.SyncSource(ctx, originSource.ID); err != nil {
		t.Fatal(err)
	}
	imported, _, err := subscriber.db.ListRedeemCodes(ctx, 0, 100, true, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(imported) != 1 || imported[0].Code != "GOOD" || imported[0].Validation != store.RedeemValidationPending {
		t.Fatalf("subscriber registry=%+v", imported)
	}

	if err := subscriber.db.EnsureRedeemAttempts(ctx); err != nil {
		t.Fatal(err)
	}
	attempt, err := subscriber.db.NextRedeemAttempt(ctx)
	if err != nil || attempt == nil {
		t.Fatalf("subscriber attempt=%+v err=%v", attempt, err)
	}
	if err := subscriber.db.CompleteRedeemAttempt(ctx, attempt.ID, attempt.RunToken, store.RedeemValidationSuccess, "success", nil); err != nil {
		t.Fatal(err)
	}
	var returnOutbox int
	if err := subscriber.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM redeem_exchange_outbox`).Scan(&returnOutbox); err != nil {
		t.Fatal(err)
	}
	if returnOutbox != 0 {
		t.Fatalf("subscriber queued %d deliveries back to the source", returnOutbox)
	}

	subscriberSource, err := origin.db.UpsertRedeemSource(ctx, store.RedeemSourceInput{
		Name: "subscriber", Type: store.RedeemSourceMyGardenWorld, BaseURL: subscriber.server.URL,
		Enabled: true, PushEnabled: true, PollIntervalSeconds: 300,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := origin.redeem.SyncSource(ctx, subscriberSource.ID); err != nil {
		t.Fatal(err)
	}
	entries, _, err := origin.db.ListRedeemCodes(ctx, 0, 100, true, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("origin registry grew to %d entries after circular subscription", len(entries))
	}
	var afterFirst int64
	for _, entry := range entries {
		if entry.ID == good.ID {
			afterFirst = entry.Revision
		}
	}
	if err := origin.redeem.SyncSource(ctx, subscriberSource.ID); err != nil {
		t.Fatal(err)
	}
	entries, _, err = origin.db.ListRedeemCodes(ctx, 0, 100, true, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.ID == good.ID && entry.Revision != afterFirst {
			t.Fatalf("repeat circular sync advanced revision from %d to %d", afterFirst, entry.Revision)
		}
	}
}

func TestNativeRedeemSyncDefendsAgainstUnfilteredPeerResponses(t *testing.T) {
	ctx := context.Background()
	subscriber := newRedeemTestNode(t)
	peer := &untrustedRedeemPeer{}
	path, handler := mygardenworldv1connect.NewRedeemExchangeServiceHandler(peer)
	mux := http.NewServeMux()
	mux.Handle(path, handler)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	source, err := subscriber.db.UpsertRedeemSource(ctx, store.RedeemSourceInput{
		Name: "untrusted", Type: store.RedeemSourceMyGardenWorld, BaseURL: server.URL,
		Enabled: true, PollIntervalSeconds: 300,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := subscriber.redeem.SyncSource(ctx, source.ID); err != nil {
		t.Fatal(err)
	}
	if !peer.onlyPropagatable {
		t.Fatal("native sync did not request a propagatable-only feed")
	}
	entries, _, err := subscriber.db.ListRedeemCodes(ctx, 0, 100, true, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Code != "GOOD" {
		t.Fatalf("subscriber accepted unverified peer entries: %+v", entries)
	}
}
