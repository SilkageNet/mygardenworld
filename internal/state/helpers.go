package state

import (
	"encoding/json"
	"sort"
	"time"
)

func cloneInt32Map(src map[int32]int32) map[int32]int32 {
	dst := make(map[int32]int32, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func setOf(ids []int32) map[int32]struct{} {
	if len(ids) == 0 {
		return nil
	}
	m := make(map[int32]struct{}, len(ids))
	for _, id := range ids {
		m[id] = struct{}{}
	}
	return m
}

func readItemCountsRaw(raw json.RawMessage) []ItemCount {
	var stacks []json.RawMessage
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	if json.Unmarshal(raw, &stacks) == nil {
		out := make([]ItemCount, 0, len(stacks))
		for _, rawStack := range stacks {
			parts := readInt32OrderedListRaw(rawStack)
			if len(parts) < 2 || parts[0] <= 0 || parts[1] <= 0 {
				continue
			}
			out = append(out, ItemCount{ItemID: parts[0], Count: parts[1]})
		}
		return out
	}
	// INumMap is observed both as [[itemId,count], ...] and as a JSON object
	// depending on the RPC serializer. Accept both without inventing entries.
	counts := readInt32RawMap(raw)
	ids := make([]int32, 0, len(counts))
	for itemID, count := range counts {
		if itemID > 0 && count > 0 {
			ids = append(ids, itemID)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]ItemCount, 0, len(ids))
	for _, itemID := range ids {
		out = append(out, ItemCount{ItemID: itemID, Count: counts[itemID]})
	}
	return out
}

func readInt32OrderedListRaw(raw json.RawMessage) []int32 {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		out := make([]int32, 0, len(arr))
		for _, rawValue := range arr {
			if n, ok := readInt32Raw(rawValue); ok {
				out = append(out, n)
			}
		}
		return out
	}
	if n, ok := readInt32Raw(raw); ok {
		return []int32{n}
	}
	return nil
}

func rawCollectionCount(raw json.RawMessage) int32 {
	if len(raw) == 0 || string(raw) == "null" {
		return 0
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		return int32(len(arr))
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err == nil {
		return int32(len(m))
	}
	if n, ok := readInt32Raw(raw); ok && n > 0 {
		return n
	}
	return 0
}

func readInt32ListRaw(raw json.RawMessage) []int32 {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		out := make([]int32, 0, len(arr))
		for _, rawValue := range arr {
			if n, ok := readInt32Raw(rawValue); ok {
				out = append(out, n)
			}
		}
		return uniqueSortedInt32s(out)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err == nil {
		out := make([]int32, 0, len(m))
		if denseIndexMap(m) {
			for i := 0; i < len(m); i++ {
				if n, ok := readInt32Raw(m[itoaState(i)]); ok {
					out = append(out, n)
				}
			}
		} else {
			for key, rawValue := range m {
				id := atoi32(key)
				if id == 0 || !truthyRaw(rawValue) {
					continue
				}
				out = append(out, id)
			}
		}
		return uniqueSortedInt32s(out)
	}
	if n, ok := readInt32Raw(raw); ok {
		return []int32{n}
	}
	return nil
}

func readInt32ListRawAllowZero(raw json.RawMessage) []int32 {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		out := make([]int32, 0, len(arr))
		for _, rawValue := range arr {
			if n, ok := readInt32Raw(rawValue); ok {
				out = append(out, n)
			}
		}
		return uniqueSortedInt32sAllowZero(out)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err == nil {
		out := make([]int32, 0, len(m))
		if denseIndexMap(m) {
			for i := 0; i < len(m); i++ {
				if n, ok := readInt32Raw(m[itoaState(i)]); ok {
					out = append(out, n)
				}
			}
		} else {
			for key, rawValue := range m {
				if !truthyRaw(rawValue) {
					continue
				}
				out = append(out, atoi32(key))
			}
		}
		return uniqueSortedInt32sAllowZero(out)
	}
	if n, ok := readInt32Raw(raw); ok {
		return []int32{n}
	}
	return nil
}

func readInt32JSONField(fields map[string]json.RawMessage, keys ...string) (int32, bool) {
	for _, key := range keys {
		if raw, ok := fields[key]; ok {
			return readInt32Raw(raw)
		}
	}
	return 0, false
}

func readInt64JSONField(fields map[string]json.RawMessage, keys ...string) (int64, bool) {
	for _, key := range keys {
		raw, ok := fields[key]
		if !ok {
			continue
		}
		var n int64
		if err := json.Unmarshal(raw, &n); err == nil {
			return n, true
		}
		var f float64
		if err := json.Unmarshal(raw, &f); err == nil {
			return int64(f), true
		}
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			n := atoi64(s)
			if n != 0 || s == "0" {
				return n, true
			}
		}
	}
	return 0, false
}

func readInt32Raw(raw json.RawMessage) (int32, bool) {
	var n int32
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, true
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return int32(f), true
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		n := atoi32(s)
		if n != 0 || s == "0" {
			return n, true
		}
	}
	return 0, false
}

func readInt64Raw(raw json.RawMessage) (int64, bool) {
	return readInt64JSONField(map[string]json.RawMessage{"0": raw}, "0")
}

func sameLocalDay(rawDate int64, now time.Time) bool {
	if rawDate <= 0 {
		return false
	}
	if rawDate >= 19000101 && rawDate <= 29991231 {
		return now.Format("20060102") == itoa64State(rawDate)
	}
	var t time.Time
	if rawDate > 1_000_000_000_000 {
		t = time.UnixMilli(rawDate)
	} else {
		t = time.Unix(rawDate, 0)
	}
	y1, m1, d1 := t.Local().Date()
	y2, m2, d2 := now.Local().Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

func denseIndexMap(m map[string]json.RawMessage) bool {
	if len(m) == 0 {
		return false
	}
	for i := 0; i < len(m); i++ {
		if _, ok := m[itoaState(i)]; !ok {
			return false
		}
	}
	return true
}

func truthyRaw(raw json.RawMessage) bool {
	if len(raw) == 0 || string(raw) == "null" {
		return false
	}
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return b
	}
	if n, ok := readInt32Raw(raw); ok {
		return n != 0
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s != "" && s != "0" && s != "false"
	}
	return true
}

func uniqueSortedInt32s(in []int32) []int32 {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[int32]struct{}, len(in))
	out := make([]int32, 0, len(in))
	for _, id := range in {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func uniqueSortedInt32sAllowZero(in []int32) []int32 {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[int32]struct{}, len(in))
	out := make([]int32, 0, len(in))
	for _, id := range in {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func itoaState(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func itoa64State(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func readInt32RawMap(raw json.RawMessage) map[int32]int32 {
	out := map[int32]int32{}
	if len(raw) == 0 {
		return out
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return out
	}
	for key, rawValue := range values {
		id := atoi32(key)
		if id == 0 {
			continue
		}
		if n, ok := readInt32Raw(rawValue); ok {
			out[id] = n
		}
	}
	return out
}

func readInt64RawMap(raw json.RawMessage) map[int32]int64 {
	out := map[int32]int64{}
	if len(raw) == 0 {
		return out
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return out
	}
	for key, rawValue := range values {
		id := atoi32(key)
		if id == 0 {
			continue
		}
		if n, ok := readInt64JSONField(map[string]json.RawMessage{"0": rawValue}, "0"); ok {
			out[id] = n
		}
	}
	return out
}

func readNestedInt32RawMapTotals(raw json.RawMessage) (map[int32]int32, int32) {
	out := map[int32]int32{}
	if len(raw) == 0 || string(raw) == "null" {
		return out, 0
	}
	var outer map[string]json.RawMessage
	if err := json.Unmarshal(raw, &outer); err != nil {
		return out, 0
	}
	var total int32
	for _, rawInner := range outer {
		inner := readInt32RawMap(rawInner)
		for typ, count := range inner {
			if count <= 0 {
				continue
			}
			out[typ] += count
			total += count
		}
	}
	return out, total
}

func readInt(m map[string]any, keys ...string) int {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if i := readInt32Any(v); i != 0 {
				return int(i)
			}
		}
	}
	return 0
}

func readInt64(m map[string]any, keys ...string) int64 {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			switch x := v.(type) {
			case float64:
				return int64(x)
			case int:
				return int64(x)
			case int64:
				return x
			case json.Number:
				i, _ := x.Int64()
				return i
			}
		}
	}
	return 0
}

func readInt64Slice(m map[string]any, keys ...string) []int64 {
	for _, k := range keys {
		v, ok := m[k]
		if !ok || v == nil {
			continue
		}
		switch x := v.(type) {
		case []any:
			out := make([]int64, 0, len(x))
			for _, item := range x {
				if uid := readInt64Any(item); uid > 0 {
					out = append(out, uid)
				}
			}
			if len(out) > 0 {
				return out
			}
		case map[string]any:
			out := make([]int64, 0, len(x))
			for key, item := range x {
				if uid := readInt64Any(item); uid > 0 {
					out = append(out, uid)
					continue
				}
				if uid, err := parseInt64Key(key); err == nil && uid > 0 {
					out = append(out, uid)
				}
			}
			if len(out) > 0 {
				return out
			}
		}
	}
	return nil
}

func readInt64Any(v any) int64 {
	switch x := v.(type) {
	case float64:
		return int64(x)
	case int:
		return int64(x)
	case int32:
		return int64(x)
	case int64:
		return x
	case json.Number:
		i, _ := x.Int64()
		return i
	}
	return 0
}

func readInt32Any(v any) int32 {
	switch x := v.(type) {
	case float64:
		return int32(x)
	case int:
		return int32(x)
	case int32:
		return x
	case int64:
		return int32(x)
	case json.Number:
		i, _ := x.Int64()
		return int32(i)
	}
	return 0
}

func atoi32(s string) int32 {
	var n int32
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int32(c-'0')
	}
	return n
}

func atoi64(s string) int64 {
	var n int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int64(c-'0')
	}
	return n
}
