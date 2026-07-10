package clientcatalog

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestDecodeRestoresRecursiveClientOffsets(t *testing.T) {
	key := "garden"
	fixture := map[string]any{
		encodeChars("c_storyMainChapter", key): map[string]any{
			"a": key,
			"m": encodeChars(`{"$":"id","sectionId":"F"}`, key),
			"list": []any{map[string]any{"v": map[string]any{
				"32": map[string]any{"F": []any{4109, 4110, 4111, 4112, 4113, 4114}},
			}}},
		},
		encodeChars("c_storyMainSection", key): map[string]any{
			"a": key,
			"m": encodeChars(`{"$":"id","cost":"A"}`, key),
			"list": []any{map[string]any{"v": map[string]any{
				"4101": map[string]any{"A": []any{[]any{142, 171}}},
			}}},
		},
		encodeChars("c_pearl", key): map[string]any{
			"a": key,
			"m": encodeChars(`{"$":"id","$hireItem":"H"}`, key),
			"list": []any{map[string]any{"v": map[string]any{
				"-1": map[string]any{"H": 1035},
			}}},
		},
	}
	raw, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	tables, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	chapter := tables["c_storyMainChapter"].Rows["32"]
	if got := jsonInts(chapter["sectionId"]); !reflect.DeepEqual(got, []int64{4101, 4102, 4103, 4104, 4105, 4106}) {
		t.Fatalf("section ids = %v", got)
	}
	if got, _ := chapter["id"].(json.Number).Int64(); got != 32 {
		t.Fatalf("implicit id = %v", chapter["id"])
	}

	section := tables["c_storyMainSection"].Rows["4101"]
	cost := section["cost"].([]any)[0].([]any)
	if got := jsonInts(cost); !reflect.DeepEqual(got, []int64{56, 85}) {
		t.Fatalf("section cost = %v", got)
	}

	hireItem, err := tables["c_pearl"].Rows["-1"]["$hireItem"].(json.Number).Int64()
	if err != nil || hireItem != 1003 {
		t.Fatalf("hire item = %v, err=%v", tables["c_pearl"].Rows["-1"]["$hireItem"], err)
	}
}

func encodeChars(value, key string) string {
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

func jsonInts(values any) []int64 {
	items, _ := values.([]any)
	out := make([]int64, 0, len(items))
	for _, item := range items {
		n, _ := item.(json.Number).Int64()
		out = append(out, n)
	}
	return out
}
