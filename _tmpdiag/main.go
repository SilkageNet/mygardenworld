package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
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

	fmt.Println("=== accounts ===")
	rows, err := db.Query(`SELECT id, name FROM accounts ORDER BY id`)
	if err != nil {
		panic(err)
	}
	var yeID int64
	for rows.Next() {
		var id int64
		var name string
		_ = rows.Scan(&id, &name)
		fmt.Printf("%d %s\n", id, name)
		if strings.Contains(name, "叶小楠") {
			yeID = id
		}
	}
	rows.Close()
	if yeID == 0 {
		fmt.Println("叶小楠 not found")
		os.Exit(1)
	}
	fmt.Printf("\n叶小楠 account_id=%d\n", yeID)

	// Recent race ops
	fmt.Println("\n=== recent race ops ===")
	rows, err = db.Query(`
		SELECT id, ts, kind, substr(result_json,1,4000)
		FROM operation_log
		WHERE account_id=? AND kind LIKE 'fmlRace.%'
		ORDER BY id DESC LIMIT 12
	`, yeID)
	if err != nil {
		panic(err)
	}
	type opRow struct {
		id     int64
		ts     time.Time
		kind   string
		result string
	}
	var ops []opRow
	for rows.Next() {
		var r opRow
		_ = rows.Scan(&r.id, &r.ts, &r.kind, &r.result)
		ops = append(ops, r)
	}
	rows.Close()

	for i := len(ops) - 1; i >= 0; i-- {
		r := ops[i]
		fmt.Printf("\n==== [%s] id=%d %s ====\n", r.ts.Local().Format("01-02 15:04:05"), r.id, r.kind)
		analyze110(r.result)
	}

	// Also look at latest enter + getTaskList full field 110
	fmt.Println("\n=== latest enter/getTaskList field 110 dump ===")
	rows, err = db.Query(`
		SELECT id, ts, kind, result_json
		FROM operation_log
		WHERE account_id=? AND kind IN ('fmlRace.enter','fmlRace.getTaskList','fmlRace.finishTask')
		ORDER BY id DESC LIMIT 6
	`, yeID)
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var ts time.Time
		var kind, result string
		_ = rows.Scan(&id, &ts, &kind, &result)
		fmt.Printf("\n---- [%s] id=%d %s ----\n", ts.Local().Format("15:04:05"), id, kind)
		dump110(result)
		dumpBatch(result)
	}
}

func analyze110(result string) {
	var root map[string]json.RawMessage
	if json.Unmarshal([]byte(result), &root) != nil {
		fmt.Println("  (unmarshal fail)")
		return
	}
	// result may wrap as {body:{v:{25:...}}} or {v:{25:...}} etc
	ns25 := findNS25(root)
	if ns25 == nil {
		fmt.Println("  no ns25")
		return
	}
	raw110, ok := ns25["110"]
	if !ok {
		fmt.Println("  field 110 absent")
	} else {
		fmt.Printf("  110 raw: %s\n", truncate(string(raw110), 500))
		var m map[string]map[string]json.RawMessage
		if json.Unmarshal(raw110, &m) == nil {
			for k, rcd := range m {
				fmt.Printf("  110[%s]: fTaskNum(3)=%s buy(6)=%s take(7) present=%v uid(0)=%s batch(1)=%s\n",
					k, string(rcd["3"]), string(rcd["6"]), rcd["7"] != nil, string(rcd["0"]), string(rcd["1"]))
			}
		}
	}
	if raw111, ok := ns25["111"]; ok {
		fmt.Printf("  111 batch: %s\n", truncate(string(raw111), 200))
	}
	if raw117, ok := ns25["117"]; ok {
		fmt.Printf("  117 curRcd: %s\n", truncate(string(raw117), 200))
	}
	if raw0, ok := ns25["0"]; ok {
		var fml map[string]json.RawMessage
		if json.Unmarshal(raw0, &fml) == nil {
			if v, ok := fml["103"]; ok {
				fmt.Printf("  0.103 raceLvl: %s\n", string(v))
			}
		}
	}
}

func dump110(result string) {
	var root map[string]json.RawMessage
	_ = json.Unmarshal([]byte(result), &root)
	ns25 := findNS25(root)
	if ns25 == nil {
		fmt.Println("no ns25")
		return
	}
	raw110, ok := ns25["110"]
	if !ok {
		fmt.Println("110 missing")
		return
	}
	var pretty any
	_ = json.Unmarshal(raw110, &pretty)
	b, _ := json.MarshalIndent(pretty, "  ", "  ")
	fmt.Printf("110:\n  %s\n", string(b))
}

func dumpBatch(result string) {
	var root map[string]json.RawMessage
	_ = json.Unmarshal([]byte(result), &root)
	ns25 := findNS25(root)
	if ns25 == nil {
		return
	}
	for _, key := range []string{"111", "117", "112"} {
		if raw, ok := ns25[key]; ok {
			fmt.Printf("%s: %s\n", key, truncate(string(raw), 300))
		}
	}
	if raw0, ok := ns25["0"]; ok {
		var fml map[string]json.RawMessage
		if json.Unmarshal(raw0, &fml) == nil {
			if v, ok := fml["103"]; ok {
				fmt.Printf("0.103: %s\n", string(v))
			}
		}
	}
}

func findNS25(root map[string]json.RawMessage) map[string]json.RawMessage {
	// Try common wrappers
	candidates := []map[string]json.RawMessage{root}
	for _, wrap := range []string{"body", "result", "data", "v", "V"} {
		if raw, ok := root[wrap]; ok {
			var m map[string]json.RawMessage
			if json.Unmarshal(raw, &m) == nil {
				candidates = append(candidates, m)
				for _, wrap2 := range []string{"v", "V", "body"} {
					if raw2, ok := m[wrap2]; ok {
						var m2 map[string]json.RawMessage
						if json.Unmarshal(raw2, &m2) == nil {
							candidates = append(candidates, m2)
						}
					}
				}
			}
		}
	}
	for _, c := range candidates {
		if raw, ok := c["25"]; ok {
			var ns map[string]json.RawMessage
			if json.Unmarshal(raw, &ns) == nil {
				return ns
			}
		}
		// already ns25-like
		if _, ok := c["110"]; ok {
			return c
		}
		if _, ok := c["114"]; ok {
			return c
		}
	}
	// Deep search for "25"
	b, _ := json.Marshal(root)
	var anyRoot any
	_ = json.Unmarshal(b, &anyRoot)
	if found := deepFindKey(anyRoot, "25"); found != nil {
		raw, _ := json.Marshal(found)
		var ns map[string]json.RawMessage
		if json.Unmarshal(raw, &ns) == nil {
			return ns
		}
	}
	return nil
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
