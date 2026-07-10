package babigame

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/SilkageNet/mygardenworld/internal/clientcatalog"
)

// FarmLandConfigRow is the runtime c_farmLand row subset needed by automation.
type FarmLandConfigRow struct {
	ID        int32
	OpenLevel int32
	Cost      []int32
	Wasteland []int32
}

// LoadFarmLandConfig loads c_farmLand from the current package entry/resource
// config, mirroring what the official client sees for this launch.
func LoadFarmLandConfig(ctx context.Context, httpc *HTTPClient, pkg PackageConfig) ([]FarmLandConfigRow, error) {
	base := firstPackageCDN(pkg)
	entryPath := pkg.EntryPath
	if entryPath == "" && pkg.GameVersion != "" {
		entryPath = "index/index-gn-mix-" + pkg.GameVersion + ".json"
	}
	if base == "" || entryPath == "" {
		return nil, fmt.Errorf("missing package cdn/entry path")
	}

	entryURL := base + "/" + strings.TrimLeft(entryPath, "/")
	entryRaw, err := httpc.GetURL(ctx, entryURL)
	if err != nil {
		return nil, fmt.Errorf("entry config: %w", err)
	}
	resourcePath, err := findResourceConfigPath(entryRaw)
	if err != nil {
		return nil, err
	}
	resourceURL := base + "/" + strings.TrimLeft(resourcePath, "/")
	resourceRaw, err := httpc.GetURL(ctx, resourceURL)
	if err != nil {
		return nil, fmt.Errorf("resource config: %w", err)
	}
	dataURL, err := gameDataURL(base, resourceRaw)
	if err != nil {
		return nil, err
	}
	dataRaw, err := httpc.GetURL(ctx, dataURL)
	if err != nil {
		return nil, fmt.Errorf("g-data: %w", err)
	}
	lands, err := decodeFarmLandRows(dataRaw)
	if err != nil {
		return nil, err
	}
	return lands, nil
}

func firstPackageCDN(pkg PackageConfig) string {
	for _, cdn := range pkg.CDNs {
		if s := strings.TrimRight(strings.TrimSpace(cdn), "/"); s != "" {
			return s
		}
	}
	return ""
}

func findResourceConfigPath(raw []byte) (string, error) {
	var root any
	if err := json.Unmarshal(raw, &root); err != nil {
		return "", fmt.Errorf("entry config json: %w", err)
	}
	var found string
	var walk func(any)
	walk = func(v any) {
		if found != "" {
			return
		}
		switch x := v.(type) {
		case string:
			if strings.Contains(x, "assets/resources/config") && strings.HasSuffix(x, ".json") {
				found = x
			}
		case []any:
			for _, item := range x {
				walk(item)
			}
		case map[string]any:
			for _, item := range x {
				walk(item)
			}
		}
	}
	walk(root)
	if found == "" {
		return "", fmt.Errorf("entry config missing assets/resources/config*.json")
	}
	return found, nil
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
		if path, _ := entry[0].(string); path == "mo/zh/data/g-data" {
			assetID, _ = strconv.Atoi(id)
			break
		}
	}
	if assetID < 0 {
		return "", fmt.Errorf("resource config missing mo/zh/data/g-data")
	}
	if assetID >= len(cfg.UUIDs) {
		return "", fmt.Errorf("resource config uuid index out of range: %d", assetID)
	}
	version := versionByID(cfg.Versions.Native, assetID)
	if version == "" {
		return "", fmt.Errorf("resource config missing native version for asset %d", assetID)
	}
	uuid := decodeCocosUUID(cfg.UUIDs[assetID])
	escapedUUID := url.PathEscape(uuid)
	return fmt.Sprintf("%s/assets/resources/native/%s/%s.%s.text", base, uuid[:2], escapedUUID, version), nil
}

type resourceConfig struct {
	UUIDs    []string                 `json:"uuids"`
	Paths    map[string][]any         `json:"paths"`
	Versions resourceConfigVersionMap `json:"versions"`
}

type resourceConfigVersionMap struct {
	Native []any `json:"native"`
}

func versionByID(versionArray []any, id int) string {
	for i := 0; i+1 < len(versionArray); i += 2 {
		if intFromAny(versionArray[i]) == int32(id) {
			return stringOf(versionArray[i+1])
		}
	}
	return ""
}

func decodeFarmLandRows(raw []byte) ([]FarmLandConfigRow, error) {
	tables, err := clientcatalog.Decode(raw)
	if err != nil {
		return nil, err
	}
	table, ok := tables["c_farmLand"]
	if !ok {
		return nil, fmt.Errorf("g-data missing c_farmLand")
	}
	return expandFarmLandTable(table), nil
}

func expandFarmLandTable(table clientcatalog.Table) []FarmLandConfigRow {
	out := make([]FarmLandConfigRow, 0)
	for idStr, decoded := range table.Rows {
		id := intFromAny(firstNonEmpty(decoded, "id"))
		if id == 0 {
			id = intFromAny(idStr)
		}
		if id <= 0 {
			continue
		}
		out = append(out, FarmLandConfigRow{
			ID:        id,
			OpenLevel: intFromAny(firstNonEmpty(decoded, "openLvl", "openLevel")),
			Cost:      int32Slice(firstNonEmpty(decoded, "cost")),
			Wasteland: int32Slice(firstNonEmpty(decoded, "wasteland")),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
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

func int32Slice(v any) []int32 {
	xs, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]int32, 0, len(xs))
	for _, item := range xs {
		out = append(out, intFromAny(item))
	}
	return out
}
