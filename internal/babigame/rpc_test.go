package babigame

import (
	"os"
	"regexp"
	"testing"
)

func TestNormalizeRPCNameAcceptsGameJSNames(t *testing.T) {
	name, err := NormalizeRPCName("gs.usrLand.plant")
	if err != nil {
		t.Fatalf("NormalizeRPCName returned error: %v", err)
	}
	if name != RPCUsrLandPlant {
		t.Fatalf("NormalizeRPCName = %q, want %q", name, RPCUsrLandPlant)
	}
}

func TestKnownRPCNamesIncludesObservedGameJSNames(t *testing.T) {
	for _, name := range []string{"gs.usrLand.plantBatch", "index.reLogin", "gs.flowerRack.recvOneKey"} {
		if !IsKnownRPCName(name) {
			t.Fatalf("IsKnownRPCName(%q) = false", name)
		}
	}

	names := KnownRPCNames()
	if len(names) == 0 {
		t.Fatal("KnownRPCNames returned no names")
	}
	names[0] = "mutated.name"
	if KnownRPCNames()[0] == "mutated.name" {
		t.Fatal("KnownRPCNames returned backing storage")
	}
}

func TestKnownRPCSpecsCoverKnownNames(t *testing.T) {
	names := KnownRPCNames()
	specs := KnownRPCSpecs()
	if len(specs) != len(names) {
		t.Fatalf("KnownRPCSpecs len = %d, want %d", len(specs), len(names))
	}
	for _, name := range names {
		spec, ok := LookupRPCSpec(name.String())
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

func TestRPCSpecRequestFieldsAreCopied(t *testing.T) {
	spec, ok := LookupRPCSpec("gs.usrLand.plantBatch")
	if !ok {
		t.Fatal("LookupRPCSpec(usrLand.plantBatch) = false")
	}
	if spec.RequestShape != RPCRequestFields {
		t.Fatalf("usrLand.plantBatch shape = %q", spec.RequestShape)
	}
	if len(spec.RequestFields) != 2 || spec.RequestFields[0] != "landIds" || spec.RequestFields[1] != "flowerId" {
		t.Fatalf("usrLand.plantBatch fields = %#v", spec.RequestFields)
	}
	spec.RequestFields[0] = "mutated"
	spec, _ = LookupRPCSpec("usrLand.plantBatch")
	if spec.RequestFields[0] == "mutated" {
		t.Fatal("LookupRPCSpec returned shared RequestFields storage")
	}

	specs := KnownRPCSpecs()
	if len(specs) == 0 {
		t.Fatal("KnownRPCSpecs returned no specs")
	}
	specs[0].RequestFields = []string{"mutated"}
	again := KnownRPCSpecs()
	if len(again[0].RequestFields) == 1 && again[0].RequestFields[0] == "mutated" {
		t.Fatal("KnownRPCSpecs returned shared RequestFields storage")
	}
}

func TestRPCFacadeDoesNotExposeBareAnyRequestFields(t *testing.T) {
	raw, err := os.ReadFile("rpc_facade.go")
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
			t.Fatalf("rpc_facade.go contains bare request field type pattern %q", pattern)
		}
	}
}
