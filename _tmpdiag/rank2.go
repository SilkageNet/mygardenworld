package main
import (
  "database/sql"
  "fmt"
  "time"
  _ "modernc.org/sqlite"
)
func main() {
  db, _ := sql.Open("sqlite", `file:e:/work/mygardenworld/data/garden.db?mode=ro&_pragma=busy_timeout(5000)`)
  defer db.Close()
  var cnt int
  db.QueryRow(`SELECT COUNT(*) FROM operation_log WHERE kind LIKE 'fmlRace.getFmlRaceUsrRankList'`).Scan(&cnt)
  fmt.Println("getFmlRaceUsrRankList ops:", cnt)
  db.QueryRow(`SELECT COUNT(*) FROM operation_log WHERE kind LIKE 'fmlRace.getFmlRaceTaskScore'`).Scan(&cnt)
  fmt.Println("getFmlRaceTaskScore ops:", cnt)
  db.QueryRow(`SELECT COUNT(*) FROM operation_log WHERE kind LIKE 'fmlRace.getTaskLogList'`).Scan(&cnt)
  fmt.Println("getTaskLogList ops:", cnt)
  db.QueryRow(`SELECT COUNT(*) FROM operation_log WHERE result_json LIKE '%"25":{%"116"%'`).Scan(&cnt)
  fmt.Println(`result with 25..116:`, cnt)
  // Also check login/bootstrap payloads for field 110 with fTaskNum for ye
  rows, _ := db.Query(`SELECT id, ts, kind, length(result_json) FROM operation_log
    WHERE account_id=3 AND result_json LIKE '%"110"%' AND result_json LIKE '%"3":%'
    ORDER BY id DESC LIMIT 10`)
  fmt.Println("--- ye 110 with field 3 ---")
  for rows.Next() {
    var id, n int64; var ts time.Time; var kind string
    rows.Scan(&id,&ts,&kind,&n)
    fmt.Printf("[%s] %d %s len=%d\n", ts.Local().Format("01-02 15:04:05"), id, kind, n)
  }
}
