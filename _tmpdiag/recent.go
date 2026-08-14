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
  rows, _ := db.Query(`SELECT id, ts, kind, length(result_json),
    CASE WHEN result_json LIKE '%"110"%' THEN 1 ELSE 0 END,
    CASE WHEN result_json LIKE '%"110"%' AND result_json LIKE '%"3":%' THEN 1 ELSE 0 END
    FROM operation_log WHERE account_id=3 AND kind LIKE 'fmlRace.%' AND ts >= '2026-08-14 03:13:00'
    ORDER BY id`)
  for rows.Next() {
    var id, n, h110, h3 int64; var ts time.Time; var kind string
    rows.Scan(&id,&ts,&kind,&n,&h110,&h3)
    fmt.Printf("[%s] %d %s len=%d has110=%d hasFTask=%d\n", ts.Local().Format("15:04:05"), id, kind, n, h110, h3)
  }
}
