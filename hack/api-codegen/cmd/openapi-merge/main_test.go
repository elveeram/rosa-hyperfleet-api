package main

import (
	"encoding/json"
	"testing"
)

func TestPreserveReadOnly(t *testing.T) {
	existingCluster := map[string]any{
		"properties": map[string]any{
			"proxy": map[string]any{"readOnly": true},
		},
	}
	generatedCluster := json.RawMessage(`{"properties":{"proxy":{"type":"object"}}}`)
	mergedCluster, err := preserveReadOnly(generatedCluster, existingCluster)
	if err != nil {
		t.Fatalf("preserve Cluster.proxy readOnly: %v", err)
	}
	assertReadOnly(t, mergedCluster, "properties", "proxy")

	existingList := map[string]any{
		"properties": map[string]any{
			"items": map[string]any{
				"items": map[string]any{
					"properties": map[string]any{
						"proxy": map[string]any{"readOnly": true},
					},
				},
			},
		},
	}
	generatedList := json.RawMessage(`{"properties":{"items":{"items":{"properties":{"proxy":{"type":"object"}}}}}}`)
	mergedList, err := preserveReadOnly(generatedList, existingList)
	if err != nil {
		t.Fatalf("preserve ClusterList.items.proxy readOnly: %v", err)
	}
	assertReadOnly(t, mergedList, "properties", "items", "items", "properties", "proxy")
}

func assertReadOnly(t *testing.T, data json.RawMessage, path ...string) {
	t.Helper()
	var schema any
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("unmarshal merged schema: %v", err)
	}
	current := schema
	for _, part := range path {
		object, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("schema path %v stopped at %q", path, part)
		}
		current, ok = object[part]
		if !ok {
			t.Fatalf("schema path %v missing %q", path, part)
		}
	}
	property, ok := current.(map[string]any)
	if !ok || property["readOnly"] != true {
		t.Errorf("schema path %v has readOnly=%v, want true", path, current)
	}
}
