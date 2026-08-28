//go:build ignore

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"

	_ "modernc.org/sqlite"
)

func main() {
	db, _ := sql.Open("sqlite", "file:data-b/garden.db?_pragma=journal_mode(WAL)")
	defer db.Close()
	var policyJSON string
	db.QueryRow(`SELECT policy_json FROM account_policies WHERE account_id=2`).Scan(&policyJSON)
	var policy map[string]any
	json.Unmarshal([]byte(policyJSON), &policy)
	order, _ := policy["order"].(map[string]any)
	fa, _ := order["flower_art"].(map[string]any)
	cust, _ := order["customer"].(map[string]any)
	res, _ := order["resident"].(map[string]any)
	plant, _ := policy["plant"].(map[string]any)
	planting, _ := plant["planting"].(map[string]any)
	fmt.Printf("automation=%v\n", policy["automation_enabled"])
	fmt.Printf("flower_art: sell=%v craft=%v sell_art_ids=%v\n", fa["sell_enabled"], fa["craft_enabled"], fa["sell_art_ids"])
	fmt.Printf("customer: enabled=%v\n", cust["enabled"])
	fmt.Printf("resident: normal=%v satin=%v decorate=%v\n", res["normal_enabled"], res["satin_enabled"], res["decorate_enabled"])
	fmt.Printf("planting: auto=%v harvest=%v\n", planting["auto_enabled"], planting["auto_harvest_enabled"])
}
