package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestReadCompatibility(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		wait       bool
		wantErr    error
	}{
		{"current resource has no status", `{"id":"r","name":"db"}`, 200, true, nil},
		{"legacy readiness", `{"ID":"r","Status":"created"}`, 202, true, nil},
		{"canonical readiness", `{"id":"r","status":"created"}`, 202, true, nil},
		{"refresh does not poll creating", `{"id":"r","status":"creating"}`, 202, false, nil},
		{"refresh accepts paused", `{"id":"r","status":"paused"}`, 200, false, nil},
		{"deleted", `"resource not found"`, 404, false, ErrNotFound},
		{"old server missing route", "404 page not found\n", 404, false, ErrReadUnsupported},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/api/v1/terraform/resource/read/r" || r.Header.Get("Authorization") != "test-token" {
					t.Errorf("unexpected request %s", r.URL.Path)
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_, err := NewClient("test-token", srv.URL+"/api/v1/").ReadResource(ctx, "r", tc.wait)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("got %v, want %v", err, tc.wantErr)
			}
			if calls != 1 {
				t.Fatalf("refresh made %d requests", calls)
			}
		})
	}
}

func TestReadErrorsAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
	}{
		{401, `"unauthorized"`}, {403, `"forbidden"`}, {500, `"unavailable"`},
		{404, "proxy failure"}, {200, "not JSON"}, {200, "null"}, {200, "{}"},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.status)
			_, _ = w.Write([]byte(tc.body))
		}))
		_, err := NewClient("token", srv.URL).ReadSession(context.Background(), "r", false)
		srv.Close()
		if err == nil || errors.Is(err, ErrNotFound) || errors.Is(err, ErrReadUnsupported) {
			t.Fatalf("HTTP %d %q: %v", tc.status, tc.body, err)
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{"status":"creating"}`)) }))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := NewClient("token", srv.URL).ReadSession(ctx, "r", true)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("poll did not respect context: %v", err)
	}
}

func TestSessionPolicyWireCompatibility(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		var payload map[string]interface{}
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				if r.Header.Get("Content-Type") != "application/json" {
					t.Error("missing content type")
				}
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
				}
			}
			_, _ = w.Write([]byte(`{"id":"s","status":"created"}`))
		}))
		c := NewClient("token", srv.URL)
		var policies []SessionPolicy
		if explicit {
			value, total, duration := false, 0, ""
			policies = []SessionPolicy{{AutoApproval: &value, JITMultiApprover: &value, JITTotalApprovers: &total, MaxJITRequestDuration: &duration, DisableWebCLI: &value}}
		}
		for _, update := range []bool{false, true} {
			var err error
			if update {
				_, err = c.UpdateSession(context.Background(), "s", "ep", "db", "", "", "", "cli", false, nil, "", nil, "", "", nil, nil, "", false, policies...)
			} else {
				_, err = c.CreateSession(context.Background(), "ep", "db", "", "", "", "cli", false, nil, "", nil, "", "", nil, nil, "", false, policies...)
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"auto_approval", "jit_multi_approver", "jit_total_approvers", "max_jit_request_duration", "disable_web_cli"} {
				if _, ok := payload[key]; ok != explicit {
					t.Errorf("%s presence = %v, want %v", key, ok, explicit)
				}
			}
			if explicit && (payload["auto_approval"] != false || payload["jit_total_approvers"] != float64(0) || payload["max_jit_request_duration"] != "") {
				t.Fatalf("explicit clears lost: %v", payload)
			}
		}
		srv.Close()
	}
}

func TestDeleteSessionHandlesMissingAndForceDeleteFailure(t *testing.T) {
	for _, missing := range []bool{true, false} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.Contains(r.URL.Path, "/read/"):
				if missing {
					w.WriteHeader(404)
					_, _ = w.Write([]byte(`"session not found"`))
				} else {
					_, _ = w.Write([]byte(`{"status":"terminated"}`))
				}
			case strings.Contains(r.URL.Path, "/forcedelete/"):
				w.WriteHeader(500)
			default:
				_, _ = w.Write([]byte(`{}`))
			}
		}))
		ok, err := NewClient("token", srv.URL).DeleteSession(context.Background(), "s")
		srv.Close()
		if ok != missing || (err == nil) != missing {
			t.Fatalf("missing=%v: ok=%v err=%v", missing, ok, err)
		}
	}
}
