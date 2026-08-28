//go:build ignore

package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	pb "github.com/SilkageNet/mygardenworld/gen/mygardenworld/v1"
	"github.com/SilkageNet/mygardenworld/internal/policycfg"
	"google.golang.org/protobuf/encoding/protojson"
	_ "modernc.org/sqlite"
)

func main() {
	db, _ := sql.Open("sqlite", "file:data-b/garden.db?_pragma=journal_mode(WAL)")
	defer db.Close()

	acct := int64(2)
	var name, policyJSON sql.NullString
	db.QueryRow(`SELECT a.name, ap.policy_json FROM accounts a LEFT JOIN account_policies ap ON ap.account_id=a.id WHERE a.id=?`, acct).Scan(&name, &policyJSON)
	fmt.Printf("=== %s ===\n", name.String)

	var policy pb.Policy
	_ = protojson.Unmarshal([]byte(policyJSON.String), &policy)
	policy = *policycfg.Normalize(&policy)
	fa := policy.GetOrder().GetFlowerArt()
	fmt.Printf("automation=%v sell=%v craft=%v night_pause=%v sell_art_ids=%v\n",
		policy.GetAutomationEnabled(), fa.GetSellEnabled(), fa.GetCraftEnabled(), fa.GetSellNightPauseEnabled(), fa.GetSellArtIds())
	fmt.Printf("customer_enabled=%v reject_unavailable=%v\n",
		policy.GetOrder().GetCustomer().GetEnabled(), policy.GetOrder().GetCustomer().GetRejectUnavailableEnabled())

	fmt.Println("\n--- recent flower rack ops ---")
	rows, _ := db.Query(`SELECT ts, kind, args_json FROM operation_log WHERE account_id=? AND (kind LIKE 'flowerRack.%' OR kind LIKE 'flowerArt.%') ORDER BY id DESC LIMIT 20`, acct)
	for rows.Next() {
		var ts, kind, args string
		rows.Scan(&ts, &kind, &args)
		fmt.Printf("  %s %s %s\n", ts, kind, trunc(args, 90))
	}
	rows.Close()

	fmt.Println("\n--- recent rack events ---")
	rows2, _ := db.Query(`SELECT ts, kind, label, message FROM event_log WHERE account_id=? AND (kind LIKE '%flower_rack%' OR label LIKE '%花艺%' OR message LIKE '%花架%') ORDER BY id DESC LIMIT 15`, acct)
	for rows2.Next() {
		var ts, kind, label, msg string
		rows2.Scan(&ts, &kind, &label, &msg)
		fmt.Printf("  %s [%s] %s: %s\n", ts, kind, label, trunc(msg, 100))
	}
	rows2.Close()

	fmt.Println("\n--- blocked/deferred ops mentioning rack ---")
	rows3, _ := db.Query(`SELECT ts, kind, message FROM event_log WHERE account_id=? AND (message LIKE '%花架%' OR message LIKE '%flowerRack%' OR message LIKE '%花艺上架%') ORDER BY id DESC LIMIT 10`, acct)
	for rows3.Next() {
		var ts, kind, msg string
		rows3.Scan(&ts, &kind, &msg)
		fmt.Printf("  %s %s: %s\n", ts, kind, trunc(msg, 120))
	}
	rows3.Close()

	// Try snapshot from latest operation result if any state in event payloads - skip for now
	now := time.Now()
	sh := now.In(time.FixedZone("Asia/Shanghai", 8*60*60))
	fmt.Printf("\nShanghai now: %s hour=%d auto_list_active=%v\n", sh.Format(time.RFC3339), sh.Hour(),
		automationFlowerArtAutoListActive(fa, now))
}

func automationFlowerArtAutoListActive(fa *pb.FlowerArtPolicy, now time.Time) bool {
	if fa == nil || !fa.GetSellEnabled() {
		return false
	}
	if !fa.GetSellNightPauseEnabled() {
		return true
	}
	return now.In(time.FixedZone("Asia/Shanghai", 8*60*60)).Hour() >= 8
}

func trunc(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
