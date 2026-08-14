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
  rows, _ := db.Query(`SELECT id, account_id, ts, kind, length(result_json), substr(result_json,1,200)
    FROM operation_log WHERE kind LIKE '%UsrRank%' OR kind LIKE '%TaskScore%' OR result_json LIKE '%"116"%'
    ORDER BY id DESC LIMIT 15`)
  for rows.Next() {
    var id, aid, n int64; var ts time.Time; var kind, head string
    rows.Scan(&id,&aid,&ts,&kind,&n,&head)
    fmt.Printf("[%s] acc=%d id=%d %s len=%d head=%s\n", ts.Local().Format("01-02 15:04"), aid, id, kind, n, head)
  }
  // any 116 in recent race ops for ye
  rows2, _ := db.Query(`SELECT id, ts, kind FROM operation_log WHERE account_id=3 AND (result_json LIKE '%"116"%' OR kind LIKE '%Rank%' OR kind LIKE '%TaskScore%') ORDER BY id DESC LIMIT 10`)
  fmt.Println("--- ye rank/score ---")
  for rows2.Next() {
    var id int64; var ts time.Time; var kind string
    rows2.Scan(&id,&ts,&kind)
    fmt.Printf("%d %s %s\n", id, ts.Local().Format("01-02 15:04:05"), kind)
  }
  // dump latest finish 110 again + check if 116 ever appeared for anyone
  var cnt int
  db.QueryRow(`SELECT COUNT(*) FROM operation_log WHERE result_json LIKE '%"116"%'`).Scan(&cnt)
  fmt.Println("ops with 116:", cnt)
  db.QueryRow(`SELECT COUNT(*) FROM operation_log WHERE kind LIKE 'fmlRace.getFmlRaceUsrRankList'`).Scan(&cnt)
  fmt.Println("getFmlRaceUsrRankList ops:", cnt)
}
