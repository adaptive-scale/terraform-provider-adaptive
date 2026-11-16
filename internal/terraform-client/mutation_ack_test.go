package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMutationRejectsSuccessfulHTTPErrorEnvelopes(t *testing.T) {
	for _, f := range protocolFixtures() {
		if f.action == "read" || f.action == "create" {
			continue
		}
		for _, body := range []string{`"error while terminating session"`, `{"error":"synthetic-secret-canary"}`, `null`, `[]`, `42`, `true`, `{"success":false}`, `{"ok":false}`} {
			t.Run(f.family+"/"+f.action+"/"+body, func(t *testing.T) {
				calls := 0
				s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; fmt.Fprint(w, body) }))
				defer s.Close()
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
				defer cancel()
				if err := f.call(NewClient("token", s.URL), ctx); err == nil {
					t.Fatal("accepted unsuccessful mutation")
				} else if strings.Contains(err.Error(), "synthetic-secret-canary") {
					t.Fatal("payload leaked")
				}
				wantCalls := 1
				if f.family == "session" && f.action == "delete" && body == `"error while terminating session"` {
					wantCalls = 2
				} // one authoritative absence check, never a mutation replay
				if calls != wantCalls {
					t.Fatalf("failure did not stop after bounded verification: %d requests", calls)
				}
			})
		}
	}
}
func TestUpdateDTOsRepresentBackendAcknowledgements(t *testing.T) {
	var r UpdateResourceResponse
	_ = json.Unmarshal([]byte(`{"success":true}`), &r)
	b, _ := json.Marshal(r)
	if !strings.Contains(string(b), `"success":true`) {
		t.Fatal("resource success acknowledgement discarded")
	}
	var a UpdateAuthorizationResponse
	_ = json.Unmarshal([]byte(`{"ok":true}`), &a)
	b, _ = json.Marshal(a)
	if !strings.Contains(string(b), `"ok":true`) {
		t.Fatal("authorization ok acknowledgement discarded")
	}
}
func TestEndpointTerminationErrorOnlySucceedsOnConfirmedAbsence(t *testing.T) {
	for _, absent := range []bool{true, false} {
		t.Run(fmt.Sprint(absent), func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "GET" {
					if absent {
						w.WriteHeader(404)
						return
					}
					fmt.Fprint(w, `{"id":"id","name":"n","status":"created"}`)
					return
				}
				fmt.Fprint(w, `"error while terminating session"`)
			}))
			defer s.Close()
			ok, err := NewClient("token", s.URL).DeleteSession(context.Background(), "id")
			if absent {
				if !ok || err != nil {
					t.Fatal("absent endpoint deletion not idempotent", err)
				}
			} else if ok || err == nil {
				t.Fatal("termination failure hidden")
			}
		})
	}
}

func TestEndpointDeleteDoesNotHidePermanentForceErrors(t *testing.T) {
	for _, code := range []int{401, 403, 409, 500} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			forced := false
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.Contains(r.URL.Path, "/forcedelete/") {
					forced = true
					w.WriteHeader(code)
					fmt.Fprint(w, `"arbitrary failure"`)
					return
				}
				if strings.Contains(r.URL.Path, "/read/") {
					if forced {
						w.WriteHeader(404)
						return
					}
					fmt.Fprint(w, `{"id":"id","name":"n","status":"terminated"}`)
					return
				}
				fmt.Fprint(w, `{"id":"id","name":"n","status":"terminated"}`)
			}))
			defer s.Close()
			if ok, err := NewClient("token", s.URL).DeleteSession(context.Background(), "id"); ok || err == nil {
				t.Fatal("permanent force-delete error hidden")
			}
		})
	}
}

func TestEndpointDeleteRaceRequiresAuthoritativeAbsence(t *testing.T) {
	for _, absent := range []bool{true, false} {
		t.Run(fmt.Sprint(absent), func(t *testing.T) {
			force := false
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.Contains(r.URL.Path, "/forcedelete/"):
					force = true
					w.WriteHeader(500)
					fmt.Fprint(w, `"failed to get session"`)
				case strings.Contains(r.URL.Path, "/read/"):
					if force && absent {
						w.WriteHeader(404)
						return
					}
					fmt.Fprint(w, `{"id":"id","name":"n","status":"terminated"}`)
				default:
					fmt.Fprint(w, `{"id":"id","name":"n","status":"terminated"}`)
				}
			}))
			defer s.Close()
			ok, err := NewClient("token", s.URL).DeleteSession(context.Background(), "id")
			if absent {
				if !ok || err != nil {
					t.Fatal("404 race not idempotent", err)
				}
			} else if ok || err == nil {
				t.Fatal("permanent failure hidden")
			}
		})
	}
}
