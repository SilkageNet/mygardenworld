package babigame

import (
	"context"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto"
)

func TestClientProtoNormalizeRPCNameAcceptsGameJSNames(t *testing.T) {
	name, err := clientproto.NormalizeRPCName("gs.usrLand.plant")
	if err != nil {
		t.Fatalf("NormalizeRPCName returned error: %v", err)
	}
	if name != clientproto.RPCUsrLandPlant {
		t.Fatalf("NormalizeRPCName = %q, want %q", name, clientproto.RPCUsrLandPlant)
	}
}

func TestKnownRPCNamesIncludesObservedGameJSNames(t *testing.T) {
	for _, name := range []string{"gs.usrLand.plantBatch", "index.reLogin", "gs.flowerRack.recvOneKey"} {
		if !clientproto.IsKnownRPCName(name) {
			t.Fatalf("IsKnownRPCName(%q) = false", name)
		}
	}

	names := clientproto.KnownRPCNames()
	if len(names) == 0 {
		t.Fatal("KnownRPCNames returned no names")
	}
	names[0] = "mutated.name"
	if clientproto.KnownRPCNames()[0] == "mutated.name" {
		t.Fatal("KnownRPCNames returned backing storage")
	}
}

func TestKnownRPCNamesCoverPhoneCapture20260701(t *testing.T) {
	raw, err := os.ReadFile("testdata/observed_rpcs_20260701_phone.txt")
	if err != nil {
		t.Fatalf("read observed fixture: %v", err)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		name := strings.TrimSpace(line)
		if name == "" || strings.HasPrefix(name, "#") {
			continue
		}
		if !clientproto.IsKnownRPCName(name) {
			t.Fatalf("IsKnownRPCName(%q) = false", name)
		}
	}
}

func TestNamespaceCatalogCoversObservedAndModeledNamespaces(t *testing.T) {
	keys := ObservedNamespaceKeys()
	if len(keys) == 0 {
		t.Fatal("ObservedNamespaceKeys returned no namespaces")
	}
	keys[0] = "mutated"
	if ObservedNamespaceKeys()[0] == "mutated" {
		t.Fatal("ObservedNamespaceKeys returned backing storage")
	}
	for _, key := range []string{"7", "22", "24", "28", "31", "100", "101", "104", "105", "109", "114", "116", "117", "118", "119", "129", "140"} {
		if !IsKnownNamespace(key) {
			t.Fatalf("IsKnownNamespace(%q)=false, want true", key)
		}
		if !IsModeledNamespace(key) {
			t.Fatalf("IsModeledNamespace(%q)=false, want true", key)
		}
	}
	for _, key := range []string{"165", "166"} {
		if !IsKnownNamespace(key) {
			t.Fatalf("raw namespace %s should be known", key)
		}
		if IsModeledNamespace(key) {
			t.Fatalf("raw namespace %s should not be modeled", key)
		}
	}
}

func TestGeneratedProtocolSchemaLookup(t *testing.T) {
	land, ok := clientproto.LookupStateSchema("ILand")
	if !ok {
		t.Fatal("LookupStateSchema(ILand) = false")
	}
	if field := findStateSchemaField(land.Fields, "nextTime"); field == nil || field.Index != 5 {
		t.Fatalf("ILand.nextTime = %+v", field)
	}
	box, ok := clientproto.LookupStateSchema("G.IBenefitBox")
	if !ok {
		t.Fatal("LookupStateSchema(G.IBenefitBox) = false")
	}
	if field := findStateSchemaField(box.Fields, "resetCntTime"); field == nil || field.Index != 2 {
		t.Fatalf("IBenefitBox.resetCntTime = %+v", field)
	}
	ns, ok := clientproto.LookupNamespaceSchema("114")
	if !ok {
		t.Fatal("LookupNamespaceSchema(114) = false")
	}
	if ns.Schema != "G.IWaterwheel" {
		t.Fatalf("namespace 114 schema = %q, want G.IWaterwheel", ns.Schema)
	}
}

func TestKnownRPCSpecsCoverKnownNames(t *testing.T) {
	names := clientproto.KnownRPCNames()
	specs := clientproto.KnownRPCSpecs()
	if len(specs) != len(names) {
		t.Fatalf("KnownRPCSpecs len = %d, want %d", len(specs), len(names))
	}
	for _, name := range names {
		spec, ok := clientproto.LookupRPCSpec(name.String())
		if !ok {
			t.Fatalf("LookupRPCSpec(%q) = false", name)
		}
		if spec.Name != name {
			t.Fatalf("LookupRPCSpec(%q).Name = %q", name, spec.Name)
		}
		if spec.ResponseSchema != "StateDelta" {
			t.Fatalf("LookupRPCSpec(%q).ResponseSchema = %q", name, spec.ResponseSchema)
		}
	}
}

func findStateSchemaField(fields []clientproto.ClientStateSchemaField, name string) *clientproto.ClientStateSchemaField {
	for i := range fields {
		if fields[i].Name == name {
			return &fields[i]
		}
	}
	return nil
}

func TestRPCSpecRequestFieldsAreCopied(t *testing.T) {
	spec, ok := clientproto.LookupRPCSpec("gs.usrLand.plantBatch")
	if !ok {
		t.Fatal("LookupRPCSpec(usrLand.plantBatch) = false")
	}
	if spec.RequestShape != clientproto.RPCRequestFields {
		t.Fatalf("usrLand.plantBatch shape = %q", spec.RequestShape)
	}
	if len(spec.RequestFields) != 2 || spec.RequestFields[0] != "landIds" || spec.RequestFields[1] != "flowerId" {
		t.Fatalf("usrLand.plantBatch fields = %#v", spec.RequestFields)
	}
	spec.RequestFields[0] = "mutated"
	spec, _ = clientproto.LookupRPCSpec("usrLand.plantBatch")
	if spec.RequestFields[0] == "mutated" {
		t.Fatal("LookupRPCSpec returned shared RequestFields storage")
	}

	specs := clientproto.KnownRPCSpecs()
	if len(specs) == 0 {
		t.Fatal("KnownRPCSpecs returned no specs")
	}
	specs[0].RequestFields = []string{"mutated"}
	again := clientproto.KnownRPCSpecs()
	if len(again[0].RequestFields) == 1 && again[0].RequestFields[0] == "mutated" {
		t.Fatal("KnownRPCSpecs returned shared RequestFields storage")
	}
}

func TestCelebrityRPCSpecUsesCapturedRequestShape(t *testing.T) {
	celebrity, ok := clientproto.LookupRPCSpec("celebrity.getAllTypesInfo")
	if !ok || celebrity.RequestShape != clientproto.RPCRequestRaw || len(celebrity.RequestFields) != 0 {
		t.Fatalf("celebrity.getAllTypesInfo spec = %+v, ok=%t", celebrity, ok)
	}
}

func TestRPCFacadeDoesNotExposeBareAnyRequestFields(t *testing.T) {
	raw, err := os.ReadFile("clientrpc/rpc_facade.go")
	if err != nil {
		t.Fatalf("read rpc_facade.go: %v", err)
	}
	text := string(raw)
	for _, pattern := range []string{
		`(?m)\sany\s+` + "`json:",
		`(?m)\s\[\]any\s+` + "`json:",
		`(?m)\smap\[string\]any\s+` + "`json:",
	} {
		if regexp.MustCompile(pattern).FindStringIndex(text) != nil {
			t.Fatalf("clientrpc/rpc_facade.go contains bare request field type pattern %q", pattern)
		}
	}
}

func TestGeneratedRequestStructsLiveInClientProto(t *testing.T) {
	facadeRaw, err := os.ReadFile("clientrpc/rpc_facade.go")
	if err != nil {
		t.Fatalf("read rpc_facade.go: %v", err)
	}
	facadeText := string(facadeRaw)
	if regexp.MustCompile(`(?m)^type\s+\w+Request\s+struct\s*\{`).FindStringIndex(facadeText) != nil {
		t.Fatal("clientrpc/rpc_facade.go should not define generated request structs")
	}
	if strings.Contains(facadeText, "= clientproto.UsrLandPlantRequest") {
		t.Fatal("clientrpc/rpc_facade.go should not alias generated request structs")
	}
	if !strings.Contains(facadeText, "req clientproto.UsrLandPlantRequest") {
		t.Fatal("clientrpc/rpc_facade.go should accept generated request structs from clientproto")
	}

	typesRaw, err := os.ReadFile("clientproto/types.go")
	if err != nil {
		t.Fatalf("read clientproto/types.go: %v", err)
	}
	typesText := string(typesRaw)
	for _, want := range []string{
		"package clientproto",
		"type UsrLandPlantRequest struct",
		"type IBenefitBox struct",
	} {
		if !strings.Contains(typesText, want) {
			t.Fatalf("clientproto/types.go missing %q", want)
		}
	}
}

func TestClientRPCNotConnectedDoesNotLeavePending(t *testing.T) {
	cfg, err := ConfigForChannel(ChannelIOS)
	if err != nil {
		t.Fatalf("ConfigForChannel: %v", err)
	}
	session := &Session{Cfg: cfg, RouteToken: "route-token", GsIdx: 1}
	client := NewClient(session)

	_, _, err = client.rpc(context.Background(), clientproto.RPCUsrLandWater.String(), clientproto.UsrLandWaterRequest{LandId: 1001}, session.RouteArg(), time.Second, true)
	if err == nil || !strings.Contains(err.Error(), "not connected") {
		t.Fatalf("rpc error = %v, want not connected", err)
	}

	client.mu.Lock()
	pending := len(client.pending)
	client.mu.Unlock()
	if pending != 0 {
		t.Fatalf("pending len = %d, want 0 after not connected rpc", pending)
	}
}

func TestRequestOptionsChooseOneStateApplyOwner(t *testing.T) {
	enabled, disabled := true, false
	tests := []struct {
		name          string
		applyHook     bool
		applyOverride *bool
		wantDispatch  bool
	}{
		{name: "subscriber fallback without hook", wantDispatch: true},
		{name: "apply hook owns response", applyHook: true},
		{name: "manual apply suppresses subscriber", applyOverride: &disabled},
		{name: "explicit auto apply without hook", applyOverride: &enabled, wantDispatch: true},
		{name: "explicit manual apply with hook", applyHook: true, applyOverride: &disabled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := requestOptions{applyV: tt.applyOverride}
			if got := opts.shouldDispatchNamespaces(tt.applyHook); got != tt.wantDispatch {
				t.Fatalf("shouldDispatchNamespaces()=%t, want %t", got, tt.wantDispatch)
			}
		})
	}
}
