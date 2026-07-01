package cataloggen

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// ProtocolField is one numbered field from a client mo.DS schema.
type ProtocolField struct {
	Name  string
	Index int
	Type  string
}

// ProtocolSchema is one mo.DS.setSingle schema recovered from game.js.
type ProtocolSchema struct {
	Name   string
	Fields []ProtocolField
}

// NamespaceSchema maps a top-level v namespace key to the client state schema.
type NamespaceSchema struct {
	Key       string
	FieldName string
	Schema    string
}

// ProtocolRPC describes one gs.* or index.* RPC recovered from game.js.
type ProtocolRPC struct {
	Name          string
	Group         string
	Method        string
	RequestShape  string
	RequestFields []ProtocolField
}

// ClientProtocol is the static protocol view extracted from an unpacked client.
type ClientProtocol struct {
	Schemas          []ProtocolSchema
	NamespaceSchemas []NamespaceSchema
	RPCs             []ProtocolRPC
}

const (
	protocolRequestEmpty  = "empty"
	protocolRequestFields = "fields"
	protocolRequestRaw    = "raw"
)

// ExtractClientProtocol reads the unpacked mini-program and extracts state
// schemas plus RPC metadata from static game.js definitions.
func ExtractClientProtocol(miniRoot string) (ClientProtocol, error) {
	root, err := findMiniRoot(miniRoot)
	if err != nil {
		return ClientProtocol{}, err
	}
	text, err := readProtocolSource(root)
	if err != nil {
		return ClientProtocol{}, err
	}
	return ExtractClientProtocolFromText(text)
}

func readProtocolSource(miniRoot string) (string, error) {
	paths := []string{
		filepath.Join(miniRoot, "src", "assets", "scripts", "game.js"),
		filepath.Join(miniRoot, "common", "game.js"),
		filepath.Join(miniRoot, "game.js"),
	}
	seen := make(map[string]struct{})
	var chunks []string
	for _, path := range paths {
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		chunks = append(chunks, string(raw))
	}
	if len(chunks) == 0 {
		return "", fmt.Errorf("mini root %q has no readable game.js protocol sources", miniRoot)
	}
	return strings.Join(chunks, "\n"), nil
}

// ExtractClientProtocolFromText extracts protocol metadata from game.js text.
func ExtractClientProtocolFromText(text string) (ClientProtocol, error) {
	schemas, err := extractSchemas(text)
	if err != nil {
		return ClientProtocol{}, err
	}
	rpcs := extractRPCs(text, schemas)
	namespaces := extractNamespaceSchemas(schemas)
	stateSchemas := make([]ProtocolSchema, 0, len(schemas))
	for _, schema := range schemas {
		if isStateSchema(schema.Name) {
			stateSchemas = append(stateSchemas, schema)
		}
	}
	sort.Slice(stateSchemas, func(i, j int) bool { return stateSchemas[i].Name < stateSchemas[j].Name })
	return ClientProtocol{
		Schemas:          stateSchemas,
		NamespaceSchemas: namespaces,
		RPCs:             rpcs,
	}, nil
}

func extractSchemas(text string) ([]ProtocolSchema, error) {
	const marker = `mo.DS.setSingle(`
	var schemas []ProtocolSchema
	for searchAt := 0; ; {
		idx := strings.Index(text[searchAt:], marker)
		if idx < 0 {
			break
		}
		pos := searchAt + idx + len(marker)
		name, next, ok := parseJSStringAt(text, skipSpace(text, pos))
		if !ok {
			searchAt = pos
			continue
		}
		comma := skipSpace(text, next)
		if comma >= len(text) || text[comma] != ',' {
			searchAt = next
			continue
		}
		objStart := skipSpace(text, comma+1)
		if objStart >= len(text) || text[objStart] != '{' {
			searchAt = next
			continue
		}
		object, objEnd, ok := balancedJSBlock(text, objStart, '{', '}')
		if !ok {
			return nil, fmt.Errorf("unterminated schema object for %s", name)
		}
		fields, err := parseSchemaObject(object)
		if err != nil {
			return nil, fmt.Errorf("%s schema: %w", name, err)
		}
		schemas = append(schemas, ProtocolSchema{Name: name, Fields: fields})
		searchAt = objEnd
	}
	if len(schemas) == 0 {
		return nil, fmt.Errorf("no mo.DS.setSingle schemas found")
	}
	byName := make(map[string]ProtocolSchema, len(schemas))
	for _, schema := range schemas {
		byName[schema.Name] = schema
	}
	schemas = schemas[:0]
	for _, schema := range byName {
		schemas = append(schemas, schema)
	}
	sort.Slice(schemas, func(i, j int) bool { return schemas[i].Name < schemas[j].Name })
	return schemas, nil
}

func parseSchemaObject(object string) ([]ProtocolField, error) {
	body := strings.TrimSpace(object)
	if strings.HasPrefix(body, "{") && strings.HasSuffix(body, "}") {
		body = strings.TrimSpace(body[1 : len(body)-1])
	}
	if body == "" {
		return nil, nil
	}
	parts := splitTopLevel(body, ',')
	fields := make([]ProtocolField, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		keyRaw, valueRaw, ok := splitTopLevelOnce(part, ':')
		if !ok {
			continue
		}
		key := cleanJSKey(keyRaw)
		value := strings.TrimSpace(valueRaw)
		field, ok := parseSchemaField(key, value)
		if !ok {
			continue
		}
		fields = append(fields, field)
	}
	sortFields(fields)
	return fields, nil
}

func parseSchemaField(name, value string) (ProtocolField, bool) {
	if name == "" {
		return ProtocolField{}, false
	}
	if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
		return ProtocolField{Name: name, Index: n}, true
	}
	if s, ok := parseJSStringLiteral(value); ok {
		idx, typ, ok := strings.Cut(s, ":")
		if !ok {
			if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
				return ProtocolField{Name: name, Index: n}, true
			}
			return ProtocolField{}, false
		}
		n, err := strconv.Atoi(strings.TrimSpace(idx))
		if err != nil {
			return ProtocolField{}, false
		}
		return ProtocolField{Name: name, Index: n, Type: strings.TrimSpace(typ)}, true
	}
	return ProtocolField{}, false
}

func extractNamespaceSchemas(schemas []ProtocolSchema) []NamespaceSchema {
	var sync ProtocolSchema
	for _, schema := range schemas {
		if schema.Name == "G.ISyncData" {
			sync = schema
			break
		}
	}
	out := make([]NamespaceSchema, 0, len(sync.Fields))
	for _, field := range sync.Fields {
		if field.Index < 0 || field.Type == "" {
			continue
		}
		out = append(out, NamespaceSchema{
			Key:       strconv.Itoa(field.Index),
			FieldName: field.Name,
			Schema:    canonicalStateSchemaName(field.Type),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		li, liErr := strconv.Atoi(out[i].Key)
		lj, ljErr := strconv.Atoi(out[j].Key)
		if liErr == nil && ljErr == nil && li != lj {
			return li < lj
		}
		return out[i].Key < out[j].Key
	})
	return out
}

func extractRPCs(text string, schemas []ProtocolSchema) []ProtocolRPC {
	argByName := make(map[string]ProtocolRPC)
	argByFold := make(map[string]ProtocolRPC)
	argRe := regexp.MustCompile(`^G\.GS\.([A-Za-z0-9_$]+)Iface\.IArg_(.+)$`)
	for _, schema := range schemas {
		m := argRe.FindStringSubmatch(schema.Name)
		if m == nil {
			continue
		}
		group, method := m[1], m[2]
		name := group + "." + method
		shape := protocolRequestFields
		if len(schema.Fields) == 0 {
			shape = protocolRequestEmpty
		}
		rpc := ProtocolRPC{
			Name:          name,
			Group:         group,
			Method:        method,
			RequestShape:  shape,
			RequestFields: cloneProtocolFields(schema.Fields),
		}
		argByName[name] = rpc
		argByFold[foldRPCKey(group, method)] = rpc
	}
	byName := make(map[string]ProtocolRPC)
	requestFolds := make(map[string]struct{})
	requestRe := regexp.MustCompile(`(?:request2|requestWithErr)\s*\(\s*["']gs\.([^"']+)["']`)
	for _, match := range requestRe.FindAllStringSubmatch(text, -1) {
		if len(match) < 2 {
			continue
		}
		name := match[1]
		group, method, ok := strings.Cut(name, ".")
		if !ok || group == "" || method == "" {
			continue
		}
		requestFolds[foldRPCKey(group, method)] = struct{}{}
		if _, ok := byName[name]; ok {
			continue
		}
		if rpc, ok := argByName[name]; ok {
			byName[name] = rpc
			continue
		}
		if rpc, ok := argByFold[foldRPCKey(group, method)]; ok {
			rpc.Name = name
			rpc.Group = group
			rpc.Method = method
			byName[name] = rpc
			continue
		}
		byName[name] = ProtocolRPC{Name: name, Group: group, Method: method, RequestShape: protocolRequestRaw}
	}
	for _, rpc := range argByName {
		if _, ok := byName[rpc.Name]; ok {
			continue
		}
		if _, ok := requestFolds[foldRPCKey(rpc.Group, rpc.Method)]; ok {
			continue
		}
		byName[rpc.Name] = rpc
	}
	applyRPCOverrides(byName)
	for _, rpc := range indexRPCs() {
		byName[rpc.Name] = rpc
	}
	out := make([]ProtocolRPC, 0, len(byName))
	for _, rpc := range byName {
		sortFields(rpc.RequestFields)
		out = append(out, rpc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func applyRPCOverrides(byName map[string]ProtocolRPC) {
	overrides := []ProtocolRPC{
		{Name: "actJyCall.dailyRecv", Group: "actJyCall", Method: "dailyRecv", RequestShape: protocolRequestRaw},
		{Name: "actJyCall.draw", Group: "actJyCall", Method: "draw", RequestShape: protocolRequestRaw},
		{Name: "actJyCall.enter", Group: "actJyCall", Method: "enter", RequestShape: protocolRequestRaw},
		{
			Name: "ReapPopup.shjm", Group: "ReapPopup", Method: "shjm", RequestShape: protocolRequestFields,
			RequestFields: []ProtocolField{{Name: "point", Index: 0}},
		},
		{
			Name: "PlantRqst.zhtc", Group: "PlantRqst", Method: "zhtc", RequestShape: protocolRequestFields,
			RequestFields: []ProtocolField{{Name: "point", Index: 0}},
		},
		{
			Name: "customerOrderRqst.dkgkck", Group: "customerOrderRqst", Method: "dkgkck", RequestShape: protocolRequestFields,
			RequestFields: []ProtocolField{{Name: "point", Index: 0}},
		},
		{
			Name: "flowerOrderRqst.showR", Group: "flowerOrderRqst", Method: "showR", RequestShape: protocolRequestFields,
			RequestFields: []ProtocolField{{Name: "point", Index: 0}},
		},
		{
			Name: "waterRqst.djst", Group: "waterRqst", Method: "djst", RequestShape: protocolRequestFields,
			RequestFields: []ProtocolField{{Name: "point", Index: 0}},
		},
		{
			Name: "frd.enter", Group: "frd", Method: "enter", RequestShape: protocolRequestFields,
			RequestFields: []ProtocolField{{Name: "needBlackList", Index: 0}, {Name: "needApplyList", Index: 1}, {Name: "needFriendList", Index: 2}},
		},
		{
			Name: "homeRqst.showBird", Group: "homeRqst", Method: "showBird", RequestShape: protocolRequestFields,
			RequestFields: []ProtocolField{{Name: "time", Index: 0}},
		},
		{
			Name: "sdk.sendGoods", Group: "sdk", Method: "sendGoods", RequestShape: protocolRequestEmpty,
		},
	}
	for _, override := range overrides {
		byName[override.Name] = override
	}
}

func foldRPCKey(group, method string) string {
	return strings.ToLower(group) + "." + strings.ToLower(method)
}

func indexRPCs() []ProtocolRPC {
	loginFields := []ProtocolField{
		{Name: "aid", Index: 0}, {Name: "gsIdx", Index: 1}, {Name: "token", Index: 2},
		{Name: "osType", Index: 3}, {Name: "isNative", Index: 4}, {Name: "deviceId", Index: 5},
		{Name: "isSimulator", Index: 6}, {Name: "deviceInfo", Index: 7}, {Name: "inviter", Index: 8},
		{Name: "shareExt", Index: 9}, {Name: "version", Index: 10}, {Name: "area", Index: 11},
		{Name: "chnId", Index: 12},
	}
	return []ProtocolRPC{
		{
			Name: "index.createUsr", Group: "index", Method: "createUsr", RequestShape: protocolRequestFields,
			RequestFields: []ProtocolField{
				{Name: "aid", Index: 0}, {Name: "gsIdx", Index: 1}, {Name: "token", Index: 2},
				{Name: "isNative", Index: 3}, {Name: "nick", Index: 4}, {Name: "sex", Index: 5},
				{Name: "ico", Index: 6}, {Name: "ext", Index: 7}, {Name: "inviter", Index: 8},
			},
		},
		{Name: "index.login", Group: "index", Method: "login", RequestShape: protocolRequestFields, RequestFields: cloneProtocolFields(loginFields)},
		{Name: "index.reLogin", Group: "index", Method: "reLogin", RequestShape: protocolRequestFields, RequestFields: cloneProtocolFields(loginFields)},
	}
}

// GenerateClientProtocolTypesGo renders isolated generated protocol DTOs.
func GenerateClientProtocolTypesGo(protocol ClientProtocol) ([]byte, error) {
	groups := groupRPCs(protocol.RPCs)
	reqTypes := facadeRequestTypeNames(groups)
	var b bytes.Buffer
	b.WriteString("// Code generated by gardencatalog from tmp/mini; DO NOT EDIT.\n")
	b.WriteString("package clientproto\n\n")
	b.WriteString("import (\n\t\"encoding/json\"\n\t\"errors\"\n\t\"fmt\"\n\t\"strings\"\n)\n\n")
	b.WriteString("type RPCName string\n\n")
	b.WriteString("func (n RPCName) String() string { return string(n) }\n\n")
	b.WriteString("type RPCID = int32\n")
	b.WriteString("type RPCUID = int64\n")
	b.WriteString("type RPCInt = int32\n")
	b.WriteString("type RPCBool = bool\n")
	b.WriteString("type RPCString = string\n")
	b.WriteString("type RPCIDList = []int32\n")
	b.WriteString("type RPCUIDList = []int64\n")
	b.WriteString("type RPCStringList = []string\n")
	b.WriteString("type RPCObject = map[string]any\n")
	b.WriteString("type RPCArray = []any\n")
	b.WriteString("type RPCValue = any\n")
	b.WriteString("type RPCPoint = []any\n")
	b.WriteString("type RawValue = json.RawMessage\n\n")
	b.WriteString("type StateDelta map[string]json.RawMessage\n\n")
	b.WriteString("func (d StateDelta) Namespace(key string) json.RawMessage {\n\tif d == nil {\n\t\treturn nil\n\t}\n\treturn d[key]\n}\n\n")
	b.WriteString("type RawRequest map[string]any\n\n")
	b.WriteString("type RPCRequestShape string\n\n")
	b.WriteString("const (\n\tRPCRequestEmpty RPCRequestShape = \"empty\"\n\tRPCRequestFields RPCRequestShape = \"fields\"\n\tRPCRequestRaw RPCRequestShape = \"raw\"\n)\n\n")
	b.WriteString("type RPCSpec struct {\n\tName RPCName\n\tGroup string\n\tMethod string\n\tRequestShape RPCRequestShape\n\tRequestFields []string\n\tResponseSchema string\n}\n\n")
	b.WriteString("func NormalizeRPCName(name string) (RPCName, error) {\n\tname = strings.TrimSpace(name)\n\tname = strings.TrimPrefix(name, \"gs.\")\n\tif name == \"\" {\n\t\treturn \"\", errors.New(\"empty RPC name\")\n\t}\n\tif !strings.Contains(name, \".\") {\n\t\treturn \"\", fmt.Errorf(\"invalid RPC name %q\", name)\n\t}\n\treturn RPCName(name), nil\n}\n\n")
	writeClientIndexTypes(&b)
	for _, group := range sortedGroupNames(groups) {
		for _, rpc := range groups[group] {
			if rpc.Group == "index" && (rpc.Method == "createUsr" || rpc.Method == "login" || rpc.Method == "reLogin") {
				continue
			}
			writeClientRequestType(&b, rpc, reqTypes[rpc.Name])
		}
	}
	writeClientStateTypes(&b, protocol.Schemas)
	return format.Source(b.Bytes())
}

func writeClientIndexTypes(b *bytes.Buffer) {
	b.WriteString("type IndexCreateUsrRequest struct {\n\tAID RPCUID `json:\"aid,omitempty\"`\n\tGsIdx RPCInt `json:\"gsIdx,omitempty\"`\n\tToken RPCString `json:\"token,omitempty\"`\n\tIsNative RPCBool `json:\"isNative,omitempty\"`\n\tNick RPCString `json:\"nick,omitempty\"`\n\tSex RPCInt `json:\"sex,omitempty\"`\n\tIco RPCString `json:\"ico,omitempty\"`\n\tExt RPCObject `json:\"ext,omitempty\"`\n\tInviter RPCObject `json:\"inviter,omitempty\"`\n}\n\n")
	b.WriteString("type IndexDeviceInfo struct {\n\tOSType RPCString `json:\"osType,omitempty\"`\n\tDeviceID RPCString `json:\"deviceId,omitempty\"`\n\tIsEmulator RPCValue `json:\"isEmulator,omitempty\"`\n\tOSVersion RPCString `json:\"osVersion,omitempty\"`\n\tBrand RPCString `json:\"brand,omitempty\"`\n\tModel RPCString `json:\"model,omitempty\"`\n\tNetworkType RPCString `json:\"networkType,omitempty\"`\n\tSysLanguage RPCString `json:\"sysLanguage,omitempty\"`\n\tScreenWidthPx RPCString `json:\"screenWidthPx,omitempty\"`\n\tScreenHeightPx RPCString `json:\"screenHeightPx,omitempty\"`\n\tDeviceType RPCString `json:\"deviceType,omitempty\"`\n\tAppVersion RPCString `json:\"appVersion,omitempty\"`\n}\n\n")
	b.WriteString("type IndexLoginRequest struct {\n\tAID RPCUID `json:\"aid,omitempty\"`\n\tGsIdx RPCInt `json:\"gsIdx,omitempty\"`\n\tToken RPCString `json:\"token,omitempty\"`\n\tOSType RPCInt `json:\"osType,omitempty\"`\n\tIsNative RPCBool `json:\"isNative,omitempty\"`\n\tDeviceID RPCString `json:\"deviceId,omitempty\"`\n\tIsSimulator RPCInt `json:\"isSimulator,omitempty\"`\n\tDeviceInfo IndexDeviceInfo `json:\"deviceInfo,omitempty\"`\n\tInviter RPCObject `json:\"inviter,omitempty\"`\n\tShareExt RPCObject `json:\"shareExt,omitempty\"`\n\tVersion RPCString `json:\"version,omitempty\"`\n\tArea RPCString `json:\"area,omitempty\"`\n\tChnID RPCInt `json:\"chnId,omitempty\"`\n}\n\n")
	b.WriteString("type IndexReLoginRequest = IndexLoginRequest\n\n")
}

func writeClientRequestType(b *bytes.Buffer, rpc ProtocolRPC, reqType string) {
	switch rpc.RequestShape {
	case protocolRequestEmpty:
		fmt.Fprintf(b, "type %s struct{}\n\n", reqType)
	case protocolRequestFields:
		fmt.Fprintf(b, "type %s struct {\n", reqType)
		used := make(map[string]int)
		for _, field := range rpc.RequestFields {
			fieldName := uniqueFieldName(used, exportFieldName(field.Name))
			fmt.Fprintf(b, "\t%s %s `json:\"%s,omitempty\"`\n", fieldName, inferRPCFieldType(field), field.Name)
		}
		b.WriteString("}\n\n")
	default:
		fmt.Fprintf(b, "type %s RawRequest\n\n", reqType)
	}
}

func writeClientStateTypes(b *bytes.Buffer, schemas []ProtocolSchema) {
	known := make(map[string]string, len(schemas)*2)
	for _, schema := range schemas {
		typ := stateSchemaTypeName(schema.Name)
		known[schema.Name] = typ
		known[strings.TrimPrefix(schema.Name, "G.")] = typ
	}
	for _, schema := range schemas {
		typeName := stateSchemaTypeName(schema.Name)
		fmt.Fprintf(b, "type %s struct {\n", typeName)
		used := make(map[string]int)
		for _, field := range schema.Fields {
			fieldName := uniqueFieldName(used, exportFieldName(field.Name))
			fmt.Fprintf(b, "\t%s %s `json:\"%d,omitempty\"`\n", fieldName, inferStateFieldType(field, known), field.Index)
		}
		b.WriteString("}\n\n")
	}
}

func stateSchemaTypeName(name string) string {
	name = strings.TrimPrefix(name, "G.")
	var b strings.Builder
	for i, r := range name {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (i > 0 && r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "UnknownSchema"
	}
	out := b.String()
	runes := []rune(out)
	if runes[0] >= 'a' && runes[0] <= 'z' {
		runes[0] -= 'a' - 'A'
	}
	return string(runes)
}

func inferStateFieldType(field ProtocolField, known map[string]string) string {
	typ := strings.TrimSpace(field.Type)
	if typ == "Date" {
		return "int64"
	}
	if strings.HasSuffix(typ, "[]") {
		base := strings.TrimSuffix(typ, "[]")
		if goType, ok := known[base]; ok {
			return "[]" + goType
		}
		if goType, ok := known["G."+base]; ok {
			return "[]" + goType
		}
		return "RawValue"
	}
	if goType, ok := known[typ]; ok {
		return goType
	}
	if goType, ok := known["G."+typ]; ok {
		return goType
	}
	if strings.ContainsAny(typ, "{}[]|:") {
		return "RawValue"
	}
	lower := strings.ToLower(field.Name)
	if strings.Contains(lower, "map") || strings.Contains(lower, "list") || strings.Contains(lower, "arr") || strings.Contains(lower, "ext") || strings.Contains(lower, "data") {
		return "RawValue"
	}
	if strings.Contains(lower, "name") || strings.Contains(lower, "nick") || strings.Contains(lower, "code") || strings.Contains(lower, "ico") ||
		strings.Contains(lower, "ip") || strings.Contains(lower, "msg") || strings.Contains(lower, "sign") || strings.Contains(lower, "desc") ||
		strings.Contains(lower, "text") || strings.Contains(lower, "url") || strings.Contains(lower, "version") || strings.Contains(lower, "uuid") {
		return "string"
	}
	if lower == "uid" || lower == "aid" || strings.HasSuffix(lower, "uid") || strings.HasSuffix(lower, "aid") {
		return "int64"
	}
	return "int32"
}

// GenerateClientProtocolSchemaGo renders isolated clientproto schema metadata.
func GenerateClientProtocolSchemaGo(protocol ClientProtocol) ([]byte, error) {
	var b bytes.Buffer
	b.WriteString("// Code generated by gardencatalog from tmp/mini; DO NOT EDIT.\n")
	b.WriteString("package clientproto\n\n")
	b.WriteString("import \"sort\"\n\n")
	b.WriteString("type ClientStateSchemaField struct {\n\tName string\n\tIndex int\n\tType string\n}\n\n")
	b.WriteString("type ClientStateSchema struct {\n\tName string\n\tFields []ClientStateSchemaField\n}\n\n")
	b.WriteString("type ClientNamespaceSchema struct {\n\tKey string\n\tFieldName string\n\tSchema string\n}\n\n")
	b.WriteString("var clientStateSchemas = []ClientStateSchema{\n")
	for _, schema := range protocol.Schemas {
		fmt.Fprintf(&b, "\t{Name: %q, Fields: []ClientStateSchemaField{", schema.Name)
		for _, field := range schema.Fields {
			fmt.Fprintf(&b, "{Name:%q, Index:%d, Type:%q},", field.Name, field.Index, field.Type)
		}
		b.WriteString("}},\n")
	}
	b.WriteString("}\n\n")
	b.WriteString("var clientStateSchemaByName = func() map[string]ClientStateSchema {\n\tout := make(map[string]ClientStateSchema, len(clientStateSchemas)*2)\n\tfor _, schema := range clientStateSchemas {\n\t\tout[schema.Name] = schema\n\t\tif strings.HasPrefix(schema.Name, \"G.\") {\n\t\t\tout[strings.TrimPrefix(schema.Name, \"G.\")] = schema\n\t\t}\n\t}\n\treturn out\n}()\n\n")
	b.WriteString("var clientNamespaceSchemas = []ClientNamespaceSchema{\n")
	for _, ns := range protocol.NamespaceSchemas {
		fmt.Fprintf(&b, "\t{Key:%q, FieldName:%q, Schema:%q},\n", ns.Key, ns.FieldName, ns.Schema)
	}
	b.WriteString("}\n\n")
	b.WriteString("var clientNamespaceSchemaByKey = func() map[string]ClientNamespaceSchema {\n\tout := make(map[string]ClientNamespaceSchema, len(clientNamespaceSchemas))\n\tfor _, schema := range clientNamespaceSchemas {\n\t\tout[schema.Key] = schema\n\t}\n\treturn out\n}()\n\n")
	b.WriteString("func KnownStateSchemas() []ClientStateSchema {\n\tout := append([]ClientStateSchema(nil), clientStateSchemas...)\n\tfor i := range out {\n\t\tout[i].Fields = append([]ClientStateSchemaField(nil), out[i].Fields...)\n\t}\n\tsort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })\n\treturn out\n}\n\n")
	b.WriteString("func LookupStateSchema(name string) (ClientStateSchema, bool) {\n\tname = strings.TrimSpace(name)\n\tif name == \"\" {\n\t\treturn ClientStateSchema{}, false\n\t}\n\tschema, ok := clientStateSchemaByName[name]\n\tif !ok && !strings.HasPrefix(name, \"G.\") {\n\t\tschema, ok = clientStateSchemaByName[\"G.\"+name]\n\t}\n\tif !ok {\n\t\treturn ClientStateSchema{}, false\n\t}\n\tschema.Fields = append([]ClientStateSchemaField(nil), schema.Fields...)\n\treturn schema, true\n}\n\n")
	b.WriteString("func KnownNamespaceSchemas() []ClientNamespaceSchema {\n\tout := append([]ClientNamespaceSchema(nil), clientNamespaceSchemas...)\n\tsort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })\n\treturn out\n}\n\n")
	b.WriteString("func LookupNamespaceSchema(key string) (ClientNamespaceSchema, bool) {\n\tschema, ok := clientNamespaceSchemaByKey[strings.TrimSpace(key)]\n\treturn schema, ok\n}\n")
	raw := bytes.ReplaceAll(b.Bytes(), []byte("import \"sort\""), []byte("import (\n\t\"sort\"\n\t\"strings\"\n)"))
	return format.Source(raw)
}

// GenerateClientRPCNamesGo renders isolated clientproto RPC names and metadata.
func GenerateClientRPCNamesGo(protocol ClientProtocol) ([]byte, error) {
	consts := rpcIdentifiers(protocol.RPCs)
	var b bytes.Buffer
	b.WriteString("// Code generated by gardencatalog from tmp/mini; DO NOT EDIT.\n")
	b.WriteString("package clientproto\n\n")
	b.WriteString("import \"sort\"\n\n")
	b.WriteString("const (\n")
	for _, rpc := range protocol.RPCs {
		fmt.Fprintf(&b, "\t%s RPCName = %q\n", consts[rpc.Name], rpc.Name)
	}
	b.WriteString(")\n\n")
	b.WriteString("var gameJSRPCNames = []RPCName{\n")
	for _, rpc := range protocol.RPCs {
		fmt.Fprintf(&b, "\t%s,\n", consts[rpc.Name])
	}
	b.WriteString("}\n\n")
	b.WriteString("var gameJSRPCNameSet = func() map[RPCName]struct{} {\n\tout := make(map[RPCName]struct{}, len(gameJSRPCNames))\n\tfor _, name := range gameJSRPCNames {\n\t\tout[name] = struct{}{}\n\t}\n\treturn out\n}()\n\n")
	b.WriteString("var gameJSRPCSpecs = []RPCSpec{\n")
	for _, rpc := range protocol.RPCs {
		fmt.Fprintf(&b, "\t{Name: %s, Group: %q, Method: %q, RequestShape: %s, RequestFields: ", consts[rpc.Name], rpc.Group, rpc.Method, rpcShapeConst(rpc.RequestShape))
		if len(rpc.RequestFields) == 0 {
			b.WriteString("nil")
		} else {
			b.WriteString("[]string{")
			for _, field := range rpc.RequestFields {
				fmt.Fprintf(&b, "%q,", field.Name)
			}
			b.WriteString("}")
		}
		b.WriteString(", ResponseSchema: \"StateDelta\"},\n")
	}
	b.WriteString("}\n\n")
	b.WriteString("var gameJSRPCSpecMap = func() map[RPCName]RPCSpec {\n\tout := make(map[RPCName]RPCSpec, len(gameJSRPCSpecs))\n\tfor _, spec := range gameJSRPCSpecs {\n\t\tout[spec.Name] = spec\n\t}\n\treturn out\n}()\n\n")
	b.WriteString("// KnownRPCNames returns the RPC names statically observed in the unpacked client.\nfunc KnownRPCNames() []RPCName {\n\tout := append([]RPCName(nil), gameJSRPCNames...)\n\tsort.Slice(out, func(i, j int) bool { return out[i] < out[j] })\n\treturn out\n}\n\n")
	b.WriteString("// KnownRPCSpecs returns metadata for every statically observed game.js RPC.\nfunc KnownRPCSpecs() []RPCSpec {\n\tout := append([]RPCSpec(nil), gameJSRPCSpecs...)\n\tfor i := range out {\n\t\tout[i].RequestFields = append([]string(nil), out[i].RequestFields...)\n\t}\n\tsort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })\n\treturn out\n}\n\n")
	b.WriteString("// LookupRPCSpec returns static metadata for one observed game.js RPC.\nfunc LookupRPCSpec(name string) (RPCSpec, bool) {\n\tnormalized, err := NormalizeRPCName(name)\n\tif err != nil {\n\t\treturn RPCSpec{}, false\n\t}\n\tspec, ok := gameJSRPCSpecMap[normalized]\n\tif !ok {\n\t\treturn RPCSpec{}, false\n\t}\n\tspec.RequestFields = append([]string(nil), spec.RequestFields...)\n\treturn spec, true\n}\n\n")
	b.WriteString("// IsKnownRPCName reports whether name was observed in game.js.\nfunc IsKnownRPCName(name string) bool {\n\tnormalized, err := NormalizeRPCName(name)\n\tif err != nil {\n\t\treturn false\n\t}\n\t_, ok := gameJSRPCNameSet[normalized]\n\treturn ok\n}\n")
	return format.Source(b.Bytes())
}

// GenerateRPCFacadeGo renders the isolated generated RPC facade.
func GenerateRPCFacadeGo(protocol ClientProtocol) ([]byte, error) {
	consts := rpcIdentifiers(protocol.RPCs)
	var b bytes.Buffer
	b.WriteString("// Code generated by gardencatalog from tmp/mini; DO NOT EDIT.\n")
	b.WriteString("package clientrpc\n\n")
	b.WriteString("import (\n\t\"context\"\n\n\t\"github.com/SilkageNet/mygardenworld/internal/babigame\"\n\tclientproto \"github.com/SilkageNet/mygardenworld/internal/babigame/clientproto\"\n)\n\n")
	b.WriteString("// Code in this file is based on statically observed gs.* calls in tmp/mini game.js.\n")
	b.WriteString("// Request structs live in clientproto; this package only binds those DTOs to\n")
	b.WriteString("// babigame's websocket transport.\n\n")
	b.WriteString("type Client struct{ c *babigame.RPCClient }\n\n")
	b.WriteString("func NewClient(c *babigame.RPCClient) *Client { return &Client{c: c} }\n\n")
	b.WriteString("func (c *Client) CallStateDelta(ctx context.Context, name string, args any, opts ...babigame.RequestOption) (babigame.RPCResponse[clientproto.StateDelta], error) {\n\treturn c.c.CallStateDelta(ctx, name, args, opts...)\n}\n\n")
	groups := groupRPCs(protocol.RPCs)
	reqTypes := facadeRequestTypeNames(groups)
	for _, group := range sortedGroupNames(groups) {
		groupType := exportRPCIdent(group)
		fmt.Fprintf(&b, "// %s returns typed RPC helpers for the %s namespace.\n", groupType, group)
		fmt.Fprintf(&b, "func (c *Client) %s() %sRPC { return %sRPC{c: c.c} }\n\n", groupType, groupType, groupType)
		fmt.Fprintf(&b, "type %sRPC struct{ c *babigame.RPCClient }\n\n", groupType)
		if groupType == "Index" {
			writeIndexFacade(&b)
		}
		for _, rpc := range groups[group] {
			if groupType == "Index" && (rpc.Method == "createUsr" || rpc.Method == "login" || rpc.Method == "reLogin") {
				continue
			}
			reqType := reqTypes[rpc.Name]
			respType := strings.TrimSuffix(reqType, "Request") + "Response"
			methodName := exportRPCIdent(rpc.Method)
			fmt.Fprintf(&b, "// %s is the namespace-delta response for gs.%s.\n", respType, rpc.Name)
			fmt.Fprintf(&b, "type %s = babigame.RPCResponse[clientproto.StateDelta]\n\n", respType)
			writeFacadeMethod(&b, groupType, methodName, reqType, respType, consts[rpc.Name], rpc)
		}
	}
	return format.Source(b.Bytes())
}

func writeIndexFacade(b *bytes.Buffer) {
	b.WriteString("type IndexCreateUsrResponse = babigame.RPCResponse[clientproto.StateDelta]\n\n")
	b.WriteString("func (r IndexRPC) CreateUsr(ctx context.Context, req clientproto.IndexCreateUsrRequest, opts ...babigame.RequestOption) (IndexCreateUsrResponse, error) {\n\treturn babigame.CallRPC[clientproto.StateDelta](ctx, r.c, clientproto.RPCIndexCreateUsr, req, opts...)\n}\n\n")
	b.WriteString("type IndexLoginResponse = babigame.RPCResponse[clientproto.StateDelta]\n\n")
	b.WriteString("func (r IndexRPC) Login(ctx context.Context, req clientproto.IndexLoginRequest, opts ...babigame.RequestOption) (IndexLoginResponse, error) {\n\treturn babigame.CallRPC[clientproto.StateDelta](ctx, r.c, clientproto.RPCIndexLogin, req, opts...)\n}\n\n")
	b.WriteString("type IndexReLoginResponse = babigame.RPCResponse[clientproto.StateDelta]\n\n")
	b.WriteString("func (r IndexRPC) ReLogin(ctx context.Context, req clientproto.IndexReLoginRequest, opts ...babigame.RequestOption) (IndexReLoginResponse, error) {\n\treturn babigame.CallRPC[clientproto.StateDelta](ctx, r.c, clientproto.RPCIndexReLogin, req, opts...)\n}\n\n")
}

func writeFacadeMethod(b *bytes.Buffer, groupType, methodName, reqType, respType, constName string, rpc ProtocolRPC) {
	switch rpc.RequestShape {
	case protocolRequestEmpty:
		fmt.Fprintf(b, "// %s calls gs.%s. game.js sends an empty request object.\n", methodName, rpc.Name)
	case protocolRequestFields:
		var names []string
		for _, field := range rpc.RequestFields {
			names = append(names, field.Name)
		}
		fmt.Fprintf(b, "// %s calls gs.%s. Request fields inferred from game.js: %s.\n", methodName, rpc.Name, strings.Join(names, ", "))
	default:
		fmt.Fprintf(b, "// %s calls gs.%s. The request shape is dynamic in game.js, so pass JSON-compatible fields in the request map.\n", methodName, rpc.Name)
	}
	fmt.Fprintf(b, "func (r %sRPC) %s(ctx context.Context, req clientproto.%s, opts ...babigame.RequestOption) (%s, error) {\n", groupType, methodName, reqType, respType)
	fmt.Fprintf(b, "\treturn babigame.CallRPC[clientproto.StateDelta](ctx, r.c, clientproto.%s, req, opts...)\n", constName)
	b.WriteString("}\n\n")
}

func writeGeneratedGo(path string, raw []byte) error {
	formatted, err := format.Source(raw)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, formatted, 0o644)
}

func isStateSchema(name string) bool {
	return strings.HasPrefix(name, "G.I") && !strings.HasPrefix(name, "G.GS.")
}

func canonicalStateSchemaName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || strings.HasPrefix(name, "G.") || strings.ContainsAny(name, "[]<>|") {
		return name
	}
	if strings.HasPrefix(name, "I") {
		return "G." + name
	}
	return name
}

func cloneProtocolFields(fields []ProtocolField) []ProtocolField {
	out := append([]ProtocolField(nil), fields...)
	sortFields(out)
	return out
}

func sortFields(fields []ProtocolField) {
	sort.SliceStable(fields, func(i, j int) bool {
		if fields[i].Index != fields[j].Index {
			return fields[i].Index < fields[j].Index
		}
		return fields[i].Name < fields[j].Name
	})
}

func rpcShapeConst(shape string) string {
	switch shape {
	case protocolRequestEmpty:
		return "RPCRequestEmpty"
	case protocolRequestFields:
		return "RPCRequestFields"
	default:
		return "RPCRequestRaw"
	}
}

func rpcIdentifiers(rpcs []ProtocolRPC) map[string]string {
	out := make(map[string]string, len(rpcs))
	used := make(map[string]int, len(rpcs))
	for _, rpc := range rpcs {
		base := "RPC" + exportRPCIdent(rpc.Group) + exportRPCIdent(rpc.Method)
		n := used[base]
		used[base] = n + 1
		if n > 0 {
			base = fmt.Sprintf("%s%d", base, n+1)
		}
		out[rpc.Name] = base
	}
	return out
}

func groupRPCs(rpcs []ProtocolRPC) map[string][]ProtocolRPC {
	out := make(map[string][]ProtocolRPC)
	for _, rpc := range rpcs {
		group := exportRPCIdent(rpc.Group)
		method := exportRPCIdent(rpc.Method)
		items := out[group]
		replaced := false
		for i := range items {
			if exportRPCIdent(items[i].Method) != method {
				continue
			}
			if preferFacadeRPC(rpc, items[i]) {
				items[i] = rpc
			}
			replaced = true
			break
		}
		if !replaced {
			items = append(items, rpc)
		}
		out[group] = items
	}
	for group := range out {
		sort.Slice(out[group], func(i, j int) bool { return out[group][i].Method < out[group][j].Method })
	}
	return out
}

func facadeRequestTypeNames(groups map[string][]ProtocolRPC) map[string]string {
	out := make(map[string]string)
	usedTypeNames := make(map[string]int)
	for _, group := range sortedGroupNames(groups) {
		for _, rpc := range groups[group] {
			base := exportRPCIdent(rpc.Group) + exportRPCIdent(rpc.Method) + "Request"
			out[rpc.Name] = uniqueTypeName(usedTypeNames, base)
		}
	}
	return out
}

func preferFacadeRPC(next, current ProtocolRPC) bool {
	nextLower := startsLower(next.Group)
	currentLower := startsLower(current.Group)
	if nextLower != currentLower {
		return nextLower
	}
	nextFields := next.RequestShape == protocolRequestFields || next.RequestShape == protocolRequestEmpty
	currentFields := current.RequestShape == protocolRequestFields || current.RequestShape == protocolRequestEmpty
	if nextFields != currentFields {
		return nextFields
	}
	return next.Name < current.Name
}

func startsLower(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return unicode.IsLower(r)
		}
	}
	return false
}

func sortedGroupNames(groups map[string][]ProtocolRPC) []string {
	out := make([]string, 0, len(groups))
	for group := range groups {
		out = append(out, group)
	}
	sort.Strings(out)
	return out
}

func uniqueTypeName(used map[string]int, base string) string {
	n := used[base]
	used[base] = n + 1
	if n == 0 {
		return base
	}
	return fmt.Sprintf("%s%d", base, n+1)
}

func uniqueFieldName(used map[string]int, base string) string {
	n := used[base]
	used[base] = n + 1
	if n == 0 {
		return base
	}
	return fmt.Sprintf("%s%d", base, n+1)
}

func exportRPCIdent(name string) string {
	tokens := identTokens(name)
	var b strings.Builder
	for _, token := range tokens {
		if token == "" {
			continue
		}
		lower := strings.ToLower(token)
		if repl, ok := rpcAcronym(lower); ok {
			b.WriteString(repl)
			continue
		}
		b.WriteString(upperFirst(lower))
	}
	if b.Len() == 0 {
		return "X"
	}
	return b.String()
}

func exportFieldName(name string) string {
	special := map[string]string{
		"id": "ID", "uid": "UID", "uids": "UIDs", "aid": "AID", "dUid": "DUID", "npcId": "NPCId",
		"chnId": "ChnID", "icoMD5": "IcoMD5", "osType": "OSType", "deviceId": "DeviceID",
	}
	if v, ok := special[name]; ok {
		return v
	}
	tokens := identTokens(name)
	var b strings.Builder
	for _, token := range tokens {
		if token == "" {
			continue
		}
		if isAllUpper(token) && len(token) > 1 {
			b.WriteString(token)
			continue
		}
		b.WriteString(upperFirst(strings.ToLower(token)))
	}
	if b.Len() == 0 {
		return "X"
	}
	return b.String()
}

func identTokens(s string) []string {
	var tokens []string
	var buf []rune
	flush := func() {
		if len(buf) > 0 {
			tokens = append(tokens, string(buf))
			buf = nil
		}
	}
	runes := []rune(s)
	for i, r := range runes {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			flush()
			continue
		}
		if len(buf) > 0 {
			prev := buf[len(buf)-1]
			var next rune
			if i+1 < len(runes) {
				next = runes[i+1]
			}
			if unicode.IsUpper(r) && (unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && next != 0 && unicode.IsLower(next))) {
				flush()
			}
		}
		buf = append(buf, r)
	}
	flush()
	return tokens
}

func rpcAcronym(lower string) (string, bool) {
	acronyms := map[string]string{
		"ip": "IP", "tl": "TL", "qa": "QA", "ai": "AI", "zfb": "ZFB",
	}
	v, ok := acronyms[lower]
	return v, ok
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func isAllUpper(s string) bool {
	hasLetter := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			hasLetter = true
			if unicode.IsLower(r) {
				return false
			}
		}
	}
	return hasLetter
}

func inferRPCFieldType(field ProtocolField) string {
	name := strings.TrimSpace(field.Name)
	lower := strings.ToLower(name)
	switch name {
	case "point":
		return "RPCPoint"
	case "aid", "uid", "dUid", "dstUid", "targetUid", "helpUid":
		return "RPCUID"
	case "uids", "dstUids", "frdUids":
		return "RPCUIDList"
	case "isNative", "isAnonymous", "isOpen", "isSubscribeOpen", "agree", "force", "withMb", "isAll":
		return "RPCBool"
	}
	if strings.HasSuffix(lower, "uids") || strings.Contains(lower, "uidlist") {
		return "RPCUIDList"
	}
	if strings.HasSuffix(lower, "ids") || strings.HasSuffix(lower, "idlist") || strings.HasSuffix(lower, "idarr") || strings.HasSuffix(lower, "typelist") {
		return "RPCIDList"
	}
	if strings.HasSuffix(lower, "uid") {
		return "RPCUID"
	}
	if strings.HasSuffix(lower, "id") || lower == "iid" || lower == "fid" || lower == "pid" || strings.HasSuffix(lower, "idxid") {
		return "RPCID"
	}
	if strings.HasPrefix(lower, "need") {
		return "RPCInt"
	}
	if strings.Contains(lower, "map") || strings.Contains(lower, "info") || strings.Contains(lower, "ext") || strings.Contains(lower, "args") ||
		strings.Contains(lower, "param") || strings.Contains(lower, "data") || strings.Contains(lower, "consume") || strings.Contains(lower, "setting") {
		return "RPCObject"
	}
	if strings.Contains(lower, "list") || strings.Contains(lower, "array") || strings.Contains(lower, "flowers") || strings.Contains(lower, "items") ||
		strings.Contains(lower, "cells") {
		return "RPCArray"
	}
	if strings.Contains(lower, "name") || strings.Contains(lower, "nick") || strings.Contains(lower, "token") || strings.Contains(lower, "code") ||
		strings.Contains(lower, "version") || strings.Contains(lower, "ico") || strings.Contains(lower, "msg") || strings.Contains(lower, "sign") ||
		strings.Contains(lower, "content") || strings.Contains(lower, "address") || strings.Contains(lower, "reason") || strings.Contains(lower, "keyword") ||
		strings.Contains(lower, "pwd") || strings.Contains(lower, "area") {
		return "RPCString"
	}
	return "RPCInt"
}

func skipSpace(s string, pos int) int {
	for pos < len(s) {
		switch s[pos] {
		case ' ', '\t', '\r', '\n':
			pos++
		default:
			return pos
		}
	}
	return pos
}

func parseJSStringAt(s string, pos int) (string, int, bool) {
	if pos >= len(s) || (s[pos] != '"' && s[pos] != '\'') {
		return "", pos, false
	}
	quote := s[pos]
	var b strings.Builder
	for i := pos + 1; i < len(s); i++ {
		ch := s[i]
		if ch == '\\' {
			if i+1 >= len(s) {
				return "", pos, false
			}
			b.WriteByte(s[i+1])
			i++
			continue
		}
		if ch == quote {
			return b.String(), i + 1, true
		}
		b.WriteByte(ch)
	}
	return "", pos, false
}

func parseJSStringLiteral(s string) (string, bool) {
	value, next, ok := parseJSStringAt(strings.TrimSpace(s), 0)
	if !ok || strings.TrimSpace(strings.TrimSpace(s)[next:]) != "" {
		return "", false
	}
	return value, true
}

func cleanJSKey(s string) string {
	s = strings.TrimSpace(s)
	if value, ok := parseJSStringLiteral(s); ok {
		return value
	}
	return strings.Trim(s, " \t\r\n")
}

func balancedJSBlock(s string, start int, open, close byte) (string, int, bool) {
	if start >= len(s) || s[start] != open {
		return "", start, false
	}
	depth := 0
	var quote byte
	escape := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if quote != 0 {
			if escape {
				escape = false
				continue
			}
			if ch == '\\' {
				escape = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '"' || ch == '\'' || ch == '`' {
			quote = ch
			continue
		}
		if ch == open {
			depth++
		}
		if ch == close {
			depth--
			if depth == 0 {
				return s[start : i+1], i + 1, true
			}
		}
	}
	return "", start, false
}

func splitTopLevel(s string, sep byte) []string {
	var out []string
	start := 0
	depth := 0
	var quote byte
	escape := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if quote != 0 {
			if escape {
				escape = false
				continue
			}
			if ch == '\\' {
				escape = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '"', '\'', '`':
			quote = ch
		case '{', '[', '(':
			depth++
		case '}', ']', ')':
			if depth > 0 {
				depth--
			}
		default:
			if ch == sep && depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	out = append(out, s[start:])
	return out
}

func splitTopLevelOnce(s string, sep byte) (string, string, bool) {
	depth := 0
	var quote byte
	escape := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if quote != 0 {
			if escape {
				escape = false
				continue
			}
			if ch == '\\' {
				escape = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '"', '\'', '`':
			quote = ch
		case '{', '[', '(':
			depth++
		case '}', ']', ')':
			if depth > 0 {
				depth--
			}
		default:
			if ch == sep && depth == 0 {
				return s[:i], s[i+1:], true
			}
		}
	}
	return "", "", false
}
