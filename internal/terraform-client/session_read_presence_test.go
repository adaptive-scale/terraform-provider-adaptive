package client

import (
	"encoding/json"
	"testing"
)

func TestSessionReadPreservesPolicyPresence(t *testing.T) {
	for _, tc := range []struct {
		name, body, key string
		present         bool
	}{
		{"false", `{"isJitEnabled":false}`, "isJitEnabled", true},
		{"missing", `{}`, "isJitEnabled", false},
		{"null boolean", `{"isJitEnabled":null}`, "isJitEnabled", false},
		{"legacy capitalization", `{"IsJITEnabled":false}`, "isJitEnabled", true},
		{"zero quorum", `{"jitTotalApprovers":0}`, "jitTotalApprovers", true},
		{"missing quorum", `{}`, "jitTotalApprovers", false},
		{"empty approvers", `{"accessApprovers":[]}`, "accessApprovers", true},
		{"null approvers", `{"accessApprovers":null}`, "accessApprovers", true},
		{"missing approvers", `{}`, "accessApprovers", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var response SessionReadResponse
			if err := json.Unmarshal([]byte(tc.body), &response); err != nil {
				t.Fatal("could not decode policy response")
			}
			observer, ok := interface{}(&response).(interface{ HasField(string) bool })
			if !ok {
				t.Fatal("endpoint read DTO loses missing/null versus authoritative zero-value presence")
			}
			if observer.HasField(tc.key) != tc.present {
				t.Error("endpoint read presence is not authoritative")
			}
		})
	}
}
