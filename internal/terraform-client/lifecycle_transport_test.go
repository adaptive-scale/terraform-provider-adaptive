package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAllSevenCreateRoutesRejectUnusableIDs(t *testing.T) {
	count := 0
	for _, f := range protocolFixtures() {
		if f.action != "create" {
			continue
		}
		count++
		for _, id := range []string{"", " ", "id/path", "id\n", "../id"} {
			t.Run(f.family+"/"+id, func(t *testing.T) {
				calls := 0
				s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					_ = json.NewEncoder(w).Encode(map[string]string{"id": id})
				}))
				defer s.Close()
				if err := f.call(NewClient("token", s.URL), context.Background()); err == nil {
					t.Fatal("unusable create ID accepted")
				}
				if calls != 1 {
					t.Fatal("unusable ID reached polling or duplicate create")
				}
			})
		}
	}
	if count != 7 {
		t.Fatal("create route inventory", count)
	}
}

func TestLifecycleTransportRejectsUnsafeResponses(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
	}{
		{"empty create ID", `{ "id":" " }`, 200},
		{"null create", `null`, 200},
		{"trailing JSON", `{"id":"x"} {}`, 200},
		{"secret error", `{"error":"synthetic-secret-canary"}`, 500},
		{"oversized success", `{"id":"x","config":"` + strings.Repeat("x", 4<<20) + `"}`, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer s.Close()
			_, err := NewClient("token", s.URL).CreateResource(context.Background(), "name", "postgres", nil, nil, "")
			if err == nil {
				t.Fatal("unsafe response accepted")
			}
			if strings.Contains(err.Error(), "synthetic-secret-canary") {
				t.Fatal("response payload disclosed")
			}
			if calls != 1 {
				t.Fatalf("create replayed %d times", calls)
			}
		})
	}
}
