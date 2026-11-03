package client

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
)

func TestMaxJITDurationRequestPresence(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		present     bool
		value       string
	}{
		{"omitted", `{}`, false, ""},
		{"clear", `{"max_jit_request_duration":""}`, true, ""},
		{"set", `{"max_jit_request_duration":"2h"}`, true, "2h"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var req CreateSessionRequest
			if err := json.Unmarshal([]byte(tc.input), &req); err != nil {
				t.Fatal(err)
			}
			for _, update := range []bool{false, true} {
				rec := &recorder{}
				c := serve(t, rec, func(w http.ResponseWriter, r *http.Request) {
					_, _ = w.Write([]byte(`{"id":"ep-1","name":"endpoint","status":"created"}`))
				})
				var err error
				if update {
					_, err = c.UpdateSession(context.Background(), "ep-1", req)
				} else {
					_, err = c.CreateSession(context.Background(), req)
				}
				if err != nil {
					t.Fatal(err)
				}
				got, present := rec.nth(t, 0).body["max_jit_request_duration"]
				if present != tc.present || (present && got != tc.value) {
					t.Errorf("update=%v: wire value=%#v present=%v, want %q present=%v", update, got, present, tc.value, tc.present)
				}
			}
		})
	}
	f, ok := reflect.TypeOf(CreateSessionRequest{}).FieldByName("MaxJITRequestDuration")
	if !ok || f.Type != reflect.TypeOf((*string)(nil)) {
		t.Error("request duration must be *string")
	}
}

func TestMaxJITDurationReadDTO(t *testing.T) {
	c := serve(t, nil, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"ep-1","name":"endpoint","maxJitRequestDuration":"2h"}`))
	})
	got, err := c.ReadSession(context.Background(), "ep-1")
	if err != nil {
		t.Fatal(err)
	}
	field := reflect.ValueOf(got).Elem().FieldByName("MaxJITRequestDuration")
	if !field.IsValid() || field.Kind() != reflect.String || field.String() != "2h" {
		t.Fatalf("read DTO did not decode maxJitRequestDuration: %+v", got)
	}
}
