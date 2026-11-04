package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestCreateDoesNotReplayAcrossRedirect(t *testing.T) {
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected.Add(1); fmt.Fprint(w, `{"id":"id"}`) }))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", target.URL)
		w.WriteHeader(307)
	}))
	defer origin.Close()
	_, err := NewClient("token", origin.URL).CreateScript(context.Background(), ScriptRequest{})
	if err == nil || redirected.Load() != 0 {
		t.Fatal("non-idempotent create replayed or redirect accepted")
	}
}
func TestHTTPStatusCannotBeOverriddenByBody(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		fmt.Fprint(w, `{"StatusCode":200,"error":"secret-canary","unresolvedUsers":["secret-canary"]}`)
	}))
	defer s.Close()
	_, err := NewClient("token", s.URL).CreateSchedule(context.Background(), &ScheduleRequest{Name: "n", Users: []string{"safe-user"}})
	var status *HTTPError
	if !errors.As(err, &status) || status.StatusCode != 400 {
		t.Fatal("lost authoritative status")
	}
	if strings.Contains(err.Error(), "secret-canary") {
		t.Fatal("untrusted error payload leaked")
	}
}
func TestScheduleUnresolvedNamesRemainActionable(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		fmt.Fprint(w, `{"unresolvedUsers":["safe-user"],"unresolvedTeams":["safe-team"],"unresolvedEndpoints":["safe-endpoint"]}`)
	}))
	defer s.Close()
	_, err := NewClient("token", s.URL).CreateSchedule(context.Background(), &ScheduleRequest{Name: "n", Users: []string{"safe-user"}, Teams: []string{"safe-team"}, Endpoints: []string{"safe-endpoint"}})
	if err == nil {
		t.Fatal("expected error")
	}
	for _, s := range []string{"safe-user", "safe-team", "safe-endpoint"} {
		if !strings.Contains(err.Error(), s) {
			t.Fatal("lost actionable unresolved name")
		}
	}
}
func TestDecodeDiagnosticsDoNotEchoServerValues(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":"id","name":"n","jitTotalApprovers":1234567890123456789012345678901234567890}`)
	}))
	defer s.Close()
	_, err := NewClient("token", s.URL).ReadSession(context.Background(), "id")
	if err == nil || strings.Contains(err.Error(), "1234567890123456789012345678901234567890") {
		t.Fatal("decode error disclosed response value")
	}
}

func TestReadDiagnosticsDoNotEchoResponseIdentity(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"id":"synthetic-secret-canary","name":"n"}`)
	}))
	defer s.Close()
	_, err := NewClient("token", s.URL).ReadResource(context.Background(), "id")
	if err == nil || strings.Contains(err.Error(), "synthetic-secret-canary") {
		t.Fatal("response identity disclosed")
	}
}

func TestInFlightCancellationDoesNotLeakBetweenResources(t *testing.T) {
	entered := make(chan struct{})
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/first") {
			close(entered)
			<-r.Context().Done()
			return
		}
		fmt.Fprint(w, `{"id":"second","name":"n"}`)
	}))
	defer s.Close()
	c := NewClient("token", s.URL)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := c.ReadResource(ctx, "first"); done <- err }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("request not entered")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("lost in-flight cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("request did not cancel")
	}
	if _, err := c.ReadResource(context.Background(), "second"); err != nil {
		t.Fatal("resource context leaked", err)
	}
}
