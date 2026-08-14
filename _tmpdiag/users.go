package main
import (
  "database/sql"
  "fmt"
  _ "modernc.org/sqlite"
)
func main() {
  db, _ := sql.Open("sqlite", `file:e:/work/mygardenworld/data/garden.db?mode=ro&_pragma=busy_timeout(5000)`)
  defer db.Close()
  rows, _ := db.Query(`SELECT id, username, email, role FROM users`)
  for rows.Next() {
    var id int64; var u,e,r string
    // try flexible
    cols, _ := rows.Columns()
    fmt.Println("cols", cols)
    break
  }
  rows.Close()
  rows, err := db.Query(`PRAGMA table_info(users)`)
  if err != nil { panic(err) }
  for rows.Next() {
    var cid int; var name, ctype string; var notnull, pk int; var dflt any
    rows.Scan(&cid,&name,&ctype,&notnull,&dflt,&pk)
    fmt.Printf("%s %s\n", name, ctype)
  }
  rows.Close()
  rows, _ = db.Query(`SELECT * FROM users`)
  cols, _ := rows.Columns()
  fmt.Println("user cols", cols)
  for rows.Next() {
    vals := make([]any, len(cols))
    ptrs := make([]any, len(cols))
    for i := range vals { ptrs[i] = &vals[i] }
    rows.Scan(ptrs...)
    for i, c := range cols {
      if c == "password_hash" || c == "password" { fmt.Printf("%s=<redacted>\n", c); continue }
      fmt.Printf("%s=%v\n", c, vals[i])
    }
  }
}
