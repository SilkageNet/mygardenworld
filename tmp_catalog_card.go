package main

import (
	"fmt"
	"sort"
	"github.com/SilkageNet/mygardenworld/internal/state"
)

func main() {
	if table, ok := state.StaticTableByName("c_card"); ok {
		keys := make([]string, 0, len(table.Rows))
		for k := range table.Rows { keys = append(keys, k) }
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("c_card %s: %s\n", k, string(table.Rows[k]))
		}
	}
}
