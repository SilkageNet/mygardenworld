//go:build ignore

package main

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func main() {
	db, _ := sql.Open("sqlite", "file:data-b/garden.db?_pragma=journal_mode(WAL)")
	defer db.Close()
	fmt.Println("=== acct2 ops since restart (23:53Z) ===")
	rows, _ := db.Query(`SELECT ts, kind, args_json FROM operation_log WHERE account_id=2 ORDER BY id DESC LIMIT 15`)
	for rows.Next() {
		var ts, kind, args string
		rows.Scan(&ts, &kind, &args)
		fmt.Printf("%s %s %s\n", ts, kind, args)
	}
}
