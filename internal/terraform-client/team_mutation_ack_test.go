package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// DefaultResponse.Status is untagged in the backend's team update/delete handlers.
func TestTeamMutationAcceptsBackendStatusCasing(t *testing.T) {
	for _, action := range []string{"update", "delete"} {
		for _, key := range []string{"Status", "status"} {
			t.Run(action+"/"+key, func(t *testing.T) {
				body := fmt.Sprintf(`{"%s":"team %sd successfully"}`, key, action)
				testTeamMutationAcknowledgement(t, action, body, http.StatusOK, true)
			})
		}
	}
}

func TestTeamMutationRejectsAmbiguousAcknowledgements(t *testing.T) {
	for _, action := range []string{"update", "delete"} {
		expected := "team " + action + "d successfully"
		positive := fmt.Sprintf(`"Status":%q`, expected)
		cases := map[string]string{
			"empty":                     "",
			"whitespace":                " \n\t",
			"empty_object":              `{}`,
			"null":                      `null`,
			"array":                     `[]`,
			"string":                    `"success"`,
			"null_status":               `{"Status":null}`,
			"boolean_status":            `{"Status":true}`,
			"numeric_status":            `{"Status":1}`,
			"nested_status":             `{"Status":{"status":"success"}}`,
			"generic_status":            `{"Status":"success"}`,
			"wrong_route":               `{"Status":"session deleted successfully"}`,
			"other_team_action":         `{"Status":"team created successfully"}`,
			"case_insensitive_key":      fmt.Sprintf(`{"STATUS":%q}`, expected),
			"case_insensitive_value":    fmt.Sprintf(`{"Status":%q}`, "Team "+action+"d successfully"),
			"padded_value":              fmt.Sprintf(`{"Status":%q}`, " "+expected+" "),
			"error_only":                `{"Error":"failed to update team"}`,
			"error_with_ack":            `{` + positive + `,"Error":"synthetic-secret-canary"}`,
			"lower_error_with_ack":      `{` + positive + `,"error":"synthetic-secret-canary"}`,
			"null_error_with_ack":       `{` + positive + `,"Error":null}`,
			"unrelated_with_ack":        `{` + positive + `,"unrelated":true}`,
			"conflicting_case":          `{` + positive + `,"status":"failed"}`,
			"conflicting_case_reversed": `{"status":"failed",` + positive + `}`,
			"same_value_case":           fmt.Sprintf(`{%s,"status":%q}`, positive, expected),
			"duplicate_key":             `{` + positive + `,` + positive + `}`,
			"duplicate_conflict":        `{"Status":"failed",` + positive + `}`,
			"duplicate_lower_key":       fmt.Sprintf(`{"status":"failed","status":%q}`, expected),
			"escaped_duplicate_key":     fmt.Sprintf(`{%s,"\u0053tatus":%q}`, positive, expected),
			"malformed":                 `{` + positive,
			"trailing_object":           `{` + positive + `}{}`,
			"trailing_null":             `{` + positive + `}null`,
		}
		for name, body := range cases {
			t.Run(action+"/"+name, func(t *testing.T) {
				// Exercise the validator too: transport-level checks must not mask
				// acceptance of malformed JSON or null by this route's parser.
				req := httptest.NewRequest(http.MethodPost, "/api/v1/terraform/team/"+action+"/group-id", nil)
				if err := validateMutationAcknowledgement(req, []byte(body)); err == nil {
					t.Error("route validator accepted invalid acknowledgement")
				}
				testTeamMutationAcknowledgement(t, action, body, http.StatusOK, false)
			})
		}
	}
}

func TestTeamMutationAcknowledgementKeepsSiblingContracts(t *testing.T) {
	for _, tc := range []struct {
		route       string
		body        string
		wantSuccess bool
	}{
		{"script/update", `"success"`, true},
		{"script/delete", `"success"`, true},
		{"script/update", `{"Status":"team updated successfully"}`, false},
		{"resource/update", `{"success":true}`, true},
		{"resource/update", `{"Success":true}`, false},
		{"authorization/update", `{"ok":true}`, true},
		{"authorization/delete", ``, true},
		{"session/forcedelete", `{"status":"session deleted successfully"}`, true},
		{"session/forcedelete", `{"Status":"session deleted successfully"}`, false},
		{"session/forcedelete", ``, false},
		{"team/forcedelete", `{"Status":"team deleted successfully"}`, false},
		{"team/update", `{"Status":"team deleted successfully"}`, false},
		{"team/delete", `{"Status":"team updated successfully"}`, false},
	} {
		t.Run(tc.route+"/"+tc.body, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/terraform/"+tc.route+"/group-id", nil)
			if err := validateMutationAcknowledgement(req, []byte(tc.body)); (err == nil) != tc.wantSuccess {
				t.Errorf("route acknowledgement error=%v, want success=%t", err, tc.wantSuccess)
			}
		})
	}
}

func TestTeamMutationRejectsUnsuccessfulHTTPStatusWithPositiveBody(t *testing.T) {
	for _, action := range []string{"update", "delete"} {
		for _, code := range []int{400, 403, 500} {
			t.Run(fmt.Sprintf("%s/%d", action, code), func(t *testing.T) {
				body := fmt.Sprintf(`{"Status":"team %sd successfully"}`, action)
				testTeamMutationAcknowledgement(t, action, body, code, false)
			})
		}
	}
}

func testTeamMutationAcknowledgement(t *testing.T, action, body string, code int, wantSuccess bool) {
	t.Helper()
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/terraform/team/"+action+"/group-id" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.WriteHeader(code)
		fmt.Fprint(w, body)
	}))
	defer s.Close()
	c := NewClient("token", s.URL)
	var err error
	if action == "update" {
		err = c.UpdateTeam(context.Background(), "group-id", "group", []string{"member@example.com"}, nil, "")
	} else {
		var deleted bool
		deleted, err = c.DeleteTeam(context.Background(), "group-id", "group")
		if deleted != wantSuccess {
			t.Errorf("DeleteTeam returned deleted=%t, want %t", deleted, wantSuccess)
		}
	}
	if (err == nil) != wantSuccess {
		t.Errorf("mutation error = %v, want success=%t", err, wantSuccess)
	}
	if err != nil && strings.Contains(err.Error(), "synthetic-secret-canary") {
		t.Error("mutation error leaked response data")
	}
	if calls != 1 {
		t.Errorf("mutation made %d requests, want exactly one", calls)
	}
}
