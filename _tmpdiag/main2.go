package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", `file:e:/work/mygardenworld/data/garden.db?mode=ro&_pragma=busy_timeout(10000)`)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Why getTaskList unmarshal fails
	rows, err := db.Query(`
		SELECT id, ts, length(result_json), substr(result_json,1,120), substr(result_json,-80)
		FROM operation_log
		WHERE account_id=3 AND kind='fmlRace.getTaskList'
		ORDER BY id DESC LIMIT 3
	`)
	if err != nil {
		panic(err)
	}
	for rows.Next() {
		var id, n int64
		var ts time.Time
		var head, tail string
		_ = rows.Scan(&id, &ts, &n, &head, &tail)
		fmt.Printf("getTaskList id=%d len=%d head=%q tail=%q\n", id, n, head, tail)
	}
	rows.Close()

	// Find any 110 that contains fTaskNum field "3"
	fmt.Println("\n=== ops whose result contains 110 and field 3 ===")
	rows, err = db.Query(`
		SELECT id, ts, kind, result_json
		FROM operation_log
		WHERE account_id=3 AND kind LIKE 'fmlRace.%'
		  AND result_json LIKE '%"110"%'
		ORDER BY id DESC LIMIT 40
	`)
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	type hit struct {
		id   int64
		ts   time.Time
		kind string
		raw  string
	}
	var hits []hit
	for rows.Next() {
		var h hit
		_ = rows.Scan(&h.id, &h.ts, &h.kind, &h.raw)
		hits = append(hits, h)
	}
	for _, h := range hits {
		ns := find110(h.raw)
		has3 := strings.Contains(ns, `"3"`) || strings.Contains(ns, `"3":`)
		fmt.Printf("[%s] id=%d %s hasField3=%v 110=%s\n",
			h.ts.Local().Format("01-02 15:04:05"), h.id, h.kind, has3, truncate(ns, 350))
	}

	// finishTask history
	fmt.Println("\n=== finishTask / takeTask recent ===")
	rows2, err := db.Query(`
		SELECT id, ts, kind, length(result_json),
		  CASE WHEN result_json LIKE '%"110"%' THEN 1 ELSE 0 END
		FROM operation_log
		WHERE account_id=3 AND kind IN ('fmlRace.finishTask','fmlRace.takeTask','fmlRace.enter','fmlRace.giveUpTask')
		ORDER BY id DESC LIMIT 30
	`)
	if err != nil {
		panic(err)
	}
	for rows2.Next() {
		var id, n, has110 int64
		var ts time.Time
		var kind string
		_ = rows2.Scan(&id, &ts, &n, &kind, &has110)
		// oops column order
	}
	rows2.Close()
	rows2, _ = db.Query(`
		SELECT id, ts, kind, length(result_json),
		  CASE WHEN result_json LIKE '%"110"%' THEN 1 ELSE 0 END AS has110
		FROM operation_log
		WHERE account_id=3 AND kind IN ('fmlRace.finishTask','fmlRace.takeTask','fmlRace.enter','fmlRace.giveUpTask')
		ORDER BY id DESC LIMIT 40
	`)
	for rows2.Next() {
		var id, n, has110 int64
		var ts time.Time
		var kind string
		_ = rows2.Scan(&id, &ts, &kind, &n, &has110)
		fmt.Printf("[%s] id=%d %s len=%d has110=%d\n", ts.Local().Format("01-02 15:04:05"), id, kind, n, has110)
	}
	rows2.Close()
}

func find110(result string) string {
	var root any
	if json.Unmarshal([]byte(result), &root) != nil {
		return "(bad json)"
	}
	found := deepFindKey(root, "110")
	if found == nil {
		return "(missing)"
	}
	b, _ := json.Marshal(found)
	return string(b)
}

func deepFindKey(v any, key string) any {
	switch t := v.(type) {
	case map[string]any:
		if x, ok := t[key]; ok {
			return x
		}
		for _, child := range t {
			if found := deepFindKey(child, key); found != nil {
				return found
			}
		}
	case []any:
		for _, child := range t {
			if found := deepFindKey(child, key); found != nil {
				return found
			}
		}
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
