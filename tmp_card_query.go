package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

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
	if err != nil { panic(err) }
	token := login.Msg.GetAccessToken()

	query := mygardenworldv1connect.NewQueryServiceClient(http.DefaultClient, base)
	req := connect.NewRequest(&pb.GetSnapshotRequest{AccountId: "1"})
	req.Header().Set("Authorization", "Bearer "+token)
	snap, err := query.GetSnapshot(ctx, req)
	if err != nil { panic(err) }
	msg := snap.Msg
	fmt.Printf("account=%s role=%s\n", msg.GetAccountName(), msg.GetRoleName())
	fmt.Printf("observed namespaces: %v\n", msg.GetObservedNamespaces())
	
	// inventory for card packs
	for id, cnt := range msg.GetInventory() {
		if cnt > 0 && (id >= 500000 || id >= 400000) {
			fmt.Printf("inv %d = %d\n", id, cnt)
		}
	}
	b, _ := json.MarshalIndent(msg, "", "  ")
	fmt.Println(string(b[:min(len(b), 5000)]))
}

func min(a,b int) int { if a<b { return a }; return b }
