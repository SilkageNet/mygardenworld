//go:build ignore

package main

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func main() {
	db, _ := sql.Open("sqlite", "data-b/garden.db")
	defer db.Close()

	rows, _ := db.Query(`SELECT ts, domain, action, message FROM event_log WHERE account_id = 1 AND domain LIKE 'union.land%' ORDER BY id DESC LIMIT 40`)
	defer rows.Close()
	for rows.Next() {
		var ts int64
		var domain, action, msg string
		_ = rows.Scan(&ts, &domain, &action, &msg)
		fmt.Printf("%d %s/%s %s\n", ts, domain, action, msg)
	}

	fmt.Println("\n=== plant events mentioning 月见草 or 23108 ===")
	rows2, _ := db.Query(`SELECT ts, domain, message FROM event_log WHERE account_id = 1 AND (message LIKE '%月见草%' OR message LIKE '%23108%') ORDER BY id DESC LIMIT 20`)
	defer rows2.Close()
	for rows2.Next() {
		var ts int64
		var domain, msg string
		_ = rows2.Scan(&ts, &domain, &msg)
		fmt.Printf("%d [%s] %s\n", ts, domain, msg)
	}
}
