package main
import (
  "database/sql"
  "encoding/json"
  "fmt"
  _ "modernc.org/sqlite"
)
func main() {
  db, _ := sql.Open("sqlite", `file:e:/work/mygardenworld/data/garden.db?mode=ro&_pragma=busy_timeout(10000)`)
  defer db.Close()
  var r string
  db.QueryRow(`SELECT result_json FROM operation_log WHERE id=515173`).Scan(&r)
  var root map[string]json.RawMessage
  json.Unmarshal([]byte(r), &root)
  var ns25 map[string]json.RawMessage
  json.Unmarshal(root["25"], &ns25)
  keys := make([]string, 0, len(ns25))
  for k := range ns25 { keys = append(keys, k) }
  fmt.Println(keys)
}
