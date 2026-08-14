package main
import (
  "database/sql"
  "encoding/json"
  "fmt"
  "time"
  _ "modernc.org/sqlite"
)
func main() {
  db, _ := sql.Open("sqlite", `file:e:/work/mygardenworld/data/garden.db?mode=ro&_pragma=busy_timeout(5000)`)
  defer db.Close()
  rows, _ := db.Query(`SELECT id, ts, kind, length(result_json), substr(result_json,1,180)
    FROM operation_log WHERE account_id=3 AND kind LIKE 'fmlRace.%' AND ts >= '2026-08-14 03:34:00'
    ORDER BY id`)
  for rows.Next() {
    var id, n int64; var ts time.Time; var kind, head string
    rows.Scan(&id,&ts,&kind,&n,&head)
    fmt.Printf("[%s] %d %s len=%d\n  %s\n", ts.Local().Format("15:04:05"), id, kind, n, head)
  }

  // Dump any UsrRank result fully analyzing 116
  rows2, _ := db.Query(`SELECT id, ts, result_json FROM operation_log
    WHERE account_id=3 AND kind='fmlRace.getFmlRaceUsrRankList' ORDER BY id DESC LIMIT 3`)
  for rows2.Next() {
    var id int64; var ts time.Time; var raw string
    rows2.Scan(&id,&ts,&raw)
    fmt.Printf("\n==== rank %d %s ====\n", id, ts.Local().Format("15:04:05"))
    var root map[string]json.RawMessage
    json.Unmarshal([]byte(raw), &root)
    ns := root["25"]
    if ns == nil {
      // try find
      var anyRoot any
      json.Unmarshal([]byte(raw), &anyRoot)
      fmt.Printf("keys top-level / sample: %s\n", trunc(raw, 500))
      continue
    }
    var ns25 map[string]json.RawMessage
    json.Unmarshal(ns, &ns25)
    fmt.Printf("ns25 keys: ")
    for k := range ns25 { fmt.Printf("%s ", k) }
    fmt.Println()
    if raw116, ok := ns25["116"]; ok {
      fmt.Printf("116: %s\n", trunc(string(raw116), 800))
    }
    if raw110, ok := ns25["110"]; ok {
      fmt.Printf("110: %s\n", trunc(string(raw110), 400))
    }
  }
}
func trunc(s string, n int) string {
  if len(s) <= n { return s }
  return s[:n] + "..."
}
