package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJITGroupWirePresence(t *testing.T) {
	for _, tc := range []struct {
		groups   *[]string
		expected string
	}{{nil, ""}, {groupList(), `"access_approver_groups":[]`}, {groupList("On call"), `"access_approver_groups":["On call"]`}} {
		data, err := json.Marshal(CreateSessionRequest{AccessApproverGroups: tc.groups})
		if err != nil {
			t.Fatal(err)
		}
		if tc.expected == "" {
			if strings.Contains(string(data), "access_approver_groups") {
				t.Fatal(string(data))
			}
		} else if !strings.Contains(string(data), tc.expected) {
			t.Fatal(string(data))
		}
	}
}
func groupList(values ...string) *[]string { list := append([]string{}, values...); return &list }

func TestJITGroupCapabilityAndRead(t *testing.T) {
	for _, status := range []int{200, 404, 403} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				t.Error("capability/read must not mutate")
			}
			w.WriteHeader(status)
			if strings.Contains(r.URL.Path, "/read/") {
				_, _ = w.Write([]byte(`{"accessApproverGroups":["On call"]}`))
			} else {
				_, _ = w.Write([]byte(`{"supported":true,"enabled":false}`))
			}
		}))
		c := NewClient("test-token", server.URL)
		if err := c.RequireJITApproverGroups(context.Background(), true); err == nil {
			t.Error("accepted disabled server")
		}
		err := c.RequireJITApproverGroups(context.Background(), false)
		if (err == nil) != (status == 200) {
			t.Fatalf("unexpected clear capability: %v", err)
		}
		groups, err := c.ReadJITApproverGroups(context.Background(), "ep")
		if status == 200 {
			if err != nil || len(groups) != 1 || groups[0] != "On call" {
				t.Fatalf("lost group readback %v: %v", groups, err)
			}
		} else if err == nil {
			t.Error("accepted failed read")
		}
		server.Close()
	}
}
