package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEndpointDeleteAcceptsBackendSessionModel(t *testing.T) {
	// TerraformDeleteSession returns the untagged model.Session, not the
	// camelCase DTO used by the independent Terraform read route.
	forced := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/forcedelete/"):
			forced++
			fmt.Fprint(w, `{"status":"session deleted successfully"}`)
		case r.Method == http.MethodGet:
			fmt.Fprint(w, `{"id":"ep-1","name":"owned","status":"terminated"}`)
		default:
			_ = json.NewEncoder(w).Encode(struct {
				ID     string
				Name   string
				Status string
			}{"ep-1", "owned", "terminated"})
		}
	}))
	defer s.Close()
	ok, err := NewClient("synthetic-token", s.URL).DeleteSession(context.Background(), "ep-1")
	if err != nil || !ok || forced != 1 {
		t.Fatalf("backend session response must reach force-delete: ok=%t force=%d err=%v", ok, forced, err)
	}
}

func TestEndpointDeleteAcknowledgementIdentityAndAmbiguity(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want bool
	}{
		{"lower", `{"id":"ep-1","status":"terminated"}`, true},
		{"model", `{"ID":"ep-1","Status":"terminated","Name":"owned"}`, true},
		{"wrong_model_id", `{"ID":"other","Status":"terminated"}`, false},
		{"missing_id", `{"Status":"terminated"}`, false},
		{"missing_status", `{"ID":"ep-1"}`, false},
		{"empty_status", `{"ID":"ep-1","Status":""}`, false},
		{"wrong_status_type", `{"ID":"ep-1","Status":true}`, false},
		{"conflicting_ids", `{"id":"ep-1","ID":"other","status":"terminated"}`, false},
		{"duplicate_ids", `{"id":"ep-1","id":"ep-1","status":"terminated"}`, false},
		{"conflicting_status", `{"id":"ep-1","status":"terminated","Status":"created"}`, false},
		{"duplicate_status", `{"ID":"ep-1","Status":"terminated","Status":"terminated"}`, false},
		{"trailing_object", `{"ID":"ep-1","Status":"terminated"}{}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodPost, "http://localhost/api/v1/terraform/session/delete/ep-1", nil)
			if got := validateMutationAcknowledgement(req, []byte(tc.body)) == nil; got != tc.want {
				t.Fatalf("acknowledged=%t want=%t", got, tc.want)
			}
		})
	}
}

func TestEndpointDeleteAlreadyTerminatedAfterRejectedTermination(t *testing.T) {
	for _, status := range []string{"terminated", "created"} {
		t.Run(status, func(t *testing.T) {
			forced := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.Contains(r.URL.Path, "/forcedelete/"):
					forced++
					fmt.Fprint(w, `{"status":"session deleted successfully"}`)
				case r.Method == http.MethodGet:
					fmt.Fprintf(w, `{"id":"ep-1","name":"owned","status":%q}`, status)
				default:
					fmt.Fprint(w, `"error while terminating session"`)
				}
			}))
			defer s.Close()
			ok, err := NewClient("synthetic-token", s.URL).DeleteSession(context.Background(), "ep-1")
			if status == "terminated" {
				if !ok || err != nil || forced != 1 {
					t.Fatalf("confirmed terminal endpoint must be removable: ok=%t forced=%d err=%v", ok, forced, err)
				}
			} else if ok || err == nil || forced != 0 {
				t.Fatal("nonterminal termination failure must not force-delete")
			}
		})
	}
}
