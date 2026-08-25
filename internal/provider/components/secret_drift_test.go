package components

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/adaptive-scale/terraform-provider-adaptive/internal/provider/integrations"
)

func TestDriftedDigestKeys(t *testing.T) {
	cases := []struct {
		name    string
		applied map[string]string
		current map[string]string
		want    []string
	}{
		{"nil applied (caller seeds instead)", nil, map[string]string{"password": "d1"}, []string{"password"}},
		{"nil current (old server)", map[string]string{"password": "d1"}, nil, nil},
		{"unchanged", map[string]string{"password": "d1"}, map[string]string{"password": "d1"}, nil},
		{"changed", map[string]string{"password": "d1"}, map[string]string{"password": "d2"}, []string{"password"}},
		// Inverted deliberately: a secret appearing where none was recorded is
		// someone setting one outside the configuration, which has to surface.
		{"added since the last write is drift", map[string]string{"password": "d1"},
			map[string]string{"password": "d1", "sshKey": "d9"}, []string{"sshKey"}},
		{"removed key is not drift", map[string]string{"password": "d1", "sshKey": "d9"},
			map[string]string{"password": "d1"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := driftedDigestKeys(c.applied, c.current); !reflect.DeepEqual(got, c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestAttrForConfigKey(t *testing.T) {
	for _, c := range []struct{ typ, key, want string }{
		{"postgres", "password", "password"},
		{"postgres", "username", "username"},
		{"aws", "aws_secret_access_key", "secret_access_key"},
		{"aws", "aws_region_name", "region_name"},
		{"gcp", "key_file", "key_file"},
		{"azure", "clientSecret", "client_secret"},
	} {
		t.Run(c.typ+"/"+c.key, func(t *testing.T) {
			got, ok := integrations.AttrForConfigKey(c.typ, c.key)
			if !ok {
				t.Fatal("did not resolve")
			}
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}

	for _, c := range []struct{ typ, key string }{
		{"postgres", "no_such_key"},
		{"postgres", ""},
		{"postgres", "targets[0].password"},
	} {
		if attr, ok := integrations.AttrForConfigKey(c.typ, c.key); ok {
			t.Errorf("%s/%q resolved to %q; an unresolvable key must report false "+
				"rather than mark the wrong attribute", c.typ, c.key, attr)
		}
	}
}

// isSecretStringAttr is what stops a fingerprint being written somewhere it
// would either print in clear or not fit at all.
func TestIsSecretStringAttr(t *testing.T) {
	if !isSecretStringAttr("password") {
		t.Error("password must be a sensitive string, or the marker would print in clear")
	}
	if isSecretStringAttr("username") {
		t.Error("username is not sensitive and must not be marked")
	}
	if isSecretStringAttr("targets") {
		t.Error("targets is a list; a fingerprint has nowhere to go there")
	}
	if isSecretStringAttr("no_such_attribute") {
		t.Error("an unknown attribute must not be marked")
	}
}

// readWithDigests drives the real Read against a canned resource body.
func readWithDigests(t *testing.T, state map[string]interface{}, serverDigest string) (*schema.ResourceData, string) {
	t.Helper()
	api := &fakeAPI{}
	client := api.start(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"id":"r-1","name":"db","integrationType":"postgres",
			"configuration":{"hostname":"h","username":"app","port":"5432"},
			"redactedKeys":["password"],
			"redactedDigests":{"password":"` + serverDigest + `"}}`))
	})

	d := schema.TestResourceDataRaw(t, ResourceAdaptiveResource().Schema, state)
	d.SetId("r-1")
	if diags := ResourceAdaptiveResourceRead(context.Background(), d, client); diags.HasError() {
		t.Fatalf("read: %+v", diags)
	}
	return d, d.Get("password").(string)
}

// The regression test for the reported bug. A secret the configuration never set
// used to drift in silence: the old mechanism signalled by blanking, and an
// absent argument already reads back as "", so the plan had nothing to show.
func TestSecretDriftSurfacesForUnmanagedAttribute(t *testing.T) {
	// No password in the configuration — the "not under management" case.
	_, password := readWithDigests(t, map[string]interface{}{
		"name": "db", "type": "postgres", "hostname": "h", "username": "app",
		"applied_digests": map[string]interface{}{"password": "stale-digest"},
	}, "server-digest")

	if password == "" {
		t.Fatal("drift on an unmanaged secret must still reach state; " +
			"an empty password is exactly the silence this fixes")
	}
	if password != "server-digest" {
		t.Errorf("expected the fingerprint as the marker, got %q", password)
	}
}

func TestSecretDriftSurfacesForManagedAttribute(t *testing.T) {
	_, password := readWithDigests(t, map[string]interface{}{
		"name": "db", "type": "postgres", "hostname": "h", "username": "app",
		"password":        "s3cret",
		"applied_digests": map[string]interface{}{"password": "stale-digest"},
	}, "server-digest")

	if password == "s3cret" {
		t.Error("a drifted secret must not keep the state value, or the plan stays clean")
	}
	if password != "server-digest" {
		t.Errorf("expected the fingerprint as the marker, got %q", password)
	}
}

// The common case: agreeing fingerprints must leave the argument alone. A false
// positive here is a diff on every plan, for every resource holding a secret.
func TestSecretDriftLeavesMatchingDigestsAlone(t *testing.T) {
	_, password := readWithDigests(t, map[string]interface{}{
		"name": "db", "type": "postgres", "hostname": "h", "username": "app",
		"password":        "s3cret",
		"applied_digests": map[string]interface{}{"password": "server-digest"},
	}, "server-digest")

	if password != "s3cret" {
		t.Errorf("an unchanged secret must keep the configuration's value, got %q", password)
	}
}

// The upgrade path: state written before applied_digests existed, and every
// import. Marking here would flag every secret on every resource the first time
// anyone refreshed.
func TestSecretDriftSeedsWhenNothingRecorded(t *testing.T) {
	d, password := readWithDigests(t, map[string]interface{}{
		"name": "db", "type": "postgres", "hostname": "h", "username": "app",
		"password": "s3cret",
	}, "server-digest")

	if password != "s3cret" {
		t.Errorf("password must keep the configuration's value while seeding, got %q", password)
	}
	seeded := stateStringMap(d, "applied_digests")
	if seeded["password"] != "server-digest" {
		t.Errorf("fingerprints should be seeded from the server, got %v", seeded)
	}
}
