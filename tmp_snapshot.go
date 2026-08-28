//go:build ignore

package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"connectrpc.com/connect"
	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1/mygardenworldv1connect"
)

func main() {
	ctx := context.Background()
	base := "http://127.0.0.1:50052"
	auth := mygardenworldv1connect.NewAuthServiceClient(http.DefaultClient, base)
	login, err := auth.Login(ctx, connect.NewRequest(&pb.LoginRequest{
		Username: "admin",
		Password: "Use-A-Long-Local-Admin-Password-123!",
	}))
	if err != nil {
		panic(err)
	}
	token := login.Msg.GetAccessToken()

	query := mygardenworldv1connect.NewQueryServiceClient(http.DefaultClient, base)
	req := connect.NewRequest(&pb.GetSnapshotRequest{AccountId: "2"})
	req.Header().Set("Authorization", "Bearer "+token)
	snap, err := query.GetSnapshot(ctx, req)
	if err != nil {
		panic(err)
	}
	msg := snap.Msg

	fmt.Println("=== 叶小楠 live snapshot ===")
	fmt.Printf("connected namespaces include 104=%v 109=%v\n",
		hasNS(msg.GetObservedNamespaces(), "104"), hasNS(msg.GetObservedNamespaces(), "109"))

	var art310201 int32
	for _, art := range msg.GetSellableFlowerArts() {
		if art.GetArtId() == 310201 {
			art310201 = art.GetStock()
		}
		if art.GetStock() > 0 {
			fmt.Printf("  art stock: %d (%s) = %d\n", art.GetArtId(), art.GetArtName(), art.GetStock())
		}
	}
	fmt.Printf("310201 stock=%d\n", art310201)
	fmt.Printf("total planned ops=%d\n", len(msg.GetPlannedOperations()))

	if ledger := msg.GetInventoryLedger(); ledger != nil {
		fmt.Println("ledger allocated items:")
		for _, item := range ledger.GetItems() {
			if item.GetAllocated() > 0 && item.GetItemId() >= 300000 {
				fmt.Printf("  art %d owned=%d allocated=%d available=%d\n", item.GetItemId(), item.GetOwned(), item.GetAllocated(), item.GetAvailable())
			}
		}
	}

	fmt.Println("\nall planned ops top 15:")
	for i, op := range msg.GetPlannedOperations() {
		if i >= 15 {
			break
		}
		fmt.Printf("  #%d prio=%d exec=%v rpc=%s status=%v blocked=%v\n",
			i, op.GetPriority(), op.GetExecutable(), op.GetRpc(), op.GetStatus(), op.GetBlockedReasons())
	}

	fmt.Println("\nplanned ops (flower rack / customer art):")
	for i, op := range msg.GetPlannedOperations() {
		if i >= 25 {
			break
		}
		kind := op.GetRpc()
		if !strings.Contains(kind, "flowerRack") && !strings.Contains(kind, "flowerArt") && !strings.Contains(kind, "orderCustomer") {
			continue
		}
		fmt.Printf("  #%d prio=%d exec=%v kind=%s reason=%s blocked=%v\n",
			i, op.GetPriority(), op.GetExecutable(), kind, trunc(op.GetReason(), 80), op.GetBlockedReasons())
	}

	fmt.Println("\nfirst executable op overall:")
	for _, op := range msg.GetPlannedOperations() {
		if op.GetExecutable() {
			fmt.Printf("  prio=%d kind=%s reason=%s\n", op.GetPriority(), op.GetRpc(), trunc(op.GetReason(), 100))
			break
		}
	}

	if diag := msg.GetDiagnostics(); diag != nil {
		fmt.Printf("runner: current_op=%q blocked=%v\n", diag.GetCurrentOperation(), diag.GetBlockedReasons())
	}
	for _, ds := range msg.GetDomainStatuses() {
		if strings.Contains(ds.GetDomain(), "flower") || strings.Contains(ds.GetDomain(), "order") {
			fmt.Printf("domain %s observed=%v status=%v reasons=%v\n", ds.GetDomain(), ds.GetObserved(), ds.GetStatus(), ds.GetBlockedReasons())
		}
	}
	if bs := msg.GetBlockingSummary(); bs != nil {
		for _, g := range bs.GetGroups() {
			if strings.Contains(g.GetDomain(), "flower") {
				fmt.Printf("blocking: domain=%s stage=%s reasons=%v\n", g.GetDomain(), g.GetStage(), g.GetReasons())
			}
		}
	}
}

func hasNS(list []string, id string) bool {
	for _, ns := range list {
		if ns == id {
			return true
		}
	}
	return false
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
