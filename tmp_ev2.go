//go:build ignore
package main
import ("database/sql"; "fmt"; _ "modernc.org/sqlite")
func main(){
 db,_:=sql.Open("sqlite","data-b/garden.db"); defer db.Close()
 var n int
 db.QueryRow(`SELECT COUNT(*) FROM event_log WHERE account_id=1`).Scan(&n)
 fmt.Println("events:", n)
 rows,_:=db.Query(`SELECT domain, action, message FROM event_log WHERE account_id=1 ORDER BY id DESC LIMIT 15`)
 for rows.Next(){ var d,a,m string; rows.Scan(&d,&a,&m); fmt.Println(d,a,m[:min(len(m),120)]) }
}
func min(a,b int)int{if a<b{return a};return b}
