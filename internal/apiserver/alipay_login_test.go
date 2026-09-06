package apiserver

import (
	"context"
	"testing"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/store"
)

type fakeAlipayProvider struct {
	polls int
}

func (f *fakeAlipayProvider) BeginQR(context.Context) (babigame.AlipayQRChallenge, error) {
	return babigame.AlipayQRChallenge{Token: "private-qr-token", URL: "alipays://qr-content"}, nil
}

func (f *fakeAlipayProvider) PollQR(context.Context, string) (babigame.AlipayWebGrant, bool, error) {
	f.polls++
	if f.polls == 1 {
		return babigame.AlipayWebGrant{}, false, nil
	}
	return babigame.AlipayWebGrant{Token: "private-web-token", UserID: "u1"}, true, nil
}

func (f *fakeAlipayProvider) LoginWithWebGrant(context.Context, *babigame.HTTPClient, babigame.AlipayWebGrant) (*babigame.Session, error) {
	panic("not used by coordinator unit test")
}

func TestAlipayLoginCoordinatorWaitsThenCompletes(t *testing.T) {
	provider := &fakeAlipayProvider{}
	coordinator := NewAlipayLoginCoordinator(provider)
	loginID, qr, _, err := coordinator.start(context.Background(), 7, alipayLoginOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if qr != "alipays://qr-content" || loginID == "" {
		t.Fatalf("loginID=%q qr=%q", loginID, qr)
	}
	creatorCalls := 0
	create := func(_ context.Context, grant babigame.AlipayWebGrant, _ alipayLoginOptions) (*store.Account, error) {
		creatorCalls++
		if grant.UserID != "u1" || grant.Token != "private-web-token" {
			t.Fatalf("grant=%+v", grant)
		}
		return &store.Account{ID: 9, Name: "derived", Channel: "alipay", Username: grant.UserID}, nil
	}

	first := coordinator.poll(context.Background(), 7, loginID, create)
	if first.Status != pb.AlipayLoginStatus_ALIPAY_LOGIN_STATUS_WAITING_FOR_SCAN || creatorCalls != 0 {
		t.Fatalf("first=%+v creatorCalls=%d", first, creatorCalls)
	}
	second := coordinator.poll(context.Background(), 7, loginID, create)
	if second.Status != pb.AlipayLoginStatus_ALIPAY_LOGIN_STATUS_COMPLETE || second.Account == nil || second.Account.ID != 9 || creatorCalls != 1 {
		t.Fatalf("second=%+v creatorCalls=%d", second, creatorCalls)
	}
	third := coordinator.poll(context.Background(), 7, loginID, create)
	if third.Status != pb.AlipayLoginStatus_ALIPAY_LOGIN_STATUS_COMPLETE || creatorCalls != 1 {
		t.Fatalf("terminal poll=%+v creatorCalls=%d", third, creatorCalls)
	}
}

func TestAlipayLoginCoordinatorHidesOtherOwners(t *testing.T) {
	coordinator := NewAlipayLoginCoordinator(&fakeAlipayProvider{})
	loginID, _, _, err := coordinator.start(context.Background(), 7, alipayLoginOptions{})
	if err != nil {
		t.Fatal(err)
	}
	got := coordinator.poll(context.Background(), 8, loginID, func(context.Context, babigame.AlipayWebGrant, alipayLoginOptions) (*store.Account, error) {
		t.Fatal("creator must not run")
		return nil, nil
	})
	if got.Status != pb.AlipayLoginStatus_ALIPAY_LOGIN_STATUS_FAILED || got.Error == "" {
		t.Fatalf("snapshot=%+v", got)
	}
}
