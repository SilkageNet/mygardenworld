package main

import (
	"database/sql"
	"fmt"
	"strings"
	_ "modernc.org/sqlite"
)

func main() {
	for _, path := range []string{"e:/work/mygardenworld/data/garden.db", "e:/work/mygardenworld/data-b/garden.db"} {
		fmt.Println("===", path, "===")
		db, _ := sql.Open("sqlite", path)
		defer db.Close()
		rows, err := db.Query("PRAGMA table_info(accounts)")
		if err == nil {
			for rows.Next() {
				var cid int; var name, ctype string; var notnull, pk int; var dflt sql.NullString
				rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
				fmt.Printf("col: %s\n", name)
			}
		}
		rows2, err := db.Query("SELECT * FROM accounts")
		if err != nil { fmt.Println(err); continue }
		cols, _ := rows2.Columns()
		fmt.Println("columns:", strings.Join(cols, ", "))
		for rows2.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals { ptrs[i] = &vals[i] }
			rows2.Scan(ptrs...)
			line := ""
			for i, c := range cols {
				line += fmt.Sprintf("%s=%v ", c, vals[i])
			}
			if strings.Contains(line, "顾") || strings.Contains(line, "萱") || strings.Contains(line, "yixuan") || strings.Contains(line, "guyixuan") {
				fmt.Println("MATCH:", line)
			}
		}
	}
}
