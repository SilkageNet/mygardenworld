package babigame

import (
	"encoding/json"
	"testing"
)

func TestDecodeFarmLandRowsUsesDecodedClientValues(t *testing.T) {
	const (
		key       = "garden"
		tableName = "c_farmLand"
		rowID     = 1025
	)
	columns := `{"$":"id","openLvl":"A","cost":"B","wasteland":"C"}`
	row := map[string]any{
		"A": encodeClientCatalogNumber(tableName, "A", rowID, 13),
		"B": []any{
			encodeClientCatalogNumber(tableName, "B", rowID, 11),
			encodeClientCatalogNumber(tableName, "B", rowID, 800),
		},
		"C": []any{
			encodeClientCatalogNumber(tableName, "C", rowID, 1401),
			5,
		},
	}
	fixture := map[string]any{
		encodeClientCatalogChars(tableName, key): map[string]any{
			"a": key,
			"m": encodeClientCatalogChars(columns, key),
			"list": []any{map[string]any{"v": map[string]any{
				"1025": row,
			}}},
		},
	}
	raw, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}

	lands, err := decodeFarmLandRows(raw)
	if err != nil {
		t.Fatalf("decodeFarmLandRows: %v", err)
	}
	if len(lands) != 1 {
		t.Fatalf("lands=%+v, want one row", lands)
	}
	got := lands[0]
	if got.ID != rowID || got.OpenLevel != 13 || !equalInt32s(got.Cost, []int32{11, 800}) || !equalInt32s(got.Wasteland, []int32{1401, 5}) {
		t.Fatalf("decoded land=%+v", got)
	}
}

func encodeClientCatalogNumber(tableName, columnCode string, rowID, decoded int) int {
	if decoded < 10 {
		return decoded
	}
	offset := (rowID%100 + clientCatalogChecksum(tableName+"#"+columnCode)) % 100
	return decoded + offset
}

func clientCatalogChecksum(value string) int {
	total := 0
	for _, r := range value {
		total += int(r)
	}
	return total % 100
}

func encodeClientCatalogChars(value, key string) string {
	codes := make([]int, 0, len(value))
	for _, r := range value {
		for i := len(key) - 1; i >= 0; i-- {
			r ^= rune(key[i])
		}
		codes = append(codes, int(r))
	}
	raw, _ := json.Marshal(codes)
	return string(raw[1 : len(raw)-1])
}

func equalInt32s(a, b []int32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
