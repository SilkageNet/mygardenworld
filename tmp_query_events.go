package main

import (
	"database/sql"
	"fmt"
	"strings"
	_ "modernc.org/sqlite"
)

func main() {
	db, _ := sql.Open("sqlite", "e:/work/mygardenworld/data-b/garden.db")
	defer db.Close()
	rows, _ := db.Query(`SELECT id, kind, label, payload_json FROM event_log WHERE account_id=1 AND payload_json LIKE '%cardCollectData%' ORDER BY id DESC LIMIT 5`)
	for rows.Next() {
		var id int64; var kind, label, payload sql.NullString
		rows.Scan(&id, &kind, &label, &payload)
		fmt.Printf("id=%d kind=%s label=%s\n", id, kind.String, label.String)
		if payload.Valid { dumpCardCollect(payload.String) }
	}
	rows2, _ := db.Query(`SELECT id, payload_json FROM event_log WHERE account_id=1 AND payload_json LIKE '%luckyCard%' ORDER BY id DESC LIMIT 5`)
	for rows2.Next() {
		var id int64; var payload sql.NullString
		rows2.Scan(&id, &payload)
		fmt.Printf("luckyCard id=%d\n", id)
		if payload.Valid { dumpCardCollect(payload.String) }
	}
	rows3, _ := db.Query(`SELECT id, payload_json FROM event_log WHERE account_id=1 AND payload_json LIKE '%packOpen%' ORDER BY id DESC LIMIT 5`)
	count := 0
	for rows3.Next() {
		count++
		var id int64; var payload sql.NullString
		rows3.Scan(&id, &payload)
		fmt.Printf("packOpen id=%d\n", id)
		if payload.Valid { dumpCardCollect(payload.String) }
	}
	if count == 0 { fmt.Println("no packOpen events") }
}

func dumpCardCollect(s string) {
	for _, needle := range []string{"cardCollectData", "luckyCardRcd", "luckyCardMap", "packOpen", "\"146\""} {
		idx := strings.Index(s, needle)
		if idx >= 0 {
			end := idx + 1500
			if end > len(s) { end = len(s) }
			fmt.Printf("--- %s ---\n%s\n", needle, s[idx:end])
		}
	}
}
