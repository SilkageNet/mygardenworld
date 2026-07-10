// Package clientcatalog decodes the obfuscated static configuration tables
// shipped with the official client.
package clientcatalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Table is one fully decoded client configuration table.
type Table struct {
	Columns map[string]string
	Rows    map[string]map[string]any
}

type encodedTable struct {
	Key        string          `json:"a"`
	Transforms json.RawMessage `json:"c"`
	Defaults   json.RawMessage `json:"d"`
	Columns    json.RawMessage `json:"m"`
	Flag       any             `json:"f"`
	List       []encodedChunk  `json:"list"`
}

type encodedChunk struct {
	Defaults json.RawMessage           `json:"d"`
	Rows     map[string]map[string]any `json:"v"`
}

// Decode expands g-data into the same table rows returned by the client's
// mo.D accessors. In particular, numeric values are offset per row and column;
// decoding table/column names alone is not sufficient.
func Decode(raw []byte) (map[string]Table, error) {
	var root map[string]json.RawMessage
	if err := decodeJSON(raw, &root); err != nil {
		return nil, fmt.Errorf("g-data json: %w", err)
	}

	var shared []any
	if encoded, ok := root["_"]; ok {
		_ = decodeJSON(encoded, &shared)
	}

	out := make(map[string]Table, len(root))
	for encodedName, tableRaw := range root {
		if encodedName == "_" {
			continue
		}
		var table encodedTable
		if err := decodeJSON(tableRaw, &table); err != nil {
			return nil, fmt.Errorf("decode table %q: %w", encodedName, err)
		}
		if len(table.Columns) == 0 || string(table.Columns) == "null" {
			continue
		}
		name := encodedName
		if table.Key != "" {
			decoded, err := DeCharCode(encodedName, table.Key)
			if err != nil {
				return nil, fmt.Errorf("decode table name %q: %w", encodedName, err)
			}
			name = decoded
		}

		columns, err := decodeStringMap(table.Columns, table.Key)
		if err != nil {
			return nil, fmt.Errorf("decode %s columns: %w", name, err)
		}
		transforms, err := decodeIntMap(table.Transforms, table.Key)
		if err != nil {
			return nil, fmt.Errorf("decode %s transforms: %w", name, err)
		}
		tableDefaults, err := decodeAnyMap(table.Defaults, table.Key)
		if err != nil {
			return nil, fmt.Errorf("decode %s defaults: %w", name, err)
		}

		rows := make(map[string]map[string]any)
		for _, chunk := range table.List {
			chunkDefaults, err := decodeAnyMap(chunk.Defaults, table.Key)
			if err != nil {
				return nil, fmt.Errorf("decode %s chunk defaults: %w", name, err)
			}
			defaults := tableDefaults
			if len(chunkDefaults) > 0 {
				defaults = mergeMaps(tableDefaults, chunkDefaults)
			}
			for rowID, encodedRow := range chunk.Rows {
				rows[rowID] = decodeRow(name, rowID, table.Key, truthy(table.Flag), columns, transforms, defaults, encodedRow, shared)
			}
		}
		out[name] = Table{Columns: columns, Rows: rows}
	}
	return out, nil
}

func decodeRow(tableName, rowID, key string, negativeFlag bool, columns map[string]string, transforms map[string]int, defaults, encoded map[string]any, shared []any) map[string]any {
	decoded := make(map[string]any, len(columns))
	numericID, numeric := parseNumericID(rowID)
	rowMod := int64(0)
	if numeric {
		rowMod = numericID % 100
	}
	for field, code := range columns {
		if field == "$" {
			continue
		}
		value, present := encoded[code]
		if present && key != "" {
			offset := (rowMod + int64(checksum(tableName+"#"+code, 2))) % 100
			if numeric && numericID < 0 && negativeFlag && offset < 0 {
				offset = -offset
			}
			value = decodeNumericValue(value, offset)
		}
		if !present || value == nil {
			value, present = defaults[code]
		} else if transform := transforms[code]; transform != 0 {
			value = applyTransform(value, transform, key, shared)
		}
		if present && value != nil {
			decoded[field] = value
		}
	}
	if idField := columns["$"]; idField != "" {
		if _, ok := decoded[idField]; !ok {
			if numeric {
				decoded[idField] = json.Number(strconv.FormatInt(numericID, 10))
			} else {
				decoded[idField] = rowID
			}
		}
	}
	return decoded
}

func applyTransform(value any, transform int, key string, shared []any) any {
	switch transform {
	case 2:
		idx, ok := numberAsInt(value)
		if ok && idx >= 0 && idx < int64(len(shared)) && shared[idx] != nil && truthy(shared[idx]) {
			return shared[idx]
		}
	case 3, 4:
		s, ok := value.(string)
		if !ok || key == "" {
			return value
		}
		decoded, err := DeCharCode(s, key)
		if err != nil {
			return value
		}
		if transform == 3 {
			return decoded
		}
		var parsed any
		if decodeJSON([]byte(decoded), &parsed) == nil {
			return parsed
		}
	}
	return value
}

func decodeNumericValue(value any, offset int64) any {
	switch x := value.(type) {
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = decodeNumericValue(x[i], offset)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, item := range x {
			out[k] = decodeNumericValue(item, offset)
		}
		return out
	case json.Number:
		if i, err := x.Int64(); err == nil {
			if i >= 10 {
				i -= offset
			}
			return json.Number(strconv.FormatInt(i, 10))
		}
		if f, err := x.Float64(); err == nil {
			return decodedFloat(f, float64(offset))
		}
	case float64:
		return decodedFloat(x, float64(offset))
	case float32:
		return decodedFloat(float64(x), float64(offset))
	case int:
		return decodeSigned(int64(x), offset)
	case int32:
		return decodeSigned(int64(x), offset)
	case int64:
		return decodeSigned(x, offset)
	}
	return value
}

func decodeSigned(value, offset int64) any {
	if value >= 10 {
		value -= offset
	}
	return json.Number(strconv.FormatInt(value, 10))
}

func decodedFloat(value, offset float64) any {
	if value < 10 {
		return value
	}
	value -= offset
	if value < 1e8 && value != math.Trunc(value) {
		value = math.Round(value*1e4) / 1e4
	}
	return value
}

func decodeStringMap(raw json.RawMessage, key string) (map[string]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	decoded, err := decodeMetadata(raw, key)
	if err != nil {
		return nil, err
	}
	var out map[string]string
	if err := decodeJSON(decoded, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func decodeIntMap(raw json.RawMessage, key string) (map[string]int, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	decoded, err := decodeMetadata(raw, key)
	if err != nil {
		return nil, err
	}
	var out map[string]int
	if err := decodeJSON(decoded, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func decodeAnyMap(raw json.RawMessage, key string) (map[string]any, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	decoded, err := decodeMetadata(raw, key)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := decodeJSON(decoded, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func decodeMetadata(raw json.RawMessage, key string) ([]byte, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	if raw[0] != '"' {
		return raw, nil
	}
	var encoded string
	if err := json.Unmarshal(raw, &encoded); err != nil {
		return nil, err
	}
	if key == "" {
		return []byte(encoded), nil
	}
	decoded, err := DeCharCode(encoded, key)
	if err != nil {
		return nil, err
	}
	return []byte(decoded), nil
}

// DeCharCode mirrors mo.CRYPTO.deCharCode. csv may be either a bare
// comma-separated list or a JSON array.
func DeCharCode(csv, key string) (string, error) {
	if key == "" {
		key = "smallaitt"
	}
	text := strings.TrimSpace(csv)
	if !strings.HasPrefix(text, "[") {
		text = "[" + text + "]"
	}
	var codes []int
	if err := json.Unmarshal([]byte(text), &codes); err != nil {
		return "", err
	}
	runes := make([]rune, 0, len(codes))
	for _, code := range codes {
		r := rune(code)
		for i := len(key) - 1; i >= 0; i-- {
			r ^= rune(key[i])
		}
		runes = append(runes, r)
	}
	return string(runes), nil
}

func checksum(value string, digits int) int {
	total := 0
	for _, r := range value {
		total += int(r)
	}
	if digits > 0 {
		total %= int(math.Pow10(min(digits, 5)))
	}
	return total
}

func parseNumericID(value string) (int64, bool) {
	if value == "" {
		return 0, false
	}
	i, err := strconv.ParseInt(value, 10, 64)
	return i, err == nil
}

func numberAsInt(value any) (int64, bool) {
	switch x := value.(type) {
	case json.Number:
		i, err := x.Int64()
		return i, err == nil
	case float64:
		return int64(x), x == math.Trunc(x)
	case int:
		return int64(x), true
	case int64:
		return x, true
	default:
		return 0, false
	}
}

func mergeMaps(base, override map[string]any) map[string]any {
	if len(base) == 0 {
		return override
	}
	out := make(map[string]any, len(base)+len(override))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range override {
		out[k] = v
	}
	return out
}

func truthy(value any) bool {
	switch x := value.(type) {
	case nil:
		return false
	case bool:
		return x
	case json.Number:
		f, _ := x.Float64()
		return f != 0
	case float64:
		return x != 0
	case string:
		return x != ""
	default:
		return true
	}
}

func decodeJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	return decoder.Decode(target)
}
