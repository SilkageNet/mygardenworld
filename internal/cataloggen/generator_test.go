package cataloggen

import (
	"encoding/json"
	"testing"
)

func TestFindMiniIndexFields(t *testing.T) {
	text := `window.__ccbIndexJson={list:["assets/resources/config.6634d.json"]}; module.exports={version:"360.0.23"}`
	if got := findResourceConfigPathInText(text); got != "assets/resources/config.6634d.json" {
		t.Fatalf("findResourceConfigPathInText()=%q", got)
	}
	if got := findGameVersionInText(text + `; module.exports={version:"3.6.5"}`); got != "360.0.23" {
		t.Fatalf("findGameVersionInText()=%q", got)
	}
}

func TestNormalizeViewRowStripsAssetsAndKeepsText(t *testing.T) {
	row := map[string]any{
		"name":          "水滴",
		"sname":         "水滴",
		"desc":          "种花必不可少的资源~",
		"icon":          "items/water.png",
		"getWayPram":    "|shopId:106",
		"restore":       []any{[]any{json.Number("1"), json.Number("120001")}},
		"flyRegionType": json.Number("4"),
	}
	got, removed := normalizeViewRow("c_item", "7", row)
	if removed == 0 {
		t.Fatal("normalizeViewRow removed no asset fields")
	}
	if _, ok := got["icon"]; ok {
		t.Fatalf("normalizeViewRow kept icon field: %+v", got)
	}
	if got["display_name"] != "水滴" || got["get_way_param"] != "|shopId:106" || got["fly_region_type"] != json.Number("4") {
		t.Fatalf("normalizeViewRow()=%+v", got)
	}
	stacks, ok := got["restore"].([]any)
	if !ok || len(stacks) != 1 {
		t.Fatalf("restore not normalized: %+v", got["restore"])
	}
	stack, ok := stacks[0].(map[string]any)
	if !ok || stack["item_id"] != json.Number("1") || stack["count"] != json.Number("120001") {
		t.Fatalf("restore stack=%+v", stacks[0])
	}
}

func TestAssetFieldDetectionCoversResourceIdentifiers(t *testing.T) {
	fields := []string{"icon", "source_url", "mapflower", "backGround", "sp", "logoLogicFunc"}
	for _, field := range fields {
		if !isAssetField(field) {
			t.Fatalf("isAssetField(%q)=false", field)
		}
	}
	for _, field := range []string{"colorType", "seasonType", "getWayText"} {
		if isAssetField(field) {
			t.Fatalf("isAssetField(%q)=true", field)
		}
	}
}
