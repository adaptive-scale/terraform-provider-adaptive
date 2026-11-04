package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEndpointPollTransitions(t *testing.T) {
	for _, action := range []string{"create", "delete"} {
		t.Run(action, func(t *testing.T) {
			reads, creates, forces := 0, 0, 0
			states := []string{"absent", "creating", "restarting", "created"}
			if action == "delete" {
				states = []string{"created", "restarting", "terminating", "terminated"}
			}
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.Contains(r.URL.Path, "/create"):
					creates++
					fmt.Fprint(w, `{"id":"id"}`)
				case strings.Contains(r.URL.Path, "/forcedelete/"):
					forces++
					if reads != len(states) {
						t.Error("force-delete preceded terminal state")
					}
					fmt.Fprint(w, `{"status":"session deleted successfully"}`)
				case strings.Contains(r.URL.Path, "/read/"):
					state := states[reads]
					reads++
					if state == "absent" {
						w.WriteHeader(404)
						return
					}
					fmt.Fprintf(w, `{"id":"id","name":"n","status":%q}`, state)
				default:
					fmt.Fprint(w, `{"id":"id","name":"n","status":"terminating"}`)
				}
			}))
			defer s.Close()
			c := NewClient("token", s.URL)
			c.pollInterval = time.Millisecond
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if action == "create" {
				if _, err := c.CreateSession(ctx, CreateSessionRequest{}); err != nil {
					t.Fatal(err)
				}
				if creates != 1 {
					t.Fatal("create replayed")
				}
			} else {
				if ok, err := c.DeleteSession(ctx, "id"); !ok || err != nil {
					t.Fatal(err)
				}
				if forces != 1 {
					t.Fatal("force-delete missing")
				}
			}
			if reads != len(states) {
				t.Fatal("poll stopped before ready/terminal")
			}
		})
	}
}

func TestEndpointDeleteStopsAtTerminalFailure(t *testing.T) {
	for _, status := range []string{"termination-failed", "failed-to-delete", "does-not-exist", "unknown"} {
		t.Run(status, func(t *testing.T) {
			force := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "/forcedelete/") {
					force++
					fmt.Fprint(w, `{}`)
					return
				}
				fmt.Fprintf(w, `{"id":"id","name":"n","status":%q}`, status)
			}))
			defer s.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
			defer cancel()
			if ok, err := NewClient("token", s.URL).DeleteSession(ctx, "id"); ok || err == nil {
				t.Fatal("failed deletion accepted")
			}
			if ctx.Err() != nil {
				t.Fatal("terminal failure consumed deadline")
			}
			if force != 0 {
				t.Fatal("force-delete issued in unsupported state")
			}
		})
	}
}
