package main
import (
  "database/sql"
  "fmt"
  _ "modernc.org/sqlite"
)
func main() {
  db, _ := sql.Open("sqlite", `file:e:/work/mygardenworld/data/garden.db?mode=ro&_pragma=busy_timeout(5000)`)
  defer db.Close()
  rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' ORDER BY 1`)
  if err != nil { panic(err) }
  for rows.Next() {
    var n string; rows.Scan(&n); fmt.Println(n)
  }
}
