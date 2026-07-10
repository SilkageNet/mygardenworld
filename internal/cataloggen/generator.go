package cataloggen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/SilkageNet/mygardenworld/internal/clientcatalog"
)

const defaultCDNBase = "https://hygncdn.babigame.cn"

// Options controls catalog generation from an unpacked mini program.
type Options struct {
	MiniRoot           string
	CDNBase            string
	StateOutput        string
	WebOutput          string
	ProtocolPackageDir string
	RPCFacadeOutput    string
	HTTPClient         *http.Client
}

// Result describes the source data and generated catalog sizes.
type Result struct {
	MiniRoot           string
	Version            string
	CDNBase            string
	ResourceConfigURL  string
	GameDataURL        string
	StateOutput        string
	WebOutput          string
	ProtocolPackageDir string
	RPCFacadeOutput    string
	Tables             int
	Items              int
	Flowers            int
	FarmLands          int
	RemovedAssets      int
	StateSchemas       int
	NamespaceSchemas   int
	RPCs               int
}

type catalogData struct {
	Tables    map[string]StaticTable    `json:"tables"`
	Items     map[string]map[string]any `json:"items"`
	Flowers   map[string]map[string]any `json:"flowers"`
	FarmLands map[string]map[string]any `json:"farm_lands"`
}

type webCatalogData struct {
	Items     map[string]map[string]any `json:"items"`
	Flowers   map[string]map[string]any `json:"flowers"`
	FarmLands map[string]map[string]any `json:"farm_lands"`
}

// StaticTable is a fully decoded client config table.
type StaticTable struct {
	Columns map[string]string          `json:"columns"`
	Rows    map[string]json.RawMessage `json:"rows"`
}

type resourceConfig struct {
	UUIDs    []string                 `json:"uuids"`
	Paths    map[string][]any         `json:"paths"`
	Versions resourceConfigVersionMap `json:"versions"`
}

type resourceConfigVersionMap struct {
	Native []any `json:"native"`
}

type miniIndex struct {
	ResourceConfigPath string
	Version            string
	CDNBase            string
}

// Generate refreshes the backend and frontend catalog JSON files from the
// latest unpacked mini-program index and the matching Cocos resource data.
func Generate(ctx context.Context, opts Options) (Result, error) {
	if opts.MiniRoot == "" {
		opts.MiniRoot = "tmp/mini"
	}
	if opts.StateOutput == "" {
		opts.StateOutput = filepath.Join("internal", "state", "catalog_data.json")
	}
	if opts.WebOutput == "" {
		opts.WebOutput = filepath.Join("web", "src", "lib", "game", "catalog.json")
	}
	if opts.ProtocolPackageDir == "" {
		opts.ProtocolPackageDir = filepath.Join("internal", "babigame", "clientproto")
	}
	if opts.RPCFacadeOutput == "" {
		opts.RPCFacadeOutput = filepath.Join("internal", "babigame", "clientrpc", "rpc_facade.go")
	}
	httpc := opts.HTTPClient
	if httpc == nil {
		httpc = &http.Client{Timeout: 45 * time.Second}
	}

	miniRoot, err := findMiniRoot(opts.MiniRoot)
	if err != nil {
		return Result{}, err
	}
	protocol, err := ExtractClientProtocol(miniRoot)
	if err != nil {
		return Result{}, fmt.Errorf("extract client protocol: %w", err)
	}
	index, err := readMiniIndex(miniRoot)
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(opts.CDNBase) != "" {
		index.CDNBase = opts.CDNBase
	}
	index.CDNBase = strings.TrimRight(strings.TrimSpace(index.CDNBase), "/")
	if index.CDNBase == "" {
		index.CDNBase = defaultCDNBase
	}

	resourceRaw, resourceURL, err := loadResourceConfig(ctx, httpc, miniRoot, index)
	if err != nil {
		return Result{}, err
	}
	dataURL, err := gameDataURL(index.CDNBase, resourceRaw)
	if err != nil {
		return Result{}, err
	}
	dataRaw, err := httpGet(ctx, httpc, dataURL)
	if err != nil {
		return Result{}, fmt.Errorf("g-data: %w", err)
	}

	tables, removed, err := DecodeTables(dataRaw)
	if err != nil {
		return Result{}, err
	}
	items, flowers, farmLands, viewRemoved, err := buildViews(tables)
	if err != nil {
		return Result{}, err
	}
	removed += viewRemoved

	stateCatalog := catalogData{
		Tables:    tables,
		Items:     items,
		Flowers:   flowers,
		FarmLands: farmLands,
	}
	webCatalog := webCatalogData{
		Items:     items,
		Flowers:   flowers,
		FarmLands: farmLands,
	}
	if err := writeJSON(opts.StateOutput, stateCatalog); err != nil {
		return Result{}, fmt.Errorf("write state catalog: %w", err)
	}
	if err := writeJSON(opts.WebOutput, webCatalog); err != nil {
		return Result{}, fmt.Errorf("write web catalog: %w", err)
	}
	if err := os.MkdirAll(opts.ProtocolPackageDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create protocol package dir: %w", err)
	}
	protocolTypesGo, err := GenerateClientProtocolTypesGo(protocol)
	if err != nil {
		return Result{}, fmt.Errorf("generate client protocol types: %w", err)
	}
	if err := writeGeneratedGo(filepath.Join(opts.ProtocolPackageDir, "types.go"), protocolTypesGo); err != nil {
		return Result{}, fmt.Errorf("write client protocol types: %w", err)
	}
	clientSchemaGo, err := GenerateClientProtocolSchemaGo(protocol)
	if err != nil {
		return Result{}, fmt.Errorf("generate client protocol schema: %w", err)
	}
	if err := writeGeneratedGo(filepath.Join(opts.ProtocolPackageDir, "schema.go"), clientSchemaGo); err != nil {
		return Result{}, fmt.Errorf("write client protocol schema: %w", err)
	}
	clientRPCNamesGo, err := GenerateClientRPCNamesGo(protocol)
	if err != nil {
		return Result{}, fmt.Errorf("generate client rpc names: %w", err)
	}
	if err := writeGeneratedGo(filepath.Join(opts.ProtocolPackageDir, "rpc_names.go"), clientRPCNamesGo); err != nil {
		return Result{}, fmt.Errorf("write client rpc names: %w", err)
	}
	rpcFacadeGo, err := GenerateRPCFacadeGo(protocol)
	if err != nil {
		return Result{}, fmt.Errorf("generate rpc facade: %w", err)
	}
	if err := writeGeneratedGo(opts.RPCFacadeOutput, rpcFacadeGo); err != nil {
		return Result{}, fmt.Errorf("write rpc facade: %w", err)
	}

	return Result{
		MiniRoot:           miniRoot,
		Version:            index.Version,
		CDNBase:            index.CDNBase,
		ResourceConfigURL:  resourceURL,
		GameDataURL:        dataURL,
		StateOutput:        opts.StateOutput,
		WebOutput:          opts.WebOutput,
		ProtocolPackageDir: opts.ProtocolPackageDir,
		RPCFacadeOutput:    opts.RPCFacadeOutput,
		Tables:             len(tables),
		Items:              len(items),
		Flowers:            len(flowers),
		FarmLands:          len(farmLands),
		RemovedAssets:      removed,
		StateSchemas:       len(protocol.Schemas),
		NamespaceSchemas:   len(protocol.NamespaceSchemas),
		RPCs:               len(protocol.RPCs),
	}, nil
}

// DecodeTables expands the obfuscated Cocos client tables into named rows.
func DecodeTables(raw []byte) (map[string]StaticTable, int, error) {
	decodedTables, err := clientcatalog.Decode(raw)
	if err != nil {
		return nil, 0, err
	}
	tables := make(map[string]StaticTable, len(decodedTables))
	removed := 0
	for name, table := range decodedTables {
		if isAssetField(name) {
			removed++
			continue
		}
		columns := make(map[string]string, len(table.Columns))
		for field, code := range table.Columns {
			if isAssetField(field) {
				removed++
				continue
			}
			columns[field] = code
		}
		rows := make(map[string]json.RawMessage)
		for id, decodedRow := range table.Rows {
			decoded := make(map[string]any, len(decodedRow))
			for field, value := range decodedRow {
				if isAssetField(field) {
					removed++
					continue
				}
				cleaned, ok, n := sanitizeValue(value)
				removed += n
				if !ok {
					removed++
					continue
				}
				decoded[field] = cleaned
			}
			if shouldDropResourceRow(name, id, decoded) {
				removed++
				continue
			}
			rowRaw, err := marshalJSON(decoded)
			if err != nil {
				return nil, removed, fmt.Errorf("marshal %s row %s: %w", name, id, err)
			}
			rows[id] = rowRaw
		}
		tables[name] = StaticTable{Columns: columns, Rows: rows}
	}
	return tables, removed, nil
}

func findMiniRoot(root string) (string, error) {
	root = filepath.Clean(root)
	if _, err := os.Stat(filepath.Join(root, "game.js")); err == nil {
		return filepath.Abs(root)
	}
	var candidates []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if _, err := os.Stat(filepath.Join(path, "game.js")); err == nil {
			candidates = append(candidates, path)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("scan mini root %q: %w", root, err)
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("mini root %q does not contain an unpacked game.js", root)
	}
	sort.Slice(candidates, func(i, j int) bool {
		iUnpacked := strings.HasSuffix(filepath.Base(candidates[i]), "_unpacked")
		jUnpacked := strings.HasSuffix(filepath.Base(candidates[j]), "_unpacked")
		if iUnpacked != jUnpacked {
			return iUnpacked
		}
		return candidates[i] < candidates[j]
	})
	return filepath.Abs(candidates[0])
}

func readMiniIndex(miniRoot string) (miniIndex, error) {
	gameRaw, err := os.ReadFile(filepath.Join(miniRoot, "game.js"))
	if err != nil {
		return miniIndex{}, fmt.Errorf("read game.js: %w", err)
	}
	index := miniIndex{
		ResourceConfigPath: findResourceConfigPathInText(string(gameRaw)),
		Version:            findGameVersionInText(string(gameRaw)),
		CDNBase:            findCDNBaseInText(string(gameRaw)),
	}
	if index.ResourceConfigPath == "" {
		return miniIndex{}, errors.New("game.js does not reference assets/resources/config*.json")
	}
	if index.CDNBase == "" {
		index.CDNBase = discoverCDNBase(miniRoot)
	}
	return index, nil
}

func findResourceConfigPathInText(text string) string {
	re := regexp.MustCompile(`assets/resources/config\.[A-Za-z0-9]+\.json`)
	return re.FindString(text)
}

func findGameVersionInText(text string) string {
	re := regexp.MustCompile(`version:\s*"([0-9]+\.[0-9]+\.[0-9]+)"`)
	matches := re.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return ""
	}
	for i := len(matches) - 1; i >= 0; i-- {
		if majorVersion(matches[i][1]) >= 100 {
			return matches[i][1]
		}
	}
	return matches[len(matches)-1][1]
}

func majorVersion(version string) int {
	part, _, _ := strings.Cut(version, ".")
	n, _ := strconv.Atoi(part)
	return n
}

func discoverCDNBase(miniRoot string) string {
	paths := []string{
		filepath.Join(miniRoot, "common", "game.js"),
		filepath.Join(miniRoot, "src", "assets", "scripts", "game.js"),
		filepath.Join(miniRoot, "game.js"),
	}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if base := findCDNBaseInText(string(raw)); base != "" {
			return base
		}
	}
	return defaultCDNBase
}

func findCDNBaseInText(text string) string {
	re := regexp.MustCompile(`https?://[^"'\s]+`)
	for _, match := range re.FindAllString(text, -1) {
		candidate := strings.TrimRight(match, "/")
		if strings.Contains(strings.ToLower(candidate), "hygncdn") {
			return candidate
		}
	}
	return ""
}

func loadResourceConfig(ctx context.Context, httpc *http.Client, miniRoot string, index miniIndex) ([]byte, string, error) {
	localPath := filepath.Join(miniRoot, filepath.FromSlash(index.ResourceConfigPath))
	if raw, err := os.ReadFile(localPath); err == nil {
		return raw, localPath, nil
	}
	resourceURL := index.CDNBase + "/" + strings.TrimLeft(index.ResourceConfigPath, "/")
	raw, err := httpGet(ctx, httpc, resourceURL)
	if err != nil {
		return nil, resourceURL, fmt.Errorf("resource config: %w", err)
	}
	return raw, resourceURL, nil
}

func gameDataURL(base string, raw []byte) (string, error) {
	var cfg resourceConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return "", fmt.Errorf("resource config json: %w", err)
	}
	assetID := -1
	for id, entry := range cfg.Paths {
		if len(entry) == 0 {
			continue
		}
		path, _ := entry[0].(string)
		if path == "mo/zh/data/g-data" {
			assetID, _ = strconv.Atoi(id)
			break
		}
	}
	if assetID < 0 {
		return "", errors.New("resource config missing mo/zh/data/g-data")
	}
	if assetID >= len(cfg.UUIDs) {
		return "", fmt.Errorf("resource config uuid index out of range: %d", assetID)
	}
	version := versionByID(cfg.Versions.Native, assetID)
	if version == "" {
		return "", fmt.Errorf("resource config missing native version for asset %d", assetID)
	}
	uuid := decodeCocosUUID(cfg.UUIDs[assetID])
	if len(uuid) < 2 {
		return "", fmt.Errorf("invalid resource uuid for asset %d: %q", assetID, uuid)
	}
	escapedUUID := url.PathEscape(uuid)
	return fmt.Sprintf("%s/assets/resources/native/%s/%s.%s.text", strings.TrimRight(base, "/"), uuid[:2], escapedUUID, version), nil
}

func versionByID(versionArray []any, id int) string {
	for i := 0; i+1 < len(versionArray); i += 2 {
		if intFromAny(versionArray[i]) == int32(id) {
			return stringOf(versionArray[i+1])
		}
	}
	return ""
}

func httpGet(ctx context.Context, httpc *http.Client, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "mygardenworld-cataloggen/1.0")
	resp, err := httpc.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return io.ReadAll(resp.Body)
}

func buildViews(tables map[string]StaticTable) (map[string]map[string]any, map[string]map[string]any, map[string]map[string]any, int, error) {
	items, removed, err := buildView(tables, "c_item")
	if err != nil {
		return nil, nil, nil, removed, err
	}
	flowers, n, err := buildView(tables, "c_flower")
	removed += n
	if err != nil {
		return nil, nil, nil, removed, err
	}
	farmLands, n, err := buildView(tables, "c_farmLand")
	removed += n
	if err != nil {
		return nil, nil, nil, removed, err
	}
	return items, flowers, farmLands, removed, nil
}

func buildView(tables map[string]StaticTable, tableName string) (map[string]map[string]any, int, error) {
	table, ok := tables[tableName]
	if !ok {
		return nil, 0, fmt.Errorf("decoded catalog missing %s", tableName)
	}
	out := make(map[string]map[string]any, len(table.Rows))
	removed := 0
	for id, raw := range table.Rows {
		row, err := rawObject(raw)
		if err != nil {
			return nil, removed, fmt.Errorf("%s row %s: %w", tableName, id, err)
		}
		view, n := normalizeViewRow(tableName, id, row)
		removed += n
		out[id] = view
	}
	return out, removed, nil
}

func normalizeViewRow(tableName, id string, row map[string]any) (map[string]any, int) {
	out := make(map[string]any, len(row)+1)
	if n, ok := parseID(id); ok {
		out["id"] = n
	} else {
		out["id"] = id
	}
	removed := 0
	for field, value := range row {
		if strings.HasPrefix(field, "$") || isAssetField(field) {
			removed++
			continue
		}
		name := viewFieldName(tableName, field)
		if name == "" || isAssetField(name) {
			removed++
			continue
		}
		cleaned, ok, n := sanitizeValue(value)
		removed += n
		if !ok {
			removed++
			continue
		}
		out[name] = normalizeViewValue(tableName, name, cleaned)
	}
	if tableName == "c_item" {
		if display, ok := stringValue(out["display_name"]); ok && display != "" {
			out["display_name"] = display
		} else if short, ok := stringValue(out["short_name"]); ok && short != "" {
			out["display_name"] = short
		} else if name, ok := stringValue(out["name"]); ok && name != "" {
			out["display_name"] = name
		}
	}
	return out, removed
}

func viewFieldName(tableName, field string) string {
	switch tableName {
	case "c_item":
		switch field {
		case "sname":
			return "short_name"
		case "name1":
			return "display_name"
		case "desc":
			return "description"
		case "desc2":
			return "description2"
		case "bType":
			return "base_type"
		case "sType":
			return "sub_type"
		case "cls":
			return "class"
		case "useType":
			return "use_type"
		case "isBiIgnore":
			return "bio_ignore"
		case "isAutoUse":
			return "auto_use"
		case "flyRegionType":
			return "fly_region_type"
		case "sTime":
			return "start_time"
		case "mergesTo":
			return "merges_to"
		case "getWayText":
			return "get_way_text"
		case "getWayDetail":
			return "get_way_detail"
		case "getWays":
			return "get_ways"
		case "getWayPram":
			return "get_way_param"
		case "passTerm":
			return "pass_term"
		case "jumpTo":
			return "jump_to"
		case "ownNumHide":
			return "own_num_hide"
		case "transItems":
			return "trans_items"
		}
	case "c_flower":
		switch field {
		case "genusType":
			return "genus_type"
		case "colorType":
			return "color_types"
		case "seasonType":
			return "season_types"
		case "sunType":
			return "sun_types"
		case "sellRange":
			return "sell_range"
		case "exp":
			return "experience"
		case "gld":
			return "gold"
		case "seedId":
			return "seed_id"
		case "eliteId":
			return "elite_id"
		case "culCost":
			return "cultivate_cost"
		case "culTime":
			return "cultivate_time"
		case "achExpAdd":
			return "achievement_exp_add"
		case "vaseCfg":
			return "vase_config"
		case "shareRwd":
			return "share_reward"
		case "isHide":
			return "hidden"
		case "isBlock":
			return "blocked"
		case "isZooEvent":
			return "zoo_event"
		case "viceFlower":
			return "vice_flower"
		case "elvesFlower":
			return "elves_flower"
		}
	case "c_farmLand":
		if field == "openLvl" {
			return "open_level"
		}
	}
	return camelToSnake(field)
}

func normalizeViewValue(tableName, field string, value any) any {
	switch tableName {
	case "c_item":
		switch field {
		case "items", "sells", "costs", "merges_to", "merges", "reset", "restore", "trans_items":
			return itemStackList(value)
		}
	case "c_flower":
		switch field {
		case "cultivate_cost", "reward", "vase_config", "share_reward":
			return itemStackList(value)
		}
	}
	return value
}

func itemStackList(value any) any {
	values, ok := value.([]any)
	if !ok {
		return value
	}
	out := make([]any, 0, len(values))
	for _, item := range values {
		parts, ok := item.([]any)
		if !ok {
			return value
		}
		if len(parts) == 0 {
			continue
		}
		stack := map[string]any{"item_id": parts[0]}
		if len(parts) > 1 && intFromAny(parts[1]) != 0 {
			stack["count"] = parts[1]
		}
		if len(parts) > 2 {
			stack["extra"] = parts[2:]
		}
		out = append(out, stack)
	}
	return out
}

func sanitizeValue(value any) (any, bool, int) {
	switch x := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		removed := 0
		for key, item := range x {
			if isAssetField(key) {
				removed++
				continue
			}
			cleaned, ok, n := sanitizeValue(item)
			removed += n
			if !ok {
				removed++
				continue
			}
			out[key] = cleaned
		}
		return out, true, removed
	case []any:
		out := make([]any, 0, len(x))
		removed := 0
		for _, item := range x {
			cleaned, ok, n := sanitizeValue(item)
			removed += n
			if !ok {
				removed++
				continue
			}
			out = append(out, cleaned)
		}
		return out, true, removed
	case string:
		cleaned := stripImageTags(x)
		if isAssetString(cleaned) {
			return nil, false, 0
		}
		return cleaned, true, 0
	default:
		return value, true, 0
	}
}

func shouldDropResourceRow(tableName, id string, row map[string]any) bool {
	if tableName != "c_trans" {
		return false
	}
	if strings.HasPrefix(id, "R_") {
		return true
	}
	subType, _ := stringValue(row["subType"])
	return subType == "res"
}

func isAssetField(field string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(field, "_", ""))
	if normalized == "" {
		return false
	}
	exact := map[string]struct{}{
		"sp":              {},
		"spid":            {},
		"logo":            {},
		"logologicfunc":   {},
		"background":      {},
		"mapflower":       {},
		"mapflowerland":   {},
		"floweralbumland": {},
		"effect":          {},
		"coordinate":      {},
		"coordinate2":     {},
		"coordinate3":     {},
		"coordinate4":     {},
		"coordinate5":     {},
		"coordinate6":     {},
		"source":          {},
		"sourceurl":       {},
	}
	if _, ok := exact[normalized]; ok {
		return true
	}
	needles := []string{
		"icon",
		"image",
		"img",
		"sprite",
		"spine",
		"atlas",
		"texture",
		"avatar",
		"headpic",
		"headurl",
		"picture",
		"picpath",
	}
	for _, needle := range needles {
		if strings.Contains(normalized, needle) {
			return true
		}
	}
	return false
}

func stripImageTags(s string) string {
	re := regexp.MustCompile(`(?i)<img\b[^>]*>`)
	return re.ReplaceAllString(s, "")
}

func isAssetString(s string) bool {
	lower := strings.ToLower(strings.TrimSpace(s))
	if lower == "" {
		return false
	}
	extensions := []string{".png", ".jpg", ".jpeg", ".webp", ".gif", ".plist", ".atlas"}
	for _, ext := range extensions {
		if strings.Contains(lower, ext) {
			return true
		}
	}
	prefixes := []string{"image/", "images/", "texture/", "textures/", "spine/", "sprite/", "sprites/"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func rawObject(raw json.RawMessage) (map[string]any, error) {
	var out map[string]any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func marshalJSON(value any) (json.RawMessage, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

func writeJSON(path string, value any) error {
	raw, err := marshalJSONNoEscape(value)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

func marshalJSONNoEscape(value any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func decodeCocosUUID(value string) string {
	compressed := strings.Split(value, "@")[0]
	if len(compressed) != 22 {
		return value
	}
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	hexChars := []byte("0123456789abcdef")
	uuid := []byte("00000000-0000-0000-0000-000000000000")
	uuid[0] = value[0]
	uuid[1] = value[1]
	slots := make([]int, 0, 32)
	for i, ch := range uuid {
		if ch != '-' {
			slots = append(slots, i)
		}
	}
	slot := 2
	for i := 2; i < 22; i += 2 {
		hi := strings.IndexByte(alphabet, value[i])
		lo := strings.IndexByte(alphabet, value[i+1])
		if hi < 0 || lo < 0 {
			return value
		}
		uuid[slots[slot]] = hexChars[hi>>2]
		slot++
		uuid[slots[slot]] = hexChars[((hi&3)<<2)|(lo>>4)]
		slot++
		uuid[slots[slot]] = hexChars[lo&15]
		slot++
	}
	return string(uuid)
}

func intFromAny(v any) int32 {
	switch x := v.(type) {
	case int32:
		return x
	case int:
		return int32(x)
	case int64:
		return int32(x)
	case float64:
		return int32(x)
	case json.Number:
		n, _ := x.Int64()
		return int32(n)
	case string:
		n, _ := strconv.ParseInt(x, 10, 32)
		return int32(n)
	default:
		return 0
	}
}

func parseID(s string) (int32, bool) {
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, false
	}
	return int32(n), true
}

func stringOf(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case json.Number:
		return x.String()
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		return fmt.Sprint(v)
	}
}

func stringValue(v any) (string, bool) {
	s, ok := v.(string)
	return s, ok
}

func camelToSnake(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	var prevLowerOrDigit bool
	for i, r := range s {
		if r == '-' || r == ' ' {
			r = '_'
		}
		if r >= 'A' && r <= 'Z' {
			if i > 0 && prevLowerOrDigit {
				b.WriteByte('_')
			}
			b.WriteRune(r + ('a' - 'A'))
			prevLowerOrDigit = false
			continue
		}
		b.WriteRune(r)
		prevLowerOrDigit = (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
	}
	return b.String()
}
