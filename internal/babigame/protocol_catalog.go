package babigame

// NamespaceSpec describes one top-level v namespace observed in the 2026-07-01
// phone capture. Modeled means internal/state has a typed parser for the
// namespace; unmodeled namespaces are still subscribed and retained as raw
// snapshots for diagnostics.
type NamespaceSpec struct {
	Key     string
	Modeled bool
}

var namespaceCatalog = []NamespaceSpec{
	{Key: "2"}, {Key: "3"}, {Key: "6"}, {Key: "7", Modeled: true},
	{Key: "16"}, {Key: "19"}, {Key: "20"}, {Key: "21"},
	{Key: "22", Modeled: true}, {Key: "23", Modeled: true}, {Key: "24", Modeled: true}, {Key: "25", Modeled: true},
	{Key: "27"}, {Key: "28", Modeled: true}, {Key: "31", Modeled: true}, {Key: "33"},
	{Key: "34"}, {Key: "35"}, {Key: "100", Modeled: true},
	{Key: "101", Modeled: true}, {Key: "102", Modeled: true}, {Key: "103", Modeled: true},
	{Key: "104", Modeled: true}, {Key: "105", Modeled: true},
	{Key: "106", Modeled: true}, {Key: "107", Modeled: true}, {Key: "108", Modeled: true},
	{Key: "109", Modeled: true}, {Key: "110"}, {Key: "111"},
	{Key: "112", Modeled: true}, {Key: "113", Modeled: true}, {Key: "114", Modeled: true},
	{Key: "115", Modeled: true}, {Key: "116", Modeled: true},
	{Key: "117", Modeled: true}, {Key: "118", Modeled: true},
	{Key: "119", Modeled: true}, {Key: "120"}, {Key: "122"},
	{Key: "123"}, {Key: "124", Modeled: true}, {Key: "125"}, {Key: "126"},
	{Key: "128"}, {Key: "129", Modeled: true}, {Key: "130"},
	{Key: "131"}, {Key: "132"}, {Key: "134"}, {Key: "136"},
	{Key: "139"}, {Key: "140", Modeled: true}, {Key: "143"}, {Key: "144"},
	{Key: "148"}, {Key: "154"}, {Key: "155"}, {Key: "161"},
	{Key: "162"}, {Key: "165"}, {Key: "166"},
}

var namespaceCatalogByKey = func() map[string]NamespaceSpec {
	out := make(map[string]NamespaceSpec, len(namespaceCatalog))
	for _, spec := range namespaceCatalog {
		out[spec.Key] = spec
	}
	return out
}()

// NamespaceCatalog returns a defensive copy of the observed namespace catalog.
func NamespaceCatalog() []NamespaceSpec {
	out := make([]NamespaceSpec, len(namespaceCatalog))
	copy(out, namespaceCatalog)
	return out
}

// ObservedNamespaceKeys returns every top-level v namespace observed in the
// capture baseline. Runner subscriptions should use this list so new raw-only
// namespaces remain visible without hand-maintained duplicate lists.
func ObservedNamespaceKeys() []string {
	out := make([]string, 0, len(namespaceCatalog))
	for _, spec := range namespaceCatalog {
		out = append(out, spec.Key)
	}
	return out
}

// IsKnownNamespace reports whether a namespace is part of the current capture
// baseline, regardless of whether state has a typed model for it.
func IsKnownNamespace(key string) bool {
	_, ok := namespaceCatalogByKey[key]
	return ok
}

// IsModeledNamespace reports whether internal/state has a typed parser for the
// namespace. Unknown namespace counts should use this instead of keeping a
// separate state-local registry.
func IsModeledNamespace(key string) bool {
	spec, ok := namespaceCatalogByKey[key]
	return ok && spec.Modeled
}
