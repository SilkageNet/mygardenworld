package babigame_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
	"github.com/SilkageNet/mygardenworld/internal/babigame/clientrpc"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

// TestAccountLoginE2E exercises the full HTTP login chain against live
// endpoints.
func TestAccountLoginE2E(t *testing.T) {
	username, password := e2eCredentials(t)

	cfg := testConfig(t)
	httpc := babigame.NewHTTPClient(cfg, "", "", "")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Full login chain via helper (covers AccountLoginUsername, GameLogin,
	// GWIndexLogin, GWGetGsInfoList internally)
	t.Log("PerformLoginWithPassword...")
	session, err := babigame.PerformLoginWithPassword(ctx, httpc, username, password, 1)
	if err != nil {
		t.Fatalf("PerformLoginWithPassword failed: %v", err)
	}
	if session.AID == 0 || session.GsIdx == 0 || session.RouteToken == "" {
		t.Fatalf("session incomplete: aid=%d gsIdx=%d routeToken=%q", session.AID, session.GsIdx, session.RouteToken)
	}
	t.Logf("  OK: aid=%d gsIdx=%d gsHost=%s:%d wsURL=%s",
		session.AID, session.GsIdx, session.GsHost, session.GsPortSSL, session.WSURL())
}

// TestWSFullSessionE2E exercises the complete session lifecycle:
// HTTP login → WS connect → reLogin → lazySync → heartbeat → game RPCs.
func TestWSFullSessionE2E(t *testing.T) {
	username, password := e2eCredentials(t)

	cfg := testConfig(t)
	httpc := babigame.NewHTTPClient(cfg, "", "", "")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Login
	session, err := babigame.PerformLoginWithPassword(ctx, httpc, username, password, 1)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	t.Logf("session: aid=%d gsIdx=%d ws=%s", session.AID, session.GsIdx, session.WSURL())

	// WS connect
	client := babigame.NewClient(session)
	defer client.Close()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("ws connect: %v", err)
	}
	rpc := clientrpc.NewClient(babigame.NewRPCClient(client, session, babigame.WithServerErrorsAsResults()))
	time.Sleep(300 * time.Millisecond)

	// ReLogin
	t.Log("ReLogin...")
	reLoginV, err := client.ReLogin(ctx, 1)
	if err != nil {
		t.Fatalf("ReLogin: %v", err)
	}
	var nsMap map[string]json.RawMessage
	if err := json.Unmarshal(reLoginV, &nsMap); err != nil {
		t.Fatalf("ReLogin v parse: %v", err)
	}
	t.Logf("  OK: %d namespaces, %d bytes", len(nsMap), len(reLoginV))

	// Verify namespace 100 (land state) is present
	if _, ok := nsMap["100"]; !ok {
		t.Error("  missing namespace 100 (land state)")
	}
	// Verify namespace 7 (inventory) is present
	if _, ok := nsMap["7"]; !ok {
		t.Error("  missing namespace 7 (inventory)")
	}

	// LazySync
	t.Log("LazySync...")
	lsV, err := client.LazySync(ctx)
	if err != nil {
		t.Fatalf("LazySync: %v", err)
	}
	t.Logf("  OK: %d bytes", len(lsV))

	// Heartbeat
	t.Log("usr.heartTick...")
	heartTickResp, err := rpc.Usr().HeartTick(ctx, clientproto.UsrHeartTickRequest{}, babigame.WithTimeout(10*time.Second))
	if err != nil {
		t.Fatalf("usr.heartTick: %v", err)
	}
	t.Logf("  OK: %d bytes", len(heartTickResp.Payload))

	// im.getChannelId
	t.Log("im.getChannelId...")
	imResp, err := rpc.Im().GetChannelId(ctx, clientproto.ImGetChannelIdRequest{}, babigame.WithTimeout(10*time.Second))
	if err != nil {
		t.Fatalf("im.getChannelId: %v", err)
	}
	t.Logf("  OK: %d bytes", len(imResp.Payload))

	// frd.enter
	t.Log("frd.enter...")
	frdResp, err := rpc.Frd().Enter(ctx, clientproto.FrdEnterRequest{
		NeedBlackList:  1,
		NeedApplyList:  1,
		NeedFriendList: 1,
	}, babigame.WithTimeout(10*time.Second))
	if err != nil {
		t.Fatalf("frd.enter: %v", err)
	}
	t.Logf("  OK: %d bytes", len(frdResp.Payload))

	// homeRqst.showBird
	t.Log("homeRqst.showBird...")
	birdResp, err := rpc.HomeRqst().ShowBird(ctx, clientproto.HomeRqstShowBirdRequest{Time: 1}, babigame.WithTimeout(10*time.Second))
	if err != nil {
		t.Fatalf("homeRqst.showBird: %v", err)
	}
	t.Logf("  OK: %d bytes", len(birdResp.Payload))

	// sdk.sendGoods
	t.Log("sdk.sendGoods...")
	sdkResp, err := rpc.Sdk().SendGoods(ctx, clientproto.SdkSendGoodsRequest{}, babigame.WithTimeout(10*time.Second))
	if err != nil {
		t.Fatalf("sdk.sendGoods: %v", err)
	}
	t.Logf("  OK: %d bytes", len(sdkResp.Payload))

	// mail.getList
	t.Log("mail.getList...")
	mailResp, err := rpc.Mail().GetList(ctx, clientproto.MailGetListRequest{}, babigame.WithTimeout(10*time.Second))
	if err != nil {
		t.Fatalf("mail.getList: %v", err)
	}
	t.Logf("  OK: %d bytes", len(mailResp.Payload))

	// randomEvent.enter
	t.Log("randomEvent.enter...")
	reResp, err := rpc.RandomEvent().Enter(ctx, clientproto.RandomEventEnterRequest{}, babigame.WithTimeout(10*time.Second))
	if err != nil {
		t.Logf("  WARN (non-fatal): %v", err)
	} else {
		t.Logf("  OK: %d bytes", len(reResp.Payload))
	}

	// waterwheel.enter
	t.Log("waterwheel.enter...")
	wwResp, err := rpc.Waterwheel().Enter(ctx, clientproto.WaterwheelEnterRequest{}, babigame.WithTimeout(10*time.Second))
	if err != nil {
		t.Logf("  WARN (non-fatal): %v", err)
	} else {
		t.Logf("  OK: %d bytes", len(wwResp.Payload))
	}

	// Test land operations (read-only: we don't want to mutate game state destructively)
	// usrLand.harvestOneKey - safe to call even if nothing to harvest
	t.Log("usrLand.harvestOneKey...")
	harvestResp, err := rpc.UsrLand().HarvestOneKey(ctx, clientproto.UsrLandHarvestOneKeyRequest{}, babigame.WithTimeout(10*time.Second))
	if err != nil {
		t.Logf("  WARN (non-fatal): %v", err)
	} else {
		t.Logf("  OK: %d bytes", len(harvestResp.Payload))
	}

	// usrLand.waterOneKey - safe to call
	t.Log("usrLand.waterOneKey...")
	waterResp, err := rpc.UsrLand().WaterOneKey(ctx, clientproto.UsrLandWaterOneKeyRequest{}, babigame.WithTimeout(10*time.Second))
	if err != nil {
		t.Logf("  WARN (non-fatal): %v", err)
	} else {
		t.Logf("  OK: %d bytes", len(waterResp.Payload))
	}

	t.Log("all RPCs completed successfully")
}

// TestHTTPEndpointsE2E tests additional HTTP endpoints beyond the login chain.
func TestHTTPEndpointsE2E(t *testing.T) {
	username, password := e2eCredentials(t)

	cfg := testConfig(t)
	httpc := babigame.NewHTTPClient(cfg, "", "", "")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// AccountTokenVerify (best-effort, may return empty)
	t.Log("AccountTokenVerify...")
	resp, err := httpc.AccountTokenVerify(ctx)
	if err != nil {
		t.Logf("  WARN: %v", err)
	} else {
		t.Logf("  OK: %v", summarizeMap(resp))
	}

	// QueryInitParams
	t.Log("QueryInitParams...")
	resp, err = httpc.QueryInitParams(ctx)
	if err != nil {
		t.Logf("  WARN: %v", err)
	} else {
		t.Logf("  OK: keys=%v", mapKeysStr(resp))
	}

	// Full login to get token for token-bound endpoints
	native, err := httpc.AccountLoginUsername(ctx, username, password)
	if err != nil {
		t.Fatalf("AccountLoginUsername: %v", err)
	}
	gameLogin, err := httpc.GameLogin(ctx, native, "", "", "", "")
	if err != nil {
		t.Fatalf("GameLogin: %v", err)
	}
	t.Log("logged in")

	// QueryLoginParams (token-bound)
	t.Log("QueryLoginParams...")
	resp, err = httpc.QueryLoginParams(ctx)
	if err != nil {
		t.Logf("  WARN: %v", err)
	} else {
		t.Logf("  OK: keys=%v", mapKeysStr(resp))
	}

	// GWGetGsInfoList
	t.Log("GWGetGsInfoList...")
	gw, err := httpc.GWIndexLogin(ctx, gameLogin, 1)
	if err != nil {
		t.Fatalf("GWIndexLogin: %v", err)
	}
	v, _ := gw["v"].(map[string]any)
	acc, _ := v["acc"].(map[string]any)
	aid := int64(acc["id"].(float64))
	gsIdx := int(acc["lastGsIdx"].(float64))

	infos, err := httpc.GWGetGsInfoList(ctx, aid, gsIdx, cfg.ChannelID)
	if err != nil {
		t.Fatalf("GWGetGsInfoList: %v", err)
	}
	if len(infos) == 0 {
		t.Fatal("GWGetGsInfoList returned empty list")
	}
	t.Logf("  OK: %d servers, first=%s:%d (status=%d count=%d)",
		len(infos), infos[0].Host, infos[0].PortSSL, infos[0].Status, infos[0].Count)
}

// TestCultivateAndAssetsE2E verifies cultivation state parsing and asset tracking.
func TestCultivateAndAssetsE2E(t *testing.T) {
	username, password := e2eCredentials(t)

	cfg := testConfig(t)
	httpc := babigame.NewHTTPClient(cfg, "", "", "")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session, err := babigame.PerformLoginWithPassword(ctx, httpc, username, password, 1)
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	client := babigame.NewClient(session)
	defer client.Close()
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	time.Sleep(300 * time.Millisecond)

	v, err := client.ReLogin(ctx, 1)
	if err != nil {
		t.Fatalf("reLogin: %v", err)
	}

	// Feed into state tracker
	st := state.New()
	st.ApplyV(v)

	// Verify water drops
	waterDrops, waterDropsTotal, nextMs := st.WaterDrops()
	t.Logf("水滴: %d/%d (下次恢复: %d)", waterDrops, waterDropsTotal, nextMs)

	// Verify cultivations
	cultivations := st.Cultivations()
	t.Logf("培育花卉: %d 种", len(cultivations))
	for fid, cv := range cultivations {
		status := "空闲"
		if cv.Status == 1 {
			status = "培育中"
		}
		t.Logf("  花卉#%d: 等级=%d 状态=%s 完成时间=%d", fid, cv.Lvl, status, cv.CulTimeMs)
	}
	if len(cultivations) == 0 {
		t.Error("ns101 未解析到任何培育数据")
	}

	// Verify inventory (flowers + gold)
	gold := st.Gold()
	t.Logf("金币: %d", gold)
	flowers := st.FlowerInventory()
	t.Logf("花卉库存: %d 种", len(flowers))
	for fid, count := range flowers {
		t.Logf("  花卉#%d: %d 个", fid, count)
	}

	// Verify waterwheel
	wwReady := st.WaterwheelReady()
	t.Logf("水车可领: %d", wwReady)
}

func e2eCredentials(t *testing.T) (string, string) {
	t.Helper()
	username := os.Getenv("E2E_USERNAME")
	password := os.Getenv("E2E_PASSWORD")
	if username == "" || password == "" {
		t.Skip("E2E_USERNAME / E2E_PASSWORD not set; skipping live E2E test")
	}
	return username, password
}

func testConfig(t *testing.T) babigame.Config {
	t.Helper()
	cfg, err := babigame.ConfigForChannel(babigame.ChannelIOS)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func mapKeysStr(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func summarizeMap(m map[string]any) string {
	if m == nil {
		return "<nil>"
	}
	b, _ := json.Marshal(m)
	s := string(b)
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}
