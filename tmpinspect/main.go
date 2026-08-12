package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"

	connect "connectrpc.com/connect"
	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1/mygardenworldv1connect"
	_ "modernc.org/sqlite"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	auth := mygardenworldv1connect.NewAuthServiceClient(http.DefaultClient, "http://127.0.0.1:50051")
	login, err := auth.Login(ctx, connect.NewRequest(&pb.LoginRequest{Username: "admin", Password: "change-me-first"}))
	if err != nil {
		panic(err)
	}
	q := mygardenworldv1connect.NewQueryServiceClient(&authHTTPClient{token: login.Msg.GetAccessToken()}, "http://127.0.0.1:50051")
	st, err := q.GetStatus(ctx, connect.NewRequest(&pb.GetStatusRequest{}))
	if err != nil {
		panic(err)
	}
	for _, a := range st.Msg.GetAccounts() {
		fmt.Printf("id=%s name=%s connected=%v auto=%v health=%s err=%q lastOp=%s\n",
			a.GetAccountId(), a.GetAccountName(), a.GetConnected(), a.GetAutomationEnabled(), a.GetHealth(), a.GetLastError(), a.GetDiagnostics().GetLastOperation())
	}

	db, _ := sql.Open("sqlite", `file:e:/work/mygardenworld/data/garden.db?_pragma=busy_timeout(5000)&mode=ro`)
	defer db.Close()
	fmt.Println("\naccount 3 session events last 10m:")
	rows, _ := db.Query(`
		SELECT ts, kind, message FROM event_log
		WHERE account_id=3 AND (domain LIKE '%session%' OR kind LIKE '%session%' OR kind='error' OR message LIKE '%登录%' OR message LIKE '%连接%')
		  AND ts >= datetime('now','-15 minutes')
		ORDER BY ts DESC LIMIT 40
	`)
	for rows.Next() {
		var ts time.Time
		var kind, msg string
		_ = rows.Scan(&ts, &kind, &msg)
		fmt.Printf("[%s] %s %s\n", ts.Local().Format("15:04:05"), kind, msg)
	}
	rows.Close()

	fmt.Println("\nany buy events account 3 last 10m:")
	rows, _ = db.Query(`
		SELECT ts, kind, action, message FROM event_log
		WHERE account_id=3 AND domain='basic.shop.cultivate' AND ts >= datetime('now','-15 minutes')
		ORDER BY ts DESC LIMIT 20
	`)
	for rows.Next() {
		var ts time.Time
		var kind, action, msg string
		_ = rows.Scan(&ts, &kind, &action, &msg)
		fmt.Printf("[%s] %s/%s %s\n", ts.Local().Format("15:04:05"), kind, action, msg)
	}
	rows.Close()
}

type authHTTPClient struct {
	token string
	c     http.Client
}

func (a *authHTTPClient) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+a.token)
	return a.c.Do(req)
}
