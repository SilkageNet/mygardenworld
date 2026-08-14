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
  db, _ := sql.Open("sqlite", `file:e:/work/mygardenworld/data/garden.db?mode=ro&_pragma=busy_timeout(5000)`)
  defer db.Close()
  rows, _ := db.Query(`SELECT id, account_id, ts, kind, result_json FROM operation_log WHERE result_json LIKE '%"116"%' AND kind LIKE 'fmlRace.%' ORDER BY id DESC LIMIT 20`)
  n := 0
  for rows.Next() {
    var id, aid int64; var ts time.Time; var kind, raw string
    rows.Scan(&id,&aid,&ts,&kind,&raw)
    // find 25.116
    var root map[string]json.RawMessage
    if json.Unmarshal([]byte(raw), &root) != nil { continue }
    var ns25 map[string]json.RawMessage
    if json.Unmarshal(root["25"], &ns25) != nil { continue }
    raw116, ok := ns25["116"]
    if !ok { continue }
    n++
    fmt.Printf("\n[%s] acc=%d id=%d %s 116=%s\n", ts.Local().Format("01-02 15:04:05"), aid, id, kind, trunc(string(raw116), 400))
  }
  fmt.Println("found", n)
  // Also check if any top-level login V has 25.110 with 3 for ye
  rows2, _ := db.Query(`SELECT id, ts, kind, length(result_json) FROM operation_log WHERE account_id=3 AND kind NOT LIKE 'fmlRace.%' AND result_json LIKE '%"110"%' AND result_json LIKE '%1786291200000%' ORDER BY id DESC LIMIT 5`)
  fmt.Println("--- non-race with batch 110 ---")
  for rows2.Next() {
    var id, ln int64; var ts time.Time; var kind string
    rows2.Scan(&id,&ts,&kind,&ln)
    fmt.Printf("[%s] %d %s len=%d\n", ts.Local().Format("01-02 15:04:05"), id, kind, ln)
  }
}
func trunc(s string, n int) string {
  s = strings.ReplaceAll(s, "\n", " ")
  if len(s) <= n { return s }
  return s[:n]+"..."
}
