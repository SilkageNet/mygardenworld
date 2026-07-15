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
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	addr := "http://127.0.0.1:50051"
	authClient := mygardenworldv1connect.NewAuthServiceClient(http.DefaultClient, addr)
	loginResp, err := authClient.Login(ctx, connect.NewRequest(&pb.LoginRequest{
		Username: "admin",
		Password: "admin123",
	}))
	if err != nil {
		fatalf("login: %v", err)
	}
	httpClient := &authHTTPClient{token: loginResp.Msg.GetAccessToken()}
	query := mygardenworldv1connect.NewQueryServiceClient(httpClient, addr)
	policyClient := mygardenworldv1connect.NewPolicyServiceClient(httpClient, addr)
	autoClient := mygardenworldv1connect.NewAutomationServiceClient(httpClient, addr)

	statusResp, err := query.GetStatus(ctx, connect.NewRequest(&pb.GetStatusRequest{}))
	if err != nil {
		fatalf("status: %v", err)
	}
	if len(statusResp.Msg.GetAccounts()) == 0 {
		fatalf("no accounts")
	}
	accID := statusResp.Msg.GetAccounts()[0].GetAccountId()
	fmt.Printf("account=%s connected=%v\n", accID, statusResp.Msg.GetAccounts()[0].GetConnected())

	getPolicy := func() *pb.Policy {
		resp, err := policyClient.GetPolicy(ctx, connect.NewRequest(&pb.GetPolicyRequest{AccountId: accID}))
		if err != nil {
			fatalf("getPolicy: %v", err)
		}
		p := resp.Msg.GetPolicy()
		if p.GetUnion() == nil {
			p.Union = &pb.UnionPolicy{}
		}
		if p.Union.Race == nil {
			p.Union.Race = &pb.UnionRacePolicy{}
		}
		return p
	}
	setRace := func(mut func(r *pb.UnionRacePolicy)) {
		p := getPolicy()
		mut(p.Union.Race)
		p.Union.Race.Enabled = true
		p.Union.Race.AutoEnableModules = true
		p.AutomationEnabled = true
		_, err := policyClient.SetPolicy(ctx, connect.NewRequest(&pb.SetPolicyRequest{
			AccountId: accID,
			Policy:    p,
		}))
		if err != nil {
			fatalf("setPolicy: %v", err)
		}
	}
	raceSnap := func() (hasTask bool, msID int64, score, finish, target int32, label string) {
		resp, err := query.GetSnapshot(ctx, connect.NewRequest(&pb.GetSnapshotRequest{AccountId: accID}))
		if err != nil {
			fatalf("snapshot: %v", err)
		}
		r := resp.Msg.GetFmlRace()
		if r == nil || r.GetTaken() == nil || !r.GetTaken().GetHasTask() {
			return false, 0, 0, 0, 0, ""
		}
		t := r.GetTaken()
		return true, t.GetTaskMsId(), t.GetScore(), t.GetFinishCnt(), t.GetTargetCnt(), t.GetTaskLabel()
	}
	connected := func() bool {
		resp, err := query.GetStatus(ctx, connect.NewRequest(&pb.GetStatusRequest{}))
		if err != nil {
			return false
		}
		for _, a := range resp.Msg.GetAccounts() {
			if a.GetAccountId() == accID {
				return a.GetConnected()
			}
		}
		return false
	}
	wait := func(desc string, wantHasTask bool, timeout time.Duration) (msID int64, score int32, label string) {
		deadline := time.Now().Add(timeout)
		var last string
		for time.Now().Before(deadline) {
			if !connected() {
				fmt.Printf("  [%s] waiting reconnect...\n", desc)
				time.Sleep(3 * time.Second)
				continue
			}
			has, id, sc, fin, tgt, lb := raceSnap()
			cur := fmt.Sprintf("has=%v msId=%d score=%d %d/%d label=%q", has, id, sc, fin, tgt, lb)
			if cur != last {
				fmt.Printf("  [%s] %s\n", desc, cur)
				last = cur
			}
			if has == wantHasTask {
				return id, sc, lb
			}
			time.Sleep(2 * time.Second)
		}
		fatalf("timeout waiting %s (want hasTask=%v)", desc, wantHasTask)
		return 0, 0, ""
	}

	// Ensure session is up.
	if !connected() {
		fmt.Println("==> reconnect account")
		if _, err := autoClient.Reload(ctx, connect.NewRequest(&pb.ReloadRequest{AccountId: accID})); err != nil {
			fmt.Printf("reload: %v (trying Start)\n", err)
			if _, err := autoClient.Start(ctx, connect.NewRequest(&pb.StartRequest{AccountId: accID})); err != nil {
				fatalf("start: %v", err)
			}
		}
		deadline := time.Now().Add(90 * time.Second)
		for time.Now().Before(deadline) {
			if connected() {
				fmt.Println("✓ connected")
				break
			}
			time.Sleep(2 * time.Second)
		}
		if !connected() {
			fatalf("account still disconnected")
		}
	}

	origMax := getPolicy().GetUnion().GetRace().GetMaxTaskScore()
	fmt.Printf("original max_task_score=%d\n", origMax)

	has, msID, score, fin, tgt, label := raceSnap()
	fmt.Printf("initial: has=%v msId=%d score=%d %d/%d label=%q\n", has, msID, score, fin, tgt, label)

	// If already holding, give it up first (may need cooldown wait).
	if has {
		fmt.Printf("\n==> give up current task (score=%d)\n", score)
		floor := score
		if floor < 1 {
			floor = 100
		}
		setRace(func(r *pb.UnionRacePolicy) { r.MaxTaskScore = floor })
		wait("giveUp-current", false, 4*time.Minute)
		fmt.Println("✓ current task given up")
		// Server giveUp cooldown ~3min; wait before take+giveUp again.
		fmt.Println("==> wait 3m for giveUp cooldown")
		time.Sleep(3 * time.Minute)
	}

	fmt.Println("\n==> take a new race task")
	setRace(func(r *pb.UnionRacePolicy) { r.MaxTaskScore = 0 })
	msID, score, label = wait("take", true, 90*time.Second)
	fmt.Printf("✓ taken msId=%d score=%d label=%q\n", msID, score, label)

	fmt.Printf("\n==> give up newly taken task (score=%d)\n", score)
	floor := score
	if floor < 1 {
		floor = 100
	}
	setRace(func(r *pb.UnionRacePolicy) { r.MaxTaskScore = floor })
	wait("giveUp-new", false, 4*time.Minute)
	fmt.Println("✓ newly taken task given up")

	restore := origMax
	if restore == 56 {
		// leftover from previous experiment; use a sane floor
		restore = 28
	}
	setRace(func(r *pb.UnionRacePolicy) { r.MaxTaskScore = restore })
	fmt.Printf("\ndone: restored max_task_score=%d\n", restore)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

type authHTTPClient struct {
	token string
}

func (c *authHTTPClient) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	return http.DefaultClient.Do(req)
}
