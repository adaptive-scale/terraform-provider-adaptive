package client

import (
	"encoding/json"
	"testing"
)

func TestAuthorizationPresenceAwareAliases(t *testing.T) {
	for _, tt := range []struct{ name, body, permissions, resourceType string }{
		{"legacy", `{"legacyPermission":"SELECT","resource_type":"postgres"}`, "SELECT", "postgres"},
		{"canonical", `{"permissions":"ALL","legacyPermission":"SELECT","resourceType":"mysql","resource_type":"postgres"}`, "ALL", "mysql"},
		{"empty canonical", `{"permissions":"","legacyPermission":"SELECT","resourceType":"","resource_type":"postgres"}`, "", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var r AuthorizationReadResponse
			if err := json.Unmarshal([]byte(tt.body), &r); err != nil {
				t.Fatal(err)
			}
			if r.Permissions != tt.permissions || r.ResourceType != tt.resourceType {
				t.Fatalf("aliases: got permissions=%q type=%q", r.Permissions, r.ResourceType)
			}
		})
	}
}

func TestScriptParameterDescriptionsEmptyMapEmitted(t *testing.T) {
	b, err := json.Marshal(ScriptRequest{ParameterDescriptions: map[string]string{}})
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(b, &body); err != nil {
		t.Fatal(err)
	}
	if string(body["ParameterDescriptions"]) != "{}" {
		t.Fatalf("empty metadata must serialize as {}, got %s", b)
	}
}
