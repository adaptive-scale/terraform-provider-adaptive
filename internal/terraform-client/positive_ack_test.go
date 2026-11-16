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

func TestRouteMutationRequiresPositiveAcknowledgement(t *testing.T) {
	for _, f := range protocolFixtures() {
		if f.action == "create" || f.action == "read" {
			continue
		}
		bodies := []string{"", `{}`, `{"unrelated":"synthetic-secret-canary"}`, `null`, `{"ok":null}`, `{"success":null}`, `{"ok":false}`, `{"success":false}`, `{"status":null}`, `{"status":false}`, `{"status":"success"}`, `{"message":"success"}`, `"success"`, `{"id":"other"}`, `[]`}
		for _, body := range bodies {
			if f.family == "authorization" && f.action == "delete" && body == "" || f.family == "script" && body == `"success"` {
				continue
			}
			t.Run(f.family+"/"+f.action+"/"+body, func(t *testing.T) {
				calls := 0
				s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; fmt.Fprint(w, body) }))
				defer s.Close()
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
				defer cancel()
				err := f.call(NewClient("token", s.URL), ctx)
				if err == nil {
					t.Fatal("accepted response without route-specific positive acknowledgement")
				}
				if strings.Contains(err.Error(), "synthetic-secret-canary") {
					t.Fatal("response value leaked")
				}
				if calls != 1 {
					t.Fatalf("invalid acknowledgement reached followup: %d", calls)
				}
			})
		}
	}
}

func TestAcknowledgementClassificationIndependentOfSuccessfulStatus(t *testing.T) {
	for _, code := range []int{200, 201, 202, 204} {
		for _, f := range protocolFixtures() {
			if f.action == "read" || f.action == "create" {
				continue
			}
			t.Run(fmt.Sprintf("%d/%s/%s", code, f.family, f.action), func(t *testing.T) {
				s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code); fmt.Fprint(w, `{"unrelated":true}`) }))
				defer s.Close()
				req, _ := http.NewRequest("POST", s.URL+"/api/v1/terraform/"+f.family+"/"+f.action+"/id", nil)
				_, err := NewClient("token", s.URL).do(context.Background(), req)
				// Authorization delete genuinely returns an empty 200 body. A 204 also has
				// no body on the wire, so its empty acknowledgement is equally valid.
				if code == 204 && f.family == "authorization" && f.action == "delete" {
					if err != nil {
						t.Fatal(err)
					}
					return
				}
				if err == nil {
					t.Fatal("status bypassed route acknowledgement contract")
				}
			})
		}
	}
}

func TestCanonicalUpdateAcknowledgementsRetainObjectID(t *testing.T) {
	for _, family := range []string{"resource", "authorization"} {
		t.Run(family, func(t *testing.T) {
			body := `{"success":true}`
			if family == "authorization" {
				body = `{"ok":true}`
			}
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer s.Close()
			c := NewClient("token", s.URL)
			if family == "resource" {
				r, e := c.UpdateResource(context.Background(), "original", "mysql", []byte("safe"), nil, "")
				if e != nil || r.ID != "original" {
					t.Fatal("lost original identity", r, e)
				}
			} else {
				r, e := c.UpdateAuthorization(context.Background(), "original", "role", "d", "SELECT", "mysql")
				if e != nil || r.ID != "original" {
					t.Fatal("lost original identity", r, e)
				}
			}
		})
	}
}
