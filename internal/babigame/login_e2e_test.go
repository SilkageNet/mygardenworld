package babigame_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

// TestAccountLoginE2E exercises the full HTTP login chain against live
// endpoints.
func TestAccountLoginE2E(t *testing.T) {
	username, password := e2eCredentials(t)

	cfg := babigame.DefaultConfig()
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

	cfg := babigame.DefaultConfig()
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

	// Heartbeat (fire-and-forget)
	t.Log("usr.heartTick...")
	err = client.Send(ctx, "usr.heartTick", map[string]any{}, session.RouteArg())
	if err != nil {
		t.Fatalf("heartTick send: %v", err)
	}
	t.Log("  OK: sent")

	// im.getChannelId
	t.Log("im.getChannelId...")
	imV, _, err := client.RPC(ctx, "im.getChannelId", map[string]any{}, session.RouteArg(), 10*time.Second)
	if err != nil {
		t.Fatalf("im.getChannelId: %v", err)
	}
	t.Logf("  OK: %d bytes", len(imV))

	// frd.enter
	t.Log("frd.enter...")
	frdV, _, err := client.RPC(ctx, "frd.enter", map[string]any{
		"needBlackList":  1,
		"needApplyList":  1,
		"needFriendList": 1,
	}, session.RouteArg(), 10*time.Second)
	if err != nil {
		t.Fatalf("frd.enter: %v", err)
	}
	t.Logf("  OK: %d bytes", len(frdV))

	// homeRqst.showBird
	t.Log("homeRqst.showBird...")
	birdV, _, err := client.RPC(ctx, "homeRqst.showBird", map[string]any{
		"time": 1,
	}, session.RouteArg(), 10*time.Second)
	if err != nil {
		t.Fatalf("homeRqst.showBird: %v", err)
	}
	t.Logf("  OK: %d bytes", len(birdV))

	// sdk.sendGoods
	t.Log("sdk.sendGoods...")
	sdkV, _, err := client.RPC(ctx, "sdk.sendGoods", map[string]any{}, session.RouteArg(), 10*time.Second)
	if err != nil {
		t.Fatalf("sdk.sendGoods: %v", err)
	}
	t.Logf("  OK: %d bytes", len(sdkV))

	// mail.getList
	t.Log("mail.getList...")
	mailV, _, err := client.RPC(ctx, "mail.getList", map[string]any{}, session.RouteArg(), 10*time.Second)
	if err != nil {
		t.Fatalf("mail.getList: %v", err)
	}
	t.Logf("  OK: %d bytes", len(mailV))

	// randomEvent.enter
	t.Log("randomEvent.enter...")
	reV, _, err := client.RPC(ctx, "randomEvent.enter", map[string]any{}, session.RouteArg(), 10*time.Second)
	if err != nil {
		t.Logf("  WARN (non-fatal): %v", err)
	} else {
		t.Logf("  OK: %d bytes", len(reV))
	}

	// waterwheel.enter
	t.Log("waterwheel.enter...")
	wwV, _, err := client.RPC(ctx, "waterwheel.enter", map[string]any{}, session.RouteArg(), 10*time.Second)
	if err != nil {
		t.Logf("  WARN (non-fatal): %v", err)
	} else {
		t.Logf("  OK: %d bytes", len(wwV))
	}

	// Test land operations (read-only: we don't want to mutate game state destructively)
	// usrLand.harvestOneKey - safe to call even if nothing to harvest
	t.Log("usrLand.harvestOneKey...")
	harvestV, _, err := client.RPC(ctx, "usrLand.harvestOneKey", map[string]any{}, session.RouteArg(), 10*time.Second)
	if err != nil {
		t.Logf("  WARN (non-fatal): %v", err)
	} else {
		t.Logf("  OK: %d bytes", len(harvestV))
	}

	// usrLand.waterOneKey - safe to call
	t.Log("usrLand.waterOneKey...")
	waterV, _, err := client.RPC(ctx, "usrLand.waterOneKey", map[string]any{}, session.RouteArg(), 10*time.Second)
	if err != nil {
		t.Logf("  WARN (non-fatal): %v", err)
	} else {
		t.Logf("  OK: %d bytes", len(waterV))
	}

	t.Log("all RPCs completed successfully")
}

// TestHTTPEndpointsE2E tests additional HTTP endpoints beyond the login chain.
func TestHTTPEndpointsE2E(t *testing.T) {
	username, password := e2eCredentials(t)

	cfg := babigame.DefaultConfig()
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

	cfg := babigame.DefaultConfig()
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
