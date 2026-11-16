package client

import (
	"context"
	"fmt"
	"net/http"
	"testing"
)

func nilReadResult[T any](got *T, err error) (bool, error) {
	return got == nil, err
}

func TestReadIdentityContract(t *testing.T) {
	ctx := context.Background()
	families := []struct {
		name     string
		identity string
		read     func(*Client) (bool, error)
	}{
		{"resource", "name", func(c *Client) (bool, error) { return nilReadResult(c.ReadResource(ctx, "r-1")) }},
		{"session", "name", func(c *Client) (bool, error) { return nilReadResult(c.ReadSession(ctx, "r-1")) }},
		{"authorization", "name", func(c *Client) (bool, error) { return nilReadResult(c.ReadAuthorization(ctx, "r-1")) }},
		{"team", "name", func(c *Client) (bool, error) { return nilReadResult(c.ReadTeam(ctx, "r-1")) }},
		{"script", "name", func(c *Client) (bool, error) { return nilReadResult(c.ReadScript(ctx, "r-1")) }},
		{"schedule", "name", func(c *Client) (bool, error) { return nilReadResult(c.GetSchedule(ctx, "r-1")) }},
		{"dataprotection", "resourceName", func(c *Client) (bool, error) { return nilReadResult(c.ReadDataProtection(ctx, "r-1")) }},
	}
	for _, family := range families {
		for _, status := range []int{http.StatusOK, http.StatusAccepted} {
			cases := []struct {
				name    string
				body    string
				wantErr bool
			}{
				{"null", `null`, true},
				{"empty", `{}`, true},
				{"empty-envelope", `{"data":{}}`, true},
				{"wrapped-object", fmt.Sprintf(`{"data":{"id":"r-1",%q:"known"}}`, family.identity), true},
				{"id-only", `{"id":"r-1"}`, true},
				{"empty-identity", fmt.Sprintf(`{"id":"r-1",%q:""}`, family.identity), true},
				{"wrong-id", fmt.Sprintf(`{"id":"wrong",%q:"known"}`, family.identity), true},
				{"legacy-no-id", fmt.Sprintf(`{%q:"known"}`, family.identity), false},
				{"legacy-empty-id", fmt.Sprintf(`{"id":"",%q:"known"}`, family.identity), false},
				{"matching-id", fmt.Sprintf(`{"id":"r-1",%q:"known"}`, family.identity), false},
			}
			for _, tc := range cases {
				t.Run(family.name+"/"+http.StatusText(status)+"/"+tc.name, func(t *testing.T) {
					c := serve(t, nil, func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(status)
						_, _ = w.Write([]byte(tc.body))
					})
					isNil, err := family.read(c)
					if (err != nil) != tc.wantErr {
						t.Errorf("read error = %v, wantErr %v", err, tc.wantErr)
					}
					if isNil != tc.wantErr {
						t.Errorf("nil response = %v, want %v (invalid responses must not reach callbacks)", isNil, tc.wantErr)
					}
				})
			}
		}
		t.Run(family.name+"/not-found", func(t *testing.T) {
			c := serve(t, nil, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`not JSON`))
			})
			if isNil, err := family.read(c); !isNil || err != nil {
				t.Errorf("404 must return (nil, nil), got nil=%v err=%v", isNil, err)
			}
		})
	}
}
