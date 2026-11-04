package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type routeFixture struct {
	family, action, payload, response string
	call                              func(*Client, context.Context) error
}

func protocolFixtures() []routeFixture {
	return []routeFixture{
		{"resource", "create", `{"name":"n","integrationType":"postgres","config":"safe","userTags":["tag"]}`, `{"id":"id"}`, func(c *Client, x context.Context) error {
			_, e := c.CreateResource(x, "n", "postgres", []byte("safe"), []string{"tag"}, "")
			return e
		}},
		{"resource", "read", "", `{"id":"id","name":"n"}`, func(c *Client, x context.Context) error { _, e := c.ReadResource(x, "id"); return e }},
		{"resource", "update", `{"integrationType":"postgres","config":"safe","userTags":["tag"]}`, `{"success":true}`, func(c *Client, x context.Context) error {
			_, e := c.UpdateResource(x, "id", "postgres", []byte("safe"), []string{"tag"}, "")
			return e
		}},
		{"resource", "delete", "", `{"success":true}`, func(c *Client, x context.Context) error { _, e := c.DeleteResource(x, "id", "n"); return e }},
		{"session", "create", `{"sessionName":"n","resourceName":"r","sessionType":"cli","is_jit_enabled":false,"access_approvers":null,"memory":"","cpu":"","usertags":null,"script_only_access":false,"disable_output_capture":false,"disable_data_studio":false,"disable_web_cli":false}`, `{"id":"id"}`, func(c *Client, x context.Context) error {
			_, e := c.CreateSession(x, CreateSessionRequest{SessionName: "n", ResourceName: "r", SessionType: "cli"})
			return e
		}},
		{"session", "read", "", `{"id":"id","name":"n","status":"created"}`, func(c *Client, x context.Context) error { _, e := c.ReadSession(x, "id"); return e }},
		{"session", "update", `{"sessionName":"n","resourceName":"r","sessionType":"cli","is_jit_enabled":false,"access_approvers":null,"memory":"","cpu":"","usertags":null,"script_only_access":false,"disable_output_capture":false,"disable_data_studio":false,"disable_web_cli":false}`, `{"id":"id"}`, func(c *Client, x context.Context) error {
			_, e := c.UpdateSession(x, "id", CreateSessionRequest{SessionName: "n", ResourceName: "r", SessionType: "cli"})
			return e
		}},
		{"session", "delete", "", `{"id":"id","status":"terminated"}`, func(c *Client, x context.Context) error { _, e := c.DeleteSession(x, "id"); return e }},
		{"session", "forcedelete", "", `{"status":"session deleted successfully"}`, func(c *Client, x context.Context) error { _, e := c.forceDeleteSession(x, "id"); return e }},
		{"authorization", "create", `{"name":"n","resource":"r","description":"d","permissions":"SELECT"}`, `{"id":"id"}`, func(c *Client, x context.Context) error {
			_, e := c.CreateAuthorization(x, "n", "d", "SELECT", "r")
			return e
		}},
		{"authorization", "read", "", `{"id":"id","name":"n","status":"created"}`, func(c *Client, x context.Context) error { _, e := c.ReadAuthorization(x, "id"); return e }},
		{"authorization", "update", `{"name":"n","description":"d","permissions":"SELECT","resourceType":"postgres"}`, `{"ok":true}`, func(c *Client, x context.Context) error {
			_, e := c.UpdateAuthorization(x, "id", "n", "d", "SELECT", "postgres")
			return e
		}},
		{"authorization", "delete", "", "", func(c *Client, x context.Context) error { _, e := c.DeleteAuthorization(x, "id"); return e }},
		{"team", "create", `{"Name":"n","Members":["u"],"Endpoints":["e"]}`, `{"id":"id"}`, func(c *Client, x context.Context) error {
			_, e := c.CreateTeam(x, "n", []string{"u"}, []string{"e"})
			return e
		}},
		{"team", "read", "", `{"id":"id","name":"n"}`, func(c *Client, x context.Context) error { _, e := c.ReadTeam(x, "id"); return e }},
		{"team", "update", `{"Name":"n","Members":[],"Endpoints":[],"SlackChannelID":"s"}`, `{"status":"team updated successfully"}`, func(c *Client, x context.Context) error {
			return c.UpdateTeam(x, "id", "n", []string{}, []string{}, "s")
		}},
		{"team", "delete", "", `{"status":"team deleted successfully"}`, func(c *Client, x context.Context) error { _, e := c.DeleteTeam(x, "id", "n"); return e }},
		{"script", "create", `{"Name":"n","Command":"safe","Endpoint":"e","ParameterDescriptions":null}`, `{"id":"id"}`, func(c *Client, x context.Context) error {
			_, e := c.CreateScript(x, ScriptRequest{Name: "n", Command: "safe", Endpoint: "e"})
			return e
		}},
		{"script", "read", "", `{"id":"id","name":"n"}`, func(c *Client, x context.Context) error { _, e := c.ReadScript(x, "id"); return e }},
		{"script", "update", `{"Name":"n","Command":"safe","Endpoint":"e","ParameterDescriptions":null}`, `"success"`, func(c *Client, x context.Context) error {
			return c.UpdateScript(x, "id", ScriptRequest{Name: "n", Command: "safe", Endpoint: "e"})
		}},
		{"script", "delete", "", `"success"`, func(c *Client, x context.Context) error { _, e := c.DeleteScript(x, "id", "n"); return e }},
		{"schedule", "create", `{"name":"n","scheduleType":"daily","startHour":0,"startMinute":0,"endHour":0,"endMinute":0}`, `{"id":"id","name":"n"}`, func(c *Client, x context.Context) error {
			_, e := c.CreateSchedule(x, &ScheduleRequest{Name: "n", ScheduleType: "daily"})
			return e
		}},
		{"schedule", "read", "", `{"id":"id","name":"n"}`, func(c *Client, x context.Context) error { _, e := c.GetSchedule(x, "id"); return e }},
		{"schedule", "update", `{"name":"n","scheduleType":"daily","startHour":0,"startMinute":0,"endHour":0,"endMinute":0}`, `{"id":"id","name":"n"}`, func(c *Client, x context.Context) error {
			_, e := c.UpdateSchedule(x, "id", &ScheduleRequest{Name: "n", ScheduleType: "daily"})
			return e
		}},
		{"schedule", "delete", "", `{"message":"schedule deleted","id":"id"}`, func(c *Client, x context.Context) error { _, e := c.DeleteSchedule(x, "id", "n"); return e }},
		{"dataprotection", "create", `{"resourceName":"r","masks":[]}`, `{"id":"id","resourceName":"r"}`, func(c *Client, x context.Context) error {
			_, e := c.CreateDataProtection(x, DataProtectionRequest{ResourceName: "r", Masks: []DataProtectionMask{}})
			return e
		}},
		{"dataprotection", "read", "", `{"id":"id","resourceName":"r"}`, func(c *Client, x context.Context) error { _, e := c.ReadDataProtection(x, "id"); return e }},
		{"dataprotection", "update", `{"resourceName":"r","masks":[]}`, `{"id":"id","resourceName":"r"}`, func(c *Client, x context.Context) error {
			_, e := c.UpdateDataProtection(x, "id", DataProtectionRequest{ResourceName: "r", Masks: []DataProtectionMask{}})
			return e
		}},
		{"dataprotection", "delete", "", `{"message":"data protection turned off","id":"id"}`, func(c *Client, x context.Context) error { return c.DeleteDataProtection(x, "id") }},
	}
}

type closeTrackingTransport struct {
	base           http.RoundTripper
	opened, closed atomic.Int32
}
type closeTrackingBody struct {
	io.ReadCloser
	owner *closeTrackingTransport
}

func (b *closeTrackingBody) Close() error { b.owner.closed.Add(1); return b.ReadCloser.Close() }
func (t *closeTrackingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	v, e := t.base.RoundTrip(r)
	if e == nil {
		t.opened.Add(1)
		v.Body = &closeTrackingBody{v.Body, t}
	}
	return v, e
}

func TestProtocol29RouteMatrix(t *testing.T) {
	fixtures := protocolFixtures()
	if len(fixtures) != 29 {
		t.Fatal("route count", len(fixtures))
	}
	seen := map[string]bool{}
	for _, f := range fixtures {
		key := f.family + "/" + f.action
		if seen[key] {
			t.Fatal("duplicate", key)
		}
		seen[key] = true
		for _, mode := range []string{"success", "malformed", "not-found", "unauthorized", "forbidden", "conflict", "server-string", "server-object", "server-malformed", "cancel", "disconnect"} {
			t.Run(key+"/"+mode, func(t *testing.T) {
				path := "/api/v1/terraform/" + key
				if f.action != "create" {
					path += "/id"
				}
				var calls atomic.Int32
				s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != path {
						// Only explicit readiness and deletion follow-up reads are permitted.
						if r.Method == "GET" && r.URL.Path == "/api/v1/terraform/"+f.family+"/read/id" {
							status := "created"
							if f.action == "delete" {
								status = "terminated"
							}
							fmt.Fprintf(w, `{"id":"id","name":"n","status":%q}`, status)
							return
						}
						if f.family == "session" && f.action == "delete" && r.URL.Path == "/api/v1/terraform/session/forcedelete/id" {
							fmt.Fprint(w, `{"status":"session deleted successfully"}`)
							return
						}
						t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
						w.WriteHeader(500)
						return
					}
					calls.Add(1)
					method := "POST"
					if f.action == "read" {
						method = "GET"
					}
					if r.Method != method {
						t.Error("method", r.Method)
					}
					if r.Header.Get("Authorization") != "token" {
						t.Error("authorization")
					}
					b, _ := io.ReadAll(r.Body)
					if f.payload != "" {
						if r.Header.Get("Content-Type") != "application/json" {
							t.Error("content type")
						}
						var got, want any
						_ = json.Unmarshal(b, &got)
						_ = json.Unmarshal([]byte(f.payload), &want)
						if !reflect.DeepEqual(got, want) {
							t.Error("wire payload mismatch")
						}
					} else if len(b) != 0 {
						t.Error("unexpected payload")
					}
					switch mode {
					case "malformed":
						fmt.Fprint(w, `{"broken":`)
					case "not-found":
						w.WriteHeader(404)
						fmt.Fprint(w, `"synthetic-canary"`)
					case "unauthorized":
						w.WriteHeader(401)
						fmt.Fprint(w, `"synthetic-canary"`)
					case "forbidden":
						w.WriteHeader(403)
						fmt.Fprint(w, `"synthetic-canary"`)
					case "conflict":
						w.WriteHeader(409)
						fmt.Fprint(w, `{"error":"synthetic-canary"}`)
					case "server-string":
						w.WriteHeader(500)
						fmt.Fprint(w, `"synthetic-canary"`)
					case "server-malformed":
						w.WriteHeader(500)
						fmt.Fprint(w, `{"error":`)
					case "server-object":
						w.WriteHeader(500)
						fmt.Fprint(w, `{"error":{"secret":"synthetic-canary"}}`)
					case "disconnect":
						h := w.(http.Hijacker)
						conn, _, _ := h.Hijack()
						conn.Close()
					default:
						if f.family == "session" && f.action == "read" {
							w.WriteHeader(202)
						}
						fmt.Fprint(w, f.response)
					}
				}))
				defer s.Close()
				c := NewClient("token", s.URL)
				tr := &closeTrackingTransport{base: http.DefaultTransport}
				c.httpClient.Transport = tr
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				defer cancel()
				if mode == "cancel" {
					cancel()
				}
				err := f.call(c, ctx)
				success := mode == "success" || (mode == "not-found" && (f.action == "read" || (f.family == "session" && (f.action == "delete" || f.action == "forcedelete"))))
				if success {
					if err != nil {
						t.Fatal(err)
					}
				} else {
					if err == nil {
						t.Fatal("expected failure")
					}
					if strings.Contains(err.Error(), "synthetic-canary") {
						t.Fatal("secret leaked")
					}
				}
				code := map[string]int{"not-found": 404, "unauthorized": 401, "forbidden": 403, "conflict": 409, "server-string": 500, "server-object": 500, "server-malformed": 500}[mode]
				if code != 0 && !success {
					var status *HTTPError
					if !errors.As(err, &status) || status.StatusCode != code {
						t.Fatalf("lost typed HTTP status %d", code)
					}
				}
				if mode == "cancel" && !errors.Is(err, context.Canceled) {
					t.Fatalf("lost cancellation %v", err)
				}
				if calls.Load() > 1 {
					t.Fatalf("route replayed %d times", calls.Load())
				}
				if tr.opened.Load() != tr.closed.Load() {
					t.Fatal("network body not closed")
				}
			})
		}
	}
}
