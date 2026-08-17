package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// recorder captures what the client actually put on the wire.
type recordedReq struct {
	method string
	path   string
	body   map[string]any
}

type recorder struct {
	mu   sync.Mutex
	reqs []recordedReq
}

func (r *recorder) record(req *http.Request) {
	rec := recordedReq{method: req.Method, path: req.URL.Path}
	if req.Body != nil {
		_ = json.NewDecoder(req.Body).Decode(&rec.body)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reqs = append(r.reqs, rec)
}

// nth returns the n-th recorded request, failing the test if it was never made.
func (r *recorder) nth(t *testing.T, n int) recordedReq {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if n >= len(r.reqs) {
		t.Fatalf("expected at least %d requests, saw %d", n+1, len(r.reqs))
	}
	return r.reqs[n]
}

func (r *recorder) last(t *testing.T) recordedReq {
	t.Helper()
	r.mu.Lock()
	n := len(r.reqs)
	r.mu.Unlock()
	return r.nth(t, n-1)
}

// serve spins up a fake Adaptive API. The handler is given the recorded request
// so tests can branch on the path.
func serve(t *testing.T, rec *recorder, handler func(w http.ResponseWriter, r *http.Request)) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if rec != nil {
			rec.record(r)
		}
		w.Header().Set("Content-Type", "application/json")
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	return NewClient("test-token", srv.URL)
}

func TestNewClientURLNormalization(t *testing.T) {
	if got := NewClient("t", "").workspaceURL; got != defaultAdaptiveURL {
		t.Errorf("empty workspace URL: got %q want %q", got, defaultAdaptiveURL)
	}
	// A trailing slash must not produce a double slash before /api/v1: the
	// backend routes are path-exact and would 404.
	for _, in := range []string{"https://app.adaptive.live", "https://app.adaptive.live/", "https://app.adaptive.live///"} {
		if got := NewClient("t", in).workspaceURL; got != "https://app.adaptive.live/api/v1" {
			t.Errorf("NewClient(%q): got %q", in, got)
		}
	}
}

// ---------------------------------------------------------------------------
// Typed reads
// ---------------------------------------------------------------------------

func TestReadResourceDecodes(t *testing.T) {
	rec := &recorder{}
	c := serve(t, rec, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"id": "res-1", "name": "prod-db", "integrationType": "postgres",
			"userTags": ["a", "b"], "defaultCluster": "c1",
			"configuration": {"host": "db.internal", "port": 5432},
			"redactedKeys": ["password"],
			"redactedDigests": {"password": "sha256:abc"},
			"status": "created", "createdAt": "2026-01-01T00:00:00Z", "updatedAt": "2026-01-02T00:00:00Z"
		}`))
	})

	got, err := c.ReadResource(context.Background(), "res-1")
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	if got := rec.last(t).path; got != "/api/v1/terraform/resource/read/res-1" {
		t.Errorf("path: got %q", got)
	}
	if got.Name != "prod-db" || got.IntegrationType != "postgres" || got.DefaultCluster != "c1" {
		t.Errorf("scalar fields not decoded: %+v", got)
	}
	if len(got.UserTags) != 2 || got.Configuration["host"] != "db.internal" {
		t.Errorf("collection fields not decoded: %+v", got)
	}
	if got.RedactedDigests["password"] != "sha256:abc" || len(got.RedactedKeys) != 1 {
		t.Errorf("redaction fields not decoded: %+v", got)
	}
}

// The session read route answers 202 Accepted on some deployments and 200 on
// others; both must decode rather than being reported as an error.
func TestReadSessionAcceptsOKAndAccepted(t *testing.T) {
	for _, code := range []int{http.StatusOK, http.StatusAccepted} {
		c := serve(t, nil, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
			_, _ = w.Write([]byte(`{
				"id": "s-1", "name": "ep", "resource": "prod-db", "cluster": "c1",
				"authorization": "ro", "status": "created", "sessionType": "cli",
				"ttl": "8h", "idleTimeout": "15m", "pauseTimeout": "1h",
				"memory": "1Gi", "cpu": "500m",
				"isJitEnabled": true, "jitMode": "approval", "jitMultiApprover": true,
				"jitTotalApprovers": 3, "autoApproval": true,
				"sessionUsers": ["u@x.com"], "accessApprovers": ["a@x.com"],
				"groups": ["g1"], "userTags": ["t1"],
				"scriptOnlyAccess": true, "disableOutputCapture": true,
				"disableDataStudio": true, "disableWebCli": true,
				"exposed": true, "exposeType": "https", "exposeStatus": "ready"
			}`))
		})

		got, err := c.ReadSession(context.Background(), "s-1")
		if err != nil {
			t.Fatalf("ReadSession(status %d): %v", code, err)
		}
		if got == nil {
			t.Fatalf("ReadSession(status %d): nil result", code)
		}
		if !got.IsJITEnabled || got.JITMode != "approval" || !got.JITMultiApprover || got.JITTotalApprovers != 3 || !got.AutoApproval {
			t.Errorf("JIT fields not decoded: %+v", got)
		}
		if !got.DisableDataStudio || !got.DisableWebCLI || !got.DisableOutputCapture || !got.ScriptOnlyAccess {
			t.Errorf("toggle fields not decoded: %+v", got)
		}
		if !got.Exposed || got.ExposeType != "https" || got.ExposeStatus != "ready" {
			t.Errorf("expose fields not decoded: %+v", got)
		}
		if got.Resource != "prod-db" || got.Cluster != "c1" || got.Authorization != "ro" || got.TTL != "8h" {
			t.Errorf("association fields not decoded: %+v", got)
		}
		if len(got.SessionUsers) != 1 || len(got.AccessApprovers) != 1 || len(got.Groups) != 1 || len(got.UserTags) != 1 {
			t.Errorf("list fields not decoded: %+v", got)
		}
	}
}

func TestReadAuthorizationDecodes(t *testing.T) {
	rec := &recorder{}
	c := serve(t, rec, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"a-1","name":"ro","description":"read only",
			"resourceType":"postgres","permissions":"SELECT","status":"created","type":"custom"}`))
	})

	got, err := c.ReadAuthorization(context.Background(), "a-1")
	if err != nil {
		t.Fatalf("ReadAuthorization: %v", err)
	}
	if got := rec.last(t).path; got != "/api/v1/terraform/authorization/read/a-1" {
		t.Errorf("path: got %q", got)
	}
	if got.Name != "ro" || got.Description != "read only" || got.ResourceType != "postgres" || got.Permissions != "SELECT" {
		t.Errorf("not decoded: %+v", got)
	}
}

// ReadTeam must hit the /team/read/:id route. The previous GetTeam issued
// GET /team/:id, which is not a route the backend serves.
func TestReadTeamHitsReadRoute(t *testing.T) {
	rec := &recorder{}
	c := serve(t, rec, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/terraform/team/read/t-1") {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"id":"t-1","name":"oncall","members":["a@x.com","b@x.com"],
			"endpoints":["ep1"],"slackChannelId":"C123","createdBy":"admin@x.com","createdAt":"2026-01-01T00:00:00Z"}`))
	})

	got, err := c.ReadTeam(context.Background(), "t-1")
	if err != nil {
		t.Fatalf("ReadTeam: %v", err)
	}
	if got == nil {
		t.Fatal("ReadTeam returned nil for an existing team")
	}
	if req := rec.last(t); req.method != http.MethodGet || req.path != "/api/v1/terraform/team/read/t-1" {
		t.Errorf("request: got %s %s", req.method, req.path)
	}
	if got.Name != "oncall" || got.SlackChannelID != "C123" || len(got.Members) != 2 || len(got.Endpoints) != 1 {
		t.Errorf("not decoded: %+v", got)
	}
}

func TestReadScriptDecodes(t *testing.T) {
	c := serve(t, nil, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"sc-1","name":"backup","endpoint":"ep1","description":"nightly",
			"parameterDescriptions":{"path":"where to write"},"isAutoGen":false,
			"commandOmitted":true,"commandDigest":"sha256:xyz","createdBy":"a@x.com"}`))
	})

	got, err := c.ReadScript(context.Background(), "sc-1")
	if err != nil {
		t.Fatalf("ReadScript: %v", err)
	}
	if got.Name != "backup" || got.Endpoint != "ep1" || got.Description != "nightly" {
		t.Errorf("not decoded: %+v", got)
	}
	if !got.CommandOmitted || got.CommandDigest != "sha256:xyz" {
		t.Errorf("command fingerprint not decoded: %+v", got)
	}
	if got.ParameterDescriptions["path"] != "where to write" {
		t.Errorf("parameter descriptions not decoded: %+v", got)
	}
}

func TestGetScheduleDecodesFullReadView(t *testing.T) {
	c := serve(t, nil, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"sch-1","name":"biz-hours","scheduleType":"custom","isActive":true,
			"allDay":false,"description":"business hours","startHour":9,"startMinute":30,"endHour":17,"endMinute":0,
			"weekdays":["monday","tuesday"],"startDay":1,"endDay":15,
			"specificDates":["2030-01-01T00:00:00Z"],"users":["u@x.com"],"teams":["oncall"],
			"endpoints":["ep1"],"maxAccessTime":60,"timezone":"UTC","operationType":"autoapprove"}`))
	})

	got, err := c.GetSchedule(context.Background(), "sch-1")
	if err != nil {
		t.Fatalf("GetSchedule: %v", err)
	}
	if got.Description != "business hours" || got.StartHour != 9 || got.StartMinute != 30 || got.EndHour != 17 {
		t.Errorf("window fields not decoded: %+v", got)
	}
	if len(got.Weekdays) != 2 || got.StartDay != 1 || got.EndDay != 15 || len(got.SpecificDates) != 1 {
		t.Errorf("pattern fields not decoded: %+v", got)
	}
	if got.MaxAccessTime == nil || *got.MaxAccessTime != 60 {
		t.Errorf("maxAccessTime not decoded: %+v", got.MaxAccessTime)
	}
	if len(got.Users) != 1 || len(got.Teams) != 1 || len(got.EndpointNames()) != 1 {
		t.Errorf("membership fields not decoded: %+v", got)
	}
}

// EndpointNames falls back to the older mappedEndpoints key so a schedule read
// against an older backend still reports its endpoints.
func TestScheduleEndpointNamesFallback(t *testing.T) {
	r := &ScheduleResponse{MappedEndpoints: []string{"ep1"}}
	if got := r.EndpointNames(); len(got) != 1 || got[0] != "ep1" {
		t.Errorf("fallback to mappedEndpoints: got %v", got)
	}
	r.Endpoints = []string{"ep2"}
	if got := r.EndpointNames(); len(got) != 1 || got[0] != "ep2" {
		t.Errorf("endpoints should win: got %v", got)
	}
}

// A 404 on any read means the object is gone, which callers turn into
// "drop it from state" — it must not surface as an error.
func TestReadsTreatNotFoundAsGone(t *testing.T) {
	c := serve(t, nil, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
	})
	ctx := context.Background()

	if got, err := c.ReadResource(ctx, "x"); got != nil || err != nil {
		t.Errorf("ReadResource: got (%v, %v)", got, err)
	}
	if got, err := c.ReadSession(ctx, "x"); got != nil || err != nil {
		t.Errorf("ReadSession: got (%v, %v)", got, err)
	}
	if got, err := c.ReadAuthorization(ctx, "x"); got != nil || err != nil {
		t.Errorf("ReadAuthorization: got (%v, %v)", got, err)
	}
	if got, err := c.ReadTeam(ctx, "x"); got != nil || err != nil {
		t.Errorf("ReadTeam: got (%v, %v)", got, err)
	}
	if got, err := c.ReadScript(ctx, "x"); got != nil || err != nil {
		t.Errorf("ReadScript: got (%v, %v)", got, err)
	}
	if got, err := c.GetSchedule(ctx, "x"); got != nil || err != nil {
		t.Errorf("GetSchedule: got (%v, %v)", got, err)
	}
	if got, err := c.ReadDataProtection(ctx, "x"); got != nil || err != nil {
		t.Errorf("ReadDataProtection: got (%v, %v)", got, err)
	}
}

func TestReadSurfacesServerError(t *testing.T) {
	c := serve(t, nil, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	if _, err := c.ReadAuthorization(context.Background(), "a-1"); err == nil {
		t.Fatal("expected an error on a 500 read")
	} else if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error should carry the server body: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Writes
// ---------------------------------------------------------------------------

// The endpoint toggles added in this round are only useful if they reach the
// wire under the keys the backend binds.
func TestCreateSessionSerializesNewFields(t *testing.T) {
	rec := &recorder{}
	c := serve(t, rec, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/read/") {
			_, _ = w.Write([]byte(`{"id":"s-1","status":"created"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"s-1"}`))
	})

	autoApproval, multi := true, true
	total := 2
	resp, err := c.CreateSession(context.Background(), CreateSessionRequest{
		SessionName:          "ep",
		ResourceName:         "prod-db",
		SessionType:          "cli",
		DisableOutputCapture: true,
		DisableDataStudio:    true,
		DisableWebCLI:        true,
		JITMode:              "approval",
		AutoApproval:         &autoApproval,
		JITMultiApprover:     &multi,
		JITTotalApprovers:    &total,
	})
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if resp.ID != "s-1" {
		t.Errorf("id: got %q", resp.ID)
	}

	create := rec.nth(t, 0)
	if create.path != "/api/v1/terraform/session/create" {
		t.Errorf("create path: got %q", create.path)
	}
	// The create is followed by a status read, which is how CreateSession waits
	// for the endpoint to become functional.
	if got := rec.nth(t, 1).path; got != "/api/v1/terraform/session/read/s-1" {
		t.Errorf("create should poll the session status, second request was %q", got)
	}
	for key, want := range map[string]any{
		"disable_output_capture": true,
		"disable_data_studio":    true,
		"disable_web_cli":        true,
		"jit_mode":               "approval",
		"auto_approval":          true,
		"jit_multi_approver":     true,
		"jit_total_approvers":    float64(2),
	} {
		if got, ok := create.body[key]; !ok {
			t.Errorf("payload missing %q", key)
		} else if got != want {
			t.Errorf("payload %q: got %v want %v", key, got, want)
		}
	}

	if _, err := c.UpdateSession(context.Background(), "s-1", CreateSessionRequest{
		SessionName: "ep",
		JITMode:     "approval",
	}); err != nil {
		t.Fatalf("UpdateSession: %v", err)
	}
	update := rec.last(t)
	if update.path != "/api/v1/terraform/session/update/s-1" {
		t.Errorf("update path: got %q", update.path)
	}
	if update.body["jit_mode"] != "approval" {
		t.Errorf("update payload: %v", update.body)
	}
}

// The optional JIT fields are pointers so an unset input is omitted entirely
// and the server keeps its own default instead of being reset.
func TestCreateSessionOmitsUnsetOptionalJITFields(t *testing.T) {
	rec := &recorder{}
	c := serve(t, rec, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"s-1"}`))
	})
	if _, err := c.UpdateSession(context.Background(), "s-1", CreateSessionRequest{SessionName: "ep"}); err != nil {
		t.Fatalf("UpdateSession: %v", err)
	}
	body := rec.last(t).body
	for _, key := range []string{"jit_mode", "auto_approval", "jit_multi_approver", "jit_total_approvers"} {
		if _, ok := body[key]; ok {
			t.Errorf("unset optional %q should be omitted from the payload", key)
		}
	}
	// The plain bools are always sent: false is a meaningful value there.
	for _, key := range []string{"disable_output_capture", "disable_data_studio", "disable_web_cli"} {
		if _, ok := body[key]; !ok {
			t.Errorf("%q must always be sent", key)
		}
	}
}

// Scripts bind PascalCase keys server-side.
func TestScriptRequestPayloadKeys(t *testing.T) {
	rec := &recorder{}
	c := serve(t, rec, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"sc-1"}`))
	})

	req := ScriptRequest{
		Name:                  "backup",
		Command:               "pg_dump",
		Endpoint:              "ep1",
		Description:           "nightly",
		ParameterDescriptions: map[string]string{"path": "where to write"},
	}
	if _, err := c.CreateScript(context.Background(), req); err != nil {
		t.Fatalf("CreateScript: %v", err)
	}
	create := rec.last(t)
	if create.path != "/api/v1/terraform/script/create" {
		t.Errorf("create path: got %q", create.path)
	}
	for key, want := range map[string]any{
		"Name": "backup", "Command": "pg_dump", "Endpoint": "ep1", "Description": "nightly",
	} {
		if got := create.body[key]; got != want {
			t.Errorf("payload %q: got %v want %v", key, got, want)
		}
	}
	params, ok := create.body["ParameterDescriptions"].(map[string]any)
	if !ok || params["path"] != "where to write" {
		t.Errorf("ParameterDescriptions not sent: %v", create.body["ParameterDescriptions"])
	}

	if err := c.UpdateScript(context.Background(), "sc-1", req); err != nil {
		t.Fatalf("UpdateScript: %v", err)
	}
	if got := rec.last(t).path; got != "/api/v1/terraform/script/update/sc-1" {
		t.Errorf("update path: got %q", got)
	}
}

// The slack channel only reaches the backend through the update route, so the
// key it binds must be exact.
func TestUpdateTeamSendsSlackChannelID(t *testing.T) {
	rec := &recorder{}
	c := serve(t, rec, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	})

	if err := c.UpdateTeam(context.Background(), "t-1", "oncall", []string{"a@x.com"}, []string{"ep1"}, "C123"); err != nil {
		t.Fatalf("UpdateTeam: %v", err)
	}
	req := rec.last(t)
	if req.path != "/api/v1/terraform/team/update/t-1" {
		t.Errorf("path: got %q", req.path)
	}
	if req.body["SlackChannelID"] != "C123" {
		t.Errorf("SlackChannelID not sent: %v", req.body)
	}
	if req.body["Name"] != "oncall" {
		t.Errorf("Name not sent: %v", req.body)
	}
}

func TestCreateTeamPayload(t *testing.T) {
	rec := &recorder{}
	c := serve(t, rec, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"t-1"}`))
	})

	got, err := c.CreateTeam(context.Background(), "oncall", []string{"a@x.com"}, nil)
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}
	if got.ID != "t-1" {
		t.Errorf("id: got %q", got.ID)
	}
	req := rec.last(t)
	if req.path != "/api/v1/terraform/team/create" || req.body["Name"] != "oncall" {
		t.Errorf("request: %s %v", req.path, req.body)
	}
	// The create route has no slack field — a group's channel is applied by a
	// follow-up update.
	if _, ok := req.body["SlackChannelID"]; ok {
		t.Errorf("create must not send SlackChannelID: %v", req.body)
	}
}

// ---------------------------------------------------------------------------
// Data protection
// ---------------------------------------------------------------------------

func TestDataProtectionCRUDPaths(t *testing.T) {
	rec := &recorder{}
	c := serve(t, rec, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"res-1","resourceName":"prod-db","authorizationName":"masked_prod-db",
			"scoped":true,"status":"applied","lastAppliedAt":"2026-01-01T00:00:00Z",
			"masks":[{"databaseName":"app","tables":[{"tableName":"users","schema":"public","masked":true,
			"maskedColumns":[{"columnName":"email","maskingType":"email"}]}]}]}`))
	})
	ctx := context.Background()

	scoped := true
	req := DataProtectionRequest{
		ResourceName: "prod-db",
		Scoped:       &scoped,
		Masks: []DataProtectionMask{{
			DatabaseName: "app",
			Tables: []DataProtectionTable{{
				TableName:     "users",
				Schema:        "public",
				Masked:        &scoped,
				MaskedColumns: []DataProtectionColumnMask{{ColumnName: "email", MaskingType: "email"}},
			}},
		}},
	}

	got, err := c.CreateDataProtection(ctx, req)
	if err != nil {
		t.Fatalf("CreateDataProtection: %v", err)
	}
	create := rec.last(t)
	if create.path != "/api/v1/terraform/dataprotection/create" {
		t.Errorf("create path: got %q", create.path)
	}
	if create.body["resourceName"] != "prod-db" || create.body["scoped"] != true {
		t.Errorf("create payload: %v", create.body)
	}
	if got.ID != "res-1" || got.AuthorizationName != "masked_prod-db" || got.Scoped == nil || !*got.Scoped {
		t.Errorf("create response not decoded: %+v", got)
	}
	if len(got.Masks) != 1 || len(got.Masks[0].Tables) != 1 ||
		len(got.Masks[0].Tables[0].MaskedColumns) != 1 ||
		got.Masks[0].Tables[0].MaskedColumns[0].MaskingType != "email" {
		t.Errorf("nested masks not decoded: %+v", got.Masks)
	}

	if _, err := c.UpdateDataProtection(ctx, "res-1", req); err != nil {
		t.Fatalf("UpdateDataProtection: %v", err)
	}
	if got := rec.last(t).path; got != "/api/v1/terraform/dataprotection/update/res-1" {
		t.Errorf("update path: got %q", got)
	}

	if _, err := c.ReadDataProtection(ctx, "res-1"); err != nil {
		t.Fatalf("ReadDataProtection: %v", err)
	}
	if req := rec.last(t); req.method != http.MethodGet || req.path != "/api/v1/terraform/dataprotection/read/res-1" {
		t.Errorf("read request: got %s %s", req.method, req.path)
	}

	if err := c.DeleteDataProtection(ctx, "res-1"); err != nil {
		t.Fatalf("DeleteDataProtection: %v", err)
	}
	if req := rec.last(t); req.method != http.MethodPost || req.path != "/api/v1/terraform/dataprotection/delete/res-1" {
		t.Errorf("delete request: got %s %s", req.method, req.path)
	}
}

// An unset Scoped means "server default"; it must not be sent as false.
func TestDataProtectionOmitsUnsetScoped(t *testing.T) {
	rec := &recorder{}
	c := serve(t, rec, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"res-1"}`))
	})
	if _, err := c.CreateDataProtection(context.Background(), DataProtectionRequest{ResourceName: "prod-db"}); err != nil {
		t.Fatalf("CreateDataProtection: %v", err)
	}
	body := rec.last(t).body
	if _, ok := body["scoped"]; ok {
		t.Errorf("unset scoped should be omitted: %v", body)
	}
}

// ---------------------------------------------------------------------------
// Deletes
// ---------------------------------------------------------------------------

// Deleting an endpoint must hard-delete the row once the session is terminal,
// otherwise the resource it points at cannot be deleted afterwards.
func TestDeleteSessionForceDeletesAfterTermination(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	c := serve(t, nil, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.URL.Path)
		mu.Unlock()
		if strings.Contains(r.URL.Path, "/read/") {
			_, _ = w.Write([]byte(`{"id":"s-1","status":"terminated"}`))
			return
		}
		_, _ = w.Write([]byte(`{}`))
	})

	ok, err := c.DeleteSession(context.Background(), "s-1")
	if err != nil || !ok {
		t.Fatalf("DeleteSession: (%v, %v)", ok, err)
	}
	want := []string{
		"/api/v1/terraform/session/delete/s-1",
		"/api/v1/terraform/session/read/s-1",
		"/api/v1/terraform/session/forcedelete/s-1",
	}
	if len(seen) != len(want) {
		t.Fatalf("request sequence: got %v want %v", seen, want)
	}
	for i := range want {
		if seen[i] != want[i] {
			t.Errorf("request %d: got %q want %q", i, seen[i], want[i])
		}
	}
}

// A session whose row is already gone needs no force delete.
func TestDeleteSessionSkipsForceDeleteWhenAlreadyGone(t *testing.T) {
	var mu sync.Mutex
	forced := false
	c := serve(t, nil, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/forcedelete/"):
			mu.Lock()
			forced = true
			mu.Unlock()
			_, _ = w.Write([]byte(`{}`))
		case strings.Contains(r.URL.Path, "/read/"):
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	})

	ok, err := c.DeleteSession(context.Background(), "s-1")
	if err != nil || !ok {
		t.Fatalf("DeleteSession: (%v, %v)", ok, err)
	}
	if forced {
		t.Error("force delete issued for a session that no longer exists")
	}
}

func TestBadTokenIsReported(t *testing.T) {
	c := serve(t, nil, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	if _, err := c.ReadAuthorization(context.Background(), "a-1"); err == nil ||
		!strings.Contains(err.Error(), "bad token") {
		t.Errorf("401 should report a bad token, got %v", err)
	}
}
