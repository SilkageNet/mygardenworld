package cataloggen

import (
	"encoding/json"
	"regexp"
	"strings"
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

func TestExtractClientProtocolFromTextSchemasAndRPCs(t *testing.T) {
	fixture := `
mo.DS.setSingle("G.ISyncData", {$usrTot:"7:IUsrTot", usrLandTot:"100:IUsrLandTot", waterwheel:"114:IWaterwheel", benefitBoxTot:"116:IBenefitBoxTot"});
mo.DS.setSingle("G.ILand", {flowerId:0, state:1, lvl:2, harvestCnt:3, nextTime:"5:Date", plantTime:"7:Date"});
mo.DS.setSingle("G.IBenefitBox", {uid:0, drawCnt:1, resetCntTime:"2:Date", uTime:"3:Date", cTime:"4:Date"});
mo.DS.setSingle("G.GS.usrLandIface.IArg_plant", {landId:0, flowerId:1});
mo.DS.setSingle("G.GS.waterwheelIface.IArg_recv", {});
this.request2("gs.usrLand.plant", {landId:t, flowerId:e}, cb);
this.request2("gs.waterwheel.recv", {}, cb);
`
	protocol, err := ExtractClientProtocolFromText(fixture)
	if err != nil {
		t.Fatalf("ExtractClientProtocolFromText: %v", err)
	}
	land := findProtocolSchema(protocol.Schemas, "G.ILand")
	if land == nil {
		t.Fatal("G.ILand schema missing")
	}
	if field := findProtocolField(land.Fields, "nextTime"); field == nil || field.Index != 5 || field.Type != "Date" {
		t.Fatalf("G.ILand.nextTime = %+v", field)
	}
	box := findProtocolSchema(protocol.Schemas, "G.IBenefitBox")
	if box == nil {
		t.Fatal("G.IBenefitBox schema missing")
	}
	if field := findProtocolField(box.Fields, "resetCntTime"); field == nil || field.Index != 2 || field.Type != "Date" {
		t.Fatalf("G.IBenefitBox.resetCntTime = %+v", field)
	}
	ns := findNamespaceSchema(protocol.NamespaceSchemas, "100")
	if ns == nil || ns.Schema != "G.IUsrLandTot" {
		t.Fatalf("namespace 100 = %+v", ns)
	}
	plant := findProtocolRPC(protocol.RPCs, "usrLand.plant")
	if plant == nil {
		t.Fatal("usrLand.plant missing")
	}
	if plant.RequestShape != protocolRequestFields || len(plant.RequestFields) != 2 ||
		plant.RequestFields[0].Name != "landId" || plant.RequestFields[1].Name != "flowerId" {
		t.Fatalf("usrLand.plant = %+v", plant)
	}
	recv := findProtocolRPC(protocol.RPCs, "waterwheel.recv")
	if recv == nil || recv.RequestShape != protocolRequestEmpty {
		t.Fatalf("waterwheel.recv = %+v", recv)
	}
}

func TestExtractClientProtocolFromTextMergesInheritedRequestFields(t *testing.T) {
	fixture := `
mo.DS.setSingle("G.GS.ZooIface.IArg_base", {traceId: 0});
mo.DS.setSingle("G.GS.ZooIface.IArg_petBase", {petId: 0}, "IArg_base");
mo.DS.setSingle("G.GS.ZooIface.IArg_addFoodstuff", {foodstuffIds: 0}, "IArg_petBase");
mo.DS.setSingle("G.GS.ZooIface.IArg_strokePet", {}, "IArg_petBase");
this.request2("gs.zoo.addFoodstuff", {petId:t, foodstuffIds:e}, cb);
this.request2("gs.zoo.strokePet", {petId:t}, cb);
`
	schemas, err := extractSchemas(fixture)
	if err != nil {
		t.Fatalf("extractSchemas: %v", err)
	}
	petBase := findProtocolSchema(schemas, "G.GS.ZooIface.IArg_petBase")
	if petBase == nil || petBase.Parent != "IArg_base" {
		t.Fatalf("pet base parent = %+v", petBase)
	}

	protocol, err := ExtractClientProtocolFromText(fixture)
	if err != nil {
		t.Fatalf("ExtractClientProtocolFromText: %v", err)
	}
	addFoodstuff := findProtocolRPC(protocol.RPCs, "zoo.addFoodstuff")
	if addFoodstuff == nil {
		t.Fatal("zoo.addFoodstuff missing")
	}
	for _, want := range []string{"traceId", "petId", "foodstuffIds"} {
		if findProtocolField(addFoodstuff.RequestFields, want) == nil {
			t.Fatalf("zoo.addFoodstuff missing inherited field %q: %+v", want, addFoodstuff.RequestFields)
		}
	}
	strokePet := findProtocolRPC(protocol.RPCs, "zoo.strokePet")
	if strokePet == nil || strokePet.RequestShape != protocolRequestFields {
		t.Fatalf("zoo.strokePet = %+v", strokePet)
	}
	for _, want := range []string{"traceId", "petId"} {
		if findProtocolField(strokePet.RequestFields, want) == nil {
			t.Fatalf("zoo.strokePet missing inherited field %q: %+v", want, strokePet.RequestFields)
		}
	}
}

func TestProtocolGeneratorInfersObservedNumericLists(t *testing.T) {
	fixture := `
mo.DS.setSingle("G.GS.opptIface.IArg_getDetailOppts", {uids: 0, extKeys: "1:CONST.OPPT.EXT_KEY[]"});
mo.DS.setSingle("G.GS.ZooIface.IArg_recvSouvenirRwd", {idxList: 0});
this.request2("gs.oppt.getDetailOppts", {uids:t, extKeys:e}, cb);
this.request2("gs.zoo.recvSouvenirRwd", {idxList:t}, cb);
`
	protocol, err := ExtractClientProtocolFromText(fixture)
	if err != nil {
		t.Fatalf("ExtractClientProtocolFromText: %v", err)
	}
	clientTypesGo, err := GenerateClientProtocolTypesGo(protocol)
	if err != nil {
		t.Fatalf("GenerateClientProtocolTypesGo: %v", err)
	}
	for _, want := range []string{
		`ExtKeys\s+RPCIDList\s+` + "`json:\"extKeys,omitempty\"`",
		`IdxList\s+RPCIDList\s+` + "`json:\"idxList,omitempty\"`",
	} {
		if !regexp.MustCompile(want).Match(clientTypesGo) {
			t.Fatalf("client protocol types output missing %q\n%s", want, clientTypesGo)
		}
	}
}

func TestProtocolGeneratorAppliesDessertCaptureOverrides(t *testing.T) {
	fixture := `
mo.DS.setSingle("G.ISyncData", {celebrityInfo:"165:ICelebrityInfo"});
mo.DS.setSingle("G.ICelebrityInfo", {celebrityTypes:0, celebrityMap:1, celebrityLikeMap:2});
mo.DS.setSingle("G.IActDessertData", {totalScore:0, mapData:"1:{[magnification: number]: IActDessertMap}"});
mo.DS.setSingle("G.IActDessertMap", {step:0, itemUse:"1:{[itemId: number]: number}", map:2, gameStatus:3, firstMerge:4, isRunning:5, totalGain:6, curId:7, score:8, lvMap:9});
this.request2("gs.act.recv", args, cb);
this.request2("gs.celebrity.getAllTypesInfo", {}, cb);
`
	protocol, err := ExtractClientProtocolFromText(fixture)
	if err != nil {
		t.Fatalf("ExtractClientProtocolFromText: %v", err)
	}

	recv := findProtocolRPC(protocol.RPCs, "act.recv")
	if recv == nil || recv.RequestShape != protocolRequestFields || len(recv.RequestFields) != 3 {
		t.Fatalf("act.recv override = %+v", recv)
	}
	for i, want := range []string{"batchId", "taskIdx", "taskId"} {
		if recv.RequestFields[i].Name != want {
			t.Fatalf("act.recv field[%d]=%+v, want %q", i, recv.RequestFields[i], want)
		}
	}
	celebrity := findProtocolRPC(protocol.RPCs, "celebrity.getAllTypesInfo")
	if celebrity == nil || celebrity.RequestShape != protocolRequestEmpty || len(celebrity.RequestFields) != 0 {
		t.Fatalf("celebrity.getAllTypesInfo override = %+v", celebrity)
	}

	legacy := findNamespaceSchema(protocol.NamespaceSchemas, "165")
	canonical := findNamespaceSchema(protocol.NamespaceSchemas, "166")
	if legacy == nil || legacy.FieldName != "celebrityInfoLegacy" || legacy.Schema != "G.ICelebrityInfo" {
		t.Fatalf("legacy celebrity namespace = %+v", legacy)
	}
	if canonical == nil || canonical.FieldName != "celebrityInfo" || canonical.Schema != "G.ICelebrityInfo" {
		t.Fatalf("canonical celebrity namespace = %+v", canonical)
	}

	dessertMap := findProtocolSchema(protocol.Schemas, "G.IActDessertMap")
	for fieldName, wantType := range map[string]string{
		"itemUse":    "{[itemId: number]: number}",
		"firstMerge": "{[level: number]: number}",
		"isRunning":  "boolean",
		"totalGain":  "{[itemId: number]: number}",
		"lvMap":      "{[level: number]: number}",
	} {
		field := findProtocolField(dessertMap.Fields, fieldName)
		if field == nil || field.Type != wantType {
			t.Fatalf("IActDessertMap.%s=%+v, want type %q", fieldName, field, wantType)
		}
	}

	clientTypesGo, err := GenerateClientProtocolTypesGo(protocol)
	if err != nil {
		t.Fatalf("GenerateClientProtocolTypesGo: %v", err)
	}
	generated := string(clientTypesGo)
	for _, want := range []string{
		"type ActRecvRequest struct",
		"TaskIdx RPCInt `json:\"taskIdx\"`",
		"type CelebrityGetAllTypesInfoRequest struct{}",
	} {
		if !strings.Contains(generated, want) {
			t.Fatalf("client protocol output missing %q\n%s", want, generated)
		}
	}
	for _, want := range []string{
		`MapData\s+map\[int32\]IActDessertMap\s+` + "`json:\"1,omitempty\"`",
		`ItemUse\s+map\[int32\]int32\s+` + "`json:\"1,omitempty\"`",
		`FirstMerge\s+map\[int32\]int32\s+` + "`json:\"4,omitempty\"`",
		`IsRunning\s+bool\s+` + "`json:\"5,omitempty\"`",
		`TotalGain\s+map\[int32\]int32\s+` + "`json:\"6,omitempty\"`",
		`LvMap\s+map\[int32\]int32\s+` + "`json:\"9,omitempty\"`",
		`CelebrityInfoLegacy\s+ICelebrityInfo\s+` + "`json:\"165,omitempty\"`",
		`CelebrityInfo\s+ICelebrityInfo\s+` + "`json:\"166,omitempty\"`",
	} {
		if !regexp.MustCompile(want).MatchString(generated) {
			t.Fatalf("client protocol output missing pattern %q\n%s", want, generated)
		}
	}
}

func TestProtocolGeneratorsStableFixture(t *testing.T) {
	fixture := `
mo.DS.setSingle("G.ISyncData", {usrLandTot:"100:IUsrLandTot", benefitBoxTot:"116:IBenefitBoxTot", freeWater:"117:IFreeWater"});
mo.DS.setSingle("G.ILand", {flowerId:0, state:1});
mo.DS.setSingle("G.IBenefitBox", {uid:0, drawCnt:1, resetCntTime:"2:Date", uTime:"3:Date", cTime:"4:Date"});
mo.DS.setSingle("G.IFreeWater", {uid:0, recvIdx:1, rTime:"2:Date", uTime:"3:Date", cTime:"4:Date"});
mo.DS.setSingle("G.GS.usrLandIface.IArg_plant", {landId:0, flowerId:1});
mo.DS.setSingle("G.GS.waterwheelIface.IArg_recv", {});
mo.DS.setSingle("G.GS.freeWaterIface.IArg_recv", {idx:0});
mo.DS.setSingle("G.GS.orderCustomerIface.IArg_genOrder", {guestNpcIdList:0});
this.request2("gs.usrLand.plant", {landId:t, flowerId:e}, cb);
this.request2("gs.waterwheel.recv", {}, cb);
this.request2("gs.freeWater.recv", {idx:t}, cb);
this.request2("gs.orderCustomer.genOrder", {guestNpcIdList:t}, cb);
`
	protocol, err := ExtractClientProtocolFromText(fixture)
	if err != nil {
		t.Fatalf("ExtractClientProtocolFromText: %v", err)
	}
	clientTypesGo, err := GenerateClientProtocolTypesGo(protocol)
	if err != nil {
		t.Fatalf("GenerateClientProtocolTypesGo: %v", err)
	}
	clientSchemaGo, err := GenerateClientProtocolSchemaGo(protocol)
	if err != nil {
		t.Fatalf("GenerateClientProtocolSchemaGo: %v", err)
	}
	clientRPCGo, err := GenerateClientRPCNamesGo(protocol)
	if err != nil {
		t.Fatalf("GenerateClientRPCNamesGo: %v", err)
	}
	facadeGo, err := GenerateRPCFacadeGo(protocol)
	if err != nil {
		t.Fatalf("GenerateRPCFacadeGo: %v", err)
	}
	for _, want := range []string{`type ILand struct`, `FlowerId int32 ` + "`json:\"0,omitempty\"`", `type IBenefitBox struct`, `ResetCntTime int64 ` + "`json:\"2,omitempty\"`", `type IFreeWater struct`, `RecvIdx []int32 ` + "`json:\"1,omitempty\"`", `type UsrLandPlantRequest struct`, `Idx RPCInt ` + "`json:\"idx\"`", `GuestNpcIdList RPCIDList ` + "`json:\"guestNpcIdList\"`"} {
		if !strings.Contains(string(clientTypesGo), want) {
			t.Fatalf("client protocol types output missing %q\n%s", want, clientTypesGo)
		}
	}
	for _, want := range []string{`Name: "G.ILand"`, `Key: "100"`, `Schema: "G.IUsrLandTot"`, `Key: "117"`, `Schema: "G.IFreeWater"`} {
		if !strings.Contains(string(clientSchemaGo), want) {
			t.Fatalf("client protocol schema output missing %q\n%s", want, clientSchemaGo)
		}
	}
	for _, want := range []string{`RPCUsrLandPlant`, `"usrLand.plant"`, `RequestFields: []string{"landId", "flowerId"}`} {
		if !strings.Contains(string(clientRPCGo), want) {
			t.Fatalf("client rpc names output missing %q\n%s", want, clientRPCGo)
		}
	}
	for _, want := range []string{`package clientrpc`, `func NewClient(c *babigame.RPCClient) *Client`, `req clientproto.UsrLandPlantRequest`, `babigame.CallRPC[clientproto.StateDelta]`} {
		if !strings.Contains(string(facadeGo), want) {
			t.Fatalf("rpc facade output missing %q\n%s", want, facadeGo)
		}
	}
}

func TestProtocolGeneratorAvoidsDuplicateStateJSONTags(t *testing.T) {
	fixture := `
mo.DS.setSingle("G.IDupe", {first:1, second:1});
`
	protocol, err := ExtractClientProtocolFromText(fixture)
	if err != nil {
		t.Fatalf("ExtractClientProtocolFromText: %v", err)
	}
	clientTypesGo, err := GenerateClientProtocolTypesGo(protocol)
	if err != nil {
		t.Fatalf("GenerateClientProtocolTypesGo: %v", err)
	}
	got := string(clientTypesGo)
	if !regexp.MustCompile("First\\s+int32 `json:\"1,omitempty\"`").MatchString(got) {
		t.Fatalf("generated first field with wrong tag:\n%s", got)
	}
	if !regexp.MustCompile("Second\\s+int32 `json:\"-\"`").MatchString(got) {
		t.Fatalf("generated duplicate-index field with wrong tag:\n%s", got)
	}
	if strings.Count(got, "`json:\"1,omitempty\"`") != 1 {
		t.Fatalf("generated duplicate JSON tags:\n%s", got)
	}
}

func findProtocolSchema(schemas []ProtocolSchema, name string) *ProtocolSchema {
	for i := range schemas {
		if schemas[i].Name == name {
			return &schemas[i]
		}
	}
	return nil
}

func findProtocolField(fields []ProtocolField, name string) *ProtocolField {
	for i := range fields {
		if fields[i].Name == name {
			return &fields[i]
		}
	}
	return nil
}

func findNamespaceSchema(schemas []NamespaceSchema, key string) *NamespaceSchema {
	for i := range schemas {
		if schemas[i].Key == key {
			return &schemas[i]
		}
	}
	return nil
}

func findProtocolRPC(rpcs []ProtocolRPC, name string) *ProtocolRPC {
	for i := range rpcs {
		if rpcs[i].Name == name {
			return &rpcs[i]
		}
	}
	return nil
}
