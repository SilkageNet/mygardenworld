//go:build ignore

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	connect "connectrpc.com/connect"
	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1/mygardenworldv1connect"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	addr := "http://127.0.0.1:50051"

	// Login first
	authClient := mygardenworldv1connect.NewAuthServiceClient(http.DefaultClient, addr)
	loginResp, err := authClient.Login(ctx, connect.NewRequest(&pb.LoginRequest{
		Username: "admin",
		Password: "admin123",
	}))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Login error: %v\n", err)
		os.Exit(1)
	}

	token := loginResp.Msg.GetAccessToken()
	fmt.Printf("✓ Logged in, token: %s...\n\n", token[:20])

	// Create authenticated client
	httpClient := &authHTTPClient{token: token}
	client := mygardenworldv1connect.NewQueryServiceClient(httpClient, addr)

	// Get status to find account
	statusResp, err := client.GetStatus(ctx, connect.NewRequest(&pb.GetStatusRequest{}))
	if err != nil {
		fmt.Fprintf(os.Stderr, "GetStatus error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("=== Accounts ===")
	for _, acc := range statusResp.Msg.GetAccounts() {
		fmt.Printf("  ID=%s Name=%s Connected=%v\n", acc.GetAccountId(), acc.GetAccountName(), acc.GetConnected())
	}

	if len(statusResp.Msg.GetAccounts()) == 0 {
		fmt.Println("No accounts found")
		os.Exit(0)
	}

	accID := statusResp.Msg.GetAccounts()[0].GetAccountId()

	// Get snapshot
	snapResp, err := client.GetSnapshot(ctx, connect.NewRequest(&pb.GetSnapshotRequest{AccountId: accID}))
	if err != nil {
		fmt.Fprintf(os.Stderr, "GetSnapshot error: %v\n", err)
		os.Exit(1)
	}

	snap := snapResp.Msg
	fmt.Printf("\n=== Snapshot: %s ===\n", snap.GetAccountName())

	// Race state
	race := snap.GetFmlRace()
	if race == nil {
		fmt.Println("\n=== Race: not observed ===")
	} else {
		fmt.Printf("\n=== Race: observed=%v batchActive=%v batchStatus=%d ===\n", race.GetObserved(), race.GetBatchActive(), race.GetBatchStatus())
		fmt.Printf("  BatchStartMs=%d BatchEndMs=%d\n", race.GetBatchStartMs(), race.GetBatchEndMs())
		if race.GetBatchStartMs() > 0 {
			fmt.Printf("  Start=%s\n", time.UnixMilli(race.GetBatchStartMs()).Format("2006-01-02 15:04:05"))
		}
		if race.GetBatchEndMs() > 0 {
			fmt.Printf("  End=%s\n", time.UnixMilli(race.GetBatchEndMs()).Format("2006-01-02 15:04:05"))
		}
		fmt.Printf("  Now=%s\n", time.Now().Format("2006-01-02 15:04:05"))
		if t := race.GetTaken(); t != nil && t.GetHasTask() {
			target := t.GetTargetLabel()
			if target == "" {
				target = "(无目标参数)"
			}
			fmt.Printf("  Taken: %s → %s\n", t.GetTaskLabel(), target)
			fmt.Printf("         msId=%d taskId=%d type=%d score=%d finish=%d/%d\n",
				t.GetTaskMsId(), t.GetTaskId(), t.GetTaskType(), t.GetScore(), t.GetFinishCnt(), t.GetTargetCnt())
		} else {
			fmt.Println("  Taken: none")
		}
		fmt.Printf("  Pool: %d tasks\n", len(race.GetTasks()))
		for _, t := range race.GetTasks() {
			upgrade := ""
			if t.GetIsUpgrade() {
				upgrade = " [已升级]"
			}
			cd := ""
			if t.GetAppearTimeMs() > time.Now().UnixMilli() {
				cd = "CD "
			}
			target := t.GetTargetLabel()
			if target != "" {
				target = " → " + target
			}
			skip := ""
			if r := t.GetTakeSkipReason(); r != "" {
				skip = " skip=" + r
			}
			fmt.Printf("    msId=%d %s%s%s score=%d%s%s\n",
				t.GetMsId(), cd, t.GetTaskLabel(), target, t.GetScore(), upgrade, skip)
		}
	}
}

type authHTTPClient struct {
	token string
}

func (c *authHTTPClient) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	return http.DefaultClient.Do(req)
}
