//go:build ignore

package main

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

func main() {
	db, _ := sql.Open("sqlite", "file:data-b/garden.db?_pragma=journal_mode(WAL)")
	defer db.Close()

	fmt.Println("=== 叶小楠 rack-related events since restart ===")
	rows, _ := db.Query(`SELECT ts, kind, label, message FROM event_log WHERE account_id=2 AND ts >= '2026-08-24T23:53:00Z' AND (kind LIKE '%flower%' OR kind LIKE '%rack%' OR message LIKE '%花架%' OR message LIKE '%花艺%') ORDER BY id DESC LIMIT 20`)
	for rows.Next() {
		var ts, kind, label, msg string
		rows.Scan(&ts, &kind, &label, &msg)
		if len(msg) > 120 {
			msg = msg[:120] + "..."
		}
		fmt.Printf("%s [%s] %s: %s\n", ts, kind, label, msg)
	}
	rows.Close()

	fmt.Println("\n=== all accounts last flowerRack op ===")
	rows2, _ := db.Query(`SELECT account_id, ts, kind, args_json FROM operation_log WHERE kind LIKE 'flowerRack.%' ORDER BY id DESC LIMIT 10`)
	for rows2.Next() {
		var acct int
		var ts, kind, args string
		rows2.Scan(&acct, &ts, &kind, &args)
		fmt.Printf("acct=%d %s %s %s\n", acct, ts, kind, args)
	}
	rows2.Close()

	fmt.Printf("\nNow: %v\n", time.Now().In(time.FixedZone("CST", 8*3600)))
}
