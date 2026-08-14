package main
import (
  "database/sql"
  "fmt"
  "time"
  _ "modernc.org/sqlite"
)
func main() {
  db, _ := sql.Open("sqlite", `file:e:/work/mygardenworld/data/garden.db?mode=ro&_pragma=busy_timeout(10000)`)
  defer db.Close()
  rows, _ := db.Query(`SELECT id, ts, kind, result_json FROM operation_log WHERE account_id=3 AND id IN (512196,515916,488543)`)
  for rows.Next() {
    var id int64; var ts time.Time; var kind, r string
    rows.Scan(&id,&ts,&kind,&r)
    fmt.Printf("\n==== %d %s %s ====\n%s\n", id, ts.Local().Format("15:04:05"), kind, r)
  }
}
