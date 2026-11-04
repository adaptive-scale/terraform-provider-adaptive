package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEndpointCreateRetainsIdentityOnReadinessFailure(t *testing.T) {
	for _, status := range []string{"failed", "failed-to-restart", "terminated", "unhealthy", "creating", "restarting", "denied"} {
		t.Run(status, func(t *testing.T) {
			creates := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/create") {
					creates++
					fmt.Fprint(w, `{"id":"ep-1"}`)
					return
				}
				if status == "denied" {
					w.WriteHeader(403)
					return
				}
				fmt.Fprintf(w, `{"id":"ep-1","name":"ep","status":%q}`, status)
			}))
			defer s.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
			defer cancel()
			resp, err := NewClient("token", s.URL).CreateSession(ctx, CreateSessionRequest{})
			if err == nil {
				t.Fatal("readiness failure accepted")
			}
			if resp == nil || resp.ID != "ep-1" {
				t.Fatal("recoverable identity lost")
			}
			if creates != 1 {
				t.Fatal("create replayed")
			}
			if status == "creating" || status == "restarting" {
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("lost deadline: %v", err)
				}
			}
		})
	}
}
func TestEndpointDeleteRejectsTerminationErrorBody(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `"error while terminating session"`) }))
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if ok, err := NewClient("token", s.URL).DeleteSession(ctx, "ep-1"); ok || err == nil {
		t.Fatal("termination error accepted")
	}
}
func TestEndpointForceDeleteStatesMatchBackend(t *testing.T) {
	if isSessionReaped("does-not-exist") {
		t.Fatal("unsupported backend force-delete state")
	}
	for _, s := range []string{"terminated", "marked-for-deletion", "failed", "failed-to-restart"} {
		if !isSessionReaped(s) {
			t.Fatal(s)
		}
	}
}
